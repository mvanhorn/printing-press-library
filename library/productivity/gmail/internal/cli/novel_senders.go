package cli

import (
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"

	"github.com/spf13/cobra"
)

type senderRow struct {
	Email          string `json:"email"`
	Name           string `json:"name"`
	MessageCount   int    `json:"message_count"`
	UnsubscribeURL string `json:"unsubscribe_url,omitempty"`
}

func newSendersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "senders",
		Short: "Analyze email senders from local store",
	}
	cmd.AddCommand(newSendersTopCmd(flags))
	return cmd
}

func newSendersTopCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var period string
	var showUnsubscribe bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Rank your top email senders by volume with unsubscribe link detection",
		Long: `Queries the local SQLite store to rank senders by message volume.
No live API call needed after sync.

The --period flag accepts duration strings: 7d, 30d, 1w, 24h.
Add --unsubscribe to show detected List-Unsubscribe URLs alongside each sender.`,
		Example: `  gmail-pp-cli senders top --limit 20
  gmail-pp-cli senders top --limit 20 --period 30d --unsubscribe
  gmail-pp-cli senders top --agent --select email,message_count,unsubscribe_url`,
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
			if period != "" {
				sinceTime, err = parseSinceDuration(period)
				if err != nil {
					return usageErr(fmt.Errorf("--period: %w", err))
				}
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(data,'') FROM messages`)
			if err != nil {
				return fmt.Errorf("querying messages: %w", err)
			}
			defer rows.Close()

			type senderAgg struct {
				name           string
				count          int
				unsubscribeURL string
			}
			byEmail := map[string]*senderAgg{}

			for rows.Next() {
				var dataJSON string
				if err := rows.Scan(&dataJSON); err != nil || dataJSON == "" {
					continue
				}
				msg, err := parseGmailMsg(dataJSON)
				if err != nil {
					continue
				}
				if !sinceTime.IsZero() {
					if t := msg.internalTime(); t.IsZero() || t.Before(sinceTime) {
						continue
					}
				}
				from := msg.header("From")
				if from == "" {
					continue
				}
				addr, parseErr := mail.ParseAddress(from)
				email := from
				name := ""
				if parseErr == nil {
					email = strings.ToLower(addr.Address)
					name = addr.Name
				}
				if agg, ok := byEmail[email]; ok {
					agg.count++
					if agg.unsubscribeURL == "" && showUnsubscribe {
						if u := msg.header("List-Unsubscribe"); u != "" {
							agg.unsubscribeURL = cleanUnsubscribeURL(u)
						}
					}
				} else {
					unsub := ""
					if showUnsubscribe {
						if u := msg.header("List-Unsubscribe"); u != "" {
							unsub = cleanUnsubscribeURL(u)
						}
					}
					byEmail[email] = &senderAgg{name: name, count: 1, unsubscribeURL: unsub}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading messages: %w", err)
			}

			var result []senderRow
			for email, agg := range byEmail {
				result = append(result, senderRow{
					Email:          email,
					Name:           agg.name,
					MessageCount:   agg.count,
					UnsubscribeURL: agg.unsubscribeURL,
				})
			}
			sort.Slice(result, func(i, j int) bool {
				return result[i].MessageCount > result[j].MessageCount
			})
			if limit > 0 && len(result) > limit {
				result = result[:limit]
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(result) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No messages found in local store. Run 'gmail-pp-cli sync --full' first.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			if showUnsubscribe {
				fmt.Fprintln(tw, "COUNT\tEMAIL\tNAME\tUNSUBSCRIBE URL")
				for _, r := range result {
					fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", r.MessageCount, r.Email, r.Name, r.UnsubscribeURL)
				}
			} else {
				fmt.Fprintln(tw, "COUNT\tEMAIL\tNAME")
				for _, r := range result {
					fmt.Fprintf(tw, "%d\t%s\t%s\n", r.MessageCount, r.Email, r.Name)
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of top senders to show")
	cmd.Flags().StringVar(&period, "period", "", "Only count messages from this period (e.g. 30d, 7d, 1w)")
	cmd.Flags().BoolVar(&showUnsubscribe, "unsubscribe", false, "Show detected List-Unsubscribe URL for each sender")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}

// cleanUnsubscribeURL extracts the first URL or mailto from a List-Unsubscribe header value.
// The header can contain comma-separated angle-bracket URLs like:
//
//	<https://example.com/unsub>, <mailto:unsub@example.com>
func cleanUnsubscribeURL(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "<") && strings.HasSuffix(part, ">") {
			return part[1 : len(part)-1]
		}
	}
	return header
}
