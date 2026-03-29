package config

import (
	"os"
	"time"
)

type CommonConfig struct {
	ProjectID string
	Region    string
	LogLevel  string
}

func newCommon() CommonConfig {
	return CommonConfig{
		ProjectID: os.Getenv("PROJECTID"),
		Region:    os.Getenv("REGION"),
		LogLevel:  os.Getenv("LOGLEVEL"),
	}
}

func parseDuration(value string) time.Duration {
	if value == "" {
		return 0
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return d
}
