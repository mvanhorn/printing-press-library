// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: side-by-side product comparison.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type compareRow struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Kcal100g   float64 `json:"kcal_100g"`
	Protein    float64 `json:"protein_g_100g"`
	Fat        float64 `json:"fat_g_100g"`
	Sugars     float64 `json:"sugars_g_100g"`
	SatFat     float64 `json:"saturated_fat_g_100g"`
	Salt       float64 `json:"salt_g_100g"`
	NutriScore string  `json:"nutriscore_grade"`
	Nova       float64 `json:"nova_group"`
	EcoScore   string  `json:"ecoscore_grade"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare <code1> <code2> [code3...]",
		Short:       "Put two or more products side-by-side on per-100g nutriments, Nutri-Score, NOVA, Eco-Score, and sugar.",
		Long:        "Fetch two or more products by barcode and render them side-by-side on per-100g nutriments and the Nutri-Score / NOVA / Eco-Score health markers. Use it for store-brand vs name-brand or any 'which is healthier' decision.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli compare 3017620422003 7622210449283\n  openfoodfacts-pp-cli compare 3017620422003 7622210449283 --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return usageErr(fmt.Errorf("need at least two barcodes\nUsage: %s <code1> <code2> [code3...]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows := make([]compareRow, 0, len(args))
			for _, code := range args {
				prod, found, err := fetchProduct(cmd.Context(), c, code)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if !found {
					rows = append(rows, compareRow{Code: code, Name: "(not found)"})
					continue
				}
				n := nutrFromObject(prod)
				rows = append(rows, compareRow{
					Code:       code,
					Name:       prodString(prod, "product_name"),
					Kcal100g:   n.Kcal,
					Protein:    n.Protein,
					Fat:        n.Fat,
					Sugars:     n.Sugars,
					SatFat:     n.SatFat,
					Salt:       n.Salt,
					NutriScore: prodString(prod, "nutriscore_grade"),
					Nova:       prodFloat(prod, "nova_group"),
					EcoScore:   prodString(prod, "ecoscore_grade"),
				})
			}
			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tNAME\tKCAL/100g\tPROT\tFAT\tSUGAR\tSAT-FAT\tSALT\tNUTRI\tNOVA\tECO")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%.0f\t%.1f\t%.1f\t%.1f\t%.1f\t%.2f\t%s\t%s\t%s\n",
					r.Code, truncate(r.Name, 28), r.Kcal100g, r.Protein, r.Fat, r.Sugars, r.SatFat, r.Salt,
					strings.ToUpper(grade(r.NutriScore)), nova(r.Nova), strings.ToUpper(grade(r.EcoScore)))
			}
			return tw.Flush()
		},
	}
	return cmd
}

// grade returns a placeholder for empty/unknown grade values.
func grade(g string) string {
	g = strings.TrimSpace(g)
	if g == "" || g == "unknown" || g == "not-applicable" {
		return "-"
	}
	return g
}

// nova renders a NOVA group number, blanking zero (absent).
func nova(v float64) string {
	if v <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", int(v))
}
