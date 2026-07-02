// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: assemble a purchase-request (cotacao) doc from the LOCAL store.
// pp:data-source local

package cli

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// cotacaoItem is one product line of the quotation.
type cotacaoItem struct {
	CatalogID  string            `json:"catalog_id"`
	Name       string            `json:"name"`
	Brand      string            `json:"brand"`
	Price      float64           `json:"price"`
	Currency   string            `json:"currency"`
	CapturedAt string            `json:"captured_at"`
	Specs      map[string]string `json:"specs"`
}

const cotacaoMaxSpecs = 5

func newNovelCotacaoCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string

	cmd := &cobra.Command{
		Use:   "cotacao <catalog_id> [catalog_id...]",
		Short: "Assemble a purchase-request-shaped quotation across chosen products with price, seller, key specs",
		Long: "Build a purchase-request (cotacao) document for one or more locally-stored products: " +
			"name, brand, latest observed price + capture time, and a few key technical specs. " +
			"Renders markdown (default) or CSV; --json emits the structured document.",
		Example:     "  mercadolivre-pp-cli cotacao MLB51764304 MLB40287816 --format md",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			format := strings.ToLower(strings.TrimSpace(flagFormat))
			if format == "" {
				format = "md"
			}
			if format != "md" && format != "csv" {
				return usageErr(fmt.Errorf("--format %q: expected md or csv", flagFormat))
			}

			s, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			var items []cotacaoItem
			for _, id := range args {
				item := cotacaoItem{CatalogID: id, Specs: map[string]string{}}
				if pb, ok, err := s.GetProductBasic(id); err != nil {
					return err
				} else if ok {
					item.Name = pb.Name
					item.Brand = pb.Brand
					item.Price = pb.Price
					item.Currency = pb.Currency
				}
				// Prefer the freshest observed price + its capture time.
				if snap, ok, err := s.LatestPriceSnapshot(id); err != nil {
					return err
				} else if ok {
					item.Price = snap.Price
					if snap.Currency != "" {
						item.Currency = snap.Currency
					}
					if !snap.CapturedAt.IsZero() {
						item.CapturedAt = snap.CapturedAt.UTC().Format(time.RFC3339)
					}
				}
				attrs, err := s.ProductAttributes(id)
				if err != nil {
					return err
				}
				for _, name := range topSpecNames(attrs, cotacaoMaxSpecs) {
					item.Specs[name] = attrs[name]
				}
				items = append(items, item)
			}

			// Structured output for agents/JSON consumers.
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), items, flags)
			}
			if format == "csv" {
				return writeCotacaoCSV(cmd, items)
			}
			return writeCotacaoMarkdown(cmd, items)
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "md", "Output format: md (markdown table, default) or csv")
	return cmd
}

// topSpecNames returns up to max attribute names (alphabetical) for the doc.
func topSpecNames(attrs map[string]string, max int) []string {
	names := make([]string, 0, len(attrs))
	for n := range attrs {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > max {
		names = names[:max]
	}
	return names
}

func formatPrice(p float64) string {
	if p <= 0 {
		return ""
	}
	return strconv.FormatFloat(p, 'f', 2, 64)
}

func specsInline(item cotacaoItem) string {
	names := make([]string, 0, len(item.Specs))
	for n := range item.Specs {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, n+": "+item.Specs[n])
	}
	return strings.Join(parts, "; ")
}

func writeCotacaoMarkdown(cmd *cobra.Command, items []cotacaoItem) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "# Cotação")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "| Catalog ID | Produto | Marca | Preço | Moeda | Capturado em | Specs |")
	fmt.Fprintln(out, "|---|---|---|---|---|---|---|")
	for _, it := range items {
		fmt.Fprintf(out, "| %s | %s | %s | %s | %s | %s | %s |\n",
			mdCell(it.CatalogID), mdCell(it.Name), mdCell(it.Brand),
			mdCell(formatPrice(it.Price)), mdCell(it.Currency),
			mdCell(it.CapturedAt), mdCell(specsInline(it)))
	}
	return nil
}

// mdCell escapes pipe characters so a spec value never breaks the table.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}

func writeCotacaoCSV(cmd *cobra.Command, items []cotacaoItem) error {
	w := csv.NewWriter(cmd.OutOrStdout())
	defer w.Flush()
	if err := w.Write([]string{"catalog_id", "name", "brand", "price", "currency", "captured_at", "specs"}); err != nil {
		return err
	}
	for _, it := range items {
		if err := w.Write([]string{
			it.CatalogID, it.Name, it.Brand, formatPrice(it.Price),
			it.Currency, it.CapturedAt, specsInline(it),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
