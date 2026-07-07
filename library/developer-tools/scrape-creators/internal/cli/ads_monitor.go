// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Novel command: snapshot a brand's live ads and diff against the last run.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

type adNetworkResult struct {
	Network string   `json:"network"`
	Count   int      `json:"count"`
	New     []string `json:"new"`
	Gone    []string `json:"gone"`
}

func newNovelAdsMonitorCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "monitor <brand>",
		Short:   "Snapshot a brand's live ads across Facebook, TikTok, Google, and LinkedIn; on rerun, diff new vs. gone ads.",
		Example: "  scrape-creators-pp-cli ads monitor nike",
		// pp:no-error-path-probe: any string is a valid brand to search, so an
		// unknown brand returns an empty ad set with exit 0, not an error — the
		// verifier's invalid-argument probe does not apply.
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		// No mcp:read-only hint: this command writes an ad snapshot to the local
		// store (a store update), so it is not "read-only" per the agent-native
		// tool-safety contract. A missing hint yields a permission prompt; a
		// false read-only hint on a writer is the documented real bug.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("brand is required"))
			}
			brand := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("scrape-creators-pp-cli")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			priorBatch, priorIDs, err := store.LatestAdSnapshot(ctx, db.DB(), brand)
			if err != nil {
				return err
			}
			firstRun := priorBatch == ""

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fan out the ad-library fetches concurrently, each bounded by its
			// own sub-request ctx. Diffing stays sequential below so output
			// ordering is deterministic.
			type probe struct {
				data json.RawMessage
				err  error
			}
			probes := make([]probe, len(adNetworks))
			var wg sync.WaitGroup
			for i, n := range adNetworks {
				wg.Add(1)
				go func(i int, n adNetwork) {
					defer wg.Done()
					sctx, scancel := subRequestCtx(ctx)
					defer scancel()
					data, gerr := c.Get(sctx, n.path, map[string]string{n.queryParam: brand})
					probes[i] = probe{data: data, err: gerr}
				}(i, n)
			}
			wg.Wait()

			results := make([]adNetworkResult, 0, len(adNetworks))
			failures := make([]fetchFailure, 0)
			// storedIDs is what we persist as the new baseline: current ids for
			// networks that succeeded, carried-forward prior ids for networks
			// that failed (so a transient error does not churn the diff).
			storedIDs := map[string][]string{}

			for i, n := range adNetworks {
				data, gerr := probes[i].data, probes[i].err
				if gerr != nil {
					failures = append(failures, fetchFailure{Source: n.name, Error: sanitizeFetchErr(gerr)})
					if prev := priorIDs[n.name]; len(prev) > 0 {
						storedIDs[n.name] = prev
					}
					continue
				}
				items := resultArray(data, n.resultKey)
				curr := make([]string, 0, len(items))
				seen := map[string]bool{}
				for _, it := range items {
					id := extractItemID(it, n.idField)
					if id == "" || seen[id] {
						continue
					}
					seen[id] = true
					curr = append(curr, id)
				}
				storedIDs[n.name] = curr

				res := adNetworkResult{Network: n.name, Count: len(curr), New: make([]string, 0), Gone: make([]string, 0)}
				if firstRun {
					res.New = append(res.New, curr...)
				} else {
					prevSet := toSet(priorIDs[n.name])
					currSet := seen
					for _, id := range curr {
						if !prevSet[id] {
							res.New = append(res.New, id)
						}
					}
					for _, id := range priorIDs[n.name] {
						if !currSet[id] {
							res.Gone = append(res.Gone, id)
						}
					}
				}
				sort.Strings(res.New)
				sort.Strings(res.Gone)
				results = append(results, res)
			}

			if err := store.InsertAdSnapshotBatch(ctx, db.DB(), brand, storedIDs, time.Now()); err != nil {
				return err
			}

			warnFetchFailures(cmd, "ads monitor", failures)

			if novelWantsMachine(cmd.OutOrStdout(), flags) {
				envelope := map[string]any{
					"brand":          brand,
					"first_run":      firstRun,
					"networks":       results,
					"fetch_failures": failures,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			w := cmd.OutOrStdout()
			if firstRun {
				fmt.Fprintf(w, "Baseline snapshot for %q (first run; no prior to diff)\n\n", brand)
			} else {
				fmt.Fprintf(w, "Ad diff for %q vs. last run\n\n", brand)
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "NETWORK\tCURRENT\tNEW\tGONE")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\n", r.Network, r.Count, len(r.New), len(r.Gone))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
