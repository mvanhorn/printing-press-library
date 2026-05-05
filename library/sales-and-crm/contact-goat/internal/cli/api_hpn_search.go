// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// api_hpn_search.go: `api hpn search` and its sub-subcommands
// (find-more, get). Each spends Happenstance credits, so the cost-preview
// and budget gates from deepline.go's pattern apply.
//
// Group-id handling: the OpenAPI spec accepts group_ids in the request
// body directly (POST /v1/search). We pass them through that field rather
// than building @-mention prefixes via api.FormatGroupMention, because
// (a) the body field is unambiguous (no quoting / escaping concerns) and
// (b) the user did not pass natural-language @mention text — they passed
// --group-id flags. FormatGroupMention is still exported from the api
// package and available for callers building the search text by hand.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

// CreditCostPerFindMore is the documented cost (in Happenstance credits)
// of one /v1/search/{id}/find-more call. Surfaced in the cost preview
// before the call goes out.
const CreditCostPerFindMore = 2

// hpnSearchEnvelope is the JSON envelope `api hpn search` (and the
// sub-subcommands) emit on stdout. Mirrors what `coverage --source api
// --json` would produce so jq pipelines built against either entry
// point work the same way (the work in unit 5 proved shape compatibility
// for the search-class results array).
type hpnSearchEnvelope struct {
	SearchID  string            `json:"search_id"`
	URL       string            `json:"url,omitempty"`
	Query     string            `json:"query"`
	Status    string            `json:"status"`
	Source    string            `json:"source"`
	Completed bool              `json:"completed"`
	Count     int               `json:"count"`
	Results   []hpnSearchResult `json:"results"`
	HasMore   bool              `json:"has_more,omitempty"`
	NextPage  string            `json:"next_page,omitempty"`
}

// hpnSearchResult is one row of hpnSearchEnvelope.Results. We render
// straight from the canonical client.Person produced by the unit-3
// normalizer (api.ToClientPerson), so jq pipelines that target
// .results[].name keep working whether the result came from a /v1/search
// row or from a normalizer shim.
type hpnSearchResult struct {
	Name           string      `json:"name"`
	CurrentTitle   string      `json:"current_title,omitempty"`
	CurrentCompany string      `json:"current_company,omitempty"`
	LinkedInURL    string      `json:"linkedin_url,omitempty"`
	Score          float64     `json:"score,omitempty"`
	Bridges        []bridgeRef `json:"bridges,omitempty"`
	Rationale      string      `json:"rationale,omitempty"`
}

