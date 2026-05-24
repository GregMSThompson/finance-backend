package config

import (
	"os"

	"github.com/GregMSThompson/finance-backend/internal/dto"
)

type WorkerConfig struct {
	CommonConfig
	AppEnv                  string
	WorkerAudience          string
	WorkerTestAPIKey        string
	WorkerTestAPIKeyEnabled bool
	PlaidClientID           string
	PlaidSecret             string
	PlaidEnvironment        dto.PlaidEnvironment
	KMSKeyName              string
}

func NewWorker() *WorkerConfig {
	return &WorkerConfig{
		CommonConfig:            newCommon(),
		AppEnv:                  os.Getenv("APPENV"),
		WorkerAudience:          os.Getenv("WORKERAUDIENCE"),
		WorkerTestAPIKey:        os.Getenv("WORKERTESTAPIKEY"),
		WorkerTestAPIKeyEnabled: parseBool(os.Getenv("WORKERTESTAPIKEYENABLED")),
		PlaidClientID:           os.Getenv("PLAIDCLIENTID"),
		PlaidSecret:             os.Getenv("PLAIDSECRET"),
		PlaidEnvironment:        getPlaidEnvironment(os.Getenv("PLAIDENVIRONMENT")),
		KMSKeyName:              os.Getenv("KMSKEYNAME"),
	}
}