// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// campaignDigest is the joined-report view for a single campaign that
// digest renders. Composed from /campaigns/{id} + /reports/{id} +
// /reports/{id}/email-activity (top entries) + /reports/{id}/ecommerce-product-activity.
type campaignDigest struct {
	CampaignID    string             `json:"campaign_id"`
	Subject       string             `json:"subject_line,omitempty"`
	SendTime      string             `json:"send_time,omitempty"`
	EmailsSent    int                `json:"emails_sent"`
	OpenRate      float64            `json:"open_rate"`
	ClickRate     float64            `json:"click_rate"`
	UniqueOpens   int                `json:"unique_opens"`
	UniqueClicks  int                `json:"unique_clicks"`
	HardBounces   int                `json:"hard_bounces"`
	SoftBounces   int                `json:"soft_bounces"`
	Unsubscribed  int                `json:"unsubscribed"`
	Revenue       float64            `json:"total_revenue,omitempty"`
	TopLinks      []digestLink       `json:"top_links,omitempty"`
	TopProducts   []digestProduct    `json:"top_products,omitempty"`
}

type digestLink struct {
	URL         string  `json:"url"`
	UniqueClicks int    `json:"unique_clicks"`
	ClickRate   float64 `json:"click_rate"`
}

type digestProduct struct {
	Title       string  `json:"title"`
	SKU         string  `json:"sku,omitempty"`
	Currency    string  `json:"currency,omitempty"`
	TotalRevenue float64 `json:"total_revenue"`
	TotalOrders  int    `json:"total_orders"`
}

// fetchCampaignDigest pulls a single campaign's joined report. Uses parallel
// fetches across the four endpoints. Defensive: if a sub-endpoint returns an
// error (e.g. 404 on ecommerce-product-activity for a non-ecommerce campaign),
// the digest still returns with what it could fetch.
func fetchCampaignDigest(c apiClient, id string) (campaignDigest, error) {
	type slot struct {
		key  string
		data json.RawMessage
		err  error
	}
	results := make(chan slot, 3)
	var wg sync.WaitGroup
	endpoints := []struct{ key, path string }{
		{"report", fmt.Sprintf("/reports/%s", id)},
		{"links", fmt.Sprintf("/reports/%s/click-details", id)},
		{"products", fmt.Sprintf("/reports/%s/ecommerce-product-activity", id)},
	}
	for _, ep := range endpoints {
		wg.Add(1)
		go func(key, path string) {
			defer wg.Done()
			data, err := c.Get(path, nil)
			results <- slot{key: key, data: data, err: err}
		}(ep.key, ep.path)
	}
	wg.Wait()
	close(results)

	parts := map[string]json.RawMessage{}
	for r := range results {
		if r.err == nil {
			parts[r.key] = r.data
		}
	}

	// The base report is required; if it's missing we propagate the error.
	if parts["report"] == nil {
		return campaignDigest{}, fmt.Errorf("could not fetch report for campaign %s", id)
	}

	d := campaignDigest{CampaignID: id}
	var rep map[string]any
	if err := json.Unmarshal(parts["report"], &rep); err == nil {
		if v, ok := rep["subject_line"].(string); ok {
			d.Subject = v
		}
		if v, ok := rep["send_time"].(string); ok {
			d.SendTime = v
		}
		if v, ok := rep["emails_sent"].(float64); ok {
			d.EmailsSent = int(v)
		}
		if v, ok := rep["open_rate"].(float64); ok {
			d.OpenRate = v
		}
		if v, ok := rep["click_rate"].(float64); ok {
			d.ClickRate = v
		}
		if v, ok := rep["unique_opens"].(float64); ok {
			d.UniqueOpens = int(v)
		}
		if v, ok := rep["unique_clicks"].(float64); ok {
			d.UniqueClicks = int(v)
		}
		if v, ok := rep["hard_bounces"].(float64); ok {
			d.HardBounces = int(v)
		}
		if v, ok := rep["soft_bounces"].(float64); ok {
			d.SoftBounces = int(v)
		}
		if v, ok := rep["unsubscribed"].(float64); ok {
			d.Unsubscribed = int(v)
		}
		if ec, ok := rep["ecommerce"].(map[string]any); ok {
			if v, ok := ec["total_revenue"].(float64); ok {
				d.Revenue = v
			}
		}
	}

	// Top links
	if raw, ok := parts["links"]; ok {
		var lr map[string]any
		_ = json.Unmarshal(raw, &lr)
		if urls, ok := lr["urls_clicked"].([]any); ok {
			for _, u := range urls {
				m, _ := u.(map[string]any)
				if m == nil {
					continue
				}
				link := digestLink{}
				if v, ok := m["url"].(string); ok {
					link.URL = v
				}
				if v, ok := m["unique_clicks"].(float64); ok {
					link.UniqueClicks = int(v)
				}
				if v, ok := m["click_percentage"].(float64); ok {
					link.ClickRate = v
				}
				if link.URL != "" {
					d.TopLinks = append(d.TopLinks, link)
				}
			}
		}
		sort.Slice(d.TopLinks, func(i, j int) bool { return d.TopLinks[i].UniqueClicks > d.TopLinks[j].UniqueClicks })
		if len(d.TopLinks) > 5 {
			d.TopLinks = d.TopLinks[:5]
		}
	}

	// Top products (only present for ecommerce campaigns)
	if raw, ok := parts["products"]; ok {
		var pr map[string]any
		_ = json.Unmarshal(raw, &pr)
		if items, ok := pr["products"].([]any); ok {
			for _, p := range items {
				m, _ := p.(map[string]any)
				if m == nil {
					continue
				}
				prod := digestProduct{}
				if v, ok := m["title"].(string); ok {
					prod.Title = v
				}
				if v, ok := m["sku"].(string); ok {
					prod.SKU = v
				}
				if v, ok := m["currency_code"].(string); ok {
					prod.Currency = v
				}
				if v, ok := m["total_revenue"].(float64); ok {
					prod.TotalRevenue = v
				}
				if v, ok := m["total_orders"].(float64); ok {
					prod.TotalOrders = int(v)
				}
				if prod.Title != "" {
					d.TopProducts = append(d.TopProducts, prod)
				}
			}
		}
		sort.Slice(d.TopProducts, func(i, j int) bool { return d.TopProducts[i].TotalRevenue > d.TopProducts[j].TotalRevenue })
		if len(d.TopProducts) > 5 {
			d.TopProducts = d.TopProducts[:5]
		}
	}

	return d, nil
}

