package config

import "os"

type GoalEvaluatorConfig struct {
	CommonConfig
	CloudTasksQueue   string
	WorkerAudience    string
	WorkerURL         string
	WorkerServiceAcct string
}

func NewGoalEvaluator() *GoalEvaluatorConfig {
	return &GoalEvaluatorConfig{
		CommonConfig:      newCommon(),
		CloudTasksQueue:   os.Getenv("CLOUDTASKSQUEUE"),
		WorkerAudience:    os.Getenv("WORKERAUDIENCE"),
		WorkerURL:         os.Getenv("WORKERURL"),
		WorkerServiceAcct: os.Getenv("WORKERSERVICEACCT"),
	}
}
