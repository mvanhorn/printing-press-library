// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/cliutil"
	"github.com/spf13/cobra"
)

// defaultInteractiveDetailTimeout bounds the follow-up detail fetch triggered
// by --interactive's result picker, independent of the root --timeout flag
// (the search itself has already completed by this point).
const defaultInteractiveDetailTimeout = 15 * time.Second

// allSearchSources is the full, ordered set of sites `search` fans out to.
// Order here is the tie-break order for --sort relevant (each source's own
// results stay in the order it returned them; sources are concatenated in
// this order).
var allSearchSources = []string{"printables", "thingiverse", "cults3d"}

// searchJob pairs a source name with the per-source result limit it should
// request, so cliutil.FanoutRun's generic S type carries everything a
// worker needs without a second lookup by name.
type searchJob struct {
	source string
	limit  int
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var sourceList string
	var limit int
	var sortBy string
	var interactive bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Printables, Thingiverse, and Cults3D at once and merge the results",
		Long: `Fans a single query out to all three sites in parallel and prints one
merged list. A source that fails (missing credentials, rate limit, network
error) is dropped from the results with a warning on stderr instead of
failing the whole command — whatever sources succeeded still print.

Thingiverse requires THINGIVERSE_TOKEN in the environment. Cults3D requires
CULTS3D_USERNAME and CULTS3D_API_KEY. Printables needs no credentials.`,
		Example: `  # Search all three sites (10 results total, split across sources)
  printgoat-pp-cli search "benchy"

  # Only Printables and Thingiverse
  printgoat-pp-cli search "gopro mount" --source printables,thingiverse

  # 30 results, sorted by popularity where supported, agent-friendly JSON
  printgoat-pp-cli search "articulated dragon" --limit 30 --sort popular --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasChangedLocalFlags(cmd) && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "missing required argument",
						"usage": fmt.Sprintf("%s%s", cmd.CommandPath(), " <query>"),
					}, flags); printErr != nil {
						return printErr
					}
				}
				return usageErr(fmt.Errorf("missing required argument\nUsage: %s%s", cmd.CommandPath(), " <query>"))
			}
			query := args[0]

			sources, err := parseSearchSources(sourceList)
			if err != nil {
				return usageErr(err)
			}
			if limit <= 0 {
				limit = 10
			}
			switch sortBy {
			case "relevant", "popular", "newest":
			default:
				return usageErr(fmt.Errorf("invalid --sort %q: must be relevant, popular, or newest", sortBy))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			perSource := distributeLimit(limit, len(sources))
			jobs := make([]searchJob, len(sources))
			for i, src := range sources {
				jobs[i] = searchJob{source: src, limit: perSource[i]}
			}

			results, errs := cliutil.FanoutRun(ctx, jobs,
				func(j searchJob) string { return j.source },
				func(ctx context.Context, j searchJob) ([]unifiedModel, error) {
					return runSourceSearch(ctx, c, j.source, query, j.limit, sortBy)
				},
				cliutil.WithConcurrency(len(jobs)),
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			if len(results) == 0 && len(errs) > 0 {
				var msgs []string
				for _, e := range errs {
					msgs = append(msgs, fmt.Sprintf("%s: %v", e.Source, e.Err))
				}
				return classifyAPIError(fmt.Errorf("all sources failed: %s", strings.Join(msgs, "; ")), flags)
			}

			combined := make([]unifiedModel, 0, limit)
			for _, r := range results {
				combined = append(combined, r.Value...)
			}
			sortUnifiedModels(combined, sortBy)
			if len(combined) > limit {
				combined = combined[:limit]
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(combined) == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "No results.")
					return nil
				}
				rows := unifiedModelsToRows(combined)
				if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
					return err
				}
				if interactive && isTerminal(cmd.OutOrStdout()) && !cliutil.IsVerifyEnv() && !flags.noInput {
					return runInteractivePicker(cmd, c, combined)
				}
				return nil
			}
			// printJSONFiltered defaults its envelope's meta.source to
			// "local"; this command only ever fans out to live APIs, so
			// marshal and route through printOutputWithFlagsMeta directly
			// to report "live" instead (matching generated read commands'
			// convention).
			raw, err := marshalJSONNoEscape(combined)
			if err != nil {
				return err
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": "live"})
		},
	}

	cmd.Flags().StringVar(&sourceList, "source", "", "Comma-separated sources to search: printables,thingiverse,cults3d (default: all)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum total results across all sources")
	cmd.Flags().StringVar(&sortBy, "sort", "relevant", "Sort order: relevant, popular, or newest")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "After printing results in a terminal, prompt to pick a result number and view its detail (no-op under --agent/--json/--no-input or in non-terminal output)")

	return cmd
}

// runInteractivePicker prompts on stdin for a result number from a just-printed
// human-readable table and prints that result's full detail. It only runs when
// the caller has already confirmed a real terminal, non-verify, non-no-input
// context, so this never blocks a CI run or an agent invocation waiting on
// input that will never arrive.
func runInteractivePicker(cmd *cobra.Command, c *client.Client, results []unifiedModel) error {
	fmt.Fprintf(cmd.OutOrStdout(), "\nSelect a result number to view details (or press Enter to skip): ")
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return nil
	}
	choice := strings.TrimSpace(scanner.Text())
	if choice == "" {
		return nil
	}
	n, err := strconv.Atoi(choice)
	if err != nil || n < 1 || n > len(results) {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid selection %q; expected a number between 1 and %d\n", choice, len(results))
		return nil
	}
	selected := results[n-1]
	ctx, cancel := context.WithTimeout(cmd.Context(), defaultInteractiveDetailTimeout)
	defer cancel()
	detail, err := fetchModelDetail(ctx, c, selected.Source, selected.ID)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "fetching detail for %s:%s: %v\n", selected.Source, selected.ID, err)
		return nil
	}
	raw, err := marshalJSONNoEscape(detail)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return printOutput(cmd.OutOrStdout(), raw, true)
}

// parseSearchSources parses the comma-separated --source flag, defaulting to
// every known source when empty and rejecting unknown source names outright
// (a silent typo here would look like "cults3d returned nothing").
func parseSearchSources(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		out := make([]string, len(allSearchSources))
		copy(out, allSearchSources)
		return out, nil
	}
	valid := map[string]bool{}
	for _, s := range allSearchSources {
		valid[s] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, tok := range strings.Split(raw, ",") {
		s := strings.ToLower(strings.TrimSpace(tok))
		if s == "" {
			continue
		}
		if !valid[s] {
			return nil, fmt.Errorf("unknown --source %q: must be one or more of printables, thingiverse, cults3d", s)
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--source must name at least one of printables, thingiverse, cults3d")
	}
	return out, nil
}

// distributeLimit splits total as evenly as possible across n sources, with
// any remainder going to the earliest sources, and a floor of 1 per source
// so a small --limit still samples every requested site before the merged
// list gets truncated back down to total.
func distributeLimit(total, n int) []int {
	if n <= 0 {
		return nil
	}
	base := total / n
	rem := total % n
	out := make([]int, n)
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
		if out[i] < 1 {
			out[i] = 1
		}
	}
	return out
}

// runSourceSearch dispatches to the named source's search function. sortBy
// only changes behavior for printables today (mapped onto its ordering
// enum); other sources ignore it and return their own default order.
func runSourceSearch(ctx context.Context, c *client.Client, source, query string, limit int, sortBy string) ([]unifiedModel, error) {
	switch source {
	case "printables":
		ordering := "best_match"
		switch sortBy {
		case "popular":
			ordering = "popular"
		case "newest":
			// Best-effort: Printables' SearchChoicesEnum value for
			// newest-first was not confirmed against the live API at
			// research time. If the API rejects it, this source's error
			// surfaces as a per-source fanout warning, not a hard failure.
			ordering = "newest"
		}
		return searchPrintables(ctx, c, query, limit, ordering)
	case "thingiverse":
		return searchThingiverse(ctx, c, query, limit)
	case "cults3d":
		return searchCults3D(ctx, c, query, limit)
	default:
		return nil, fmt.Errorf("unknown source %q", source)
	}
}

// sortUnifiedModels re-orders the merged result set in place. "relevant"
// (the default) is a no-op: each source's own relevance ranking is
// preserved, sources simply appended in allSearchSources order.
func sortUnifiedModels(models []unifiedModel, sortBy string) {
	switch sortBy {
	case "popular":
		sort.SliceStable(models, func(i, j int) bool {
			return models[i].LikesCount+models[i].DownloadCount > models[j].LikesCount+models[j].DownloadCount
		})
	case "newest":
		sort.SliceStable(models, func(i, j int) bool {
			return models[i].PublishedAt > models[j].PublishedAt
		})
	}
}

// unifiedModelsToRows converts a []unifiedModel to the []map[string]any
// shape printAutoTable expects, round-tripping through the same json tags
// used for --json output so the two views stay consistent.
func unifiedModelsToRows(models []unifiedModel) []map[string]any {
	rows := make([]map[string]any, len(models))
	for i, m := range models {
		row := map[string]any{
			"source": m.Source,
			"id":     m.ID,
			"name":   m.Name,
			"url":    m.URL,
		}
		if m.Designer != "" {
			row["designer"] = m.Designer
		}
		if m.LikesCount != 0 {
			row["likes_count"] = m.LikesCount
		}
		if m.DownloadCount != 0 {
			row["download_count"] = m.DownloadCount
		}
		if m.PublishedAt != "" {
			row["published_at"] = m.PublishedAt
		}
		rows[i] = row
	}
	return rows
}