// newAPIHpnSearchCmd builds `api hpn search`. The parent command takes a
// positional <text> arg and runs an end-to-end POST + poll + render flow.
// It also registers two sub-subcommands: find-more (paginate) and get
// (re-fetch by id).
func newAPIHpnSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		includeFriendsConnections bool
		includeMyConnections      bool
		firstDegreeOnly           bool
		minScore                  float64
		groupIDs                  []string
		budget                    int
		pollTimeoutSec            int
		pollIntervalSec           int
		allPages                  bool
		maxResults                int
	)

	cmd := &cobra.Command{
		Use:   "search <text>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Run a Happenstance search via the public API (costs 2 credits)",
		Long: `Run a search against the Happenstance public API.

Costs 2 credits per call. The cost preview prints before the call goes
out unless --yes is set or --budget 0 (the default) opts out of the
prompt. Pass --budget N to refuse to spend when a single call would
exceed N credits.

The flow is asynchronous: the CLI calls POST /v1/search, then polls
GET /v1/search/{id} until the status is COMPLETED, FAILED, or
FAILED_AMBIGUOUS — or the --poll-timeout fires.

For paginating an existing search, see ` + "`api hpn search find-more <id>`" + `
and ` + "`api hpn search get <id> [--page-id ID]`" + `. Use --all with
--max-results or --budget to auto-paginate in one command.

Filtering: --first-degree-only keeps only rows where you have a
self_graph bridge (1st-degree). --min-score N drops rows whose score
falls below N (use 5 to drop weak-signal noise). See docs/scoring.md
for the rationale and observed ranges.`,
		Example: `  contact-goat-pp-cli api hpn search "VPs at NBA" --yes
  contact-goat-pp-cli api hpn search "founders in Stripe's network" --include-friends-connections --yes
  contact-goat-pp-cli api hpn search "people in alumni" --group-id grp_abc123 --yes
  contact-goat-pp-cli api hpn search "..." --dry-run`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.TrimSpace(strings.Join(args, " "))
			if text == "" {
				return usageErr(fmt.Errorf("search text is empty — pass a non-empty natural-language query"))
			}
			if minScore < 0 {
				return usageErr(fmt.Errorf("--min-score must be >= 0 (got %g)", minScore))
			}
			if maxResults < 0 {
				return usageErr(fmt.Errorf("--max-results must be >= 0 (got %d)", maxResults))
			}
			if maxResults > 0 && !allPages {
				return usageErr(fmt.Errorf("--max-results requires --all (it only caps the auto-pagination loop)"))
			}
			if allPages && maxResults == 0 && budget == 0 {
				return usageErr(fmt.Errorf("--all requires at least one of --max-results N or --budget N to bound the credit spend"))
			}

			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}

			if !flags.dryRun {
				if blocked, msg := checkSearchBudget(budget, CreditCostPerSearch); blocked {
					if flags.asJSON {
						_ = flags.printJSON(cmd, map[string]any{
							"status":      "skipped",
							"reason":      msg,
							"would_spend": CreditCostPerSearch,
							"budget":      budget,
						})
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), msg)
					}
					return nil
				}
				if !flags.yes && !flags.noInput {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"Will spend %d credits. Re-run with --yes to proceed, --budget 0 to disable the prompt, or --dry-run to preview.\n",
						CreditCostPerSearch,
					)
					return usageErr(fmt.Errorf("confirmation required: pass --yes to proceed"))
				}
			}

			// --first-degree-only auto-implies --include-my-connections at
			// the API layer (otherwise the response will never contain
			// 1st-degree rows for the post-fetch filter to keep).
			effectiveIncludeMyConnections := includeMyConnections || firstDegreeOnly

			opts := &api.SearchOptions{
				GroupIDs:                  groupIDs,
				IncludeFriendsConnections: includeFriendsConnections,
				IncludeMyConnections:      effectiveIncludeMyConnections,
			}
			pollOpts := buildPollSearchOptions(pollTimeoutSec, pollIntervalSec, "")

			firstEnv, err := runHpnSearch(cmd.Context(), c, text, opts, pollOpts)
			if err != nil {
				return classifyHpnError(err)
			}

			// Best-effort fetch of the current user's UUID via the cookie
			// surface so ToClientPersonWithBridges can retag self-bridges
			// to BridgeKindSelfGraph. Without this the bearer surface
			// cannot distinguish 1st-degree (you know them via your own
			// synced contacts) from 2nd-degree (via a friend). When cookie
			// auth is unavailable we fall back to "" — bridges all stay
			// BridgeKindFriend, which matches today's behavior.
			currentUUID := lookupCurrentUserUUID(flags)

			if !allPages {
				return emitHpnSearchEnvelope(cmd, flags, firstEnv, text, currentUUID, firstDegreeOnly, minScore)
			}
			return runHpnSearchAll(cmd, flags, c, firstEnv, text, currentUUID, firstDegreeOnly, minScore, maxResults, budget, pollOpts)
		},
	}

	cmd.Flags().BoolVar(&includeFriendsConnections, "include-friends-connections", false, "Widen search to your Happenstance friends' connections (2nd-degree)")
	cmd.Flags().BoolVar(&includeMyConnections, "include-my-connections", false, "Include your own LinkedIn-synced connections (1st-degree)")
	cmd.Flags().BoolVar(&firstDegreeOnly, "first-degree-only", false, "Keep only results where you have a 1st-degree (self_graph) bridge. Implies --include-my-connections.")
	cmd.Flags().Float64Var(&minScore, "min-score", 0, "Drop results with score below this threshold. 0 (default) disables the filter; >= 5 typically drops weak-signal public-graph hits. See docs/scoring.md.")
	cmd.Flags().BoolVar(&allPages, "all", false, "Auto-paginate via find-more until has_more=false, --max-results N, or --budget N. Each find-more spends 2 credits.")
	cmd.Flags().IntVar(&maxResults, "max-results", 0, "Cap on raw results when --all is set. 0 (default) means unbounded; --all then requires --budget for safety.")
	cmd.Flags().StringSliceVar(&groupIDs, "group-id", nil, "Group id to scope the search to (repeatable). Discover via 'api hpn groups list'")
	cmd.Flags().IntVar(&budget, "budget", 0, "Max credits to spend per call. 0 disables the budget gate (default).")
	cmd.Flags().IntVar(&pollTimeoutSec, "poll-timeout", int(api.DefaultPollTimeout.Seconds()), "Max seconds to wait for the async search to converge")
	cmd.Flags().IntVar(&pollIntervalSec, "poll-interval", int(api.DefaultPollInterval.Seconds()), "Seconds between poll attempts")

	cmd.AddCommand(newAPIHpnSearchFindMoreCmd(flags))
	cmd.AddCommand(newAPIHpnSearchGetCmd(flags))

	return cmd
}

