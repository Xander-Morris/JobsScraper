package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCreateProfile(t *testing.T) {
	creds := map[string]string{
		"email":    "test@example.com",
		"password": "securepassword123",
	}
	bodyBytes, _ := json.Marshal(creds)

	r := httptest.NewRequest("POST", "/api/profile/create", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()

	handleCreateProfile(rec, r)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}
