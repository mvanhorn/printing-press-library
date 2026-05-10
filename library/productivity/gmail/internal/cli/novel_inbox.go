package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"

	"github.com/spf13/cobra"
)

type inboxAgeBucket struct {
	Bucket     string `json:"bucket"`
	Count      int    `json:"count"`
	OldestDate string `json:"oldest_date"`
	NewestDate string `json:"newest_date"`
	Label      string `json:"label"`
}

var ageBucketOrder = []string{"today", "1-7d", "8-30d", "30-90d", "90d+"}

func newInboxCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Analyze your inbox from local store",
	}
	cmd.AddCommand(newInboxAgeCmd(flags))
	return cmd
}

func newInboxAgeCmd(flags *rootFlags) *cobra.Command {
	var label string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "age",
		Short: "See how old your unread mail is — bucketed by today / 1-7d / 8-30d / 30-90d / 90d+ per label",
		Long: `Queries the local SQLite store for unread messages and buckets them by age:
  today   Messages from the last 24 hours
  1-7d    Messages from 1 to 7 days ago
  8-30d   Messages from 8 to 30 days ago
  30-90d  Messages from 30 to 90 days ago
  90d+    Messages older than 90 days

No live API call is needed after sync. Run before a bulk-archive session
to understand the shape of your unread pile.`,
		Example: `  gmail-pp-cli inbox age
  gmail-pp-cli inbox age --label INBOX
  gmail-pp-cli inbox age --agent --select bucket,count,oldest_date`,
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

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(data,''), COALESCE(internal_date,'') FROM messages`)
			if err != nil {
				return fmt.Errorf("querying messages: %w", err)
			}
			defer rows.Close()

			// label → bucket → {count, oldest, newest}
			type bucketStats struct {
				count      int
				oldestDate time.Time
				newestDate time.Time
			}
			// key: label|bucket
			type statsKey struct{ label, bucket string }
			stats := map[statsKey]*bucketStats{}

			for rows.Next() {
				var dataJSON, internalDate string
				if err := rows.Scan(&dataJSON, &internalDate); err != nil || dataJSON == "" {
					continue
				}
				msg, err := parseGmailMsg(dataJSON)
				if err != nil {
					continue
				}
				if !msg.hasLabel("UNREAD") {
					continue
				}
				t := msg.internalTime()
				if t.IsZero() {
					continue
				}
				bucket := ageBucket(t)

				for _, lbl := range msg.LabelIDs {
					if lbl == "UNREAD" {
						continue
					}
					if label != "" && lbl != label {
						continue
					}
					key := statsKey{label: lbl, bucket: bucket}
					if s, ok := stats[key]; ok {
						s.count++
						if t.Before(s.oldestDate) {
							s.oldestDate = t
						}
						if t.After(s.newestDate) {
							s.newestDate = t
						}
					} else {
						stats[key] = &bucketStats{count: 1, oldestDate: t, newestDate: t}
					}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading messages: %w", err)
			}

			var result []inboxAgeBucket
			for k, s := range stats {
				result = append(result, inboxAgeBucket{
					Bucket:     k.bucket,
					Count:      s.count,
					OldestDate: s.oldestDate.Format("2006-01-02"),
					NewestDate: s.newestDate.Format("2006-01-02"),
					Label:      k.label,
				})
			}
			// Sort by label then bucket order
			bucketIdx := map[string]int{}
			for i, b := range ageBucketOrder {
				bucketIdx[b] = i
			}
			sort.Slice(result, func(i, j int) bool {
				if result[i].Label != result[j].Label {
					return result[i].Label < result[j].Label
				}
				return bucketIdx[result[i].Bucket] < bucketIdx[result[j].Bucket]
			})

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(result) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No unread messages found. Run 'gmail-pp-cli sync --full' to populate the local store.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "LABEL\tBUCKET\tCOUNT\tOLDEST\tNEWEST")
			for _, r := range result {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", r.Label, r.Bucket, r.Count, r.OldestDate, r.NewestDate)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Filter to a specific Gmail label (e.g. INBOX, IMPORTANT)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
