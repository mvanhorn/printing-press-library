// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/store"
	"github.com/spf13/cobra"
)

// newReviewsCmd builds the top-level `reviews` parent + `list` child.
// Reads locally synced reviews with optional syndication-eligible filter.
func newReviewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reviews",
		Short:       "List locally synced product reviews",
		Long:        "Filter and project synced reviews for a product. Supports a syndication-eligible flag filter.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReviewsListCmd(flags))
	return cmd
}

// reviewListRow is the per-review row emitted by `reviews list`.
type reviewListRow struct {
	ID                  string  `json:"id"`
	ProductID           string  `json:"product_id"`
	Title               string  `json:"title"`
	Rating              float64 `json:"rating"`
	SyndicationEligible bool    `json:"syndication_eligible"`
	CreatedAt           string  `json:"created_at,omitempty"`
}

func newReviewsListCmd(flags *rootFlags) *cobra.Command {
	var product string
	var since string
	var syndicationEligible bool
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "list [flags]",
		Short:       "List locally synced reviews for a product",
		Long:        "Lists reviews for --product from the local store. Pass --syndication-eligible to keep only reviews whose JSON marks them syndication-eligible.\n\nSearches reviews.attributes for the syndication-eligible flag. Field naming may vary across G2 API versions; the filter checks syndication_eligible, eligible_for_syndication, and syndicateable.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  g2-pp-cli reviews list --product slack --syndication-eligible --since 7d",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(product) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--product is required"))
			}
			var cutoff time.Time
			if strings.TrimSpace(since) != "" {
				window, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
				cutoff = time.Now().Add(-window)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("g2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "products_reviews", flags.maxAge)

			if cliutil.IsDogfoodEnv() && limit > 50 {
				limit = 50
			}

			rows, err := queryReviewsList(db.DB(), product, cutoff, syndicationEligible, limit)
			if err != nil {
				return fmt.Errorf("querying reviews: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"id", "product_id", "title", "rating", "syndication_eligible", "created_at"}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					r.ID,
					r.ProductID,
					r.Title,
					fmt.Sprintf("%.1f", r.Rating),
					fmt.Sprintf("%t", r.SyndicationEligible),
					r.CreatedAt,
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "Product slug or ID (required)")
	cmd.Flags().StringVar(&since, "since", "", "Recency window (e.g. 7d, 24h, 1w)")
	cmd.Flags().BoolVar(&syndicationEligible, "syndication-eligible", false, "Keep only reviews marked syndication-eligible")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/g2-pp-cli/data.db)")
	return cmd
}

// queryReviewsList runs the SQL query against the typed `reviews` table
// (the dependent hierarchical child of `products`). When the typed table
// has rows but `data` JSON isn't preserved, returns the typed columns.
func queryReviewsList(db *sql.DB, product string, cutoff time.Time, syndicationEligible bool, limit int) ([]reviewListRow, error) {
	conds := []string{`(products_id = ? OR products_id IN (SELECT id FROM products WHERE slug = ? OR id = ?))`}
	argv := []any{product, product, product}
	if !cutoff.IsZero() {
		conds = append(conds, `synced_at >= ?`)
		argv = append(argv, cutoff)
	}
	q := `SELECT id, products_id, data, synced_at FROM reviews WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY synced_at DESC LIMIT ?`
	argv = append(argv, limit*4) // overfetch so the syndication filter can sieve

	rs, err := db.Query(q, argv...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rs.Close()

	var out []reviewListRow
	for rs.Next() {
		var (
			id        sql.NullString
			productID sql.NullString
			data      sql.NullString
			syncedAt  sql.NullTime
		)
		if err := rs.Scan(&id, &productID, &data, &syncedAt); err != nil {
			return nil, err
		}
		row := reviewListRow{ID: id.String, ProductID: productID.String}
		if data.Valid {
			var obj map[string]any
			if json.Unmarshal([]byte(data.String), &obj) == nil {
				attrs, _ := obj["attributes"].(map[string]any)
				if attrs == nil {
					attrs = obj
				}
				row.Title, _ = attrs["title"].(string)
				if v, ok := attrs["rating"].(float64); ok {
					row.Rating = v
				} else if v, ok := attrs["star_rating"].(float64); ok {
					row.Rating = v
				}
				row.SyndicationEligible = extractSyndicationFlag(attrs)
				row.CreatedAt, _ = attrs["created_at"].(string)
			}
		}
		if syndicationEligible && !row.SyndicationEligible {
			continue
		}
		out = append(out, row)
		if len(out) >= limit {
			break
		}
	}
	return out, rs.Err()
}

// extractSyndicationFlag finds the syndication-eligible boolean under
// any of several known field names. The G2 v2 OpenAPI spec doesn't
// document the exact key, so we try the most likely variants.
func extractSyndicationFlag(attrs map[string]any) bool {
	for _, k := range []string{
		"syndication_eligible",
		"eligible_for_syndication",
		"syndicateable",
		"syndicate_eligible",
		"is_syndication_eligible",
	} {
		if v, ok := attrs[k].(bool); ok {
			return v
		}
		if v, ok := attrs[k].(string); ok {
			lower := strings.ToLower(v)
			if lower == "true" || lower == "yes" || lower == "1" {
				return true
			}
		}
		if v, ok := attrs[k].(float64); ok {
			return v != 0
		}
	}
	return false
}
