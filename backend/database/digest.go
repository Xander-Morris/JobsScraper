package database

import (
	"context"
	"time"
)

// DigestProfile is a profile opted into the job-match email digest.
type DigestProfile struct {
	ProfileID        int64
	Email            string
	LastDigestSentAt *time.Time
}

func ListProfilesForDigest(ctx context.Context) ([]DigestProfile, error) {
	db, err := GetDb()

	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `SELECT id, email, last_digest_sent_at FROM profiles WHERE email_notifications_enabled`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var profiles []DigestProfile

	for rows.Next() {
		var p DigestProfile
		var lastSent *time.Time

		if err := rows.Scan(&p.ProfileID, &p.Email, &lastSent); err != nil {
			return nil, err
		}

		p.LastDigestSentAt = lastSent
		profiles = append(profiles, p)
	}

	return profiles, rows.Err()
}

func MarkDigestSent(ctx context.Context, profileID int64, sentAt time.Time) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `UPDATE profiles SET last_digest_sent_at = $1 WHERE id = $2`, sentAt, profileID)

	return err
}
