package database

import (
	"context"
	"fmt"
	"time"
)

const PasswordResetTokenLifetime = time.Hour

// CreatePasswordResetToken replaces the profile's outstanding reset token and sweeps expired ones.
func CreatePasswordResetToken(ctx context.Context, profileID int64) (string, error) {
	token, err := NewRefreshToken()
	if err != nil {
		return "", err
	}

	db, err := GetDb()
	if err != nil {
		return "", err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM profile_password_reset_tokens
		WHERE profile_id = $1 OR expires_at <= NOW()`, profileID); err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO profile_password_reset_tokens (token_hash, profile_id, expires_at)
		VALUES ($1, $2, $3)`, refreshTokenHash(token), profileID, time.Now().Add(PasswordResetTokenLifetime)); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return token, nil
}

// ResetPassword consumes a valid reset token, sets the new password, and signs out every session.
// Returns sql.ErrNoRows if the token is unknown, used, or expired.
func ResetPassword(ctx context.Context, token, password string) (int64, error) {
	if len(password) < 8 {
		return 0, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidProfile)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return 0, err
	}

	db, err := GetDb()
	if err != nil {
		return 0, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var profileID int64
	if err := tx.QueryRowContext(ctx, `DELETE FROM profile_password_reset_tokens
		WHERE token_hash = $1 AND expires_at > NOW()
		RETURNING profile_id`, refreshTokenHash(token)).Scan(&profileID); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE profiles SET password = $1 WHERE id = $2", hash, profileID); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM profile_refresh_tokens WHERE profile_id = $1", profileID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return profileID, nil
}
