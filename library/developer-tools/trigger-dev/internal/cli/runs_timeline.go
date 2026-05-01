package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newRunsTimelineCmd(flags *rootFlags) *cobra.Command {
	var period string
	var taskFilter string
	var width int

	cmd := &cobra.Command{
		Use:   "timeline",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Visual ASCII timeline of run starts, completions, and failures",
		Long: `Display an ASCII timeline of runs over a time period. Each character represents
a time bucket. Shows patterns in run timing and failure clusters at a glance.

Legend: . = ok, X = failed, # = crashed, - = no runs`,
		Example: `  # Timeline of all runs in the last 24 hours
  trigger-dev-pp-cli runs timeline --period 24h

  # Timeline for a specific task
  trigger-dev-pp-cli runs timeline --task my-task --period 12h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would display timeline for period=%s\n", period)
				return nil
			}

			params := map[string]string{
				"page[size]":                "100",
				"filter[createdAt][period]": period,
			}
			if taskFilter != "" {
				params["taskIdentifier"] = taskFilter
			}

			resp, err := c.Get("/api/v1/runs", params)
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

			type run struct {
				Status    string    `json:"status"`
				CreatedAt time.Time `json:"createdAt"`
			}

			var runs []run
			for _, raw := range envelope.Data {
				var r run
				if err := json.Unmarshal(raw, &r); err == nil {
					runs = append(runs, r)
				}
			}

			if len(runs) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No runs found in the last %s.\n", period)
				return nil
			}

			// Determine time range
			now := time.Now()
			var duration time.Duration
			switch {
			case strings.HasSuffix(period, "h"):
				h := 0
				fmt.Sscanf(period, "%dh", &h)
				duration = time.Duration(h) * time.Hour
			case strings.HasSuffix(period, "d"):
				d := 0
				fmt.Sscanf(period, "%dd", &d)
				duration = time.Duration(d) * 24 * time.Hour
			default:
				duration = 24 * time.Hour
			}
			start := now.Add(-duration)
			bucketDur := duration / time.Duration(width)

			// Build timeline
			type bucket struct {
				ok     int
				failed int
			}
			buckets := make([]bucket, width)

			for _, r := range runs {
				if r.CreatedAt.Before(start) {
					continue
				}
				idx := int(r.CreatedAt.Sub(start) / bucketDur)
				if idx >= width {
					idx = width - 1
				}
				if idx < 0 {
					idx = 0
				}
				switch r.Status {
				case "FAILED", "CRASHED", "SYSTEM_FAILURE":
					buckets[idx].failed++
				default:
					buckets[idx].ok++
				}
			}

			// Render
			if flags.asJSON {
				type jsonBucket struct {
					Time   string `json:"time"`
					OK     int    `json:"ok"`
					Failed int    `json:"failed"`
				}
				var jb []jsonBucket
				for i, b := range buckets {
					t := start.Add(time.Duration(i) * bucketDur)
					jb = append(jb, jsonBucket{
						Time:   t.Local().Format("15:04"),
						OK:     b.ok,
						Failed: b.failed,
					})
				}
				return flags.printJSON(cmd, jb)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Run Timeline (last %s)\n", period)
			fmt.Fprintf(cmd.OutOrStdout(), "Legend: . = ok, X = fail, - = none\n\n")

			// Print timeline bar
			var bar strings.Builder
			for _, b := range buckets {
				if b.failed > 0 {
					bar.WriteByte('X')
				} else if b.ok > 0 {
					bar.WriteByte('.')
				} else {
					bar.WriteByte('-')
				}
			}

			startLabel := start.Local().Format("15:04")
			endLabel := now.Local().Format("15:04")
			fmt.Fprintf(cmd.OutOrStdout(), "%s |%s| %s\n", startLabel, bar.String(), endLabel)

			// Summary
			totalOK, totalFail := 0, 0
			for _, b := range buckets {
				totalOK += b.ok
				totalFail += b.failed
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d runs: %d ok, %d failed\n", totalOK+totalFail, totalOK, totalFail)

			return nil
		},
	}

	cmd.Flags().StringVar(&period, "period", "24h", "Time period (e.g., 6h, 12h, 24h, 7d)")
	cmd.Flags().StringVar(&taskFilter, "task", "", "Filter by task identifier")
	cmd.Flags().IntVar(&width, "width", 60, "Timeline width in characters")

	return cmd
}
