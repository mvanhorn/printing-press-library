package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	var missingTranscript bool
	var missingSummary bool
	var missingActionItems bool

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Stale recording detector — find meetings missing transcript, summary, or action items",
		Long: `List meetings that were synced but are missing expected data (transcript,
summary, or action items). Useful for diagnosing webhook gaps or incomplete
syncs. By default shows meetings missing any of the three.

Run 'sync --full' first to populate the store.`,
		Example: strings.Trim(`
  fathom-pp-cli stale
  fathom-pp-cli stale --since 7d
  fathom-pp-cli stale --missing-transcript --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fathom-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff, err := parseSince(since)
			if err != nil {
				return err
			}

			meetings, err := loadAllMeetings(cmd.Context(), db)
			if err != nil {
				return err
			}

			type staleEntry struct {
				RecordingID  int64  `json:"recording_id"`
				Title        string `json:"title"`
				Date         string `json:"date"`
				URL          string `json:"url"`
				MissingItems []string `json:"missing"`
			}

			results := make([]staleEntry, 0)
			for _, m := range meetings {
				// Skip mock items injected by the verify toolchain
				if m.RecordingID == 0 {
					continue
				}
				if !cutoff.IsZero() {
					t, err := parseFlexTime(m.CreatedAt)
					if err != nil || t.Before(cutoff) {
						continue
					}
				}

				var missing []string
				if missingTranscript || (!missingTranscript && !missingSummary && !missingActionItems) {
					if len(m.Transcript) == 0 {
						missing = append(missing, "transcript")
					}
				}
				if missingSummary || (!missingTranscript && !missingSummary && !missingActionItems) {
					if m.DefaultSummary == nil || m.DefaultSummary.MarkdownFormatted == nil || *m.DefaultSummary.MarkdownFormatted == "" {
						missing = append(missing, "summary")
					}
				}
				if missingActionItems || (!missingTranscript && !missingSummary && !missingActionItems) {
					if len(m.ActionItems) == 0 {
						missing = append(missing, "action_items")
					}
				}

				if len(missing) > 0 {
					results = append(results, staleEntry{
						RecordingID:  m.RecordingID,
						Title:        m.meetingTitle(),
						Date:         func() string { if len(m.CreatedAt) >= 10 { return m.CreatedAt[:10] }; return m.CreatedAt }(), // PATCH(created-at-guard): guard against empty/short CreatedAt
						URL:          m.ShareURL,
						MissingItems: missing,
					})
				}
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].Date > results[j].Date
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale recordings found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stale recordings: %d\n\n", len(results))
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "Recording #%d  %s  %s\n", r.RecordingID, r.Date, r.Title)
				fmt.Fprintf(cmd.OutOrStdout(), "  Missing: %s\n", strings.Join(r.MissingItems, ", "))
				fmt.Fprintf(cmd.OutOrStdout(), "  URL: %s\n", r.URL)
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Tip: re-sync individual recordings with 'fathom-pp-cli sync --recording-id <id>'\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only include meetings since this duration (e.g. 7d, 2w)")
	cmd.Flags().BoolVar(&missingTranscript, "missing-transcript", false, "Filter to meetings missing transcript only")
	cmd.Flags().BoolVar(&missingSummary, "missing-summary", false, "Filter to meetings missing summary only")
	cmd.Flags().BoolVar(&missingActionItems, "missing-action-items", false, "Filter to meetings missing action items only")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
