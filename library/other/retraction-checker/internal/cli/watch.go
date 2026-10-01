// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for retraction-checker-pp-cli.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/retraction-checker/internal/cliutil"
	"github.com/spf13/cobra"
)

// ---------- Constants ----------

// watchTTLDays defines how many days a seen entry is kept before being pruned.
// 365 days is chosen because retractions are rare and we want to avoid
// re‑reporting the same retraction if the user runs the command infrequently.
const watchTTLDays = 365

// ---------- Types ----------

// watchNotice represents a single retraction notice from Crossref.
type watchNotice struct {
	DOI         string `json:"doi"`
	Title       string `json:"title,omitempty"`
	RetractedTo string `json:"retracted_doi,omitempty"`
	Date        string `json:"date,omitempty"`
}

// SeenEntry stores a DOI and the last time it was seen as a Unix timestamp.
// Using int64 keeps the on‑disk format compact and ensures backward compatibility.
type SeenEntry struct {
	DOI      string `json:"doi"`
	LastSeen int64  `json:"last_seen"` // Unix seconds
}

// watchBaseline holds the persistent state for a given topic query.
type watchBaseline struct {
	Query     string      `json:"query"`
	UpdatedAt string      `json:"updated_at"` // RFC3339 timestamp
	Seen      []SeenEntry `json:"seen"`       // changed from []string
}

// watchOutput is the JSON output structure for the watch command.
type watchOutput struct {
	Query        string        `json:"query"`
	FirstRun     bool          `json:"first_run"`
	BaselineDate string        `json:"baseline_date,omitempty"`
	NewCount     int           `json:"new_count"`
	TrackedTotal int           `json:"tracked_total"`
	New          []watchNotice `json:"new"`
	Note         string        `json:"note,omitempty"`
}

// ---------- Path helpers ----------

// watchDir returns the directory where watch state files are stored.
func watchDir() (string, error) {
	base, err := cliutil.StateDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "watch")
	return dir, os.MkdirAll(dir, 0o700)
}

// watchPath generates the state file path for a given query.
func watchPath(query string) (string, error) {
	dir, err := watchDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(query))
	return filepath.Join(dir, hex.EncodeToString(sum[:8])+".json"), nil
}

// Earlier releases kept watch state under the user config directory even when
// a state override was set. Import that checkpoint into the new state path so
// an upgrade does not skip notices for an existing watch.
func legacyWatchPath(query string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir() // Match the earlier watchDir fallback.
	}
	sum := sha256.Sum256([]byte(query))
	return filepath.Join(configDir, "retraction-checker-pp-cli", "watch", hex.EncodeToString(sum[:8])+".json"), nil
}

// ---------- Pruning logic ----------

// pruneSeenEntries removes entries older than the given TTL (in days).
// Returns the active slice and the number of removed entries.
func pruneSeenEntries(entries []SeenEntry, ttlDays int) ([]SeenEntry, int) {
	if len(entries) == 0 {
		return entries, 0
	}
	cutoff := time.Now().AddDate(0, 0, -ttlDays).Unix()
	active := make([]SeenEntry, 0, len(entries))
	removed := 0
	for _, e := range entries {
		if e.LastSeen >= cutoff {
			active = append(active, e)
		} else {
			removed++
		}
	}
	return active, removed
}

// ---------- Load / Save ----------

