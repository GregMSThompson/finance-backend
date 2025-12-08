package config

import (
	"os"
)

type Config struct {
	ProjectID string
	LogLevel  string
}

func New() *Config {
	return &Config{
		ProjectID: os.Getenv("PROJECTID"),
		LogLevel:  os.Getenv("LOGLEVEL"),
	}
}
