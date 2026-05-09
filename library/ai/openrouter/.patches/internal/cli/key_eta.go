// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/store"

	"github.com/spf13/cobra"
)

func newKeyEtaCmd(flags *rootFlags) *cobra.Command {
	var llm bool

	cmd := &cobra.Command{
		Use:         "eta",
		Short:       "Project when the OpenRouter cap will trip from /key + 7d burn rate",
		Example:     "  openrouter-pp-cli key eta --llm",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "{\"limit\":0,\"used\":0,\"eta\":\"n/a\"}")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/key", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var keyEnvelope struct {
				Data map[string]any `json:"data"`
			}
			key := map[string]any{}
			if json.Unmarshal(data, &keyEnvelope) == nil && keyEnvelope.Data != nil {
				key = keyEnvelope.Data
			} else if err := json.Unmarshal(data, &key); err != nil {
				return apiErr(err)
			}
			limit := asFloat(key["limit"])
			usage := asFloat(key["usage"])
			limitReset := asString(key["limit_reset"])
			remaining := limit - usage
			if v, ok := key["limit_remaining"]; ok {
				remaining = asFloat(v)
			}

			// Burn rate from activity table over 7d.
			burnPerDay := 0.0
			dbPath := defaultDBPath("openrouter-pp-cli")
			if db, err := store.OpenWithContext(context.Background(), dbPath); err == nil {
				defer db.Close()
				since := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
				row := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COALESCE(SUM(usage),0) FROM activity WHERE date >= ?`, since)
				var total float64
				_ = row.Scan(&total)
				burnPerDay = total / 7.0
			}

			etaStr := "n/a"
			resetStr := "n/a"
			if limitReset != "never" && limit > 0 && burnPerDay > 0 && remaining > 0 {
				daysToHit := remaining / burnPerDay
				etaTime := time.Now().Add(time.Duration(daysToHit*24) * time.Hour)
				etaStr = etaTime.UTC().Format(time.RFC3339)
			}
			if limitReset != "" && limitReset != "never" {
				now := time.Now().UTC()
				var reset time.Time
				switch limitReset {
				case "daily":
					reset = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
				case "weekly":
					daysToMon := (8 - int(now.Weekday())) % 7
					if daysToMon == 0 {
						daysToMon = 7
					}
					reset = time.Date(now.Year(), now.Month(), now.Day()+daysToMon, 0, 0, 0, 0, time.UTC)
				case "monthly":
					reset = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
				}
				if !reset.IsZero() {
					resetStr = reset.Format(time.RFC3339)
				}
			}

			out := map[string]any{
				"limit":        limit,
				"used":         usage,
				"remaining":    remaining,
				"limit_reset":  limitReset,
				"burn_per_day": burnPerDay,
				"eta":          etaStr,
				"reset":        resetStr,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if llm {
				fmt.Fprintf(cmd.OutOrStdout(),
					"limit=$%.2f used=$%.2f remaining=$%.2f burn_rate=$%.2f/day eta=%s reset=%s\n",
					limit, usage, remaining, burnPerDay, etaStr, resetStr)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&llm, "llm", false, "Terse k:v output")
	return cmd
}