// loadWatchBaseline reads the baseline file and prunes it using the given TTL.
// It detects the format by trying to unmarshal as []SeenEntry first,
// then falling back to []string (old format) – both via the standard JSON parser.
func loadWatchBaseline(path, legacy string, ttlDays int) (watchBaseline, error) {
	data, source, err := cliutil.ReadFileWithLegacyFallback(path, legacy)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return watchBaseline{Seen: []SeenEntry{}}, nil
		}
		return watchBaseline{}, err
	}

	// First, parse the top-level fields (query, updated_at, and raw seen).
	var raw struct {
		Query     string          `json:"query"`
		UpdatedAt string          `json:"updated_at"`
		Seen      json.RawMessage `json:"seen"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return watchBaseline{}, fmt.Errorf("decoding watch baseline %s: %w", source, err)
	}
	if _, err := time.Parse(time.RFC3339, raw.UpdatedAt); err != nil {
		return watchBaseline{}, fmt.Errorf("invalid watch baseline time in %s: %w", source, err)
	}

	// If "seen" is missing, null, or empty array, return empty.
	if len(raw.Seen) == 0 || string(raw.Seen) == "null" || string(raw.Seen) == "[]" {
		return watchBaseline{
			Query:     raw.Query,
			UpdatedAt: raw.UpdatedAt,
			Seen:      []SeenEntry{},
		}, nil
	}

	// Try new format: []SeenEntry
	var newSeen []SeenEntry
	if err := json.Unmarshal(raw.Seen, &newSeen); err == nil {
		active, _ := pruneSeenEntries(newSeen, ttlDays)
		return watchBaseline{
			Query:     raw.Query,
			UpdatedAt: raw.UpdatedAt,
			Seen:      active,
		}, nil
	}

	// Fallback: old format – []string
	var oldSeen []string
	if err := json.Unmarshal(raw.Seen, &oldSeen); err == nil {
		// Migrate: use UpdatedAt or current time as LastSeen (Unix timestamp).
		var t time.Time
		if raw.UpdatedAt != "" {
			t, _ = time.Parse(time.RFC3339, raw.UpdatedAt)
		}
		if t.IsZero() {
			t = time.Now()
		}
		now := t.Unix()
		seen := make([]SeenEntry, 0, len(oldSeen))
		for _, doi := range oldSeen {
			seen = append(seen, SeenEntry{DOI: doi, LastSeen: now})
		}
		active, _ := pruneSeenEntries(seen, ttlDays)
		return watchBaseline{
			Query:     raw.Query,
			UpdatedAt: raw.UpdatedAt,
			Seen:      active,
		}, nil
	}

	return watchBaseline{}, fmt.Errorf("invalid watch entries in %s", source)
}

// saveWatchBaseline writes the baseline to disk after pruning with the given TTL.
// It stores the polling start time, sorts entries by DOI for deterministic JSON,
// and prunes old entries.
func saveWatchBaseline(path string, b watchBaseline, ttlDays int) error {
	if _, err := time.Parse(time.RFC3339, b.UpdatedAt); err != nil {
		return fmt.Errorf("invalid watch checkpoint time: %w", err)
	}
	// Prune old entries.
	active, _ := pruneSeenEntries(b.Seen, ttlDays)
	b.Seen = active

	// Sort by DOI for stable, deterministic output.
	sort.Slice(b.Seen, func(i, j int) bool {
		return b.Seen[i].DOI < b.Seen[j].DOI
	})

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return cliutil.AtomicWritePrivateFile(path, data, 0o600, 0o700)
}

// ---------- API fetch (uses existing client) ----------

// fetchRetractionNotices retrieves recent retraction notices from Crossref.
// It uses the project's existing HTTP client and types.
func fetchRetractionNotices(cmd *cobra.Command, flags *rootFlags, mailto, query string, rows int, since, until time.Time) ([]watchNotice, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return fetchRetractionNoticesFrom(ctx, c, mailto, query, rows, since, until)
}

func fetchRetractionNoticesFrom(ctx context.Context, c crossrefGetter, mailto, query string, rows int, since, until time.Time) ([]watchNotice, error) {
	if rows < 1 {
		return nil, fmt.Errorf("rows must be at least 1")
	}
	if since.IsZero() || until.IsZero() || since.After(until) {
		return nil, fmt.Errorf("watch index-date window is invalid")
	}
	// Index date includes Crossref's outside sources, including Retraction
	// Watch. Bound both ends so the cursor result set stays fixed during polling.
	filters := []string{
		"update-type:retraction",
		"from-index-date:" + since.UTC().Format("2006-01-02T15:04:05"),
		"until-index-date:" + until.UTC().Format("2006-01-02T15:04:05"),
	}
	params := map[string]string{
		"filter": strings.Join(filters, ","),
		"rows":   fmt.Sprintf("%d", rows),
		"select": "DOI,title,update-to",
		"cursor": "*",
	}
	if query != "" {
		params["query"] = query
	}
	if mailto != "" {
		params["mailto"] = mailto
	}
	notices := make([]watchNotice, 0, rows)
	seenCursors := map[string]bool{"*": true}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := c.Get(ctx, "/works", params)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Message struct {
				Items      []crossrefWorkMessage `json:"items"`
				NextCursor string                `json:"next-cursor"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		for _, it := range envelope.Message.Items {
			n := watchNotice{DOI: it.DOI}
			if len(it.Title) > 0 {
				n.Title = it.Title[0]
			}
			if len(it.UpdateTo) > 0 {
				n.RetractedTo = it.UpdateTo[0].DOI
				n.Date = it.UpdateTo[0].Updated.iso()
			}
			notices = append(notices, n)
		}
		next := envelope.Message.NextCursor
		if next == "" || len(envelope.Message.Items) < rows {
			return notices, nil
		}
		if seenCursors[next] {
			return nil, fmt.Errorf("Crossref repeated a watch cursor")
		}
		seenCursors[next] = true
		params["cursor"] = next
	}
}

// ---------- Watch command ----------

