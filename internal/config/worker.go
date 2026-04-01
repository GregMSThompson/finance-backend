package config

import "os"

type WorkerConfig struct {
	CommonConfig
	AppEnv                  string
	WorkerAudience          string
	WorkerTestAPIKey        string
	WorkerTestAPIKeyEnabled bool
}

func NewWorker() *WorkerConfig {
	return &WorkerConfig{
		CommonConfig:            newCommon(),
		AppEnv:                  os.Getenv("APPENV"),
		WorkerAudience:          os.Getenv("WORKERAUDIENCE"),
		WorkerTestAPIKey:        os.Getenv("WORKERTESTAPIKEY"),
		WorkerTestAPIKeyEnabled: parseBool(os.Getenv("WORKERTESTAPIKEYENABLED")),
	}
}
