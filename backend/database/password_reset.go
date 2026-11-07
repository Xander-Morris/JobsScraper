package database

import (
	"context"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/model"
	"main/internal/gen/table"
)

const PasswordResetTokenLifetime = time.Hour

// CreatePasswordResetToken replaces the profile's outstanding reset token and sweeps expired ones.
func CreatePasswordResetToken(ctx context.Context, profileID int64) (string, error) {
	token := NewRefreshToken()

	tx, err := db().BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	resets := table.ProfilePasswordResetTokens

	if _, err := exec(ctx, tx, resets.DELETE().
		WHERE(resets.ProfileID.EQ(pg.Int(profileID)).OR(resets.ExpiresAt.LT_EQ(pg.NOW())))); err != nil {
		return "", err
	}

	if _, err := exec(ctx, tx, resets.INSERT(resets.TokenHash, resets.ProfileID, resets.ExpiresAt).MODEL(model.ProfilePasswordResetTokens{
		TokenHash: refreshTokenHash(token),
		ProfileID: int32(profileID),
		ExpiresAt: time.Now().Add(PasswordResetTokenLifetime),
	})); err != nil {
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
	if err := validatePassword(password); err != nil {
		return 0, err
	}

	// validatePassword kept it within bcrypt's limit, so hashing can't fail.
	hash, _ := HashPassword(password)

	tx, err := db().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var consumed model.ProfilePasswordResetTokens

	resets, profiles, sessions := table.ProfilePasswordResetTokens, table.Profiles, table.ProfileRefreshTokens

	if err := queryOne(ctx, tx, resets.DELETE().
		WHERE(resets.TokenHash.EQ(pg.String(refreshTokenHash(token))).AND(resets.ExpiresAt.GT(pg.NOW()))).
		RETURNING(resets.ProfileID), &consumed); err != nil {
		return 0, err
	}

	profileID := pg.Int(int64(consumed.ProfileID))

	if _, err := exec(ctx, tx, profiles.UPDATE().SET(profiles.Password.SET(pg.String(hash))).WHERE(profiles.ID.EQ(profileID))); err != nil {
		return 0, err
	}

	if _, err := exec(ctx, tx, sessions.DELETE().WHERE(sessions.ProfileID.EQ(profileID))); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return int64(consumed.ProfileID), nil
}
