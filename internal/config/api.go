package config

import (
	"os"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
)

type APIConfig struct {
	CommonConfig
	PlaidClientID    string
	PlaidSecret      string
	PlaidEnvironment dto.PlaidEnvironment
	KMSKeyName       string
	VertexModel      string
	AITTL            time.Duration
}

func NewAPI() *APIConfig {
	return &APIConfig{
		CommonConfig:     newCommon(),
		PlaidClientID:    os.Getenv("PLAIDCLIENTID"),
		PlaidSecret:      os.Getenv("PLAIDSECRET"),
		PlaidEnvironment: getPlaidEnvironment(os.Getenv("PLAIDENVIRONMENT")),
		KMSKeyName:       os.Getenv("KMSKEYNAME"),
		VertexModel:      os.Getenv("VERTEXMODEL"),
		AITTL:            parseDuration(os.Getenv("AITTL")),
	}
}
