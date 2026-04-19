package helpers

import (
	"log/slog"
	"os"
)

// ExitOnError logs the message and exits with status 1 if err is non-nil.
func ExitOnError(message string, err error, log *slog.Logger) {
	if err != nil {
		log.Error(message, "error", err)
		os.Exit(1)
	}
}
