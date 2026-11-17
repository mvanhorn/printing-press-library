// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

type groundView struct {
	Question      string         `json:"question"`
	Origins       []string       `json:"searched"`
	Sources       []serp.Source  `json:"sources"`
	FetchFailures []fetchFailure `json:"fetch_failures,omitempty"`
	CreditsUsed   int            `json:"credits_used"`
}

type groundOrigin struct {
	name   string
	engine *serp.Engine // nil = Google News
}

func newGroundCmd(flags *rootFlags) *cobra.Command {
	var flagEngines, country, lang string
	var limit int
	var news bool
	cmd := &cobra.Command{
		Use:   "ground [question]",
		Short: "Get one numbered, deduplicated list of sources for a question, across engines and news",
		Long: `Search several engines (and Google News with --news) for a question, merge the results by
canonical URL, and number the sources. Pages found by more engines come first. Use it to
ground an answer with citations. Costs 1 credit per engine (plus 1 for --news).
Do NOT use this command to compare how engines rank results; use 'engines compare' instead.`,
		Example: strings.Trim(`
  autom-pp-cli ground "what is model context protocol" --limit 8
  autom-pp-cli ground "serp api pricing" --engines google,brave --news --agent --select sources.n,sources.url`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "question=what is model context protocol;--engines=google,bing",
			// Any free text is a valid question, so there is no invalid input to probe.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ground")
			}
			question := strings.TrimSpace(strings.Join(args, " "))
			if question == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a question is required, for example: ground \"what is model context protocol\""))
			}
			engines, err := serp.ParseEngineList(flagEngines)
			if err != nil {
				return usageErr(err)
			}
			origins := make([]groundOrigin, 0, len(engines)+1)
			for i := range engines {
				origins = append(origins, groundOrigin{name: engines[i].Name, engine: &engines[i]})
			}
			if news {
				origins = append(origins, groundOrigin{name: "news"})
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results, failures := cliutil.FanoutRun(ctx, origins,
				func(o groundOrigin) string { return o.name },
				func(ctx context.Context, o groundOrigin) ([]serp.Result, error) {
					if o.engine == nil {
						return fetchNews(ctx, c, question, country, lang)
					}
					return fetchResults(ctx, c, *o.engine, question, searchOpts{Country: country, Lang: lang})
				}, cliutil.WithConcurrency(min(len(origins), searchConcurrency)))
			view := groundView{Question: question, Origins: make([]string, 0), Sources: make([]serp.Source, 0)}
			names := make([]string, 0, len(results))
			lists := make([][]serp.Result, 0, len(results))
			for _, r := range results {
				names = append(names, r.Source)
				lists = append(lists, r.Value)
			}
			view.Origins = names
			fails, errs := fanoutFailures(failures)
			view.FetchFailures = fails
			if len(names) == 0 {
				return printThenFail(cmd, flags, view, allFailedErr("search", fails, errs))
			}
			view.CreditsUsed = len(names)
			view.Sources = serp.MergeSources(names, lists, limit)
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d searches failed; sources come from the remaining %d\n", len(failures), len(origins), len(names))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(view.Sources) == 0 {
				fmt.Fprintln(w, "No sources found.")
				return nil
			}
			for _, s := range view.Sources {
				fmt.Fprintf(w, "[%d] %s\n    %s  (%s)\n", s.N, s.Title, s.URL, strings.Join(s.FoundIn, ", "))
				if s.Snippet != "" {
					fmt.Fprintf(w, "    %s\n", truncateText(s.Snippet, 160))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagEngines, "engines", "google,bing,brave", "Engines to search, comma-separated")
	cmd.Flags().BoolVar(&news, "news", false, "Also search Google News")
	cmd.Flags().StringVar(&country, "country", "", "Two-letter country code, for example us")
	cmd.Flags().StringVar(&lang, "lang", "", "Language code, for example en")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum sources (0 = all)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newGroundCmd(flags))
	})
}
