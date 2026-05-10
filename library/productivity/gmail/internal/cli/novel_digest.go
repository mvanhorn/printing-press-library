package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"

	"github.com/spf13/cobra"
)

type digestThread struct {
	Label       string `json:"label"`
	From        string `json:"from"`
	Subject     string `json:"subject"`
	Snippet     string `json:"snippet"`
	UnreadCount int    `json:"unread_count"`
	LatestDate  string `json:"latest_date"`
	ThreadID    string `json:"thread_id"`
}

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var since string
	var label string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Your daily inbox summary — threads grouped by label with sender, subject, and unread count",
		Long: `Queries the local SQLite store for unread messages grouped by label.
No live API call is needed after sync. Ideal for a morning inbox summary
piped to an agent or shell script.

The --since flag accepts duration strings: 24h, 7d, 1w, 30m.
Use --label to focus on a specific Gmail label (e.g. IMPORTANT, INBOX).`,
		Example: `  gmail-pp-cli digest --since yesterday
  gmail-pp-cli digest --since 24h --label IMPORTANT
  gmail-pp-cli digest --agent --select label,from,subject,snippet,unread_count`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gmail-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\n\nRun 'gmail-pp-cli sync --full' first", err)
			}
			defer db.Close()

			var sinceTime time.Time
			if since != "" && since != "yesterday" {
				sinceTime, err = parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
			} else if since == "yesterday" {
				sinceTime = time.Now().Add(-24 * time.Hour)
			}

			// Fetch all messages data for local parsing
			query := `SELECT COALESCE(data,''), COALESCE(snippet,''), COALESCE(thread_id,''), COALESCE(internal_date,'') FROM messages`
			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("querying messages: %w", err)
			}
			defer rows.Close()

			// label → map[threadID]digestThread
			type threadKey struct {
				label    string
				threadID string
			}
			threads := map[threadKey]*digestThread{}

			for rows.Next() {
				var dataJSON, snip, threadID, internalDate string
				if err := rows.Scan(&dataJSON, &snip, &threadID, &internalDate); err != nil {
					continue
				}
				if dataJSON == "" {
					continue
				}
				msg, err := parseGmailMsg(dataJSON)
				if err != nil {
					continue
				}

				// filter by time
				if !sinceTime.IsZero() {
					t := msg.internalTime()
					if t.IsZero() || t.Before(sinceTime) {
						continue
					}
				}

				// filter: must be unread
				if !msg.hasLabel("UNREAD") {
					continue
				}

				from := msg.header("From")
				subject := msg.header("Subject")
				t := msg.internalTime()

				for _, lbl := range msg.LabelIDs {
					if lbl == "UNREAD" {
						continue
					}
					if label != "" && lbl != label {
						continue
					}
					key := threadKey{label: lbl, threadID: threadID}
					if existing, ok := threads[key]; ok {
						existing.UnreadCount++
						if t.Format(time.RFC3339) > existing.LatestDate {
							existing.LatestDate = t.Format(time.RFC3339)
							existing.From = normalizeFrom(from)
							existing.Subject = subject
							existing.Snippet = snip
						}
					} else {
						threads[key] = &digestThread{
							Label:       lbl,
							From:        normalizeFrom(from),
							Subject:     subject,
							Snippet:     snip,
							UnreadCount: 1,
							LatestDate:  t.Format(time.RFC3339),
							ThreadID:    threadID,
						}
					}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading messages: %w", err)
			}

			// Flatten and sort: by label then latest_date desc
			var result []digestThread
			for _, dt := range threads {
				result = append(result, *dt)
			}
			sort.Slice(result, func(i, j int) bool {
				if result[i].Label != result[j].Label {
					return result[i].Label < result[j].Label
				}
				return result[i].LatestDate > result[j].LatestDate
			})
			if limit > 0 && len(result) > limit {
				result = result[:limit]
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			if len(result) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No unread messages found. Run 'gmail-pp-cli sync' to update local store.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "LABEL\tUNREAD\tFROM\tSUBJECT\tDATE")
			for _, dt := range result {
				subj := dt.Subject
				if len(subj) > 50 {
					subj = subj[:47] + "..."
				}
				from := dt.From
				if len(from) > 35 {
					from = from[:32] + "..."
				}
				date := dt.LatestDate
				if len(date) > 10 {
					date = date[:10]
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", dt.Label, dt.UnreadCount, from, subj, date)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Only include messages newer than this duration (e.g. 24h, 7d, 1w) or 'yesterday'")
	cmd.Flags().StringVar(&label, "label", "", "Filter to a specific Gmail label (e.g. IMPORTANT, INBOX)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of thread rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
