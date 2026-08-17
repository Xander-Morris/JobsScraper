package utils

import (
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