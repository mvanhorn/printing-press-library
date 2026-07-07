// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the FRED CLI. Carried across regen via the
// novel-command merge path; the generated stub was replaced.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// observationsEnvelope is the FRED /series/observations response shape.
type observationsEnvelope struct {
	Count        int               `json:"count"`
	Observations []fredObservation `json:"observations"`
}

type fredObservation struct {
	Date  string `json:"date"`
	Value string `json:"value"`
}

// latestView is the agent-native one-liner result.
type latestView struct {
	SeriesID string `json:"series_id"`
	Date     string `json:"date"`
	Value    string `json:"value"`
}

func fetchLatestObservation(cmd *cobra.Command, flags *rootFlags, seriesID string) (*latestView, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/series/observations", map[string]string{
		"series_id":  seriesID,
		"file_type":  "json",
		"sort_order": "desc",
		"limit":      "1",
	})
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var env observationsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, apiErr(fmt.Errorf("parsing observations for %s: %w", seriesID, err))
	}
	if len(env.Observations) == 0 {
		return nil, notFoundErr(fmt.Errorf("no observations found for series %q", seriesID))
	}
	obs := env.Observations[0]
	return &latestView{SeriesID: seriesID, Date: obs.Date, Value: obs.Value}, nil
}

// pp:data-source live
func newNovelSeriesLatestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "latest <series_id>",
		Short:       "Most recent observation (date + value) for a series",
		Long:        "Fetch only the single most recent observation for a series — the current print of an indicator — without pulling or parsing its full history.",
		Example:     "  fred-pp-cli series latest UNRATE --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a series id is required, e.g. UNRATE"))
			}
			view, err := fetchLatestObservation(cmd, flags, args[0])
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, view)
		},
	}
	return cmd
}
