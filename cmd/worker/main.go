package main

import (
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/internal/router"
	"github.com/GregMSThompson/finance-backend/internal/services"
	"github.com/GregMSThompson/finance-backend/internal/store"
	"github.com/GregMSThompson/finance-backend/internal/taskhandlers"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

func main() {
	cfg := config.NewWorker()
	bs, err := bootstrap.RunWorker(cfg)
	helpers.ExitOnError("bootstrap failed", err, bs.Log)
	defer bs.Close()

	rh := response.New(bs.Log)

	userStore := store.NewUserStore(bs.Firestore)
	alertEventStore := store.NewAlertEventStore(bs.Firestore)
	notificationSvc := services.NewNotificationService(userStore, alertEventStore, bs.Messaging)

	deps := &taskhandlers.Deps{
		Log:               bs.Log,
		ResponseHandler:   rh,
		AppEnv:            cfg.AppEnv,
		Audience:          cfg.WorkerAudience,
		TestAPIKeyEnabled: cfg.WorkerTestAPIKeyEnabled,
		TestAPIKey:        cfg.WorkerTestAPIKey,
		NotificationSvc:   notificationSvc,
	}

	r := router.NewWorkerRouter(deps)
	err = http.ListenAndServe(":8080", r)
	helpers.ExitOnError("worker start failed", err, bs.Log)
}
