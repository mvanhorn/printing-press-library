// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored `google` command family — the Scrape.do Google Scraper API
// (search, maps, news, shopping, flights, hotels, play, trends). All flat
// 10 credits and routed through the governor (lease + ledger + ceiling).
//
// `google search` is the primary workflow: it persists every SERP as a
// snapshot (raw + flattened organic rows) so `drift` and `movers` can diff
// rank changes offline, and is cache-first (a recent identical query is reused
// unless --fresh is passed) so an agent swarm never double-spends on the same
// 10-credit query within the freshness window. Hand file (no generator header).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/internal/store"
	"github.com/spf13/cobra"
)

func newGoogleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Scrape Google products (search, maps, news, shopping, flights, hotels, play, trends)",
		Long: `The Scrape.do Google Scraper API: pre-parsed JSON for Google Search and
related products. Every Google endpoint costs a flat 10 credits and is routed
through the governor (shared concurrency lease + credit ledger + spend ceiling).

'google search' is the primary command — it persists each SERP locally so
'drift' and 'movers' can diff rank changes offline, and reuses a recent
identical query unless --fresh is passed.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newGoogleSearchCmd(flags))
	cmd.AddCommand(newGoogleVerticalCmd(flags, "maps", "/plugin/google/maps/search", "Search Google Maps places by query"))
	cmd.AddCommand(newGoogleVerticalCmd(flags, "news", "/plugin/google/news", "Scrape Google News results"))
	cmd.AddCommand(newGoogleVerticalCmd(flags, "shopping", "/plugin/google/shopping", "Scrape Google Shopping results"))
	cmd.AddCommand(newGoogleVerticalCmd(flags, "flights", "/plugin/google/flights", "Scrape Google Flights results"))
	cmd.AddCommand(newGoogleVerticalCmd(flags, "hotels", "/plugin/google/hotels", "Scrape Google Hotels results"))
	cmd.AddCommand(newGoogleVerticalCmd(flags, "play", "/plugin/google/play", "Scrape Google Play Store results"))
	cmd.AddCommand(newGoogleVerticalCmd(flags, "trends", "/plugin/google/trends", "Scrape Google Trends results"))
	return cmd
}

// googleCommonFlags are the localization params shared across Google endpoints.
type googleCommonFlags struct {
	hl, gl, googleDomain, device string
	start                        int
	extra                        []string // --param key=value passthrough
	agentID                      string
	maxCredits                   int
}

func (g *googleCommonFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&g.hl, "hl", "", "Interface language code (e.g. en, es, de)")
	cmd.Flags().StringVar(&g.gl, "gl", "", "Country code for localization (e.g. us, gb, de)")
	cmd.Flags().StringVar(&g.googleDomain, "google-domain", "", "Google domain to query (e.g. google.com, google.co.uk)")
	cmd.Flags().StringVar(&g.device, "device", "", "Device emulation: desktop or mobile")
	cmd.Flags().IntVar(&g.start, "start", 0, "Result offset for pagination (0, 10, 20, ...)")
	cmd.Flags().StringArrayVar(&g.extra, "param", nil, "Extra endpoint param as key=value (repeatable)")
	cmd.Flags().StringVar(&g.agentID, "agent-id", "", "Attribution id for the credit ledger (or set SCRAPEDO_AGENT_ID)")
	cmd.Flags().IntVar(&g.maxCredits, "max-credits", 0, "Refuse to dispatch if month-to-date spend + this call would exceed N credits")
}

func (g *googleCommonFlags) params(query string) map[string]string {
	p := map[string]string{}
	if query != "" {
		p["q"] = query
	}
	if g.hl != "" {
		p["hl"] = g.hl
	}
	if g.gl != "" {
		p["gl"] = g.gl
	}
	if g.googleDomain != "" {
		p["google_domain"] = g.googleDomain
	}
	if g.device != "" {
		p["device"] = g.device
	}
	if g.start > 0 {
		p["start"] = strconv.Itoa(g.start)
	}
	for _, kv := range g.extra {
		if i := strings.Index(kv, "="); i > 0 {
			p[kv[:i]] = kv[i+1:]
		}
	}
	return p
}

func newGoogleSearchCmd(flags *rootFlags) *cobra.Command {
	var common googleCommonFlags
	var fresh bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Scrape a Google web SERP (structured JSON) and store it for offline rank tracking",
		Long: `Run a Google web search and get pre-parsed JSON (organic_results, top_ads,
related_questions, related_searches, knowledge_graph, ai_overview, and more).
Each result is stored locally as a snapshot so 'drift' and 'movers' can diff
rank changes offline. Cache-first: a recent identical query within --max-age is
reused (no re-spend) unless --fresh is passed.`,
		Example: strings.Trim(`
  scrape-do-pp-cli google search "best crm software"
  scrape-do-pp-cli google search "coffee makers" --gl us --hl en --json
  scrape-do-pp-cli google search "coffee makers" --agent --select organic_results.position,organic_results.title,organic_results.link
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "query=coffee makers",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("google search requires a query"))
			}
			paramHash := serpParamHash(query, common.gl, common.hl, common.googleDomain, common.device)

			if dryRunOK(flags) {
				payload := map[string]any{"would_search": query, "estimated_credits": 10, "param_hash": paramHash}
				return emitGov(cmd, flags, payload, fmt.Sprintf("would search %q [~10 credits]", query))
			}

			st, ext, err := openExtras(cmd.Context(), "")
			if err != nil {
				return err
			}
			defer st.Close()

			// Cache-first: reuse a recent snapshot unless --fresh / --no-cache.
			if !fresh && !flags.noCache {
				if raw, fetchedAt, found, _ := ext.LatestSnapshotRaw(cmd.Context(), paramHash); found && flags.maxAge > 0 && time.Since(fetchedAt) < flags.maxAge {
					if !flags.quiet {
						fmt.Fprintf(os.Stderr, "[google search] cache hit (%.0fm old); pass --fresh to re-scrape\n", time.Since(fetchedAt).Minutes())
					}
					return emitSERP(cmd, flags, []byte(raw), 0, "cache", -1)
				}
			}

			req := scrapeRequest{
				kind: "google:search", path: "/plugin/google/search", params: common.params(query),
				target: query, family: "google:" + query, mode: modeGoogle, estCost: 10,
				agent: resolveAgentID(common.agentID), maxCredits: common.maxCredits,
			}
			res, err := flags.runGoverned(cmd.Context(), ext, req)
			if err != nil {
				return err
			}

			// Persist the snapshot + flattened organic rows for drift/movers.
			organic := extractOrganic(res.Body)
			_ = ext.SaveSnapshot(cmd.Context(), store.SnapshotMeta{
				ParamHash: paramHash, Query: query, Gl: common.gl, Hl: common.hl,
				GoogleDomain: common.googleDomain, Device: common.device,
				FetchedAt: time.Now(), Raw: string(res.Body),
			}, organic)

			return emitSERP(cmd, flags, res.Body, res.Cost, res.CostSource, res.RemainingCredits)
		},
	}
	common.register(cmd)
	cmd.Flags().BoolVar(&fresh, "fresh", false, "Bypass the local cache and force a fresh (billed) search")
	return cmd
}

