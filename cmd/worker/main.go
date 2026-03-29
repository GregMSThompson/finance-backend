package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/internal/router"
	"github.com/GregMSThompson/finance-backend/internal/services"
	"github.com/GregMSThompson/finance-backend/internal/store"
	"github.com/GregMSThompson/finance-backend/internal/taskhandlers"
)

func exitOnError(message string, err error, log *slog.Logger) {
	if err != nil {
		log.Error(message, "error", err)
		os.Exit(1)
	}
}

func main() {
	cfg := config.NewWorker()
	bs, err := bootstrap.RunWorker(cfg)
	exitOnError("bootstrap failed", err, bs.Log)
	defer bs.Close()

	rh := response.New(bs.Log)

	userStore := store.NewUserStore(bs.Firestore)
	alertEventStore := store.NewAlertEventStore(bs.Firestore)
	notificationSvc := services.NewNotificationService(userStore, alertEventStore, bs.Messaging)

	deps := &taskhandlers.Deps{
		Log:             bs.Log,
		ResponseHandler: rh,
		Audience:        cfg.WorkerAudience,
		TestAPIKey:      cfg.WorkerTestAPIKey,
		NotificationSvc: notificationSvc,
	}

	r := router.NewWorkerRouter(deps)
	err = http.ListenAndServe(":8080", r)
	exitOnError("worker start failed", err, bs.Log)
}
