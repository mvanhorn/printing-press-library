package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

// Hand-written commands without a generator scaffold. root.go wires them
// under the generated parents (markets, slots, basket, order, auth) and as new
// top-level groups (products, offers, categories, payment).

func newProductsCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "products", Short: "Search the selected market's products with prices and show one product.", RunE: parentNoSubcommandRunE(flags), Annotations: map[string]string{"pp:parent-group": "true", "mcp:read-only": "true", "pp:typed-exit-codes": "0,2"}}
	c.AddCommand(newProductsSearchCmd(flags), newProductsShowCmd(flags))
	return c
}

func newPaymentCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{Use: "payment", Short: "Payment methods of the selected market's checkout.", RunE: parentNoSubcommandRunE(flags), Annotations: map[string]string{"pp:parent-group": "true", "mcp:read-only": "true", "pp:typed-exit-codes": "0,2"}}
	c.AddCommand(newPaymentOptionsCmd(flags))
	return c
}

func newEdekaMarketsSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "search <postal-code-or-town>",
		Short:       "Find EDEKA markets that sell online near a postal code or in a town (pickup, delivery, delivery postal codes).",
		Long:        "Uses the edeka.shop market finder. A postal code returns the markets that serve it; a town is first looked up\nin the portal's town list. The 'slug' is what 'markets select' and --market take. Works without login.",
		Example:     "  edeka-pp-cli markets search 89073\n  edeka-pp-cli markets search Stuttgart --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "query=89073"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "markets search")
			}
			portal, err := openPortal(flags)
			if err != nil {
				return err
			}
			ms, err := portal.SearchMarkets(cmd.Context(), joinArgs(args))
			if err != nil {
				return edekaErr(err)
			}
			if ms == nil {
				ms = []edeka.Market{}
			}
			if len(ms) == 0 {
				return emitThen(cmd, flags, map[string]any{"query": joinArgs(args), "count": 0, "markets": ms}, notFoundErr(fmt.Errorf("no EDEKA online market found for %q (use a postal code or a town name)", joinArgs(args))))
			}
			return emit(cmd, flags, map[string]any{"query": joinArgs(args), "count": len(ms), "markets": ms}, func(w io.Writer) {
				if len(ms) == 0 {
					fmt.Fprintln(w, "No EDEKA online market found.")
				}
				if ms[0].Nearby {
					fmt.Fprintf(w, "No market serves %s; nearest by postal code:\n", joinArgs(args))
				}
				for _, m := range ms {
					near := ""
					if m.Nearby {
						near = "  (serves " + m.ServedZip + ")"
					}
					fmt.Fprintf(w, "%-28s %-36s %s %s %s  pickup:%v delivery:%v%s\n", m.Slug, truncate(m.Name, 36), m.Street, m.Zip, m.City, m.Pickup, m.Delivery, near)
				}
			})
		},
	}
	return cmd
}

func newMarketsSelectCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "select <subdomain>",
		Short:       "Choose the market the shop commands work with (stored for later calls).",
		Long:        "Stores the market (subdomain of its shop, e.g. doerflinger-ulm) for later commands. With an imported session the\nmarket is also written into the session file, so EDEKA_SESSION_FILE keeps one market per customer.",
		Example:     "  edeka-pp-cli markets select doerflinger-ulm",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "subdomain=doerflinger-ulm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if len(args) == 0 {
				return cmd.Help()
			}
			host, err := edeka.NormalizeMarket(args[0])
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "markets select "+edeka.MarketSlug(host))
			}
			out := map[string]any{"market": edeka.MarketSlug(host), "host": host, "shop_url": "https://" + host + "/"}
			if p, err := edeka.SaveSelectedMarket(host); err == nil {
				out["saved_to"] = p
			} else {
				return configErr(err)
			}
			if path, err := edeka.SessionPath(); err == nil {
				s, err := edeka.UpdateMarket(path, edeka.MarketSlug(host))
				switch {
				case err == nil:
					out["session_market"] = s.Market
					out["session_has_login_for_market"] = s.Has(edeka.RememberCookie, host)
				case !errors.Is(err, edeka.ErrNoSession):
					return configErr(err)
				}
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "Selected market %s (%s).\n", edeka.MarketSlug(host), "https://"+host+"/")
			})
		},
	}
}

func newMarketsCurrentCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "current",
		Short:       "Show the selected market and whether the imported session is logged in there.",
		Example:     "  edeka-pp-cli markets current --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "markets current")
			}
			sh, err := openPublicShop(flags, "")
			if err != nil {
				return err
			}
			out := map[string]any{"market": edeka.MarketSlug(sh.Host), "host": sh.Host, "shop_url": sh.BaseURL() + "/", "session": sh.Path != ""}
			if sh.Path != "" {
				ok, err := sh.LoggedIn(cmd.Context())
				if err == nil {
					out["logged_in"] = ok
				}
				_ = sh.Persist()
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "Market %s (%s). Session: %v. Logged in: %v\n", out["market"], out["shop_url"], out["session"], out["logged_in"])
			})
		},
	}
}

func newProductsSearchCmd(flags *rootFlags) *cobra.Command {
	var market string
	var limit int
	cmd := &cobra.Command{
		Use:         "search <words>",
		Short:       "Search the market's products with price, unit price, offer and weight flags.",
		Long:        "Runs the market's product search (32 products per page, up to --limit). Works without login; with a session the\n'in_basket' field shows quantities already in the basket. Prices are recorded for 'price history'.",
		Example:     "  edeka-pp-cli products search hafermilch --limit 10\n  edeka-pp-cli products search milch --agent --select id,title,price",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "words=milch;--limit=5;--market=doerflinger-ulm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "products search")
			}
			if limit < 1 || limit > 320 {
				return usageErr(errors.New("--limit must be between 1 and 320"))
			}
			sh, err := openPublicShop(flags, market)
			if err != nil {
				return err
			}
			ps, err := searchAll(cmd.Context(), sh, joinArgs(args), limit)
			if err != nil {
				return edekaErr(err)
			}
			recordPrices(cmd.Context(), ps, sh.Host, "search")
			_ = sh.Persist()
			out := map[string]any{"market": sh.Host, "query": joinArgs(args), "count": len(ps), "products": ps}
			if !anyTitleMatches(joinArgs(args), ps) {
				// the shop answers unknown words with unrelated fallback products
				out["count"], out["products"] = 0, []edeka.Product{}
				return emitThen(cmd, flags, out, notFoundErr(fmt.Errorf("no product matches %q in market %s", joinArgs(args), sh.Host)))
			}
			return emit(cmd, flags, out, func(w io.Writer) { printProducts(w, ps) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 32, "Maximum products (1-320)")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newProductsShowCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "show <article-id-or-url>",
		Short:       "Show one product: price, unit price, availability and description.",
		Example:     "  edeka-pp-cli products show 5915407005MXST --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "article=5915407005MXST;--market=doerflinger-ulm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "products show")
			}
			sh, err := openPublicShop(flags, market)
			if err != nil {
				return err
			}
			d, err := sh.Product(cmd.Context(), args[0])
			if err != nil {
				if strings.Contains(err.Error(), "not found in this market") {
					return notFoundErr(err)
				}
				return edekaErr(err)
			}
			if d.ID != "" && d.PriceCents > 0 {
				recordPrices(cmd.Context(), []edeka.Product{{ID: d.ID, Title: d.Title, PriceCents: d.PriceCents, UnitPrice: d.UnitPrice}}, sh.Host, "detail")
			}
			return emit(cmd, flags, d, func(w io.Writer) {
				fmt.Fprintf(w, "%s\n  %s  %s  (%s)\n  %s\n", d.Title, d.Price, d.UnitPrice, d.Availability, truncate(d.Description, 400))
			})
		},
	}
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newOffersCmd(flags *rootFlags) *cobra.Command {
	var market, category, search string
	var limit, page, skip int
	cmd := &cobra.Command{
		Use:   "offers",
		Short: "This week's offers of the selected market, optionally one offer category or a word filter.",
		Long: "Reads the market's offer category (/Angebote/, 32 articles per page) up to --limit articles, starting at --page\n" +
			"after skipping --skip matching offers on that page. To continue, pass the answer's next_page and next_skip.\n" +
			"--category takes the offer sub-category path segment shown by 'categories --offers' (e.g. Kuehlprodukte).\n" +
			"--search keeps only offers whose title matches the words. Works without login; prices are recorded for 'price history'.",
		Example:     "  edeka-pp-cli offers --limit 10\n  edeka-pp-cli offers --category Kuehlprodukte --agent --select title,price,unit_price",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--limit=10;--market=doerflinger-ulm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "offers")
			}
			if limit < 1 || limit > 640 {
				return usageErr(errors.New("--limit must be between 1 and 640"))
			}
			if page < 0 || skip < 0 {
				return usageErr(errors.New("--page and --skip must not be negative"))
			}
			sh, err := openPublicShop(flags, market)
			if err != nil {
				return err
			}
			win, err := collectOffers(func(p int) (edeka.Grid, error) {
				g, err := sh.Offers(cmd.Context(), category, p)
				if err == nil {
					recordPrices(cmd.Context(), g.Products, sh.Host, "offers")
				}
				return g, err
			}, page, skip, limit, search)
			if err != nil {
				return edekaErr(err)
			}
			out := win.Offers
			res := map[string]any{"market": sh.Host, "category": category, "count": len(out), "offers": out, "next_page": win.NextPage, "next_skip": win.NextSkip}
			return emit(cmd, flags, res, func(w io.Writer) { printProducts(w, out) })
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Offer sub-category, e.g. Kuehlprodukte (see 'categories --offers')")
	cmd.Flags().StringVar(&search, "search", "", "Keep only offers whose title matches these words")
	cmd.Flags().IntVar(&limit, "limit", 32, "Maximum offers (1-640)")
	cmd.Flags().IntVar(&page, "page", 0, "First page to read (zero-based)")
	cmd.Flags().IntVar(&skip, "skip", 0, "Matching offers to skip on the first page (from a previous answer's next_skip)")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

