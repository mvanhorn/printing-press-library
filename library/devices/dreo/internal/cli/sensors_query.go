// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/store"

	"github.com/spf13/cobra"
)

func newSensorsQueryCmd(rflags *rootFlags) *cobra.Command {
	var (
		device string
		metric string
		since  time.Duration
		until  time.Duration
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query the local sensor_readings table",
		Long: `Read rows from the local sensor_readings table populated by
'sensors record'. Filter by device sn, metric name, and a time window.

--since and --until are durations relative to "now"; --since 1h returns
rows from the last hour.`,
		Example: `  dreo-pp-cli sensors query --since 1h
  dreo-pp-cli sensors query --metric temperature --device HTF...
  dreo-pp-cli sensors query --since 24h --limit 100 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			var sinceT, untilT time.Time
			if since > 0 {
				sinceT = time.Now().Add(-since)
			}
			if until > 0 {
				untilT = time.Now().Add(-until)
			}
			rows, err := st.QuerySensorReadings(ctx, device, sinceT, untilT, metric, limit)
			if err != nil {
				return err
			}
			// Guarantee non-nil slice so --json renders `[]` (matching the
			// plain-text "No readings match." path), not `null`. JSON
			// consumers that iterate the result get a 0-length no-op
			// instead of a nil-type error.
			if rows == nil {
				rows = []store.Reading{}
			}

			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, rflags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No readings match. Run `dreo-pp-cli sensors record` to populate.")
				return nil
			}
			headers := []string{"TIME", "SN", "METRIC", "VALUE"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					r.Ts.Format(time.RFC3339),
					r.Sn,
					r.Metric,
					fmt.Sprintf("%g", r.Value),
				})
			}
			return rflags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&device, "device", "", "Restrict to one device sn")
	cmd.Flags().StringVar(&metric, "metric", "", "Restrict to one metric (temperature, humidity, pm25, ...)")
	cmd.Flags().DurationVar(&since, "since", 0, "Lookback duration (e.g. 1h, 24h)")
	cmd.Flags().DurationVar(&until, "until", 0, "Exclude rows newer than this duration ago")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max rows to return")
	return cmd
}
