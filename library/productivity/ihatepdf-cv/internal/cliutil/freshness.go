package cliutil

import (
	"context"
	"os"
	"time"
)

const DefaultFreshnessWindow = 30 * time.Minute

// EnsureFresh reports the age and usability of the local store without
// pretending that a local-only CLI has a remote source to refresh from.
// Missing stores are a healthy empty state; callers can distinguish that from
// stale data using status and age_seconds.
func EnsureFresh(ctx context.Context, dbPath string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	result := map[string]any{
		"source":              "local",
		"checked_at":          now.Format(time.RFC3339),
		"stale_after_seconds": int(DefaultFreshnessWindow.Seconds()),
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			result["status"] = "empty"
			result["fresh"] = true
			result["detail"] = "local store has not been created yet"
			return result, nil
		}
		return nil, err
	}
	age := now.Sub(info.ModTime().UTC())
	result["age_seconds"] = int(age.Seconds())
	result["fresh"] = age <= DefaultFreshnessWindow
	if age <= DefaultFreshnessWindow {
		result["status"] = "fresh"
	} else {
		result["status"] = "stale"
		result["detail"] = "local data is labeled stale; this CLI does not invent a remote refresh"
	}
	return result, nil
}