// newAPIHpnSearchFindMoreCmd builds `api hpn search find-more <id>`. Calls
// POST /v1/search/{id}/find-more and renders the new page id (which the
// caller can then re-fetch via `api hpn search get <id> --page-id ID`).
func newAPIHpnSearchFindMoreCmd(flags *rootFlags) *cobra.Command {
	var budget int
	cmd := &cobra.Command{
		Use:   "find-more <search_id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fetch the next page of an existing search (costs 2 credits)",
		Long: `Calls POST /v1/search/{id}/find-more on a parent search. Returns the
new page id; use it on a follow-up ` + "`api hpn search get <id> --page-id <page_id>`" + `
to fetch the additional results.

Costs 2 credits per call. Same cost-preview / --budget / --yes contract
as ` + "`api hpn search`" + `.`,
		Example: `  contact-goat-pp-cli api hpn search find-more srch_abc123 --yes`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			searchID := strings.TrimSpace(args[0])
			if searchID == "" {
				return usageErr(fmt.Errorf("search_id is empty"))
			}
			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}
			if !flags.dryRun {
				if blocked, msg := checkSearchBudget(budget, CreditCostPerFindMore); blocked {
					if flags.asJSON {
						_ = flags.printJSON(cmd, map[string]any{
							"status":      "skipped",
							"reason":      msg,
							"would_spend": CreditCostPerFindMore,
							"budget":      budget,
						})
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), msg)
					}
					return nil
				}
				if !flags.yes && !flags.noInput {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"Will spend %d credits to fetch the next page. Re-run with --yes to proceed.\n",
						CreditCostPerFindMore,
					)
					return usageErr(fmt.Errorf("confirmation required: pass --yes to proceed"))
				}
			}
			env, err := c.FindMore(cmd.Context(), searchID)
			if err != nil {
				return classifyHpnError(err)
			}
			out := map[string]any{
				"page_id":          env.PageId,
				"parent_search_id": env.ParentSearchId,
				"source":           "api",
				"hint":             fmt.Sprintf("contact-goat-pp-cli api hpn search get %s --page-id %s", searchID, env.PageId),
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&budget, "budget", 0, "Max credits to spend per call. 0 disables the budget gate (default).")
	return cmd
}

// newAPIHpnSearchGetCmd builds `api hpn search get <id> [--page-id ID]`.
// Free probe; renders the search envelope identically to the parent
// command's output for shape parity.
func newAPIHpnSearchGetCmd(flags *rootFlags) *cobra.Command {
	var pageID string
	cmd := &cobra.Command{
		Use:   "get <search_id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Re-fetch an existing search by id (free)",
		Long: `Calls GET /v1/search/{id} and renders the envelope in the same shape
as ` + "`api hpn search`" + `. Free probe — no credits spent. Pass --page-id when
re-fetching after a find-more call to surface the additional results.`,
		Example: `  contact-goat-pp-cli api hpn search get srch_abc123
  contact-goat-pp-cli api hpn search get srch_abc123 --page-id page_xyz`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			searchID := strings.TrimSpace(args[0])
			if searchID == "" {
				return usageErr(fmt.Errorf("search_id is empty"))
			}
			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}
			env, err := c.GetSearch(cmd.Context(), searchID, pageID)
			if err != nil {
				return classifyHpnError(err)
			}
			// Re-fetch path: no filter flags, but still plumb currentUUID
			// so the retag works correctly for the rendered output.
			currentUUID := lookupCurrentUserUUID(flags)
			return emitHpnSearchEnvelope(cmd, flags, env, env.Text, currentUUID, false, 0)
		},
	}
	cmd.Flags().StringVar(&pageID, "page-id", "", "Page id from a previous find-more call (forwards as ?page_id=)")
	return cmd
}

