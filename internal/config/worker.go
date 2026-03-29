package config

import "os"

type WorkerConfig struct {
	CommonConfig
	WorkerAudience   string
	WorkerTestAPIKey string
}

func NewWorker() *WorkerConfig {
	return &WorkerConfig{
		CommonConfig:     newCommon(),
		WorkerAudience:   os.Getenv("WORKERAUDIENCE"),
		WorkerTestAPIKey: os.Getenv("WORKERTESTAPIKEY"),
	}
}
