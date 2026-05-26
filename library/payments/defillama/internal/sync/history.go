// Historical backfill (P2.1) -- per-protocol TVL/fees/dex history, plus
// chain-level historical TVL.
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/client"
)

// SyncProtocolDetail fetches per-protocol TVL/fees/dex history.
// If backfill is false, only the last 90 days are kept; otherwise all-time.
func (e *Engine) SyncProtocolDetail(ctx context.Context, slug string, backfill bool) error {
	if err := e.syncProtocolTVLHist(ctx, slug, backfill); err != nil {
		return fmt.Errorf("tvl-hist: %w", err)
	}
	// Fees and DEX history are optional. Most protocols don't have one or both.
	// We silently skip 4xx responses ("no data for this protocol") and only
	// surface unexpected errors.
	if err := e.syncProtocolFeesHist(ctx, slug, backfill); err != nil {
		if !isNotApplicable(err) {
			e.Report(slug, "fees-hist failed: "+err.Error())
		}
	}
	if err := e.syncProtocolDexHist(ctx, slug, backfill); err != nil {
		if !isNotApplicable(err) {
			e.Report(slug, "dex-hist failed: "+err.Error())
		}
	}
	return nil
}

// isNotApplicable returns true when an error indicates the protocol simply
// has no data for that domain (4xx from the DefiLlama API, "no series" sentinels).
func isNotApplicable(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "status 400") ||
		strings.Contains(s, "status 404") ||
		strings.Contains(s, "no fees/revenue series") ||
		strings.Contains(s, "no dex volume series") ||
		strings.Contains(s, "please visit")
}

