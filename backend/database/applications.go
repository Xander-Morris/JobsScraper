package database

import (
	"context"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/model"
	"main/internal/gen/table"
)

func MarkJobApplied(ctx context.Context, profileID, jobID int64) error {
	var row model.ProfileJobApplications

	applications := table.ProfileJobApplications
	stmt := applications.INSERT(applications.ProfileID, applications.JobID).
		VALUES(pg.Int(profileID), pg.Int(jobID)).
		ON_CONFLICT(applications.ProfileID, applications.JobID).DO_UPDATE(pg.SET(applications.AppliedAt.SET(pg.NOW()))).
		RETURNING(applications.ID)

	return queryOne(ctx, db(), stmt, &row)
}

func UnmarkJobApplied(ctx context.Context, profileID, jobID int64) error {
	applications := table.ProfileJobApplications
	_, err := exec(ctx, db(), applications.DELETE().
		WHERE(applications.ProfileID.EQ(pg.Int(profileID)).AND(applications.JobID.EQ(pg.Int(jobID)))))

	return err
}
