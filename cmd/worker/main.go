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

	// stores
	userStore := store.NewUserStore(bs.Firestore)
	notificationStore := store.NewNotificationStore(bs.Firestore)
	bankStore := store.NewBankStore(bs.Firestore, kmsHelper)
	orphanedBankStore := store.NewOrphanedBankStore(bs.Firestore, kmsHelper)
	transactionStore := store.NewTransactionStore(bs.Firestore)
	accountStore := store.NewAccountStore(bs.Firestore)
	goalStore := store.NewGoalStore(bs.Firestore)
	goalSnapshotStore := store.NewGoalSnapshotStore(bs.Firestore)
	jobStore := store.NewJobStore(bs.Firestore)

	// services
	notificationSvc := services.NewNotificationService(userStore, notificationStore, bs.Messaging)
	// Worker only consumes jobs; pass nil enqueuer since Submit is not called here.
	jobSvc := services.NewJobService(jobStore, nil)
	bankSvc := services.NewBankService(bankStore, transactionStore, accountStore, bs.PlaidAdapter, orphanedBankStore, jobSvc)
	accountsSvc := services.NewAccountsService(bs.PlaidAdapter, bankStore, accountStore, jobSvc)
	plaidSvc := services.NewPlaidService(bs.PlaidAdapter, bankStore, transactionStore, jobSvc, bankSvc, accountsSvc)
	txSvc := services.NewTransactionsService(transactionStore)
	analyticsSvc := services.NewAnalyticsService(transactionStore)
	goalSvc := services.NewGoalService(goalStore, goalSnapshotStore, jobSvc, txSvc, analyticsSvc)

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
		GoalSvc:           goalSvc,
		AccountsSvc:       accountsSvc,
	}

	r := router.NewWorkerRouter(deps)
	err = http.ListenAndServe(":8080", r)
	helpers.ExitOnError("worker start failed", err, bs.Log)
}
