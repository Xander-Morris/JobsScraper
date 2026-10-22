package database

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}

	if hash == "correct horse" {
		t.Fatal("HashPassword returned the plaintext")
	}

	if !CheckPasswordHash("correct horse", hash) {
		t.Error("CheckPasswordHash rejected the right password")
	}

	if CheckPasswordHash("wrong horse", hash) {
		t.Error("CheckPasswordHash accepted the wrong password")
	}

	if CheckPasswordHash("correct horse", "not-a-hash") {
		t.Error("CheckPasswordHash accepted a malformed hash")
	}
}

func TestRefreshTokenExpiry(t *testing.T) {
	now := time.Now()

	persistent := RefreshTokenExpiry(true).Sub(now)
	session := RefreshTokenExpiry(false).Sub(now)

	if persistent < RefreshTokenLifetime-time.Minute || persistent > RefreshTokenLifetime+time.Minute {
		t.Errorf("persistent expiry in %v, want ~%v", persistent, RefreshTokenLifetime)
	}

	if session < SessionRefreshTokenLifetime-time.Minute || session > SessionRefreshTokenLifetime+time.Minute {
		t.Errorf("session expiry in %v, want ~%v", session, SessionRefreshTokenLifetime)
	}
}

func TestNewRefreshTokenUnique(t *testing.T) {
	a := NewRefreshToken()

	b := NewRefreshToken()

	if a == b || len(a) < 40 {
		t.Errorf("tokens %q / %q should be long and distinct", a, b)
	}

	if refreshTokenHash(a) == a || refreshTokenHash(a) != refreshTokenHash(a) {
		t.Error("refreshTokenHash should be deterministic and differ from the token")
	}
}

func TestCreateProfileValidation(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()

	tests := []ProfileRequest{
		{Email: "", Password: "securepassword123"},
		{Email: "a@example.com", Password: ""},
		{Email: "a@example.com", Password: "short"},
		{Email: "not-an-email", Password: "securepassword123"},
		{Email: "nobody@nomx.invalid", Password: "securepassword123"},
	}

	for _, req := range tests {
		if _, err := CreateProfile(ctx, &req); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("CreateProfile(%q, %q) err = %v, want ErrInvalidInput", req.Email, req.Password, err)
		}
	}
}

func TestCreateProfileDuplicateEmail(t *testing.T) {
	newTestDB(t)
	newTestProfile(t)

	if _, err := CreateProfile(t.Context(), &ProfileRequest{Email: "test@example.com", Password: "securepassword123"}); err == nil {
		t.Fatal("duplicate email accepted")
	}
}

func TestGetProfileByEmailAndExists(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	gotID, hash, err := GetProfileByEmail(ctx, "test@example.com")
	if err != nil || gotID != id {
		t.Fatalf("GetProfileByEmail = %d, %v; want %d", gotID, err, id)
	}

	if !CheckPasswordHash("securepassword123", hash) {
		t.Error("stored hash doesn't match the password")
	}

	if _, _, err := GetProfileByEmail(ctx, "missing@example.com"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetProfileByEmail(missing) = %v, want sql.ErrNoRows", err)
	}
}

func TestRefreshTokenRotation(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	for _, persistent := range []bool{true, false} {
		old := NewRefreshToken()
		if err := StoreRefreshToken(ctx, id, old, RefreshTokenExpiry(persistent), persistent); err != nil {
			t.Fatalf("StoreRefreshToken: %v", err)
		}

		next := NewRefreshToken()
		session, err := RotateRefreshToken(ctx, old, next)
		if err != nil {
			t.Fatalf("RotateRefreshToken: %v", err)
		}

		if session.ProfileID != id || session.Persistent != persistent || time.Until(session.ExpiresAt) <= 0 {
			t.Errorf("session = %+v, want profile %d persistent=%v", session, id, persistent)
		}

		// The old token is single-use.
		if _, err := RotateRefreshToken(ctx, old, "another"); !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("reusing rotated token err = %v, want sql.ErrNoRows", err)
		}

		// The new one works once.
		final := NewRefreshToken()
		if _, err := RotateRefreshToken(ctx, next, final); err != nil {
			t.Errorf("rotating new token: %v", err)
		}
	}
}

func TestRotateExpiredRefreshToken(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	token := NewRefreshToken()
	if err := StoreRefreshToken(ctx, id, token, time.Now().Add(-time.Minute), true); err != nil {
		t.Fatal(err)
	}

	if _, err := RotateRefreshToken(ctx, token, "next"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expired token err = %v, want sql.ErrNoRows", err)
	}

	if _, err := RotateRefreshToken(ctx, "never-stored", "next"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("unknown token err = %v, want sql.ErrNoRows", err)
	}
}

func TestDeleteRefreshToken(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	token := NewRefreshToken()
	if err := StoreRefreshToken(ctx, id, token, RefreshTokenExpiry(true), true); err != nil {
		t.Fatal(err)
	}

	if err := DeleteRefreshToken(ctx, token); err != nil {
		t.Fatal(err)
	}

	if _, err := RotateRefreshToken(ctx, token, "next"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("deleted token still rotates: %v", err)
	}

	if err := DeleteRefreshToken(ctx, "unknown"); err != nil {
		t.Errorf("deleting unknown token = %v, want nil", err)
	}
}

func TestResetPassword(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	session := NewRefreshToken()
	if err := StoreRefreshToken(ctx, id, session, RefreshTokenExpiry(true), true); err != nil {
		t.Fatal(err)
	}

	stale, err := CreatePasswordResetToken(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	token, err := CreatePasswordResetToken(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ResetPassword(ctx, token, "short"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("short password err = %v, want ErrInvalidInput", err)
	}

	if _, err := ResetPassword(ctx, stale, "newpassword123"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("superseded token err = %v, want sql.ErrNoRows", err)
	}

	gotID, err := ResetPassword(ctx, token, "newpassword123")
	if err != nil || gotID != id {
		t.Fatalf("ResetPassword = %d, %v", gotID, err)
	}

	_, hash, _ := GetProfileByEmail(ctx, "test@example.com")
	if !CheckPasswordHash("newpassword123", hash) {
		t.Error("password not updated")
	}

	if _, err := ResetPassword(ctx, token, "another123456"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("reused reset token err = %v, want sql.ErrNoRows", err)
	}

	// Every session is signed out.
	if _, err := RotateRefreshToken(ctx, session, "next"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("old session survived reset: %v", err)
	}
}
