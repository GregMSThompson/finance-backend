package main

import (
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/crypto"
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

	kmsHelper := crypto.NewKMS(bs.KMS, cfg.KMSKeyName)

	userStore := store.NewUserStore(bs.Firestore)
	alertEventStore := store.NewAlertEventStore(bs.Firestore)
	bankStore := store.NewBankStore(bs.Firestore, kmsHelper)
	transactionStore := store.NewTransactionStore(bs.Firestore)
	jobStore := store.NewJobStore(bs.Firestore)

	notificationSvc := services.NewNotificationService(userStore, alertEventStore, bs.Messaging)
	// Worker only consumes jobs; pass nil enqueuer since Submit is not called here.
	jobSvc := services.NewJobService(jobStore, nil)
	plaidSvc := services.NewPlaidService(bs.PlaidAdapter, bankStore, transactionStore, jobSvc)
	bankSvc := services.NewBankService(bankStore, transactionStore, jobSvc)

	deps := &taskhandlers.Deps{
		Log:               bs.Log,
		ResponseHandler:   rh,
		AppEnv:            cfg.AppEnv,
		Audience:          cfg.WorkerAudience,
		TestAPIKeyEnabled: cfg.WorkerTestAPIKeyEnabled,
		TestAPIKey:        cfg.WorkerTestAPIKey,
		NotificationSvc:   notificationSvc,
		JobSvc:            jobSvc,
		PlaidSvc:          plaidSvc,
		BankSvc:           bankSvc,
	}

	r := router.NewWorkerRouter(deps)
	err = http.ListenAndServe(":8080", r)
	helpers.ExitOnError("worker start failed", err, bs.Log)
}
