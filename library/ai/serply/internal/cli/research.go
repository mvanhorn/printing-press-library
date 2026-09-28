// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

type researchSource struct {
	N        int    `json:"n"`
	Vertical string `json:"vertical"`
	serpResult
}

type researchFailure struct {
	Vertical string `json:"vertical"`
	Error    string `json:"error"`
}

type researchView struct {
	Topic         string            `json:"topic"`
	Verticals     []string          `json:"verticals"`
	Counts        map[string]int    `json:"counts"`
	Sources       []researchSource  `json:"sources"`
	FetchFailures []researchFailure `json:"fetch_failures,omitempty"`
}

var defaultResearchVerticals = []string{"web", "news", "scholar"}

func parseResearchVerticals(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultResearchVerticals, nil
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		v := strings.ToLower(strings.TrimSpace(part))
		if v == "" || seen[v] {
			continue
		}
		if _, ok := serpVerticals[v]; !ok {
			return nil, usageErr(fmt.Errorf("unknown vertical %q for --verticals: use web, news or scholar", v))
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, usageErr(fmt.Errorf("--verticals needs at least one of web, news, scholar"))
	}
	return out, nil
}

// mergeResearch numbers sources in vertical order and drops URLs already cited
// by an earlier vertical, so each link gets exactly one citation number.
func mergeResearch(order []string, byVertical map[string][]serpResult) ([]researchSource, map[string]int) {
	sources := []researchSource{}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, v := range order {
		counts[v] = 0
		for _, r := range byVertical[v] {
			k := normalizeLink(r.Link)
			if seen[k] {
				continue
			}
			seen[k] = true
			counts[v]++
			sources = append(sources, researchSource{N: len(sources) + 1, Vertical: v, serpResult: r})
		}
	}
	return sources, counts
}

func newNovelResearchCmd(flags *rootFlags) *cobra.Command {
	var flagQ, flagVerticals string
	var opts serpOptions

	cmd := &cobra.Command{
		Use:   "research [query]",
		Short: "Get one deduplicated, numbered source list for a topic from web, news and scholar results at once.",
		Long: strings.Trim(`
Search Google web, Google News and Google Scholar for one topic in parallel,
drop duplicate URLs, and return a single numbered source list. Human output is
a markdown brief grouped by vertical with [n] citations; --json returns the
structured list. Each vertical is one search credit, so the default run costs
three. A vertical that fails is reported in fetch_failures and the others
still return.

Use this command for a multi-source cited brief on a topic. Do NOT use it when
one vertical is enough; call 'web', 'news' or 'scholar' directly.`, "\n"),
		Example: strings.Trim(`
  serply-pp-cli research "retrieval augmented generation evaluation" --num 5 --agent
  serply-pp-cli research "small language models" --verticals news,scholar`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "query=retrieval augmented generation evaluation;--num=3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "research")
			}
			topic := strings.TrimSpace(strings.Join(args, " "))
			if topic == "" {
				topic = strings.TrimSpace(flagQ)
			}
			if topic == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a query is required, as an argument or --q"))
			}
			verticals, err := parseResearchVerticals(flagVerticals)
			if err != nil {
				return err
			}
			if err := validateDevice(opts.Device); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			type outcome struct {
				results []serpResult
				err     error
			}
			outcomes := make(map[string]outcome, len(verticals))
			var mu sync.Mutex
			var wg sync.WaitGroup
			for _, v := range verticals {
				wg.Add(1)
				go func(v string) {
					defer wg.Done()
					res, err := fetchVertical(ctx, c, serpVerticals[v], topic, opts, false)
					mu.Lock()
					outcomes[v] = outcome{results: res, err: err}
					mu.Unlock()
				}(v)
			}
			wg.Wait()

			byVertical := map[string][]serpResult{}
			var failures []researchFailure
			var firstErr error
			for _, v := range verticals {
				o := outcomes[v]
				if o.err != nil {
					failures = append(failures, researchFailure{Vertical: v, Error: o.err.Error()})
					if firstErr == nil {
						firstErr = o.err
					}
					continue
				}
				byVertical[v] = o.results
			}
			if len(failures) == len(verticals) {
				return classifyAPIError(cmd.OutOrStdout(), firstErr, flags)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d verticals failed; brief built from the remaining %d\n", len(failures), len(verticals), len(verticals)-len(failures))
			}
			sources, counts := mergeResearch(verticals, byVertical)
			view := researchView{Topic: topic, Verticals: verticals, Counts: counts, Sources: sources, FetchFailures: failures}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, view)
			}
			fmt.Fprint(cmd.OutOrStdout(), renderResearchMarkdown(view))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagQ, "q", "", "Topic to research, as an alternative to the positional argument.")
	cmd.Flags().IntVar(&opts.Num, "num", 5, "Results to request per vertical; keep it small, each vertical is one credit.")
	cmd.Flags().StringVar(&flagVerticals, "verticals", "", "Comma-separated verticals to include: web, news, scholar (default all three).")
	cmd.Flags().StringVar(&opts.Location, "x-proxy-location", "", "Two-letter country code to search from, for example US or GB.")
	cmd.Flags().StringVar(&opts.Device, "x-user-agent", "", "Device type to emulate: desktop or mobile.")
	cmd.Flags().StringVar(&opts.Gl, "gl", "", "Country code for results, for example us.")
	cmd.Flags().StringVar(&opts.Hl, "hl", "", "Interface language code, for example en.")
	return cmd
}

var researchHeadings = map[string]string{"web": "Web", "news": "News", "scholar": "Scholar"}

func renderResearchMarkdown(view researchView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Research: %s\n", view.Topic)
	for _, v := range view.Verticals {
		fmt.Fprintf(&b, "\n## %s\n\n", researchHeadings[v])
		wrote := false
		for _, s := range view.Sources {
			if s.Vertical != v {
				continue
			}
			wrote = true
			line := s.Title
			if line == "" {
				line = s.Link
			}
			meta := []string{}
			if s.Author != "" {
				meta = append(meta, s.Author)
			}
			if s.Published != "" {
				meta = append(meta, s.Published)
			}
			if len(meta) > 0 {
				line += " (" + strings.Join(meta, ", ") + ")"
			}
			fmt.Fprintf(&b, "- [%d] %s\n", s.N, line)
			if s.Snippet != "" {
				fmt.Fprintf(&b, "  %s\n", s.Snippet)
			}
		}
		if !wrote {
			b.WriteString("- no results\n")
		}
	}
	if len(view.FetchFailures) > 0 {
		b.WriteString("\n## Failed verticals\n\n")
		for _, f := range view.FetchFailures {
			fmt.Fprintf(&b, "- %s: %s\n", f.Vertical, f.Error)
		}
	}
	b.WriteString("\n## Sources\n\n")
	for _, s := range view.Sources {
		fmt.Fprintf(&b, "[%d] %s\n", s.N, s.Link)
	}
	return b.String()
}
