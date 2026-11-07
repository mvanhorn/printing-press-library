// Copyright 2026 Ricardo Batista and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/cloro/internal/cliutil"
)

// citedEngine is one engine that cited can query.
type citedEngine struct {
	Name string
	Path string
	// Body builds the request body for a prompt and a country.
	Body func(prompt, country string) map[string]any
	// Sources reads the cited sources from the response body.
	Sources func(data json.RawMessage) ([]citedSource, error)
}

type citedSource struct {
	Position int    `json:"position"`
	URL      string `json:"url"`
}

type citedEngineResult struct {
	Engine    string   `json:"engine"`
	Cited     bool     `json:"cited"`
	Positions []int    `json:"positions"`
	URLs      []string `json:"urls"`
	Sources   int      `json:"sources"`
}

type citedFailure struct {
	Engine string `json:"engine"`
	Error  string `json:"error"`
}

type citedView struct {
	Domain        string              `json:"domain"`
	Prompt        string              `json:"prompt"`
	Country       string              `json:"country"`
	CitedBy       []string            `json:"cited_by"`
	Engines       []citedEngineResult `json:"engines"`
	FetchFailures []citedFailure      `json:"fetch_failures,omitempty"`
}

func promptBody(prompt, country string) map[string]any {
	return map[string]any{"prompt": prompt, "country": country}
}

// sourcesAt reads result.<path...>.sources from a monitor response.
func sourcesAt(path ...string) func(json.RawMessage) ([]citedSource, error) {
	return func(data json.RawMessage) ([]citedSource, error) {
		var node any
		if err := json.Unmarshal(data, &node); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		for _, key := range append([]string{"result"}, path...) {
			obj, ok := node.(map[string]any)
			if !ok {
				return nil, nil
			}
			node = obj[key]
		}
		obj, ok := node.(map[string]any)
		if !ok {
			return nil, nil
		}
		raw, err := json.Marshal(obj["sources"])
		if err != nil {
			return nil, err
		}
		var sources []citedSource
		if err := json.Unmarshal(raw, &sources); err != nil {
			return nil, fmt.Errorf("parsing sources: %w", err)
		}
		return sources, nil
	}
}

// citedEngines lists the engines in their default order. The AI Overview
// runs a Google search with the prompt as the query. Bing AI Mode is not in
// the default list; name it in --engines to run it.
var citedEngines = []citedEngine{
	{Name: "chatgpt", Path: "/v1/monitor/chatgpt", Body: promptBody, Sources: sourcesAt()},
	{Name: "aimode", Path: "/v1/monitor/aimode", Body: promptBody, Sources: sourcesAt()},
	{Name: "gemini", Path: "/v1/monitor/gemini", Body: promptBody, Sources: sourcesAt()},
	{
		Name: "aioverview",
		Path: "/v1/monitor/google",
		Body: func(prompt, country string) map[string]any {
			return map[string]any{
				"query":   prompt,
				"country": country,
				"include": map[string]any{"aioverview": true},
			}
		},
		Sources: sourcesAt("aioverview"),
	},
	{Name: "bing-aimode", Path: "/v1/monitor/bing/aimode", Body: promptBody, Sources: sourcesAt()},
}

const citedDefaultEngines = "chatgpt,aimode,gemini,aioverview"

// citedAllEngines names every engine that --engines accepts.
const citedAllEngines = citedDefaultEngines + ",bing-aimode"

// parseCitedEngines returns the engines named in a comma-separated list, in
// the order given, without duplicates.
func parseCitedEngines(list string) ([]citedEngine, error) {
	byName := map[string]citedEngine{}
	for _, e := range citedEngines {
		byName[e.Name] = e
	}
	seen := map[string]bool{}
	var out []citedEngine
	for _, name := range strings.Split(list, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || seen[name] {
			continue
		}
		e, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown engine %q; use one or more of %s", name, citedAllEngines)
		}
		seen[name] = true
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--engines is empty; use one or more of %s", citedAllEngines)
	}
	return out, nil
}

