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

const (
	RefreshTokenLifetime = 30 * 24 * time.Hour
	// Caps idle time for sessions without "remember me"; their cookie also dies with the browser.
	SessionRefreshTokenLifetime = 24 * time.Hour
)

func RefreshTokenExpiry(persistent bool) time.Time {
	if persistent {
		return time.Now().Add(RefreshTokenLifetime)
	}
	return time.Now().Add(SessionRefreshTokenLifetime)
}

type RefreshSession struct {
	ProfileID  int64
	Persistent bool
	ExpiresAt  time.Time
}

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

func StoreRefreshToken(ctx context.Context, profileID int64, token string, expiresAt time.Time, persistent bool) error {
	_, err := db().ExecContext(ctx, `INSERT INTO profile_refresh_tokens (token_hash, profile_id, expires_at, persistent)
		VALUES ($1, $2, $3, $4)`, refreshTokenHash(token), profileID, expiresAt, persistent)
	return err
}

// RotateRefreshToken consumes a valid token and replaces it with a new one of the same persistence.
func RotateRefreshToken(ctx context.Context, oldToken, newToken string) (RefreshSession, error) {
	tx, err := db().BeginTx(ctx, nil)
	if err != nil {
		return RefreshSession{}, err
	}
	defer tx.Rollback()

	var session RefreshSession
	err = tx.QueryRowContext(ctx, `DELETE FROM profile_refresh_tokens
		WHERE token_hash = $1 AND expires_at > NOW()
		RETURNING profile_id, persistent`, refreshTokenHash(oldToken)).Scan(&session.ProfileID, &session.Persistent)
	if err != nil {
		if err == sql.ErrNoRows {
			return RefreshSession{}, sql.ErrNoRows
		}
		return RefreshSession{}, err
	}

	session.ExpiresAt = RefreshTokenExpiry(session.Persistent)
	if _, err := tx.ExecContext(ctx, `INSERT INTO profile_refresh_tokens (token_hash, profile_id, expires_at, persistent)
		VALUES ($1, $2, $3, $4)`, refreshTokenHash(newToken), session.ProfileID, session.ExpiresAt, session.Persistent); err != nil {
		return RefreshSession{}, err
	}

	if err := tx.Commit(); err != nil {
		return RefreshSession{}, err
	}

	return session, nil
}

func DeleteRefreshToken(ctx context.Context, token string) error {
	_, err := db().ExecContext(ctx, "DELETE FROM profile_refresh_tokens WHERE token_hash = $1", refreshTokenHash(token))
	return err
}
