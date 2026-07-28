// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

type auditRule struct {
	Rule        string   `json:"rule"`
	Description string   `json:"description"`
	Count       int      `json:"count"`
	ProductIDs  []string `json:"product_ids"`
}

type catalogAuditView struct {
	ScannedProducts   int         `json:"scanned_products"`
	ScannedVariations int         `json:"scanned_variations"`
	Rules             []auditRule `json:"rules"`
	TotalFindings     int         `json:"total_findings"`
	Note              string      `json:"note,omitempty"`
}

// auditProduct is one drained product row.
type auditProduct struct {
	ID            string
	Name          string
	SKU           string
	Type          string
	Status        string
	Price         string
	StockStatus   string
	HasImages     bool
	HasCategories bool
	Description   string
	Short         string
}

type auditProductData struct {
	Images      []json.RawMessage `json:"images"`
	Categories  []json.RawMessage `json:"categories"`
	Description string            `json:"description"`
	Short       string            `json:"short_description"`
}

var auditRuleDescriptions = map[string]string{
	"missing_sku":                 "Products with no SKU set",
	"missing_image":               "Products with no images attached",
	"empty_description":           "Products with neither a description nor a short description",
	"zero_price":                  "Published products with no usable price",
	"uncategorized":               "Products not assigned to any category",
	"out_of_stock_published":      "Published products that are not in stock",
	"variable_without_variations": "Variable products with no variations synced",
	"variation_missing_price":     "Variations with no usable price",
}

