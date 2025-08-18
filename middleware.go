package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// StepUUIDKey is the context key for storing step UUIDs.
const StepUUIDKey contextKey = "step_uuid"

// UUIDMiddleware returns a middleware that assigns a unique UUID to each step execution.
// The UUID is stored in the context with the key StepUUIDKey and can be retrieved
// using ctx.Value(StepUUIDKey).(string).
func UUIDMiddleware[T any]() Middleware[T] {
	return func(next Step[T]) Step[T] {
		return &MidFunc[T]{
			Name: "UUID",
			Next: next,
			Fn: func(ctx context.Context, req *T) (*T, error) {
				stepUUID := uuid.New().String()
				ctx = context.WithValue(ctx, StepUUIDKey, stepUUID)
				return next.Run(ctx, req)
			},
		}
	}
}

func LoggerMiddleware[T any](l *slog.Logger) Middleware[T] {
	return func(next Step[T]) Step[T] {
		return &MidFunc[T]{
			Name: "Logger",
			Next: next,
			Fn: func(ctx context.Context, res *T) (*T, error) {
				start := time.Now()
				name := Name(next)
				if name != "MidFunc" {
					l.Info("start", "Type", name, "STEP", next)
				}
				resp, err := next.Run(ctx, res)

				if name != "MidFunc" {
					l.Info("done", "Type", name, "duration", time.Since(start),
						"Result", fmt.Sprintf("%v", resp))
				}
				return resp, err
			},
		}
	}
}
