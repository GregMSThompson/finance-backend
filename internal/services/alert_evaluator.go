package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

const evalDateLayout = "2006-01-02"

type evalPeriod struct {
	now       time.Time
	today     string
	yesterday string
}

type evaluatorUserStore interface {
	List(ctx context.Context) ([]*models.User, error)
}

type evaluatorAlertStore interface {
	List(ctx context.Context, uid string, activeOnly bool) ([]*models.Alert, error)
}

type evaluatorNotificationStore interface {
	Create(ctx context.Context, uid string, n *models.Notification) error
}

type evaluatorAnalytics interface {
	GetSpendTotal(ctx context.Context, uid string, args dto.AnalyticsSpendTotalArgs) (dto.AnalyticsSpendTotalResult, error)
	GetIncomeVsExpenses(ctx context.Context, uid string, args dto.AnalyticsIncomeVsExpensesArgs) (dto.IncomeVsExpensesResult, error)
	GetRecurringTransactions(ctx context.Context, uid string, args dto.AnalyticsRecurringArgs) (dto.RecurringTransactionsResult, error)
}

type evaluatorTransactions interface {
	ListTransactions(ctx context.Context, uid string, args dto.TransactionListArgs) (dto.TransactionListResult, error)
}

type evaluatorTasksClient interface {
	EnqueueNotificationDelivery(ctx context.Context, req dto.DeliverNotificationRequest) error
}

type alertEvaluatorService struct {
	users         evaluatorUserStore
	alerts        evaluatorAlertStore
	notifications evaluatorNotificationStore
	analytics     evaluatorAnalytics
	transactions  evaluatorTransactions
	tasks         evaluatorTasksClient
	clockNow      func() time.Time
	period        evalPeriod
}

func NewAlertEvaluatorService(
	users evaluatorUserStore,
	alerts evaluatorAlertStore,
	notifications evaluatorNotificationStore,
	analytics evaluatorAnalytics,
	transactions evaluatorTransactions,
	tasks evaluatorTasksClient,
) *alertEvaluatorService {
	return &alertEvaluatorService{
		users:         users,
		alerts:        alerts,
		notifications: notifications,
		analytics:     analytics,
		transactions:  transactions,
		tasks:         tasks,
		clockNow:      time.Now,
	}
}

func (s *alertEvaluatorService) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)

	now := s.clockNow()
	s.period = evalPeriod{
		now:       now,
		today:     now.Format(evalDateLayout),
		yesterday: now.AddDate(0, 0, -1).Format(evalDateLayout),
	}

	users, err := s.users.List(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	log.Info("starting alert evaluation", "userCount", len(users))

	for _, user := range users {
		if err := s.evaluateUser(ctx, user); err != nil {
			log.Error("failed to evaluate user alerts", "uid", user.UID, "error", err)
		}
	}

	return nil
}

func (s *alertEvaluatorService) evaluateUser(ctx context.Context, user *models.User) error {
	log := logger.FromContext(ctx)

	alerts, err := s.alerts.List(ctx, user.UID, true)
	if err != nil {
		return fmt.Errorf("list alerts: %w", err)
	}

	for _, alert := range alerts {
		triggered, title, body, err := s.evaluateAlert(ctx, user.UID, alert)
		if err != nil {
			log.Error("failed to evaluate alert", "uid", user.UID, "alertId", alert.AlertID, "type", alert.Type, "error", err)
			continue
		}
		if !triggered {
			continue
		}
		if err := s.dispatchAlert(ctx, user, alert, title, body); err != nil {
			log.Error("failed to dispatch alert", "uid", user.UID, "alertId", alert.AlertID, "error", err)
		}
	}

	return nil
}

func (s *alertEvaluatorService) evaluateAlert(ctx context.Context, uid string, alert *models.Alert) (triggered bool, title, body string, err error) {
	switch alert.Type {
	case models.AlertTypeSpendThreshold:
		return s.evaluateSpendThreshold(ctx, uid, alert)
	case models.AlertTypeLargeTransaction:
		return s.evaluateLargeTransaction(ctx, uid, alert)
	case models.AlertTypeIncomeReceived:
		return s.evaluateIncomeReceived(ctx, uid, alert)
	case models.AlertTypeSubscriptionIncrease:
		return s.evaluateSubscriptionIncrease(ctx, uid)
	default:
		return false, "", "", nil
	}
}

func (s *alertEvaluatorService) evaluateSpendThreshold(ctx context.Context, uid string, alert *models.Alert) (bool, string, string, error) {
	from := evalWindowFrom(alert.Config.Window, s.period.now)

	args := dto.AnalyticsSpendTotalArgs{
		DateFrom: helpers.Ptr(from),
		DateTo:   helpers.Ptr(s.period.today),
	}
	switch alert.Config.Dimension {
	case "category":
		args.PFCPrimary = helpers.Ptr(alert.Config.Category)
	case "merchant":
		args.Merchant = helpers.Ptr(alert.Config.Merchant)
	}

	result, err := s.analytics.GetSpendTotal(ctx, uid, args)
	if err != nil {
		return false, "", "", err
	}

	if int(result.Total*100) < alert.Config.AmountMinor {
		return false, "", "", nil
	}

	subject := "overall spending"
	switch alert.Config.Dimension {
	case "category":
		subject = alert.Config.Category + " spending"
	case "merchant":
		subject = alert.Config.Merchant + " spending"
	}

	title := "Spending Alert"
	body := fmt.Sprintf("Your %s has reached %s in the last %s.", subject, helpers.FormatCurrency(result.Total, result.Currency), alert.Config.Window)
	return true, title, body, nil
}

