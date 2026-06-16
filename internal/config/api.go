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
	CloudTasksJobQueue    string
	WorkerAudience        string
	WorkerURL             string
	WorkerServiceAcct     string
	APIURL                string // public origin of this service (e.g. https://api.example.com)
	PlaidWebhookURL       string // derived from APIURL
	PlaidOAuthRedirectURL string // derived from APIURL; must match the AASA paths list
	AppleAppID            string
}

// Paths the API exposes that need full URLs in Plaid configuration. Kept here
// (rather than shared with the router) because the derived URLs are this
// package's responsibility; the router owns its mounts independently and the
// two need to be kept in sync by hand.
const (
	plaidWebhookPath = "/plaid/webhook"
	plaidOAuthPath   = "/plaid-oauth"
)

func NewAPI() *APIConfig {
	apiURL := os.Getenv("APIURL")
	return &APIConfig{
		CommonConfig:          newCommon(),
		PlaidClientID:         os.Getenv("PLAIDCLIENTID"),
		PlaidSecret:           os.Getenv("PLAIDSECRET"),
		PlaidEnvironment:      getPlaidEnvironment(os.Getenv("PLAIDENVIRONMENT")),
		KMSKeyName:            os.Getenv("KMSKEYNAME"),
		VertexModel:           os.Getenv("VERTEXMODEL"),
		AITTL:                 parseDuration(os.Getenv("AITTL")),
		CloudTasksJobQueue:    os.Getenv("CLOUDTASKSJOBQUEUE"),
		WorkerAudience:        os.Getenv("WORKERAUDIENCE"),
		WorkerURL:             os.Getenv("WORKERURL"),
		WorkerServiceAcct:     os.Getenv("WORKERSERVICEACCT"),
		APIURL:                apiURL,
		PlaidWebhookURL:       deriveURL(apiURL, plaidWebhookPath),
		PlaidOAuthRedirectURL: deriveURL(apiURL, plaidOAuthPath),
		AppleAppID:            os.Getenv("APPLEAPPID"),
	}
}

// deriveURL returns base+path, or "" if base is empty so that callers downstream
// (the Plaid adapter) gracefully skip setting the field rather than sending an
// invalid relative URL to Plaid.
func deriveURL(base, path string) string {
	if base == "" {
		return ""
	}
	return base + path
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
