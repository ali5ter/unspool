package api

import (
	"regexp"
	"strconv"
)

// durationPattern matches the subset of ISO-8601 durations YouTube actually
// emits for contentDetails.duration (e.g. "PT4M13S", "PT45S", "PT1H2M").
var durationPattern = regexp.MustCompile(`^P(?:(\d+)D)?T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// parseISO8601Duration converts an ISO-8601 duration string to whole
// seconds. Returns 0 for anything it can't parse rather than erroring — a
// malformed duration shouldn't abort a whole sync batch.
func parseISO8601Duration(s string) int {
	m := durationPattern.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	days := atoiOr0(m[1])
	hours := atoiOr0(m[2])
	minutes := atoiOr0(m[3])
	seconds := atoiOr0(m[4])
	return days*86400 + hours*3600 + minutes*60 + seconds
}

func atoiOr0(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
