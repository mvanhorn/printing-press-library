package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/ollama-cloud/internal/advisor"
	"github.com/spf13/cobra"
)

func newCostTraceCmd(flags *rootFlags) *cobra.Command {
	var (
		logPath  string
		since    string
		groupBy  string
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "cost-trace",
		Short: "Aggregate advisor-log cost estimates over a time window",
		Long: strings.TrimSpace(`
Reads the advisor JSONL log and emits per-model or per-task-hint cost rollups.
Use to decide whether the free Ollama Cloud tier is enough or an upgrade pays
off; or to spot anomalies (a single task-hint dominating spend).
`),
		Example: strings.Trim(`
  ollama-cloud-pp-cli cost-trace --since 7d --group-by model
  ollama-cloud-pp-cli cost-trace --since 24h --group-by task-hint
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := logPath
			if path == "" {
				path = advisor.DefaultLogPath()
			}
			cutoff, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			rows, err := readAdvisorLog(path, cutoff)
			if err != nil {
				return apiErr(err)
			}
			groups := groupRows(rows, groupBy)
			if limit > 0 && len(groups) > limit {
				groups = groups[:limit]
			}
			envelope := map[string]any{
				"log_path":     path,
				"since":        since,
				"group_by":     groupBy,
				"row_count":    len(rows),
				"groups":       groups,
				"computed_at":  time.Now().UTC(),
			}
			out, _ := json.MarshalIndent(envelope, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&logPath, "log", "", "Override advisor log path")
	cmd.Flags().StringVar(&since, "since", "7d", "Time window: 7d, 24h, 1h, all")
	cmd.Flags().StringVar(&groupBy, "group-by", "model", "Grouping: model | task-hint")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap result rows (0 = no cap)")
	return cmd
}

type costGroup struct {
	Key         string  `json:"key"`
	Count       int     `json:"count"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	AvgLatencyMs int     `json:"avg_latency_ms"`
	TiebreakUses int     `json:"tiebreak_uses"`
}

func groupRows(rows []advisor.LogEntry, groupBy string) []costGroup {
	bucket := map[string]*costGroup{}
	for _, r := range rows {
		var key string
		switch groupBy {
		case "task-hint":
			key = r.TaskHint
			if key == "" {
				key = "(none)"
			}
		default:
			key = r.Recommended
		}
		g, ok := bucket[key]
		if !ok {
			g = &costGroup{Key: key}
			bucket[key] = g
		}
		g.Count++
		g.TotalCostUSD += r.EstCostUSD
		g.AvgLatencyMs += r.EstLatencyMs
		if r.TiebreakUsed {
			g.TiebreakUses++
		}
	}
	out := make([]costGroup, 0, len(bucket))
	for _, g := range bucket {
		if g.Count > 0 {
			g.AvgLatencyMs /= g.Count
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalCostUSD > out[j].TotalCostUSD })
	return out
}

func readAdvisorLog(path string, cutoff time.Time) ([]advisor.LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	var out []advisor.LogEntry
	for scan.Scan() {
		line := scan.Bytes()
		if len(line) == 0 {
			continue
		}
		var e advisor.LogEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines, don't fail the whole trace
		}
		if !cutoff.IsZero() && e.AdvisedAt.Before(cutoff) {
			continue
		}
		out = append(out, e)
	}
	return out, scan.Err()
}

func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "all" {
		return time.Time{}, nil
	}
	d, err := time.ParseDuration(s)
	if err == nil {
		return time.Now().Add(-d), nil
	}
	// Accept "7d", "30d"
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Now().AddDate(0, 0, -days), nil
		}
	}
	return time.Time{}, fmt.Errorf("--since: bad duration %q (use 24h, 7d, all)", s)
}
