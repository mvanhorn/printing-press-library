// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newCdpReverseEtlHealthCmd is the named verb over Reverse-ETL run history.
// The official cio CLI exposes RETL only via generic api passthrough; this
// command surfaces status, row counts, and error reasons per sync, with an
// optional --watch poll mode.
//
// Note: Reverse-ETL is a Premium-tier Customer.io feature. Endpoints are
// reachable but return 403 on Essentials accounts; the command surfaces the
// 403 with an actionable error.
func newCdpReverseEtlHealthCmd(flags *rootFlags) *cobra.Command {
	var watch bool
	var since string
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show Reverse-ETL sync health: status, row counts, error reasons (optionally --watch every 60s)",
		Long: `Read the live Reverse-ETL syncs endpoint, group runs by sync ID, and
emit one row per sync with the latest status, row count, and any error
reason. With --watch, polls every 60 seconds until cancelled.

Reverse-ETL requires a Premium-tier workspace. If the endpoint returns 403,
upgrade or check 'customer-io-pp-cli workspaces current'.`,
		Example: strings.Trim(`
  customer-io-pp-cli cdp-reverse-etl health
  customer-io-pp-cli cdp-reverse-etl health --since 24h --json
  customer-io-pp-cli cdp-reverse-etl health --watch
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			emit := func() error {
				report, err := buildReverseETLHealth(c, since)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), report, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Reverse-ETL syncs (%d)\n\n", len(report["syncs"].([]map[string]any)))
				for _, s := range report["syncs"].([]map[string]any) {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-30s status=%-12s last_run=%-20s rows=%-8d %s\n",
						s["sync_id"], s["status"], s["last_run"], s["last_row_count"], s["last_error"])
				}
				return nil
			}

			if !watch {
				return emit()
			}
			if interval <= 0 {
				interval = 60 * time.Second
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			for {
				if err := emit(); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll continuously every --interval (60s default) until cancelled")
	cmd.Flags().StringVar(&since, "since", "", "Only count runs newer than this duration (e.g. 1h, 24h, 7d)")
	cmd.Flags().DurationVar(&interval, "interval", 60*time.Second, "Polling interval for --watch")
	return cmd
}

func buildReverseETLHealth(c clientGetter, since string) (map[string]any, error) {
	cutoff, err := parseSinceCutoff(since)
	if err != nil {
		return nil, err
	}
	data, err := c.Get("/cdp/api/reverse_etl/syncs", nil)
	if err != nil {
		return nil, err
	}
	type retlSync struct {
		ID       string          `json:"id"`
		Name     string          `json:"name"`
		Status   string          `json:"status"`
		Schedule string          `json:"schedule"`
		Enabled  bool            `json:"enabled"`
		Latest   json.RawMessage `json:"latest_run"`
	}
	// The /cdp/api/reverse_etl/syncs endpoint returns either an envelope
	// ({"syncs":[...]} or {"items":[...]}) or a bare array depending on
	// account configuration. Try array first since unmarshalling an array
	// into the envelope struct hard-fails before the fallback runs.
	var syncs []retlSync
	var arr []retlSync
	if err := json.Unmarshal(data, &arr); err == nil {
		syncs = arr
	} else {
		var raw struct {
			Syncs []retlSync `json:"syncs"`
			Items []retlSync `json:"items"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("decoding syncs: %w", err)
		}
		syncs = raw.Syncs
		if len(syncs) == 0 {
			syncs = raw.Items
		}
	}
	rows := make([]map[string]any, 0, len(syncs))
	healthy, unhealthy := 0, 0
	for _, s := range syncs {
		var last struct {
			Status     string `json:"status"`
			RowCount   int    `json:"row_count"`
			Error      string `json:"error"`
			StartedAt  int64  `json:"started_at"`
			FinishedAt int64  `json:"finished_at"`
		}
		_ = json.Unmarshal(s.Latest, &last)
		if cutoff > 0 && last.StartedAt > 0 && last.StartedAt < cutoff {
			continue
		}
		row := map[string]any{
			"sync_id":        s.ID,
			"name":           s.Name,
			"enabled":        s.Enabled,
			"schedule":       s.Schedule,
			"status":         nonEmpty(s.Status, last.Status),
			"last_run":       formatUnixOrEmpty(last.StartedAt),
			"last_row_count": last.RowCount,
			"last_error":     last.Error,
		}
		rows = append(rows, row)
		switch strings.ToLower(nonEmpty(s.Status, last.Status)) {
		case "succeeded", "running", "scheduled", "ok":
			healthy++
		case "":
			// Unknown — neither.
		default:
			unhealthy++
		}
	}
	return map[string]any{
		"window":    since,
		"syncs":     rows,
		"healthy":   healthy,
		"unhealthy": unhealthy,
		"total":     len(rows),
	}, nil
}

func formatUnixOrEmpty(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}
