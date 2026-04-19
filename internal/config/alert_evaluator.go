package config

import "os"

type AlertEvaluatorConfig struct {
	CommonConfig
	CloudTasksQueue    string
	WorkerURL          string
	WorkerServiceAcct  string
}

func NewAlertEvaluator() *AlertEvaluatorConfig {
	return &AlertEvaluatorConfig{
		CommonConfig:      newCommon(),
		CloudTasksQueue:   os.Getenv("CLOUDTASKSQUEUE"),
		WorkerURL:         os.Getenv("WORKERURL"),
		WorkerServiceAcct: os.Getenv("WORKERSERVICEACCT"),
	}
}