// offerWindow is one --limit sized slice of the offers and where the next
// slice starts (NextPage nil: nothing left).
type offerWindow struct {
	Offers   []edeka.Product
	NextPage *int
	NextSkip int
}

// collectOffers reads offer pages from page on, skipping the first skip
// matching offers of that page, until limit offers are collected. A cut
// inside a page is reported as NextPage plus NextSkip, so following the
// continuation never skips an offer.
func collectOffers(fetch func(page int) (edeka.Grid, error), page, skip, limit int, search string) (offerWindow, error) {
	type pos struct{ page, idx int }
	var out []edeka.Product
	var at []pos
	hasNext, last := false, page
	for p := page; p < page+20 && len(out) < limit; p++ {
		last = p
		g, err := fetch(p)
		if err != nil {
			return offerWindow{}, err
		}
		idx := 0
		for _, pr := range g.Products {
			if search != "" && edeka.MatchScore(search, pr.Title) <= 0 {
				continue
			}
			if p > page || idx >= skip {
				pr.OnOffer = true
				out = append(out, pr)
				at = append(at, pos{p, idx})
			}
			idx++
		}
		hasNext = g.HasNext
		if !g.HasNext || len(g.Products) == 0 {
			break
		}
	}
	w := offerWindow{Offers: out}
	switch {
	case len(out) > limit:
		next := at[limit].page
		w.NextPage, w.NextSkip = &next, at[limit].idx
		w.Offers = out[:limit]
	case hasNext:
		next := last + 1
		w.NextPage = &next
	}
	if w.Offers == nil {
		w.Offers = []edeka.Product{}
	}
	return w, nil
}

