package cli

// PATCH: Hand-built umbrella-page edition comparison command.
// pp:data-source live -- editions fetches the live Blu-ray.com umbrella page
// (/main/<id>/) and parses every disc edition from the response HTML.

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	"github.com/spf13/cobra"
	xhtml "golang.org/x/net/html"
)

type editionRow struct {
	ID           int     `json:"id"`
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	Country      string  `json:"country,omitempty"`
	Distributor  string  `json:"distributor,omitempty"`
	ReleaseDate  string  `json:"release_date,omitempty"`
	ListPrice    float64 `json:"list_price,omitempty"`
	CurrentPrice float64 `json:"current_price,omitempty"`
	Rating       struct {
		Overall string `json:"overall,omitempty"`
	} `json:"rating"`
	URL string `json:"url"`
}

func newNovelEditionsCmd(flags *rootFlags) *cobra.Command {
	var country string
	var noEnrich bool
	cmd := &cobra.Command{
		Use:         "editions <umbrella-id>",
		Short:       "Compare all disc editions from a Blu-ray.com movie umbrella page.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli editions 9929 --json
  blu-ray-pp-cli editions 9929 --country US --no-enrich --json --select id,kind,country,distributor,current_price
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("umbrella-id must be numeric"))
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}
			// PATCH: GetRelease serves as a local-catalog existence probe. Missing
			// rows are not fatal (the network fetch is the source of truth for
			// editions), but database-level errors should bubble up rather than
			// being silently swallowed. Fixes Greptile P2 on PR #634.
			if _, _, err := s.GetRelease(cmd.Context(), id); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("looking up umbrella id %d in local catalog: %w", id, err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, err := bluRayGet(cmd.Context(), c, bluRaySiteURL(c, fmt.Sprintf("/main/%d/", id)), false)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows, err := parseEditionsHTML(body, country)
			if err != nil {
				return err
			}
			// PATCH: The default ("enriched") path applies the ListPrice ->
			// CurrentPrice fallback so the CURRENT column is populated for
			// editions where the umbrella page only published a list price.
			// --no-enrich opts out: it returns the raw umbrella-page values
			// even when CurrentPrice is empty, for callers who need to see
			// exactly what the page said. The prior implementation had the
			// condition inverted — only --no-enrich applied the fallback, so
			// the default produced strictly LESS data than --no-enrich, the
			// opposite of the flag's name. Fixes Greptile P1 on PR #634.
			if !noEnrich {
				for i := range rows {
					if rows[i].CurrentPrice == 0 {
						rows[i].CurrentPrice = rows[i].ListPrice
					}
				}
			}
			if flags.asJSON || flags.selectFields != "" || flags.csv || flags.quiet || flags.plain {
				return flags.printJSON(cmd, rows)
			}
			var table [][]string
			for _, r := range rows {
				table = append(table, []string{strconv.Itoa(r.ID), r.Kind, r.Country, r.Distributor, r.ReleaseDate, formatPrice(r.ListPrice), formatPrice(r.CurrentPrice), r.Rating.Overall})
			}
			return flags.printTable(cmd, []string{"ID", "KIND", "COUNTRY", "DISTRIBUTOR", "RELEASE", "LIST", "CURRENT", "RATING"}, table)
		},
	}
	cmd.Flags().StringVar(&country, "country", "", "Country filter (e.g. US, UK, DE).")
	cmd.Flags().BoolVar(&noEnrich, "no-enrich", false, "Return raw umbrella-page values; skip the list-price -> current-price fallback applied in the default 'enriched' path.")
	return cmd
}

func parseEditionsHTML(body []byte, country string) ([]editionRow, error) {
	doc, err := parseHTMLLatin1(body)
	if err != nil {
		return nil, err
	}
	scope := findHTMLElementByID(doc, "content_overview")
	if scope == nil {
		// PATCH: Prefer the editions block; fall back for legacy fixtures/pages without it.
		fmt.Fprintln(os.Stderr, "warning: editions parser content_overview block not found; scanning full document")
		scope = doc
	}
	var out []editionRow
	seen := map[int]bool{}
	walkHTML(scope, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode || !strings.EqualFold(n.Data, "a") {
			return
		}
		link := absoluteBluRayURL(attrValue(n, "href"))
		kind, slug, id, ok := parseReleaseURL(link)
		if !ok || seen[id] {
			return
		}
		text := cleanHTMLText(nodeText(n))
		if text == "" || strings.EqualFold(text, "details") {
			text = titleFromSlug(slug)
		}
		row := editionRow{ID: id, Kind: kind, Title: text, URL: link}
		container := n.Parent
		for container != nil && container.Type == xhtml.ElementNode && !strings.EqualFold(container.Data, "tr") {
			container = container.Parent
		}
		if container != nil {
			row.Country = parseCountryText(nodeText(container))
			prices := priceRE.FindAllStringSubmatch(nodeText(container), -1)
			if len(prices) > 0 {
				row.CurrentPrice, _ = strconv.ParseFloat(normalizePriceMatch(prices[0][1]), 64)
			}
			if len(prices) > 1 {
				row.ListPrice, _ = strconv.ParseFloat(normalizePriceMatch(prices[1][1]), 64)
			}
		}
		if country != "" && !strings.EqualFold(row.Country, country) {
			return
		}
		seen[id] = true
		out = append(out, row)
	})
	return out, nil
}

func findHTMLElementByID(root *xhtml.Node, id string) *xhtml.Node {
	var found *xhtml.Node
	walkHTML(root, func(n *xhtml.Node) {
		if found != nil || n.Type != xhtml.ElementNode {
			return
		}
		if attrValue(n, "id") == id {
			found = n
		}
	})
	return found
}

func parseCountryText(text string) string {
	for _, c := range []string{"US", "UK", "CA", "DE", "FR", "IT", "ES", "AU", "JP"} {
		if strings.Contains(text, c) {
			return c
		}
	}
	return ""
}
