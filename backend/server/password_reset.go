package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"main/database"
	"main/notify"
)

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func handleRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	req := &passwordResetRequest{}

	if err := decodeJSON(w, r, req); err != nil || req.Email == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := sendPasswordResetEmail(r.Context(), req.Email); err != nil {
		slog.Error("request password reset", "error", err)
	}

	// Same response either way, so this can't be used to probe for accounts.
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

func sendPasswordResetEmail(ctx context.Context, email string) error {
	profileID, _, err := database.GetProfileByEmail(email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get profile by email: %w", err)
	}

	token, err := database.CreatePasswordResetToken(ctx, profileID)
	if err != nil {
		return fmt.Errorf("create reset token: %w", err)
	}

	if err := notify.SendEmail(ctx, email, "Reset your Jobs Scraper password", renderPasswordResetEmail(token)); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func renderPasswordResetEmail(token string) string {
	return fmt.Sprintf(
		"Someone asked to reset the password for your Jobs Scraper account.\n\nSet a new one here (link expires in %d minutes):\n%s\n\nIf that wasn't you, ignore this email and your password stays the same.\n",
		int(database.PasswordResetTokenLifetime.Minutes()),
		passwordResetLink(token),
	)
}

func passwordResetLink(token string) string {
	return fmt.Sprintf("%s/reset-password?token=%s", strings.TrimSuffix(allowedOrigin(), "/"), url.QueryEscape(token))
}

func handleConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	req := &passwordResetConfirmRequest{}

	if err := decodeJSON(w, r, req); err != nil || req.Token == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	profileID, err := database.ResetPassword(r.Context(), req.Token, req.Password)
	if err != nil {
		if errors.Is(err, database.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "reset link is invalid or expired")
			return
		}

		slog.Error("confirm password reset", "error", err)
		writeError(w, http.StatusInternalServerError, "could not reset password")
		return
	}

	token, err := createSession(w, profileID)
	if err != nil {
		slog.Error("confirm password reset: create session", "error", err)
		writeError(w, http.StatusInternalServerError, "could not reset password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
