package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

// Hand-written commands that have no generator scaffold: auth import,
// auth session, basket show/clear, recipe get.
func init() {
	novelCommandHooks = append(novelCommandHooks, func(root *cobra.Command, flags *rootFlags) {
		if auth, _, err := root.Find([]string{"auth"}); err == nil && auth != root {
			_ = auth // auth import/session are wired in root.go next to auth renew
		}
		if basket, _, err := root.Find([]string{"basket"}); err == nil && basket != root {
			basket.AddCommand(newBasketShowCmd(flags), newBasketClearCmd(flags))
		}
		if recipe, _, err := root.Find([]string{"recipe"}); err == nil && recipe != root {
			recipe.AddCommand(newRecipeGetCmd(flags))
		}
	})
}

func newAuthImportCmd(flags *rootFlags) *cobra.Command {
	var file, cdp string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import the REWE session cookies from a logged-in browser (DevTools endpoint or JSON export).",
		Long: "REWE needs one real browser login (captcha + two-factor). Afterwards import the cookies:\n" +
			"  --cdp   a Chromium started with --remote-debugging-port (loopback only)\n" +
			"  --file  a JSON cookie export (array, Playwright storage state or CDP cookies); '-' reads stdin\n" +
			"The session is stored at the REWE_SESSION_FILE path (default: user config dir) with owner-only permissions.",
		Example:     "  rewe-pp-cli auth import --cdp http://127.0.0.1:9222\n  rewe-pp-cli auth import --file cookies.json",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsMCPChildEnv() {
				return usageErr(errors.New("auth import is an operator action and is not available through MCP"))
			}
			if file == "" && cdp == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth import")
			}
			var cookies []rewe.Cookie
			var err error
			if cdp != "" {
				cookies, err = rewe.CookiesFromCDP(cmd.Context(), cdp)
			} else {
				var b []byte
				if file == "-" {
					b, err = io.ReadAll(cmd.InOrStdin())
				} else {
					b, err = os.ReadFile(file)
				}
				if err == nil {
					cookies, err = rewe.ParseCookieExport(b)
				}
			}
			if err != nil {
				return usageErr(err)
			}
			s := &rewe.Session{Cookies: cookies}
			if !s.Has("KEYCLOAK_IDENTITY") && !s.Has("rstp") {
				return authErr(errors.New("the export holds no REWE login cookies (KEYCLOAK_IDENTITY or rstp): log in to rewe.de in that browser first"))
			}
			path, err := rewe.SessionPath()
			if err != nil {
				return configErr(err)
			}
			if err := s.Save(path); err != nil {
				return configErr(err)
			}
			sh, err := rewe.OpenWith(s, path)
			if err != nil {
				return err
			}
			c, err := sh.Ensure(cmd.Context())
			if err != nil {
				return reweErr(err)
			}
			_ = sh.Persist()
			syncGeneratedCredential(flags, sh.Session)
			out := map[string]any{"imported": len(cookies), "logged_in": c.IsLoggedIn, "market": c.SelectedMarket, "service": c.SelectedService, "zip": c.CustomerZipCode, "session_file": path, "sso_expires": s.SSOExpiry().Format(time.RFC3339)}
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "Imported %d REWE cookies. Logged in: %v. Market %s (%s, %s).\n", len(cookies), c.IsLoggedIn, c.SelectedMarket, c.SelectedService, c.CustomerZipCode)
			})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON cookie export ('-' for stdin)")
	cmd.Flags().StringVar(&cdp, "cdp", "", "Chromium DevTools endpoint, e.g. http://127.0.0.1:9222")
	return cmd
}

func newAuthSessionCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "session",
		Short:       "Show whether the REWE shop session is logged in and how long single sign-on lasts (renews if needed).",
		Example:     "  rewe-pp-cli auth session --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth session")
			}
			sh, c, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			out := map[string]any{"logged_in": c.IsLoggedIn, "renewed_now": sh.Renewed(), "market": c.SelectedMarket, "service": c.SelectedService, "zip": c.CustomerZipCode, "sso_expires": sh.Session.SSOExpiry().Format(time.RFC3339)}
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "Logged in: %v (renewed now: %v). Market %s, %s, %s. Single sign-on until %s.\n", c.IsLoggedIn, sh.Renewed(), c.SelectedMarket, c.SelectedService, c.CustomerZipCode, sh.Session.SSOExpiry().Local().Format("2006-01-02"))
			})
		},
	}
}

type basketView struct {
	ID         string           `json:"id"`
	Market     string           `json:"market"`
	Service    string           `json:"service"`
	Items      []basketViewItem `json:"items"`
	TotalCents int              `json:"total_cents"`
	Total      string           `json:"total"`
	Slot       any              `json:"slot,omitempty"`
}

