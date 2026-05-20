// Run an Actor and emit only items not seen in prior runs of that Actor.
//
// This is the headline novel feature: novelty diffing. Built on the
// pp_dataset_items table (extensions.go), the normalize registry, and the
// cost package's pre-flight projection.
//
// Usage shapes:
//
//	apify-pp-cli run apidojo/twitter-scraper-lite --input @q.json --only-new --format markdown
//	apify-pp-cli run trudax/reddit-scraper --max-cost 0.25 --wait
//	apify-pp-cli run apify/google-news-scraper --memory 4096 --timeout 600 --format json
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/cost"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/normalize"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/store"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/syncstate"
)

// writeSyncState records that a run or workflow hydrated the local store.
// doctor and external tooling read ~/.local/share/apify-pp-cli/sync_state.json
// to see when fresh data last landed. Best-effort — never fails the command.
func writeSyncState(actor string, ok bool, itemsHydrated int) {
	st := &syncstate.State{
		LastAttemptedAt: time.Now().UTC(),
		OK:              ok,
		ItemsHydrated:   itemsHydrated,
		TokenSource:     "env:APIFY_TOKEN",
	}
	if ok {
		st.LastSyncedAt = st.LastAttemptedAt
	}
	_ = syncstate.Save("", st)
}

// newRunCmd returns the top-level `run` command.
func newRunCmd(flags *rootFlags) *cobra.Command {
	var (
		input          string
		wait           bool
		timeoutSecs    int
		memoryMB       int
		onlyNew        bool
		format         string
		maxCost        float64
		noProjection   bool
		preset         string
		buildTag       string
		webhookOverride string
	)

	cmd := &cobra.Command{
		Use:   "run <actor>",
		Short: "Run an Actor and emit only items not seen in prior runs of that Actor",
		Long: strings.Trim(`
Run an Actor on the Apify platform. With --only-new, hashes incoming items
against the local store and emits only items not seen in prior runs.

Cost projection from local history (p50/p90) prints before every run unless
--no-projection or --agent is set. With --max-cost, refuses to start if the
projection exceeds the budget.

Examples:
  apify-pp-cli run apidojo/twitter-scraper-lite --input @q.json --only-new --format markdown
  apify-pp-cli run trudax/reddit-scraper --max-cost 0.25 --wait
  apify-pp-cli run apify/google-news-scraper --memory 4096 --timeout 600 --format json
`, "\n"),
		Example: strings.Trim(`
  apify-pp-cli run apidojo/twitter-scraper-lite --input @q.json --only-new
  apify-pp-cli run trudax/reddit-scraper --max-cost 0.25 --wait --json
`, "\n"),
		Annotations: map[string]string{
			// Mutates external state (charges the user's Apify account),
			// so NOT read-only. No mcp:read-only annotation.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			actor := args[0]

			// --only-new can only diff items once the run has produced a
			// dataset, which requires waiting for terminal status. Without
			// --wait the item-fetch branch is skipped and the user sees an
			// empty list indistinguishable from "all items already seen".
			// Fail loudly instead of silently no-opping.
			if onlyNew && !wait {
				return usageErr(fmt.Errorf(
					"--only-new requires --wait: novelty diffing needs the run to finish so its dataset items can be compared"))
			}

			// Verify probes call with --dry-run; short-circuit silently
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(),
					"would run actor %q with input=%q (dry-run)\n", actor, input)
				return nil
			}

			ctx := cmd.Context()

			// 1) Open store and ensure extensions
			db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
			if err != nil {
				return configErr(fmt.Errorf("opening local store: %w", err))
			}
			defer db.Close()
			if err := db.EnsureExtensions(ctx); err != nil {
				return configErr(fmt.Errorf("ensuring extension tables: %w", err))
			}

			// 2) Cost projection + budget enforcement.
			// The projection LINE is suppressed under --agent/--compact/--no-projection,
			// but --max-cost ENFORCEMENT always runs when the cap is set — an
			// agent invoking `run --agent --max-cost` must still be capped.
			showProjection := !noProjection && !flags.agent && !flags.compact
			if showProjection || maxCost > 0 {
				history, err := db.LoadActorRunHistory(ctx, actor, 50)
				if err == nil {
					stats := historyToStats(history)
					proj := cost.Project(actor, stats)
					if showProjection {
						fmt.Fprintln(cmd.ErrOrStderr(), cost.FormatProjection(proj, maxCost))
					}
					if maxCost > 0 {
						if cost.ExceedsBudget(proj, maxCost) {
							return apiErr(fmt.Errorf(
								"projected cost $%.2f exceeds --max-cost $%.2f; raise the cap or omit --max-cost to run uncapped",
								proj.P50USD, maxCost))
						}
						if !cost.CanEnforce(proj, maxCost) {
							fmt.Fprintf(cmd.ErrOrStderr(),
								"WARNING: --max-cost $%.2f cannot be enforced — no prior runs of %q are cached; the run will proceed uncapped\n",
								maxCost, actor)
						}
					}
				}
			}

			// 3) Resolve preset if requested
			if preset != "" {
				presetData, err := db.LoadPreset(ctx, preset, actor)
				if err != nil {
					return configErr(fmt.Errorf("loading preset %q: %w", preset, err))
				}
				if len(presetData) == 0 {
					return notFoundErr(fmt.Errorf(
						"preset %q not found for actor %q; save one with `apify-pp-cli preset save`",
						preset, actor))
				}
				if input == "" {
					input = string(presetData)
				}
			}

			// 4) Build request body (input JSON)
			inputJSON, err := resolveInput(input)
			if err != nil {
				return usageErr(fmt.Errorf("parsing --input: %w", err))
			}

			// 5) Construct query params
			params := map[string]string{}
			if timeoutSecs > 0 {
				params["timeout"] = fmt.Sprintf("%d", timeoutSecs)
			}
			if memoryMB > 0 {
				params["memory"] = fmt.Sprintf("%d", memoryMB)
			}
			if buildTag != "" {
				params["build"] = buildTag
			}
			if webhookOverride != "" {
				params["webhooks"] = webhookOverride
			}
			if wait {
				// Apify's `waitForFinish` blocks server-side up to N seconds
				waitSecs := 60
				if timeoutSecs > 0 && timeoutSecs < 60 {
					waitSecs = timeoutSecs
				}
				params["waitForFinish"] = fmt.Sprintf("%d", waitSecs)
			}

			// 6) POST /v2/acts/{actorId}/runs
			c, err := flags.newClient()
			if err != nil {
				return configErr(err)
			}
			path := fmt.Sprintf("/v2/acts/%s/runs", actorPathSegment(actor))
			body, status, err := c.PostWithParams(path, params, json.RawMessage(inputJSON))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status >= 400 {
				return apiErr(fmt.Errorf("actor run start failed: HTTP %d", status))
			}
			runResp := struct {
				Data RunData `json:"data"`
			}{}
			if err := json.Unmarshal(body, &runResp); err != nil {
				return apiErr(fmt.Errorf("parsing run start response: %w", err))
			}
			run := runResp.Data

			// 7) If wait, poll until terminal (with cap)
			if wait && !isTerminalStatus(run.Status) {
				deadline := time.Now().Add(15 * time.Minute)
				if timeoutSecs > 0 {
					deadline = time.Now().Add(time.Duration(timeoutSecs) * time.Second)
				}
				run, err = pollRunUntilTerminal(ctx, c, run.ID, deadline)
				if err != nil {
					return err
				}
			}

			// 8) Persist run history regardless of wait outcome
			_ = db.RecordActorRun(ctx, run.ID, run.ActID, actor, run.Status,
				run.Stats.ComputeUnits, run.Options.MemoryMbytes,
				secondsBetween(run.StartedAt, run.FinishedAt),
				run.DefaultDatasetID, run.StartedAt, run.FinishedAt, inputJSON)

			// 9) If terminal + SUCCEEDED + dataset exists, fetch items
			var items []*normalize.Item
			if isTerminalStatus(run.Status) && run.Status == "SUCCEEDED" && run.DefaultDatasetID != "" {
				rawItems, err := fetchDatasetItems(c, run.DefaultDatasetID)
				if err != nil {
					return apiErr(fmt.Errorf("fetching dataset items: %w", err))
				}
				reg, _ := normalize.NewRegistry()
				items = reg.NormalizeBatch(actor, rawItems)

				// 10) --only-new dedupe
				if onlyNew && len(items) > 0 {
					hashes := make([]string, len(items))
					for i, it := range items {
						hashes[i] = it.Hash
					}
					seen, _ := db.HashesSeen(ctx, hashes)
					novel := items[:0]
					for _, it := range items {
						if !seen[it.Hash] {
							novel = append(novel, it)
						}
					}
					items = novel
				}

				// 11) Persist items (always, even on --only-new — we still want
				// the items in the store for future dedupe)
				for _, it := range items {
					_, _ = db.UpsertNormalizedItem(ctx,
						it.Hash, it.SourceActor, run.ID, run.DefaultDatasetID,
						it.URL, it.Title, it.Body, it.Author,
						timeStrOrEmptyHelper(it.PublishedAt), it.EngagementScore,
						it.FetchedAt, it.Raw)
				}
			}

			// 11b) Record sync state — a successful run hydrates the local
			// store, so doctor and external tooling can read sync_state.json
			// to see when data last landed.
			writeSyncState(actor, run.Status == "SUCCEEDED", len(items))

			// 12) Render output
			return renderRunOutput(cmd, flags, run, items, format)
		},
	}

	cmd.Flags().StringVar(&input, "input", "", "Actor input JSON (literal or @file)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Block until the run reaches terminal status")
	cmd.Flags().IntVar(&timeoutSecs, "timeout-secs", 0, "Override Actor's default timeout (seconds)")
	cmd.Flags().IntVar(&memoryMB, "memory", 0, "Override memory allocation (MB)")
	cmd.Flags().BoolVar(&onlyNew, "only-new", false, "Emit only items not seen in prior runs of this Actor (requires --wait + SUCCEEDED)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json | markdown | raw")
	cmd.Flags().Float64Var(&maxCost, "max-cost", 0, "Refuse to start run if p50 cost projection exceeds this USD amount")
	cmd.Flags().BoolVar(&noProjection, "no-projection", false, "Skip cost projection output")
	cmd.Flags().StringVar(&preset, "preset", "", "Load saved input from a preset (overridden by --input)")
	cmd.Flags().StringVar(&buildTag, "build", "", "Actor build tag (e.g. latest, beta, 0.1.5)")
	cmd.Flags().StringVar(&webhookOverride, "webhooks", "", "Webhook subscriptions override (URL-encoded JSON)")

	return cmd
}