func newCategoriesCmd(flags *rootFlags) *cobra.Command {
	var market string
	var offersOnly bool
	var level int
	cmd := &cobra.Command{
		Use:         "categories",
		Short:       "The market's product categories (paths usable with 'offers --category').",
		Example:     "  edeka-pp-cli categories --level 1\n  edeka-pp-cli categories --offers",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--level=1;--market=doerflinger-ulm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "categories")
			}
			sh, err := openPublicShop(flags, market)
			if err != nil {
				return err
			}
			cs, err := sh.Categories(cmd.Context())
			if err != nil {
				return edekaErr(err)
			}
			out := []edeka.Category{}
			for _, c := range cs {
				if level > 0 && c.Level != level {
					continue
				}
				isOffer := strings.HasPrefix(c.Path, "/Angebote/")
				if offersOnly != isOffer {
					continue
				}
				if offersOnly {
					c.Path = strings.Trim(strings.TrimPrefix(c.Path, "/Angebote/"), "/")
					if c.Path == "" {
						continue
					}
				}
				out = append(out, c)
			}
			return emit(cmd, flags, map[string]any{"market": sh.Host, "count": len(out), "categories": out}, func(w io.Writer) {
				for _, c := range out {
					fmt.Fprintf(w, "%s%-40s %s\n", strings.Repeat("  ", max(c.Level-1, 0)), c.Name, c.Path)
				}
			})
		},
	}
	cmd.Flags().BoolVar(&offersOnly, "offers", false, "List the offer sub-categories instead of the assortment")
	cmd.Flags().IntVar(&level, "level", 0, "Only this tree level (1-3)")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newBasketShowCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Show the basket: items, quantities, prices and total.",
		Example:     "  edeka-pp-cli basket show --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "basket show")
			}
			sh, err := openShop(cmd, flags, market)
			if err != nil {
				return err
			}
			b, err := sh.Basket(cmd.Context())
			_ = sh.Persist()
			if err != nil {
				return edekaErr(err)
			}
			return emit(cmd, flags, b, func(w io.Writer) { printBasket(w, b) })
		},
	}
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func printBasket(w io.Writer, b *edeka.Basket) {
	if len(b.Items) == 0 {
		fmt.Fprintln(w, "The basket is empty.")
		return
	}
	for _, it := range b.Items {
		fmt.Fprintf(w, "%2dx %-16s %-46s %9s\n", it.Quantity, it.ID, truncate(it.Title, 46), edeka.FormatCents(it.TotalCents))
	}
	fmt.Fprintf(w, "Total: %s (%d pieces)\n", b.Total, b.Count)
}

func newBasketSetCmd(flags *rootFlags) *cobra.Command {
	var market string
	var grams int
	cmd := &cobra.Command{
		Use:         "set <article-id> <quantity>",
		Short:       "Set the quantity of a basket article (0 removes it).",
		Example:     "  edeka-pp-cli basket set 5915407005MXST 3 --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "article=5915407005MXST;quantity=1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if done, err := verifyDryRun(cmd, flags, "basket set"); done {
				return err
			}
			if len(args) < 2 {
				return cmd.Help()
			}
			var qty int
			if _, err := fmt.Sscanf(args[1], "%d", &qty); err != nil || qty < 0 || qty > 99 {
				return usageErr(fmt.Errorf("quantity must be a number from 0 to 99"))
			}
			return changeBasketItem(cmd, flags, market, args[0], qty, grams, "set")
		},
	}
	cmd.Flags().IntVar(&grams, "grams", 0, "Weight in grams for weight articles")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newBasketRemoveCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "remove <article-id>",
		Short:       "Remove an article from the basket.",
		Example:     "  edeka-pp-cli basket remove 5915407005MXST --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "article=5915407005MXST"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if done, err := verifyDryRun(cmd, flags, "basket remove"); done {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			return changeBasketItem(cmd, flags, market, args[0], 0, 0, "remove")
		},
	}
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func changeBasketItem(cmd *cobra.Command, flags *rootFlags, market, id string, qty, grams int, action string) error {
	if !edeka.LooksLikeArticleID(id) {
		return usageErr(fmt.Errorf("%q is not an article id (see 'basket show' or 'products search')", id))
	}
	sh, err := openShop(cmd, flags, market)
	if err != nil {
		return err
	}
	defer func() { _ = sh.Persist() }()
	b, err := sh.Basket(cmd.Context())
	if err != nil {
		return edekaErr(err)
	}
	var cur *edeka.BasketItem
	for i := range b.Items {
		if b.Items[i].ID == id {
			cur = &b.Items[i]
		}
	}
	if cur == nil && qty == 0 {
		return notFoundErr(fmt.Errorf("article %s is not in the basket", id))
	}
	out := map[string]any{"article_id": id, "action": action, "quantity": qty, "dry_run": flags.dryRun}
	if cur != nil {
		out["title"] = cur.Title
		out["previous_quantity"] = cur.Quantity
	}
	if flags.dryRun {
		return emit(cmd, flags, out, func(w io.Writer) { fmt.Fprintf(w, "Would %s %s to quantity %d.\n", action, id, qty) })
	}
	if cur != nil && grams == 0 {
		nb, err := sh.UpdateBasketItem(cmd.Context(), *cur, qty, qty == 0)
		if err != nil {
			return edekaErr(err)
		}
		out["basket_total"] = nb.Total
		out["basket_pieces"] = nb.Count
	} else {
		r, err := sh.SetQuantity(cmd.Context(), id, qty, grams)
		if err != nil {
			return edekaErr(err)
		}
		out["basket_total"] = edeka.FormatCents(int(r.BruttoSum*100 + 0.5))
		out["basket_pieces"] = r.ProductCount
	}
	return emit(cmd, flags, out, func(w io.Writer) {
		fmt.Fprintf(w, "Done: %s now %d. Basket total %s.\n", id, qty, out["basket_total"])
	})
}

func newBasketClearCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "clear",
		Short:       "Remove every item from the basket.",
		Example:     "  edeka-pp-cli basket clear --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if done, err := verifyDryRun(cmd, flags, "basket clear"); done {
				return err
			}
			sh, err := openShop(cmd, flags, market)
			if err != nil {
				return err
			}
			defer func() { _ = sh.Persist() }()
			b, err := sh.Basket(cmd.Context())
			if err != nil {
				return edekaErr(err)
			}
			out := map[string]any{"removed": 0, "dry_run": flags.dryRun, "items": b.Items}
			if flags.dryRun || len(b.Items) == 0 {
				out["would_remove"] = len(b.Items)
				return emit(cmd, flags, out, func(w io.Writer) {
					if len(b.Items) == 0 {
						fmt.Fprintln(w, "The basket is already empty.")
						return
					}
					fmt.Fprintf(w, "Would remove %d items:\n", len(b.Items))
					printBasket(w, b)
				})
			}
			n := 0
			for _, it := range b.Items {
				if _, err := sh.UpdateBasketItem(cmd.Context(), it, 0, true); err != nil {
					return edekaErr(fmt.Errorf("removing %s: %w", it.ID, err))
				}
				n++
			}
			out["removed"] = n
			return emit(cmd, flags, out, func(w io.Writer) { fmt.Fprintf(w, "Removed %d items; the basket is empty.\n", n) })
		},
	}
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newSlotsListCmd(flags *rootFlags) *cobra.Command {
	var market, typ, date string
	var all bool
	var days int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Free pickup (or delivery) slots of the selected market, day by day.",
		Example:     "  edeka-pp-cli slots list --days 2\n  edeka-pp-cli slots list --date 2026-11-20 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--days=2;--market=doerflinger-ulm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "slots list")
			}
			sh, err := openPublicShop(flags, market)
			if err != nil {
				return err
			}
			cal, err := sh.Calendar(cmd.Context())
			if err != nil {
				return edekaErr(err)
			}
			list := cal.Pickup
			if typ == "delivery" {
				list = cal.Delivery
			}
			out := []edeka.Slot{}
			seenDays := map[string]bool{}
			for _, s := range list {
				if (!all && !s.Available) || (date != "" && s.Date != date) {
					continue
				}
				if !seenDays[s.Date] {
					if days > 0 && len(seenDays) >= days {
						break
					}
					seenDays[s.Date] = true
				}
				out = append(out, s)
			}
			res := map[string]any{"market": sh.Host, "type": typ, "count": len(out), "slots": out, "types_offered": cal.Types}
			if typ == "delivery" && len(cal.Delivery) == 0 && len(cal.DeliveryAreas) > 0 {
				res["delivery_areas"] = cal.DeliveryAreas
				res["hint"] = "delivery slots show only after a delivery area is chosen in the shop"
			}
			return emit(cmd, flags, res, func(w io.Writer) {
				if len(out) == 0 {
					fmt.Fprintln(w, "No free slots found.")
				}
				for _, s := range out {
					fmt.Fprintln(w, slotText(s))
				}
			})
		},
	}
	cmd.Flags().StringVar(&typ, "type", "pickup", "Slot type: pickup or delivery")
	cmd.Flags().StringVar(&date, "date", "", "Only this day (YYYY-MM-DD)")
	cmd.Flags().IntVar(&days, "days", 3, "Number of days to list (0 = all)")
	cmd.Flags().BoolVar(&all, "all", false, "Include booked-out slots")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newSlotsReserveCmd(flags *rootFlags) *cobra.Command {
	var market, typ string
	cmd := &cobra.Command{
		Use:         "reserve <earliest-or-date-time>",
		Short:       "Choose a pickup or delivery slot for the order (the shop may hold it for a limited time).",
		Example:     "  edeka-pp-cli slots reserve earliest\n  edeka-pp-cli slots reserve '2026-11-20 16:00' --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "slot=earliest", "pp:live-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if done, err := verifyDryRun(cmd, flags, "slots reserve"); done {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			sh, err := openShop(cmd, flags, market)
			if err != nil {
				return err
			}
			defer func() { _ = sh.Persist() }()
			s, err := pickSlot(cmd.Context(), sh, joinArgs(args), typ)
			if err != nil {
				return err
			}
			out := map[string]any{"market": sh.Host, "slot": s, "text": slotText(s), "reserved": false, "dry_run": flags.dryRun}
			if flags.dryRun {
				return emit(cmd, flags, out, func(w io.Writer) { fmt.Fprintf(w, "Would reserve %s\n", slotText(s)) })
			}
			if err := sh.ReserveSlot(cmd.Context(), s); err != nil {
				return edekaErr(err)
			}
			out["reserved"] = true
			if txt, err := sh.SelectedSlotText(cmd.Context()); err == nil && txt != "" {
				out["shop_text"] = txt
			}
			return emit(cmd, flags, out, func(w io.Writer) { fmt.Fprintf(w, "Reserved %s\n", slotText(s)) })
		},
	}
	cmd.Flags().StringVar(&typ, "type", "pickup", "Slot type: pickup or delivery")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newSlotsReleaseCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "release",
		Short:       "Ask the shop to drop the slot chosen in this session (some markets refuse; it then lapses on its own).",
		Example:     "  edeka-pp-cli slots release --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if done, err := verifyDryRun(cmd, flags, "slots release"); done {
				return err
			}
			if flags.dryRun {
				return writeDryRun(cmd.OutOrStdout(), flags, "slots release")
			}
			sh, err := openShop(cmd, flags, market)
			if err != nil {
				return err
			}
			defer func() { _ = sh.Persist() }()
			serverOK, err := sh.ReleaseSlot(cmd.Context())
			if err != nil {
				return edekaErr(err)
			}
			if !serverOK {
				out := map[string]any{"released": false, "note": "this market's shop refused the release (HTTP 500, also in the browser); the reservation lapses on its own, see 'slots selected' for the minutes left, or reserve another slot"}
				if txt, err := sh.SelectedSlotText(cmd.Context()); err == nil && txt != "" {
					out["shop_text"] = txt
				}
				return emitThen(cmd, flags, out, apiErr(errors.New("the shop refused to release the slot; it lapses on its own")))
			}
			return emit(cmd, flags, map[string]any{"released": true}, func(w io.Writer) { fmt.Fprintln(w, "Slot released.") })
		},
	}
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newPaymentOptionsCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "options",
		Short:       "Payment methods the checkout offers (needs items in the basket and a chosen slot).",
		Long:        "Walks the checkout to the payment step (the address step is submitted unchanged) and lists the methods. No order.",
		Example:     "  edeka-pp-cli payment options --json",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payment options")
			}
			sh, err := openShop(cmd, flags, market)
			if err != nil {
				return err
			}
			defer func() { _ = sh.Persist() }()
			opts, err := sh.PaymentOptions(cmd.Context())
			if err != nil {
				return edekaErr(err)
			}
			return emit(cmd, flags, map[string]any{"market": sh.Host, "options": opts}, func(w io.Writer) {
				for _, o := range opts {
					fmt.Fprintf(w, "%-18s %-30s pay at handover: %v\n", o.ID, o.Name, o.AtPickup)
				}
			})
		},
	}
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newOrderListCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Orders in the customer's order history at the selected market.",
		Example:     "  edeka-pp-cli order list --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "order list")
			}
			sh, err := openShop(cmd, flags, market)
			if err != nil {
				return err
			}
			defer func() { _ = sh.Persist() }()
			orders, count, err := sh.Orders(cmd.Context())
			if err != nil {
				return edekaErr(err)
			}
			if orders == nil {
				orders = []edeka.OrderSummary{}
			}
			return emit(cmd, flags, map[string]any{"market": sh.Host, "count": count, "orders": orders}, func(w io.Writer) {
				if len(orders) == 0 {
					fmt.Fprintf(w, "No orders (the shop reports %d).\n", count)
				}
				for _, o := range orders {
					fmt.Fprintf(w, "%-14s %-10s %9s  %s\n", o.Number, o.Date, o.Total, truncate(o.Text, 60))
				}
			})
		},
	}
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

