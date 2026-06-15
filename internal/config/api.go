package config

import (
	"os"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/dto"
)

type APIConfig struct {
	CommonConfig
	PlaidClientID      string
	PlaidSecret        string
	PlaidEnvironment   dto.PlaidEnvironment
	KMSKeyName         string
	VertexModel        string
	AITTL              time.Duration
	CloudTasksJobQueue string
	WorkerAudience     string
	WorkerURL          string
	WorkerServiceAcct  string
	PlaidWebhookURL    string
	AppleAppID         string
}

func NewAPI() *APIConfig {
	return &APIConfig{
		CommonConfig:       newCommon(),
		PlaidClientID:      os.Getenv("PLAIDCLIENTID"),
		PlaidSecret:        os.Getenv("PLAIDSECRET"),
		PlaidEnvironment:   getPlaidEnvironment(os.Getenv("PLAIDENVIRONMENT")),
		KMSKeyName:         os.Getenv("KMSKEYNAME"),
		VertexModel:        os.Getenv("VERTEXMODEL"),
		AITTL:              parseDuration(os.Getenv("AITTL")),
		CloudTasksJobQueue: os.Getenv("CLOUDTASKSJOBQUEUE"),
		WorkerAudience:     os.Getenv("WORKERAUDIENCE"),
		WorkerURL:          os.Getenv("WORKERURL"),
		WorkerServiceAcct:  os.Getenv("WORKERSERVICEACCT"),
		PlaidWebhookURL:    os.Getenv("PLAIDWEBHOOKURL"),
		AppleAppID:         os.Getenv("APPLEAPPID"),
	}
}

func getPlaidEnvironment(env string) dto.PlaidEnvironment {
	switch env {
	case "sandbox":
		return dto.PlaidSandbox
	case "development":
		return dto.PalidDevelopment
	default:
		return dto.PlaidProduction
	}
}
