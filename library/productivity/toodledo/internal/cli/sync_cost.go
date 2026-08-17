// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: sync cost preview. Calls account/get for the per-resource edit
// cursors and reports how many of the 100-call-per-token budget an incremental
// sync would spend, before fetching any rows. A planner, not a wrapper.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/toodledo/internal/cliutil"
	"github.com/spf13/cobra"
)

var syncCursorFields = map[string]string{
	"tasks":     "lastedit_task",
	"folders":   "lastedit_folder",
	"contexts":  "lastedit_context",
	"goals":     "lastedit_goal",
	"locations": "lastedit_location",
	"notes":     "lastedit_note",
	"outlines":  "lastedit_outline",
	"lists":     "lastedit_list",
}

var syncResourceOrder = []string{"tasks", "folders", "contexts", "goals", "locations", "notes", "outlines", "lists"}

type resourceCost struct {
	Resource         string `json:"resource"`
	UpstreamLastEdit int64  `json:"upstream_lastedit"`
	LocalSyncedAt    int64  `json:"local_synced_at"`
	WouldFetch       bool   `json:"would_fetch"`
	EstCalls         int    `json:"est_calls"`
}

type syncCostResult struct {
	TokenBudget    int            `json:"token_budget"`
	AccountCall    int            `json:"account_call"`
	Resources      []resourceCost `json:"resources"`
	ProjectedCalls int            `json:"projected_calls"`
	WithinBudget   bool           `json:"within_budget"`
	Note           string         `json:"note,omitempty"`
}

// pp:data-source live
func newNovelSyncCostCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagResources, dbPath string
	cmd := &cobra.Command{
		Use:   "sync-cost",
		Short: "Preview how many of your 100 per-token API calls a sync would spend",
		Long: `Call account/get for the per-resource edit cursors, compare them against the
local mirror's last-sync times, and report the projected number of API calls an
incremental sync would spend against Toodledo's hard 100-call-per-token budget —
without fetching any rows.`,
		Example: strings.Trim(`
  toodledo-pp-cli sync-cost
  toodledo-pp-cli sync-cost --since 7d
  toodledo-pp-cli sync-cost --resources tasks,notes --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			var sinceUnix int64 = -1 // -1 = compare to local sync time
			if strings.TrimSpace(flagSince) != "" {
				dur, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				sinceUnix = time.Now().Add(-dur).Unix()
			}
			selected := syncResourceOrder
			if strings.TrimSpace(flagResources) != "" {
				selected = nil
				for _, r := range strings.Split(flagResources, ",") {
					r = strings.TrimSpace(r)
					if r == "" {
						continue
					}
					if _, ok := syncCursorFields[r]; !ok {
						return usageErr(fmt.Errorf("unknown resource %q (known: %s)", r, strings.Join(syncResourceOrder, ", ")))
					}
					selected = append(selected, r)
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, "/account/get.php", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cursors := parseAccountCursors(data)

			localSynced := map[string]int64{}
			if db, ok, openErr := openLocalMirror(cmd, toodledoDBPath(dbPath)); openErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open local mirror (%v); treating all resources as unsynced\n", openErr)
			} else if ok {
				defer db.Close()
				for _, r := range selected {
					if _, ls, _, e := db.GetSyncState(r); e == nil && !ls.IsZero() {
						localSynced[r] = ls.Unix()
					}
				}
			}

			res := syncCostResult{TokenBudget: 100, AccountCall: 1, Resources: []resourceCost{}}
			projected := 1
			for _, r := range selected {
				upstream := cursors[syncCursorFields[r]]
				floor := localSynced[r]
				if sinceUnix >= 0 {
					floor = sinceUnix
				}
				would := upstream > 0 && upstream > floor
				est := 0
				if would {
					est = 1
					projected++
				}
				res.Resources = append(res.Resources, resourceCost{
					Resource: r, UpstreamLastEdit: upstream, LocalSyncedAt: localSynced[r],
					WouldFetch: would, EstCalls: est,
				})
			}
			res.ProjectedCalls = projected
			res.WithinBudget = projected <= res.TokenBudget
			res.Note = "Lower bound: each changed resource costs at least 1 call; a resource with >1000 changed rows pages and costs more."

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Projected sync cost: %d / %d token calls (1 account probe + %d resource fetches)\n",
				res.ProjectedCalls, res.TokenBudget, projected-1)
			tw := newTabWriter(w)
			fmt.Fprintln(tw, bold("RESOURCE\tFETCH\tUPSTREAM EDIT\tLOCAL SYNCED"))
			for _, rc := range res.Resources {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", rc.Resource, yesNo(rc.WouldFetch), unixOrDash(rc.UpstreamLastEdit), unixOrDash(rc.LocalSyncedAt))
			}
			_ = tw.Flush()
			fmt.Fprintf(w, "\n%s\n", res.Note)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Estimate cost of syncing changes in this window (e.g. 7d, 24h); default compares to last sync")
	cmd.Flags().StringVar(&flagResources, "resources", "", "Comma-separated resources to include (default: all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	return cmd
}

func parseAccountCursors(data json.RawMessage) map[string]int64 {
	out := map[string]int64{}
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return out
	}
	for k, raw := range obj {
		var n json.Number
		if json.Unmarshal(raw, &n) == nil {
			if v, err := n.Int64(); err == nil {
				out[k] = v
				continue
			}
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				out[k] = v
			}
		}
	}
	return out
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func unixOrDash(u int64) string {
	if u <= 0 {
		return "-"
	}
	return time.Unix(u, 0).UTC().Format("2006-01-02 15:04")
}