type basketViewItem struct {
	Title      string `json:"title"`
	Quantity   int    `json:"quantity"`
	PriceCents int    `json:"price_cents"`
	TotalCents int    `json:"total_cents"`
	ProductID  string `json:"product_id"`
	ListingID  string `json:"listing_id"`
}

func viewBasket(b *rewe.Basket) basketView {
	v := basketView{ID: b.ID, Market: b.ServiceSelection.WWIdent, Service: b.ServiceSelection.ServiceType, TotalCents: b.Summary.TotalPrice, Total: rewe.FormatCents(b.Summary.TotalPrice), Items: []basketViewItem{}}
	for _, li := range b.LineItems {
		v.Items = append(v.Items, basketViewItem{Title: li.Product.Title, Quantity: li.Quantity, PriceCents: li.Price, TotalCents: li.TotalPrice, ProductID: li.Product.ProductID, ListingID: li.Product.Listing.ListingID})
	}
	if b.TimeSlotInformation != nil && b.TimeSlotInformation.TimeSlotID != "" {
		v.Slot = b.TimeSlotInformation
	}
	return v
}

func printBasket(w io.Writer, v basketView) {
	for _, it := range v.Items {
		fmt.Fprintf(w, "%2dx %-50s %10s\n", it.Quantity, it.Title, rewe.FormatCents(it.TotalCents))
	}
	fmt.Fprintf(w, "Total: %s (%d items)\n", v.Total, len(v.Items))
}

func newBasketShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Short:       "Show the current basket (finds the basket id for you).",
		Example:     "  rewe-pp-cli basket show --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "basket show")
			}
			sh, _, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			id, err := sh.BasketID(cmd.Context())
			if errors.Is(err, rewe.ErrEmptyBasket) {
				v := basketView{Items: []basketViewItem{}, Total: rewe.FormatCents(0)}
				return emit(cmd, flags, v, func(w io.Writer) { fmt.Fprintln(w, "The basket is empty.") })
			}
			if err != nil {
				return reweErr(err)
			}
			b, err := sh.GetBasket(cmd.Context(), id)
			if err != nil {
				return reweErr(err)
			}
			v := viewBasket(b)
			return emit(cmd, flags, v, func(w io.Writer) { printBasket(w, v) })
		},
	}
}

func newBasketClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "clear",
		Short:   "Remove every item from the basket.",
		Example: "  rewe-pp-cli basket clear --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyDryRun(cmd, flags, "basket clear"); done {
				return err
			}
			sh, _, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			id, err := sh.BasketID(cmd.Context())
			if errors.Is(err, rewe.ErrEmptyBasket) {
				v := basketView{Items: []basketViewItem{}, Total: rewe.FormatCents(0)}
				return emit(cmd, flags, v, func(w io.Writer) { fmt.Fprintln(w, "The basket is empty.") })
			}
			if err != nil {
				return reweErr(err)
			}
			b, err := sh.GetBasket(cmd.Context(), id)
			if err != nil {
				return reweErr(err)
			}
			if flags.dryRun {
				return emit(cmd, flags, map[string]any{"dry_run": true, "would_remove": len(b.LineItems), "basket": viewBasket(b)}, func(w io.Writer) {
					fmt.Fprintf(w, "Would remove %d items:\n", len(b.LineItems))
					printBasket(w, viewBasket(b))
				})
			}
			for _, li := range b.LineItems {
				if b, err = sh.RemoveListing(cmd.Context(), id, li.Product.Listing.ListingID); err != nil {
					return reweErr(err)
				}
			}
			_ = sh.Persist()
			v := viewBasket(b)
			return emit(cmd, flags, v, func(w io.Writer) { fmt.Fprintln(w, "Basket cleared."); printBasket(w, v) })
		},
	}
}

func newRecipeGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "get [slug-or-url]",
		Short:       "Show a REWE recipe: name, servings, ingredients and steps.",
		Example:     "  rewe-pp-cli recipe get gemuese-lasagne --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "recipe get")
			}
			sh, err := openShopAnonymous()
			if err != nil {
				return err
			}
			r, err := sh.Recipe(cmd.Context(), args[0])
			if err != nil {
				return reweErr(err)
			}
			return emit(cmd, flags, r, func(w io.Writer) {
				fmt.Fprintf(w, "%s (%s)\n", r.Name, r.Yield)
				for _, i := range r.Ingredients {
					fmt.Fprintf(w, "  - %s\n", i)
				}
				for n, s := range r.Instructions {
					fmt.Fprintf(w, "%d. %s\n", n+1, strings.TrimSpace(s))
				}
			})
		},
	}
}
