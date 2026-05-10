package cli

import (
	"fmt"
	"net/mail"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"

	"github.com/spf13/cobra"
)

type newsletterRow struct {
	Domain         string `json:"domain"`
	SenderEmail    string `json:"sender_email"`
	SenderName     string `json:"sender_name,omitempty"`
	MessageCount   int    `json:"message_count"`
	UnsubscribeURL string `json:"unsubscribe_url"`
}

func newNewslettersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "newsletters",
		Short: "Detect and list newsletter senders from local store",
	}
	cmd.AddCommand(newNewslettersListCmd(flags))
	return cmd
}

func newNewslettersListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Surface every sender with a List-Unsubscribe header grouped by domain",
		Long: `Scans the local SQLite message store for messages carrying a
List-Unsubscribe header (RFC 2369) and groups results by sender domain.
This is your actionable unsubscribe queue — run before a Friday cleanup.

No live API call is needed after sync.`,
		Example: `  gmail-pp-cli newsletters list
  gmail-pp-cli newsletters list --agent
  gmail-pp-cli newsletters list | jq '.[] | select(.message_count > 5) | .unsubscribe_url'`,
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

			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT COALESCE(data,'') FROM messages`)
			if err != nil {
				return fmt.Errorf("querying messages: %w", err)
			}
			defer rows.Close()

			type senderKey struct{ email, domain, unsub string }
			type agg struct {
				senderEmail string
				senderName  string
				domain      string
				unsubURL    string
				count       int
			}
			byKey := map[string]*agg{}

			for rows.Next() {
				var dataJSON string
				if err := rows.Scan(&dataJSON); err != nil || dataJSON == "" {
					continue
				}
				msg, err := parseGmailMsg(dataJSON)
				if err != nil {
					continue
				}
				unsub := msg.header("List-Unsubscribe")
				if unsub == "" {
					continue
				}
				from := msg.header("From")
				email, name, domain := parseFrom(from)
				unsubURL := cleanUnsubscribeURL(unsub)
				key := email + "|" + unsubURL
				if a, ok := byKey[key]; ok {
					a.count++
				} else {
					byKey[key] = &agg{
						senderEmail: email,
						senderName:  name,
						domain:      domain,
						unsubURL:    unsubURL,
						count:       1,
					}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading messages: %w", err)
			}

			var result []newsletterRow
			for _, a := range byKey {
				result = append(result, newsletterRow{
					Domain:         a.domain,
					SenderEmail:    a.senderEmail,
					SenderName:     a.senderName,
					MessageCount:   a.count,
					UnsubscribeURL: a.unsubURL,
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
				fmt.Fprintln(cmd.OutOrStdout(), "No newsletters detected. Run 'gmail-pp-cli sync --full' to populate the local store.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COUNT\tDOMAIN\tSENDER\tUNSUBSCRIBE URL")
			for _, r := range result {
				url := r.UnsubscribeURL
				if len(url) > 60 {
					url = url[:57] + "..."
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", r.MessageCount, r.Domain, r.SenderEmail, url)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum number of senders to return")
	return cmd
}

func parseFrom(from string) (email, name, domain string) {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		// simple fallback
		if i := strings.Index(from, "@"); i >= 0 {
			email = strings.ToLower(strings.TrimSpace(from))
			if j := strings.LastIndex(email, "@"); j >= 0 {
				domain = email[j+1:]
			}
		}
		return
	}
	email = strings.ToLower(addr.Address)
	name = addr.Name
	if i := strings.LastIndex(email, "@"); i >= 0 {
		domain = email[i+1:]
	}
	return
}
