package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/amazon"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/store"
)

// addResult is the structured payload emitted for --json runs.
type addResult struct {
	Query           string `json:"query"`
	Matched         bool   `json:"matched"`
	MatchQuality    string `json:"match_quality,omitempty"` // "strict" or "loose"
	Reason          string `json:"reason,omitempty"`
	ASIN            string `json:"asin,omitempty"`
	Title           string `json:"title,omitempty"`
	PurchaseCount   int    `json:"purchase_count,omitempty"`
	LastPurchasedAt string `json:"last_purchased_at,omitempty"`
	Added           bool   `json:"added"`
	DryRun          bool   `json:"dry_run"`
	Quantity        int    `json:"quantity"`
	UsedLegacy      bool   `json:"used_legacy,omitempty"`
}

func newAddCmd() *cobra.Command {
	var qty int
	var allowLoose, legacy bool
	cmd := &cobra.Command{
		Use:   "add <query>",
		Short: "Resolve a query to a previously-purchased ASIN and add to cart",
		Long: strings.TrimSpace(`
Repurchase-only: refuses to add anything that isn't already in your local
history. When multiple history items match, picks the most-frequently-purchased
one (not the most recent). Use --dry-run to preview the match without writing
to the cart.

If the resolver returns a loose match (match_quality=loose), real adds require
--allow-loose. This forces explicit acknowledgement that some of the query
tokens didn't match the title. Dry runs always show the match regardless.`),
		Example: "  amazon-pp-cli add 'bath tissue'\n" +
			"  amazon-pp-cli add 'detergent' --dry-run --json\n" +
			"  amazon-pp-cli add 'AAA batteries' --quantity 2 --profile personal\n" +
			"  amazon-pp-cli add 'cauli puffs' --allow-loose",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			query := joinArgs(args)
			res := addResult{Query: query, DryRun: app.DryRun, Quantity: qty}
			if qty <= 0 {
				return coded(ExitUsage, "--quantity must be >= 1 (got %d)", qty)
			}
			match, quality, err := resolveQueryToASIN(app.Store, query)
			if err != nil {
				return err
			}
			if match == nil {
				res.Reason = "no match in history (this CLI is repurchase-only; run `history search` to see what's loaded)"
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
				}
				return coded(ExitNotFound, "%s", res.Reason)
			}
			res.Matched = true
			res.MatchQuality = string(quality)
			res.ASIN = match.ASIN
			res.Title = match.Title
			res.PurchaseCount = match.PurchaseCount
			if !match.LastPurchasedAt.IsZero() {
				res.LastPurchasedAt = match.LastPurchasedAt.Format(time.RFC3339)
			}
			if app.DryRun {
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would add %s (%s) — purchase_count=%d, last=%s, quality=%s\n",
					match.ASIN, truncate(match.Title, 60), match.PurchaseCount,
					match.LastPurchasedAt.Format("2006-01-02"), quality)
				return nil
			}
			// Loose-match commit gate: structural defence against autopilot agents.
			if quality == store.MatchLoose && !allowLoose {
				res.Reason = fmt.Sprintf("loose match (query %q hit on partial tokens of %q) — pass --allow-loose after confirming this is the item you want",
					query, truncate(match.Title, 60))
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
				}
				return coded(ExitLoose, "%s", res.Reason)
			}
			if err := app.RequireSession(); err != nil {
				return err
			}
			// Browser-driven add: navigate to the product page, click the real
			// "Add to Cart" button, verify the item lands in the ACTIVE cart.
			// The legacy static POST to /gp/aws/cart/add.html returns 200 but
			// silently routes the item to items-of-interest under bot detection.
			if !legacy {
				ctx, cancel := contextWithTimeout(app.Ctx, 120*time.Second)
				defer cancel()
				br, herr := runBrowserHelperAdd(ctx, app.Cfg.CookiesPath(app.Profile.Name), match.ASIN, qty)
				if herr != nil {
					if app.JSON && br != nil {
						// br carries the failure reason; surface it as the result
						res.Added = false
						res.Reason = br.Reason
						_ = json.NewEncoder(cmd.OutOrStdout()).Encode(res)
					}
					return herr
				}
				if br.Status == "add_failed" {
					res.Added = false
					res.Reason = br.Reason
					if app.JSON {
						return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
					}
					return coded(ExitTransient, "add failed: %s", br.Reason)
				}
				res.Added = true
				if br.Title != "" {
					res.Title = br.Title
				}
				if br.Quantity > 0 {
					res.Quantity = br.Quantity
				}
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "added %s × %d (%s)\n", match.ASIN, res.Quantity, truncate(res.Title, 60))
				return nil
			}
			// Legacy static-POST path. Known to silently fail (Amazon routes to
			// items-of-interest); kept only for debugging.
			client, err := amazon.New(app.Profile, app.Session)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout(app.Ctx, 30*time.Second)
			defer cancel()
			line, err := client.AddToCart(ctx, match.ASIN, qty)
			if err != nil {
				return coded(ExitTransient, "add: %v", err)
			}
			res.Added = true
			if line.Title != "" {
				res.Title = line.Title
			}
			res.Quantity = line.Quantity
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s × %d (%s)\n", match.ASIN, line.Quantity, truncate(res.Title, 60))
			return nil
		},
	}
	cmd.Flags().IntVar(&qty, "quantity", 1, "Number of units to add (default 1)")
	cmd.Flags().BoolVar(&allowLoose, "allow-loose", false, "Allow committing a loose FTS match (only some query tokens hit the title). Required when match_quality=loose for non-dry-run.")
	cmd.Flags().BoolVar(&legacy, "legacy", false, "Use the static-POST add path instead of the headless browser. Known to silently route items to items-of-interest; only use for debugging.")
	return cmd
}

// resolveQueryToASIN runs the FTS search and applies the most-frequent
// tiebreak (which is already the SQL ORDER BY, so we just take the first row).
// The returned MatchQuality lets callers signal loose matches to the user.
func resolveQueryToASIN(s *store.Store, query string) (*store.PurchasedItem, store.MatchQuality, error) {
	rows, quality, err := s.SearchPurchasedItems(query, 5)
	if err != nil {
		return nil, "", err
	}
	if len(rows) == 0 {
		return nil, "", nil
	}
	return &rows[0], quality, nil
}
