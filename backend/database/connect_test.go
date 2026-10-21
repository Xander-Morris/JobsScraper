package database

import (
	"errors"
	"strings"
	"testing"
	"time"

	emailverifier "github.com/AfterShip/email-verifier"
	"main/utils"
)

// swapPool clears the shared pool for a test and restores it after.
func swapPool(t *testing.T) {
	t.Helper()
	prev := SetDB(nil)
	t.Cleanup(func() {
		CloseDb()
		SetDB(prev)
	})
}

func TestConnect(t *testing.T) {
	conn := utils.Getenv("TEST_DATABASE_CONNECTION")
	if conn == "" {
		t.Skip("TEST_DATABASE_CONNECTION not set")
	}
	swapPool(t)
	t.Setenv("DATABASE_CONNECTION", conn)

	if err := Connect(t.Context()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Already connected is a no-op.
	if err := Connect(t.Context()); err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	if err := Ping(t.Context()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	CloseDb()
	CloseDb()
}

func TestConnectErrors(t *testing.T) {
	swapPool(t)

	t.Setenv("DATABASE_CONNECTION", "not a :// valid url")
	if err := Connect(t.Context()); err == nil {
		t.Error("Connect with unparseable string succeeded")
	}

	t.Setenv("DATABASE_CONNECTION", "postgres://postgres@127.0.0.1:1/none?connect_timeout=1")
	if err := Connect(t.Context()); err == nil {
		t.Error("Connect to closed port succeeded")
	}
}

func TestLazyPoolReportsBadConnectionString(t *testing.T) {
	swapPool(t)
	t.Setenv("DATABASE_CONNECTION", "not a :// valid url")

	if err := DB().PingContext(t.Context()); err == nil {
		t.Errorf("ping on bad pool = %v, want parse error", err)
	}
}

func TestLazyPoolOpensFromEnv(t *testing.T) {
	conn := utils.Getenv("TEST_DATABASE_CONNECTION")
	if conn == "" {
		t.Skip("TEST_DATABASE_CONNECTION not set")
	}
	swapPool(t)
	t.Setenv("DATABASE_CONNECTION", conn)

	if err := DB().PingContext(t.Context()); err != nil {
		t.Fatalf("ping lazy pool: %v", err)
	}
}

func TestVerifyEmail(t *testing.T) {
	prevVerify, prevTimeout := verifyAddress, emailVerificationTimeout
	t.Cleanup(func() { verifyAddress, emailVerificationTimeout = prevVerify, prevTimeout })

	want := &emailverifier.Result{Email: "a@b.co"}
	verifyAddress = func(string) (*emailverifier.Result, error) { return want, nil }
	if got, err := verifyEmail("a@b.co"); err != nil || got != want {
		t.Errorf("verifyEmail = %v, %v", got, err)
	}

	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	verifyAddress = func(string) (*emailverifier.Result, error) { <-block; return nil, nil }
	emailVerificationTimeout = time.Millisecond
	if _, err := verifyEmail("a@b.co"); !errors.Is(err, errEmailVerificationTimeout) {
		t.Errorf("verifyEmail err = %v, want timeout", err)
	}
}

func TestCreateProfileVerificationOutcomes(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()

	SetEmailVerifier(func(string) (*emailverifier.Result, error) { return nil, errEmailVerificationTimeout })
	if _, err := CreateProfile(ctx, &ProfileRequest{Email: "slow@example.com", Password: "securepassword123"}); err != nil {
		t.Errorf("timeout with valid syntax should still register: %v", err)
	}
	if _, err := CreateProfile(ctx, &ProfileRequest{Email: "not-an-email", Password: "securepassword123"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("timeout with bad syntax = %v, want invalid input", err)
	}

	SetEmailVerifier(func(string) (*emailverifier.Result, error) { return nil, errors.New("dns down") })
	if _, err := CreateProfile(ctx, &ProfileRequest{Email: "x@example.com", Password: "securepassword123"}); err == nil || errors.Is(err, ErrInvalidInput) {
		t.Errorf("verifier error = %v, want server error", err)
	}
}

func TestPasswordOverBcryptLimit(t *testing.T) {
	newTestDB(t)
	long := strings.Repeat("p", 80)

	if _, err := HashPassword(long); err == nil {
		t.Error("HashPassword over 72 bytes succeeded")
	}
	if _, err := CreateProfile(t.Context(), &ProfileRequest{Email: "long@example.com", Password: long}); err == nil {
		t.Error("CreateProfile with 80-byte password succeeded")
	}

	token, err := CreatePasswordResetToken(t.Context(), newTestProfile(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ResetPassword(t.Context(), token, long); err == nil {
		t.Error("ResetPassword with 80-byte password succeeded")
	}
}

func TestAddEducationRejectsBadDates(t *testing.T) {
	newTestDB(t)
	pid := newTestProfile(t)

	for _, req := range []AddEducationRequest{
		{SchoolName: "MIT", Major: "CS", Degree: "BS", StartDate: "soon"},
		{SchoolName: "MIT", Major: "CS", Degree: "BS", EndDate: "later"},
	} {
		if _, err := AddEducation(t.Context(), pid, &req); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("AddEducation(%+v) = %v, want invalid input", req, err)
		}
	}
}
