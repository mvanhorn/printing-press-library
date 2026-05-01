// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// domains_report aggregates link counts and click totals per custom domain
// from the local /store cache (store.Open). Surfaces share-of-traffic and
// helps spot underused domains worth retiring.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type domainStat struct {
	Domain    string  `json:"domain"`
	Slug      string  `json:"slug,omitempty"`
	Verified  bool    `json:"verified"`
	Archived  bool    `json:"archived"`
	LinkCount int     `json:"link_count"`
	Clicks    int     `json:"total_clicks"`
	SharePct  float64 `json:"share_of_clicks_pct"`
}

func newDomainsReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Per-domain usage report — link counts and click distribution across custom domains",
		Long: `Aggregate every link in the local store grouped by short domain. Surfaces
over- and under-used custom domains so you know which ones are pulling weight
before renewal time.

Run sync first to populate domains and links.`,
		Example: `  dub-pp-cli domains report --json
  dub-pp-cli domains report --agent --select domain,link_count,share_of_clicks_pct`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			// build domain meta lookup
			meta := make(map[string]struct {
				slug     string
				verified bool
				archived bool
			})
			dRows, err := db.DB().Query(`SELECT data FROM domains`)
			if err == nil {
				for dRows.Next() {
					var raw string
					if err := dRows.Scan(&raw); err != nil {
						continue
					}
					var obj map[string]any
					if err := json.Unmarshal([]byte(raw), &obj); err != nil {
						continue
					}
					slug := stringField(obj, "slug")
					name := stringField(obj, "domain")
					if name == "" {
						name = slug
					}
					verified := false
					if v, ok := obj["verified"].(bool); ok {
						verified = v
					}
					archived := false
					if v, ok := obj["archived"].(bool); ok {
						archived = v
					}
					meta[name] = struct {
						slug     string
						verified bool
						archived bool
					}{slug: slug, verified: verified, archived: archived}
				}
				dRows.Close()
			}

			// aggregate links by domain
			lRows, err := db.DB().Query(`SELECT data FROM links`)
			if err != nil {
				return apiErr(fmt.Errorf("query links: %w", err))
			}
			defer lRows.Close()

			byDomain := make(map[string]*domainStat)
			totalClicks := 0
			for lRows.Next() {
				var raw string
				if err := lRows.Scan(&raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				domain := stringField(obj, "domain")
				if domain == "" {
					continue
				}
				clicks := intField(obj, "clicks")
				totalClicks += clicks
				stat, ok := byDomain[domain]
				if !ok {
					m := meta[domain]
					stat = &domainStat{Domain: domain, Slug: m.slug, Verified: m.verified, Archived: m.archived}
					byDomain[domain] = stat
				}
				stat.LinkCount++
				stat.Clicks += clicks
			}

			out := make([]domainStat, 0, len(byDomain))
			for _, s := range byDomain {
				if totalClicks > 0 {
					s.SharePct = float64(s.Clicks) * 100 / float64(totalClicks)
				}
				out = append(out, *s)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Clicks > out[j].Clicks })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no domains found in local store")
				return nil
			}
			headers := []string{"DOMAIN", "VERIFIED", "ARCHIVED", "LINKS", "CLICKS", "SHARE%"}
			rowsTbl := make([][]string, 0, len(out))
			for _, s := range out {
				v := "no"
				if s.Verified {
					v = "yes"
				}
				a := "no"
				if s.Archived {
					a = "yes"
				}
				rowsTbl = append(rowsTbl, []string{s.Domain, v, a, fmt.Sprintf("%d", s.LinkCount), fmt.Sprintf("%d", s.Clicks), fmt.Sprintf("%.1f%%", s.SharePct)})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}
	return cmd
}
