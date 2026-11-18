package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/store"
)

// edekaErr maps EDEKA errors to the CLI's typed exit codes.
func edekaErr(err error) error {
	if err == nil {
		return nil
	}
	var rl *cliutil.RateLimitError
	var he *edeka.HTTPError
	switch {
	case errors.Is(err, edeka.ErrNoSession), errors.Is(err, edeka.ErrReloginNeeded):
		return authErr(err)
	case errors.Is(err, edeka.ErrNoMarket):
		return usageErr(err)
	case errors.Is(err, edeka.ErrCheckoutBlocked):
		return apiErr(err)
	case errors.As(err, &rl):
		return rateLimitErr(err)
	case errors.As(err, &he):
		if he.Status == 404 {
			return notFoundErr(err)
		}
		return apiErr(err)
	}
	return err
}

// marketFlagUsage is the help text of every --market flag.
const marketFlagUsage = "Market subdomain, e.g. doerflinger-ulm (default: EDEKA_MARKET or 'markets select')"

// applyShopConn hands the root connection flags (--ca-cert, --tls-server-name,
// config-file trust settings and --rate-limit) to the EDEKA shop client, so
// shopping commands connect the same way the generated commands do.
func applyShopConn(flags *rootFlags) error {
	if flags == nil {
		return nil
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	flags.applyTLSFlags(cfg)
	cs := edeka.ConnSettings{Rate: flags.rateLimit}
	ca, name := strings.TrimSpace(cfg.CACert), strings.TrimSpace(cfg.TLSServerName)
	if ca != "" || name != "" {
		tc := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: name}
		if ca != "" {
			pemBytes, err := os.ReadFile(filepath.Clean(ca))
			if err != nil {
				return configErr(fmt.Errorf("reading CA bundle %s: %w", ca, err))
			}
			pool, err := x509.SystemCertPool()
			if err != nil || pool == nil {
				pool = x509.NewCertPool()
			}
			if !pool.AppendCertsFromPEM(pemBytes) {
				return configErr(fmt.Errorf("no certificates in CA bundle %s", ca))
			}
			tc.RootCAs = pool
		}
		cs.TLS = tc
	}
	edeka.Configure(cs)
	return nil
}

// openShop opens the selected market with the imported session and makes sure
// it is logged in (the remember-me cookie renews the shop session silently).
func openShop(cmd *cobra.Command, flags *rootFlags, market string) (*edeka.Shop, error) {
	if err := applyShopConn(flags); err != nil {
		return nil, err
	}
	sh, err := edeka.Open(market, true)
	if err != nil {
		return nil, edekaErr(err)
	}
	if err := sh.Ensure(cmd.Context()); err != nil {
		return nil, edekaErr(err)
	}
	return sh, nil
}

// openPublicShop opens the selected market with the session when there is
// one, otherwise anonymously (markets, offers, search work without login).
func openPublicShop(flags *rootFlags, market string) (*edeka.Shop, error) {
	if err := applyShopConn(flags); err != nil {
		return nil, err
	}
	sh, err := edeka.Open(market, false)
	return sh, edekaErr(err)
}

// openPortal opens the edeka.shop portal (market finder) without a market.
func openPortal(flags *rootFlags) (*edeka.Shop, error) {
	if err := applyShopConn(flags); err != nil {
		return nil, err
	}
	s := &edeka.Session{}
	if path, err := edeka.SessionPath(); err == nil {
		if loaded, err := edeka.LoadSession(path); err == nil {
			s = loaded
		}
	}
	return edeka.OpenWith(s, "", "edeka.shop")
}

// emit prints v as JSON for agents/pipes, otherwise calls human.
func emit(cmd *cobra.Command, flags *rootFlags, v any, human func(w io.Writer)) error {
	w := cmd.OutOrStdout()
	if flags.asJSON || flags.agent || !isTerminal(w) || human == nil {
		return printJSONFiltered(w, v, flags)
	}
	human(w)
	return nil
}

// verifyDryRun short-circuits --dry-run under the Printing Press verifier and
// dogfood, where no EDEKA session exists. Real users keep the full preview.
func verifyDryRun(cmd *cobra.Command, flags *rootFlags, action string) (bool, error) {
	if dryRunOK(flags) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv()) {
		return true, writeDryRun(cmd.OutOrStdout(), flags, action)
	}
	return false, nil
}

