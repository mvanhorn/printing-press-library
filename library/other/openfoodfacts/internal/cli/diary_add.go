// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: log a food to the local macro diary.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type diaryEntry struct {
	ID         int64      `json:"id"`
	Day        string     `json:"day"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Grams      float64    `json:"grams"`
	Nutriments nutriments `json:"nutriments"`
}

func newNovelDiaryAddCmd(flags *rootFlags) *cobra.Command {
	var flagServings string
	var flagGrams string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "add <code>",
		Short:       "Log a food by barcode to the local diary, scaled by servings or grams.",
		Long:        "Fetch a product by barcode, scale its per-100g nutriments by the amount eaten, and append the entry to today's local diary. Amount is --grams if given; otherwise --servings × the product's serving size (or × 100g when no serving size is known).",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli diary add 3017620422003 --grams 30\n  openfoodfacts-pp-cli diary add 3017620422003 --servings 1.5", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("barcode is required\nUsage: %s <code> [--grams G | --servings N]", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			code := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			prod, found, err := fetchProduct(cmd.Context(), c, code)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !found {
				return notFoundErr(fmt.Errorf("no product found for barcode %s", code))
			}

			grams, err := resolveGrams(prod, flagGrams, flagServings, cmd.ErrOrStderr())
			if err != nil {
				return usageErr(err)
			}
			n := nutrFromObject(prod).scaled(grams / 100.0)
			name := prodString(prod, "product_name")

			db, err := openLocalStore(cmd.Context(), flagDB)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			day := time.Now().Format("2006-01-02")
			res, err := db.DB().ExecContext(cmd.Context(),
				`INSERT INTO diary_entry (day, code, name, grams, kcal, protein, fat, satfat, carbs, sugars, fiber, salt, sodium, created_at)
				 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				day, code, name, grams, n.Kcal, n.Protein, n.Fat, n.SatFat, n.Carbs, n.Sugars, n.Fiber, n.Salt, n.Sodium,
				time.Now().UTC().Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("logging diary entry: %w", err)
			}
			id, _ := res.LastInsertId()
			entry := diaryEntry{ID: id, Day: day, Code: code, Name: name, Grams: grams, Nutriments: n}

			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entry, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged %s (%.0fg) to %s: %.0f kcal, %.1fg protein, %.1fg fat, %.1fg carbs\n",
				displayName(name, code), grams, day, n.Kcal, n.Protein, n.Fat, n.Carbs)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagServings, "servings", "", "Number of servings eaten (uses the product's serving size, or 100g if unknown)")
	cmd.Flags().StringVar(&flagGrams, "grams", "", "Exact grams eaten (overrides --servings)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

// resolveGrams turns --grams/--servings plus the product's serving size into a
// concrete gram amount. Defaults to 100g when nothing is specified.
func resolveGrams(prod map[string]any, gramsFlag, servingsFlag string, warn interface{ Write([]byte) (int, error) }) (float64, error) {
	if strings.TrimSpace(gramsFlag) != "" {
		g, err := parsePositiveFloat(gramsFlag, "grams")
		if err != nil {
			return 0, err
		}
		return g, nil
	}
	if strings.TrimSpace(servingsFlag) != "" {
		s, err := parsePositiveFloat(servingsFlag, "servings")
		if err != nil {
			return 0, err
		}
		if sg, ok := servingGrams(prod); ok {
			return s * sg, nil
		}
		fmt.Fprintf(warn, "note: no serving size known for this product; assuming 1 serving = 100g\n")
		return s * 100.0, nil
	}
	return 100.0, nil
}

func displayName(name, code string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return code
}
