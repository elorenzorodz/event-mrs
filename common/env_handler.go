package common

import (
	"log"
	"os"
)

func GetEnvVariable(name string) string {
	envValue := os.Getenv(name)

	if envValue == "" {
		log.Fatal("Environment variable not found: ", name)
	}

	return envValue
}