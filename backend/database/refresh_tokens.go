package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"
)

const RefreshTokenLifetime = 30 * 24 * time.Hour

func NewRefreshToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func refreshTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func StoreRefreshToken(ctx context.Context, profileID int64, token string, expiresAt time.Time) error {
	db, err := GetDb()
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `INSERT INTO profile_refresh_tokens (token_hash, profile_id, expires_at)
		VALUES ($1, $2, $3)`, refreshTokenHash(token), profileID, expiresAt)
	return err
}

// RotateRefreshToken consumes a valid token and replaces it with a new one.
func RotateRefreshToken(ctx context.Context, oldToken, newToken string, expiresAt time.Time) (int64, error) {
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
	err = tx.QueryRowContext(ctx, `DELETE FROM profile_refresh_tokens
		WHERE token_hash = $1 AND expires_at > NOW()
		RETURNING profile_id`, refreshTokenHash(oldToken)).Scan(&profileID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, sql.ErrNoRows
		}
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO profile_refresh_tokens (token_hash, profile_id, expires_at)
		VALUES ($1, $2, $3)`, refreshTokenHash(newToken), profileID, expiresAt); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return profileID, nil
}

func DeleteRefreshToken(ctx context.Context, token string) error {
	db, err := GetDb()
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "DELETE FROM profile_refresh_tokens WHERE token_hash = $1", refreshTokenHash(token))
	return err
}
