package database

import (
	"context"
	"encoding/json"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/model"
	"main/internal/gen/table"
)

type ProfileWorkExperienceBullet struct {
	ID       int64  `json:"id"`
	Bullet   string `json:"bullet"`
	Position int    `json:"position"`
}

type ProfileWorkExperience struct {
	ID        int64                         `json:"id"`
	Company   string                        `json:"company"`
	JobTitle  string                        `json:"job_title"`
	JobType   JobType                       `json:"job_type"`
	Location  *string                       `json:"location"`
	StartDate *string                       `json:"start_date"`
	EndDate   *string                       `json:"end_date"`
	Bullets   []ProfileWorkExperienceBullet `json:"bullets"`
}

type AddWorkExperienceRequest struct {
	Company   string `json:"company"`
	JobTitle  string `json:"job_title"`
	JobType   string `json:"job_type"`
	Location  string `json:"location"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type AddWorkExperienceBulletRequest struct {
	Bullet   string `json:"bullet"`
	Position int    `json:"position"`
}

type UpdateWorkExperienceBulletRequest struct {
	Bullet string `json:"bullet"`
}

func listWorkExperience(ctx context.Context, profileID int64) ([]ProfileWorkExperience, error) {
	var rows []struct {
		model.ProfilesWorkExperience
		Bullets []model.ProfilesWorkExperienceBullets
	}

	// Bullets ride along in a join, so the profile loads in one query, not one per entry.
	experience, bullets := table.ProfilesWorkExperience, table.ProfilesWorkExperienceBullets
	stmt := pg.SELECT(experience.AllColumns, bullets.AllColumns).
		FROM(experience.LEFT_JOIN(bullets, bullets.WorkExperienceID.EQ(experience.ID))).
		WHERE(experience.ProfileID.EQ(pg.Int(profileID))).
		ORDER_BY(experience.StartDate.DESC().NULLS_LAST(), experience.ID, bullets.Position, bullets.ID)

	if err := stmt.QueryContext(ctx, db(), &rows); err != nil {
		return nil, err
	}

	var entries []ProfileWorkExperience

	for _, row := range rows {
		entry := ProfileWorkExperience{
			ID:        int64(row.ID),
			Company:   row.Company,
			JobTitle:  row.JobTitle,
			JobType:   JobType(row.JobType),
			Location:  row.Location,
			StartDate: formatDate(row.StartDate),
			EndDate:   formatDate(row.EndDate),
		}

		// No bullets leaves Bullets nil.
		for _, bullet := range row.Bullets {
			entry.Bullets = append(entry.Bullets, ProfileWorkExperienceBullet{
				ID:       int64(bullet.ID),
				Bullet:   bullet.Bullet,
				Position: int(bullet.Position),
			})
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// workExperienceRow validates req into the row AddWorkExperience and UpdateWorkExperience write.
func workExperienceRow(profileID int64, req *AddWorkExperienceRequest) (model.ProfilesWorkExperience, error) {
	if req.Company == "" || req.JobTitle == "" {
		return model.ProfilesWorkExperience{}, invalidInput("company and job_title are required")
	}

	jobType, ok := ParseJobType(req.JobType)

	if !ok {
		return model.ProfilesWorkExperience{}, invalidInput("invalid job_type: %s", req.JobType)
	}

	startDate, err := parseOptionalDate(req.StartDate)

	if err != nil {
		return model.ProfilesWorkExperience{}, invalidInput("invalid start_date: %s", err)
	}

	endDate, err := parseOptionalDate(req.EndDate)

	if err != nil {
		return model.ProfilesWorkExperience{}, invalidInput("invalid end_date: %s", err)
	}

	row := model.ProfilesWorkExperience{
		ProfileID: new(int32(profileID)),
		Company:   req.Company,
		JobTitle:  req.JobTitle,
		JobType:   int32(jobType),
		StartDate: startDate,
		EndDate:   endDate,
	}

	if req.Location != "" {
		row.Location = &req.Location
	}

	return row, nil
}

func AddWorkExperience(ctx context.Context, profileID int64, req *AddWorkExperienceRequest) (int64, error) {
	row, err := workExperienceRow(profileID, req)

	if err != nil {
		return 0, err
	}

	experience := table.ProfilesWorkExperience
	key := pg.ColumnList{experience.ProfileID, experience.Company, experience.JobTitle, experience.StartDate}
	stmt := experience.INSERT(experience.MutableColumns).MODEL(row).
		ON_CONFLICT(key...).DO_UPDATE(pg.SET(setExcluded(experience.MutableColumns, experience.EXCLUDED.AllColumns, key...))).
		RETURNING(experience.ID)

	if err := queryOne(ctx, db(), stmt, &row); err != nil {
		return 0, err
	}

	return int64(row.ID), nil
}

func UpdateWorkExperience(ctx context.Context, profileID, workExperienceID int64, req *AddWorkExperienceRequest) error {
	row, err := workExperienceRow(profileID, req)

	if err != nil {
		return err
	}

	experience := table.ProfilesWorkExperience

	return execOne(ctx, experience.UPDATE(experience.MutableColumns.Except(experience.ProfileID)).MODEL(row).
		WHERE(ownedWorkExperience(workExperienceID, profileID)))
}

// ownedWorkExperience matches the entry only when profileID owns it.
func ownedWorkExperience(workExperienceID, profileID int64) pg.BoolExpression {
	experience := table.ProfilesWorkExperience

	return experience.ID.EQ(pg.Int(workExperienceID)).AND(experience.ProfileID.EQ(pg.Int(profileID)))
}

// ownedBullet matches the bullet only under workExperienceID, and only when profileID owns that entry.
func ownedBullet(bulletID, workExperienceID, profileID int64) pg.BoolExpression {
	experience, bullets := table.ProfilesWorkExperience, table.ProfilesWorkExperienceBullets

	return bullets.ID.EQ(pg.Int(bulletID)).
		AND(bullets.WorkExperienceID.EQ(pg.Int(workExperienceID))).
		AND(bullets.WorkExperienceID.IN(pg.SELECT(experience.ID).FROM(experience).WHERE(experience.ProfileID.EQ(pg.Int(profileID)))))
}

func DeleteWorkExperience(ctx context.Context, profileID, workExperienceID int64) error {
	experience, bullets := table.ProfilesWorkExperience, table.ProfilesWorkExperienceBullets

	// Scoped to the owner, so a wrong profile can't wipe someone else's bullets.
	if _, err := exec(ctx, db(), bullets.DELETE().WHERE(bullets.WorkExperienceID.IN(
		pg.SELECT(experience.ID).FROM(experience).WHERE(ownedWorkExperience(workExperienceID, profileID))))); err != nil {
		return err
	}

	return execOne(ctx, experience.DELETE().WHERE(ownedWorkExperience(workExperienceID, profileID)))
}

func AddWorkExperienceBullet(ctx context.Context, profileID, workExperienceID int64, req *AddWorkExperienceBulletRequest) (int64, error) {
	if req.Bullet == "" {
		return 0, invalidInput("bullet is required")
	}

	var owned model.ProfilesWorkExperience

	experience, bullets := table.ProfilesWorkExperience, table.ProfilesWorkExperienceBullets

	// sql.ErrNoRows when the entry is missing or someone else's.
	if err := queryOne(ctx, db(), pg.SELECT(experience.ID).FROM(experience).
		WHERE(ownedWorkExperience(workExperienceID, profileID)), &owned); err != nil {
		return 0, err
	}

	row := model.ProfilesWorkExperienceBullets{WorkExperienceID: &owned.ID, Bullet: req.Bullet, Position: int32(req.Position)}
	key := pg.ColumnList{bullets.WorkExperienceID, bullets.Bullet}
	stmt := bullets.INSERT(bullets.MutableColumns).MODEL(row).
		ON_CONFLICT(key...).DO_UPDATE(pg.SET(setExcluded(bullets.MutableColumns, bullets.EXCLUDED.AllColumns, key...))).
		RETURNING(bullets.ID)

	if err := queryOne(ctx, db(), stmt, &row); err != nil {
		return 0, err
	}

	return int64(row.ID), nil
}

func UpdateWorkExperienceBullet(ctx context.Context, profileID, workExperienceID, bulletID int64, req *UpdateWorkExperienceBulletRequest) error {
	if req.Bullet == "" {
		return invalidInput("bullet is required")
	}

	bullets := table.ProfilesWorkExperienceBullets

	return execOne(ctx, bullets.UPDATE().SET(bullets.Bullet.SET(pg.String(req.Bullet))).
		WHERE(ownedBullet(bulletID, workExperienceID, profileID)))
}

func DeleteWorkExperienceBullet(ctx context.Context, profileID, workExperienceID, bulletID int64) error {
	bullets := table.ProfilesWorkExperienceBullets

	return execOne(ctx, bullets.DELETE().WHERE(ownedBullet(bulletID, workExperienceID, profileID)))
}

type JobType int

const (
	JobTypeUnknown JobType = iota
	JobTypeContract
	JobTypeInternship
	JobTypePartTime
	JobTypeFullTime
)

func ParseJobType(s string) (JobType, bool) {
	switch s {
	case "", "unknown":
		return JobTypeUnknown, true
	case "contract":
		return JobTypeContract, true
	case "internship":
		return JobTypeInternship, true
	case "part_time":
		return JobTypePartTime, true
	case "full_time":
		return JobTypeFullTime, true
	default:
		return JobTypeUnknown, false
	}
}

func (j JobType) String() string {
	switch j {
	case JobTypeContract:
		return "contract"
	case JobTypeInternship:
		return "internship"
	case JobTypePartTime:
		return "part_time"
	case JobTypeFullTime:
		return "full_time"
	default:
		return "unknown"
	}
}

func (j JobType) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.String())
}

func (j *JobType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsed, _ := ParseJobType(s)
	*j = parsed

	return nil
}