func newAuthImportCmd(flags *rootFlags) *cobra.Command {
	var file, cdp, market string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import the EDEKA session cookies from a logged-in browser (DevTools endpoint or JSON export).",
		Long: "Log in to your market's shop (https://<market>.edeka.shop) once in a browser, then import the cookies:\n" +
			"  --cdp   a Chromium started with --remote-debugging-port (loopback only)\n" +
			"  --file  a JSON cookie export (array, Playwright storage state or CDP cookies); '-' reads stdin\n" +
			"The session is stored at EDEKA_SESSION_FILE (default: session.json in the CLI config dir, which --home moves) with owner-only permissions.\n" +
			"The shop's one-year remember-me cookie (oxid_1) renews the shop session automatically.",
		Example:     "  edeka-pp-cli auth import --cdp http://127.0.0.1:9222\n  edeka-pp-cli auth import --file cookies.json --market doerflinger-ulm",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if cliutil.IsMCPChildEnv() {
				return usageErr(errors.New("auth import is an operator action and is not available through MCP"))
			}
			if file == "" && cdp == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth import")
			}
			var cookies []edeka.Cookie
			var err error
			if cdp != "" {
				cookies, err = edeka.CookiesFromCDP(cmd.Context(), cdp)
			} else {
				var b []byte
				if file == "-" {
					b, err = io.ReadAll(cmd.InOrStdin())
				} else {
					b, err = os.ReadFile(filepath.Clean(file))
				}
				if err == nil {
					cookies, err = edeka.ParseCookieExport(b)
				}
			}
			if err != nil {
				return usageErr(err)
			}
			return saveImportedEDEKA(cmd, flags, cookies, market)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON cookie export ('-' for stdin)")
	cmd.Flags().StringVar(&cdp, "cdp", "", "Chromium DevTools endpoint, e.g. http://127.0.0.1:9222")
	cmd.Flags().StringVar(&market, "market", "", "Market subdomain to use (default: the market the cookies belong to)")
	return cmd
}

