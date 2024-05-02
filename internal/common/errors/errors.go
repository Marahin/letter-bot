package errors

import (
	"context"

	"go.uber.org/zap"
)

type LogEntry interface {
	Error(args ...interface{})
}

// LogErrors allows for error handling in defer calls:
// ```
// defer functionThatReturnsError()
// ````
// This would lead to uncatched error. Wrapping it with LogError solves this issue:
// ```
// defer LogError(functionThatReturnsError())
// ```
func LogError(log LogEntry, err error) {
	if err != nil {
		log.Error(err)
	}
}

// Deliberately ignore error. This is specifically used in cases where we
// do not care about the error message at all, such as the case
// of `defer tx.Rollback(ctx)`: https://stackoverflow.com/a/62533516
func IgnoreError(err error) {
	zap.S().Debugf("IgnoreError: %s", err)
}

// This is for case of `tx.Rollback(ctx)` in defer.
// defer calls happen when the function body ends; however,
// their arguments are evaluated at the moment `defer` is called.
// For instance, if we want to deliberately ignore errors (so golangci-lint doesn't yell at us)
// but still have `defer tx.Rollback(ctx)` (which does nothing if the transaction succeeded)
// we can't run `defer IgnoreError(tx.Rollback(ctx))` because the `tx.Rollback` part is evaluated
// the very moment. And so it cancels transaction at the moment of calling defer.
func ExecuteAndIgnoreErrorF(f func(context.Context) error, ctx context.Context) {
	IgnoreError(f(ctx))
}
