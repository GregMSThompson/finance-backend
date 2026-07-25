package config

import (
	"os"
	"strconv"
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

func parseBool(value string) bool {
	if value == "" {
		return false
	}

	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return enabled
}
