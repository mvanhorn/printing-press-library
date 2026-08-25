// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "since",
		Short: "What schedule items were added, moved, or cancelled since the last time you ran it.",
		Long: "Compare the upcoming schedule against the snapshot saved the last time you ran this command, " +
			"reporting events that were added, moved (start-time changed), or cancelled.\n\n" +
			"The first run saves a baseline. Use this command for schedule deltas since your last check. " +
			"Do NOT use it for the current schedule; use 'week' or 'agenda'.",
		Example:     "  sprocket-pp-cli since\n  sprocket-pp-cli since --days 45 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			if days < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			events, err := fetchCalendar(ctx, c, now, now.AddDate(0, 0, days))
			if err != nil {
				return err
			}
			snapPath, err := sinceSnapshotPath(clubKeyFromBaseURL(c.RequestBaseURL()))
			if err != nil {
				return err
			}
			prev, hadPrev, err := loadSnapshot(snapPath)
			if err != nil {
				return err
			}
			report := diffSnapshots(prev, events)
			report.Baseline = !hadPrev
			if err := saveSnapshot(snapPath, events); err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}
			return renderSinceReport(cmd, flags, report, len(events))
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "number of days ahead to track for changes")
	return cmd
}

type snapEvent struct {
	Start     string `json:"start"`
	Title     string `json:"title"`
	Cancelled bool   `json:"cancelled"`
}

type sinceChange struct {
	Title    string `json:"title"`
	Start    string `json:"start,omitempty"`
	OldStart string `json:"oldStart,omitempty"`
}

type sinceReport struct {
	Baseline  bool          `json:"baseline"`
	Added     []sinceChange `json:"added"`
	Moved     []sinceChange `json:"moved"`
	Cancelled []sinceChange `json:"cancelled"`
}

// snapKey returns a stable snapshot key for an event. Real events carry a
// unique non-zero ID; for the rare element whose ID is 0, fall back to a
// composite of start + title so multiple zero-ID events do not collide on the
// key "0" and silently overwrite each other in the snapshot map.
func snapKey(e calEvent) string {
	if e.ID != 0 {
		return fmt.Sprintf("%d", e.ID)
	}
	return "z|" + e.StartRaw + "|" + e.Title
}

// diffSnapshots compares a previous keyed snapshot against current events.
// Pure and unit-tested. Keys come from snapKey (ID, or a composite for ID 0).
func diffSnapshots(prev map[string]snapEvent, cur []calEvent) sinceReport {
	rep := sinceReport{Added: []sinceChange{}, Moved: []sinceChange{}, Cancelled: []sinceChange{}}
	for _, e := range cur {
		key := snapKey(e)
		old, ok := prev[key]
		switch {
		case !ok:
			rep.Added = append(rep.Added, sinceChange{Title: e.Title, Start: e.StartRaw})
		case e.Cancelled && !old.Cancelled:
			rep.Cancelled = append(rep.Cancelled, sinceChange{Title: e.Title, Start: e.StartRaw})
		case e.StartRaw != old.Start:
			rep.Moved = append(rep.Moved, sinceChange{Title: e.Title, Start: e.StartRaw, OldStart: old.Start})
		}
	}
	return rep
}

func renderSinceReport(cmd *cobra.Command, flags *rootFlags, rep sinceReport, tracked int) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return flags.printJSON(cmd, rep)
	}
	w := cmd.OutOrStdout()
	if rep.Baseline {
		fmt.Fprintf(w, "Baseline saved (%d events tracked). Run again later to see what changed.\n", tracked)
		return nil
	}
	if len(rep.Added) == 0 && len(rep.Moved) == 0 && len(rep.Cancelled) == 0 {
		fmt.Fprintln(w, "No schedule changes since last check.")
		return nil
	}
	if len(rep.Added) > 0 {
		fmt.Fprintf(w, "Added (%d):\n", len(rep.Added))
		for _, ch := range rep.Added {
			fmt.Fprintf(w, "  + %s — %s\n", ch.Start, ch.Title)
		}
	}
	if len(rep.Moved) > 0 {
		fmt.Fprintf(w, "Moved (%d):\n", len(rep.Moved))
		for _, ch := range rep.Moved {
			fmt.Fprintf(w, "  ~ %s: %s -> %s\n", ch.Title, ch.OldStart, ch.Start)
		}
	}
	if len(rep.Cancelled) > 0 {
		fmt.Fprintf(w, "Cancelled (%d):\n", len(rep.Cancelled))
		for _, ch := range rep.Cancelled {
			fmt.Fprintf(w, "  x %s — %s\n", ch.Start, ch.Title)
		}
	}
	return nil
}

// clubKeyFromBaseURL derives a filesystem-safe key from the configured base
// URL's host so each club's `since` snapshot is stored separately. Without
// this, switching SPROCKET_CLUB / SPROCKET_BASE_URL between runs would
// overwrite one club's snapshot with another's and report every event as
// "Added" on the next run. The key is restricted to [a-z0-9-], so it can never
// introduce path traversal.
func clubKeyFromBaseURL(baseURL string) string {
	host := baseURL
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		host = u.Host
	}
	var b strings.Builder
	for _, r := range strings.ToLower(host) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	key := strings.Trim(b.String(), "-")
	if key == "" {
		return "default"
	}
	return key
}

func sinceSnapshotPath(clubKey string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving config dir: %w", err)
	}
	return filepath.Join(dir, "sprocket-pp-cli", "since-snapshot-"+clubKey+".json"), nil
}

func loadSnapshot(path string) (map[string]snapEvent, bool, error) {
	// #nosec G304 -- path is program-controlled: os.UserConfigDir() joined with
	// the fixed "sprocket-pp-cli/since-snapshot-<clubKey>.json" segments, where
	// clubKey is sanitized to [a-z0-9-] (see clubKeyFromBaseURL/sinceSnapshotPath).
	// No raw user- or network-supplied component reaches this read.
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]snapEvent{}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("reading snapshot: %w", err)
	}
	var snap map[string]snapEvent
	if err := json.Unmarshal(data, &snap); err != nil {
		// Corrupt snapshot: treat as no baseline rather than failing.
		return map[string]snapEvent{}, false, nil
	}
	return snap, true, nil
}

func saveSnapshot(path string, events []calEvent) error {
	snap := make(map[string]snapEvent, len(events))
	for _, e := range events {
		snap[snapKey(e)] = snapEvent{Start: e.StartRaw, Title: e.Title, Cancelled: e.Cancelled}
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
