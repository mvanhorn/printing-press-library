package cli

import (
	"strconv"
	"time"
)

// parseAPITime accepts the timestamp shapes MultiMail returns in synced data:
// RFC3339/RFC3339Nano strings for email and audit events, plus epoch
// millisecond/second values for mailbox.created_at. SQLite json_extract returns
// numeric JSON values as text when scanned into a string, so numeric parsing is
// required to avoid permanently blank age metrics.
func parseAPITime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, true
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		// MultiMail documents mailbox.created_at as epoch milliseconds. Accept
		// seconds too so older/local fixtures do not render as year-1970 ages.
		if n > 1_000_000_000_000 {
			return time.UnixMilli(n), true
		}
		if n > 0 {
			return time.Unix(n, 0), true
		}
	}
	return time.Time{}, false
}
