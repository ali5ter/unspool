package api

import (
	"context"
	"errors"
	"time"

	"google.golang.org/api/googleapi"
)

// retryTransient retries fn a few times with backoff when it fails with an
// error that looks like YouTube's backend not yet being consistent for a
// just-created resource. Confirmed directly against the live API: adding an
// item to a playlist immediately after creating that playlist can fail with
// a 409 "SERVICE_UNAVAILABLE" for a few seconds before succeeding on retry
// with no other change — the same class of eventual-consistency lag already
// worked around for playlistItems.list in internal/queue. Every operation
// wrapped with this must be safe to call more than once (fn should not have
// already-applied side effects on a "successful" partial failure).
func retryTransient(ctx context.Context, fn func() error) error {
	const maxAttempts = 4
	backoff := 1 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if !isTransientAPIError(err) {
			return err
		}
		lastErr = err
		if attempt < maxAttempts {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff *= 2
		}
	}
	return lastErr
}

// isTransientAPIError reports whether err is a googleapi.Error whose status
// code is one observed to be a transient eventual-consistency hiccup rather
// than a real, retry-proof failure: 409 Conflict (seen directly against the
// live API for playlistItems.insert on a fresh playlist), 503 Service
// Unavailable, and 404 Not Found (seen for playlistItems.list on a fresh
// playlist).
func isTransientAPIError(err error) bool {
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) {
		return false
	}
	switch gerr.Code {
	case 404, 409, 503:
		return true
	default:
		return false
	}
}