// newNovelWatchCmd creates the "watch" subcommand.
func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		mailto string
		rows   int
		reset  bool
	)
	cmd := &cobra.Command{
		Use:   "watch <topic>",
		Short: "Monitor a topic or reading list for newly-announced retractions since the last run.",
		Long: "Persist a baseline of retraction notices for a topic and, on each subsequent run,\n" +
			"report notices that are new since the baseline. The first run establishes the\n" +
			"baseline without fetching historical notices. Use --reset to clear the stored baseline.\n" +
			"State is kept under the CLI state directory. Keyless.\n" +
			"Entries older than 365 days are automatically removed from the local watch state.",
		Example:     "  retraction-checker-pp-cli watch \"machine learning\" --json",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a topic argument is required"))
			}
			if rows < 1 || rows > 1000 {
				return usageErr(fmt.Errorf("--rows must be between 1 and 1000"))
			}
			query := args[0]
			path, err := watchPath(query)
			if err != nil {
				return err
			}
			legacy, err := legacyWatchPath(query)
			if err != nil {
				return err
			}
			if reset {
				for _, oldPath := range []string{path, legacy} {
					if oldPath == "" {
						continue
					}
					if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
						return fmt.Errorf("resetting watch baseline: %w", err)
					}
				}
			}

			// Load baseline with TTL pruning (internal constant).
			base, err := loadWatchBaseline(path, legacy, watchTTLDays)
			if err != nil {
				return err
			}
			firstRun := base.UpdatedAt == ""
			pollStarted := time.Now().UTC().Truncate(time.Second)

			// The first run starts a checkpoint without claiming to have read
			// all historical notices. Later runs page through a bounded index
			// date window, including updates from outside Crossref members.
			var notices []watchNotice
			if !firstRun {
				since, _ := time.Parse(time.RFC3339, base.UpdatedAt)
				notices, err = fetchRetractionNotices(cmd, flags, mailto, query, rows, since, pollStarted)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}

			// Build a set of DOIs from the baseline for quick membership tests.
			baselineSet := make(map[string]struct{}, len(base.Seen))
			for _, e := range base.Seen {
				baselineSet[e.DOI] = struct{}{}
			}

			// Build a map for saving: DOI -> LastSeen (merge baseline + new notices).
			seenMap := make(map[string]int64, len(base.Seen)+len(notices))
			for _, e := range base.Seen {
				seenMap[e.DOI] = e.LastSeen
			}
			now := time.Now().Unix()
			for _, n := range notices {
				seenMap[n.DOI] = now
			}

			// Identify new notices (those not in the baseline). The first run
			// establishes the baseline and must not alert on historical notices.
			newNotices := unseenWatchNotices(firstRun, baselineSet, notices)

			// Prepare the slice for saving.
			seenSlice := make([]SeenEntry, 0, len(seenMap))
			for doi, last := range seenMap {
				seenSlice = append(seenSlice, SeenEntry{DOI: doi, LastSeen: last})
			}

			// Build output.
			out := watchOutput{
				Query:        query,
				FirstRun:     firstRun,
				BaselineDate: base.UpdatedAt,
				New:          newNotices,
				NewCount:     len(newNotices),
				TrackedTotal: len(seenMap),
			}
			if firstRun {
				out.Note = "baseline established; new retractions will be reported on the next run"
			}

			// Save the checkpoint before reporting success. A failed save must
			// not make an alert look durably acknowledged.
			if err := saveWatchBaseline(path, watchBaseline{
				Query:     query,
				UpdatedAt: pollStarted.Format(time.RFC3339),
				Seen:      seenSlice,
			}, watchTTLDays); err != nil {
				return fmt.Errorf("saving watch baseline: %w", err)
			}

			// Output.
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if firstRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Baseline established for %q; new notices will be checked on the next run.\n", query)
				return nil
			}
			if out.NewCount == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No new retractions for %q since %s.\n", query, base.UpdatedAt)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d new retraction(s) for %q:\n\n", out.NewCount, query)
			for _, n := range out.New {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n    %s\n", n.Date, n.Title, n.DOI)
			}
			return nil
		},
	}

	// Define flags.
	cmd.Flags().StringVar(&mailto, "mailto", "", "Contact email for the Crossref polite pool (better rate limits)")
	cmd.Flags().IntVar(&rows, "rows", 50, "Crossref page size when checking newly indexed retraction notices")
	cmd.Flags().BoolVar(&reset, "reset", false, "Clear the stored baseline for this topic before running")

	return cmd
}

func unseenWatchNotices(firstRun bool, baseline map[string]struct{}, notices []watchNotice) []watchNotice {
	unseen := []watchNotice{}
	if firstRun {
		return unseen
	}
	for _, notice := range notices {
		if _, ok := baseline[notice.DOI]; !ok {
			unseen = append(unseen, notice)
			baseline[notice.DOI] = struct{}{}
		}
	}
	return unseen
}
