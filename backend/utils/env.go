package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

func findEnvFile() string {
	dir, err := os.Getwd()

	if err != nil {
		return ""
	}

	for {
		candidate := filepath.Join(dir, ".env")

		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}

		dir = parent
	}
}

// loadDotEnv merges the nearest .env into the process env once; real env vars win.
var loadDotEnv = sync.OnceFunc(func() {
	if path := findEnvFile(); path != "" {
		_ = godotenv.Load(path)
	}
})

// Getenv is os.Getenv with the nearest .env loaded first.
func Getenv(key string) string {
	loadDotEnv()

	return os.Getenv(key)
}

// RequireEnv checks every given key is set to a non-empty value, and returns
// an error naming all the missing ones at once. Call it once at startup so
// misconfiguration fails loud instead of surfacing later as a mystery empty
// JWT secret or nil DB connection string.
func RequireEnv(keys ...string) error {
	var missing []string

	for _, key := range keys {
		if Getenv(key) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}
