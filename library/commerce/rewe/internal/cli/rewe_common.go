package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/store"
)

// reweErr maps REWE errors to the CLI's typed exit codes.
func reweErr(err error) error {
	if err == nil {
		return nil
	}
	var rl *cliutil.RateLimitError
	switch {
	case errors.Is(err, rewe.ErrNoSession), errors.Is(err, rewe.ErrReloginNeeded):
		return authErr(err)
	case errors.Is(err, rewe.ErrEmptyBasket):
		return usageErr(errors.New("the basket is empty: add items first (basket add, list add, recipe basket --add)"))
	case errors.As(err, &rl):
		return rateLimitErr(err)
	case rewe.IsCloudflareBlock(err):
		return apiErr(fmt.Errorf("Cloudflare blocked the request; wait a few minutes and retry, or re-import the browser cookies (cf_clearance): %w", err))
	}
	var he *rewe.HTTPError
	if errors.As(err, &he) {
		if he.Status == 404 {
			return notFoundErr(err)
		}
		return apiErr(err)
	}
	return err
}

// openShop loads the session and makes sure the shop session is logged in,
// renewing it through single sign-on when needed.
func openShop(cmd *cobra.Command, flags *rootFlags) (*rewe.Shop, *rewe.Configuration, error) {
	sh, err := rewe.Open()
	if err != nil {
		return nil, nil, reweErr(err)
	}
	c, err := sh.Ensure(cmd.Context())
	if err != nil {
		return nil, nil, reweErr(err)
	}
	if sh.Renewed() {
		syncGeneratedCredential(flags, sh.Session)
	}
	return sh, c, nil
}

// openShopAnonymous loads the session when there is one (for market cookies),
// otherwise works without login (public endpoints).
func openShopAnonymous() (*rewe.Shop, error) {
	sh, err := rewe.Open()
	if errors.Is(err, rewe.ErrNoSession) {
		path, perr := rewe.SessionPath()
		if perr != nil {
			return nil, perr
		}
		empty := &rewe.Session{}
		return rewe.OpenWith(empty, path)
	}
	return sh, reweErr(err)
}

// syncGeneratedCredential is kept as the single place to hook a credential
// store in. It intentionally stores nothing: the generated client gets the
// live session from the client hook in rewe_session_hook.go, and a second
// copy of the cookies would only go stale (the shop rotates its session).
func syncGeneratedCredential(flags *rootFlags, s *rewe.Session) {}

// emit prints v as JSON for agents/pipes, otherwise calls human.
func emit(cmd *cobra.Command, flags *rootFlags, v any, human func(w io.Writer)) error {
	w := cmd.OutOrStdout()
	if flags.asJSON || flags.agent || !isTerminal(w) || human == nil {
		return printJSONFiltered(w, v, flags)
	}
	human(w)
	return nil
}

// openReweDB opens the local store and makes sure the REWE tables exist.
func openReweDB(ctx context.Context) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("rewe-pp-cli"))
	if err != nil {
		return nil, err
	}
	_, err = db.DB().ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS rewe_prices (product_id TEXT NOT NULL, name TEXT, price_cents INTEGER NOT NULL, source TEXT NOT NULL, market TEXT, seen_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS rewe_prices_pid ON rewe_prices(product_id, seen_at);
CREATE TABLE IF NOT EXISTS rewe_offers (nan TEXT NOT NULL, week TEXT NOT NULL, market TEXT NOT NULL, title TEXT, details TEXT, price_cents INTEGER, price TEXT, category TEXT, label TEXT, seen_at TEXT NOT NULL, PRIMARY KEY (nan, week, market));`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// recordPrices stores product prices seen in a search (best effort).
func recordPrices(ctx context.Context, products []rewe.Product, market string) {
	db, err := openReweDB(ctx)
	if err != nil {
		return
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return
	}
	for _, p := range products {
		if p.ProductID == "" || p.PriceCents <= 0 {
			continue
		}
		_, _ = tx.ExecContext(ctx, `INSERT INTO rewe_prices(product_id,name,price_cents,source,market,seen_at) VALUES(?,?,?,?,?,?)`, p.ProductID, p.Name, p.PriceCents, "search", market, now)
	}
	_ = tx.Commit()
}

// recordOffers upserts offers into the local store.
func recordOffers(ctx context.Context, offers []rewe.Offer) error {
	db, err := openReweDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, o := range offers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO rewe_offers(nan,week,market,title,details,price_cents,price,category,label,seen_at) VALUES(?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(nan,week,market) DO UPDATE SET title=excluded.title, details=excluded.details, price_cents=excluded.price_cents, price=excluded.price, category=excluded.category, label=excluded.label, seen_at=excluded.seen_at`,
			o.NAN, o.Week, o.Market, o.Title, o.Details, o.PriceCents, o.Price, o.Category, o.Label, now); err != nil {
			return err
		}
		if o.PriceCents > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO rewe_prices(product_id,name,price_cents,source,market,seen_at) VALUES(?,?,?,?,?,?)`, "nan:"+o.NAN, o.Title, o.PriceCents, "offer", o.Market, now); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
