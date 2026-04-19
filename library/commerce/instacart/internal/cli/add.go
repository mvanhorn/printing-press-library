package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// historyMaxAgeDays caps how old a history match can be before the
// resolver falls through to live search. 365 days is generous enough that
// seasonal items (pumpkin pie mix in November) still resolve, while
// protecting against really stale signals.
const historyMaxAgeDays = 365

// newAddCmd is THE killer feature. `instacart add costco "2% milk"` resolves
// the product via direct GraphQL and fires UpdateCartItemsMutation against
// your live cart. No browser control, no Playwright, under a second round trip.
func newAddCmd() *cobra.Command {
	var qty float64
	var yes bool
	var cartID string
	var itemID string
	var noHistory bool
	cmd := &cobra.Command{
		Use:   "add <retailer> <query...>",
		Short: "Add a product to a retailer cart by natural language",
		Long: `Resolves a product from a natural-language query and adds it to your
active cart at <retailer>. Uses the three-call GraphQL chain
(ShopCollectionScoped -> Autosuggestions -> Items) to find real items with
real names, then fires UpdateCartItemsMutation to add the top match.

Argument shape: first positional arg is the retailer slug, remaining args
are joined as the search query. For backward compatibility the old shape
"add <query> <retailer>" is detected when the LAST arg matches a known
retailer slug; a deprecation notice prints to stderr once.

Override with --item-id to skip search and use an exact Instacart item id.
Use --dry-run to preview without firing the mutation. The item is added to
your active cart but NOT checked out -- you still complete checkout in the
Instacart app or web UI.`,
		Example: `  instacart add costco "2% milk"
  instacart add costco "organic eggs" --qty 2 --yes
  instacart add costco milk --dry-run --json
  instacart add --item-id items_1576-17315429 costco --yes`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}

			var query, retailer string
			if itemID != "" {
				if len(args) < 1 {
					return coded(ExitUsage, "usage: instacart add --item-id <id> <retailer>")
				}
				retailer = strings.ToLower(args[0])
				if len(args) > 1 {
					retailer = strings.ToLower(args[len(args)-1])
				}
			} else {
				if len(args) < 2 {
					return coded(ExitUsage, "usage: instacart add <retailer> <query...>  (or pass --item-id)")
				}
				retailer, query = detectArgShape(app, args)
			}

			var pick SearchResult
			resolvedVia := "live"
			if itemID != "" {
				pick = SearchResult{
					Name:      "(item id supplied: " + itemID + ")",
					ItemID:    itemID,
					ProductID: itemID[strings.LastIndex(itemID, "-")+1:],
					Retailer:  retailer,
				}
				resolvedVia = "item-id"
			} else {
				// History-first resolver: when the user has bought something
				// that FTS-matches the query at this retailer recently enough,
				// skip the three-call live search entirely. Falls through on
				// low confidence, stale last_purchased_at, or --no-history.
				if !noHistory {
					if hit := resolveFromHistory(app, retailer, query); hit != nil {
						pick = *hit
						resolvedVia = "history"
					}
				}
				if pick.ItemID == "" {
					results, err := gql.ResolveProducts(app.Ctx, app.Session, app.Cfg, app.Store, retailer, query, 5)
					if err != nil {
						return coded(ExitNotFound, "could not resolve %q at %s: %v", query, retailer, err)
					}
					if len(results) == 0 {
						return coded(ExitNotFound, "no results for %q at %s", query, retailer)
					}
					pick = results[0]
				}
			}
			if pick.ItemID == "" {
				return coded(ExitNotFound, "no item id resolved; pass --item-id explicitly")
			}

			if app.DryRun {
				preview := map[string]any{
					"query":        query,
					"retailer":     retailer,
					"resolved_via": resolvedVia,
					"picked": map[string]any{
						"name":       pick.Name,
						"item_id":    pick.ItemID,
						"product_id": pick.ProductID,
					},
					"mutation": "UpdateCartItemsMutation",
					"quantity": qty,
					"status":   "dry-run (mutation not fired)",
				}
				if resolvedVia != "history" && historyIsEmpty(app) {
					preview["hint"] = backfillHint()
				}
				if app.JSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(preview)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would add %g x %s (via %s)\n  item_id=%s\n  retailer=%s\n",
					qty, pick.Name, resolvedVia, pick.ItemID, retailer)
				return nil
			}

			if !yes && !app.JSON {
				fmt.Fprintf(cmd.OutOrStderr(), "Add %g x %s\n  (%s) to your %s cart? [y/N]: ",
					qty, pick.Name, pick.ItemID, retailer)
				var reply string
				fmt.Scanln(&reply)
				if !strings.EqualFold(reply, "y") && !strings.EqualFold(reply, "yes") {
					return coded(ExitConflict, "cancelled by user")
				}
			}

			if cartID == "" {
				if id, err := resolveActiveCartID(app, retailer); err == nil {
					cartID = id
				}
			}

			client := gql.NewClient(app.Session, app.Cfg, app.Store)
			vars := instacart.UpdateCartItemsVars{
				CartItemUpdates: []instacart.CartItemUpdate{{
					ItemID:         pick.ItemID,
					Quantity:       qty,
					QuantityType:   "each",
					TrackingParams: json.RawMessage(`{}`),
				}},
				CartType:         "grocery",
				CartID:           cartID,
				RequestTimestamp: time.Now().UnixMilli(),
			}
			resp, err := client.Mutation(app.Ctx, "UpdateCartItemsMutation", vars, "")
			if err != nil {
				return err
			}
			var parsed struct {
				Data instacart.UpdateCartItemsResponse `json:"data"`
			}
			_ = json.Unmarshal(resp.RawBody, &parsed)
			if parsed.Data.UpdateCartItems.ErrorType != "" {
				return coded(ExitConflict, "Instacart rejected the add: %s", parsed.Data.UpdateCartItems.ErrorType)
			}

			// Write-back to purchased_items so the history-first resolver gets
			// stronger every time the user actually buys something. Increments
			// purchase_count; refreshes last_purchased_at and last_price_cents.
			if err := writeBackPurchasedItem(app, retailer, pick); err != nil {
				// Non-fatal -- the user's add succeeded, history write-back is
				// best-effort.
				fmt.Fprintf(cmd.OutOrStderr(), "warning: history write-back failed: %v\n", err)
			}

			if app.JSON {
				envelope := map[string]any{
					"added":        pick,
					"cart_id":      cartID,
					"resolved_via": resolvedVia,
					"result":       parsed.Data.UpdateCartItems,
				}
				if resolvedVia != "history" && historyIsEmpty(app) {
					envelope["hint"] = backfillHint()
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(envelope)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added to %s cart:\n  %s (via %s)\n  item_id=%s\n", retailer, pick.Name, resolvedVia, pick.ItemID)
			if parsed.Data.UpdateCartItems.Cart != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  cart now has %d item(s)\n", parsed.Data.UpdateCartItems.Cart.ItemCount)
			}
			return nil
		},
	}
	cmd.Flags().Float64Var(&qty, "qty", 1, "Quantity to add")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&cartID, "cart-id", "", "Explicit cart ID (otherwise resolved from active carts)")
	cmd.Flags().StringVar(&itemID, "item-id", "", "Exact Instacart item id (format: items_<locationId>-<productId>). Bypasses natural-language search.")
	cmd.Flags().BoolVar(&noHistory, "no-history", false, "Skip the history-first resolver and go straight to live GraphQL search")
	return cmd
}