// runHpnSearchAll wraps the single-page runHpnSearch + emit flow with an
// auto-pagination loop. Each find-more call costs CreditCostPerFindMore
// credits; the loop stops when has_more=false, the running raw-result
// count reaches maxResults, the next call would exceed budget, or the
// context is cancelled.
//
// Pre-conditions enforced upstream by RunE:
//   - allPages is true
//   - at least one of (maxResults > 0, budget > 0)
//
// On budget exhaustion or cap hit, accumulated results are still emitted
// (rather than discarded) with a stderr notice explaining why pagination
// stopped. Bearer 402 (out of credits) mid-loop bails the same way.
func runHpnSearchAll(cmd *cobra.Command, flags *rootFlags, c *api.Client, firstEnv api.SearchEnvelope, query, currentUUID string, firstDegreeOnly bool, minScore float64, maxResults, budget int, pollOpts *api.PollSearchOptions) error {
	allRows := normalizeHpnPage(firstEnv, currentUUID)
	pagesFetched := 1
	creditsSpent := CreditCostPerSearch

	hasMore := firstEnv.HasMore
	searchID := firstEnv.Id

	for hasMore {
		if maxResults > 0 && len(allRows) >= maxResults {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"reached --max-results %d after %d pages (%d credits spent); stopping pagination\n",
				maxResults, pagesFetched, creditsSpent,
			)
			break
		}
		if budget > 0 && creditsSpent+CreditCostPerFindMore > budget {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"next find-more would exceed --budget %d (already spent %d); stopping pagination after %d pages\n",
				budget, creditsSpent, pagesFetched,
			)
			break
		}

		fmEnv, fmErr := c.FindMore(cmd.Context(), searchID)
		if fmErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"find-more failed after %d pages (%d credits spent), emitting accumulated results: %v\n",
				pagesFetched, creditsSpent, fmErr,
			)
			break
		}
		creditsSpent += CreditCostPerFindMore
		fmt.Fprintf(cmd.ErrOrStderr(),
			"page %d: spent %d credits (%d total)\n",
			pagesFetched+1, CreditCostPerFindMore, creditsSpent,
		)

		pageOpts := &api.PollSearchOptions{PageID: fmEnv.PageId}
		if pollOpts != nil {
			pageOpts.Timeout = pollOpts.Timeout
			pageOpts.Interval = pollOpts.Interval
		}
		pageEnv, pollErr := c.PollSearch(cmd.Context(), searchID, pageOpts)
		if pollErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"poll for page %d failed, emitting %d accumulated results: %v\n",
				pagesFetched+1, len(allRows), pollErr,
			)
			break
		}
		allRows = append(allRows, normalizeHpnPage(pageEnv, currentUUID)...)
		hasMore = pageEnv.HasMore
		pagesFetched++
	}

	envMeta := firstEnv
	envMeta.HasMore = hasMore
	envMeta.NextPage = ""
	return renderHpnEnvelope(cmd, flags, allRows, envMeta, query, firstDegreeOnly, minScore)
}

// runHpnSearch is the POST + poll loop, factored out so tests can drive
// it directly against an httptest fixture without going through cobra.
func runHpnSearch(ctx context.Context, c *api.Client, text string, opts *api.SearchOptions, pollOpts *api.PollSearchOptions) (api.SearchEnvelope, error) {
	created, err := c.Search(ctx, text, opts)
	if err != nil {
		return api.SearchEnvelope{}, err
	}
	if created.Id == "" {
		// Dry-run path: returns synthetic body. Surface it as-is so the
		// caller renders an empty envelope rather than blocking on poll.
		return created, nil
	}
	final, err := c.PollSearch(ctx, created.Id, pollOpts)
	if err != nil {
		return api.SearchEnvelope{}, err
	}
	// Preserve the URL from the create response (poll responses often do
	// not echo it back).
	if final.URL == "" {
		final.URL = created.URL
	}
	if final.Id == "" {
		final.Id = created.Id
	}
	return final, nil
}

