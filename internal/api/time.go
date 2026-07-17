package api

import "time"

// parseAPITimestamp parses an RFC3339 timestamp as returned by the YouTube
// Data API, returning the zero time on failure rather than erroring — a bad
// timestamp shouldn't abort a sync batch.
func parseAPITimestamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