// apiClient is the narrow interface digest + compare + deliverability + attribution
// share so future test seams can inject a fake without a full *client.Client.
type apiClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func renderDigestSingleMarkdown(d campaignDigest) string {
	var sb strings.Builder
	sb.WriteString("# Campaign digest\n\n")
	if d.Subject != "" {
		sb.WriteString(fmt.Sprintf("**%s**", d.Subject))
		if d.SendTime != "" {
			sb.WriteString(fmt.Sprintf(" — sent %s", d.SendTime))
		}
		sb.WriteString("\n\n")
	}
	sb.WriteString(fmt.Sprintf("Campaign `%s`\n\n", d.CampaignID))
	sb.WriteString("| Metric | Value |\n|---|---|\n")
	sb.WriteString(fmt.Sprintf("| Emails sent | %d |\n", d.EmailsSent))
	sb.WriteString(fmt.Sprintf("| Open rate | %.2f%% (%d unique) |\n", d.OpenRate*100, d.UniqueOpens))
	sb.WriteString(fmt.Sprintf("| Click rate | %.2f%% (%d unique) |\n", d.ClickRate*100, d.UniqueClicks))
	sb.WriteString(fmt.Sprintf("| Bounces | %d hard, %d soft |\n", d.HardBounces, d.SoftBounces))
	sb.WriteString(fmt.Sprintf("| Unsubscribes | %d |\n", d.Unsubscribed))
	if d.Revenue > 0 {
		sb.WriteString(fmt.Sprintf("| Revenue | $%.2f |\n", d.Revenue))
	}
	if len(d.TopLinks) > 0 {
		sb.WriteString("\n## Top clicked links\n\n")
		for i, l := range d.TopLinks {
			sb.WriteString(fmt.Sprintf("%d. [%s](%s) — %d unique clicks\n", i+1, truncURL(l.URL, 60), l.URL, l.UniqueClicks))
		}
	}
	if len(d.TopProducts) > 0 {
		sb.WriteString("\n## Top converted products\n\n")
		for i, p := range d.TopProducts {
			sb.WriteString(fmt.Sprintf("%d. **%s** — $%.2f (%d orders)\n", i+1, p.Title, p.TotalRevenue, p.TotalOrders))
		}
	}
	return sb.String()
}

