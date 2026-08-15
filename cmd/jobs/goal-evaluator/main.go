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
	cfg := config.NewGoalEvaluator()
	bs, err := bootstrap.RunGoalEvaluator(cfg)
	helpers.ExitOnError("bootstrap failed", err, bs.Log)
	defer bs.Close()

	ctx := clock.WithClock(logger.ToContext(context.Background(), bs.Log), time.Now)

	userStore := store.NewUserStore(bs.Firestore)
	goalStore := store.NewGoalStore(bs.Firestore)
	goalSnapshotStore := store.NewGoalSnapshotStore(bs.Firestore)
	notificationStore := store.NewNotificationStore(bs.Firestore)
	transactionStore := store.NewTransactionStore(bs.Firestore)

	analyticsSvc := services.NewAnalyticsService(transactionStore)
	evaluatorSvc := services.NewGoalEvaluatorService(userStore, goalStore, goalSnapshotStore, analyticsSvc, notificationStore, bs.CloudTasks)

	err = evaluatorSvc.Run(ctx)
	helpers.ExitOnError("goal evaluation failed", err, bs.Log)
}
