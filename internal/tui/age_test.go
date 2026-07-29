package tui

import (
	"testing"
	"time"
)

// TestAgeParts locks the issue #4 contract: recent content reads as a compact
// relative span, anything older than ~a year reads as an absolute month-year
// (never an ever-growing day count like "1284d").
func TestAgeParts(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		at       time.Time
		want     string
		relative bool
	}{
		{"zero", time.Time{}, "—", false},
		{"minutes", now.Add(-30 * time.Minute), "30m", true},
		{"hours", now.Add(-5 * time.Hour), "5h", true},
		{"days", now.Add(-3 * 24 * time.Hour), "3d", true},
		{"weeks", now.Add(-3 * 7 * 24 * time.Hour), "3w", true},
		{"months", now.Add(-100 * 24 * time.Hour), "3mo", true},
		{"over a year is a date", time.Date(2023, time.March, 14, 0, 0, 0, 0, time.UTC), "Mar 2023", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, rel := ageParts(c.at)
			if got != c.want || rel != c.relative {
				t.Fatalf("ageParts(%v) = (%q, %v), want (%q, %v)", c.at, got, rel, c.want, c.relative)
			}
			if humanAge(c.at) != c.want {
				t.Fatalf("humanAge(%v) = %q, want %q", c.at, humanAge(c.at), c.want)
			}
		})
	}
}
