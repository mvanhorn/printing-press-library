// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/store"
	"github.com/spf13/cobra"
)

// Shared data layer for the hand-written InvestorPaisa commands.
//
// Every command reads one of three site snapshots: the 48-metric screener
// table, the intraday live snapshot, and the market overview. Fetching a
// snapshot live also stores it in the local ip_snapshots table (one row per
// kind and as-of date), which is what lets `changes` and `breadth` look back
// at days the site itself no longer shows.

const (
	ipKindScreener = "screener"
	ipKindLive     = "live"
	ipKindOverview = "overview"
)

type ipRow = map[string]any

type ipSnapshot struct {
	Kind      string
	AsOf      string
	FetchedAt string
	Body      json.RawMessage
}

// ipSource labels where a command's data came from.
type ipSource struct {
	Source string `json:"source"` // live | local
	AsOf   string `json:"as_of,omitempty"`
}

func ipNum(r ipRow, k string) (float64, bool) {
	v, ok := r[k]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) {
		return 0, false
	}
	return f, true
}

func ipStrField(r ipRow, k string) string {
	s, _ := r[k].(string)
	return s
}

func ipDBPath(dbPath string) string {
	if dbPath != "" {
		return dbPath
	}
	return defaultDBPath("investorpaisa-pp-cli")
}

