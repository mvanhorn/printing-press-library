// Copyright 2026 abhirup-dev and contributors. Licensed under Apache-2.0.
// Hand-authored novel command: stock-watch.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newNovelStockWatchCmd builds `stock-watch <sc_id>`: the live pricefeed quote
// for one ticker plus the most recent news tagged to that stock, in one call.
//
// The sc_id is moneycontrol's internal stock identifier (e.g. RI for Reliance,
// HDF01 for HDFC Bank). Tag-page news is fetched by deriving the slug; the
// canonical slug for most large caps is the lowercased company name with
// spaces -> hyphens (reliance-industries), but the user can override with
// --tag-slug when the derivation is wrong.
func newNovelStockWatchCmd(flags *rootFlags) *cobra.Command {
	var tagSlug string
	var newsLimit int
	var scIDFlag string

	cmd := &cobra.Command{
		Use:   "stock-watch",
		Short: "Live quote plus recent tagged news for one stock.",
		Long: "Live quote plus recent tagged news for one stock.\n\n" +
			"sc_id is moneycontrol's internal identifier (RI = Reliance, HDF01 = HDFC Bank,\n" +
			"ITC = ITC, INF = Infosys). Tag-page news is derived from --tag-slug, which\n" +
			"defaults to a best-effort slug; override it when the derivation is wrong.",
		Example: `  moneycontrol-pp-cli stock-watch --sc-id RI
  moneycontrol-pp-cli stock-watch --sc-id RI --tag-slug reliance-industries --news-limit 5 --json`,
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--sc-id=RI;--tag-slug=reliance-industries;--news-limit=1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stock-watch")
			}
			scID := scIDFlag
			if scID == "" && len(args) >= 1 {
				scID = args[0]
			}
			if scID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("sc_id is required (pass --sc-id or a positional, e.g. RI for Reliance)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			priceClient, err := newPriceAPIClient(flags)
			if err != nil {
				return err
			}
			wwwClient, err := flags.newClient()
			if err != nil {
				return err
			}

			// Quote leg
			quoteRaw, err := priceClient.Get(ctx, "/pricefeed/nse/equitycash/"+scID, nil)
			if err != nil {
				return fmt.Errorf("fetching quote for %s: %w", scID, err)
			}
			var quoteWrap struct {
				Code string          `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			_ = json.Unmarshal(quoteRaw, &quoteWrap)
			if quoteWrap.Code != "200" {
				return fmt.Errorf("quote for %s returned api code %q (wrong sc_id?)", scID, quoteWrap.Code)
			}

			// News leg — derive tag slug if not provided
			slug := tagSlug
			if slug == "" {
				slug = deriveTagSlug(scID)
			}
			news, newsErr := fetchNewsLinks(ctx, wwwClient, "/news/tags/"+slug+".html", newsLimit)

			view := struct {
				ScID    string          `json:"sc_id"`
				Quote   json.RawMessage `json:"quote"`
				News    []articleLink   `json:"news"`
				TagSlug string          `json:"tag_slug"`
				NewsErr string          `json:"news_error,omitempty"`
			}{
				ScID:    scID,
				Quote:   quoteWrap.Data,
				News:    news,
				TagSlug: slug,
			}
			if newsErr != nil {
				view.NewsErr = newsErr.Error()
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: news fetch for tag %q failed; quote still returned\n", slug)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			// Human output
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "STOCK WATCH: %s\n", scID)
			fmt.Fprintf(out, "Quote: %s\n", compactJSONOneLine(quoteWrap.Data))
			if newsErr != nil {
				fmt.Fprintf(out, "News: (unavailable: %s)\n", newsErr)
			} else {
				fmt.Fprintf(out, "News (tag=%s):\n", slug)
				for i, h := range news {
					fmt.Fprintf(out, "  %d. %s\n     %s\n", i+1, h.Title, h.URL)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&scIDFlag, "sc-id", "", "moneycontrol sc_id (e.g. RI for Reliance); alternatively pass as a positional")
	cmd.Flags().StringVar(&tagSlug, "tag-slug", "", "moneycontrol news tag slug (default: derived from sc_id)")
	cmd.Flags().IntVar(&newsLimit, "news-limit", 5, "max tagged-news headlines to return")
	return cmd
}

// deriveTagSlug returns a best-effort news tag slug for common large-cap sc_ids.
// Moneycontrol tag slugs do not follow a strict rule from sc_id; this table
// covers the most-watched names. For anything else the user passes --tag-slug.
var scIDTagSlugs = map[string]string{
	"RI":    "reliance-industries",
	"HDF01": "hdfc-bank",
	"INF":   "infosys",
	"ITC":   "itc",
	"SBIN":  "state-bank-of-india",
	"ICI01": "icici-bank",
	"TCS":   "tata-consultancy-services",
	"LT":    "larsen-toubro",
	"BHARTIARTL": "bharti-airtel",
}

func deriveTagSlug(scID string) string {
	if s, ok := scIDTagSlugs[scID]; ok {
		return s
	}
	return ""
}
