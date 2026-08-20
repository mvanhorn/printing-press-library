package cli

import (
	"fmt"
	"strings"
	"time"
)

// timelineWindow scopes a bare calendar date (YYYY-MM-DD with no --until) to
// that single day, so `timeline 2026-05-01` returns just May 1 rather than
// everything from May 1 to now. Relative expressions (7d, 2w) and explicit
// --since/--until ranges fall through to the normal window.
func timelineWindow(since, until string) (time.Time, time.Time, error) {
	s := strings.TrimSpace(since)
	if strings.TrimSpace(until) == "" {
		if d, err := time.Parse("2006-01-02", s); err == nil {
			start := d.UTC()
			return start, start.Add(24 * time.Hour), nil
		}
	}
	return sourceTimeWindow(since, until, 24*time.Hour)
}

func sourceTimeWindow(since, until string, defaultSince time.Duration) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	end := now
	if strings.TrimSpace(until) != "" {
		t, err := parseTimeExpr(until, now)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --until: %w", err)
		}
		end = t
	}
	start := end.Add(-defaultSince)
	if strings.TrimSpace(since) != "" {
		t, err := parseTimeExpr(since, now)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --since: %w", err)
		}
		start = t
	}
	return start, end, nil
}

func parseTimeExpr(v string, now time.Time) (time.Time, error) {
	v = strings.TrimSpace(v)
	if strings.HasSuffix(v, "d") {
		n := strings.TrimSuffix(v, "d")
		var days int
		if _, err := fmt.Sscanf(n, "%d", &days); err == nil {
			return now.Add(-time.Duration(days) * 24 * time.Hour), nil
		}
	}
	if strings.HasSuffix(v, "w") {
		n := strings.TrimSuffix(v, "w")
		var weeks int
		if _, err := fmt.Sscanf(n, "%d", &weeks); err == nil {
			return now.Add(-time.Duration(weeks) * 7 * 24 * time.Hour), nil
		}
	}
	if d, err := time.ParseDuration(v); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time expression: %q", v)
}
