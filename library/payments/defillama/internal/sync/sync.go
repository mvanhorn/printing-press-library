// Package sync fetches data from DefiLlama and writes it into the local SQLite mirror.
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/store"
)

type Reporter func(domain, msg string)

type Engine struct {
	C        *client.Client
	S        *store.Store
	Report   Reporter
	Parallel int
}

func New(c *client.Client, s *store.Store) *Engine {
	return &Engine{C: c, S: s, Parallel: 4, Report: func(string, string) {}}
}

var Domains = []string{
	"protocols",
	"chains",
	"pools",
	"stablecoins",
	"dexs",
	"fees",
	"options",
	"open_interest",
}

func (e *Engine) SyncAll(ctx context.Context) error {
	return e.SyncDomains(ctx, Domains)
}

func (e *Engine) SyncDomains(ctx context.Context, domains []string) error {
	type job struct {
		name string
		fn   func(context.Context) (int, error)
	}
	jobs := []job{}
	for _, d := range domains {
		switch d {
		case "protocols":
			jobs = append(jobs, job{"protocols", e.syncProtocols})
		case "chains":
			jobs = append(jobs, job{"chains", e.syncChains})
		case "pools":
			jobs = append(jobs, job{"pools", e.syncPools})
		case "stablecoins":
			jobs = append(jobs, job{"stablecoins", e.syncStablecoins})
		case "dexs":
			jobs = append(jobs, job{"dexs", e.syncDexs})
		case "fees":
			jobs = append(jobs, job{"fees", e.syncFees})
		case "options":
			jobs = append(jobs, job{"options", e.syncOptions})
		case "open_interest":
			jobs = append(jobs, job{"open_interest", e.syncOpenInterest})
		default:
			return fmt.Errorf("unknown domain %q", d)
		}
	}
	sem := make(chan struct{}, e.Parallel)
	var wg sync.WaitGroup
	errCh := make(chan error, len(jobs))
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			e.Report(j.name, "syncing")
			n, err := j.fn(ctx)
			if err != nil {
				e.Report(j.name, "error: "+err.Error())
				errCh <- fmt.Errorf("%s: %w", j.name, err)
				return
			}
			if err := e.S.SetSyncMeta(j.name, n, ""); err != nil {
				errCh <- fmt.Errorf("%s set meta: %w", j.name, err)
				return
			}
			e.Report(j.name, fmt.Sprintf("done (%d rows)", n))
		}(j)
	}
	wg.Wait()
	close(errCh)
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// ---------- helpers ----------

func numFloat(n json.Number) float64 {
	if n == "" {
		return 0
	}
	f, _ := n.Float64()
	return f
}

func rawNum(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case json.Number:
		return numFloat(x)
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case bool:
		if x {
			return 1
		}
		return 0
	}
	return 0
}

