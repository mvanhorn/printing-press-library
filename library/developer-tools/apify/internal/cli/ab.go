// `apify-pp-cli ab run <actorA> <actorB>` — run the same input through two
// competing Actors, normalize via unified schema, report cost-per-novel-item
// and overlap percentage.
//
// The "judge" picks the winner:
//   novelty       — fewer duplicates against the local store wins
//   cost-per-item — total USD / total items wins
//   cost-per-novel — total USD / novel items wins
//   item-count    — most items wins (raw volume)
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/cost"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/normalize"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/store"
)

func newABCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ab",
		Short: "Compare two Actors on the same input and pick a winner",
		Long: strings.Trim(`
Subcommands:
  run   Run two Actors with the same input, normalize, compare, pick winner

Use this to decide between two competing scrapers (e.g. apidojo vs kaitoeasyapi
for Twitter) before committing to a recurring schedule.
`, "\n"),
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newABRunCmd(flags))
	return cmd
}

func newABRunCmd(flags *rootFlags) *cobra.Command {
	var (
		input string
		judge string
		wait  bool
	)
	cmd := &cobra.Command{
		Use:   "run <actorA> <actorB>",
		Short: "Run two Actors with the same input, compare, declare a winner",
		Long: strings.Trim(`
Starts both Actors with the same input JSON, waits for both, normalizes
their datasets via the shared schema, then reports:
  - items returned (total + after dedupe)
  - overlap percentage (items both Actors found)
  - estimated USD cost (from cost.Estimate over the run stats)
  - winner per the --judge metric

Examples:
  apify-pp-cli ab run apidojo/tweet-scraper kaitoeasyapi/twitter-x-data-tweet-scraper-pay-per-result-cheapest \
      --input @q.json --judge cost-per-novel --json
  apify-pp-cli ab run apify/google-news-scraper apidojo/google-news-aggregator-scraper \
      --input '{"q":"AI"}' --judge novelty
`, "\n"),
		Example: strings.Trim(`
  apify-pp-cli ab run apidojo/tweet-scraper kaitoeasyapi/twitter-x-data-tweet-scraper --input @q.json --judge cost-per-novel --json
`, "\n"),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			actorA, actorB := args[0], args[1]
			inputBytes, err := resolveInput(input)
			if err != nil {
				return usageErr(fmt.Errorf("parsing --input: %w", err))
			}

			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			if err := db.EnsureExtensions(ctx); err != nil {
				return configErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return configErr(err)
			}
			reg, _ := normalize.NewRegistry()

			// Run both Actors concurrently so the comparison is fair: a
			// sequential second run would face different live conditions and
			// double the wall-clock time. The store serializes writes via its
			// internal mutex and the normalize registry is read-only after
			// construction, so concurrent abRunOne calls are safe.
			var resA, resB abResult
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				resA = abRunOne(ctx, c, db, reg, actorA, inputBytes, wait)
			}()
			go func() {
				defer wg.Done()
				resB = abRunOne(ctx, c, db, reg, actorB, inputBytes, wait)
			}()
			wg.Wait()

			// Compute overlap by URL set
			urlSetA := map[string]bool{}
			for _, h := range resA.URLs {
				urlSetA[h] = true
			}
			overlap := 0
			for _, u := range resB.URLs {
				if urlSetA[u] {
					overlap++
				}
			}
			overlapPct := 0.0
			denom := minInt(len(resA.URLs), len(resB.URLs))
			if denom > 0 {
				overlapPct = float64(overlap) / float64(denom) * 100
			}

			winner := pickWinner(resA, resB, judge)

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"actor_a":      actorA,
				"actor_b":      actorB,
				"input_bytes":  len(inputBytes),
				"results": map[string]any{
					actorA: resA,
					actorB: resB,
				},
				"overlap_count": overlap,
				"overlap_pct":   overlapPct,
				"judge":         judge,
				"winner":        winner,
				"compared_at":   time.Now().UTC().Format(time.RFC3339),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "Shared input JSON (literal or @file) for both Actors")
	cmd.Flags().StringVar(&judge, "judge", "cost-per-novel",
		"Winner rule: novelty | cost-per-item | cost-per-novel | item-count")
	cmd.Flags().BoolVar(&wait, "wait", true, "Wait for both runs to complete (default true)")
	return cmd
}

