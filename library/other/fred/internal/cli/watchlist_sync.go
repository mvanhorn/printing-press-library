// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the FRED CLI. Carried across regen via the
// novel-command merge path; the generated stub was replaced.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// watchlistChange is the per-series result of a sync.
type watchlistChange struct {
	SeriesID string `json:"series_id"`
	Date     string `json:"date"`
	Value    string `json:"value"`
	PrevDate string `json:"prev_date,omitempty"`
	PrevVal  string `json:"prev_value,omitempty"`
	Changed  bool   `json:"changed"`
	Error    string `json:"error,omitempty"`
}

type watchlistSyncView struct {
	Synced        int               `json:"synced"`
	ChangedCount  int               `json:"changed_count"`
	Results       []watchlistChange `json:"results"`
	FetchFailures []fetchFail       `json:"fetch_failures,omitempty"`
}

// pp:data-source live
func newNovelWatchlistSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sync",
		Short:       "Sync watched series' latest values and report what changed",
		Long:        "Fetch the latest observation for every series on the watchlist, compare it to the value stored at the previous sync, persist the new values, and report which series moved. Only the series whose latest print differs are flagged as changed.",
		Example:     "  fred-pp-cli watchlist sync --json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := loadWatchlist()
			if err != nil {
				return err
			}
			ids := sortedSeriesIDs(st)
			if len(ids) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "watchlist is empty; add series with: fred-pp-cli watchlist add <series_id>")
				return flags.printJSON(cmd, watchlistSyncView{Results: []watchlistChange{}})
			}

			results := make([]watchlistChange, 0, len(ids))
			failures := make([]fetchFail, 0)
			changed := 0
			for _, id := range ids {
				prev := st.Series[id]
				latest, err := fetchLatestObservation(cmd, flags, id)
				if err != nil {
					failures = append(failures, fetchFail{SeriesID: id, Error: err.Error()})
					results = append(results, watchlistChange{SeriesID: id, Error: err.Error(), PrevDate: prev.Date, PrevVal: prev.Value})
					continue
				}
				didChange := prev.Date != latest.Date || prev.Value != latest.Value
				if didChange {
					changed++
				}
				results = append(results, watchlistChange{
					SeriesID: id,
					Date:     latest.Date,
					Value:    latest.Value,
					PrevDate: prev.Date,
					PrevVal:  prev.Value,
					Changed:  didChange,
				})
				st.Series[id] = watchlistEntry{SeriesID: id, Date: latest.Date, Value: latest.Value}
			}

			if err := saveWatchlist(st); err != nil {
				return err
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d series failed to sync\n", len(failures), len(ids))
			}

			return flags.printJSON(cmd, watchlistSyncView{
				Synced:        len(ids) - len(failures),
				ChangedCount:  changed,
				Results:       results,
				FetchFailures: failures,
			})
		},
	}
	return cmd
}
