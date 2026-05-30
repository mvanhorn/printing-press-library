// Hand-authored — NOT generated. The `watch` command: a local watchlist of
// vessels under active due diligence — pin/unpin, refresh (re-fetch behavior),
// and since (recent activity window). Wired from root.go via newWatchParentCmd.
//
// pp:data-source auto
// pin/unpin/since are local-only (work against the SQLite watchlist table);
// refresh goes live to GFW to re-fetch identity and events for pinned vessels.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/gfw/internal/store"

	"github.com/spf13/cobra"
)

func newWatchParentCmd(flags *rootFlags) *cobra.Command {
	cmd := newNovelWatchCmd(flags)
	cmd.Short = "Watchlist of vessels under active due diligence (local)."
	cmd.Long = "Pin vessels to a local watchlist, then 'watch refresh' to re-fetch their behavior and 'watch since <dur>' to see recent activity. 'watch pin --list' shows the watchlist."
	cmd.ResetCommands()
	cmd.AddCommand(newWatchPinCmd(flags))
	cmd.AddCommand(newWatchUnpinCmd(flags))
	cmd.AddCommand(newWatchRefreshCmd(flags))
	cmd.AddCommand(newWatchSinceCmd(flags))
	return cmd
}

// parseWatchWindow accepts day-suffixed windows ("7d", "30d") plus Go
// durations ("24h"). Empty defaults to 7 days. Pure + testable.
func parseWatchWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 7 * 24 * time.Hour, nil
	}
	if rest, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(rest)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid duration %q: use forms like 7d, 24h", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid duration %q: use forms like 7d, 24h", s)
	}
	return d, nil
}

func openWriteStore(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("gfw-pp-cli"))
}

// --- pin / --list ---

func newWatchPinCmd(flags *rootFlags) *cobra.Command {
	var label string
	var list bool
	cmd := &cobra.Command{
		Use:         "pin <vesselId>",
		Short:       "Pin a vessel to the watchlist (or --list the watchlist).",
		Long:        "Adds a GFW vessel id to the local watchlist (re-pinning updates the label). 'watch pin --list' shows the watchlist; 'watch refresh' brings pinned vessels current.",
		Example:     "  gfw-pp-cli watch pin 8c7304226-6c71-edbe-0b63-c246734b3c01 --label \"Lagos deal\"\n  gfw-pp-cli watch pin --list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				if dryRunOK(flags) {
					return nil
				}
				db, err := openStoreForRead(cmd.Context(), "gfw-pp-cli")
				if err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				if db == nil {
					return printJSONFiltered(cmd.OutOrStdout(), []store.PinRow{}, flags)
				}
				defer db.Close()
				pins, err := db.ListPins()
				if err != nil {
					return fmt.Errorf("reading watchlist: %w", err)
				}
				return printJSONFiltered(cmd.OutOrStdout(), pins, flags)
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := resolveVesselID(args)
			if err != nil {
				return err
			}
			db, err := openWriteStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.PinVessel(id, label); err != nil {
				return fmt.Errorf("pinning %s: %w", id, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "pinned", "vessel_id": id, "label": label}, flags)
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Optional label (e.g. a case or deal name)")
	cmd.Flags().BoolVar(&list, "list", false, "List the watchlist instead of pinning")
	return cmd
}

// --- unpin ---

func newWatchUnpinCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "unpin <vesselId>",
		Short:       "Remove a vessel from the watchlist.",
		Example:     "  gfw-pp-cli watch unpin 8c7304226-6c71-edbe-0b63-c246734b3c01",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := resolveVesselID(args)
			if err != nil {
				return err
			}
			db, err := openWriteStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			removed, err := db.UnpinVessel(id)
			if err != nil {
				return fmt.Errorf("unpinning %s: %w", id, err)
			}
			status := "unpinned"
			if !removed {
				status = "not_pinned"
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": status, "vessel_id": id}, flags)
		},
	}
	return cmd
}

// --- watch result + shared iterate ---

