// New command — show files added since last sync.
package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newNewFilesCmd(flags *rootFlags) *cobra.Command {
	var since string
	var sinceSync bool
	var releaseBatch int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Show files from the latest release tranche (or a chosen one)",
		Long: `Show the declassified files in the most recent PURSUE release tranche.

The government publishes files in batches (release_1, release_2, ...). By
default this command shows everything in the highest-numbered tranche present
locally — the real "what just dropped" view. Override the tranche with
--release N, or fall back to sync-timing semantics with --since / --since-sync.

Run 'ufo-goat-pp-cli sync' first to populate the local store.`,
		Example: `  # Show files in the latest release tranche (default)
  ufo-goat-pp-cli new

  # Show files in a specific tranche
  ufo-goat-pp-cli new --release 2

  # Files added to the local store since your last sync (timing-based)
  ufo-goat-pp-cli new --since-sync

  # Files synced within a time window
  ufo-goat-pp-cli new --since 7d`,
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

			count, _ := db.GetFileCount()
			if count == 0 {
				return fmt.Errorf("no files in local store. Run 'ufo-goat-pp-cli sync' first")
			}

			var files []store.UFOFile
			var heading string

			switch {
			case since != "" || sinceSync:
				// Timing-based fallback: files synced after a timestamp.
				var sinceTime time.Time
				if since != "" {
					t, err := parseNewSinceDuration(since)
					if err != nil {
						return fmt.Errorf("invalid --since value %q: %w", since, err)
					}
					sinceTime = t
				} else {
					_, lastSynced, _, _ := db.GetSyncState("files")
					if lastSynced.IsZero() {
						sinceTime = time.Now().Add(-24 * time.Hour)
					} else {
						sinceTime = lastSynced.Add(-1 * time.Minute)
					}
				}
				files, err = db.GetNewFiles(sinceTime)
				if err != nil {
					return err
				}
				heading = fmt.Sprintf("%d files synced since %s", len(files), sinceTime.Format("2006-01-02 15:04"))

			default:
				// Batch mode (default): files in the chosen or latest tranche.
				batch := releaseBatch
				if batch == 0 {
					batch, err = db.GetMaxReleaseBatch()
					if err != nil {
						return fmt.Errorf("resolving latest release: %w", err)
					}
				}
				if batch == 0 {
					return fmt.Errorf("no release tranche information in local store. Run 'ufo-goat-pp-cli sync' to refresh")
				}
				files, err = db.ListUFOFiles(store.FileFilter{ReleaseBatch: batch})
				if err != nil {
					return err
				}
				heading = fmt.Sprintf("%d files in Release %d", len(files), batch)
			}

			if len(files) == 0 {
				if releaseBatch > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No files found in Release %d.\n", releaseBatch)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No new files since last check.")
				}
				return nil
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(files)
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Human output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n\n", heading)

			tw := newTabWriter(w)
			fmt.Fprintln(tw, strings.Join([]string{
				bold("ID"), bold("TITLE"), bold("TYPE"), bold("AGENCY"),
			}, "\t"))

			for _, f := range files {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
					f.ID[:8],
					truncate(f.Title, 50),
					f.Type,
					f.Agency,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&releaseBatch, "release", 0, "Show files in a specific release tranche number (default: latest)")
	cmd.Flags().BoolVar(&sinceSync, "since-sync", false, "Use sync-timing instead of tranche: files added to the store since your last sync")
	cmd.Flags().StringVar(&since, "since", "", "Use sync-timing instead of tranche: files synced within a duration (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override the synced SQLite store path (default: ~/.local/share/ufo-goat-pp-cli/data.db)")
	return cmd
}

func parseNewSinceDuration(s string) (time.Time, error) {
	re := regexp.MustCompile(`^(\d+)([dhwm])$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(s))
	if matches == nil {
		return time.Time{}, fmt.Errorf("expected format like 7d, 24h, 1w, or 30m")
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, err
	}

	now := time.Now()
	switch matches[2] {
	case "d":
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	case "h":
		return now.Add(-time.Duration(n) * time.Hour), nil
	case "w":
		return now.Add(-time.Duration(n) * 7 * 24 * time.Hour), nil
	case "m":
		return now.Add(-time.Duration(n) * time.Minute), nil
	default:
		return time.Time{}, fmt.Errorf("unknown unit %q", matches[2])
	}
}
