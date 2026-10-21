package coldstart

import (
	"sync"
	"testing"

	"main/database"
	"main/utils"
)

// fresh resets the once-per-process state and the shared pool around a test.
func fresh(t *testing.T) {
	t.Helper()
	once, ensureErr = sync.Once{}, nil
	prev := database.SetDB(nil)
	t.Cleanup(func() {
		database.CloseDb()
		database.SetDB(prev)
		once, ensureErr = sync.Once{}, nil
	})
}

func TestEnsureMissingEnv(t *testing.T) {
	fresh(t)
	t.Setenv("COLDSTART_TEST_REQUIRED", "")

	if err := Ensure("COLDSTART_TEST_REQUIRED"); err == nil {
		t.Fatal("want error for a missing variable")
	}
	// Later calls report the same failure without retrying.
	t.Setenv("COLDSTART_TEST_REQUIRED", "set")
	if err := Ensure("COLDSTART_TEST_REQUIRED"); err == nil {
		t.Error("second call should return the cached error")
	}
}

func TestEnsureUnreachableDatabase(t *testing.T) {
	fresh(t)
	t.Setenv("DATABASE_CONNECTION", "postgres://postgres@127.0.0.1:1/none?connect_timeout=1")

	if err := Ensure("DATABASE_CONNECTION"); err == nil {
		t.Fatal("want error for an unreachable database")
	}
}

func TestEnsureConnects(t *testing.T) {
	conn := utils.Getenv("TEST_DATABASE_CONNECTION")
	if conn == "" {
		t.Skip("TEST_DATABASE_CONNECTION not set")
	}
	fresh(t)
	t.Setenv("DATABASE_CONNECTION", conn)

	if err := Ensure("DATABASE_CONNECTION"); err != nil {
		t.Fatal(err)
	}
	if err := Ensure("DATABASE_CONNECTION"); err != nil {
		t.Fatal(err)
	}
}
