package config

import (
	"os"
)

type Config struct {
	ProjectID string
}

func New() *Config {
	return &Config{
		ProjectID: os.Getenv("projectID"),
	}
}
