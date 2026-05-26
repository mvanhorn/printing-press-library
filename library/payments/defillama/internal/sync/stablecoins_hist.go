// Per-chain stablecoin supply history (P2.3). One row per (chain, date) with
// stablecoin_id = "_total" (aggregate). This is what `stables flow` reads.
package sync

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/client"
)

const aggregateID = "_total"

// SyncStablecoinChainHistory fetches /stablecoincharts/{chain} for every chain
// that currently reports stablecoin supply and writes per-day chain totals
// into stablecoin_hist.
func (e *Engine) SyncStablecoinChainHistory(ctx context.Context) error {
	chains, err := e.listStablecoinChains(ctx)
	if err != nil {
		return err
	}
	total := 0
	for _, chain := range chains {
		n, err := e.syncOneStablecoinChain(ctx, chain)
		if err != nil {
			e.Report("stablecoin_flow", fmt.Sprintf("skip %s: %v", chain, err))
			continue
		}
		total += n
		e.Report("stablecoin_flow", fmt.Sprintf("%s: %d rows", chain, n))
	}
	return e.S.SetSyncMeta("stablecoin_hist", total, "")
}

// listStablecoinChains returns chain names from /stablecoinchains.
func (e *Engine) listStablecoinChains(ctx context.Context) ([]string, error) {
	body, err := e.C.Get(ctx, client.HostStablecoins, "/stablecoinchains", nil)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var arr []map[string]any
	if err := decodeJSON(body, &arr); err != nil {
		return nil, err
	}
	out := []string{}
	for _, c := range arr {
		name := rawStr(c["name"])
		if name == "" {
			continue
		}
		// Skip chains with negligible supply (saves ~30% of the calls).
		mcap := 0.0
		if m, ok := c["totalCirculatingUSD"].(map[string]any); ok {
			mcap = rawNum(m["peggedUSD"])
		}
		if mcap < 1e6 {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func (e *Engine) syncOneStablecoinChain(ctx context.Context, chain string) (int, error) {
	body, err := e.C.Get(ctx, client.HostStablecoins, "/stablecoincharts/"+url.PathEscape(chain), nil)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	var arr []map[string]any
	if err := decodeJSON(body, &arr); err != nil {
		return 0, err
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM stablecoin_hist WHERE stablecoin_id = ? AND chain = ?`, aggregateID, chain); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO stablecoin_hist (stablecoin_id, date, circulating, chain) VALUES (?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range arr {
			ts := rawNum(p["date"])
			if ts <= 0 {
				continue
			}
			circ := 0.0
			if m, ok := p["totalCirculatingUSD"].(map[string]any); ok {
				circ = rawNum(m["peggedUSD"])
			}
			if circ == 0 {
				if m, ok := p["totalCirculating"].(map[string]any); ok {
					circ = rawNum(m["peggedUSD"])
				}
			}
			date := time.Unix(int64(ts), 0).UTC().Format("2006-01-02")
			if _, err := stmt.Exec(aggregateID, date, circ, chain); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// suppress unused warnings on packages used only in some build paths
var _ = strings.ToLower
