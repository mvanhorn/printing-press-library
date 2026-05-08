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
	var dbPath string

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Show files added since your last sync",
		Long: `Show files that were added to the local store since your last sync,
or within a specified time period. Useful for tracking new releases.`,
		Example: `  # Show files added since last sync
  ufo-goat-pp-cli new

  # Show files from the last 7 days
  ufo-goat-pp-cli new --since 7d

  # Show files from the last 24 hours
  ufo-goat-pp-cli new --since 24h`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// Determine the "since" timestamp
			var sinceTime time.Time
			if since != "" {
				t, err := parseNewSinceDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since value %q: %w", since, err)
				}
				sinceTime = t
			} else {
				// Default: show files from the last sync cycle
				// Use the previous sync timestamp minus a small buffer
				_, lastSynced, _, _ := db.GetSyncState("files")
				if lastSynced.IsZero() {
					// No previous sync, show everything from last 24h
					sinceTime = time.Now().Add(-24 * time.Hour)
				} else {
					// Show files from before the latest sync
					sinceTime = lastSynced.Add(-1 * time.Minute)
				}
			}

			files, err := db.GetNewFiles(sinceTime)
			if err != nil {
				return err
			}

			if len(files) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No new files since last check.")
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
			fmt.Fprintf(w, "%d new files since %s\n\n", len(files), sinceTime.Format("2006-01-02 15:04"))

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

	cmd.Flags().StringVar(&since, "since", "", "Show files newer than this duration (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
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
