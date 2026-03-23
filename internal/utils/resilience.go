package utils

import (
	"context"
	"errors"
	"example/hello/internal/domain"
	"log/slog"

	"github.com/avast/retry-go/v4"
	"github.com/sony/gobreaker"
)

// SafeQuery is for read operations (Get)
func SafeQuery[T any](ctx context.Context, dbBreaker *gobreaker.CircuitBreaker, title string, notFoundMsg string,
	fn func() (T, error)) (T, error) {
	var result T

	// If the breaker is nil, skip the circuit breaker logic
	// and just run the query directly.
	if dbBreaker == nil {
		return fn()
	}

	// Wrap the entire logic in the Circuit Breaker
	_, err := dbBreaker.Execute(func() (interface{}, error) {

		// Put the Retry logic INSIDE the breaker
		return nil, retry.Do(
			func() error {
				var err error
				result, err = fn()

				// If it's not transient, don't waste retries
				if err != nil && !domain.IsTransient(err) {
					return retry.Unrecoverable(err)
				}
				return err
			},
			retry.Context(ctx),
			retry.DelayType(retry.BackOffDelay),
			retry.Attempts(3),
			retry.OnRetry(func(n uint, err error) {
				// Use the title here so we know WHAT is being retried
				slog.Warn("Retry in progress",
					"task", title,
					"attempt", n+1,
					"error", err)
			}),
		)
	})

	// Use the domain.Wrap to convert the final error (or nil)
	if err != nil {
		// Handle "Breaker Open" error specifically
		if errors.Is(err, gobreaker.ErrOpenState) {
			slog.Error("[RESILIENCE] Circuit Breaker OPEN", "task", title)
			// Return a special 503 Service Unavailable via domain mapping
			return result, domain.NewUnavailableServices(err)
		}
		// Unwrap the retry error
		// retry-go/v4 returns a 'retry.Error' which is a slice of all errors that occurred.
		// We want the LAST error that actually caused the final failure.
		if retryErr, ok := err.(retry.Error); ok {
			err = retryErr.WrappedErrors()[len(retryErr.WrappedErrors())-1]
		}
		return result, domain.WrapUniversalError(err, notFoundMsg)
	}

	return result, nil
}

// SafeExec is for Write operations (Create, Update, Delete)
func SafeExec(ctx context.Context, dbBreaker *gobreaker.CircuitBreaker, title string, fn func() error) error {

	// We do avoid automatic retries here to prevent double-writes
	_, err := dbBreaker.Execute(func() (interface{}, error) {
		return nil, fn()
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			slog.Error("[RESILIENCE] Circuit Breaker OPEN - Blocking Write", "task", title)
			return domain.NewUnavailableServices(err)
		}

		return domain.WrapUniversalError(err, "")
	}

	return nil
}
