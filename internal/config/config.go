package config

import (
	"os"

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
