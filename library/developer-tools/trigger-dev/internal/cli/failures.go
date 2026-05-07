// Hand-authored novel feature: top recurring failure patterns.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newFailuresCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "failures",
		Short: "Aggregations over failed runs (top recurring patterns)",
	}
	cmd.AddCommand(newFailuresTopCmd(flags))
	return cmd
}

func newFailuresTopCmd(flags *rootFlags) *cobra.Command {
	var period string
	var topN int

	cmd := &cobra.Command{
		Use:         "top",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Top recurring (task, error-signature) failure patterns over a window",
		Long: `Walks failed runs in the window, normalizes error messages by stripping
IDs/numbers/hex tokens/UUIDs, then groups by (task, signature). Mechanical —
no LLM, no NLP.`,
		Example: `  trigger-dev-pp-cli failures top --since 7d --top 20
  trigger-dev-pp-cli failures top --since 1d --json --select 'rows.task,rows.signature,rows.count'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would aggregate failures over %s, top %d\n", period, topN)
				return nil
			}
			resp, err := c.Get("/api/v1/runs", map[string]string{
				"status":                    "FAILED,CRASHED,SYSTEM_FAILURE",
				"page[size]":                "100",
				"filter[createdAt][period]": period,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type pattern struct {
				Task      string `json:"task"`
				Signature string `json:"signature"`
				Count     int    `json:"count"`
				LastSeen  string `json:"last_seen"`
				Status    string `json:"status"`
			}
			groups := map[string]*pattern{}
			for _, raw := range unwrapEnvelope(resp) {
				var run struct {
					ID             string    `json:"id"`
					Status         string    `json:"status"`
					TaskIdentifier string    `json:"taskIdentifier"`
					CreatedAt      time.Time `json:"createdAt"`
					Error          struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				if err := json.Unmarshal(raw, &run); err != nil {
					continue
				}
				signature := normalizeErrorSignature(run.Error.Message)
				if signature == "" {
					signature = run.Status
				}
				key := run.TaskIdentifier + "|" + signature
				p, ok := groups[key]
				if !ok {
					p = &pattern{Task: run.TaskIdentifier, Signature: signature, Status: run.Status}
					groups[key] = p
				}
				p.Count++
				ts := run.CreatedAt.Local().Format("2006-01-02 15:04")
				if ts > p.LastSeen {
					p.LastSeen = ts
				}
			}
			rows := make([]*pattern, 0, len(groups))
			for _, p := range groups {
				rows = append(rows, p)
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Count != rows[j].Count {
					return rows[i].Count > rows[j].Count
				}
				return rows[i].LastSeen > rows[j].LastSeen
			})
			if topN > 0 && topN < len(rows) {
				rows = rows[:topN]
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"period":          period,
					"unique_patterns": len(groups),
					"rows":            rows,
				}, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No failures in the last %s.\n", period)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Top failure patterns (last %s) — %d unique groupings\n\n", period, len(groups))
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-50s %5s  %s\n", "task", "signature", "count", "last seen")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 110))
			for _, p := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-50s %5d  %s\n",
					truncate(p.Task, 30), truncate(p.Signature, 50), p.Count, p.LastSeen)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&period, "since", "7d", "Time window (1d, 7d, 30d)")
	cmd.Flags().IntVar(&topN, "top", 20, "Show only the top N rows")
	return cmd
}

var (
	reSigUUID    = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	reSigHex     = regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`)
	reSigPrefix  = regexp.MustCompile(`\b[a-z]{2,5}_[A-Za-z0-9]{8,}\b`)
	reSigNumeric = regexp.MustCompile(`\b\d{2,}\b`)
	reSigQuoted  = regexp.MustCompile(`"[^"]*"`)
)

func normalizeErrorSignature(msg string) string {
	if msg == "" {
		return ""
	}
	s := strings.TrimSpace(msg)
	s = reSigUUID.ReplaceAllString(s, "<uuid>")
	s = reSigHex.ReplaceAllString(s, "<hex>")
	s = reSigPrefix.ReplaceAllString(s, "<id>")
	s = reSigNumeric.ReplaceAllString(s, "<n>")
	s = reSigQuoted.ReplaceAllString(s, `"<v>"`)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
