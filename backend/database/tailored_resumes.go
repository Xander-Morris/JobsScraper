package database

import (
	"context"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/model"
	"main/internal/gen/table"
	"main/llm"
)

type TailoredResume struct {
	JobID     int64              `json:"job_id"`
	Content   llm.TailoredResume `json:"content"`
	Stale     bool               `json:"stale"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// ownedTailoredResume matches profileID's tailored resume for jobID.
func ownedTailoredResume(profileID, jobID int64) pg.BoolExpression {
	tailored := table.ProfileTailoredResumes

	return tailored.ProfileID.EQ(pg.Int(profileID)).AND(tailored.JobID.EQ(pg.Int(jobID)))
}

// GetTailoredResume marks it stale if the source resume was deleted, deactivated, or re-extracted.
func GetTailoredResume(ctx context.Context, profileID, jobID int64) (TailoredResume, error) {
	var row struct {
		model.ProfileTailoredResumes
		Stale bool
	}

	tailored, resumes, extractions := table.ProfileTailoredResumes, table.ProfileResumes, table.ProfileResumeExtractions
	stale := resumes.ID.IS_NULL().
		OR(pg.NOT(resumes.IsActive)).
		OR(extractions.UpdatedAt.IS_NULL()).
		OR(extractions.UpdatedAt.GT(tailored.SourceUpdatedAt))

	stmt := pg.SELECT(tailored.JobID, tailored.Content, tailored.UpdatedAt, stale.AS("stale")).
		FROM(tailored.
			LEFT_JOIN(resumes, resumes.ID.EQ(tailored.ResumeID)).
			LEFT_JOIN(extractions, extractions.ResumeID.EQ(tailored.ResumeID))).
		WHERE(ownedTailoredResume(profileID, jobID))

	if err := queryOne(ctx, db(), stmt, &row); err != nil {
		return TailoredResume{}, err
	}

	result := TailoredResume{JobID: int64(row.JobID), Stale: row.Stale, UpdatedAt: row.UpdatedAt}

	if err := decodeJSON(&row.Content, &result.Content, "tailored resume"); err != nil {
		return TailoredResume{}, err
	}

	return result, nil
}

func UpsertTailoredResume(ctx context.Context, profileID, jobID, resumeID int64, sourceUpdatedAt time.Time, content llm.TailoredResume) error {
	tailored := table.ProfileTailoredResumes
	key := pg.ColumnList{tailored.ProfileID, tailored.JobID}
	cols := pg.ColumnList{tailored.ProfileID, tailored.JobID, tailored.ResumeID, tailored.SourceUpdatedAt, tailored.Content}

	// Ids go in as int8 params, so an out-of-range one errors instead of wrapping.
	_, err := exec(ctx, db(), tailored.INSERT(cols).
		VALUES(pg.Int(profileID), pg.Int(jobID), pg.Int(resumeID), pg.TimestampzT(sourceUpdatedAt), arg(encodeJSON(content), "jsonb")).
		ON_CONFLICT(key...).DO_UPDATE(pg.SET(
		setExcluded(cols, tailored.EXCLUDED.AllColumns, key...),
		tailored.UpdatedAt.SET(pg.NOW()),
	)))

	return err
}

func UpdateTailoredResumeContent(ctx context.Context, profileID, jobID int64, content llm.TailoredResume) error {
	tailored := table.ProfileTailoredResumes

	return execOne(ctx, tailored.UPDATE().
		SET(tailored.Content.SET(pg.StringExp(arg(encodeJSON(content), "jsonb"))), tailored.UpdatedAt.SET(pg.NOW())).
		WHERE(ownedTailoredResume(profileID, jobID)))
}
