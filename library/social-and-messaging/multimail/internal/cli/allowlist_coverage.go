package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"multimail-pp-cli/internal/store"
)

type coverageRow struct {
	Mailbox         string  `json:"mailbox"`
	MailboxID       string  `json:"mailbox_id"`
	AllowlistCount  int     `json:"allowlist_count"`
	RecentSends     int     `json:"recent_sends"`
	CoveredSends    int     `json:"covered_sends"`
	GatedSends      int     `json:"gated_sends"`
	CoveragePercent float64 `json:"coverage_percent"`
}

func newAllowlistCoverageCmd(flags *rootFlags) *cobra.Command {
	var (
		days    int
		dbPath  string
		mailbox string
	)
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "See what percentage of recent recipients are covered by allowlist patterns vs gated",
		Long: `Allowlist coverage shows how well each mailbox's sending allowlist
covers its actual sending patterns. It compares recent outbound sends
against allowlist entries in the local SQLite cache to estimate how
many sends would bypass the gated_send approval queue.

A low coverage percentage means most sends are going through the approval
queue — consider adding frequently-used recipients to the allowlist.

Requires synced data (run 'multimail-pp-cli sync --full' first).`,
		Example: strings.Trim(`
  multimail-pp-cli mailboxes allowlist coverage --json
  multimail-pp-cli mailboxes allowlist coverage --mailbox primary --days 30 --json
  multimail-pp-cli mailboxes allowlist coverage --json --select mailbox,coverage_percent,allowlist_count`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("multimail-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

			// Get mailbox names
			type mbInfo struct {
				ID   string
				Name string
			}
			var mailboxes []mbInfo
			mbRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, COALESCE(json_extract(data, '$.address'), id)
				FROM resources WHERE resource_type = 'mailboxes'`)
			if err != nil {
				return fmt.Errorf("querying mailboxes: %w", err)
			}
			defer mbRows.Close()
			for mbRows.Next() {
				var m mbInfo
				if err := mbRows.Scan(&m.ID, &m.Name); err != nil {
					continue
				}
				mailboxes = append(mailboxes, m)
			}

			if len(mailboxes) == 0 {
				return fmt.Errorf("no mailboxes in local cache — run 'multimail-pp-cli sync --full' first")
			}

			// Filter to specific mailbox if requested
			if mailbox != "" {
				mlower := strings.ToLower(mailbox)
				var filtered []mbInfo
				for _, m := range mailboxes {
					if strings.ToLower(m.Name) == mlower || m.ID == mailbox || strings.Contains(strings.ToLower(m.Name), mlower) {
						filtered = append(filtered, m)
					}
				}
				if len(filtered) == 0 {
					return fmt.Errorf("mailbox %q not found in local cache", mailbox)
				}
				mailboxes = filtered
			}

			var results []coverageRow
			for _, mb := range mailboxes {
				// Count allowlist entries for this mailbox
				var allowlistCount int
				alRow := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM allowlist WHERE mailboxes_id = ?`, mb.ID)
				_ = alRow.Scan(&allowlistCount)

				// Count recent sends for this mailbox
				var recentSends int
				sendRow := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM send
					WHERE mailboxes_id = ? AND synced_at > ?`, mb.ID, cutoff)
				_ = sendRow.Scan(&recentSends)

				// Get allowlist patterns
				var patterns []string
				patRows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT COALESCE(json_extract(data, '$.pattern'), json_extract(data, '$.email'), '')
					FROM allowlist WHERE mailboxes_id = ?`, mb.ID)
				if err == nil {
					defer patRows.Close()
					for patRows.Next() {
						var p string
						if patRows.Scan(&p) == nil && p != "" {
							patterns = append(patterns, strings.ToLower(p))
						}
					}
				}

				// Count how many recent sends match allowlist patterns
				var coveredSends int
				if len(patterns) > 0 && recentSends > 0 {
					// Check each send's recipient against allowlist patterns
					recipRows, err := db.DB().QueryContext(cmd.Context(),
						`SELECT COALESCE(
							json_extract(data, '$.to'),
							json_extract(data, '$.recipient'),
							json_extract(data, '$.recipients'),
							''
						) FROM send
						WHERE mailboxes_id = ? AND synced_at > ?`, mb.ID, cutoff)
					if err == nil {
						defer recipRows.Close()
						for recipRows.Next() {
							var recip string
							if recipRows.Scan(&recip) != nil || recip == "" {
								continue
							}
							recipLower := strings.ToLower(recip)
							for _, pat := range patterns {
								if matchAllowlistPattern(recipLower, pat) {
									coveredSends++
									break
								}
							}
						}
					}
				}

				gatedSends := recentSends - coveredSends
				if gatedSends < 0 {
					gatedSends = 0
				}

				var coveragePct float64
				if recentSends > 0 {
					coveragePct = float64(coveredSends) / float64(recentSends) * 100
				}

				results = append(results, coverageRow{
					Mailbox:         mb.Name,
					MailboxID:       mb.ID,
					AllowlistCount:  allowlistCount,
					RecentSends:     recentSends,
					CoveredSends:    coveredSends,
					GatedSends:      gatedSends,
					CoveragePercent: coveragePct,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].CoveragePercent < results[j].CoveragePercent
			})

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Look-back window in days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&mailbox, "mailbox", "", "Filter to a specific mailbox by name or ID")
	return cmd
}

// matchAllowlistPattern checks whether a recipient matches an allowlist
// pattern. Patterns can be exact emails ("user@example.com"), domain
// wildcards ("*@example.com"), or suffix matches ("@example.com").
func matchAllowlistPattern(recipient, pattern string) bool {
	if pattern == "" {
		return false
	}
	if pattern == recipient {
		return true
	}
	// *@domain.com or @domain.com → match domain suffix
	if strings.HasPrefix(pattern, "*@") {
		return strings.HasSuffix(recipient, pattern[1:])
	}
	if strings.HasPrefix(pattern, "@") {
		return strings.HasSuffix(recipient, pattern)
	}
	return false
}
