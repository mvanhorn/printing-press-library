// `categories list` exposes the Craigslist category taxonomy (178 abbreviations
// like apa, sof, sss). Reads live from reference.craigslist.org by default and
// from the local cl_categories table when --data-source local is set.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"

	"github.com/spf13/cobra"
)

// categoryRow is the typed shape returned by `categories list`. We keep this
// stable across live and local sources so downstream agents do not branch on
// where the rows came from.
type categoryRow struct {
	Abbreviation string `json:"abbreviation"`
	CategoryID   int    `json:"categoryId"`
	Description  string `json:"description"`
	Type         string `json:"type"`
}

func newCategoriesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "categories",
		Short:       "Browse the Craigslist category taxonomy",
		Long:        "Reference taxonomy for category abbreviations (e.g. apa for apartments, sof for software jobs).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newCategoriesListCmd(flags))
	return cmd
}

func newCategoriesListCmd(flags *rootFlags) *cobra.Command {
	var typeFilter, grep string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the 178 Craigslist category abbreviations, optionally filtered by --type (H/J/S/B/C/G/E/R/L) or --grep",
		Long:        "List the 178 Craigslist category abbreviations. Filter by single-letter type (H/J/S/B/C/G/E/R/L) or substring match against the description.",
		Example:     "  craigslist-pp-cli categories list --type H\n  craigslist-pp-cli categories list --grep apartments",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows, err := loadCategories(cmd.Context(), flags)
			if err != nil {
				return err
			}
			rows = filterCategories(rows, typeFilter, grep)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					items = append(items, map[string]any{
						"abbreviation": r.Abbreviation,
						"type":         r.Type,
						"description":  r.Description,
					})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by single-letter category type (H/J/S/B/C/G/E/R/L)")
	cmd.Flags().StringVar(&grep, "grep", "", "Case-insensitive substring filter on description")
	return cmd
}

// loadCategories reads from the local store when --data-source local is set,
// otherwise hits reference.craigslist.org. Live results are not auto-persisted
// here (use `catalog refresh`).
func loadCategories(ctx context.Context, flags *rootFlags) ([]categoryRow, error) {
	if flags != nil && flags.dataSource == "local" {
		return loadCategoriesFromStore(ctx)
	}
	c := craigslist.New(1.0)
	cats, err := c.GetCategories(ctx)
	if err != nil {
		// Fall back to local on auto when the live call fails.
		if flags != nil && flags.dataSource == "auto" {
			if rows, lerr := loadCategoriesFromStore(ctx); lerr == nil && len(rows) > 0 {
				return rows, nil
			}
		}
		return nil, fmt.Errorf("fetch categories: %w", err)
	}
	out := make([]categoryRow, 0, len(cats))
	for _, c := range cats {
		out = append(out, categoryRow{
			Abbreviation: c.Abbreviation,
			CategoryID:   c.CategoryID,
			Description:  c.Description,
			Type:         c.Type,
		})
	}
	return out, nil
}

func loadCategoriesFromStore(ctx context.Context) ([]categoryRow, error) {
	db, err := openCLStore(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return queryCategoriesDB(ctx, db.DB())
}

func queryCategoriesDB(ctx context.Context, db *sql.DB) ([]categoryRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT category_id, abbreviation, COALESCE(description,''), COALESCE(type,'')
		FROM cl_categories
		ORDER BY abbreviation`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()
	var out []categoryRow
	for rows.Next() {
		var r categoryRow
		if err := rows.Scan(&r.CategoryID, &r.Abbreviation, &r.Description, &r.Type); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// filterCategories applies the --type and --grep filters in place. Empty
// arguments mean "no filter".
func filterCategories(rows []categoryRow, typeFilter, grep string) []categoryRow {
	t := strings.ToUpper(strings.TrimSpace(typeFilter))
	g := strings.ToLower(strings.TrimSpace(grep))
	if t == "" && g == "" {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		if t != "" && !strings.EqualFold(r.Type, t) {
			continue
		}
		if g != "" && !strings.Contains(strings.ToLower(r.Description), g) && !strings.Contains(strings.ToLower(r.Abbreviation), g) {
			continue
		}
		out = append(out, r)
	}
	return out
}