// normalizeHpnPage projects one page's api.SearchResult rows into
// hpnSearchResult rows. Bridge retag uses currentUUID; filters are NOT
// applied here so the multi-page caller can accumulate raw rows then
// filter once at emit time.
func normalizeHpnPage(env api.SearchEnvelope, currentUUID string) []hpnSearchResult {
	rows := make([]hpnSearchResult, 0, len(env.Results))
	for _, r := range env.Results {
		p := api.ToClientPersonWithBridges(r, env.Mutuals, currentUUID)
		row := hpnSearchResult{
			Name:           p.Name,
			CurrentTitle:   p.CurrentTitle,
			CurrentCompany: p.CurrentCompany,
			LinkedInURL:    p.LinkedInURL,
			Score:          p.Score,
		}
		if len(p.Bridges) > 0 {
			row.Bridges = bridgesToFlagship(p.Bridges)
			row.Rationale = bearerRationale(p.Bridges)
			if score := bearerScore(p.Bridges, p.Score); score > row.Score {
				row.Score = score
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// filterHpnRows applies --first-degree-only and --min-score to a row
// slice, returning the surviving rows and the count dropped.
func filterHpnRows(rows []hpnSearchResult, firstDegreeOnly bool, minScore float64) ([]hpnSearchResult, int) {
	if !firstDegreeOnly && minScore <= 0 {
		return rows, 0
	}
	out := make([]hpnSearchResult, 0, len(rows))
	for _, row := range rows {
		if firstDegreeOnly {
			hasSelf := false
			for _, b := range row.Bridges {
				if b.Kind == "self_graph" {
					hasSelf = true
					break
				}
			}
			if !hasSelf {
				continue
			}
		}
		if minScore > 0 && row.Score < minScore {
			continue
		}
		out = append(out, row)
	}
	return out, len(rows) - len(out)
}

// renderHpnEnvelope applies filters to a pre-normalized row slice and
// writes the final JSON-or-table envelope. envMeta supplies envelope-level
// metadata (search_id, status, has_more, etc.) since the caller may have
// stitched multiple pages together.
func renderHpnEnvelope(cmd *cobra.Command, flags *rootFlags, rows []hpnSearchResult, envMeta api.SearchEnvelope, query string, firstDegreeOnly bool, minScore float64) error {
	totalBeforeFilters := len(rows)
	filtered, dropped := filterHpnRows(rows, firstDegreeOnly, minScore)
	if dropped > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"filters dropped %d of %d results (--first-degree-only=%v, --min-score=%g)\n",
			dropped, totalBeforeFilters, firstDegreeOnly, minScore,
		)
	}
	out := hpnSearchEnvelope{
		SearchID:  envMeta.Id,
		URL:       envMeta.URL,
		Query:     query,
		Status:    envMeta.Status,
		Source:    "api",
		Completed: envMeta.Status == api.StatusCompleted,
		Count:     len(filtered),
		Results:   filtered,
		HasMore:   envMeta.HasMore,
		NextPage:  envMeta.NextPage,
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return flags.printJSON(cmd, out)
	}
	return printHpnSearchTable(cmd.OutOrStdout(), out)
}

// emitHpnSearchEnvelope renders an api.SearchEnvelope to either a JSON
// envelope (jq-friendly) or a human-readable table, honoring the
// --json / --quiet / --compact root flags. Single-page convenience that
// composes normalizeHpnPage + renderHpnEnvelope.
//
// currentUUID retags the user's own self-entry in envelope mutuals to
// BridgeKindSelfGraph so renderers (and the --first-degree-only filter)
// can distinguish 1st-degree from 2nd-degree. Pass "" to disable retag.
//
// firstDegreeOnly drops rows lacking any self_graph bridge after retag.
// minScore drops rows with score < minScore (after bridge-affinity score
// promotion). Both filters are post-fetch and cost no extra credits.
func emitHpnSearchEnvelope(cmd *cobra.Command, flags *rootFlags, env api.SearchEnvelope, query string, currentUUID string, firstDegreeOnly bool, minScore float64) error {
	rows := normalizeHpnPage(env, currentUUID)
	return renderHpnEnvelope(cmd, flags, rows, env, query, firstDegreeOnly, minScore)
}

// lookupCurrentUserUUID best-effort fetches the authenticated user's
// Happenstance UUID via the cookie surface so bearer searches can retag
// self-bridges. Returns "" silently when cookie auth is unavailable or
// the lookup fails — callers must tolerate empty currentUUID (every
// bridge falls back to BridgeKindFriend, matching pre-2026-05 behavior).
func lookupCurrentUserUUID(flags *rootFlags) string {
	cookieClient, _ := flags.newClient()
	if cookieClient == nil || !cookieClient.HasCookieAuth() {
		return ""
	}
	uuid, _ := fetchCurrentUserUUID(cookieClient)
	return uuid
}

func printHpnSearchTable(w io.Writer, env hpnSearchEnvelope) error {
	fmt.Fprintf(w, "%s - %d results (%s)\n\n", env.Query, env.Count, env.Status)
	if env.Count == 0 {
		fmt.Fprintln(w, "no people found. Try broadening the query, or pass --include-friends-connections / --include-my-connections.")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, bold("RANK")+"\t"+bold("NAME")+"\t"+bold("TITLE")+"\t"+bold("COMPANY")+"\t"+bold("SCORE"))
	for i, p := range env.Results {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%.2f\n",
			i+1,
			truncate(p.Name, 32),
			truncate(p.CurrentTitle, 32),
			truncate(p.CurrentCompany, 28),
			p.Score,
		)
	}
	return tw.Flush()
}

// buildPollSearchOptions translates the flag values into the api.PollSearchOptions
// struct. Zero seconds means "use the api package default" — the api
// package treats opts.Timeout / opts.Interval == 0 as "use defaults".
func buildPollSearchOptions(timeoutSec, intervalSec int, pageID string) *api.PollSearchOptions {
	opts := &api.PollSearchOptions{PageID: pageID}
	if timeoutSec > 0 {
		opts.Timeout = time.Duration(timeoutSec) * time.Second
	}
	if intervalSec > 0 {
		opts.Interval = time.Duration(intervalSec) * time.Second
	}
	return opts
}

// checkSearchBudget reports whether a credit-spending call should be
// blocked by --budget. Returns (true, message) when the call would exceed
// the budget; the caller renders the message and exits 0 (the user
// opted to refuse to spend, which is a successful outcome).
//
// A budget of 0 means "unlimited" and never blocks. A negative budget
// also means unlimited (defensive: cobra's IntVar default is 0 but a
// stray --budget=-1 should not be treated as "always block").
func checkSearchBudget(budget, cost int) (bool, string) {
	if budget <= 0 {
		return false, ""
	}
	if cost <= budget {
		return false, ""
	}
	return true, fmt.Sprintf("would exceed budget: this call costs %d credits, --budget is set to %d. Skipping.", cost, budget)
}

// classifyHpnError maps a happenstance-api package error to the canonical
// cliError exit-code contract:
//   - bearer rate-limited (typed *api.RateLimitError) -> exit 7
//   - 401 unauthorized                                -> exit 4 (auth)
//   - 402 payment required                            -> exit 5 (api err)
//   - 404 not found                                   -> exit 3
//   - everything else                                 -> exit 5 (api err)
//
// FAILED_AMBIGUOUS / FAILED status surfacing is handled by the caller —
// those are server-side terminal statuses, not transport errors, and the
// envelope still decodes OK.
func classifyHpnError(err error) error {
	if err == nil {
		return nil
	}
	var rl *api.RateLimitError
	if errors.As(err, &rl) {
		return rateLimitErr(err)
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401 unauthorized"):
		return authErr(err)
	case strings.Contains(msg, "404 not found"):
		return notFoundErr(err)
	default:
		return apiErr(err)
	}
}
