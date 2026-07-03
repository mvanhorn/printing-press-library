// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cotar — full quotation from a live search, classified locally
// and rendered as a shareable Markdown.
// pp:data-source auto

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/mlextract"
	"github.com/spf13/cobra"
)

// cotacaoRow is one candidate line of the quotation. It is the pure input to
// renderCotacao, so classification/rendering is testable without any fetch.
type cotacaoRow struct {
	CatalogID       string  `json:"catalog_id"`
	Name            string  `json:"name"`
	Brand           string  `json:"brand"`
	Seller          string  `json:"seller"`
	Price           float64 `json:"price"`
	Currency        string  `json:"currency"`
	RatingValue     float64 `json:"rating_value"`
	DeliveryMinDays int     `json:"delivery_min_days"`
	DeliveryMaxDays int     `json:"delivery_max_days"`
	HasDelivery     bool    `json:"has_delivery"`
	URL             string  `json:"url"`
}

// cotacaoOpts carries the render-time knobs. Stamp/Data are injected so tests
// never depend on time.Now().
type cotacaoOpts struct {
	Termo    string
	Data     string   // human-facing timestamp (e.g. "2026-07-02 15:04")
	Currency string   // display currency label
	GroupBy  []string // parsed --por tokens; first is the primary grouping
	Top      int      // max rows per group (0 = all)
}

const cotarFooter = "Somente leitura — consulta de preços; não efetua compra."

var cotarGroupKeys = map[string]bool{"preco": true, "marca": true, "fornecedor": true, "prazo": true}

func newCotarCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit    int
		flagComPrazo bool
		flagPor      string
		flagOut      string
		flagTop      int
	)

	cmd := &cobra.Command{
		Use:   "cotar <termo>",
		Short: "Cotação completa: busca, classifica por preço/marca/fornecedor/prazo e gera um Markdown compartilhável",
		Long: "Faz uma busca no Mercado Livre pelo termo, coleta os anúncios (preço, marca, " +
			"fornecedor, avaliação), opcionalmente consulta o prazo de entrega de cada candidato, " +
			"classifica localmente conforme --por e grava uma cotação em Markdown pronta para " +
			"compartilhar. Consulta preços ao vivo e persiste no store local; nunca efetua compra.",
		Example: "  mercadolivre-pp-cli cotar furadeira --limit 20 --por marca,fornecedor --com-prazo",
		// Reads live then writes the store, but the classification is local:
		// data-source is auto so the resolver may fall back to local data.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			termo := strings.TrimSpace(args[0])
			groupBy, err := parseCotarPor(flagPor)
			if err != nil {
				return usageErr(err)
			}

			// 1. Live search -> candidates with price/brand/seller/rating/url.
			listings, prov, err := fetchCotarListings(cmd, flags, termo, flagLimit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flagLimit > 0 && len(listings) > flagLimit {
				listings = listings[:flagLimit]
			}

			// 2. Persist listings (+seller) and a price snapshot per candidate.
			if err := persistListings(cmd.Context(), listings); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: não foi possível persistir anúncios: %v\n", err)
			}

			// 3. Optionally enrich each candidate with its delivery window.
			delivery := map[string]mlextract.ProductDetail{}
			if flagComPrazo {
				for _, l := range listings {
					if l.CatalogID == "" {
						continue
					}
					pd, ok := fetchCotarDelivery(cmd, flags, l.CatalogID)
					if !ok {
						// Honest blank: leave delivery unset, never fabricate.
						continue
					}
					delivery[l.CatalogID] = pd
				}
			}

			// 4. Build the pure rows.
			currency := "BRL"
			rows := make([]cotacaoRow, 0, len(listings))
			for _, l := range listings {
				if l.Currency != "" {
					currency = l.Currency
				}
				// The search polycard carries seller but no separate brand
				// field; for tool/equipment listings the seller is effectively
				// the brand (EINHELL, DEWALT, BOSCH), so fall back to it when
				// brand is empty rather than emit an empty "Sem marca" group.
				brand := l.Brand
				if brand == "" {
					brand = l.Seller
				}
				row := cotacaoRow{
					CatalogID:   l.CatalogID,
					Name:        l.Name,
					Brand:       brand,
					Seller:      l.Seller,
					Price:       l.Price,
					Currency:    l.Currency,
					RatingValue: l.RatingValue,
					URL:         l.URL,
				}
				if pd, ok := delivery[l.CatalogID]; ok {
					row.DeliveryMinDays = pd.DeliveryMinDays
					row.DeliveryMaxDays = pd.DeliveryMaxDays
					row.HasDelivery = true
				}
				rows = append(rows, row)
			}

			now := time.Now()
			stamp := now.Format("20060102-150405")
			opts := cotacaoOpts{
				Termo:    termo,
				Data:     now.Format("2006-01-02 15:04"),
				Currency: currency,
				GroupBy:  groupBy,
				Top:      flagTop,
			}
			md := renderCotacao(rows, opts)

			// 5. Always save the .md file.
			outPath, err := resolveCotarOutPath(flagOut, termo, stamp)
			if err != nil {
				return err
			}
			if err := writeCotarFile(outPath, md); err != nil {
				return err
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(rows), prov)
			}

			// Machine consumers: structured JSON envelope (still saved the .md).
			if flags.asJSON || flags.agent {
				fmt.Fprintf(cmd.ErrOrStderr(), "cotação salva em %s\n", outPath)
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"file":       outPath,
					"termo":      termo,
					"count":      len(rows),
					"currency":   currency,
					"candidates": rows,
				}, flags)
			}

			// Human/pipe consumers: clean Markdown on stdout, path on stderr.
			fmt.Fprint(cmd.OutOrStdout(), md)
			fmt.Fprintf(cmd.ErrOrStderr(), "cotação salva em %s\n", outPath)
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Quantos anúncios considerar")
	cmd.Flags().BoolVar(&flagComPrazo, "com-prazo", false, "Consulta a ficha de cada candidato p/ obter o prazo de entrega (mais lento; N buscas)")
	cmd.Flags().StringVar(&flagPor, "por", "preco", "Agrupamento primário: preco|marca|fornecedor|prazo (aceita lista separada por vírgula)")
	cmd.Flags().StringVar(&flagOut, "out", "", "Caminho do arquivo .md (vazio = $MERCADOLIVRE_DATA_DIR/cotacoes/<slug>-<stamp>.md)")
	cmd.Flags().IntVar(&flagTop, "top", 0, "Limita linhas por grupo (0 = todas)")
	return cmd
}

// parseCotarPor validates and splits the --por comma list. Defaults to ["preco"].
func parseCotarPor(raw string) ([]string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return []string{"preco"}, nil
	}
	var out []string
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if !cotarGroupKeys[tok] {
			return nil, fmt.Errorf("--por %q: esperado preco, marca, fornecedor ou prazo", tok)
		}
		out = append(out, tok)
	}
	if len(out) == 0 {
		return []string{"preco"}, nil
	}
	return out, nil
}

// renderCotacao classifies and renders the candidates as Markdown. Pure: no
// fetch, no store, no time.Now — everything comes from rows + opts.
func renderCotacao(rows []cotacaoRow, opts cotacaoOpts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Cotação: %s\n\n", opts.Termo)
	fmt.Fprintf(&b, "Data: %s · Candidatos: %d · Moeda: %s\n\n", opts.Data, len(rows), opts.Currency)

	groupBy := opts.GroupBy
	if len(groupBy) == 0 {
		groupBy = []string{"preco"}
	}
	primary := groupBy[0]

	switch primary {
	case "preco":
		renderCotarTable(&b, sortByPrice(rows), opts.Top)
	case "prazo":
		renderCotarByPrazo(&b, rows, opts.Top)
	default: // marca | fornecedor (categorical, possibly nested)
		renderCotarGrouped(&b, rows, groupBy, opts.Top)
	}

	fmt.Fprintf(&b, "\n%s\n", cotarFooter)
	return b.String()
}

