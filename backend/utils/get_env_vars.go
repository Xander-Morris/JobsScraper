package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type EnvPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var allEnvVariables map[string]string

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

func GetEnv() map[string]string {
	if allEnvVariables != nil {
		return allEnvVariables
	}

	if path := findEnvFile(); path != "" {
		_ = godotenv.Load(path)
	}
	rawEnv := os.Environ()
	envList := make(map[string]string)

	for _, env := range rawEnv {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) == 2 {
			envList[pair[0]] = pair[1]
		}
	}

	allEnvVariables = envList

	return allEnvVariables
}

// RequireEnv checks that every given key is set to a non-empty value, returning
// an error naming all missing keys at once. Meant to be called once at startup so
// misconfiguration fails fast and loud instead of surfacing later as a mysterious
// empty JWT secret or a nil DB connection string.
func RequireEnv(keys ...string) error {
	env := GetEnv()
	var missing []string

	for _, key := range keys {
		if env[key] == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}