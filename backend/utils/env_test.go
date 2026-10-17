package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sameFile(t *testing.T, a, b string) bool {
	t.Helper()

	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}

	return os.SameFile(ai, bi)
}

func TestFindEnvFileWalksUp(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")

	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Chdir(nested)

	if got := findEnvFile(); !sameFile(t, got, envPath) {
		t.Errorf("findEnvFile() = %q, want %q", got, envPath)
	}
}

func TestFindEnvFilePrefersNearest(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")

	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{root, child} {
		if err := os.WriteFile(filepath.Join(dir, ".env"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(child)

	if got := findEnvFile(); !sameFile(t, got, filepath.Join(child, ".env")) {
		t.Errorf("findEnvFile() = %q, want the child .env", got)
	}
}

func TestGetenvReadsProcessEnv(t *testing.T) {
	t.Setenv("UTILS_TEST_GETENV", "value")

	if got := Getenv("UTILS_TEST_GETENV"); got != "value" {
		t.Errorf("Getenv() = %q, want %q", got, "value")
	}
}

func TestRequireEnv(t *testing.T) {
	t.Setenv("UTILS_TEST_SET", "x")
	t.Setenv("UTILS_TEST_EMPTY", "")

	if err := RequireEnv("UTILS_TEST_SET"); err != nil {
		t.Errorf("RequireEnv(set) = %v, want nil", err)
	}

	if err := RequireEnv(); err != nil {
		t.Errorf("RequireEnv() = %v, want nil", err)
	}

	err := RequireEnv("UTILS_TEST_SET", "UTILS_TEST_EMPTY", "UTILS_TEST_MISSING_XYZ")
	if err == nil {
		t.Fatal("RequireEnv() = nil, want error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "UTILS_TEST_EMPTY, UTILS_TEST_MISSING_XYZ") || strings.Contains(msg, "UTILS_TEST_SET,") {
		t.Errorf("error %q should name exactly the missing keys", msg)
	}
}