// renderCotarGrouped renders one subsection per value of groupBy[0], sorted by
// group label; within each group it recurses on the remaining keys, or renders
// a price-ascending table when no keys remain.
func renderCotarGrouped(b *strings.Builder, rows []cotacaoRow, groupBy []string, top int) {
	key := groupBy[0]
	groups := map[string][]cotacaoRow{}
	for _, r := range rows {
		groups[groupValue(r, key)] = append(groups[groupValue(r, key)], r)
	}
	labels := make([]string, 0, len(groups))
	for l := range groups {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	for _, label := range labels {
		fmt.Fprintf(b, "## %s\n\n", label)
		rest := groupBy[1:]
		if len(rest) > 0 {
			renderCotarGrouped(b, groups[label], rest, top)
		} else {
			renderCotarTable(b, sortByPrice(groups[label]), top)
		}
		b.WriteString("\n")
	}
}

// renderCotarByPrazo sorts candidates ascending by max delivery days; those
// without a consulted delivery window are collected into a trailing section.
func renderCotarByPrazo(b *strings.Builder, rows []cotacaoRow, top int) {
	var withPrazo, withoutPrazo []cotacaoRow
	for _, r := range rows {
		if r.HasDelivery && r.DeliveryMaxDays > 0 {
			withPrazo = append(withPrazo, r)
		} else {
			withoutPrazo = append(withoutPrazo, r)
		}
	}
	sort.SliceStable(withPrazo, func(i, j int) bool {
		if withPrazo[i].DeliveryMaxDays != withPrazo[j].DeliveryMaxDays {
			return withPrazo[i].DeliveryMaxDays < withPrazo[j].DeliveryMaxDays
		}
		return withPrazo[i].Price < withPrazo[j].Price
	})
	renderCotarTable(b, withPrazo, top)
	if len(withoutPrazo) > 0 {
		b.WriteString("\n## prazo não consultado\n\n")
		renderCotarTable(b, sortByPrice(withoutPrazo), top)
	}
}

// renderCotarTable writes a Markdown table for the given (already-sorted) rows.
func renderCotarTable(b *strings.Builder, rows []cotacaoRow, top int) {
	if top > 0 && len(rows) > top {
		rows = rows[:top]
	}
	b.WriteString("| Produto | Marca | Fornecedor | Preço | Prazo | Avaliação | Link |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s |\n",
			mdCell(orDash(r.Name)),
			mdCell(orDash(r.Brand)),
			mdCell(orDash(r.Seller)),
			mdCell(formatCotarPrice(r.Price, r.Currency)),
			mdCell(formatPrazo(r)),
			mdCell(formatRating(r.RatingValue)),
			mdCell(orDash(r.CatalogID)),
		)
	}
}

