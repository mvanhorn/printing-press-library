package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"

	"github.com/spf13/cobra"
)

type sentStatsRow struct {
	ToDomain     string `json:"to_domain"`
	MessageCount int    `json:"message_count"`
	FirstSent    string `json:"first_sent"`
	LastSent     string `json:"last_sent"`
}

func newSentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sent",
		Short: "Analyze sent mail from local store",
	}
	cmd.AddCommand(newSentStatsCmd(flags))
	return cmd
}

func newSentStatsCmd(flags *rootFlags) *cobra.Command {
	var toDomain string
	var period string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Count outbound emails to a recipient domain in a time window — the one-command compliance audit",
		Long: `Queries the local SQLite SENT message store and counts outbound emails
grouped by recipient domain. No live API call is needed after sync.

Use --to-domain to filter to a specific recipient domain (e.g. acmecorp.com).
Use --period to restrict the time window (e.g. 7d, 30d).`,
		Example: `  gmail-pp-cli sent stats
  gmail-pp-cli sent stats --to-domain acmecorp.com --period 30d
  gmail-pp-cli sent stats --agent --select to_domain,message_count`,
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

			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT COALESCE(data,'') FROM messages`)
			if err != nil {
				return fmt.Errorf("querying messages: %w", err)
			}
			defer rows.Close()

			type domainAgg struct {
				count     int
				firstSent time.Time
				lastSent  time.Time
			}
			byDomain := map[string]*domainAgg{}

			for rows.Next() {
				var dataJSON string
				if err := rows.Scan(&dataJSON); err != nil || dataJSON == "" {
					continue
				}
				msg, err := parseGmailMsg(dataJSON)
				if err != nil {
					continue
				}
				// Must be in SENT label
				if !msg.hasLabel("SENT") {
					continue
				}
				if !sinceTime.IsZero() {
					if t := msg.internalTime(); t.IsZero() || t.Before(sinceTime) {
						continue
					}
				}
				toHeader := msg.header("To")
				if toHeader == "" {
					continue
				}
				t := msg.internalTime()
				for _, addr := range strings.Split(toHeader, ",") {
					domain := extractEmailDomain(strings.TrimSpace(addr))
					if domain == "" {
						continue
					}
					if toDomain != "" && !strings.EqualFold(domain, toDomain) {
						continue
					}
					if agg, ok := byDomain[domain]; ok {
						agg.count++
						if t.Before(agg.firstSent) {
							agg.firstSent = t
						}
						if t.After(agg.lastSent) {
							agg.lastSent = t
						}
					} else {
						byDomain[domain] = &domainAgg{count: 1, firstSent: t, lastSent: t}
					}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading messages: %w", err)
			}

			var result []sentStatsRow
			for domain, agg := range byDomain {
				result = append(result, sentStatsRow{
					ToDomain:     domain,
					MessageCount: agg.count,
					FirstSent:    agg.firstSent.Format("2006-01-02"),
					LastSent:     agg.lastSent.Format("2006-01-02"),
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
				fmt.Fprintln(cmd.OutOrStdout(), "No sent messages found. Run 'gmail-pp-cli sync --full' and ensure SENT label is synced.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "COUNT\tDOMAIN\tFIRST SENT\tLAST SENT")
			for _, r := range result {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", r.MessageCount, r.ToDomain, r.FirstSent, r.LastSent)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&toDomain, "to-domain", "", "Filter to emails sent to this recipient domain (e.g. acmecorp.com)")
	cmd.Flags().StringVar(&period, "period", "", "Only count messages from this period (e.g. 7d, 30d, 1w)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of domains to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
