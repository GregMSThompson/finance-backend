package main

import (
	"context"
	"time"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/services"
	"github.com/GregMSThompson/finance-backend/internal/store"
	"github.com/GregMSThompson/finance-backend/pkg/clock"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

func main() {
	cfg := config.NewAlertEvaluator()
	bs, err := bootstrap.RunAlertEvaluator(cfg)
	helpers.ExitOnError("bootstrap failed", err, bs.Log)
	defer bs.Close()

	ctx := clock.WithClock(logger.ToContext(context.Background(), bs.Log), time.Now)

	userStore := store.NewUserStore(bs.Firestore)
	alertStore := store.NewAlertStore(bs.Firestore)
	notificationStore := store.NewNotificationStore(bs.Firestore)
	transactionStore := store.NewTransactionStore(bs.Firestore)

	analyticsSvc := services.NewAnalyticsService(transactionStore)
	txSvc := services.NewTransactionsService(transactionStore)
	evaluatorSvc := services.NewAlertEvaluatorService(userStore, alertStore, notificationStore, analyticsSvc, txSvc, bs.CloudTasks)

	err = evaluatorSvc.Run(ctx)
	helpers.ExitOnError("alert evaluation failed", err, bs.Log)
}