func rawStr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return string(x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func jsonStr(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(v)
}

// ---------- protocols ----------

func (e *Engine) syncProtocols(ctx context.Context) (int, error) {
	body, err := e.C.Get(ctx, client.HostAPI, "/protocols", nil)
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
		if _, err := tx.Exec(`DELETE FROM protocols`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM protocol_chain_tvl`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO protocols
			(id, slug, name, symbol, chain, category, tvl, mcap, change_1h, change_1d, change_7d, chains, url, description)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		chainStmt, err := tx.Prepare(`INSERT OR REPLACE INTO protocol_chain_tvl (protocol_slug, chain, tvl) VALUES (?,?,?)`)
		if err != nil {
			return err
		}
		defer chainStmt.Close()
		for _, p := range arr {
			slug, _ := p["slug"].(string)
			if slug == "" {
				continue
			}
			if _, err := stmt.Exec(
				rawStr(p["id"]),
				slug,
				rawStr(p["name"]),
				rawStr(p["symbol"]),
				rawStr(p["chain"]),
				rawStr(p["category"]),
				rawNum(p["tvl"]),
				rawNum(p["mcap"]),
				rawNum(p["change_1h"]),
				rawNum(p["change_1d"]),
				rawNum(p["change_7d"]),
				jsonStr(p["chains"]),
				rawStr(p["url"]),
				rawStr(p["description"]),
			); err != nil {
				return err
			}
			n++
			// Per-chain TVL breakdown.
			if ct, ok := p["chainTvls"].(map[string]any); ok {
				for chain, v := range ct {
					// Skip rollup keys like "Ethereum-borrowed", "staking", "pool2"; only keep
					// plain chain names (no dash).
					if strings.Contains(chain, "-") {
						continue
					}
					if chain == "staking" || chain == "pool2" || chain == "borrowed" || chain == "treasury" {
						continue
					}
					tvl := rawNum(v)
					if tvl <= 0 {
						continue
					}
					if _, err := chainStmt.Exec(slug, chain, tvl); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	return n, err
}

// ---------- chains ----------

func (e *Engine) syncChains(ctx context.Context) (int, error) {
	body, err := e.C.Get(ctx, client.HostAPI, "/v2/chains", nil)
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
		if _, err := tx.Exec(`DELETE FROM chains`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO chains (name, tvl, token_symbol, cmc_id, gecko_id) VALUES (?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, c := range arr {
			name, _ := c["name"].(string)
			if name == "" {
				continue
			}
			if _, err := stmt.Exec(
				name,
				rawNum(c["tvl"]),
				rawStr(c["tokenSymbol"]),
				rawStr(c["cmcId"]),
				rawStr(c["gecko_id"]),
			); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// ---------- pools ----------

func (e *Engine) syncPools(ctx context.Context) (int, error) {
	body, err := e.C.Get(ctx, client.HostYields, "/pools", nil)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	var resp struct {
		Status string           `json:"status"`
		Data   []map[string]any `json:"data"`
	}
	if err := decodeJSON(body, &resp); err != nil {
		return 0, err
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM pools`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO pools
			(pool_id, chain, project, symbol, tvl_usd, apy, apy_base, apy_reward, il_risk, stablecoin, exposure, pool_meta, underlying_tokens, reward_tokens)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range resp.Data {
			id, _ := p["pool"].(string)
			if id == "" {
				continue
			}
			stable := 0
			if b, ok := p["stablecoin"].(bool); ok && b {
				stable = 1
			}
			if _, err := stmt.Exec(
				id,
				rawStr(p["chain"]),
				rawStr(p["project"]),
				rawStr(p["symbol"]),
				rawNum(p["tvlUsd"]),
				rawNum(p["apy"]),
				rawNum(p["apyBase"]),
				rawNum(p["apyReward"]),
				rawStr(p["ilRisk"]),
				stable,
				rawStr(p["exposure"]),
				rawStr(p["poolMeta"]),
				jsonStr(p["underlyingTokens"]),
				jsonStr(p["rewardTokens"]),
			); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// ---------- stablecoins ----------

func (e *Engine) syncStablecoins(ctx context.Context) (int, error) {
	q := url.Values{}
	q.Set("includePrices", "true")
	body, err := e.C.Get(ctx, client.HostStablecoins, "/stablecoins", q)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	var resp struct {
		PeggedAssets []map[string]any `json:"peggedAssets"`
	}
	if err := decodeJSON(body, &resp); err != nil {
		return 0, err
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM stablecoins`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM stablecoin_chains`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO stablecoins
			(id, name, symbol, peg_type, peg_mechanism, circulating, price, mcap, chains) VALUES (?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		chStmt, err := tx.Prepare(`INSERT OR REPLACE INTO stablecoin_chains (stablecoin_id, chain, circulating) VALUES (?,?,?)`)
		if err != nil {
			return err
		}
		defer chStmt.Close()
		for _, p := range resp.PeggedAssets {
			id := rawStr(p["id"])
			if id == "" {
				continue
			}
			circ := 0.0
			if c, ok := p["circulating"].(map[string]any); ok {
				circ = rawNum(c["peggedUSD"])
				if circ == 0 {
					circ = rawNum(c["peggedVAR"])
				}
			}
			mcap := 0.0
			if pp, ok := p["price"]; ok {
				mcap = circ * rawNum(pp)
			}
			if _, err := stmt.Exec(
				id,
				rawStr(p["name"]),
				rawStr(p["symbol"]),
				rawStr(p["pegType"]),
				rawStr(p["pegMechanism"]),
				circ,
				rawNum(p["price"]),
				mcap,
				jsonStr(p["chains"]),
			); err != nil {
				return err
			}
			n++
			if cc, ok := p["chainCirculating"].(map[string]any); ok {
				for chain, v := range cc {
					vm, ok := v.(map[string]any)
					if !ok {
						continue
					}
					cur, _ := vm["current"].(map[string]any)
					if cur == nil {
						continue
					}
					amount := rawNum(cur["peggedUSD"])
					if amount == 0 {
						amount = rawNum(cur["peggedVAR"])
					}
					if _, err := chStmt.Exec(id, chain, amount); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	return n, err
}

// ---------- DEX volume ----------

func (e *Engine) syncDexs(ctx context.Context) (int, error) {
	q := url.Values{}
	q.Set("excludeTotalDataChart", "true")
	q.Set("excludeTotalDataChartBreakdown", "true")
	body, err := e.C.Get(ctx, client.HostAPI, "/overview/dexs", q)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	var resp struct {
		AllChains []string         `json:"allChains"`
		Protocols []map[string]any `json:"protocols"`
	}
	if err := decodeJSON(body, &resp); err != nil {
		return 0, err
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM dex_overview`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO dex_overview
			(protocol, display_name, total_24h, total_7d, total_30d, change_1d, change_7d, change_30d, chains) VALUES (?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range resp.Protocols {
			slug := slugFor(p)
			if slug == "" {
				continue
			}
			if _, err := stmt.Exec(
				slug,
				rawStr(p["displayName"]),
				rawNum(p["total24h"]),
				rawNum(p["total7d"]),
				rawNum(p["total30d"]),
				rawNum(p["change_1d"]),
				rawNum(p["change_7d"]),
				rawNum(p["change_30d"]),
				jsonStr(p["chains"]),
			); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	if err != nil {
		return n, err
	}
	// Per-chain DEX volume: hit /overview/dexs/{chain} for each chain reported.
	// Bubble the error so SyncDomains skips SetSyncMeta and the next staleness
	// check retries — otherwise a write failure leaves dex_chain_volume empty
	// while sync_meta records the domain as freshly synced.
	chains := resp.AllChains
	if len(chains) > 0 {
		if err := e.syncDexChainVolume(ctx, chains); err != nil {
			return n, fmt.Errorf("per-chain volume: %w", err)
		}
	}
	return n, nil
}

// syncDexChainVolume iterates chains in parallel and populates dex_chain_volume.
func (e *Engine) syncDexChainVolume(ctx context.Context, chains []string) error {
	if _, err := e.S.DB.Exec(`DELETE FROM dex_chain_volume`); err != nil {
		return err
	}
	type result struct {
		chain string
		rows  []map[string]any
		err   error
	}
	resCh := make(chan result, len(chains))
	sem := make(chan struct{}, e.Parallel)
	for _, c := range chains {
		sem <- struct{}{}
		go func(c string) {
			defer func() { <-sem }()
			q := url.Values{}
			q.Set("excludeTotalDataChart", "true")
			q.Set("excludeTotalDataChartBreakdown", "true")
			body, err := e.C.Get(ctx, client.HostAPI, "/overview/dexs/"+url.PathEscape(c), q)
			if err != nil {
				resCh <- result{chain: c, err: err}
				return
			}
			defer body.Close()
			var resp struct {
				Protocols []map[string]any `json:"protocols"`
			}
			if err := decodeJSON(body, &resp); err != nil {
				resCh <- result{chain: c, err: err}
				return
			}
			resCh <- result{chain: c, rows: resp.Protocols}
		}(c)
	}
	// Drain results. Surface per-chain fetch errors as reports (best-effort,
	// many chains have no DEXs) but bubble transaction errors -- a SQLite
	// failure during write would otherwise leave the table empty with no
	// signal to the caller.
	var firstWriteErr error
	for i := 0; i < len(chains); i++ {
		r := <-resCh
		if r.err != nil {
			e.Report("dexs", fmt.Sprintf("chain %s: %v", r.chain, r.err))
			continue
		}
		err := e.S.Tx(func(tx *sql.Tx) error {
			stmt, err := tx.Prepare(`INSERT OR REPLACE INTO dex_chain_volume
				(protocol, chain, total_24h, total_7d, total_30d) VALUES (?,?,?,?,?)`)
			if err != nil {
				return err
			}
			defer stmt.Close()
			for _, p := range r.rows {
				slug := slugFor(p)
				if slug == "" {
					continue
				}
				if _, err := stmt.Exec(
					slug, r.chain,
					rawNum(p["total24h"]),
					rawNum(p["total7d"]),
					rawNum(p["total30d"]),
				); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil && firstWriteErr == nil {
			firstWriteErr = fmt.Errorf("chain %s: %w", r.chain, err)
		}
	}
	return firstWriteErr
}

// ---------- Fees & Revenue ----------

func (e *Engine) syncFees(ctx context.Context) (int, error) {
	// /overview/fees with dataType=dailyFees and dailyRevenue, merged.
	fetch := func(dataType string) (map[string]map[string]any, error) {
		q := url.Values{}
		q.Set("excludeTotalDataChart", "true")
		q.Set("excludeTotalDataChartBreakdown", "true")
		q.Set("dataType", dataType)
		body, err := e.C.Get(ctx, client.HostAPI, "/overview/fees", q)
		if err != nil {
			return nil, err
		}
		defer body.Close()
		var resp struct {
			Protocols []map[string]any `json:"protocols"`
		}
		if err := decodeJSON(body, &resp); err != nil {
			return nil, err
		}
		m := make(map[string]map[string]any, len(resp.Protocols))
		for _, p := range resp.Protocols {
			slug := slugFor(p)
			if slug == "" {
				continue
			}
			m[slug] = p
		}
		return m, nil
	}
	feesMap, err := fetch("dailyFees")
	if err != nil {
		return 0, fmt.Errorf("dailyFees: %w", err)
	}
	revMap, err := fetch("dailyRevenue")
	if err != nil {
		return 0, fmt.Errorf("dailyRevenue: %w", err)
	}
	keys := make(map[string]struct{}, len(feesMap))
	for k := range feesMap {
		keys[k] = struct{}{}
	}
	for k := range revMap {
		keys[k] = struct{}{}
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM fees_overview`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO fees_overview
			(protocol, display_name, total_24h_fees, total_24h_rev, total_7d_fees, total_7d_rev, total_30d_fees, total_30d_rev, category, chains)
			VALUES (?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for slug := range keys {
			f := feesMap[slug]
			r := revMap[slug]
			display := ""
			category := ""
			chains := ""
			if f != nil {
				display = rawStr(f["displayName"])
				category = rawStr(f["category"])
				chains = jsonStr(f["chains"])
			} else if r != nil {
				display = rawStr(r["displayName"])
				category = rawStr(r["category"])
				chains = jsonStr(r["chains"])
			}
			fees24 := 0.0
			fees7 := 0.0
			fees30 := 0.0
			if f != nil {
				fees24 = rawNum(f["total24h"])
				fees7 = rawNum(f["total7d"])
				fees30 = rawNum(f["total30d"])
			}
			rev24 := 0.0
			rev7 := 0.0
			rev30 := 0.0
			if r != nil {
				rev24 = rawNum(r["total24h"])
				rev7 = rawNum(r["total7d"])
				rev30 = rawNum(r["total30d"])
			}
			if _, err := stmt.Exec(slug, display, fees24, rev24, fees7, rev7, fees30, rev30, category, chains); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// ---------- Options ----------

func (e *Engine) syncOptions(ctx context.Context) (int, error) {
	q := url.Values{}
	q.Set("excludeTotalDataChart", "true")
	q.Set("excludeTotalDataChartBreakdown", "true")
	body, err := e.C.Get(ctx, client.HostAPI, "/overview/options", q)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	var resp struct {
		Protocols []map[string]any `json:"protocols"`
	}
	if err := decodeJSON(body, &resp); err != nil {
		return 0, err
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM options_overview`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO options_overview
			(protocol, display_name, total_24h, total_7d, chains) VALUES (?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range resp.Protocols {
			slug := slugFor(p)
			if slug == "" {
				continue
			}
			if _, err := stmt.Exec(
				slug,
				rawStr(p["displayName"]),
				rawNum(p["total24h"]),
				rawNum(p["total7d"]),
				jsonStr(p["chains"]),
			); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// ---------- Open Interest ----------

func (e *Engine) syncOpenInterest(ctx context.Context) (int, error) {
	body, err := e.C.Get(ctx, client.HostAPI, "/overview/open-interest", nil)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	// Response shape varies; tolerate both {protocols: []} and [].
	all, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	var list []map[string]any
	var wrap struct {
		Protocols []map[string]any `json:"protocols"`
	}
	if err := decodeJSON(strings.NewReader(string(all)), &wrap); err == nil && len(wrap.Protocols) > 0 {
		list = wrap.Protocols
	} else {
		_ = decodeJSON(strings.NewReader(string(all)), &list)
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM open_interest`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO open_interest (protocol, display_name, total_oi, chains) VALUES (?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range list {
			slug := slugFor(p)
			if slug == "" {
				continue
			}
			// /overview/open-interest reports OI as total24h field.
			oi := rawNum(p["openInterestAtEnd"])
			if oi == 0 {
				oi = rawNum(p["total24h"])
			}
			if _, err := stmt.Exec(slug, rawStr(p["displayName"]), oi, jsonStr(p["chains"])); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// slugFor extracts a stable protocol slug from an overview-protocol object.
// DefiLlama uses lowercase slugs; field names vary across endpoints.
func slugFor(p map[string]any) string {
	for _, k := range []string{"slug", "moduleName", "module", "name"} {
		if v, ok := p[k].(string); ok && v != "" {
			return slugify(v)
		}
	}
	return ""
}

func slugify(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// Replace whitespace with dashes; leave alphanumerics, dashes, dots.
	b := make([]byte, 0, len(s))
	prevDash := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '.':
			b = append(b, c)
			prevDash = c == '-'
		case c == ' ' || c == '_':
			if !prevDash {
				b = append(b, '-')
				prevDash = true
			}
		}
	}
	return strings.Trim(string(b), "-")
}
