package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/model"
	"main/internal/gen/table"
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

// refreshTokenColumns are what a new token row is written with; created_at takes its default.
func refreshTokenColumns() pg.ColumnList {
	tokens := table.ProfileRefreshTokens

	return pg.ColumnList{tokens.TokenHash, tokens.ProfileID, tokens.ExpiresAt, tokens.Persistent}
}

func StoreRefreshToken(ctx context.Context, profileID int64, token string, expiresAt time.Time, persistent bool) error {
	_, err := exec(ctx, db(), table.ProfileRefreshTokens.INSERT(refreshTokenColumns()).MODEL(model.ProfileRefreshTokens{
		TokenHash:  refreshTokenHash(token),
		ProfileID:  int32(profileID),
		ExpiresAt:  expiresAt,
		Persistent: persistent,
	}))
	return err
}

// RotateRefreshToken consumes a valid token and replaces it with a new one of the same persistence,
// in one atomic statement. sql.ErrNoRows means the token was unknown, expired, used, or its profile deleted.
func RotateRefreshToken(ctx context.Context, oldToken, newToken string) (RefreshSession, error) {
	var row model.ProfileRefreshTokens

	tokens := table.ProfileRefreshTokens
	old := pg.CTE("old")
	oldPersistent := tokens.Persistent.From(old)

	stmt := pg.WITH(old.AS(tokens.DELETE().
		WHERE(tokens.TokenHash.EQ(pg.String(refreshTokenHash(oldToken))).AND(tokens.ExpiresAt.GT(pg.NOW()))).
		RETURNING(tokens.ProfileID, tokens.Persistent)))(
		tokens.INSERT(refreshTokenColumns()).
			QUERY(pg.SELECT(
				pg.String(refreshTokenHash(newToken)),
				tokens.ProfileID.From(old),
				pg.CASE().WHEN(oldPersistent).THEN(pg.TimestampzT(RefreshTokenExpiry(true))).ELSE(pg.TimestampzT(RefreshTokenExpiry(false))),
				oldPersistent,
			).FROM(old)).
			RETURNING(tokens.ProfileID, tokens.Persistent, tokens.ExpiresAt),
	)

	if err := queryOne(ctx, db(), stmt, &row); err != nil {
		return RefreshSession{}, err
	}

	return RefreshSession{ProfileID: int64(row.ProfileID), Persistent: row.Persistent, ExpiresAt: row.ExpiresAt}, nil
}

func DeleteRefreshToken(ctx context.Context, token string) error {
	tokens := table.ProfileRefreshTokens
	_, err := exec(ctx, db(), tokens.DELETE().WHERE(tokens.TokenHash.EQ(pg.String(refreshTokenHash(token)))))
	return err
}
