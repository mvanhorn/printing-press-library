// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for retraction-checker-pp-cli.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

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
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "retraction-checker-pp-cli", "watch")
	return dir, os.MkdirAll(dir, 0o755)
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
func loadWatchBaseline(path string, ttlDays int) watchBaseline {
	data, err := os.ReadFile(path)
	if err != nil {
		return watchBaseline{Seen: []SeenEntry{}}
	}

	// First, parse the top-level fields (query, updated_at, and raw seen).
	var raw struct {
		Query     string          `json:"query"`
		UpdatedAt string          `json:"updated_at"`
		Seen      json.RawMessage `json:"seen"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return watchBaseline{Seen: []SeenEntry{}}
	}

	// If "seen" is missing, null, or empty array, return empty.
	if len(raw.Seen) == 0 || string(raw.Seen) == "null" || string(raw.Seen) == "[]" {
		return watchBaseline{
			Query:     raw.Query,
			UpdatedAt: raw.UpdatedAt,
			Seen:      []SeenEntry{},
		}
	}

	// Try new format: []SeenEntry
	var newSeen []SeenEntry
	if err := json.Unmarshal(raw.Seen, &newSeen); err == nil {
		active, _ := pruneSeenEntries(newSeen, ttlDays)
		return watchBaseline{
			Query:     raw.Query,
			UpdatedAt: raw.UpdatedAt,
			Seen:      active,
		}
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
		}
	}

	// If all parsing fails, return empty baseline.
	return watchBaseline{Seen: []SeenEntry{}}
}

// saveWatchBaseline writes the baseline to disk after pruning with the given TTL.
// It sets UpdatedAt to the current UTC time, sorts entries by DOI for deterministic JSON,
// and prunes old entries.
//
// Note: this function mutates the provided watchBaseline (it sets UpdatedAt and
// may reduce the length of Seen). Callers should not rely on the original content
// after calling saveWatchBaseline.
func saveWatchBaseline(path string, b watchBaseline, ttlDays int) error {
	// Prune old entries.
	active, _ := pruneSeenEntries(b.Seen, ttlDays)
	b.Seen = active

	// Set the update timestamp.
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Sort by DOI for stable, deterministic output.
	sort.Slice(b.Seen, func(i, j int) bool {
		return b.Seen[i].DOI < b.Seen[j].DOI
	})

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ---------- API fetch (uses existing client) ----------

// fetchRetractionNotices retrieves recent retraction notices from Crossref.
// It uses the project's existing HTTP client and types.
func fetchRetractionNotices(cmd *cobra.Command, flags *rootFlags, mailto, query string, rows int) ([]watchNotice, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"filter": "update-type:retraction",
		"sort":   "updated",
		"order":  "desc",
		"rows":   fmt.Sprintf("%d", rows),
		"select": "DOI,title,update-to",
	}
	if query != "" {
		params["query"] = query
	}
	if mailto != "" {
		params["mailto"] = mailto
	}
	raw, err := c.Get(ctx, "/works", params)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Message struct {
			Items []crossrefWorkMessage `json:"items"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	notices := make([]watchNotice, 0, len(envelope.Message.Items))
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
	return notices, nil
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
			"baseline and reports nothing as new. Use --reset to clear the stored baseline.\n" +
			"State is kept under your user config directory. Keyless.\n" +
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
			query := args[0]
			path, err := watchPath(query)
			if err != nil {
				return err
			}
			if reset {
				_ = os.Remove(path)
			}

			// Load baseline with TTL pruning (internal constant).
			base := loadWatchBaseline(path, watchTTLDays)
			firstRun := base.UpdatedAt == ""

			// Fetch fresh notices from Crossref.
			notices, err := fetchRetractionNotices(cmd, flags, mailto, query, rows)
			if err != nil {
				return classifyAPIError(err, flags)
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

			// Identify new notices (those not in the baseline).
			newNotices := []watchNotice{}
			for _, n := range notices {
				if _, ok := baselineSet[n.DOI]; !ok {
					newNotices = append(newNotices, n)
				}
			}

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
				TrackedTotal: len(notices),
			}
			if firstRun {
				out.Note = fmt.Sprintf("baseline established with %d notices; new retractions will be reported on the next run", len(notices))
			}

			// Save the updated baseline – prune, set UpdatedAt, sort.
			if err := saveWatchBaseline(path, watchBaseline{
				Query: query,
				Seen:  seenSlice,
				// UpdatedAt will be set inside saveWatchBaseline.
			}, watchTTLDays); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not save watch baseline: %v\n", err)
			}

			// Output.
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if firstRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Baseline established for %q: %d notices tracked.\n", query, out.TrackedTotal)
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
	cmd.Flags().IntVar(&rows, "rows", 50, "Number of recent retraction notices to track")
	cmd.Flags().BoolVar(&reset, "reset", false, "Clear the stored baseline for this topic before running")

	return cmd
}
