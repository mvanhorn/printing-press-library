// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// campaignReportSnapshot is the minimal view of a campaign's report that the
// compare command renders. The /reports/{id} response is ~5-15KB; we extract
// only the fields needed for diffing to keep the human and --md output
// compact and the agent token cost low.
type campaignReportSnapshot struct {
	CampaignID    string  `json:"campaign_id"`
	Subject       string  `json:"subject_line,omitempty"`
	SendTime      string  `json:"send_time,omitempty"`
	EmailsSent    int     `json:"emails_sent,omitempty"`
	OpenRate      float64 `json:"open_rate"`
	UniqueOpens   int     `json:"unique_opens"`
	ClickRate     float64 `json:"click_rate"`
	UniqueClicks  int     `json:"unique_clicks"`
	ClickToOpen   float64 `json:"click_to_open_rate"`
	HardBounces   int     `json:"hard_bounces"`
	SoftBounces   int     `json:"soft_bounces"`
	Unsubscribes  int     `json:"unsubscribed"`
	AbuseReports  int     `json:"abuse_reports"`
	Revenue       float64 `json:"total_revenue,omitempty"`
}

// extractCampaignReport pulls the subset of fields compare cares about from
// a raw /reports/{id} response. Defensive parsing — missing fields default
// to zero rather than failing the whole compare.
func extractCampaignReport(data json.RawMessage, id string) campaignReportSnapshot {
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	s := campaignReportSnapshot{CampaignID: id}
	if v, ok := raw["subject_line"].(string); ok {
		s.Subject = v
	}
	if v, ok := raw["send_time"].(string); ok {
		s.SendTime = v
	}
	if v, ok := raw["emails_sent"].(float64); ok {
		s.EmailsSent = int(v)
	}
	if v, ok := raw["open_rate"].(float64); ok {
		s.OpenRate = v
	}
	if v, ok := raw["unique_opens"].(float64); ok {
		s.UniqueOpens = int(v)
	}
	if v, ok := raw["click_rate"].(float64); ok {
		s.ClickRate = v
	}
	if v, ok := raw["unique_clicks"].(float64); ok {
		s.UniqueClicks = int(v)
	}
	if s.UniqueOpens > 0 {
		s.ClickToOpen = float64(s.UniqueClicks) / float64(s.UniqueOpens)
	}
	if v, ok := raw["hard_bounces"].(float64); ok {
		s.HardBounces = int(v)
	}
	if v, ok := raw["soft_bounces"].(float64); ok {
		s.SoftBounces = int(v)
	}
	if v, ok := raw["unsubscribed"].(float64); ok {
		s.Unsubscribes = int(v)
	}
	if v, ok := raw["abuse_reports"].(float64); ok {
		s.AbuseReports = int(v)
	}
	if ec, ok := raw["ecommerce"].(map[string]any); ok {
		if v, ok := ec["total_revenue"].(float64); ok {
			s.Revenue = v
		}
	}
	return s
}

// metricRow drives both the structured JSON output and the human/markdown
// renderer. "higher_is_better" picks the winner direction; bounces and
// unsubscribes set it to false.
type metricRow struct {
	Name           string  `json:"metric"`
	A              float64 `json:"a"`
	B              float64 `json:"b"`
	Delta          float64 `json:"delta_a_minus_b"`
	HigherIsBetter bool    `json:"higher_is_better"`
	Winner         string  `json:"winner"` // "a", "b", or "tie"
	IsPct          bool    `json:"is_percent,omitempty"`
}

func (m *metricRow) computeWinner() {
	m.Delta = m.A - m.B
	if m.A == m.B {
		m.Winner = "tie"
		return
	}
	better := m.A > m.B
	if !m.HigherIsBetter {
		better = m.A < m.B
	}
	if better {
		m.Winner = "a"
	} else {
		m.Winner = "b"
	}
}

func compareMetrics(a, b campaignReportSnapshot) []metricRow {
	rows := []metricRow{
		{Name: "emails_sent", A: float64(a.EmailsSent), B: float64(b.EmailsSent), HigherIsBetter: true},
		{Name: "open_rate", A: a.OpenRate, B: b.OpenRate, HigherIsBetter: true, IsPct: true},
		{Name: "click_rate", A: a.ClickRate, B: b.ClickRate, HigherIsBetter: true, IsPct: true},
		{Name: "click_to_open_rate", A: a.ClickToOpen, B: b.ClickToOpen, HigherIsBetter: true, IsPct: true},
		{Name: "unique_opens", A: float64(a.UniqueOpens), B: float64(b.UniqueOpens), HigherIsBetter: true},
		{Name: "unique_clicks", A: float64(a.UniqueClicks), B: float64(b.UniqueClicks), HigherIsBetter: true},
		{Name: "hard_bounces", A: float64(a.HardBounces), B: float64(b.HardBounces), HigherIsBetter: false},
		{Name: "soft_bounces", A: float64(a.SoftBounces), B: float64(b.SoftBounces), HigherIsBetter: false},
		{Name: "unsubscribed", A: float64(a.Unsubscribes), B: float64(b.Unsubscribes), HigherIsBetter: false},
		{Name: "abuse_reports", A: float64(a.AbuseReports), B: float64(b.AbuseReports), HigherIsBetter: false},
		{Name: "total_revenue", A: a.Revenue, B: b.Revenue, HigherIsBetter: true},
	}
	for i := range rows {
		rows[i].computeWinner()
	}
	return rows
}

