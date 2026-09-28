// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type rankView struct {
	Query    string       `json:"query"`
	Domain   string       `json:"domain"`
	Location string       `json:"location,omitempty"`
	Found    bool         `json:"found"`
	Position int          `json:"position,omitempty"`
	Link     string       `json:"link,omitempty"`
	Title    string       `json:"title,omitempty"`
	Matches  []serpResult `json:"matches"`
	Scanned  int          `json:"scanned_results"`
	Note     string       `json:"note,omitempty"`
}

// rankDomain finds every result whose host is domain or a subdomain of it.
func rankDomain(results []serpResult, domain string) []serpResult {
	matches := make([]serpResult, 0)
	for _, r := range results {
		if domainMatches(r.Domain, domain) {
			matches = append(matches, r)
		}
	}
	return matches
}

func newNovelRankCmd(flags *rootFlags) *cobra.Command {
	var flagQ string
	var opts serpOptions

	cmd := &cobra.Command{
		Use:   "rank [domain]",
		Short: "See the position of a domain for a query, optionally from a specific country, in one call.",
		Long: strings.Trim(`
Run one Google web search and report where a domain first appears in the
organic results. Subdomains count as a match (docs.example.com matches
example.com). One call spends one search credit; --num bounds how deep the
check looks.

Use this command to find where one domain ranks for a query. Do NOT use it to
list all results; use 'web' instead.`, "\n"),
		Example: strings.Trim(`
  serply-pp-cli rank github.com --q "open source cli" --x-proxy-location US --agent
  serply-pp-cli rank serply.io --q "serp api" --num 10 --gl gb`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "domain=github.com;--q=open source cli;--num=10",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a domain argument is required, for example: rank github.com --q \"open source cli\""))
			}
			domain := normalizeDomain(args[0])
			if domain == "" {
				return usageErr(fmt.Errorf("could not read a domain from %q", args[0]))
			}
			if strings.TrimSpace(flagQ) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--q is required"))
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
			results, err := fetchVertical(ctx, c, serpVerticals["web"], flagQ, opts, false)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			view := rankView{
				Query:    flagQ,
				Domain:   domain,
				Location: strings.ToUpper(opts.Location),
				Matches:  rankDomain(results, domain),
				Scanned:  len(results),
			}
			if len(view.Matches) > 0 {
				first := view.Matches[0]
				view.Found, view.Position, view.Link, view.Title = true, first.Position, first.Link, first.Title
			} else {
				view.Note = fmt.Sprintf("%s is not in the top %d results; raise --num to look deeper", domain, len(results))
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				if !view.Found {
					fmt.Fprintf(w, "%s: not found for %q (checked %d results)\n", domain, flagQ, view.Scanned)
					return nil
				}
				fmt.Fprintf(w, "%s ranks #%d for %q\n", domain, view.Position, flagQ)
				for _, m := range view.Matches {
					fmt.Fprintf(w, "  #%-3d %s\n       %s\n", m.Position, m.Title, m.Link)
				}
				return nil
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&flagQ, "q", "", "Search query to check the domain's position for.")
	cmd.Flags().IntVar(&opts.Num, "num", 10, "How many results to check (Serply returns up to about 10 per call).")
	cmd.Flags().StringVar(&opts.Location, "x-proxy-location", "", "Two-letter country code to search from, for example US or GB.")
	cmd.Flags().StringVar(&opts.Device, "x-user-agent", "", "Device type to emulate: desktop or mobile.")
	cmd.Flags().StringVar(&opts.Gl, "gl", "", "Country code for results, for example us.")
	cmd.Flags().StringVar(&opts.Hl, "hl", "", "Interface language code, for example en.")
	return cmd
}