func ipOpenStore(ctx context.Context, dbPath string) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, ipDBPath(dbPath))
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := ipEnsureSnapshotTable(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ipEnsureSnapshotTable(ctx context.Context, db *store.Store) error {
	if _, err := db.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS ip_snapshots (
		kind TEXT NOT NULL,
		asof TEXT NOT NULL,
		fetched_at TEXT NOT NULL,
		body TEXT NOT NULL,
		PRIMARY KEY (kind, asof)
	)`); err != nil {
		return fmt.Errorf("preparing snapshot table: %w", err)
	}
	return nil
}

// ipRecordSnapshots is called after `sync`: it stores today's screener, live
// and overview snapshots so changes/breadth/sectors --trend have history.
// Failures only warn; sync itself already succeeded.
func ipRecordSnapshots(ctx context.Context, cmd *cobra.Command, flags *rootFlags, db *store.Store) {
	if err := ipEnsureSnapshotTable(ctx, db); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
		return
	}
	for _, kind := range []string{ipKindScreener, ipKindLive, ipKindOverview} {
		body, err := ipFetchLive(ctx, flags, kind)
		if err == nil {
			_, err = ipValidateSnapshot(kind, body)
		}
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not snapshot %s: %v\n", kind, err)
			continue
		}
		if err := ipSaveSnapshot(ctx, db, kind, ipSnapshotAsOf(kind, body), body); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not store %s snapshot: %v\n", kind, err)
		}
	}
}

func ipSaveSnapshot(ctx context.Context, db *store.Store, kind, asof string, body json.RawMessage) error {
	if asof == "" {
		return fmt.Errorf("refusing to store a %s snapshot with no as-of date", kind)
	}
	_, err := db.DB().ExecContext(ctx,
		`INSERT OR REPLACE INTO ip_snapshots (kind, asof, fetched_at, body) VALUES (?, ?, ?, ?)`,
		kind, asof, time.Now().UTC().Format(time.RFC3339), string(body))
	if err != nil {
		return err
	}
	if strings.HasPrefix(kind, "history:") {
		return nil
	}
	// Keep roughly two years of daily snapshots per kind.
	_, err = db.DB().ExecContext(ctx, `DELETE FROM ip_snapshots WHERE kind = ? AND asof NOT IN (
		SELECT asof FROM ip_snapshots WHERE kind = ? ORDER BY asof DESC LIMIT 500)`, kind, kind)
	return err
}

// ipLatestSnapshots returns up to n snapshots of a kind, newest as-of first.
func ipLatestSnapshots(ctx context.Context, db *store.Store, kind string, n int) ([]ipSnapshot, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT kind, asof, fetched_at, body FROM ip_snapshots WHERE kind = ? ORDER BY asof DESC LIMIT ?`, kind, n)
	if err != nil {
		return nil, err
	}
	var out []ipSnapshot
	for rows.Next() {
		var s ipSnapshot
		var body string
		if err := rows.Scan(&s.Kind, &s.AsOf, &s.FetchedAt, &body); err != nil {
			_ = rows.Close()
			return nil, err
		}
		s.Body = json.RawMessage(body)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

// ipValidateSnapshot checks that a fetched body really is the expected kind of
// snapshot (an error envelope or empty list must never be stored as one) and
// returns its as-of date.
func ipValidateSnapshot(kind string, body json.RawMessage) (string, error) {
	asof := ipSnapshotAsOf(kind, body)
	switch kind {
	case ipKindScreener:
		rows, err := ipParseRows(body)
		if err != nil || len(rows) == 0 {
			return "", fmt.Errorf("screener response has no rows")
		}
	case ipKindLive:
		var lv ipLive
		if err := json.Unmarshal(body, &lv); err != nil || len(lv.Payload.Rows) == 0 {
			return "", fmt.Errorf("live response has no rows")
		}
	case ipKindOverview:
		var o map[string]json.RawMessage
		if err := json.Unmarshal(body, &o); err != nil || o["asOf"] == nil {
			return "", fmt.Errorf("overview response is not the expected object")
		}
	}
	if asof == "" {
		return "", fmt.Errorf("%s response has no as-of date", kind)
	}
	return asof, nil
}

func ipSnapshotAsOf(kind string, body json.RawMessage) string {
	switch kind {
	case ipKindScreener:
		var rows []ipRow
		if json.Unmarshal(body, &rows) == nil && len(rows) > 0 {
			return ipStrField(rows[0], "date")
		}
	case ipKindOverview:
		var o struct {
			AsOf string `json:"asOf"`
		}
		if json.Unmarshal(body, &o) == nil {
			return o.AsOf
		}
	case ipKindLive:
		var l struct {
			Payload struct {
				SessionDate string `json:"sessionDate"`
			} `json:"payload"`
		}
		if json.Unmarshal(body, &l) == nil {
			return l.Payload.SessionDate
		}
	}
	return ""
}

// ipLoader fetches snapshots honouring --data-source (auto: live with local
// fallback, live: API only, local: stored snapshots only).
type ipLoader struct {
	cmd    *cobra.Command
	flags  *rootFlags
	dbPath string
	db     *store.Store
	memo   map[string]ipMemo
}

type ipMemo struct {
	body json.RawMessage
	src  ipSource
}

func newIPLoader(cmd *cobra.Command, flags *rootFlags, dbPath string) *ipLoader {
	return &ipLoader{cmd: cmd, flags: flags, dbPath: dbPath}
}

func (l *ipLoader) close() {
	if l.db != nil {
		_ = l.db.Close()
	}
}

func (l *ipLoader) store(ctx context.Context) (*store.Store, error) {
	if l.db != nil {
		return l.db, nil
	}
	db, err := ipOpenStore(ctx, l.dbPath)
	if err != nil {
		return nil, err
	}
	l.db = db
	return db, nil
}

var errIPNoSnapshot = errors.New("no stored snapshot")

func ipPathFor(kind string) string {
	switch kind {
	case ipKindScreener:
		return "/api/screener/rows"
	case ipKindLive:
		return "/api/live"
	default:
		return "/api/market/overview"
	}
}

// snapshot returns the raw snapshot body and where it came from.
func (l *ipLoader) snapshot(ctx context.Context, kind string) (json.RawMessage, ipSource, error) {
	if m, ok := l.memo[kind]; ok {
		return m.body, m.src, nil
	}
	body, src, err := l.snapshotUncached(ctx, kind)
	if err == nil {
		if l.memo == nil {
			l.memo = map[string]ipMemo{}
		}
		l.memo[kind] = ipMemo{body: body, src: src}
	}
	return body, src, err
}

func (l *ipLoader) snapshotUncached(ctx context.Context, kind string) (json.RawMessage, ipSource, error) {
	mode := l.flags.dataSource
	if mode == "" {
		mode = "auto"
	}
	if mode == "local" {
		return l.local(ctx, kind)
	}
	body, err := ipFetchLive(ctx, l.flags, kind)
	var asof string
	if err == nil {
		asof, err = ipValidateSnapshot(kind, body)
		if err != nil {
			err = fmt.Errorf("unexpected %s response: %w", kind, err)
		}
	}
	if err == nil {
		if db, derr := l.store(ctx); derr == nil {
			if serr := ipSaveSnapshot(ctx, db, kind, asof, body); serr != nil {
				fmt.Fprintf(l.cmd.ErrOrStderr(), "warning: could not store %s snapshot: %v\n", kind, serr)
			}
		} else {
			fmt.Fprintf(l.cmd.ErrOrStderr(), "warning: could not open the local store: %v\n", derr)
		}
		return body, ipSource{Source: "live", AsOf: asof}, nil
	}
	if mode == "auto" && !errors.Is(err, errIPAuth) {
		if b, src, lerr := l.local(ctx, kind); lerr == nil {
			fmt.Fprintf(l.cmd.ErrOrStderr(), "warning: live fetch of %s failed (%v); using the stored snapshot from %s\n", kind, err, src.AsOf)
			return b, src, nil
		}
	}
	return nil, ipSource{}, err
}

var errIPAuth = errors.New("not signed in")

// ipFetchLive fetches one snapshot body from the API, bypassing the HTTP cache.
func ipFetchLive(ctx context.Context, flags *rootFlags, kind string) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.GetNoCache(ctx, ipPathFor(kind), nil)
	if err != nil {
		cerr := classifyAPIErrorOnly(err)
		var apiErr *client.APIError
		if (errors.As(err, &apiErr) && (apiErr.StatusCode == 401 || apiErr.StatusCode == 403)) ||
			strings.Contains(err.Error(), "returned HTML instead of JSON") {
			return nil, fmt.Errorf("%w: %w", errIPAuth, cerr)
		}
		return nil, cerr
	}
	return data, nil
}

func (l *ipLoader) local(ctx context.Context, kind string) (json.RawMessage, ipSource, error) {
	db, err := l.store(ctx)
	if err != nil {
		return nil, ipSource{}, err
	}
	snaps, err := ipLatestSnapshots(ctx, db, kind, 1)
	if err != nil {
		return nil, ipSource{}, err
	}
	if len(snaps) == 0 {
		return nil, ipSource{}, fmt.Errorf("%w for %s; run 'investorpaisa-pp-cli sync' while signed in", errIPNoSnapshot, kind)
	}
	return snaps[0].Body, ipSource{Source: "local", AsOf: snaps[0].AsOf}, nil
}

func (l *ipLoader) screenerRows(ctx context.Context) ([]ipRow, ipSource, error) {
	body, src, err := l.snapshot(ctx, ipKindScreener)
	if err != nil {
		return nil, src, err
	}
	rows, err := ipParseRows(body)
	return rows, src, err
}

func ipParseRows(body json.RawMessage) ([]ipRow, error) {
	var rows []ipRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("unexpected screener response: %w", err)
	}
	return rows, nil
}

