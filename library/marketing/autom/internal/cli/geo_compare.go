// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

// maxCompareSides caps the countries of one geo compare call.
const maxCompareSides = 10

func newNovelGeoCompareCmd(flags *rootFlags) *cobra.Command {
	var flagCountries, engine, flagDomain, lang, dbPath string
	var top int
	var save, noValidate bool
	cmd := &cobra.Command{
		Use:   "compare [query]",
		Short: "Run one query on one engine across several countries: your domain's position and the shared results per country",
		Long: `Use this command to run ONE query on ONE engine across SEVERAL countries.
Country codes are first checked with Autom's free Google countries finder (skip with
--no-validate). Costs 1 credit per country.
Do NOT use this command to compare engines; use 'engines compare' instead.
Do NOT use it to track one domain's positions over time; use 'rank history' instead.`,
		Example: strings.Trim(`
  autom-pp-cli geo compare "serp api pricing" --countries fr,be,ca --domain autom.dev --agent
  autom-pp-cli geo compare "serp api pricing" --countries us,gb --engine brave --save`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":              "true",
			"mcp:read-only-unless-flags": "save",
			"pp:data-source":             "live",
			"pp:happy-args":              "query=serp api pricing;--countries=fr,be",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "geo compare")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a query is required, for example: geo compare \"serp api pricing\" --countries fr,be"))
			}
			countries := make([]string, 0)
			seen := map[string]bool{}
			for _, cc := range splitCSV(flagCountries) {
				cc = strings.ToLower(cc)
				if len(cc) != 2 {
					return usageErr(fmt.Errorf("country %q is not a two-letter code", cc))
				}
				if !seen[cc] {
					seen[cc] = true
					countries = append(countries, cc)
				}
			}
			if len(countries) > maxCompareSides {
				return usageErr(fmt.Errorf("--countries takes at most %d codes per run (each costs 1 credit)", maxCompareSides))
			}
			if len(countries) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--countries needs at least two codes, for example --countries fr,be"))
			}
			e, err := serp.LookupEngine(engine)
			if err != nil {
				return usageErr(err)
			}
			if _, err := domainArg(flagDomain, true); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if !noValidate {
				bad, err := unknownCountries(ctx, cmd, c, countries)
				if err != nil {
					return err
				}
				if len(bad) > 0 {
					return usageErr(fmt.Errorf("unknown Google country code(s): %s (look them up with: autom-pp-cli finder google-countries --query <name>)", strings.Join(bad, ", ")))
				}
			}
			results, failures := cliutil.FanoutRun(ctx, countries,
				func(cc string) string { return cc },
				func(ctx context.Context, cc string) ([]serp.Result, error) {
					return fetchResults(ctx, c, e, query, searchOpts{Country: cc, Lang: lang})
				}, cliutil.WithConcurrency(min(len(countries), searchConcurrency)))
			view := compareView{Query: query, Engine: e.Name, Lang: strings.ToLower(lang), Domain: serp.NormalizeDomain(flagDomain), Top: top, Sides: make([]sideResult, 0)}
			full := map[string][]serp.Result{}
			keys := make([]string, 0, len(results))
			for _, r := range results {
				full[r.Source] = r.Value
				keys = append(keys, r.Source)
			}
			fails, errs := fanoutFailures(failures)
			view.FetchFailures = fails
			if len(keys) == 0 {
				return printThenFail(cmd, flags, view, allFailedErr("country", fails, errs))
			}
			view.CreditsUsed = len(keys)
			fillSides(ctx, cmd, &view, keys, full, save, dbPath, func(key string) (string, searchOpts) {
				return e.Name, searchOpts{Country: key, Lang: lang}
			})
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d countries failed; the comparison covers the remaining %d\n", len(failures), len(countries), len(keys))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printCompareHuman(cmd, view, "country")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCountries, "countries", "", "Two-letter country codes, comma-separated, for example fr,be,ca")
	cmd.Flags().StringVar(&engine, "engine", "google", "Engine: google, bing, brave, duckduckgo or yahoo")
	cmd.Flags().StringVar(&flagDomain, "domain", "", "Domain whose position is reported in each country, for example autom.dev")
	cmd.Flags().StringVar(&lang, "lang", "", "Language code applied to every country, for example en")
	cmd.Flags().IntVar(&top, "top", 10, "Compare the first N results of each country")
	cmd.Flags().BoolVar(&save, "save", false, "Save each country's results page for rank history and movers")
	cmd.Flags().BoolVar(&noValidate, "no-validate", false, "Skip the free country-code check")
	addDBFlag(cmd, &dbPath)
	return cmd
}

// unknownCountries checks codes with the free Google countries finder. A
// finder failure is reported on stderr and does not block the comparison.
func unknownCountries(ctx context.Context, cmd *cobra.Command, c *client.Client, codes []string) ([]string, error) {
	bad := make([]string, 0)
	for _, cc := range codes {
		raw, err := c.Get(ctx, "/v1/finder/google-countries", fullResponse(map[string]string{"query": cc}))
		if err != nil {
			classified := classifyAPIErrorOnly(err)
			if isAuthOrRateLimit(classified) {
				return nil, classified // a key or rate-limit problem fails every search too
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not check country code %q: %s\n", cc, firstLine(err.Error()))
			continue
		}
		var list []struct {
			Code string `json:"country_code"`
		}
		if json.Unmarshal(raw, &list) != nil {
			continue
		}
		found := false
		for _, it := range list {
			if strings.EqualFold(it.Code, cc) {
				found = true
				break
			}
		}
		if !found {
			bad = append(bad, cc)
		}
	}
	return bad, nil
}
