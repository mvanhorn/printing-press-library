// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

type repeatResult struct {
	SourceID   string   `json:"source_id"`
	Repeated   []string `json:"repeated_ids,omitempty"`
	SkippedIDs []string `json:"skipped_existing_ids,omitempty"`
	Targets    []string `json:"target_dates"`
	DryRun     bool     `json:"dry_run"`
}

// newTimeEntriesRepeatCmd returns a command that re-posts an existing time
// entry to one or more future dates. It is registered onto the existing
// time-entries parent in root.go (see registerNovelFeatures below).
func newTimeEntriesRepeatCmd(flags *rootFlags) *cobra.Command {
	var (
		days       int
		toDate     string
		skipExist  bool
		skipFriday bool
	)

	cmd := &cobra.Command{
		Use:   "repeat <time-entry-id>",
		Short: "Repost an existing entry to a new date or N consecutive workdays",
		Long: `Fetches the source time entry, then POSTs copies with new spent_date values.
Idempotent: each target date checks for an existing entry with the same
(user, date, project, task, notes-hash) and skips if present.

Defaults: 5 consecutive workdays starting tomorrow, weekends skipped.`,
		Example: `  # Replay entry 12345 across the next 5 workdays (dry-run)
  harvest-pp-cli time-entries repeat 12345 --days 5 --dry-run --json

  # Replay to a specific date
  harvest-pp-cli time-entries repeat 12345 --to 2026-05-22 --json`,
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/time_entries/"+srcID, nil)
			if err != nil {
				return fmt.Errorf("fetch source entry %s: %w", srcID, err)
			}
			var src map[string]any
			if err := json.Unmarshal(raw, &src); err != nil {
				return fmt.Errorf("parse source entry: %w", err)
			}

			targets, err := buildRepeatDates(toDate, days, skipFriday)
			if err != nil {
				return err
			}

			result := repeatResult{SourceID: srcID, DryRun: flags.dryRun}
			for _, d := range targets {
				result.Targets = append(result.Targets, d)
			}

			if flags.dryRun {
				return flags.printJSON(cmd, result)
			}

			for _, d := range targets {
				body := buildRepeatBody(src, d)
				if skipExist {
					if id, found := findExistingRepeat(c, src, d); found {
						result.SkippedIDs = append(result.SkippedIDs, id)
						continue
					}
				}
				rraw, status, err := c.Post("/time_entries", body)
				if err != nil {
					return fmt.Errorf("create entry for %s: %w", d, err)
				}
				if status >= 400 {
					return fmt.Errorf("create entry for %s: HTTP %d", d, status)
				}
				var created map[string]any
				if err := json.Unmarshal(rraw, &created); err == nil {
					if id, ok := created["id"].(float64); ok {
						result.Repeated = append(result.Repeated, strconv.FormatInt(int64(id), 10))
					}
				}
			}
			return flags.printJSON(cmd, result)
		},
	}

	cmd.Flags().IntVar(&days, "days", 5, "Number of consecutive workdays after the source date")
	cmd.Flags().StringVar(&toDate, "to", "", "Specific target date (YYYY-MM-DD); overrides --days")
	cmd.Flags().BoolVar(&skipExist, "skip-existing", true, "Skip dates that already have a matching entry")
	cmd.Flags().BoolVar(&skipFriday, "weekdays-only", true, "Skip Saturday/Sunday")
	return cmd
}

func buildRepeatDates(toDate string, days int, weekdaysOnly bool) ([]string, error) {
	if toDate != "" {
		if _, err := time.Parse("2006-01-02", toDate); err != nil {
			return nil, fmt.Errorf("parse --to: %w", err)
		}
		return []string{toDate}, nil
	}
	if days <= 0 {
		return nil, fmt.Errorf("--days must be > 0")
	}
	start := time.Now().AddDate(0, 0, 1)
	out := make([]string, 0, days)
	for d := start; len(out) < days; d = d.AddDate(0, 0, 1) {
		if weekdaysOnly && (d.Weekday() == time.Saturday || d.Weekday() == time.Sunday) {
			continue
		}
		out = append(out, d.Format("2006-01-02"))
	}
	return out, nil
}

func buildRepeatBody(src map[string]any, target string) map[string]any {
	body := map[string]any{
		"spent_date": target,
	}
	if v, ok := src["hours"].(float64); ok {
		body["hours"] = v
	}
	if v, ok := src["notes"].(string); ok {
		body["notes"] = v
	}
	if v, ok := src["external_reference"].(map[string]any); ok {
		body["external_reference"] = v
	}
	if user, ok := src["user"].(map[string]any); ok {
		if id, ok := user["id"].(float64); ok {
			body["user_id"] = int64(id)
		}
	}
	if proj, ok := src["project"].(map[string]any); ok {
		if id, ok := proj["id"].(float64); ok {
			body["project_id"] = int64(id)
		}
	}
	if task, ok := src["task"].(map[string]any); ok {
		if id, ok := task["id"].(float64); ok {
			body["task_id"] = int64(id)
		}
	}
	return body
}

func findExistingRepeat(c clientLike, src map[string]any, target string) (string, bool) {
	// Best-effort: pull the day's entries for the same user and compare key fields.
	params := map[string]string{"from": target, "to": target}
	if user, ok := src["user"].(map[string]any); ok {
		if id, ok := user["id"].(float64); ok {
			params["user_id"] = strconv.FormatInt(int64(id), 10)
		}
	}
	raw, err := c.Get("/time_entries", params)
	if err != nil {
		return "", false
	}
	var resp struct {
		TimeEntries []map[string]any `json:"time_entries"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", false
	}
	srcHash := repeatNotesHash(src)
	for _, e := range resp.TimeEntries {
		if repeatNotesHash(e) == srcHash {
			if id, ok := e["id"].(float64); ok {
				return strconv.FormatInt(int64(id), 10), true
			}
		}
	}
	return "", false
}

func repeatNotesHash(obj map[string]any) string {
	var parts []byte
	if user, ok := obj["user"].(map[string]any); ok {
		if id, ok := user["id"].(float64); ok {
			parts = append(parts, []byte(strconv.FormatInt(int64(id), 10))...)
		}
	}
	if proj, ok := obj["project"].(map[string]any); ok {
		if id, ok := proj["id"].(float64); ok {
			parts = append(parts, []byte(strconv.FormatInt(int64(id), 10))...)
		}
	}
	if task, ok := obj["task"].(map[string]any); ok {
		if id, ok := task["id"].(float64); ok {
			parts = append(parts, []byte(strconv.FormatInt(int64(id), 10))...)
		}
	}
	if n, ok := obj["notes"].(string); ok {
		parts = append(parts, []byte(n)...)
	}
	if h, ok := obj["hours"].(float64); ok {
		parts = append(parts, []byte(strconv.FormatFloat(h, 'f', 2, 64))...)
	}
	sum := sha256.Sum256(parts)
	return hex.EncodeToString(sum[:8])
}

// clientLike is the minimal interface this command needs from client.Client.
// Defined here to avoid a hard import cycle if client.Client signatures evolve.
type clientLike interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}
