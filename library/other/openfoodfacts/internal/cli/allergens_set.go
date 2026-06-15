// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: store a personal allergen profile.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelAllergensSetCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "set <allergen1,allergen2,...>",
		Short:       "Store your personal allergen profile (used by `allergens check`).",
		Long:        "Save a comma-separated list of allergens to match against. `allergens check <code>` then flags any product whose allergens or traces include one of these and exits non-zero.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli allergens set milk,gluten,nuts\n  openfoodfacts-pp-cli allergens check 3017620422003", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("allergen list is required\nUsage: %s <allergen1,allergen2,...>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			profile := normalizeAllergenList(args[0])

			db, err := openLocalStore(cmd.Context(), flagDB)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			if err := savePref(cmd.Context(), db.DB(), "allergens", strings.Join(profile, ",")); err != nil {
				return fmt.Errorf("saving allergen profile: %w", err)
			}
			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"allergens": profile}, flags)
			}
			if len(profile) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "allergen profile cleared")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "allergen profile set: %s\n", strings.Join(profile, ", "))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

// normalizeAllergenList splits, lowercases, trims, dedupes and sorts a CSV list.
func normalizeAllergenList(csv string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.Split(csv, ",") {
		p := strings.TrimSpace(strings.ToLower(part))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func savePref(ctx context.Context, db *sql.DB, key, val string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO pref_kv (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, key, val)
	return err
}

func loadPref(ctx context.Context, db *sql.DB, key string) (string, error) {
	var v string
	switch err := db.QueryRowContext(ctx, `SELECT v FROM pref_kv WHERE k=?`, key).Scan(&v); err {
	case sql.ErrNoRows:
		return "", nil
	case nil:
		return v, nil
	default:
		return "", err
	}
}
