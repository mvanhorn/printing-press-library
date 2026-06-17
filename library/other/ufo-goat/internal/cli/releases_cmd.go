// Releases command — browse the PURSUE archive by government release tranche.
//
// The war.gov UAP archive is published in batches (release_1, release_2, ...).
// These commands make the tranche a first-class lens: list every batch, compare
// two of them, and check whether a brand-new batch has landed.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/manifest"
	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newReleasesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "releases",
		Short: "Browse the archive by government release tranche (batch)",
		Long: `List the PURSUE release tranches present in the local store.

The government declassifies files in batches (release_1, release_2, ...). Each
tranche has a release date and a mix of agencies and file types. This command
summarizes every tranche; use 'releases diff' to compare two and 'releases
check' to detect a newly-landed batch.

Run 'ufo-goat-pp-cli sync' first to populate the local store.`,
		Example: `  # Summarize every release tranche
  ufo-goat-pp-cli releases

  # Compare two tranches
  ufo-goat-pp-cli releases diff 1 2

  # Detect whether a new tranche has landed (cron-friendly)
  ufo-goat-pp-cli releases check`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("ufo-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ufo-goat-pp-cli sync' first.", err)
			}
			defer db.Close()
			_ = db.EnsureUFOSchema()

			if c, _ := db.GetFileCount(); c == 0 {
				return fmt.Errorf("no files in local store. Run 'ufo-goat-pp-cli sync' first")
			}

			releases, err := db.GetReleases()
			if err != nil {
				return err
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(releases)
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, strings.Join([]string{
				bold("RELEASE"), bold("DATE"), bold("FILES"), bold("BREAKDOWN"),
			}, "\t"))
			for _, r := range releases {
				label := fmt.Sprintf("Release %d", r.Batch)
				if r.Batch == 0 {
					label = "Unknown"
				}
				date := r.ReleaseDate
				if date == "" {
					date = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", label, date, r.FileCount, formatCountMap(r.Agencies))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the synced SQLite store path (default: ~/.local/share/ufo-goat-pp-cli/data.db)")
	cmd.AddCommand(newReleasesDiffCmd(flags))
	cmd.AddCommand(newReleasesCheckCmd(flags))
	return cmd
}

func newReleasesDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "diff <from> <to>",
		Short: "Compare the composition of two release tranches",
		Long: `Compare two PURSUE release tranches side by side: file counts and the
per-agency and per-type breakdown of each, plus the delta between them.`,
		Example: `  # Compare Release 1 and Release 2
  ufo-goat-pp-cli releases diff 1 2 --json`,
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			from, err := parseBatchArg(args[0])
			if err != nil {
				return err
			}
			to, err := parseBatchArg(args[1])
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("ufo-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ufo-goat-pp-cli sync' first.", err)
			}
			defer db.Close()
			_ = db.EnsureUFOSchema()

			releases, err := db.GetReleases()
			if err != nil {
				return err
			}
			byBatch := map[int]store.ReleaseSummary{}
			for _, r := range releases {
				byBatch[r.Batch] = r
			}
			fromR, okFrom := byBatch[from]
			toR, okTo := byBatch[to]
			if !okFrom {
				return fmt.Errorf("release %d not found in local store (have: %s)", from, formatBatchList(releaseBatchNumbers(releases)))
			}
			if !okTo {
				return fmt.Errorf("release %d not found in local store (have: %s)", to, formatBatchList(releaseBatchNumbers(releases)))
			}

			agencyDelta := diffCountMaps(fromR.Agencies, toR.Agencies)
			typeDelta := diffCountMaps(fromR.Types, toR.Types)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out := map[string]any{
					"from":         fromR,
					"to":           toR,
					"file_delta":   toR.FileCount - fromR.FileCount,
					"agency_delta": agencyDelta,
					"type_delta":   typeDelta,
				}
				data, _ := json.Marshal(out)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(data), true)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Release %d (%s, %d files)  →  Release %d (%s, %d files)\n",
				fromR.Batch, dashIfEmpty(fromR.ReleaseDate), fromR.FileCount,
				toR.Batch, dashIfEmpty(toR.ReleaseDate), toR.FileCount)
			fmt.Fprintf(w, "File count delta: %+d\n\n", toR.FileCount-fromR.FileCount)

			tw := newTabWriter(w)
			fmt.Fprintln(tw, strings.Join([]string{bold("AGENCY"), bold("REL " + strconv.Itoa(from)), bold("REL " + strconv.Itoa(to)), bold("Δ")}, "\t"))
			for _, k := range sortedUnionKeys(fromR.Agencies, toR.Agencies) {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%+d\n", k, fromR.Agencies[k], toR.Agencies[k], toR.Agencies[k]-fromR.Agencies[k])
			}
			tw.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the synced SQLite store path (default: ~/.local/share/ufo-goat-pp-cli/data.db)")
	return cmd
}

func newReleasesCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var noSync bool
	var exitCode bool
	var sourceName string
	var manifestURL string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Detect whether a new release tranche has landed",
		Long: `Fetch the latest manifest and report any release tranche that is newly
present since the last check. Designed for cron / scheduled agents: it prints
the new tranches (if any) and, with --exit-code, exits 3 when nothing is new so
a shell or scheduler can branch on it.

Use --no-sync to compare only against already-synced data without fetching.`,
		Example: `  # Sync and report any newly-landed tranche
  ufo-goat-pp-cli releases check

  # Cron usage: exit 3 when nothing new
  ufo-goat-pp-cli releases check --exit-code`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("ufo-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureUFOSchema(); err != nil {
				return fmt.Errorf("ensuring schema: %w", err)
			}

			var totalFiles int
			if !noSync {
				resolvedURL, _, rerr := manifest.ResolveSource(manifestURL, sourceName, "")
				if rerr != nil {
					return usageErr(rerr)
				}
				_, totalFiles, err = fetchAndStoreManifest(cmd.Context(), db, resolvedURL, false)
				if err != nil {
					return err
				}
			} else {
				totalFiles, _ = db.GetFileCount()
			}

			newBatches := detectNewBatches(db)
			latestBatch, _ := db.GetMaxReleaseBatch()

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(map[string]any{
					"event":        "release_check",
					"latest_batch": latestBatch,
					"new_batches":  newBatches,
					"total_files":  totalFiles,
					"has_new":      len(newBatches) > 0,
				})
				if err := printOutput(cmd.OutOrStdout(), json.RawMessage(data), true); err != nil {
					return err
				}
			} else if len(newBatches) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "🛸 New release tranche: %s (latest is Release %d, %d files total)\n",
					formatBatchList(newBatches), latestBatch, totalFiles)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "No new release tranche. Latest is Release %d (%d files).\n", latestBatch, totalFiles)
			}

			if exitCode && len(newBatches) == 0 {
				return &cliError{code: 3, err: fmt.Errorf("no new release tranche")}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the synced SQLite store path (default: ~/.local/share/ufo-goat-pp-cli/data.db)")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "Compare against already-synced data without fetching the manifest")
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "Exit 3 when no new tranche is found (for cron/scheduler branching)")
	cmd.Flags().StringVar(&sourceName, "source", "", "Named manifest source to check (see 'sources'). Env: UFO_SOURCE")
	cmd.Flags().StringVar(&manifestURL, "manifest-url", "", "Custom manifest CSV URL to check. Env: UFO_MANIFEST_URL")
	return cmd
}

// --- helpers ---

func parseBatchArg(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.ToLower(s), "release ")
	s = strings.TrimPrefix(s, "release_")
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid release number %q: expected a positive integer like 1 or 2", s)
	}
	return n, nil
}

func formatCountMap(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d %s", m[k], k))
	}
	return strings.Join(parts, ", ")
}

func diffCountMaps(from, to map[string]int) map[string]int {
	delta := map[string]int{}
	for _, k := range sortedUnionKeys(from, to) {
		d := to[k] - from[k]
		if d != 0 {
			delta[k] = d
		}
	}
	return delta
}

func sortedUnionKeys(a, b map[string]int) []string {
	seen := map[string]bool{}
	var keys []string
	for k := range a {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func releaseBatchNumbers(rs []store.ReleaseSummary) []int {
	out := make([]int, 0, len(rs))
	for _, r := range rs {
		if r.Batch > 0 {
			out = append(out, r.Batch)
		}
	}
	return out
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
