// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: harvest a research area into the local store.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/dspace"
)

// harvestFailure records a per-thesis enrichment failure so aggregate counts
// never silently swallow partial failures.
type harvestFailure struct {
	Handle string `json:"handle"`
	Error  string `json:"error"`
}

type harvestResult struct {
	Query         string           `json:"query"`
	Total         int              `json:"total_available"`
	ScannedHits   int              `json:"scanned_hits"`
	Stored        int              `json:"stored"`
	FetchFailures []harvestFailure `json:"fetch_failures"`
	DB            string           `json:"db"`
}

// pp:data-source live
func newNovelHarvestCmd(flags *rootFlags) *cobra.Command {
	var (
		limit  int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "harvest <query>",
		Short: "Mirror a research area's theses into the local store for offline analysis",
		Long: "Search Shodhganga for <query>, fetch the full Dublin Core metadata for each\n" +
			"matching thesis, and store the records locally. This populates the corpus that\n" +
			"guide, similar, trends, university stats, and offline search all read from.",
		Example:     "  shodhganga-pp-cli harvest \"machine learning\" --limit 100",
		Annotations: map[string]string{"pp:happy-args": "query=physics;--limit=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would harvest up to %d theses\n", limit)
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			query := args[0]
			// verify runs in mock mode with no network; report a valid empty harvest.
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(),
					harvestResult{Query: query, FetchFailures: []harvestFailure{}}, flags)
			}
			if limit <= 0 {
				limit = 50
			}
			// Live-dogfood curtails work to fit the flat per-command timeout.
			if cliutil.IsDogfoodEnv() && limit > 3 {
				limit = 3
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := newDSpaceClient(flags)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("shodhganga-pp-cli")
			}

			// Collect up to `limit` search hits, paging as needed.
			const pageSize = 50
			var hits []dspace.SearchHit
			total := 0
			for start := 0; len(hits) < limit; {
				requestSize := min(pageSize, limit-len(hits))
				page, err := c.Search(ctx, query, requestSize, start)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				total = page.Total
				if len(page.Hits) == 0 {
					break
				}
				hits = append(hits, page.Hits...)
				if len(page.Hits) < requestSize {
					break
				}
				start += requestSize
			}
			if len(hits) > limit {
				hits = hits[:limit]
			}

			s, err := openThesisStoreForWrite(cmd, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			result := harvestResult{
				Query:         query,
				Total:         total,
				ScannedHits:   len(hits),
				FetchFailures: []harvestFailure{},
				DB:            dbPath,
			}

			// Enrich concurrently: Shodhganga is slow and variable per request, so
			// a bounded worker pool fetches item pages in parallel while store
			// writes stay in this single goroutine (SQLite has one writer). Each
			// fetch's error travels with its result so a per-item timeout is
			// recorded, never fatal.
			type fetchOutcome struct {
				hit dspace.SearchHit
				th  *dspace.Thesis
				err error
			}
			const workers = 8
			jobs := make(chan dspace.SearchHit)
			outcomes := make(chan fetchOutcome)
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for h := range jobs {
						th, err := c.Item(ctx, h.ID)
						outcomes <- fetchOutcome{hit: h, th: th, err: err}
					}
				}()
			}
			go func() {
				defer close(jobs)
				for _, h := range hits {
					select {
					case jobs <- h:
					case <-ctx.Done():
						return
					}
				}
			}()
			go func() {
				wg.Wait()
				close(outcomes)
			}()

			for o := range outcomes {
				if o.err != nil {
					result.FetchFailures = append(result.FetchFailures, harvestFailure{Handle: o.hit.Handle, Error: o.err.Error()})
					continue
				}
				th := o.th
				// The listing page carries the clean title; prefer it if the item
				// page title is empty.
				if th.Title == "" {
					th.Title = o.hit.Title
				}
				data, err := json.Marshal(th)
				if err != nil {
					result.FetchFailures = append(result.FetchFailures, harvestFailure{Handle: o.hit.Handle, Error: err.Error()})
					continue
				}
				if err := s.Upsert(thesisResourceType, th.ID, data); err != nil {
					result.FetchFailures = append(result.FetchFailures, harvestFailure{Handle: o.hit.Handle, Error: err.Error()})
					continue
				}
				result.Stored++
			}
			// Stable ordering for deterministic output.
			sort.Slice(result.FetchFailures, func(i, j int) bool {
				return result.FetchFailures[i].Handle < result.FetchFailures[j].Handle
			})

			// Record sync state so local search can report corpus freshness.
			if result.Stored > 0 {
				if err := s.SaveSyncState(thesisResourceType, query, result.Stored); err != nil {
					return fmt.Errorf("saving harvest sync state: %w", err)
				}
			}

			if len(result.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d theses failed to fetch; %d stored\n",
					len(result.FetchFailures), len(hits), result.Stored)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Harvested %d theses for %q into %s (%d available upstream).\n",
				result.Stored, query, dbPath, total)
			if len(result.FetchFailures) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%d fetch failures.\n", len(result.FetchFailures))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Now try: shodhganga-pp-cli search <term> --db", dbPath)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum theses to harvest")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path (default: standard cache location)")
	return cmd
}
