package main

import (
	"context"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/services"
	"github.com/GregMSThompson/finance-backend/internal/store"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

func main() {
	cfg := config.NewAlertEvaluator()
	bs, err := bootstrap.RunAlertEvaluator(cfg)
	helpers.ExitOnError("bootstrap failed", err, bs.Log)
	defer bs.Close()

	ctx := logger.ToContext(context.Background(), bs.Log)

	userStore := store.NewUserStore(bs.Firestore)
	alertStore := store.NewAlertStore(bs.Firestore)
	alertEventStore := store.NewAlertEventStore(bs.Firestore)
	transactionStore := store.NewTransactionStore(bs.Firestore)

	analyticsSvc := services.NewAnalyticsService(transactionStore)
	txSvc := services.NewTransactionsService(transactionStore)
	evaluatorSvc := services.NewAlertEvaluatorService(userStore, alertStore, alertEventStore, analyticsSvc, txSvc, bs.CloudTasks)

	err = evaluatorSvc.Run(ctx)
	helpers.ExitOnError("alert evaluation failed", err, bs.Log)
}