// ipLive is the typed view of /api/live.
type ipLive struct {
	Payload struct {
		Agg struct {
			N          int     `json:"n"`
			Up         int     `json:"up"`
			Down       int     `json:"down"`
			Avg        float64 `json:"avg"`
			Median     float64 `json:"median"`
			Hi52       int     `json:"hi52"`
			Lo52       int     `json:"lo52"`
			Above200   float64 `json:"above200"`
			ValueCr    float64 `json:"valueCr"`
			UnusualVol int     `json:"unusualVolume"`
		} `json:"agg"`
		Rows        []ipRow                 `json:"rows"`
		Indices     map[string]ipIndexLevel `json:"indices"`
		LastTrade   string                  `json:"lastTrade"`
		SessionDate string                  `json:"sessionDate"`
	} `json:"payload"`
	FetchedAt string `json:"fetchedAt"`
}

type ipIndexLevel struct {
	Level     float64 `json:"level"`
	Chg       float64 `json:"chg"`
	ChgPct    float64 `json:"chgPct"`
	PrevClose float64 `json:"prevClose"`
}

func (l *ipLoader) liveSnapshot(ctx context.Context) (*ipLive, ipSource, error) {
	body, src, err := l.snapshot(ctx, ipKindLive)
	if err != nil {
		return nil, src, err
	}
	var lv ipLive
	if err := json.Unmarshal(body, &lv); err != nil {
		return nil, src, fmt.Errorf("unexpected live response: %w", err)
	}
	return &lv, src, nil
}