// fmtMetric formats a metric value for display. Percent metrics get a "%"
// suffix and 2 decimal places; revenue gets a "$" prefix; counts are integer.
func fmtMetric(v float64, isPct bool, name string) string {
	if isPct {
		return fmt.Sprintf("%.2f%%", v*100)
	}
	if name == "total_revenue" {
		return fmt.Sprintf("$%.2f", v)
	}
	return fmt.Sprintf("%d", int(v))
}

func renderCompareMarkdown(a, b campaignReportSnapshot, rows []metricRow) string {
	var sb strings.Builder
	sb.WriteString("# Campaign comparison\n\n")
	sb.WriteString(fmt.Sprintf("**A** — `%s`", a.CampaignID))
	if a.Subject != "" {
		sb.WriteString(fmt.Sprintf(" — %q", a.Subject))
	}
	if a.SendTime != "" {
		sb.WriteString(fmt.Sprintf(" (sent %s)", a.SendTime))
	}
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("**B** — `%s`", b.CampaignID))
	if b.Subject != "" {
		sb.WriteString(fmt.Sprintf(" — %q", b.Subject))
	}
	if b.SendTime != "" {
		sb.WriteString(fmt.Sprintf(" (sent %s)", b.SendTime))
	}
	sb.WriteString("\n\n")
	sb.WriteString("| Metric | A | B | Winner |\n|---|---|---|---|\n")
	for _, r := range rows {
		winner := r.Winner
		switch winner {
		case "a":
			winner = "**A**"
		case "b":
			winner = "**B**"
		case "tie":
			winner = "tie"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			r.Name,
			fmtMetric(r.A, r.IsPct, r.Name),
			fmtMetric(r.B, r.IsPct, r.Name),
			winner,
		))
	}
	// Count winners
	aWins, bWins, ties := 0, 0, 0
	for _, r := range rows {
		switch r.Winner {
		case "a":
			aWins++
		case "b":
			bWins++
		case "tie":
			ties++
		}
	}
	sb.WriteString(fmt.Sprintf("\n**Tally:** A wins %d, B wins %d, %d ties.\n", aWins, bWins, ties))
	return sb.String()
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var md bool

	cmd := &cobra.Command{
		Use:   "compare <campaign-a> <campaign-b>",
		Short: "Head-to-head metric diff for two campaigns. Picks a winner per metric.",
		Long: `Fetch two campaigns' reports in parallel and render a side-by-side metric diff.
For each metric (open rate, click rate, click-to-open, bounces, unsubscribes,
revenue), the row marks A or B as the winner. Mailchimp's dashboard makes you
load both reports in two tabs and eyeball the deltas.

Output:
  --json  (default for agents)  structured snapshot + metrics array
  --md                          paste-ready markdown for Notion/Slack/weekly review docs
  human terminal                a clean table with the winner highlighted`,
		Example: `  mailchimp-pp-cli compare 7f8a9b0c1d 8c9d0e1f2a
  mailchimp-pp-cli compare 7f8a9b0c1d 8c9d0e1f2a --md
  mailchimp-pp-cli compare 7f8a9b0c1d 8c9d0e1f2a --json --select metrics`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if len(args) == 0 {
					return cmd.Help()
				}
				return fmt.Errorf("compare needs two campaign IDs: compare <a> <b>")
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_compare": map[string]any{
						"GET": []string{
							fmt.Sprintf("/reports/%s", args[0]),
							fmt.Sprintf("/reports/%s", args[1]),
						},
					},
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Parallel fetch — saves wall-clock; the 10-concurrent-connection limit
			// is far above 2 so no need to serialize.
			type fetchResult struct {
				idx  int
				data json.RawMessage
				err  error
			}
			results := make(chan fetchResult, 2)
			var wg sync.WaitGroup
			for i, id := range []string{args[0], args[1]} {
				wg.Add(1)
				go func(idx int, cid string) {
					defer wg.Done()
					data, gerr := c.Get(fmt.Sprintf("/reports/%s", cid), nil)
					results <- fetchResult{idx: idx, data: data, err: gerr}
				}(i, id)
			}
			wg.Wait()
			close(results)

			var snapshots [2]campaignReportSnapshot
			for r := range results {
				if r.err != nil {
					return classifyAPIError(r.err, flags)
				}
				snapshots[r.idx] = extractCampaignReport(r.data, []string{args[0], args[1]}[r.idx])
			}

			rows := compareMetrics(snapshots[0], snapshots[1])

			if md {
				_, werr := cmd.OutOrStdout().Write([]byte(renderCompareMarkdown(snapshots[0], snapshots[1], rows)))
				return werr
			}

			// Structured output for --json, terminal table, etc.
			out := map[string]any{
				"a":       snapshots[0],
				"b":       snapshots[1],
				"metrics": rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&md, "md", false, "Render as paste-ready markdown for Notion/Slack/client weekly docs")
	return cmd
}