// normalizeCitedDomain turns "https://www.Example.com/path" into "example.com".
func normalizeCitedDomain(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// citedHostMatches reports whether a source URL is on domain or one of its
// subdomains. "shop.example.com" matches "example.com"; "badexample.com"
// does not.
func citedHostMatches(sourceURL, domain string) bool {
	u, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// matchCited finds the sources on domain. A source without a position gets
// its 1-based index in the list.
func matchCited(engine string, sources []citedSource, domain string) citedEngineResult {
	res := citedEngineResult{Engine: engine, Positions: make([]int, 0), URLs: make([]string, 0), Sources: len(sources)}
	for i, s := range sources {
		if !citedHostMatches(s.URL, domain) {
			continue
		}
		pos := s.Position
		if pos <= 0 {
			pos = i + 1
		}
		res.Cited = true
		res.Positions = append(res.Positions, pos)
		res.URLs = append(res.URLs, s.URL)
	}
	return res
}

func newNovelCitedCmd(flags *rootFlags) *cobra.Command {
	var flagPrompt string
	var flagCountry string
	var flagEngines string

	cmd := &cobra.Command{
		Use:   "cited [domain]",
		Short: "See whether ChatGPT, Google AI Mode, Gemini and the Google AI Overview cite a domain for a prompt",
		Long: strings.Trim(`
Send one prompt to each engine in parallel and report, for each engine,
whether a source it cites is on the domain, and at which source positions.
Subdomains count as a match (shop.example.com matches example.com). A source
with an empty URL does not match; resolve its redirect link with 'goto'.

The AI Overview check runs a Google search with the prompt as the query and
reads the sources of the AI Overview. Each engine is one sync request at its
sync price: ChatGPT 7 credits, AI Mode 6, Gemini 6, Google with the AI
Overview 7. Use --engines to run fewer engines. Bing AI Mode (6 credits) runs
only when you name it in --engines.

Answers and their sources change between runs of the same prompt, so one run
is a sample, not a rank.

Use this command to see whether one domain is cited for a prompt. Do NOT use
it to read the full answers; use 'chatgpt', 'aimode', 'gemini' or 'google'
instead.`, "\n"),
		Example: strings.Trim(`
  cloro-pp-cli cited nike.com --prompt "best running shoes for flat feet" --country US --engines aimode,gemini --agent
  cloro-pp-cli cited nike.com --prompt "best running shoes for flat feet"`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "domain=nike.com;--prompt=best running shoes for flat feet;--country=US;--engines=aimode,gemini",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cited")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a domain argument is required, for example: cited nike.com --prompt \"best running shoes for flat feet\""))
			}
			domain := normalizeCitedDomain(args[0])
			if domain == "" {
				return usageErr(fmt.Errorf("could not read a domain from %q", args[0]))
			}
			if strings.TrimSpace(flagPrompt) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--prompt is required"))
			}
			country := strings.ToUpper(strings.TrimSpace(flagCountry))
			if country == "" {
				return usageErr(fmt.Errorf("--country is required, for example US"))
			}
			engines, err := parseCitedEngines(flagEngines)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// A replay of a scrape bills it again, so the client must not
			// retry after an ambiguous failure.
			headers := map[string]string{"X-Printing-Press-Replay-Safe": "false"}
			results, failures := cliutil.FanoutRun(ctx, engines,
				func(e citedEngine) string { return e.Name },
				func(ctx context.Context, e citedEngine) (citedEngineResult, error) {
					data, _, err := c.PostQueryWithParamsAndHeaders(ctx, e.Path, nil, e.Body(flagPrompt, country), headers)
					if err != nil {
						return citedEngineResult{}, err
					}
					sources, err := e.Sources(data)
					if err != nil {
						return citedEngineResult{}, err
					}
					return matchCited(e.Name, sources, domain), nil
				},
				cliutil.WithConcurrency(len(engines)),
			)

			view := citedView{
				Domain:  domain,
				Prompt:  flagPrompt,
				Country: country,
				CitedBy: make([]string, 0),
				Engines: make([]citedEngineResult, 0, len(results)),
			}
			for _, r := range results {
				view.Engines = append(view.Engines, r.Value)
				if r.Value.Cited {
					view.CitedBy = append(view.CitedBy, r.Value.Engine)
				}
			}
			for _, f := range failures {
				view.FetchFailures = append(view.FetchFailures, citedFailure{Engine: f.Source, Error: f.Err.Error()})
			}
			if len(results) == 0 && len(failures) > 0 {
				return classifyAPIError(cmd.OutOrStdout(), failures[0].Err, flags)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d engines failed; results cover the remaining %d\n", len(failures), len(engines), len(results))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, view)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s for %q (%s): cited by %d of %d engines\n", domain, flagPrompt, country, len(view.CitedBy), len(view.Engines))
			for _, e := range view.Engines {
				if !e.Cited {
					fmt.Fprintf(w, "  %-11s not cited (%d sources)\n", e.Engine, e.Sources)
					continue
				}
				fmt.Fprintf(w, "  %-11s cited at position %s of %d sources\n", e.Engine, joinInts(e.Positions), e.Sources)
				for _, u := range e.URLs {
					fmt.Fprintf(w, "              %s\n", u)
				}
			}
			for _, f := range view.FetchFailures {
				fmt.Fprintf(w, "  %-11s failed: %s\n", f.Engine, f.Error)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagPrompt, "prompt", "", "Prompt to send to each engine (the search query for the AI Overview).")
	cmd.Flags().StringVar(&flagCountry, "country", "US", "ISO 3166-1 alpha-2 country code the engines answer from.")
	cmd.Flags().StringVar(&flagEngines, "engines", citedDefaultEngines, "Comma-separated engines to check: chatgpt, aimode, gemini, aioverview, bing-aimode.")
	return cmd
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, ", ")
}
