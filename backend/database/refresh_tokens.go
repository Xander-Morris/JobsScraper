package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

func NewRefreshToken() string {
	value := make([]byte, 32)
	// crypto/rand.Read never returns an error; it crashes the program instead.
	_, _ = rand.Read(value)

	return base64.RawURLEncoding.EncodeToString(value)
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

// RotateRefreshToken consumes a valid token and replaces it with a new one of the same persistence,
// in one atomic statement. sql.ErrNoRows means the token was unknown, expired, used, or its profile deleted.
func RotateRefreshToken(ctx context.Context, oldToken, newToken string) (RefreshSession, error) {
	var session RefreshSession

	err := db().QueryRowContext(ctx, `WITH old AS (
			DELETE FROM profile_refresh_tokens WHERE token_hash = $1 AND expires_at > NOW()
			RETURNING profile_id, persistent)
		INSERT INTO profile_refresh_tokens (token_hash, profile_id, expires_at, persistent)
		SELECT $2, profile_id, CASE WHEN persistent THEN $3::timestamptz ELSE $4::timestamptz END, persistent FROM old
		RETURNING profile_id, persistent, expires_at`,
		refreshTokenHash(oldToken), refreshTokenHash(newToken), RefreshTokenExpiry(true), RefreshTokenExpiry(false),
	).Scan(&session.ProfileID, &session.Persistent, &session.ExpiresAt)
	if err != nil {
		return RefreshSession{}, err
	}

	return session, nil
}

func DeleteRefreshToken(ctx context.Context, token string) error {
	_, err := db().ExecContext(ctx, "DELETE FROM profile_refresh_tokens WHERE token_hash = $1", refreshTokenHash(token))
	return err
}