// resolveFromHistory runs an FTS5 lookup against the user's local purchase
// history and returns a SearchResult when confidence is high enough to
// skip the live search entirely.
//
// Confidence rules:
//   - At least one FTS match at this retailer, OR
//   - Top match's last_purchased_at is within historyMaxAgeDays
//
// FTS5 already ranks by bm25; we rely on that ranking. The first (top)
// match is used. More sophisticated scoring can be layered on once we
// have real usage data to tune against.
//
// Returns nil when no acceptable match is found, causing the caller to
// fall through to the live GraphQL chain.
func resolveFromHistory(app *AppContext, retailer, query string) *SearchResult {
	if app.Store == nil || retailer == "" || query == "" {
		return nil
	}
	matches, err := app.Store.SearchPurchasedItems(query, retailer, 3)
	if err != nil || len(matches) == 0 {
		return nil
	}
	top := matches[0]
	if top.ItemID == "" {
		return nil
	}
	// Stale signal check: if this item was last purchased more than a year
	// ago, the user's taste may have shifted or the product may be gone.
	if !top.LastPurchasedAt.IsZero() {
		age := time.Since(top.LastPurchasedAt)
		if age > time.Duration(historyMaxAgeDays)*24*time.Hour {
			return nil
		}
	}
	// Out-of-stock signal: if last_in_stock is false, fall through to live
	// search so the user gets a current, in-stock alternative.
	if !top.LastInStock {
		return nil
	}
	return &SearchResult{
		Name:      top.Name,
		ItemID:    top.ItemID,
		ProductID: top.ProductID,
		Retailer:  retailer,
	}
}

