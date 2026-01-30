package config

import (
	"os"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
)

type Config struct {
	ProjectID        string
	Region           string
	LogLevel         string
	PlaidClientID    string
	PlaidSecret      string
	PlaidEnvironment dto.PlaidEnvironment
	KMSKeyName       string
	VertexModel      string
	AITTL            time.Duration
}

func New() *Config {
	return &Config{
		ProjectID:        os.Getenv("PROJECTID"),
		Region:           os.Getenv("REGION"),
		LogLevel:         os.Getenv("LOGLEVEL"),
		PlaidClientID:    os.Getenv("PLAIDCLIENTID"),
		PlaidSecret:      os.Getenv("PLAIDSECRET"),
		PlaidEnvironment: getPlaidEnvironment(os.Getenv("PLAIDENVIRONMENT")),
		KMSKeyName:       os.Getenv("KMSKEYNAME"),
		VertexModel:      os.Getenv("VERTEXMODEL"),
		AITTL:            parseDuration(os.Getenv("AITTL")),
	}
}

func getPlaidEnvironment(env string) dto.PlaidEnvironment {
	switch env {
	case "sandbox":
		return dto.PlaidSandbox
	case "development":
		return dto.PalidDevelopment
	default: // "production"
		return dto.PlaidProduction
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