func truncURL(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// digestRollupEntry is one row in the multi-campaign rollup view.
type digestRollupEntry struct {
	CampaignID   string  `json:"campaign_id"`
	Subject      string  `json:"subject_line,omitempty"`
	SendTime     string  `json:"send_time,omitempty"`
	EmailsSent   int     `json:"emails_sent"`
	OpenRate     float64 `json:"open_rate"`
	ClickRate    float64 `json:"click_rate"`
	Revenue      float64 `json:"total_revenue,omitempty"`
	Unsubscribed int     `json:"unsubscribed"`
}

type digestRollup struct {
	Window        int                 `json:"window"`
	Campaigns     []digestRollupEntry `json:"campaigns"`
	Totals        digestRollupTotals  `json:"totals"`
	FetchFailures []string            `json:"fetch_failures,omitempty"`
}

type digestRollupTotals struct {
	EmailsSent  int     `json:"emails_sent"`
	AvgOpenRate float64 `json:"avg_open_rate"`
	AvgClickRate float64 `json:"avg_click_rate"`
	TotalRevenue float64 `json:"total_revenue,omitempty"`
	Unsubscribed int     `json:"unsubscribed"`
}

func renderDigestRollupMarkdown(rollup digestRollup) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Last %d campaigns\n\n", rollup.Window))
	sb.WriteString("| Subject | Sent | Opens | Clicks | Revenue | Unsub |\n|---|---:|---:|---:|---:|---:|\n")
	for _, e := range rollup.Campaigns {
		subj := e.Subject
		if subj == "" {
			subj = e.CampaignID
		}
		if len(subj) > 50 {
			subj = subj[:47] + "…"
		}
		rev := "—"
		if e.Revenue > 0 {
			rev = fmt.Sprintf("$%.2f", e.Revenue)
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %.2f%% | %.2f%% | %s | %d |\n",
			subj, e.EmailsSent, e.OpenRate*100, e.ClickRate*100, rev, e.Unsubscribed))
	}
	sb.WriteString("\n## Totals\n\n")
	sb.WriteString(fmt.Sprintf("- Emails sent: **%d**\n", rollup.Totals.EmailsSent))
	sb.WriteString(fmt.Sprintf("- Avg open rate: **%.2f%%**\n", rollup.Totals.AvgOpenRate*100))
	sb.WriteString(fmt.Sprintf("- Avg click rate: **%.2f%%**\n", rollup.Totals.AvgClickRate*100))
	if rollup.Totals.TotalRevenue > 0 {
		sb.WriteString(fmt.Sprintf("- Total revenue: **$%.2f**\n", rollup.Totals.TotalRevenue))
	}
	sb.WriteString(fmt.Sprintf("- Unsubscribes: **%d**\n", rollup.Totals.Unsubscribed))
	return sb.String()
}

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var md bool
	var last int

	cmd := &cobra.Command{
		Use:   "digest [<campaign-id>]",
		Short: "Joined campaign performance: opens, clicks, revenue, top links, top products. Single-campaign or multi-campaign rollup.",
		Long: `Two shapes:

  digest <campaign-id>           — deep dive on one campaign (report + click details +
                                    ecommerce product activity joined into one summary)
  digest --last N | --week       — rollup across the last N campaigns (one row each,
                                    aggregate stats at the bottom)

The single-campaign view is what an agency uses for a per-client Friday report;
the rollup is what a founder pastes into a Monday "what shipped last week" doc.

--md renders either as paste-ready markdown for Notion, Slack, or weekly review docs.`,
		Example: `  mailchimp-pp-cli digest 7f8a9b0c1d
  mailchimp-pp-cli digest 7f8a9b0c1d --md
  mailchimp-pp-cli digest --last 5
  mailchimp-pp-cli digest --week --md`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if len(args) > 0 {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"would_fetch_digest_for": args[0],
					}, flags)
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_fetch_rollup": map[string]any{"last": last},
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Single-campaign mode
			if len(args) > 0 {
				d, err := fetchCampaignDigest(c, args[0])
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if md {
					_, werr := cmd.OutOrStdout().Write([]byte(renderDigestSingleMarkdown(d)))
					return werr
				}
				return printJSONFiltered(cmd.OutOrStdout(), d, flags)
			}

			// Rollup mode — fetch recent campaigns + their reports in parallel
			if last <= 0 {
				last = 7
			}
			campaignsRaw, err := c.Get("/campaigns", map[string]string{
				"count":  fmt.Sprintf("%d", last),
				"status": "sent",
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
				return printJSONFiltered(cmd.OutOrStdout(), digestRollup{Window: last}, flags)
			}

			// Parallel fetch /reports/{id} for each. Mailchimp's 10-concurrent cap is
			// the upper bound — for last>10 we still parallelize but the HTTP client
			// will queue.
			type rollupResult struct {
				idx int
				entry digestRollupEntry
				err error
			}
			results := make(chan rollupResult, len(campaigns))
			var wg sync.WaitGroup
			for i, ca := range campaigns {
				m, _ := ca.(map[string]any)
				id, _ := m["id"].(string)
				if id == "" {
					continue
				}
				wg.Add(1)
				go func(idx int, cid string, ci map[string]any) {
					defer wg.Done()
					rep, gerr := c.Get(fmt.Sprintf("/reports/%s", cid), nil)
					entry := digestRollupEntry{CampaignID: cid}
					if subj, ok := ci["settings"].(map[string]any); ok {
						if v, ok := subj["subject_line"].(string); ok {
							entry.Subject = v
						}
					}
					if v, ok := ci["send_time"].(string); ok {
						entry.SendTime = v
					}
					if gerr == nil {
						var r map[string]any
						_ = json.Unmarshal(rep, &r)
						if v, ok := r["emails_sent"].(float64); ok {
							entry.EmailsSent = int(v)
						}
						if v, ok := r["open_rate"].(float64); ok {
							entry.OpenRate = v
						}
						if v, ok := r["click_rate"].(float64); ok {
							entry.ClickRate = v
						}
						if v, ok := r["unsubscribed"].(float64); ok {
							entry.Unsubscribed = int(v)
						}
						if ec, ok := r["ecommerce"].(map[string]any); ok {
							if v, ok := ec["total_revenue"].(float64); ok {
								entry.Revenue = v
							}
						}
					}
					results <- rollupResult{idx: idx, entry: entry, err: gerr}
				}(i, id, m)
			}
			wg.Wait()
			close(results)

			rollup := digestRollup{Window: last}
			ordered := make([]digestRollupEntry, len(campaigns))
			fetchErrors := make([]bool, len(campaigns))
			var failedIDs []string
			for r := range results {
				ordered[r.idx] = r.entry
				if r.err != nil {
					fetchErrors[r.idx] = true
					if r.entry.CampaignID != "" {
						failedIDs = append(failedIDs, r.entry.CampaignID)
					}
				}
			}
			// Compute totals + averages over successfully fetched entries only.
			// Including failed fetches (which have all-zero metrics) would
			// silently dilute AvgOpenRate/AvgClickRate with phantom zeros —
			// a rollup of 10 campaigns where one fetch transiently 5xx'd
			// would under-report engagement rates compared to the 9
			// successful campaigns. Failed campaigns still appear in
			// rollup.Campaigns (with a fetch_failed marker) so the user
			// can see the gap, but they don't pollute the averages.
			var totalOpen, totalClick float64
			var n int
			for i, e := range ordered {
				if e.CampaignID == "" {
					continue
				}
				rollup.Campaigns = append(rollup.Campaigns, e)
				if fetchErrors[i] {
					// Don't count toward totals/averages — but keep the row
					// so the user sees that this campaign was attempted.
					continue
				}
				rollup.Totals.EmailsSent += e.EmailsSent
				rollup.Totals.TotalRevenue += e.Revenue
				rollup.Totals.Unsubscribed += e.Unsubscribed
				totalOpen += e.OpenRate
				totalClick += e.ClickRate
				n++
			}
			if n > 0 {
				rollup.Totals.AvgOpenRate = totalOpen / float64(n)
				rollup.Totals.AvgClickRate = totalClick / float64(n)
			}
			if len(failedIDs) > 0 {
				rollup.FetchFailures = failedIDs
				fmt.Fprintf(os.Stderr, "warning: %d of %d report fetches failed; averages computed over the remaining %d campaigns.\n",
					len(failedIDs), len(campaigns), n)
			}

			if md {
				_, werr := cmd.OutOrStdout().Write([]byte(renderDigestRollupMarkdown(rollup)))
				return werr
			}
			return printJSONFiltered(cmd.OutOrStdout(), rollup, flags)
		},
	}
	cmd.Flags().BoolVar(&md, "md", false, "Render as paste-ready markdown for Notion/Slack/client weekly docs")
	cmd.Flags().IntVar(&last, "last", 0, "Rollup mode: number of recent campaigns to summarize (default 7)")
	cmd.Flags().BoolVar(new(bool), "week", false, "Rollup mode: alias for --last 7 (cosmetic; --last takes precedence)")
	return cmd
}
