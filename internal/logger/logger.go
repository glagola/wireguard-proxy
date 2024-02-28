package logger

import (
	"context"
	"log"
	"log/slog"
)

type ctxKeyType string

const ctxKey ctxKeyType = "logger"

func AddLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey, logger)
}

func MustGetLogger(ctx context.Context) *slog.Logger {
	switch logger := ctx.Value(ctxKey).(type) {
	case *slog.Logger:
		return logger
	}

	log.Fatal("Failed to find logger in context")

	return nil
}