// writeBackPurchasedItem records a successful add back into the
// purchased_items table so the history-first resolver gets smarter over
// time without requiring a full `history sync` re-run. incrementCount=true
// bumps purchase_count by one.
func writeBackPurchasedItem(app *AppContext, retailer string, pick SearchResult) error {
	if app.Store == nil {
		return nil
	}
	now := time.Now()
	return app.Store.UpsertPurchasedItem(store.PurchasedItem{
		ItemID:           pick.ItemID,
		RetailerSlug:     retailer,
		ProductID:        pick.ProductID,
		Name:             pick.Name,
		LastPurchasedAt:  now,
		FirstPurchasedAt: now,
		PurchaseCount:    1,
		LastInStock:      true,
	}, true)
}

// detectArgShape supports both "add <retailer> <query...>" (new, preferred)
// and "add <query...> <retailer>" (deprecated) by checking whether the first
// or last arg matches a known retailer. Prefers the new shape on ambiguity.
func detectArgShape(app *AppContext, args []string) (retailer, query string) {
	first := strings.ToLower(args[0])
	last := strings.ToLower(args[len(args)-1])

	firstIsKnown := isKnownRetailer(app, first)
	lastIsKnown := isKnownRetailer(app, last)

	switch {
	case firstIsKnown:
		retailer = first
		query = strings.Join(args[1:], " ")
	case lastIsKnown && len(args) >= 2:
		// Backward-compat: old "query ... retailer" form.
		retailer = last
		query = strings.Join(args[:len(args)-1], " ")
		fmt.Fprintf(app.stderr(), "note: \"instacart add <query> <retailer>\" is deprecated -- use \"instacart add <retailer> <query...>\"\n")
	default:
		// Neither arg matches a known retailer. Assume new shape (first arg
		// is retailer) so that first-run searches for brand-new retailers
		// still work naturally.
		retailer = first
		query = strings.Join(args[1:], " ")
	}
	return retailer, query
}

func isKnownRetailer(app *AppContext, slug string) bool {
	r, err := app.Store.GetRetailer(slug)
	if err != nil || r == nil {
		// Hardcoded list of common slugs so first-time users don't get
		// surprised by the detector treating everything as the new shape.
		// These are popular Instacart retailers.
		commons := map[string]bool{
			"costco": true, "costco-business-center": true,
			"pcc-community-markets": true, "safeway": true, "kroger": true,
			"whole-foods-market": true, "aldi": true, "sprouts": true,
			"sprouts-farmers-market": true, "cvs": true, "walgreens": true,
			"bjs": true, "bjs-wholesale-club": true, "sams-club": true,
			"target": true, "publix": true, "trader-joes": true,
			"wegmans": true, "heb": true, "meijer": true, "harris-teeter": true,
		}
		return commons[slug]
	}
	return true
}

// resolveActiveCartID finds the user's active cart at a retailer. It first
// tries PersonalActiveCarts (always available, only needs the session), then
// falls back to ActiveCartId query if we have shopId + addressId cached.
// Returns empty string on no-cart-found (callers should let the server pick).
func resolveActiveCartID(app *AppContext, retailer string) (string, error) {
	client := gql.NewClient(app.Session, app.Cfg, app.Store)

	resp, err := client.Query(app.Ctx, "PersonalActiveCarts", map[string]any{})
	if err == nil {
		var parsed struct {
			Data instacart.PersonalActiveCartsResponse `json:"data"`
		}
		if json.Unmarshal(resp.RawBody, &parsed) == nil {
			for _, c := range parsed.Data.UserCarts.Carts {
				if c.Retailer.Slug == retailer {
					return c.ID, nil
				}
			}
		}
	}

	r, err := app.Store.GetRetailer(retailer)
	if err == nil && r != nil && r.ShopID != "" && app.Cfg.AddressID != "" {
		resp, err := client.Query(app.Ctx, "ActiveCartId", instacart.ActiveCartIDVars{
			AddressID: app.Cfg.AddressID,
			ShopID:    r.ShopID,
		})
		if err == nil {
			var parsed struct {
				Data instacart.ActiveCartIDResponse `json:"data"`
			}
			if json.Unmarshal(resp.RawBody, &parsed) == nil {
				return parsed.Data.ActiveCartID.ID, nil
			}
		}
	}

	return "", nil
}