type ipSignal struct {
	Type   string `json:"type,omitempty"`
	Tone   string `json:"tone"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
	Weight int    `json:"weight,omitempty"`
}

// ipRowSignals extracts the per-row signal list from a /api/live row.
func ipRowSignals(r ipRow) []ipSignal {
	arr, _ := r["signals"].([]any)
	out := make([]ipSignal, 0, len(arr))
	for _, it := range arr {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		s := ipSignal{Tone: ipStrField(m, "tone"), Label: ipStrField(m, "label")}
		if t := ipStrField(m, "t"); t != "" {
			s.Type = t
		} else {
			s.Type = ipStrField(m, "type")
		}
		if d := ipStrField(m, "why"); d != "" {
			s.Detail = d
		} else {
			s.Detail = ipStrField(m, "detail")
		}
		if w, ok := m["weight"].(float64); ok {
			s.Weight = int(w)
		}
		out = append(out, s)
	}
	return out
}

func ipRound(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

func ipMean(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	var s float64
	for _, v := range vs {
		s += v
	}
	return s / float64(len(vs))
}

func ipMedian(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	c := append([]float64(nil), vs...)
	sort.Float64s(c)
	m := len(c) / 2
	if len(c)%2 == 1 {
		return c[m]
	}
	return (c[m-1] + c[m]) / 2
}

// ipPercentile returns the percent of values strictly below v (0-100).
func ipPercentile(vs []float64, v float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	below := 0
	for _, x := range vs {
		if x < v {
			below++
		}
	}
	return ipRound(float64(below)/float64(len(vs))*100, 0)
}

var ipSymbolRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9&_.-]{0,19}$`)

func ipNormalizeSymbol(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// ipValidSymbol reports whether s looks like an NSE symbol (e.g. TCS, M&M, BAJAJ-AUTO).
func ipValidSymbol(s string) bool {
	return ipSymbolRe.MatchString(s) && s != "." && s != ".."
}

// normalizeSessionCookie turns a pasted value into a cookie string. A bare
// value is taken to be the Auth.js session cookie; a value that starts with a
// cookie name and "=" is assumed to already be a "name=value; name=value"
// string. Newlines and a leading "Cookie:" header name are stripped.
func normalizeSessionCookie(v string) string {
	v = strings.Join(strings.Fields(v), " ")
	if len(v) >= 7 && strings.EqualFold(v[:7], "cookie:") {
		v = strings.TrimSpace(v[7:])
	}
	if v == "" {
		return ""
	}
	if ipCookieStartRe.MatchString(v) {
		return v
	}
	return "__Secure-authjs.session-token=" + v
}

var ipCookieStartRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*=[^=]`)

func init() {
	registerClientHook(ipSeedSessionCookie)
}

// ipSeedSessionCookie makes the env-var / set-token session work. The
// generated client only seeds its cookie jar when the first cookie name has no
// "." in it (to tell cookie strings from JWTs), but the Auth.js session cookie
// is named __Secure-authjs.session-token. Seed it here, with the same guard:
// only an https investorpaisa.com base URL ever receives the cookie.
func ipSeedSessionCookie(c *client.Client) error {
	if c == nil || c.Config == nil || c.HTTPClient == nil || c.HTTPClient.Jar == nil {
		return nil
	}
	raw := normalizeSessionCookie(c.Config.CookieCredential())
	if raw == "" {
		return nil
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	if host != "investorpaisa.com" && !strings.HasSuffix(host, ".investorpaisa.com") {
		return nil
	}
	req := &http.Request{Header: http.Header{"Cookie": []string{raw}}}
	cookies := req.Cookies()
	for _, ck := range cookies {
		ck.Domain = "investorpaisa.com"
		ck.Path = "/"
		ck.Secure = true
	}
	if len(cookies) > 0 {
		c.HTTPClient.Jar.SetCookies(u, cookies)
	}
	return nil
}

func ipScrubList(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = cliutil.ScrubTerminal(s)
	}
	return out
}