type watchResult struct {
	VesselID     string            `json:"vessel_id"`
	Status       string            `json:"status"`
	Name         string            `json:"name,omitempty"`
	Flag         string            `json:"flag,omitempty"`
	EventCounts  map[string]int    `json:"event_counts,omitempty"`
	RecentEvents []json.RawMessage `json:"recent_events,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// --- refresh ---

func newWatchRefreshCmd(flags *rootFlags) *cobra.Command {
	var pinned bool
	var throttle time.Duration
	var eventLimit int
	cmd := &cobra.Command{
		Use:         "refresh",
		Short:       "Re-fetch identity + recent events for every watchlisted vessel.",
		Long:        "Iterates the watchlist, re-fetches each vessel's identity (refreshing the cache) and recent event counts, under a polite throttle.",
		Example:     "  gfw-pp-cli watch refresh --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = pinned // watchlist is always the pinned set; flag kept for the documented example
			if dryRunOK(flags) {
				return nil
			}
			ids, err := watchlistIDs(cmd.Context())
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []watchResult{}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results := make([]watchResult, 0, len(ids))
			for i, id := range ids {
				if i > 0 {
					if !sleepCtx(cmd.Context(), throttle) {
						break
					}
				}
				r := watchResult{VesselID: id, Status: "ok"}
				if vraw, verr := fetchVesselByID(cmd.Context(), c, id); verr == nil {
					vi := extractVesselIdentity(vraw, id)
					r.Name, r.Flag = vi.Name, vi.Flag
					_ = cacheVesselIdentity(cmd.Context(), flags, vi)
				}
				if eraw, eerr := fetchEvents(cmd.Context(), c, id, allEventDatasets, eventLimit, ""); eerr == nil {
					r.EventCounts = countEventTypes(entriesOf(eraw))
				} else {
					r.Status = "error"
					r.Error = eerr.Error()
				}
				results = append(results, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().BoolVar(&pinned, "pinned", true, "Refresh watchlisted (pinned) vessels")
	cmd.Flags().DurationVar(&throttle, "throttle", 300*time.Millisecond, "Delay between vessels (politeness)")
	cmd.Flags().IntVar(&eventLimit, "event-limit", 50, "Max events to fetch per vessel")
	return cmd
}

// --- since ---

func newWatchSinceCmd(flags *rootFlags) *cobra.Command {
	var throttle time.Duration
	var eventLimit int
	cmd := &cobra.Command{
		Use:         "since <duration>",
		Short:       "Recent events for watchlisted vessels within a window (e.g. 7d).",
		Long:        "For each watchlisted vessel, fetches events whose start date falls within the given window (e.g. '7d', '24h'). Use for 'what happened to my vessels in the last N days'.",
		Example:     "  gfw-pp-cli watch since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			dur, err := parseWatchWindow(args[0])
			if err != nil {
				return usageErr(err)
			}
			sinceISO := time.Now().UTC().Add(-dur).Format("2006-01-02")
			ids, err := watchlistIDs(cmd.Context())
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []watchResult{}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results := make([]watchResult, 0, len(ids))
			for i, id := range ids {
				if i > 0 {
					if !sleepCtx(cmd.Context(), throttle) {
						break
					}
				}
				r := watchResult{VesselID: id, Status: "ok"}
				if eraw, eerr := fetchEvents(cmd.Context(), c, id, allEventDatasets, eventLimit, sinceISO); eerr == nil {
					entries := entriesOf(eraw)
					r.EventCounts = countEventTypes(entries)
					r.RecentEvents = entries
				} else {
					r.Status = "error"
					r.Error = eerr.Error()
				}
				results = append(results, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().DurationVar(&throttle, "throttle", 300*time.Millisecond, "Delay between vessels (politeness)")
	cmd.Flags().IntVar(&eventLimit, "event-limit", 50, "Max events to fetch per vessel")
	return cmd
}

// watchlistIDs returns the pinned vessel ids, or an empty slice if no store yet.
func watchlistIDs(ctx context.Context) ([]string, error) {
	db, err := openStoreForRead(ctx, "gfw-pp-cli")
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if db == nil {
		return nil, nil
	}
	defer db.Close()
	return db.PinnedVesselIDs()
}

// sleepCtx sleeps d, returning false if the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}
