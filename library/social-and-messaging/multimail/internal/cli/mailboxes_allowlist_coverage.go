// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for CLI Printing Press.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelMailboxesAllowlistCoverageCmd(flags *rootFlags) *cobra.Command {
	var flagMailbox string
	var flagDays int

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "See what percentage of recent recipients are covered by allowlist patterns vs gated.",
		Long: `Allowlist coverage pattern-matches synced allowlist entries against sent
email recipients locally. No API endpoint diffs coverage — this analysis
is only possible against synced data.

Run 'multimail-pp-cli sync' first to populate local data.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "multimail-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return fmt.Errorf("no local data. Run 'multimail-pp-cli sync' first")
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "allowlist")
			hintIfStale(cmd, db, "allowlist", flags.maxAge)

			sqlDB := db.DB()

			// Load allowlist entries per mailbox
			type allowlistEntry struct {
				MailboxID string
				Pattern   string // email or domain pattern
			}
			var entries []allowlistEntry
			alRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'allowlist'`)
			if err != nil {
				return fmt.Errorf("querying allowlist: %w", err)
			}
			for alRows.Next() {
				var raw string
				if alRows.Scan(&raw) != nil {
					break
				}
				var e map[string]any
				if json.Unmarshal([]byte(raw), &e) != nil {
					continue
				}
				mailboxID, _ := e["mailbox_id"].(string)
				if mailboxID == "" {
					mailboxID, _ = e["mailboxes_id"].(string)
				}
				pattern, _ := e["pattern"].(string)
				if pattern == "" {
					pattern, _ = e["email"].(string)
				}
				if pattern == "" {
					pattern, _ = e["address"].(string)
				}
				if pattern == "" || (flagMailbox != "" && mailboxID != flagMailbox) {
					continue
				}
				entries = append(entries, allowlistEntry{MailboxID: mailboxID, Pattern: pattern})
			}
			alRows.Close()

			// Load sent emails (outbound) in time window
			cutoff := time.Now().AddDate(0, 0, -flagDays)
			type sentEmail struct {
				MailboxID  string
				Recipients []string
			}
			var sent []sentEmail

			emailRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'mailboxes_emails'`)
			if err == nil {
				for emailRows.Next() {
					var raw string
					if emailRows.Scan(&raw) != nil {
						break
					}
					var e map[string]any
					if json.Unmarshal([]byte(raw), &e) != nil {
						continue
					}
					// Only outbound emails — require explicit direction or status
					dir, _ := e["direction"].(string)
					isOutbound := dir == "outbound" || dir == "sent"
					if !isOutbound {
						// Fall back to status field for emails without direction
						status, _ := e["status"].(string)
						isOutbound = status == "sent" || status == "delivered"
					}
					if !isOutbound {
						continue
					}

					// Time filter — skip emails with unparseable timestamps
					if ts, ok := parseNumericTime(e["created_at"]); ok {
						if ts.Before(cutoff) {
							continue
						}
					} else if ts, ok := parseNumericTime(e["delivered_at"]); ok {
						if ts.Before(cutoff) {
							continue
						}
					} else {
						continue
					}

					mailboxID, _ := e["mailboxes_id"].(string)
					if mailboxID == "" {
						mailboxID, _ = e["mailbox_id"].(string)
					}
					if flagMailbox != "" && mailboxID != flagMailbox {
						continue
					}

					var recipients []string
					if to, ok := e["to"].([]any); ok {
						for _, t := range to {
							if s, ok := t.(string); ok {
								recipients = append(recipients, strings.ToLower(s))
							}
						}
					} else if to, ok := e["to"].(string); ok {
						recipients = append(recipients, strings.ToLower(to))
					}
					if len(recipients) > 0 {
						sent = append(sent, sentEmail{MailboxID: mailboxID, Recipients: recipients})
					}
				}
				emailRows.Close()
			}

			// Also check generic emails resource
			emailRows2, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'emails'`)
			if err == nil {
				for emailRows2.Next() {
					var raw string
					if emailRows2.Scan(&raw) != nil {
						break
					}
					var e map[string]any
					if json.Unmarshal([]byte(raw), &e) != nil {
						continue
					}
					dir, _ := e["direction"].(string)
					if dir != "outbound" && dir != "sent" {
						continue
					}
					if ts, ok := parseNumericTime(e["created_at"]); ok {
						if ts.Before(cutoff) {
							continue
						}
					} else {
						continue
					}
					var recipients []string
					if to, ok := e["to"].([]any); ok {
						for _, t := range to {
							if s, ok := t.(string); ok {
								recipients = append(recipients, strings.ToLower(s))
							}
						}
					}
					if len(recipients) > 0 {
						sent = append(sent, sentEmail{Recipients: recipients})
					}
				}
				emailRows2.Close()
			}

			// Build allowlist patterns per mailbox
			patternsByMailbox := map[string][]string{}
			allPatterns := []string{}
			for _, e := range entries {
				lower := strings.ToLower(e.Pattern)
				patternsByMailbox[e.MailboxID] = append(patternsByMailbox[e.MailboxID], lower)
				allPatterns = append(allPatterns, lower)
			}

			// Match recipients against allowlist
			totalRecipients := 0
			coveredRecipients := 0
			uncoveredSet := map[string]int{} // recipient -> count

			for _, s := range sent {
				patterns := patternsByMailbox[s.MailboxID]
				if len(patterns) == 0 {
					patterns = allPatterns // fallback to all patterns
				}
				for _, recip := range s.Recipients {
					totalRecipients++
					if matchesAllowlist(recip, patterns) {
						coveredRecipients++
					} else {
						uncoveredSet[recip]++
					}
				}
			}

			coveragePct := float64(0)
			if totalRecipients > 0 {
				coveragePct = float64(coveredRecipients) / float64(totalRecipients) * 100
			}

			// Top uncovered recipients
			type uncoveredEntry struct {
				Address string `json:"address"`
				Count   int    `json:"count"`
			}
			var uncovered []uncoveredEntry
			for addr, count := range uncoveredSet {
				uncovered = append(uncovered, uncoveredEntry{Address: addr, Count: count})
			}
			sort.Slice(uncovered, func(i, j int) bool {
				return uncovered[i].Count > uncovered[j].Count
			})
			if len(uncovered) > 20 {
				uncovered = uncovered[:20]
			}

			output := map[string]any{
				"period_days":          flagDays,
				"total_recipients":     totalRecipients,
				"covered_recipients":   coveredRecipients,
				"uncovered_recipients": totalRecipients - coveredRecipients,
				"coverage_pct":         coveragePct,
				"allowlist_entries":    len(entries),
				"top_uncovered":        uncovered,
				"generated_at":         time.Now().UTC().Format(time.RFC3339),
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}
	cmd.Flags().StringVar(&flagMailbox, "mailbox", "", "Filter by mailbox ID")
	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of days to look back for sent emails")
	return cmd
}