// clearImportedSession deletes the session file 'auth import' wrote (under its
// lock), so no shopping command can log in after 'auth logout'.
func clearImportedSession() error {
	path, err := edeka.SessionPath()
	if err != nil {
		return err
	}
	return edeka.ClearSession(path)
}

// edekaDBMissing reports whether the local store has not been created yet.
// Read-only commands use it under EDEKA_NO_LOCAL_DATA=1 so that reading
// never creates a store on disk.
func edekaDBMissing() bool {
	_, err := os.Stat(defaultDBPath("edeka-pp-cli"))
	return errors.Is(err, fs.ErrNotExist)
}

// openEdekaDB opens the local store and makes sure the EDEKA tables exist.
func openEdekaDB(ctx context.Context) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("edeka-pp-cli"))
	if err != nil {
		return nil, err
	}
	_, err = db.DB().ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS edeka_prices (article_id TEXT NOT NULL, title TEXT, price_cents INTEGER NOT NULL, unit_price TEXT, on_offer INTEGER NOT NULL DEFAULT 0, source TEXT NOT NULL, market TEXT NOT NULL, seen_on TEXT NOT NULL, seen_at TEXT NOT NULL, PRIMARY KEY (article_id, market, seen_on, source));
CREATE INDEX IF NOT EXISTS edeka_prices_article ON edeka_prices(article_id, seen_on);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// recordPrices stores the prices of a product grid (best effort, one row per
// article, market, day and source). It writes nothing when the operator turned
// local data off with EDEKA_NO_LOCAL_DATA=1.
func recordPrices(ctx context.Context, products []edeka.Product, market, source string) {
	if len(products) == 0 || cliutil.IsVerifyEnv() || localDataDisabled() {
		return
	}
	db, err := openEdekaDB(ctx)
	if err != nil {
		return
	}
	defer db.Close()
	now := time.Now().UTC()
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return
	}
	for _, p := range products {
		if p.ID == "" || p.PriceCents <= 0 {
			continue
		}
		_, _ = tx.ExecContext(ctx, `INSERT INTO edeka_prices(article_id,title,price_cents,unit_price,on_offer,source,market,seen_on,seen_at) VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(article_id,market,seen_on,source) DO UPDATE SET title=excluded.title, price_cents=excluded.price_cents, unit_price=excluded.unit_price, on_offer=excluded.on_offer, seen_at=excluded.seen_at`,
			p.ID, p.Title, p.PriceCents, p.UnitPrice, boolInt(p.OnOffer), source, edeka.MarketSlug(market), now.Format("2006-01-02"), now.Format(time.RFC3339))
	}
	_ = tx.Commit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// printProducts renders product rows for humans.
func printProducts(w io.Writer, ps []edeka.Product) {
	if len(ps) == 0 {
		fmt.Fprintln(w, "No products.")
		return
	}
	for _, p := range ps {
		flags := ""
		if p.OnOffer {
			flags += " [offer]"
		}
		if p.WeightArticle {
			flags += " [by weight]"
		}
		if p.InBasket > 0 {
			flags += fmt.Sprintf(" [%dx in basket]", p.InBasket)
		}
		fmt.Fprintf(w, "%-16s %-52s %9s  %s%s\n", p.ID, truncate(p.Title, 52), p.Price, p.UnitPrice, flags)
	}
}

// searchAll collects up to limit products over search pages.
func searchAll(ctx context.Context, sh *edeka.Shop, query string, limit int) ([]edeka.Product, error) {
	var out []edeka.Product
	for page := 0; page < 10 && len(out) < limit; page++ {
		g, err := sh.Search(ctx, query, page)
		if err != nil {
			return nil, err
		}
		out = append(out, g.Products...)
		if !g.HasNext || len(g.Products) == 0 {
			break
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// anyTitleMatches reports whether at least one product title shares a word
// with the query. The shop's search falls back to unrelated products for
// unknown words; this tells the two apart.
func anyTitleMatches(query string, ps []edeka.Product) bool {
	for _, p := range ps {
		if edeka.WordMatch(query, p.Title) {
			return true
		}
	}
	return false
}

// joinArgs joins positional words into one query.
func joinArgs(args []string) string { return strings.TrimSpace(strings.Join(args, " ")) }
