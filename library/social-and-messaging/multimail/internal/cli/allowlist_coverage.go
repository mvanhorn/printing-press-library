package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/multimail/internal/store"
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
covers its actual sending patterns. It compares recent outbound emails
against allowlist entries in the local SQLite cache to estimate how
many sends would bypass the gated_send approval queue.

A low coverage percentage means most sends are going through the approval
queue — consider adding frequently-used recipients to the allowlist.

Outbound emails are derived from synced mailboxes_emails data (emails
whose direction is outbound or whose from-address matches the mailbox).
Allowlist patterns are populated by sync --full.

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
			for mbRows.Next() {
				var m mbInfo
				if err := mbRows.Scan(&m.ID, &m.Name); err != nil {
					continue
				}
				mailboxes = append(mailboxes, m)
			}
			mbRows.Close()

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

				// PATCH: Derive recent sends from mailboxes_emails outbound.
				// The send endpoint is POST-only (no list GET), so sync
				// cannot populate the send table. Instead, count outbound
				// emails from synced mailboxes_emails data.
				// Only count emails that have at least one recipient field
				// ($.to, $.recipient, or $.recipients) — emails with no
				// recipients cannot be matched against the allowlist, so
				// including them would silently inflate the gated count
				// and deflate coverage_percent.
				var recentSends int
				sendRow := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM mailboxes_emails
					WHERE mailboxes_id = ?
					AND COALESCE(json_extract(data, '$.received_at'), json_extract(data, '$.created_at'), synced_at) > ?
					AND (json_extract(data, '$.direction') IN ('outbound', 'sent')
					  OR LOWER(COALESCE(json_extract(data, '$.from'), '')) = LOWER(?))
					AND (json_extract(data, '$.to') IS NOT NULL
					  OR json_extract(data, '$.recipient') IS NOT NULL
					  OR json_extract(data, '$.recipients') IS NOT NULL)`,
					mb.ID, cutoff, mb.Name)
				_ = sendRow.Scan(&recentSends)

				// Get allowlist patterns
				var patterns []string
				patRows, err := db.DB().QueryContext(cmd.Context(),
					`SELECT COALESCE(json_extract(data, '$.pattern'), json_extract(data, '$.email'), '')
					FROM allowlist WHERE mailboxes_id = ?`, mb.ID)
				if err == nil {
					for patRows.Next() {
						var p string
						if patRows.Scan(&p) == nil && p != "" {
							patterns = append(patterns, strings.ToLower(p))
						}
					}
					patRows.Close()
				}

				// Count how many recent outbound emails match allowlist patterns
				var coveredSends int
				if len(patterns) > 0 && recentSends > 0 {
					// PATCH: Check each outbound email's recipient against allowlist patterns
					recipRows, err := db.DB().QueryContext(cmd.Context(),
						`SELECT COALESCE(
							json_extract(data, '$.to'),
							json_extract(data, '$.recipient'),
							json_extract(data, '$.recipients'),
							''
						) FROM mailboxes_emails
						WHERE mailboxes_id = ?
						AND COALESCE(json_extract(data, '$.received_at'), json_extract(data, '$.created_at'), synced_at) > ?
						AND (json_extract(data, '$.direction') IN ('outbound', 'sent')
						  OR LOWER(COALESCE(json_extract(data, '$.from'), '')) = LOWER(?))`,
						mb.ID, cutoff, mb.Name)
					if err == nil {
						for recipRows.Next() {
							var recip string
							if recipRows.Scan(&recip) != nil || recip == "" {
								continue
							}
							// PATCH: handle $.recipients JSON array — extract
							// individual addresses so each can be matched against
							// allowlist patterns. A send is only "covered" when
							// ALL recipients match (AND-logic), because a single
							// unallowed recipient would gate the entire send.
							recipients := expandRecipients(recip)
							allMatched := true
							for _, r := range recipients {
								rLower := strings.ToLower(r)
								thisMatched := false
								for _, pat := range patterns {
									if matchAllowlistPattern(rLower, pat) {
										thisMatched = true
										break
									}
								}
								if !thisMatched {
									allMatched = false
									break
								}
							}
							if allMatched {
								coveredSends++
							}
						}
						recipRows.Close()
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

// expandRecipients normalises a recipient value from email data.
// If the value is a JSON array (from $.recipients or $.to), it returns
// all elements. Otherwise it returns a single-element slice.
func expandRecipients(raw string) []string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '[' {
		var arr []string
		if json.Unmarshal([]byte(raw), &arr) == nil && len(arr) > 0 {
			return arr
		}
	}
	return []string{raw}
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
