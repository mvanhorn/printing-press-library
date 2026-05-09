package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newDiffCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "diff <slug>",
		Short:   "Compare current snow report against the last cached version",
		Long:    "Fetches the current snow report and compares it against the locally cached version, showing what changed.",
		Example: "  opensnow-pp-cli diff vail\n  opensnow-pp-cli diff vail --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]

			// Get cached version
			db, err := openStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			cached, err := db.GetCachedSnowReport(slug)
			if err != nil {
				return fmt.Errorf("reading cached report: %w", err)
			}

			// Fetch current version
			c, cErr := flags.newClient()
			if cErr != nil {
				return cErr
			}

			path := "/snow-report/" + slug
			currentData, fetchErr := c.Get(path, map[string]string{})
			if fetchErr != nil {
				return classifyAPIError(fetchErr, flags)
			}
			currentData = extractResponseData(currentData)

			var currentObj map[string]any
			if err := json.Unmarshal(currentData, &currentObj); err != nil {
				return fmt.Errorf("parsing current report: %w", err)
			}

			// Cache the current report for next time
			_ = db.Upsert("snow-report", slug, currentData)

			if cached == nil {
				// First sync
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"slug":    slug,
						"status":  "first_sync",
						"message": "First sync — no previous data to compare",
						"current": currentObj,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "First sync — showing current report (no previous data to compare)")
				fmt.Fprintln(cmd.OutOrStdout())
				return printOutputWithFlags(cmd.OutOrStdout(), currentData, flags)
			}

			var cachedObj map[string]any
			if err := json.Unmarshal(cached, &cachedObj); err != nil {
				return fmt.Errorf("parsing cached report: %w", err)
			}

			type diffEntry struct {
				Field    string `json:"field"`
				Previous string `json:"previous"`
				Current  string `json:"current"`
				Change   string `json:"change"`
			}

			// Fields to compare
			compareFields := []struct {
				key   string
				label string
			}{
				{"operating_status", "Status"},
				{"status", "Status"},
				{"current_temperature", "Temp"},
				{"temp", "Temp"},
				{"snow_past_24h", "24h Snow"},
				{"new_snow_24", "24h Snow"},
				{"snow_past_48h", "48h Snow"},
				{"base_depth", "Base Depth"},
				{"snow_depth", "Base Depth"},
				{"lifts_open", "Lifts Open"},
				{"runs_open", "Runs Open"},
				{"conditions", "Conditions"},
				{"weather", "Weather"},
			}

			seen := map[string]bool{}
			var diffs []diffEntry
			for _, f := range compareFields {
				if seen[f.label] {
					continue
				}
				oldVal, oldOk := cachedObj[f.key]
				newVal, newOk := currentObj[f.key]
				if !oldOk && !newOk {
					continue
				}
				seen[f.label] = true
				oldStr := fmt.Sprintf("%v", oldVal)
				newStr := fmt.Sprintf("%v", newVal)
				if !oldOk {
					oldStr = "-"
				}
				if !newOk {
					newStr = "-"
				}

				if oldStr != newStr {
					change := fmt.Sprintf("%s -> %s", oldStr, newStr)
					// Calculate numeric diff if possible
					if oldF, ok := oldVal.(float64); ok {
						if newF, ok := newVal.(float64); ok {
							delta := newF - oldF
							sign := "+"
							if delta < 0 {
								sign = ""
							}
							change = fmt.Sprintf("%v -> %v (%s%.0f)", oldStr, newStr, sign, delta)
						}
					}
					diffs = append(diffs, diffEntry{
						Field:    f.label,
						Previous: oldStr,
						Current:  newStr,
						Change:   change,
					})
				}
			}

			if len(diffs) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"slug":    slug,
						"status":  "no_changes",
						"message": "No changes since last sync",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No changes since last sync")
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"slug":    slug,
					"status":  "changed",
					"changes": diffs,
				}, flags)
			}

			headers := []string{"Field", "Change"}
			tableRows := make([][]string, 0, len(diffs))
			for _, d := range diffs {
				tableRows = append(tableRows, []string{d.Field, d.Change})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
}
