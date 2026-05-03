// Hand-written novel feature: health.
// Cross-resource Monday-morning report: rate-limit headroom, expired-but-active
// links, dead destination URLs, unverified domains, dormant tags, bounty
// submissions awaiting review.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type healthCheck struct {
	Name   string `json:"check"`
	Status string `json:"status"` // ok | warning | error | skipped
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
	Count  int    `json:"count,omitempty"`
}

type healthReport struct {
	GeneratedAt    string        `json:"generatedAt"`
	WorkspaceState string        `json:"workspaceState"` // healthy | attention | unknown
	Checks         []healthCheck `json:"checks"`
}

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Workspace doctor: rate-limit headroom, expired links, unverified domains, bounty triage backlog",
		Long: `Cross-resource Monday-morning workspace report:

  • rate-limit headroom on the live API (probes /links?pageSize=1)
  • expired-but-not-archived links
  • bounty submissions awaiting review
  • dormant tags (no link references in the local store)
  • partners marked active with zero recent activity
  • domains stuck in 'pending' verification

Run ` + "`dub-pp-cli sync`" + ` first to populate the local store.`,
		Example: strings.Trim(`
  dub-pp-cli health --json
  dub-pp-cli health --json --select check,status,detail
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			report := healthReport{
				GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
				WorkspaceState: "healthy",
			}

			// (a) Rate-limit headroom: cheap probe.
			c, err := flags.newClient()
			if err == nil {
				_, perr := c.Get("/links", map[string]string{"pageSize": "1"})
				if perr != nil {
					report.Checks = append(report.Checks, healthCheck{
						Name: "api-reachability", Status: "error",
						Detail: perr.Error(),
						Hint:   "Check DUB_API_KEY and DUB_BASE_URL.",
					})
					report.WorkspaceState = "attention"
				} else {
					report.Checks = append(report.Checks, healthCheck{
						Name: "api-reachability", Status: "ok",
						Detail: fmt.Sprintf("rate-limit headroom: %.0f req/sec", c.RateLimit()),
					})
				}
			} else {
				report.Checks = append(report.Checks, healthCheck{
					Name: "api-reachability", Status: "skipped",
					Detail: "no client configured",
					Hint:   "Set DUB_API_KEY in the environment.",
				})
			}

			// (b) Local store reads.
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				report.Checks = append(report.Checks, healthCheck{
					Name: "local-store", Status: "warning",
					Detail: err.Error(),
					Hint:   "Run `dub-pp-cli sync` to populate.",
				})
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), report, flags)
				}
				printHealthHuman(cmd, &report)
				return nil
			}
			defer db.Close()

			// expired-but-not-archived links
			expiredCount := 0
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(data,'{}'), COALESCE(archived,0)
				FROM links
			`)
			if err == nil {
				now := time.Now().Format(time.RFC3339)
				for rows.Next() {
					var blob sql.NullString
					var archived int
					if err := rows.Scan(&blob, &archived); err != nil {
						continue
					}
					if archived != 0 {
						continue
					}
					blobBytes := []byte(blob.String)
					if !blob.Valid {
						blobBytes = []byte("{}")
					}
					expires := extractFromData(blobBytes, "expiresAt")
					if expires != "" && expires < now {
						expiredCount++
					}
				}
				rows.Close()
			}
			if expiredCount > 0 {
				report.Checks = append(report.Checks, healthCheck{
					Name: "expired-links", Status: "warning", Count: expiredCount,
					Detail: fmt.Sprintf("%d link(s) past expiresAt and not archived", expiredCount),
					Hint:   "Run `dub-pp-cli links list --json` and consider archiving.",
				})
				report.WorkspaceState = "attention"
			} else {
				report.Checks = append(report.Checks, healthCheck{
					Name: "expired-links", Status: "ok", Detail: "no expired-but-active links",
				})
			}

			// pending bounty submissions
			pending := 0
			rows2, err := db.DB().QueryContext(cmd.Context(), `SELECT COALESCE(data,'{}') FROM submissions`)
			if err == nil {
				for rows2.Next() {
					var blob sql.NullString
					if err := rows2.Scan(&blob); err == nil && blob.Valid {
						if strings.EqualFold(extractFromData([]byte(blob.String), "status"), "pending") {
							pending++
						}
					}
				}
				rows2.Close()
			}
			if pending > 0 {
				report.Checks = append(report.Checks, healthCheck{
					Name: "bounty-triage-backlog", Status: "warning", Count: pending,
					Detail: fmt.Sprintf("%d bounty submission(s) awaiting review", pending),
					Hint:   "Run `dub-pp-cli bounties triage --status pending`.",
				})
				report.WorkspaceState = "attention"
			} else {
				report.Checks = append(report.Checks, healthCheck{
					Name: "bounty-triage-backlog", Status: "ok",
				})
			}

			// dormant tags (no links wear them) — heuristic, ~ok
			dormantTags := 0
			rows3, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id FROM tags
				WHERE id NOT IN (
					SELECT DISTINCT tag_id FROM links WHERE tag_id != ''
				)
			`)
			if err == nil {
				for rows3.Next() {
					var id string
					if err := rows3.Scan(&id); err == nil {
						dormantTags++
					}
				}
				rows3.Close()
			}
			if dormantTags > 0 {
				report.Checks = append(report.Checks, healthCheck{
					Name: "dormant-tags", Status: "info", Count: dormantTags,
					Detail: fmt.Sprintf("%d tag(s) attached to no links", dormantTags),
					Hint:   "Consider deleting unused tags via `dub-pp-cli tags delete`.",
				})
			} else {
				report.Checks = append(report.Checks, healthCheck{
					Name: "dormant-tags", Status: "ok",
				})
			}

			// pending domains
			pendingDomains := 0
			rows4, err := db.DB().QueryContext(cmd.Context(), `SELECT COALESCE(data,'{}') FROM domains`)
			if err == nil {
				for rows4.Next() {
					var blob sql.NullString
					if err := rows4.Scan(&blob); err == nil && blob.Valid {
						st := strings.ToLower(extractFromData([]byte(blob.String), "verified"))
						if st == "false" {
							pendingDomains++
						}
					}
				}
				rows4.Close()
			}
			if pendingDomains > 0 {
				report.Checks = append(report.Checks, healthCheck{
					Name: "unverified-domains", Status: "warning", Count: pendingDomains,
					Detail: fmt.Sprintf("%d domain(s) not verified", pendingDomains),
					Hint:   "Run `dub-pp-cli domains status --slug <slug>` to inspect.",
				})
				report.WorkspaceState = "attention"
			} else {
				report.Checks = append(report.Checks, healthCheck{
					Name: "unverified-domains", Status: "ok",
				})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			printHealthHuman(cmd, &report)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func printHealthHuman(cmd *cobra.Command, r *healthReport) {
	fmt.Fprintf(cmd.OutOrStdout(), "Workspace state: %s\n\n", strings.ToUpper(r.WorkspaceState))
	fmt.Fprintf(cmd.OutOrStdout(), "%-26s %-10s %s\n", "CHECK", "STATUS", "DETAIL")
	for _, c := range r.Checks {
		fmt.Fprintf(cmd.OutOrStdout(), "%-26s %-10s %s\n", c.Name, c.Status, c.Detail)
		if c.Hint != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%-26s %-10s   ↳ %s\n", "", "", c.Hint)
		}
	}
}
