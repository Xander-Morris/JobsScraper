package database

import (
	"context"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/model"
	"main/internal/gen/table"
)

type ProfileSkill struct {
	ID    int64  `json:"id"`
	Skill string `json:"skill"`
}

type AddSkillRequest struct {
	Skill string `json:"skill"`
}

func listSkills(ctx context.Context, profileID int64) ([]ProfileSkill, error) {
	var rows []model.ProfilesSkills

	skills := table.ProfilesSkills
	stmt := pg.SELECT(skills.AllColumns).FROM(skills).
		WHERE(skills.ProfileID.EQ(pg.Int(profileID))).
		ORDER_BY(skills.Skill)

	if err := stmt.QueryContext(ctx, db(), &rows); err != nil {
		return nil, err
	}

	var list []ProfileSkill

	for _, row := range rows {
		list = append(list, ProfileSkill{ID: int64(row.ID), Skill: row.Skill})
	}

	return list, nil
}

func AddSkill(ctx context.Context, profileID int64, req *AddSkillRequest) (int64, error) {
	if req.Skill == "" {
		return 0, invalidInput("skill is required")
	}

	var row model.ProfilesSkills

	skills := table.ProfilesSkills
	key := pg.ColumnList{skills.ProfileID, skills.Skill}
	stmt := skills.INSERT(skills.MutableColumns).
		MODEL(model.ProfilesSkills{ProfileID: new(int32(profileID)), Skill: req.Skill}).
		ON_CONFLICT(key...).DO_UPDATE(pg.SET(setExcluded(pg.ColumnList{skills.Skill}, skills.EXCLUDED.AllColumns))).
		RETURNING(skills.ID)

	if err := queryOne(ctx, db(), stmt, &row); err != nil {
		return 0, err
	}

	return int64(row.ID), nil
}

func DeleteSkill(ctx context.Context, profileID, skillID int64) error {
	skills := table.ProfilesSkills

	return execOne(ctx, skills.DELETE().
		WHERE(skills.ID.EQ(pg.Int(skillID)).AND(skills.ProfileID.EQ(pg.Int(profileID)))))
}