// abResult captures one Actor's outcome for the comparison envelope.
type abResult struct {
	Actor      string   `json:"actor"`
	RunID      string   `json:"run_id,omitempty"`
	Status     string   `json:"status"`
	ItemsTotal int      `json:"items_total"`
	ItemsNovel int      `json:"items_novel"`
	CostUSD    float64  `json:"cost_usd"`
	CU         float64  `json:"compute_units"`
	URLs       []string `json:"-"`
	Error      string   `json:"error,omitempty"`
}

func abRunOne(ctx context.Context, c interface {
	PostWithParams(string, map[string]string, any) (json.RawMessage, int, error)
	Get(string, map[string]string) (json.RawMessage, error)
	GetNoCache(string, map[string]string) (json.RawMessage, error)
}, db *store.Store, reg *normalize.Registry,
	actor string, inputBytes []byte, wait bool) abResult {
	res := abResult{Actor: actor, Status: "unknown"}

	params := map[string]string{}
	if wait {
		params["waitForFinish"] = "60"
	}

	body, status, err := c.PostWithParams(
		fmt.Sprintf("/v2/acts/%s/runs", actorPathSegment(actor)), params, json.RawMessage(inputBytes))
	if err != nil {
		res.Error = err.Error()
		res.Status = "start_failed"
		return res
	}
	if status >= 400 {
		res.Error = fmt.Sprintf("HTTP %d", status)
		res.Status = "start_failed"
		return res
	}
	var resp struct {
		Data RunData `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		res.Error = err.Error()
		return res
	}
	run := resp.Data
	res.RunID = run.ID

	if wait && !isTerminalStatus(run.Status) {
		deadline := time.Now().Add(15 * time.Minute)
		polled, perr := pollRunUntilTerminal(ctx, c, run.ID, deadline)
		if perr != nil {
			res.Error = perr.Error()
			res.Status = run.Status
			return res
		}
		run = polled
	}
	res.Status = run.Status
	res.CU = run.Stats.ComputeUnits
	res.CostUSD = (&cost.RunStats{
		ComputeUnits:    run.Stats.ComputeUnits,
		MemoryAvgMBytes: run.Options.MemoryMbytes,
		DurationSecs:    secondsBetween(run.StartedAt, run.FinishedAt),
	}).Estimate()

	// Persist run history
	_ = db.RecordActorRun(ctx, run.ID, run.ActID, actor, run.Status,
		run.Stats.ComputeUnits, run.Options.MemoryMbytes,
		secondsBetween(run.StartedAt, run.FinishedAt),
		run.DefaultDatasetID, run.StartedAt, run.FinishedAt, inputBytes)

	if run.Status != "SUCCEEDED" || run.DefaultDatasetID == "" {
		return res
	}

	raws, err := fetchDatasetItems(c, run.DefaultDatasetID)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	items := reg.NormalizeBatch(actor, raws)
	res.ItemsTotal = len(items)
	hashes := make([]string, len(items))
	res.URLs = make([]string, 0, len(items))
	for i, it := range items {
		hashes[i] = it.Hash
		if it.URL != "" {
			res.URLs = append(res.URLs, it.URL)
		}
	}
	seen, _ := db.HashesSeen(ctx, hashes)
	for _, it := range items {
		if !seen[it.Hash] {
			res.ItemsNovel++
		}
		_, _ = db.UpsertNormalizedItem(ctx,
			it.Hash, it.SourceActor, run.ID, run.DefaultDatasetID,
			it.URL, it.Title, it.Body, it.Author,
			timeStrOrEmptyHelper(it.PublishedAt), it.EngagementScore,
			it.FetchedAt, it.Raw)
	}
	return res
}

func pickWinner(a, b abResult, judge string) string {
	switch strings.ToLower(judge) {
	case "novelty":
		if a.ItemsNovel >= b.ItemsNovel {
			return a.Actor
		}
		return b.Actor
	case "item-count":
		if a.ItemsTotal >= b.ItemsTotal {
			return a.Actor
		}
		return b.Actor
	case "cost-per-item":
		ra := safeRate(a.CostUSD, float64(a.ItemsTotal))
		rb := safeRate(b.CostUSD, float64(b.ItemsTotal))
		if ra <= rb {
			return a.Actor
		}
		return b.Actor
	case "cost-per-novel", "":
		ra := safeRate(a.CostUSD, float64(a.ItemsNovel))
		rb := safeRate(b.CostUSD, float64(b.ItemsNovel))
		if ra <= rb {
			return a.Actor
		}
		return b.Actor
	}
	return ""
}

func safeRate(num, denom float64) float64 {
	if denom <= 0 {
		return 1e9 // effectively infinite; loser
	}
	return num / denom
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
