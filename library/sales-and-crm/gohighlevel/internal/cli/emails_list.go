// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-05-20: email list — GET /emails/schedule?showStats=true with delivered/opened/clicked/openRate)

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newEmailsListCmd(flags *rootFlags) *cobra.Command {
	var flagLocationId string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List email campaigns with delivery statistics",
		Example: "  gohighlevel-pp-cli email list --location-id F9YlSB15qA1pRCrPsTSw",
		Long: `List email campaigns including delivery statistics (delivered, opened, clicked counts and
calculated open-rate percentage).

Calls GET /emails/schedule?showStats=true. Requires a location ID set via --location-id,
the global --location flag, or the active profile.`,
		Annotations: map[string]string{
			"pp:endpoint":   "emails.list",
			"pp:method":     "GET",
			"pp:path":       "/emails/schedule",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve location ID: flag > global profile
			if flagLocationId == "" {
				flagLocationId = resolveLocationID()
			}
			if flagLocationId == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"location-id\" not set (or set via --location / active profile)")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/emails/schedule"
			params := map[string]string{
				"showStats": "true",
			}
			if flagLocationId != "" {
				params["locationId"] = flagLocationId
			}

			data, prov, err := resolveRead(cmd.Context(), c, flags, "emails", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Human-friendly table: surface key campaign stats
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var raw map[string]json.RawMessage
				if json.Unmarshal(data, &raw) == nil {
					if campaignsRaw, ok := raw["campaigns"]; ok {
						var campaigns []map[string]any
						if json.Unmarshal(campaignsRaw, &campaigns) == nil && len(campaigns) > 0 {
							printProvenance(cmd, len(campaigns), prov)
							headers := []string{"ID", "NAME", "STATUS", "DELIVERED", "OPENED", "CLICKED", "OPEN RATE %"}
							var rows [][]string
							for _, camp := range campaigns {
								id := fmt.Sprintf("%v", camp["id"])
								name := fmt.Sprintf("%v", camp["name"])
								status := fmt.Sprintf("%v", camp["status"])

								delivered := extractStatInt(camp, "stats", "delivered")
								opened := extractStatInt(camp, "stats", "opened")
								clicked := extractStatInt(camp, "stats", "clicked")

								openRateStr := "—"
								if delivered > 0 {
									rate := float64(opened) / float64(delivered) * 100
									openRateStr = strconv.FormatFloat(rate, 'f', 1, 64) + "%"
								}

								rows = append(rows, []string{
									id,
									truncate(name, 40),
									status,
									strconv.FormatInt(delivered, 10),
									strconv.FormatInt(opened, 10),
									strconv.FormatInt(clicked, 10),
									openRateStr,
								})
							}
							if err := flags.printTable(cmd, headers, rows); err != nil {
								return err
							}
							if len(campaigns) >= 25 {
								fmt.Fprintf(os.Stderr, "\nShowing %d results. Add --json for full response.\n", len(campaigns))
							}
							return nil
						}
					}
				}
			}

			// JSON / pipe / other format modes
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&flagLocationId, "location-id", "", "GHL sub-account (location) ID (overrides --location / active profile)")
	return cmd
}

// extractStatInt navigates a nested map path and returns the int64 value, or 0 on any miss.
// e.g. extractStatInt(campaign, "stats", "delivered") reads campaign["stats"]["delivered"].
func extractStatInt(m map[string]any, keys ...string) int64 {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return 0
		}
		cur = mm[k]
	}
	switch v := cur.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

