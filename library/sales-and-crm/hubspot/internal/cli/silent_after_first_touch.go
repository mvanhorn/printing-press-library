// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

type silentFirstTouchItem struct {
	ID             string `json:"id"`
	Email          string `json:"email,omitempty"`
	Name           string `json:"name,omitempty"`
	LastActivityAt string `json:"last_activity_at,omitempty"`
	SilentDays     int    `json:"silent_days"`
	TotalTouches   int64  `json:"total_touches"`
}

func newNovelSilentAfterFirstTouchCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "silent-after-first-touch",
		Short: "Find contacts who engaged exactly once and then went silent — distinguish tire-kickers from genuine interest.",
		Long: strings.Trim(`
A contact qualifies when hs_email_open + hs_email_click is exactly one and
the last activity happened at least --since ago. The result is sorted
oldest-silent first so the longest-quiet single-engagers come up before the
recent ones.

Returns an empty list when the synced contacts lack hs_email_open /
hs_email_click in their properties.
`, "\n"),
		Example: strings.Trim(`
  # Contacts who opened or clicked exactly once and haven't moved in 90 days
  hubspot-pp-cli silent-after-first-touch --since 90d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			d, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", since, err))
			}
			if d < 0 {
				return usageErr(fmt.Errorf("--since must be non-negative"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be > 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			now := time.Now().UTC()
			cutoff := now.Add(-d)
			items := make([]silentFirstTouchItem, 0)
			scanned, parseErrors, err := streamContacts(cmd.Context(), db, func(p contactProps) error {
				totalTouches := p.EmailOpen + p.EmailClick
				if totalTouches != 1 {
					return nil
				}
				if !p.LastActivitySet {
					return nil
				}
				if p.LastActivityDate.After(cutoff) {
					return nil
				}
				silent := int(now.Sub(p.LastActivityDate).Hours() / 24)
				items = append(items, silentFirstTouchItem{
					ID:             p.ID,
					Email:          p.Email,
					Name:           contactDisplayName(p),
					LastActivityAt: p.LastActivityDate.UTC().Format(time.RFC3339),
					SilentDays:     silent,
					TotalTouches:   totalTouches,
				})
				return nil
			})
			if err != nil {
				return err
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].LastActivityAt != items[j].LastActivityAt {
					return items[i].LastActivityAt < items[j].LastActivityAt
				}
				return items[i].ID < items[j].ID
			})
			matches := len(items)
			items = limitInt(items, limit)
			result := map[string]any{
				"since":   since,
				"items":   items,
				"scanned": scanned,
				"matches": matches,
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "Minimum silence interval since the single touch (e.g. 90d, 12w, 2160h)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of items to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}