// SyncChainTVL fetches chain-level TVL history from /v2/historicalChainTvl/{chain}.
func (e *Engine) SyncChainTVL(ctx context.Context, chain string) error {
	path := "/v2/historicalChainTvl/" + url.PathEscape(chain)
	body, err := e.C.Get(ctx, client.HostAPI, path, nil)
	if err != nil {
		return err
	}
	defer body.Close()
	var arr []map[string]any
	if err := decodeJSON(body, &arr); err != nil {
		return err
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM chain_tvl_hist WHERE LOWER(chain) = LOWER(?)`, chain); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO chain_tvl_hist (chain, date, tvl) VALUES (?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, r := range arr {
			ts := rawNum(r["date"])
			tvl := rawNum(r["tvl"])
			if ts <= 0 {
				continue
			}
			date := time.Unix(int64(ts), 0).UTC().Format("2006-01-02")
			if _, err := stmt.Exec(chain, date, tvl); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	if err != nil {
		return err
	}
	return e.S.SetSyncMeta("chain_tvl_hist:"+strings.ToLower(chain), n, "")
}

// syncProtocolTVLHist hits /protocol/{slug} and stores the daily tvl series.
func (e *Engine) syncProtocolTVLHist(ctx context.Context, slug string, backfill bool) error {
	body, err := e.C.Get(ctx, client.HostAPI, "/protocol/"+url.PathEscape(slug), nil)
	if err != nil {
		return err
	}
	defer body.Close()
	var resp struct {
		TVL       []map[string]any            `json:"tvl"`
		ChainTvls map[string]struct {
			Tvl []map[string]any `json:"tvl"`
		} `json:"chainTvls"`
	}
	if err := decodeJSON(body, &resp); err != nil {
		return err
	}

	cutoff := ""
	if !backfill {
		cutoff = time.Now().AddDate(0, 0, -90).UTC().Format("2006-01-02")
	}

	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM protocol_tvl_hist WHERE protocol_slug = ?`, slug); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO protocol_tvl_hist (protocol_slug, date, tvl, chain) VALUES (?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		writeSeries := func(chain string, series []map[string]any) error {
			for _, p := range series {
				ts := rawNum(p["date"])
				tvl := rawNum(p["totalLiquidityUSD"])
				if tvl == 0 {
					tvl = rawNum(p["tvl"])
				}
				if ts <= 0 {
					continue
				}
				date := time.Unix(int64(ts), 0).UTC().Format("2006-01-02")
				if cutoff != "" && date < cutoff {
					continue
				}
				if _, err := stmt.Exec(slug, date, tvl, chain); err != nil {
					return err
				}
				n++
			}
			return nil
		}
		if err := writeSeries("", resp.TVL); err != nil {
			return err
		}
		// Skip per-chain rollup keys with hyphens (e.g. "Ethereum-borrowed").
		for chain, body := range resp.ChainTvls {
			if strings.Contains(chain, "-") {
				continue
			}
			if err := writeSeries(chain, body.Tvl); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return e.S.SetSyncMeta("protocol_tvl_hist:"+slug, n, "")
}

// syncProtocolFeesHist fetches both fee and revenue daily charts.
func (e *Engine) syncProtocolFeesHist(ctx context.Context, slug string, backfill bool) error {
	feeChart, err := fetchSummaryChart(ctx, e.C, "fees", slug, "dailyFees")
	if err != nil {
		return err
	}
	revChart, err := fetchSummaryChart(ctx, e.C, "fees", slug, "dailyRevenue")
	if err != nil {
		// revenue may not be reported; treat as empty
		revChart = nil
	}

	cutoff := ""
	if !backfill {
		cutoff = time.Now().AddDate(0, 0, -90).UTC().Format("2006-01-02")
	}

	byDate := map[string][2]float64{} // date -> {fees, revenue}
	for _, p := range feeChart {
		date, val := chartPoint(p)
		if date == "" {
			continue
		}
		if cutoff != "" && date < cutoff {
			continue
		}
		x := byDate[date]
		x[0] = val
		byDate[date] = x
	}
	for _, p := range revChart {
		date, val := chartPoint(p)
		if date == "" {
			continue
		}
		if cutoff != "" && date < cutoff {
			continue
		}
		x := byDate[date]
		x[1] = val
		byDate[date] = x
	}
	if len(byDate) == 0 {
		return fmt.Errorf("no fees/revenue series for %q", slug)
	}

	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM fees_hist WHERE protocol = ?`, slug); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO fees_hist (protocol, date, fees, revenue) VALUES (?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for date, vals := range byDate {
			if _, err := stmt.Exec(slug, date, vals[0], vals[1]); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	if err != nil {
		return err
	}
	return e.S.SetSyncMeta("fees_hist:"+slug, n, "")
}

func (e *Engine) syncProtocolDexHist(ctx context.Context, slug string, backfill bool) error {
	chart, err := fetchSummaryChart(ctx, e.C, "dexs", slug, "")
	if err != nil {
		return err
	}
	cutoff := ""
	if !backfill {
		cutoff = time.Now().AddDate(0, 0, -90).UTC().Format("2006-01-02")
	}
	if len(chart) == 0 {
		return fmt.Errorf("no dex volume series for %q", slug)
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM dex_hist WHERE protocol = ?`, slug); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO dex_hist (protocol, date, volume) VALUES (?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range chart {
			date, val := chartPoint(p)
			if date == "" {
				continue
			}
			if cutoff != "" && date < cutoff {
				continue
			}
			if _, err := stmt.Exec(slug, date, val); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	if err != nil {
		return err
	}
	return e.S.SetSyncMeta("dex_hist:"+slug, n, "")
}

// fetchSummaryChart hits /summary/{kind}/{slug} (optionally with dataType) and
// returns the totalDataChart array. The slice elements are either two-element
// arrays [timestamp, value] or objects {date, ...} -- chartPoint normalises.
func fetchSummaryChart(ctx context.Context, c *client.Client, kind, slug, dataType string) ([]any, error) {
	path := "/summary/" + kind + "/" + url.PathEscape(slug)
	q := url.Values{}
	q.Set("excludeTotalDataChartBreakdown", "true")
	if dataType != "" {
		q.Set("dataType", dataType)
	}
	body, err := c.Get(ctx, client.HostAPI, path, q)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	var resp struct {
		TotalDataChart []any `json:"totalDataChart"`
	}
	if err := decodeJSON(body, &resp); err != nil {
		return nil, err
	}
	return resp.TotalDataChart, nil
}

// chartPoint converts [timestamp, value] or {date, x} to a normalised date+value.
func chartPoint(p any) (string, float64) {
	switch x := p.(type) {
	case []any:
		if len(x) < 2 {
			return "", 0
		}
		ts := rawNum(x[0])
		v := rawNum(x[1])
		if ts <= 0 {
			return "", 0
		}
		return time.Unix(int64(ts), 0).UTC().Format("2006-01-02"), v
	case map[string]any:
		ts := rawNum(x["date"])
		if ts <= 0 {
			if s, ok := x["date"].(string); ok {
				if n, err := strconv.ParseInt(s, 10, 64); err == nil {
					ts = float64(n)
				}
			}
		}
		v := 0.0
		for _, k := range []string{"totalVolume", "volume", "fees", "value", "totalLiquidityUSD"} {
			if vv, ok := x[k]; ok {
				v = rawNum(vv)
				if v > 0 {
					break
				}
			}
		}
		if ts <= 0 {
			return "", v
		}
		return time.Unix(int64(ts), 0).UTC().Format("2006-01-02"), v
	}
	return "", 0
}

// SyncPoolChart fetches per-pool TVL/APY history from yields.llama.fi/chart/{pool}.
func (e *Engine) SyncPoolChart(ctx context.Context, poolID string) error {
	body, err := e.C.Get(ctx, client.HostYields, "/chart/"+url.PathEscape(poolID), nil)
	if err != nil {
		return err
	}
	defer body.Close()
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := decodeJSON(body, &resp); err != nil {
		return err
	}
	if len(resp.Data) == 0 {
		return fmt.Errorf("no chart data for pool %q", poolID)
	}
	n := 0
	err = e.S.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM pool_hist WHERE pool_id = ?`, poolID); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO pool_hist (pool_id, date, tvl, apy) VALUES (?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range resp.Data {
			ts := rawStr(p["timestamp"])
			if ts == "" {
				continue
			}
			// timestamp is ISO 8601 like "2024-01-01T00:00:00.000Z"
			t, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				continue
			}
			date := t.UTC().Format("2006-01-02")
			tvl := rawNum(p["tvlUsd"])
			apy := rawNum(p["apy"])
			if _, err := stmt.Exec(poolID, date, tvl, apy); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	if err != nil {
		return err
	}
	return e.S.SetSyncMeta("pool_hist:"+poolID, n, "")
}

// Generic JSON decode helper not exported in sync.go because it's defined there.
// We get it via the package internal helpers.
var _ = json.Decoder{}
