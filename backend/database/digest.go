package database

import (
	"context"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/model"
	"main/internal/gen/table"
)

// DigestProfile is a profile opted into the job-match email digest.
type DigestProfile struct {
	ProfileID        int64
	Email            string
	LastDigestSentAt *time.Time
}

func ListProfilesForDigest(ctx context.Context) ([]DigestProfile, error) {
	var rows []model.Profiles

	profiles := table.Profiles
	stmt := pg.SELECT(profiles.ID, profiles.Email, profiles.LastDigestSentAt).
		FROM(profiles).
		WHERE(profiles.EmailNotificationsEnabled)

	if err := stmt.QueryContext(ctx, db(), &rows); err != nil {
		return nil, err
	}

	var digestProfiles []DigestProfile

	for _, row := range rows {
		digestProfiles = append(digestProfiles, DigestProfile{
			ProfileID:        int64(row.ID),
			Email:            row.Email,
			LastDigestSentAt: row.LastDigestSentAt,
		})
	}

	return digestProfiles, nil
}

func MarkDigestSent(ctx context.Context, profileID int64, sentAt time.Time) error {
	profiles := table.Profiles
	_, err := exec(ctx, db(), profiles.UPDATE().
		SET(profiles.LastDigestSentAt.SET(pg.TimestampzT(sentAt))).
		WHERE(profiles.ID.EQ(pg.Int(profileID))))

	return err
}

// DisableEmailDigest turns off the job-match digest for a profile. Used by the
// one-click unsubscribe link in digest emails.
func DisableEmailDigest(ctx context.Context, profileID int64) error {
	profiles := table.Profiles
	_, err := exec(ctx, db(), profiles.UPDATE().
		SET(profiles.EmailNotificationsEnabled.SET(pg.Bool(false))).
		WHERE(profiles.ID.EQ(pg.Int(profileID))))

	return err
}