// emitSERP renders a SERP: full parsed JSON for --json/--agent (so --select
// dotted paths work), or a top-results table for humans. Cost goes to stderr.
func emitSERP(cmd *cobra.Command, flags *rootFlags, body []byte, cost int, costSource string, remaining int) error {
	if flags.asJSON {
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			return fmt.Errorf("parsing SERP response: %w", err)
		}
		if !flags.quiet && cost > 0 {
			fmt.Fprintf(os.Stderr, "[google search] cost=%d credits (%s) remaining=%d\n", cost, costSource, remaining)
		}
		return flags.printJSON(cmd, m)
	}
	organic := extractOrganic(body)
	if len(organic) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no organic results)")
		return nil
	}
	rows := make([][]string, 0, len(organic))
	for _, o := range organic {
		title := o.Title
		if len(title) > 70 {
			title = title[:67] + "..."
		}
		rows = append(rows, []string{strconv.Itoa(o.Position), title, o.Link})
	}
	if err := flags.printTable(cmd, []string{"POS", "TITLE", "LINK"}, rows); err != nil {
		return err
	}
	if !flags.quiet && cost > 0 {
		fmt.Fprintf(os.Stderr, "[google search] cost=%d credits (%s) remaining=%d\n", cost, costSource, remaining)
	}
	return nil
}

func newGoogleVerticalCmd(flags *rootFlags, name, path, short string) *cobra.Command {
	var common googleCommonFlags
	cmd := &cobra.Command{
		Use:   name + " <query>",
		Short: short + " (flat 10 credits, governed)",
		Example: fmt.Sprintf("  scrape-do-pp-cli google %s \"%s\" --gl us --json",
			name, sampleQueryFor(name)),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "query=" + sampleQueryFor(name),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				return emitGov(cmd, flags, map[string]any{"would_query": query, "endpoint": path, "estimated_credits": 10},
					fmt.Sprintf("would query google %s %q [~10 credits]", name, query))
			}
			st, ext, err := openExtras(cmd.Context(), "")
			if err != nil {
				return err
			}
			defer st.Close()
			req := scrapeRequest{
				kind: "google:" + name, path: path, params: common.params(query),
				target: query, family: "google:" + name, mode: modeGoogle, estCost: 10,
				agent: resolveAgentID(common.agentID), maxCredits: common.maxCredits,
			}
			res, err := flags.runGoverned(cmd.Context(), ext, req)
			if err != nil {
				return err
			}
			var m any
			if jerr := json.Unmarshal(res.Body, &m); jerr != nil {
				m = map[string]any{"raw": string(res.Body)}
			}
			if !flags.quiet && res.Cost > 0 {
				fmt.Fprintf(os.Stderr, "[google %s] cost=%d credits (%s) remaining=%d\n", name, res.Cost, res.CostSource, res.RemainingCredits)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, m)
			}
			// Human: pretty-print the JSON (these verticals have varied shapes).
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(m)
		},
	}
	common.register(cmd)
	return cmd
}

func sampleQueryFor(name string) string { //nolint:gocyclo
	switch name {
	case "maps":
		return "coffee shops in austin"
	case "news":
		return "openai"
	case "shopping":
		return "coffee maker"
	case "flights":
		return "JFK to LAX"
	case "hotels":
		return "hotels in paris"
	case "play":
		return "notion"
	case "trends":
		return "bitcoin"
	default:
		return "example"
	}
}
