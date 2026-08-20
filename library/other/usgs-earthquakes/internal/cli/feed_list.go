// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// feedCatalog is the static list of all 20 USGS GeoJSON summary feeds.
// USGS updates these every minute; cadence and approximate row counts are
// documented at https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/.
var feedCatalog = []feedEntry{
	{"significant_hour", "significant", "hour", "Significant events, past hour"},
	{"significant_day", "significant", "day", "Significant events, past day"},
	{"significant_week", "significant", "week", "Significant events, past week"},
	{"significant_month", "significant", "month", "Significant events, past month"},
	{"4.5_hour", "M4.5+", "hour", "M4.5+, past hour"},
	{"4.5_day", "M4.5+", "day", "M4.5+, past day"},
	{"4.5_week", "M4.5+", "week", "M4.5+, past week"},
	{"4.5_month", "M4.5+", "month", "M4.5+, past month"},
	{"2.5_hour", "M2.5+", "hour", "M2.5+, past hour"},
	{"2.5_day", "M2.5+", "day", "M2.5+, past day"},
	{"2.5_week", "M2.5+", "week", "M2.5+, past week"},
	{"2.5_month", "M2.5+", "month", "M2.5+, past month"},
	{"1.0_hour", "M1.0+", "hour", "M1.0+, past hour"},
	{"1.0_day", "M1.0+", "day", "M1.0+, past day"},
	{"1.0_week", "M1.0+", "week", "M1.0+, past week"},
	{"1.0_month", "M1.0+", "month", "M1.0+, past month"},
	{"all_hour", "all magnitudes", "hour", "Every event, past hour"},
	{"all_day", "all magnitudes", "day", "Every event, past day"},
	{"all_week", "all magnitudes", "week", "Every event, past week"},
	{"all_month", "all magnitudes", "month", "Every event, past month"},
}

type feedEntry struct {
	Name        string `json:"name"`
	Level       string `json:"level"`
	Period      string `json:"period"`
	Description string `json:"description"`
}

// pp:novel-static-reference
func newFeedListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "feed-list",
		Short:       "List all 20 USGS summary feed names with level, period, and description",
		Example:     "  usgs-earthquakes-pp-cli feed-list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.asJSON {
				raw, _ := json.Marshal(feedCatalog)
				return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(raw), flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "NAME\tLEVEL\tPERIOD\tDESCRIPTION")
			for _, f := range feedCatalog {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Name, f.Level, f.Period, f.Description)
			}
			return w.Flush()
		},
	}
}

// validFeedName returns true if name matches one of the 20 published feeds.
func validFeedName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, f := range feedCatalog {
		if f.Name == name {
			return true
		}
	}
	return false
}