// --- run plumbing ---

type RunData struct {
	ID               string    `json:"id"`
	ActID            string    `json:"actId"`
	Status           string    `json:"status"`
	StartedAt        time.Time `json:"startedAt"`
	FinishedAt       time.Time `json:"finishedAt"`
	DefaultDatasetID string    `json:"defaultDatasetId"`
	DefaultKVStoreID string    `json:"defaultKeyValueStoreId"`
	ExitCode         int       `json:"exitCode,omitempty"`
	Stats            struct {
		ComputeUnits float64 `json:"computeUnits"`
	} `json:"stats"`
	Options struct {
		MemoryMbytes int `json:"memoryMbytes"`
	} `json:"options"`
}

func isTerminalStatus(s string) bool {
	switch s {
	case "SUCCEEDED", "FAILED", "ABORTED", "TIMED-OUT":
		return true
	}
	return false
}

func pollRunUntilTerminal(ctx context.Context, c interface {
	GetNoCache(string, map[string]string) (json.RawMessage, error)
}, runID string, deadline time.Time) (RunData, error) {
	interval := 5 * time.Second
	for {
		if time.Now().After(deadline) {
			return RunData{}, apiErr(errors.New("polling deadline exceeded; run still in-flight"))
		}
		// GetNoCache, not Get: the response cache would pin the first
		// non-terminal status for its full TTL, so a run that finishes
		// inside that window would appear to never reach a terminal state.
		body, err := c.GetNoCache(fmt.Sprintf("/v2/actor-runs/%s", escapeSeg(runID)), nil)
		if err != nil {
			return RunData{}, err
		}
		var resp struct {
			Data RunData `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return RunData{}, err
		}
		if isTerminalStatus(resp.Data.Status) {
			return resp.Data, nil
		}
		select {
		case <-ctx.Done():
			return RunData{}, ctx.Err()
		case <-time.After(interval):
		}
		if interval < 30*time.Second {
			interval += 5 * time.Second
		}
	}
}

// datasetItemsPageSize is the per-request page size; datasetItemsMaxPages
// caps total pages so a pathological dataset can't loop forever or pull
// an unbounded payload. 1000 * 50 = 50k items, well beyond any single
// newsletter run.
const (
	datasetItemsPageSize = 1000
	datasetItemsMaxPages = 50
)

// fetchDatasetItems pages through a dataset's items via offset pagination.
// The Apify dataset items endpoint truncates at the `limit` query param;
// a single request silently drops everything past the first page. The loop
// advances `offset` until a short page is returned or the page cap is hit.
func fetchDatasetItems(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, datasetID string) ([]map[string]any, error) {
	escapedID := url.PathEscape(datasetID)
	var all []map[string]any
	for page := 0; page < datasetItemsMaxPages; page++ {
		offset := page * datasetItemsPageSize
		body, err := c.Get(fmt.Sprintf("/v2/datasets/%s/items", escapedID),
			map[string]string{
				"clean":  "true",
				"format": "json",
				"limit":  fmt.Sprintf("%d", datasetItemsPageSize),
				"offset": fmt.Sprintf("%d", offset),
			})
		if err != nil {
			return all, err
		}
		pageItems, err := parseDatasetItemsBody(body)
		if err != nil {
			return all, err
		}
		all = append(all, pageItems...)
		// Short page (or empty) means we've reached the end.
		if len(pageItems) < datasetItemsPageSize {
			break
		}
	}
	return all, nil
}

// parseDatasetItemsBody handles both the bare-array and {data:{items:[]}}
// envelope shapes the Apify dataset endpoint can return.
func parseDatasetItemsBody(body json.RawMessage) ([]map[string]any, error) {
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err == nil {
		return items, nil
	}
	var wrapped struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil {
		return wrapped.Data.Items, nil
	}
	return nil, fmt.Errorf("dataset items response was neither an array nor a {data:{items}} envelope")
}

// actorPathSegment converts an Actor reference ("username/name" or
// "username~name") into the path-safe "username~name" segment Apify expects.
// url.PathEscape guards against any character that would break path routing,
// even though current Apify slugs are already URL-safe.
func actorPathSegment(actor string) string {
	return url.PathEscape(strings.ReplaceAll(actor, "/", "~"))
}

// escapeSeg path-escapes a single URL path segment (run IDs, dataset IDs,
// store IDs). Apify IDs are alphanumeric today, but escaping keeps a
// malformed or user-supplied ID from breaking path routing.
func escapeSeg(s string) string {
	return url.PathEscape(s)
}

func resolveInput(in string) ([]byte, error) {
	if in == "" {
		return []byte("{}"), nil
	}
	if strings.HasPrefix(in, "@") {
		path := in[1:]
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	// Verify it parses as JSON
	var probe any
	if err := json.Unmarshal([]byte(in), &probe); err != nil {
		return nil, fmt.Errorf("input is not valid JSON: %w", err)
	}
	return []byte(in), nil
}

func historyToStats(rows []store.ActorRunRecord) []cost.RunStats {
	out := make([]cost.RunStats, len(rows))
	for i, r := range rows {
		out[i] = cost.RunStats{
			RunID: r.RunID,
			// cost.Project and cost.Rollup match/group on the caller-supplied
			// Actor slug, which is stored in actor_name — not the opaque
			// Apify-internal actor_id.
			ActorID:         r.ActorName,
			ComputeUnits:    r.ComputeUnits,
			MemoryAvgMBytes: r.MemoryMbytes,
			DurationSecs:    r.DurationSecs,
		}
	}
	return out
}

func secondsBetween(start, end time.Time) float64 {
	if start.IsZero() || end.IsZero() {
		return 0
	}
	return end.Sub(start).Seconds()
}

func timeStrOrEmptyHelper(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func renderRunOutput(cmd *cobra.Command, flags *rootFlags,
	run RunData, items []*normalize.Item, format string) error {
	asMarkdown := strings.ToLower(format) == "markdown"
	asRaw := strings.ToLower(format) == "raw"
	if asMarkdown && !flags.asJSON {
		return renderRunMarkdown(cmd, run, items)
	}
	if asRaw {
		// Emit raw items array (as fetched, unnormalized)
		raws := make([]json.RawMessage, len(items))
		for i, it := range items {
			raws[i] = it.Raw
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"run":   run,
			"items": raws,
		}, flags)
	}
	// Default: JSON envelope
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"run":   run,
		"items": items,
	}, flags)
}

func renderRunMarkdown(cmd *cobra.Command, run RunData, items []*normalize.Item) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "# Run %s (%s)\n\n", run.ID, run.Status)
	fmt.Fprintf(out, "- Actor: %s\n- Started: %s\n- Finished: %s\n- Compute units: %.4f\n\n",
		run.ActID, timeStrOrEmptyHelper(run.StartedAt),
		timeStrOrEmptyHelper(run.FinishedAt), run.Stats.ComputeUnits)
	fmt.Fprintf(out, "## Items (%d)\n\n", len(items))
	for _, it := range items {
		if it.Title != "" {
			fmt.Fprintf(out, "### %s\n", it.Title)
		}
		if it.URL != "" {
			fmt.Fprintf(out, "%s\n\n", it.URL)
		}
		if it.Author != "" {
			fmt.Fprintf(out, "_by %s", it.Author)
			if !it.PublishedAt.IsZero() {
				fmt.Fprintf(out, " — %s", it.PublishedAt.Format("2006-01-02"))
			}
			fmt.Fprintln(out, "_")
		}
		if it.Body != "" {
			body := it.Body
			if len(body) > 500 {
				body = body[:500] + "..."
			}
			fmt.Fprintf(out, "\n%s\n\n", body)
		}
		fmt.Fprintln(out, "---")
	}
	return nil
}