// sortByPrice returns a price-ascending copy of rows (stable, cheapest first).
func sortByPrice(rows []cotacaoRow) []cotacaoRow {
	out := make([]cotacaoRow, len(rows))
	copy(out, rows)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

func groupValue(r cotacaoRow, key string) string {
	switch key {
	case "marca":
		if strings.TrimSpace(r.Brand) == "" {
			return "Sem marca"
		}
		return r.Brand
	case "fornecedor":
		if strings.TrimSpace(r.Seller) == "" {
			return "Sem fornecedor"
		}
		return r.Seller
	default:
		return ""
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// formatPrazo renders the delivery window as "2–5 dias", "5 dias", or "—".
func formatPrazo(r cotacaoRow) string {
	if !r.HasDelivery || r.DeliveryMaxDays <= 0 {
		return "—"
	}
	if r.DeliveryMinDays > 0 && r.DeliveryMinDays != r.DeliveryMaxDays {
		return fmt.Sprintf("%d–%d dias", r.DeliveryMinDays, r.DeliveryMaxDays)
	}
	return fmt.Sprintf("%d dias", r.DeliveryMaxDays)
}

func formatRating(v float64) string {
	if v <= 0 {
		return "—"
	}
	return strings.ReplaceAll(strconv.FormatFloat(v, 'f', 1, 64), ".", ",")
}

// formatCotarPrice renders a price as pt-BR currency, e.g. "R$ 1.349,90".
func formatCotarPrice(p float64, currency string) string {
	if p <= 0 {
		return "—"
	}
	symbol := "R$"
	if currency != "" && currency != "BRL" {
		symbol = currency
	}
	s := strconv.FormatFloat(p, 'f', 2, 64) // "1349.90"
	intPart, decPart := s, "00"
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, decPart = s[:i], s[i+1:]
	}
	// Thousands separators (dots) on the integer part.
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")
	var grouped strings.Builder
	for i, digit := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			grouped.WriteByte('.')
		}
		grouped.WriteRune(digit)
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s %s%s,%s", symbol, sign, grouped.String(), decPart)
}

// resolveCotarOutPath returns the explicit --out path, or derives
// <dataDir>/cotacoes/<slug>-<stamp>.md when --out is empty.
func resolveCotarOutPath(out, termo, stamp string) (string, error) {
	if strings.TrimSpace(out) != "" {
		return out, nil
	}
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	slug := slugifyQuery(termo)
	if slug == "" {
		slug = "cotacao"
	}
	return filepath.Join(dir, "cotacoes", fmt.Sprintf("%s-%s.md", slug, stamp)), nil
}

// writeCotarFile creates the parent directory and writes the Markdown file.
func writeCotarFile(path, md string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(md), 0o644)
}

// fetchCotarListings runs the live search and parses the page into listings
// (JSON-LD + DOM seller). Mirrors the listings command's fetch path.
func fetchCotarListings(cmd *cobra.Command, flags *rootFlags, termo string, limit int) ([]mlextract.Listing, DataProvenance, error) {
	client, err := flags.newClient()
	if err != nil {
		return nil, DataProvenance{}, err
	}
	path := replacePathParam("https://lista.mercadolivre.com.br/{query}", "query", slugifyQuery(termo))
	params := map[string]string{}
	if limit != 0 {
		params["limit"] = formatCLIParamValue(limit)
	}
	data, prov, err := resolveReadWithStrategyResponsePathAndJSONGuard(
		cmd.Context(), client, flags, "auto", "listings", false, path, params, nil, "", false, cmd.ErrOrStderr())
	if err != nil {
		return nil, prov, err
	}
	listings, err := mlextract.ParseSearchListings(data)
	if err != nil {
		return nil, prov, err
	}
	return listings, prov, nil
}

// fetchCotarDelivery fetches one product page and returns its delivery window.
// ok=false on any fetch/parse failure so the caller leaves the prazo blank.
func fetchCotarDelivery(cmd *cobra.Command, flags *rootFlags, catalogID string) (mlextract.ProductDetail, bool) {
	client, err := flags.newClient()
	if err != nil {
		return mlextract.ProductDetail{}, false
	}
	path := replacePathParam("https://www.mercadolivre.com.br/p/{catalog_id}", "catalog_id", catalogID)
	data, _, err := resolveReadWithStrategyResponsePathAndJSONGuard(
		cmd.Context(), client, flags, "auto", "products", false, path, map[string]string{}, nil, "", false, cmd.ErrOrStderr())
	if err != nil {
		return mlextract.ProductDetail{}, false
	}
	ldjson, err := extractHTMLResponse(data, htmlExtractionOptions{
		Mode:           "embedded-json",
		BaseURL:        htmlExtractionRequestURL(client.BaseURL, path, map[string]string{}),
		ContentType:    client.LastContentType(),
		LinkPrefixes:   []string{},
		Limit:          0,
		ScriptSelector: "script[type=\"application/ld+json\"]",
		JSONPath:       "",
	})
	if err != nil {
		return mlextract.ProductDetail{}, false
	}
	product, err := mlextract.ParseProduct(ldjson)
	if err != nil || product == nil {
		return mlextract.ProductDetail{}, false
	}
	if err := persistProduct(cmd.Context(), product, nil); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: não foi possível persistir prazo de %s: %v\n", catalogID, err)
	}
	return *product, true
}
