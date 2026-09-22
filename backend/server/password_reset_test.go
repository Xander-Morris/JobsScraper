package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"main/database"
)

func postConfirmPasswordReset(token, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"token": token, "password": password})
	r := httptest.NewRequest("POST", "/api/profile/password-reset/confirm", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleConfirmPasswordReset(rec, r)

	return rec
}

func TestPasswordResetFlow(t *testing.T) {
	requireTestDB(t)
	ctx := context.Background()

	const email = "reset-flow@example.com"
	profileID, err := database.CreateProfile(&database.ProfileRequest{Email: email, Password: "oldpassword123"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	oldSession, err := database.NewRefreshToken()
	if err != nil {
		t.Fatalf("new refresh token: %v", err)
	}
	if err := database.StoreRefreshToken(ctx, profileID, oldSession, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("store refresh token: %v", err)
	}

	token, err := database.CreatePasswordResetToken(ctx, profileID)
	if err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	if rec := postConfirmPasswordReset(token, "short"); rec.Code != http.StatusBadRequest {
		t.Fatalf("short password status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec := postConfirmPasswordReset(token, "newpassword123")
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}

	_, hash, err := database.GetProfileByEmail(email)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if !database.CheckPasswordHash("newpassword123", hash) {
		t.Errorf("password was not updated")
	}

	if _, err := database.RotateRefreshToken(ctx, oldSession, "unused", time.Now().Add(time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("existing session survived password reset: err = %v", err)
	}

	if rec := postConfirmPasswordReset(token, "anotherpassword123"); rec.Code != http.StatusBadRequest {
		t.Errorf("reused token status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPasswordResetRejectsExpiredToken(t *testing.T) {
	requireTestDB(t)

	profileID, err := database.CreateProfile(&database.ProfileRequest{Email: "reset-expired@example.com", Password: "oldpassword123"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	token, err := database.CreatePasswordResetToken(context.Background(), profileID)
	if err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	if _, err := database.DB().Exec("UPDATE profile_password_reset_tokens SET expires_at = NOW() - INTERVAL '1 minute' WHERE profile_id = $1", profileID); err != nil {
		t.Fatalf("expire token: %v", err)
	}

	if rec := postConfirmPasswordReset(token, "newpassword123"); rec.Code != http.StatusBadRequest {
		t.Errorf("expired token status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestPasswordResetUnknownEmail(t *testing.T) {
	requireTestDB(t)

	body, _ := json.Marshal(map[string]string{"email": "nobody@example.com"})
	r := httptest.NewRequest("POST", "/api/profile/password-reset", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handleRequestPasswordReset(rec, r)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}
