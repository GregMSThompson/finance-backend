package helpers

import (
	"context"
	"log/slog"

	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

// TestCtx returns a context carrying a test logger.
func TestCtx() context.Context {
	log := slog.New(logger.NewTestHandler(slog.LevelInfo))
	return logger.ToContext(context.Background(), log)
}