// saveImportedEDEKA stores imported cookies as the EDEKA session, checks the
// login at the market and prints the result.
func saveImportedEDEKA(cmd *cobra.Command, flags *rootFlags, cookies []edeka.Cookie, market string) error {
	s := &edeka.Session{Cookies: cookies}
	ms := s.Markets()
	if len(ms) == 0 {
		return authErr(errors.New("the export holds no EDEKA login (no oxid_1 remember-me cookie on a <market>.edeka.shop host): log in to the market's shop in that browser first, with 'stay logged in'"))
	}
	path, err := edeka.SessionPath()
	if err != nil {
		return configErr(err)
	}
	if old, err := edeka.LoadSession(path); err == nil && old.Market != "" {
		s.Market = old.Market
	}
	if market != "" {
		if _, err := edeka.NormalizeMarket(market); err != nil {
			return usageErr(err)
		}
	}
	host, err := edeka.ResolveMarket(market, s)
	if err != nil {
		host = ms[0]
	}
	s.Market = edeka.MarketSlug(host)
	if err := s.Save(path); err != nil {
		return configErr(err)
	}
	if err := applyShopConn(flags); err != nil {
		return err
	}
	sh, err := edeka.OpenWith(s, path, host)
	if err != nil {
		return err
	}
	ok, lerr := sh.LoggedIn(cmd.Context())
	_ = sh.Persist()
	out := map[string]any{"imported": len(cookies), "markets_with_login": ms, "market": s.Market, "logged_in": ok, "session_file": path}
	if exp := s.RememberExpiry(host); !exp.IsZero() {
		out["remember_me_expires"] = exp.Format(time.RFC3339)
	}
	if lerr != nil {
		out["check_error"] = lerr.Error()
	}
	return emit(cmd, flags, out, func(w io.Writer) {
		fmt.Fprintf(w, "Imported %d EDEKA cookies. Market %s, logged in: %v. Session file %s\n", len(cookies), s.Market, ok, path)
	})
}

// errLoginInterrupted is returned when auth connect is stopped with Ctrl+C or
// SIGTERM; the login window is closed and nothing is stored.
var errLoginInterrupted = errors.New("auth connect was interrupted: the login window was closed and no session was stored")

