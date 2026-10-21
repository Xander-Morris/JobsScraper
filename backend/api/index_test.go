package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func stubEnsure(t *testing.T, err error) {
	t.Helper()
	prev := ensure
	ensure = func(...string) error { return err }
	t.Cleanup(func() { ensure = prev })
}

func TestHandlerMisconfigured(t *testing.T) {
	stubEnsure(t, errors.New("missing SECRET_KEY"))
	rec := httptest.NewRecorder()

	Handler(rec, httptest.NewRequest("GET", "/api/health", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerServesTheAPI(t *testing.T) {
	stubEnsure(t, nil)
	rec := httptest.NewRecorder()

	Handler(rec, httptest.NewRequest("GET", "/api/no-such-route", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 from the API's router", rec.Code)
	}
}
