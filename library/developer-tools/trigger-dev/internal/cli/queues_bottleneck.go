package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newQueuesBottleneckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bottleneck",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Identify queues with growing backlogs and concurrency bottlenecks",
		Long: `Analyze queue state to find bottlenecks: queues with high queued-to-running ratios,
queues near or at their concurrency limits, and paused queues with pending work.`,
		Example: `  # Show queue bottlenecks
  trigger-dev-pp-cli queues bottleneck

  # JSON output
  trigger-dev-pp-cli queues bottleneck --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would analyze queue bottlenecks\n")
				return nil
			}

			resp, err := c.Get("/api/v1/queues", nil)
			if err != nil {
				return classifyAPIError(err)
			}

			var envelope struct {
				Data []json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(resp, &envelope); err != nil {
				var arr []json.RawMessage
				if err2 := json.Unmarshal(resp, &arr); err2 == nil {
					envelope.Data = arr
				}
			}

			type queue struct {
				ID               string `json:"id"`
				Name             string `json:"name"`
				Type             string `json:"type"`
				Running          int    `json:"running"`
				Queued           int    `json:"queued"`
				Paused           bool   `json:"paused"`
				ConcurrencyLimit *int   `json:"concurrencyLimit"`
				Concurrency      struct {
					Current  int  `json:"current"`
					Base     *int `json:"base"`
					Override *int `json:"override"`
				} `json:"concurrency"`
			}

			type bottleneck struct {
				Name     string  `json:"name"`
				Running  int     `json:"running"`
				Queued   int     `json:"queued"`
				Limit    string  `json:"concurrency_limit"`
				Paused   bool    `json:"paused"`
				Severity string  `json:"severity"`
				Reason   string  `json:"reason"`
				Score    float64 `json:"-"`
			}

			var bottlenecks []bottleneck
			for _, raw := range envelope.Data {
				var q queue
				if err := json.Unmarshal(raw, &q); err != nil {
					continue
				}

				var issues []string
				var score float64

				limitStr := "unlimited"
				effectiveLimit := 0
				if q.Concurrency.Override != nil {
					effectiveLimit = *q.Concurrency.Override
					limitStr = fmt.Sprintf("%d (override)", effectiveLimit)
				} else if q.ConcurrencyLimit != nil {
					effectiveLimit = *q.ConcurrencyLimit
					limitStr = fmt.Sprintf("%d", effectiveLimit)
				} else if q.Concurrency.Base != nil {
					effectiveLimit = *q.Concurrency.Base
					limitStr = fmt.Sprintf("%d", effectiveLimit)
				}

				if q.Paused && q.Queued > 0 {
					issues = append(issues, fmt.Sprintf("paused with %d items queued", q.Queued))
					score += 10
				}

				if q.Queued > 0 && effectiveLimit > 0 && q.Running >= effectiveLimit {
					issues = append(issues, "at concurrency limit")
					score += float64(q.Queued)
				} else if q.Queued > q.Running*2 && q.Queued > 5 {
					issues = append(issues, "high backlog ratio")
					score += float64(q.Queued) / 2
				}

				if len(issues) > 0 {
					severity := "low"
					if score > 10 {
						severity = "medium"
					}
					if score > 50 {
						severity = "high"
					}

					bottlenecks = append(bottlenecks, bottleneck{
						Name:     q.Name,
						Running:  q.Running,
						Queued:   q.Queued,
						Limit:    limitStr,
						Paused:   q.Paused,
						Severity: severity,
						Reason:   strings.Join(issues, "; "),
						Score:    score,
					})
				}
			}

			sort.Slice(bottlenecks, func(i, j int) bool {
				return bottlenecks[i].Score > bottlenecks[j].Score
			})

			if flags.asJSON {
				return flags.printJSON(cmd, bottlenecks)
			}

			if len(bottlenecks) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No queue bottlenecks detected. All queues are healthy.\n")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Queue Bottlenecks (%d found)\n\n", len(bottlenecks))
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %8s %8s %-15s %-8s %s\n",
				"queue", "running", "queued", "limit", "severity", "reason")
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %8s %8s %-15s %-8s %s\n",
				strings.Repeat("-", 25), "--------", "--------", strings.Repeat("-", 15), "--------", strings.Repeat("-", 30))

			for _, b := range bottlenecks {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %8d %8d %-15s %-8s %s\n",
					truncate(b.Name, 25), b.Running, b.Queued, b.Limit, b.Severity, b.Reason)
			}

			return nil
		},
	}

	return cmd
}