// matchesAllowlist checks if an email address matches any allowlist pattern.
// Patterns can be exact email addresses or domain patterns (e.g., "@example.com").
func matchesAllowlist(email string, patterns []string) bool {
	email = strings.ToLower(email)
	for _, pattern := range patterns {
		pattern = strings.ToLower(pattern)
		// Exact match
		if email == pattern {
			return true
		}
		// Domain match: pattern like "@example.com" or "example.com"
		if strings.HasPrefix(pattern, "@") {
			if strings.HasSuffix(email, pattern) {
				return true
			}
		} else if strings.Contains(pattern, "@") {
			// Full email pattern
			if email == pattern {
				return true
			}
		} else {
			// Bare domain: match if email ends with @domain
			if strings.HasSuffix(email, "@"+pattern) {
				return true
			}
		}
		// Wildcard patterns
		if strings.Contains(pattern, "*") {
			if matchWildcard(pattern, email) {
				return true
			}
		}
	}
	return false
}

// matchWildcard performs a simple wildcard match where * matches any characters.
func matchWildcard(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		return strings.HasPrefix(s, parts[0]) && strings.HasSuffix(s, parts[1])
	}
	// Multiple wildcards: check prefix, suffix, and substrings between
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(s, parts[i])
		if idx < 0 {
			return false
		}
		s = s[idx+len(parts[i]):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}
