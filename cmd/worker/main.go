package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/internal/router"
	"github.com/GregMSThompson/finance-backend/internal/taskhandlers"
)

func exitOnError(message string, err error, log *slog.Logger) {
	if err != nil {
		log.Error(message, "error", err)
		os.Exit(1)
	}
}

func main() {
	cfg := config.New()
	bs, err := bootstrap.Run(cfg)
	exitOnError("bootstrap failed", err, bs.Log)
	defer bs.Close()

	rh := response.New(bs.Log)

	deps := &taskhandlers.Deps{
		Log:             bs.Log,
		ResponseHandler: rh,
		Audience:        cfg.WorkerAudience,
		TestAPIKey:      cfg.WorkerTestAPIKey,
	}

	r := router.NewWorkerRouter(deps)
	err = http.ListenAndServe(":8080", r)
	exitOnError("worker start failed", err, bs.Log)
}
