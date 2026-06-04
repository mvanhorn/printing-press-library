// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"
)

type domainPerf struct {
	Domain       string  `json:"domain"`
	EmailsSent   int     `json:"emails_sent"`
	OpenRate     float64 `json:"open_rate"`
	ClickRate    float64 `json:"click_rate"`
	BounceRate   float64 `json:"bounce_rate"`
	Unsubscribed int     `json:"unsubscribed"`
	Campaigns    int     `json:"campaigns_observed"`
}

type deliverabilityReport struct {
	Window           int          `json:"window"`
	CampaignsScanned int          `json:"campaigns_scanned"`
	Domains          []domainPerf `json:"domains"`
	AccountAvg       domainPerf   `json:"account_avg"`
	Underperformers  []string     `json:"underperformers,omitempty"`
}

func newDeliverabilityCmd(flags *rootFlags) *cobra.Command {
	var last int
	var domain string

	cmd := &cobra.Command{
		Use:   "deliverability",
		Short: "Per-domain bounce/open/click rates rolled up across recent campaigns. Flags inbox providers performing below the account average.",
		Long: `Mailchimp's /reports/{id}/domain-performance is per-campaign; the rollup
across the last N campaigns is where deliverability triage actually happens.

This command:
  1. Lists the last N sent campaigns
  2. Fetches /reports/{id}/domain-performance for each in parallel
  3. Rolls up per-domain (gmail.com, yahoo.com, outlook.com, ...) totals
  4. Surfaces domains performing below the account average open rate`,
		Example: `  mailchimp-pp-cli deliverability --last 10
  mailchimp-pp-cli deliverability --last 20 --domain gmail.com --json
  mailchimp-pp-cli deliverability --json --select underperformers`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if last <= 0 {
				last = 10
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_rollup_last":   last,
					"would_filter_domain": domain,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			campaignsRaw, err := c.Get("/campaigns", map[string]string{
				"count":      fmt.Sprintf("%d", last),
				"status":     "sent",
				"sort_field": "send_time",
				"sort_dir":   "DESC",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var cl map[string]any
			_ = json.Unmarshal(campaignsRaw, &cl)
			campaigns, _ := cl["campaigns"].([]any)
			if len(campaigns) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), deliverabilityReport{Window: last}, flags)
			}

			// Parallel fetch /reports/{id}/domain-performance for each campaign.
			type result struct {
				cid  string
				perf []map[string]any
				err  error
			}
			results := make(chan result, len(campaigns))
			var wg sync.WaitGroup
			for _, ca := range campaigns {
				m, _ := ca.(map[string]any)
				id, _ := m["id"].(string)
				if id == "" {
					continue
				}
				wg.Add(1)
				go func(cid string) {
					defer wg.Done()
					data, gerr := c.Get(fmt.Sprintf("/reports/%s/domain-performance", cid), nil)
					if gerr != nil {
						results <- result{cid: cid, err: gerr}
						return
					}
					var dp map[string]any
					_ = json.Unmarshal(data, &dp)
					rows, _ := dp["domains"].([]any)
					var perf []map[string]any
					for _, r := range rows {
						if rm, ok := r.(map[string]any); ok {
							perf = append(perf, rm)
						}
					}
					results <- result{cid: cid, perf: perf}
				}(id)
			}
			wg.Wait()
			close(results)

			// Aggregate per-domain. Bounce rate computation: bounces / emails_sent.
			type acc struct {
				EmailsSent   int
				OpensTotal   float64 // weighted by sent
				ClicksTotal  float64
				Bounces      int
				Unsubscribed int
				Campaigns    int
			}
			byDomain := map[string]*acc{}
			scanned := 0
			for r := range results {
				if r.err != nil {
					continue
				}
				scanned++
				for _, row := range r.perf {
					dom, _ := row["domain"].(string)
					if dom == "" {
						continue
					}
					if domain != "" && dom != domain {
						continue
					}
					a := byDomain[dom]
					if a == nil {
						a = &acc{}
						byDomain[dom] = a
					}
					sent := 0
					if v, ok := row["emails_sent"].(float64); ok {
						sent = int(v)
					}
					a.EmailsSent += sent
					a.Campaigns++
					if v, ok := row["opens"].(float64); ok {
						a.OpensTotal += v
					}
					if v, ok := row["clicks"].(float64); ok {
						a.ClicksTotal += v
					}
					if v, ok := row["bounces"].(float64); ok {
						a.Bounces += int(v)
					}
					if v, ok := row["unsubs"].(float64); ok {
						a.Unsubscribed += int(v)
					}
				}
			}

			report := deliverabilityReport{Window: last, CampaignsScanned: scanned}
			totalSent := 0
			var totalOpens, totalClicks float64
			totalBounces := 0
			for d, a := range byDomain {
				if a.EmailsSent == 0 {
					continue
				}
				perf := domainPerf{
					Domain:       d,
					EmailsSent:   a.EmailsSent,
					OpenRate:     a.OpensTotal / float64(a.EmailsSent),
					ClickRate:    a.ClicksTotal / float64(a.EmailsSent),
					BounceRate:   float64(a.Bounces) / float64(a.EmailsSent),
					Unsubscribed: a.Unsubscribed,
					Campaigns:    a.Campaigns,
				}
				report.Domains = append(report.Domains, perf)
				totalSent += a.EmailsSent
				totalOpens += a.OpensTotal
				totalClicks += a.ClicksTotal
				totalBounces += a.Bounces
			}
			sort.Slice(report.Domains, func(i, j int) bool { return report.Domains[i].EmailsSent > report.Domains[j].EmailsSent })

			if totalSent > 0 {
				report.AccountAvg = domainPerf{
					Domain:     "(account avg)",
					EmailsSent: totalSent,
					OpenRate:   totalOpens / float64(totalSent),
					ClickRate:  totalClicks / float64(totalSent),
					BounceRate: float64(totalBounces) / float64(totalSent),
				}
				// Domains performing below 80% of account avg open rate are flagged.
				threshold := report.AccountAvg.OpenRate * 0.8
				for _, d := range report.Domains {
					if d.OpenRate < threshold && d.EmailsSent >= 100 {
						report.Underperformers = append(report.Underperformers, d.Domain)
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&last, "last", 10, "Number of recent sent campaigns to roll up")
	cmd.Flags().StringVar(&domain, "domain", "", "Filter rollup to a single recipient domain (e.g. gmail.com)")
	return cmd
}