func auditRuleNames() []string {
	names := make([]string, 0, len(auditRuleDescriptions))
	for k := range auditRuleDescriptions {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func newNovelCatalogAuditCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagRule string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Scan the synced catalog for publishable defects",
		Long: "Runs a deterministic rule scan over synced products and variations and returns the offending\n" +
			"product IDs. There is no defect endpoint in the API, so this is a local rule engine — the\n" +
			"output is a list of IDs to fix, not prose.\n\n" +
			"Use this command to find defects in the catalog of the store you own.\n" +
			"Do NOT use this command to track another store's catalog over time; use 'catalog watch' instead.\n" +
			"Do NOT use it to compare two catalog snapshots; use 'catalog diff' instead.",
		Example: "  woocommerce-pp-cli catalog audit --agent --select rule,count",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit the synced catalog from the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "catalog audit"); err != nil {
				return err
			}
			if flagRule != "" {
				if _, ok := auditRuleDescriptions[flagRule]; !ok {
					return usageErr(fmt.Errorf("unknown --rule %q; valid rules are: %s", flagRule, strings.Join(auditRuleNames(), ", ")))
				}
			}
			if flagLimit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = resolveDBPath(dbPath)
			if localStoreMissing(cmd, flags, dbPath, "products,variations") {
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "products") {
				hintIfStale(cmd, db, "products", flags.maxAge)
			}

			// Stage 1: drain products fully.
			products, err := loadAuditProducts(ctx, db)
			if err != nil {
				return err
			}
			// Stage 2: safe to query variations now.
			varsByParent, unpricedVariations, scannedVariations, err := loadAuditVariations(ctx, db)
			variationsMissing := errors.Is(err, errVariationsUnavailable)
			if err != nil && !variationsMissing {
				return err
			}

			// Without variations synced, "variable product has no variations"
			// would fire on every variable product in the catalog.
			view := buildCatalogAuditView(products, varsByParent, unpricedVariations, scannedVariations, flagRule, flagLimit)
			if variationsMissing {
				view.Rules = dropVariationRules(view.Rules)
				view.Note = "variations are not synced, so variation rules were skipped; run 'woocommerce-pp-cli sync --resources variations' to include them"
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum product IDs listed per rule")
	cmd.Flags().StringVar(&flagRule, "rule", "", "Run a single rule instead of all of them")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// dropVariationRules removes rules that cannot be evaluated without the
// variations table, so an unsynced mirror does not masquerade as findings.
func dropVariationRules(rules []auditRule) []auditRule {
	out := make([]auditRule, 0, len(rules))
	for _, r := range rules {
		if r.Rule == "variable_without_variations" || r.Rule == "variation_missing_price" {
			continue
		}
		out = append(out, r)
	}
	return out
}

func loadAuditProducts(ctx context.Context, db *store.Store) ([]auditProduct, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, COALESCE(name,''), COALESCE(sku,''), COALESCE(type,''), COALESCE(status,''),
		       COALESCE(price,''), COALESCE(stock_status,''), COALESCE(data,'')
		FROM products`)
	if err != nil {
		return nil, fmt.Errorf("querying products: %w", err)
	}
	out := make([]auditProduct, 0)
	for rows.Next() {
		var p auditProduct
		var raw string
		if err := rows.Scan(&p.ID, &p.Name, &p.SKU, &p.Type, &p.Status, &p.Price, &p.StockStatus, &raw); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning product: %w", err)
		}
		if raw != "" {
			var d auditProductData
			if err := json.Unmarshal([]byte(raw), &d); err == nil {
				p.HasImages = len(d.Images) > 0
				p.HasCategories = len(d.Categories) > 0
				p.Description = cliutil.CleanText(d.Description)
				p.Short = cliutil.CleanText(d.Short)
			}
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating products: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing product rows: %w", err)
	}
	return out, nil
}

func loadAuditVariations(ctx context.Context, db *store.Store) (map[string]int, []string, int, error) {
	byParent := make(map[string]int)
	unpriced := make([]string, 0)
	scanned := 0

	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, COALESCE(parent_id,''), json_extract(data,'$.price') FROM variations`)
	if err != nil {
		return byParent, unpriced, 0, errVariationsUnavailable
	}
	for rows.Next() {
		var id, parent string
		var price sql.NullString
		if err := rows.Scan(&id, &parent, &price); err != nil {
			_ = rows.Close()
			return byParent, unpriced, scanned, fmt.Errorf("scanning variation: %w", err)
		}
		scanned++
		byParent[parent]++
		if !price.Valid || parseMoney(price.String) == 0 {
			unpriced = append(unpriced, id)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return byParent, unpriced, scanned, fmt.Errorf("iterating variations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return byParent, unpriced, scanned, fmt.Errorf("closing variation rows: %w", err)
	}
	return byParent, unpriced, scanned, nil
}

// buildCatalogAuditView is pure so the rule logic stays unit-testable.
func buildCatalogAuditView(products []auditProduct, varsByParent map[string]int,
	unpricedVariations []string, scannedVariations int, only string, limit int) catalogAuditView {

	hits := make(map[string][]string)
	add := func(rule, id string) { hits[rule] = append(hits[rule], id) }

	for _, p := range products {
		if p.SKU == "" {
			add("missing_sku", p.ID)
		}
		if !p.HasImages {
			add("missing_image", p.ID)
		}
		if p.Description == "" && p.Short == "" {
			add("empty_description", p.ID)
		}
		if p.Status == "publish" && parseMoney(p.Price) == 0 {
			add("zero_price", p.ID)
		}
		if !p.HasCategories {
			add("uncategorized", p.ID)
		}
		if p.Status == "publish" && p.StockStatus != "" && p.StockStatus != "instock" {
			add("out_of_stock_published", p.ID)
		}
		if p.Type == "variable" && varsByParent[p.ID] == 0 {
			add("variable_without_variations", p.ID)
		}
	}
	hits["variation_missing_price"] = append(hits["variation_missing_price"], unpricedVariations...)

	rules := make([]auditRule, 0, len(auditRuleDescriptions))
	total := 0
	for _, name := range auditRuleNames() {
		if only != "" && only != name {
			continue
		}
		ids := hits[name]
		total += len(ids)
		capped := make([]string, 0, len(ids))
		capped = append(capped, ids...)
		if limit > 0 && len(capped) > limit {
			capped = capped[:limit]
		}
		rules = append(rules, auditRule{
			Rule:        name,
			Description: auditRuleDescriptions[name],
			Count:       len(ids),
			ProductIDs:  capped,
		})
	}
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Count != rules[j].Count {
			return rules[i].Count > rules[j].Count
		}
		return rules[i].Rule < rules[j].Rule
	})

	view := catalogAuditView{
		ScannedProducts:   len(products),
		ScannedVariations: scannedVariations,
		Rules:             rules,
		TotalFindings:     total,
	}
	if len(products) == 0 {
		view.Note = "no products in the local mirror; run 'woocommerce-pp-cli sync --resources products,variations' first"
	}
	return view
}
