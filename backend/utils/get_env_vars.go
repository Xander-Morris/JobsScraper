package utils

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type EnvPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var allEnvVariables map[string]string

func GetEnv() map[string]string {
	if allEnvVariables != nil {
		return allEnvVariables
	}

	_ = godotenv.Load()
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