func newAuthConnectCmd(flags *rootFlags) *cobra.Command {
	var browser, market string
	var wait time.Duration
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Open a browser window to log in to your EDEKA market shop yourself, then keep that session for this CLI.",
		Long: "Opens Chrome, Edge or Chromium with a fresh temporary profile on the market's account page.\n" +
			"Log in there yourself and tick 'Angemeldet bleiben' (stay logged in): its one-year cookie is what\n" +
			"the CLI waits for and later renews the shop session with. The session is stored like `auth import`,\n" +
			"the window closes again and its temporary profile is deleted. DevTools listens on 127.0.0.1 only.",
		Example:     "  edeka-pp-cli auth connect --market doerflinger-ulm\n  edeka-pp-cli auth connect --market ec-ulm-lehr --wait 15m",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsMCPChildEnv() {
				return usageErr(errors.New("auth connect needs a person at the screen and is not available through MCP"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth connect")
			}
			var known *edeka.Session
			if path, err := edeka.SessionPath(); err == nil {
				known, _ = edeka.LoadSession(path)
			}
			if known == nil {
				known = &edeka.Session{}
			}
			host, err := edeka.ResolveMarket(market, known)
			if err != nil {
				return usageErr(fmt.Errorf("choose the market to log in to with --market (see `markets search <postal code>`): %w", err))
			}
			exe := browser
			if exe == "" {
				if exe, err = edeka.FindBrowser(); err != nil {
					return configErr(err)
				}
			}
			// Ctrl+C or SIGTERM cancels the login instead of ending the process,
			// so the deferred Close still ends the browser and deletes its profile.
			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			b, err := edeka.StartLoginBrowser(runCtx, exe, edeka.LoginURL(host))
			if err != nil {
				if runCtx.Err() != nil {
					return authErr(errLoginInterrupted)
				}
				return configErr(err)
			}
			defer b.Close()
			fmt.Fprintf(cmd.ErrOrStderr(), "A browser window with the %s login is open. Log in there with 'Angemeldet bleiben' ticked (up to %s); this command continues by itself.\n", host, wait)
			ctx, cancel := context.WithTimeout(runCtx, wait)
			defer cancel()
			cookies, err := edeka.WaitForLogin(ctx, b.Endpoint, b.Closed, 2*time.Second)
			if runCtx.Err() != nil {
				return authErr(errLoginInterrupted)
			}
			if err != nil {
				return authErr(err)
			}
			b.Close()
			// The browser is gone, so nothing is left to clean up: give Ctrl+C its
			// default meaning back for the network check while saving.
			stop()
			return saveImportedEDEKA(cmd, flags, edeka.FilterEDEKA(cookies), edeka.MarketSlug(host))
		},
	}
	cmd.Flags().StringVar(&browser, "browser", "", "browser executable (default: Chrome, Edge or Chromium found on this computer, or EDEKA_BROWSER)")
	cmd.Flags().StringVar(&market, "market", "", "market subdomain to log in to, e.g. doerflinger-ulm (default: the selected market)")
	cmd.Flags().DurationVar(&wait, "wait", 10*time.Minute, "how long to wait for the login")
	return cmd
}

func newAuthSessionCmd(flags *rootFlags) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "session",
		Short:       "Show whether the EDEKA session is logged in at the selected market and when the remember-me cookie expires.",
		Example:     "  edeka-pp-cli auth session --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth session")
			}
			if err := applyShopConn(flags); err != nil {
				return err
			}
			sh, err := edeka.Open(market, true)
			if err != nil {
				return edekaErr(err)
			}
			ok, err := sh.LoggedIn(cmd.Context())
			if err != nil {
				return edekaErr(err)
			}
			_ = sh.Persist()
			out := map[string]any{"market": edeka.MarketSlug(sh.Host), "logged_in": ok, "session_file": sh.Path, "markets_with_login": sh.Session.Markets()}
			if exp := sh.Session.RememberExpiry(sh.Host); !exp.IsZero() {
				out["remember_me_expires"] = exp.Format(time.RFC3339)
			}
			if !ok {
				return emitThen(cmd, flags, out, authErr(edeka.ErrReloginNeeded))
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "Logged in at %s. Remember-me until %v.\n", out["market"], out["remember_me_expires"])
			})
		},
	}
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

// emitThen prints v as JSON (for agents) and returns err for the exit code.
func emitThen(cmd *cobra.Command, flags *rootFlags, v any, err error) error {
	_ = printJSONFiltered(cmd.OutOrStdout(), v, flags)
	return err
}