func (s *alertEvaluatorService) evaluateLargeTransaction(ctx context.Context, uid string, alert *models.Alert) (bool, string, string, error) {
	pending := false
	result, err := s.transactions.ListTransactions(ctx, uid, dto.TransactionListArgs{
		Pending:  helpers.Ptr(pending),
		DateFrom: helpers.Ptr(s.period.yesterday),
		DateTo:   helpers.Ptr(s.period.today),
		OrderBy:  "amount",
		Desc:     true,
		Limit:    1,
	})
	if err != nil {
		return false, "", "", err
	}

	if len(result.Transactions) == 0 {
		return false, "", "", nil
	}

	tx := result.Transactions[0]
	if int(tx.Amount*100) < alert.Config.AmountMinor {
		return false, "", "", nil
	}

	title := "Large Transaction Detected"
	body := fmt.Sprintf("A transaction of %s was detected%s.", helpers.FormatCurrency(tx.Amount, tx.Currency), merchantSuffix(tx.Name))
	return true, title, body, nil
}

func (s *alertEvaluatorService) evaluateIncomeReceived(ctx context.Context, uid string, alert *models.Alert) (bool, string, string, error) {
	result, err := s.analytics.GetIncomeVsExpenses(ctx, uid, dto.AnalyticsIncomeVsExpensesArgs{
		DateFrom: s.period.yesterday,
		DateTo:   s.period.today,
	})
	if err != nil {
		return false, "", "", err
	}

	if result.Income <= 0 {
		return false, "", "", nil
	}

	if alert.Config.AmountMinor > 0 && int(result.Income*100) < alert.Config.AmountMinor {
		return false, "", "", nil
	}

	title := "Income Received"
	body := fmt.Sprintf("You received %s.", helpers.FormatCurrency(result.Income, result.Currency))
	return true, title, body, nil
}

func (s *alertEvaluatorService) evaluateSubscriptionIncrease(ctx context.Context, uid string) (bool, string, string, error) {
	from := s.period.now.AddDate(0, 0, -90).Format(evalDateLayout)

	result, err := s.analytics.GetRecurringTransactions(ctx, uid, dto.AnalyticsRecurringArgs{
		DateFrom: from,
		DateTo:   s.period.today,
	})
	if err != nil {
		return false, "", "", err
	}

	for _, item := range result.Items {
		if item.LastDate != s.period.yesterday {
			continue
		}
		if item.LastAmount <= item.TypicalAmount {
			continue
		}
		title := "Subscription Price Increase"
		body := fmt.Sprintf("Your %s subscription increased to %s (was typically %s).",
			item.Merchant,
			helpers.FormatCurrency(item.LastAmount, item.Currency),
			helpers.FormatCurrency(item.TypicalAmount, item.Currency),
		)
		return true, title, body, nil
	}

	return false, "", "", nil
}

func (s *alertEvaluatorService) dispatchAlert(ctx context.Context, user *models.User, alert *models.Alert, title, body string) error {
	for _, delivery := range alert.Delivery {
		notification := &models.Notification{
			NotificationID: uuid.New().String(),
			Source:         models.NotificationSourceAlert,
			SourceID:       alert.AlertID,
			Title:          title,
			Body:           body,
			Data: map[string]string{
				"source":   string(models.NotificationSourceAlert),
				"sourceId": alert.AlertID,
			},
			Delivery:  delivery,
			CreatedAt: s.period.now,
		}

		if err := s.notifications.Create(ctx, user.UID, notification); err != nil {
			return fmt.Errorf("create notification: %w", err)
		}

		if err := s.tasks.EnqueueNotificationDelivery(ctx, dto.DeliverNotificationRequest{
			NotificationID: notification.NotificationID,
			UserID:         user.UID,
			Delivery:       delivery,
		}); err != nil {
			return fmt.Errorf("enqueue notification delivery: %w", err)
		}
	}

	return nil
}

func evalWindowFrom(window string, now time.Time) string {
	switch window {
	case "7day":
		return now.AddDate(0, 0, -7).Format(evalDateLayout)
	case "30day":
		return now.AddDate(0, 0, -30).Format(evalDateLayout)
	case "60day":
		return now.AddDate(0, 0, -60).Format(evalDateLayout)
	case "90day":
		return now.AddDate(0, 0, -90).Format(evalDateLayout)
	default:
		return now.AddDate(0, 0, -1).Format(evalDateLayout)
	}
}

func merchantSuffix(name string) string {
	if name == "" {
		return ""
	}
	return " at " + name
}