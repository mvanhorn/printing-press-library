// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: check a product against the stored allergen profile.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type allergenCheckResult struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Profile   []string `json:"profile"`
	Allergens []string `json:"product_allergens"`
	Traces    []string `json:"product_traces"`
	Hits      []string `json:"hits"`
	Safe      bool     `json:"safe"`
}

func newNovelAllergensCheckCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "check <code>",
		Short:       "Check a product against your allergen profile; exit non-zero on a match.",
		Long:        "Fetch a product by barcode and intersect its allergens and traces with the profile set by `allergens set`. Prints any matches and exits with code 3 when the product is unsafe, so it works as a scriptable buy/skip gate.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli allergens check 3017620422003\n  openfoodfacts-pp-cli allergens check 3017620422003 --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("barcode is required\nUsage: %s <code>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			code := args[0]

			db, err := openLocalStore(cmd.Context(), flagDB)
			if err != nil {
				return configErr(err)
			}
			profileRaw, err := loadPref(cmd.Context(), db.DB(), "allergens")
			db.Close()
			if err != nil {
				return err
			}
			profile := normalizeAllergenList(profileRaw)
			if len(profile) == 0 {
				return configErr(fmt.Errorf("no allergen profile set — run `allergens set milk,gluten` first"))
			}

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

			allergens := prodTags(prod, "allergens_tags")
			traces := prodTags(prod, "traces_tags")
			hits := allergenHits(profile, allergens, traces)

			result := allergenCheckResult{
				Code:      code,
				Name:      prodString(prod, "product_name"),
				Profile:   profile,
				Allergens: allergens,
				Traces:    traces,
				Hits:      hits,
				Safe:      len(hits) == 0,
			}

			if wantsJSONOut(cmd, flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
			} else if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "OK: %s contains none of your allergens (%s)\n",
					displayName(result.Name, code), strings.Join(profile, ", "))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "ALLERGEN MATCH: %s contains %s\n",
					displayName(result.Name, code), strings.Join(hits, ", "))
			}

			if len(hits) > 0 {
				return &cliError{code: 3, err: fmt.Errorf("allergen match: %s", strings.Join(hits, ", "))}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

// allergenHits returns profile terms that match any product allergen or trace.
func allergenHits(profile, allergens, traces []string) []string {
	present := append(append([]string{}, allergens...), traces...)
	seen := map[string]bool{}
	hits := []string{}
	for _, term := range profile {
		for _, p := range present {
			if p == term || strings.Contains(p, term) || strings.Contains(term, p) {
				if !seen[term] {
					seen[term] = true
					hits = append(hits, term)
				}
				break
			}
		}
	}
	sort.Strings(hits)
	return hits
}
