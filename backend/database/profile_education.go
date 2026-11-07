package database

import (
	"context"

	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/shopspring/decimal"

	"main/internal/gen/model"
	"main/internal/gen/table"
)

type ProfileEducation struct {
	ID         int64    `json:"id"`
	SchoolName string   `json:"school_name"`
	Major      string   `json:"major"`
	Degree     string   `json:"degree"`
	GPA        *float64 `json:"gpa"`
	StartDate  *string  `json:"start_date"`
	EndDate    *string  `json:"end_date"`
}

type AddEducationRequest struct {
	SchoolName string   `json:"school_name"`
	Major      string   `json:"major"`
	Degree     string   `json:"degree"`
	GPA        *float64 `json:"gpa"`
	StartDate  string   `json:"start_date"`
	EndDate    string   `json:"end_date"`
}

func listEducation(ctx context.Context, profileID int64) ([]ProfileEducation, error) {
	var rows []model.ProfilesEducation

	education := table.ProfilesEducation
	stmt := pg.SELECT(education.AllColumns).FROM(education).
		WHERE(education.ProfileID.EQ(pg.Int(profileID))).
		ORDER_BY(education.StartDate.DESC().NULLS_LAST(), education.ID)

	if err := stmt.QueryContext(ctx, db(), &rows); err != nil {
		return nil, err
	}

	var list []ProfileEducation

	for _, row := range rows {
		entry := ProfileEducation{
			ID:         int64(row.ID),
			SchoolName: row.SchoolName,
			Major:      row.Major,
			Degree:     row.Degree,
			StartDate:  formatDate(row.StartDate),
			EndDate:    formatDate(row.EndDate),
		}

		if row.Gpa != nil {
			entry.GPA = new(row.Gpa.InexactFloat64())
		}

		list = append(list, entry)
	}

	return list, nil
}

// educationRow validates req into the row AddEducation and UpdateEducation write.
func educationRow(profileID int64, req *AddEducationRequest) (model.ProfilesEducation, error) {
	if req.SchoolName == "" || req.Major == "" || req.Degree == "" {
		return model.ProfilesEducation{}, invalidInput("school_name, major, and degree are required")
	}

	startDate, err := parseOptionalDate(req.StartDate)

	if err != nil {
		return model.ProfilesEducation{}, invalidInput("invalid start_date: %s", err)
	}

	endDate, err := parseOptionalDate(req.EndDate)

	if err != nil {
		return model.ProfilesEducation{}, invalidInput("invalid end_date: %s", err)
	}

	row := model.ProfilesEducation{
		ProfileID:  new(int32(profileID)),
		SchoolName: req.SchoolName,
		Major:      req.Major,
		Degree:     req.Degree,
		StartDate:  startDate,
		EndDate:    endDate,
	}

	if req.GPA != nil {
		row.Gpa = new(decimal.NewFromFloat(*req.GPA))
	}

	return row, nil
}

func AddEducation(ctx context.Context, profileID int64, req *AddEducationRequest) (int64, error) {
	row, err := educationRow(profileID, req)

	if err != nil {
		return 0, err
	}

	education := table.ProfilesEducation
	key := pg.ColumnList{education.ProfileID, education.SchoolName, education.Major, education.Degree}
	stmt := education.INSERT(education.MutableColumns).MODEL(row).
		ON_CONFLICT(key...).DO_UPDATE(pg.SET(setExcluded(education.MutableColumns, education.EXCLUDED.AllColumns, key...))).
		RETURNING(education.ID)

	if err := queryOne(ctx, db(), stmt, &row); err != nil {
		return 0, err
	}

	return int64(row.ID), nil
}

func UpdateEducation(ctx context.Context, profileID, educationID int64, req *AddEducationRequest) error {
	row, err := educationRow(profileID, req)

	if err != nil {
		return err
	}

	education := table.ProfilesEducation

	return execOne(ctx, education.UPDATE(education.MutableColumns.Except(education.ProfileID)).MODEL(row).
		WHERE(education.ID.EQ(pg.Int(educationID)).AND(education.ProfileID.EQ(pg.Int(profileID)))))
}

func DeleteEducation(ctx context.Context, profileID, educationID int64) error {
	education := table.ProfilesEducation

	return execOne(ctx, education.DELETE().
		WHERE(education.ID.EQ(pg.Int(educationID)).AND(education.ProfileID.EQ(pg.Int(profileID)))))
}
