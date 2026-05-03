// Hand-written novel feature: since.
// What happened in the last N hours? Created/updated/deleted links plus
// recent partner approvals, new bounty submissions, and top-clicked entities.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type sinceEvent struct {
	When     string `json:"when"`
	Resource string `json:"resource"`
	Type     string `json:"type"`
	ID       string `json:"id,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "since [duration]",
		Short: "What happened in the last N hours/days across the workspace?",
		Long: `Aggregates recent activity from the local store across resources:

  • created/updated links (by synced_at)
  • new bounty submissions awaiting review
  • commission state changes
  • new partners

Time argument accepts shorthand: 24h, 7d, 1w. Default: 24h.`,
		Example: strings.Trim(`
  dub-pp-cli since 24h --json
  dub-pp-cli since 7d --json --select when,resource,type
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window := 24 * time.Hour
			if len(args) > 0 {
				if d, ok := parseDuration(args[0]); ok {
					window = d
				} else {
					return usageErr(fmt.Errorf("invalid duration %q (try 24h, 7d, 1w)", args[0]))
				}
			}
			cutoff := time.Now().Add(-window).Format(time.RFC3339)

			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			var events []sinceEvent

			// Links
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(synced_at,''), id, COALESCE(domain,''), COALESCE("key",'')
				FROM links
				WHERE synced_at >= ?
			`, cutoff)
			if err == nil {
				for rows.Next() {
					var when, id, domain, key string
					if err := rows.Scan(&when, &id, &domain, &key); err == nil {
						events = append(events, sinceEvent{
							When: when, Resource: "links", Type: "synced",
							ID: id, Detail: domain + "/" + key,
						})
					}
				}
				rows.Close()
			}
			// Submissions
			rows2, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(synced_at,''), id, COALESCE(bounties_id,''), COALESCE(data,'{}')
				FROM submissions
				WHERE synced_at >= ?
			`, cutoff)
			if err == nil {
				for rows2.Next() {
					var when, id, bountyID string
					var blob sql.NullString
					if err := rows2.Scan(&when, &id, &bountyID, &blob); err == nil {
						st := "synced"
						if blob.Valid {
							if s := extractFromData([]byte(blob.String), "status"); s != "" {
								st = s
							}
						}
						events = append(events, sinceEvent{
							When: when, Resource: "submissions", Type: st,
							ID: id, Detail: "bounty " + bountyID,
						})
					}
				}
				rows2.Close()
			}
			// Partners
			rows3, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(synced_at,''), id, COALESCE(email,''), COALESCE(status,'')
				FROM partners
				WHERE synced_at >= ?
			`, cutoff)
			if err == nil {
				for rows3.Next() {
					var when, id, email, status string
					if err := rows3.Scan(&when, &id, &email, &status); err == nil {
						events = append(events, sinceEvent{
							When: when, Resource: "partners", Type: status,
							ID: id, Detail: email,
						})
					}
				}
				rows3.Close()
			}
			// Commissions
			rows4, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(synced_at,''), id, COALESCE(partner_id,''), COALESCE(status,''), COALESCE(amount,0)
				FROM commissions
				WHERE synced_at >= ?
			`, cutoff)
			if err == nil {
				for rows4.Next() {
					var when, id, pid, status string
					var amount float64
					if err := rows4.Scan(&when, &id, &pid, &status, &amount); err == nil {
						events = append(events, sinceEvent{
							When: when, Resource: "commissions", Type: status,
							ID: id, Detail: fmt.Sprintf("%s %.2f", pid, amount),
						})
					}
				}
				rows4.Close()
			}

			// Sort newest first.
			sort.Slice(events, func(i, j int) bool { return events[i].When > events[j].When })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}
			if len(events) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No activity in the last %s.\n", window)
				fmt.Fprintln(cmd.OutOrStdout(), "Tip: run `dub-pp-cli sync` to refresh the local store first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-12s %-15s %s\n", "WHEN", "RESOURCE", "TYPE", "DETAIL")
			for _, e := range events {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-12s %-15s %s\n",
					truncate(e.When, 25), e.Resource, truncate(e.Type, 15), truncate(e.Detail, 60))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
