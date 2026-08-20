// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Peekaboo extensions: zero-config guest-token bootstrap,
// city->coordinate resolution, Google Maps directions helpers, and the shared
// per-city deal fan-out used by the novel transcendence commands. This file is
// preserved across `generate --force` (regen-merge keeps hand-authored files).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/peekaboo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/peekaboo/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// normalizeWireFlags lets a command accept the upstream camelCase wire keys
// (entityId, targetEntityId) as aliases for the kebab-case CLI flags
// (--entity-id, --target-entity-id). Peekaboo's API requires the camelCase
// body keys, so agents and tools that mirror the wire shape can pass either
// spelling. Apply with cmd.SetGlobalNormalizationFunc(normalizeWireFlags).
func normalizeWireFlags(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "entityId":
		name = "entity-id"
	case "targetEntityId":
		name = "target-entity-id"
	}
	return pflag.NormalizedName(name)
}

const peekabooChromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// peekabooTokenPages are candidate pages whose server-rendered HTML embeds the
// public guest token (window.__guest__). The token is a permanent, public,
// role=guest credential served to every visitor; the CLI scrapes it once and
// caches it so every command works with zero configuration. The token VALUE is
// never written to source — only fetched at runtime.
var peekabooTokenPages = []string{
	"https://peekaboo.guru/",
	"https://peekaboo.guru/lahore/places/1/food",
}

// fetchGuestToken downloads a Peekaboo page and extracts window.__guest__.token
// from the server-rendered HTML.
func fetchGuestToken(ctx context.Context, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}
	var lastErr error
	for _, page := range peekabooTokenPages {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", peekabooChromeUA)
		req.Header.Set("Accept", "text/html")
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close() // body fully read above; close error is not actionable
		if err != nil {
			lastErr = err
			continue
		}
		if token := extractGuestToken(string(body)); token != "" {
			return token, nil
		}
		lastErr = fmt.Errorf("no guest token found in %s", page)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("guest token not found")
	}
	return "", lastErr
}

// extractGuestToken pulls the token out of `window.__guest__={...}` embedded in
// a page's HTML. String extraction (not regex) so the nested `preferences:{}`
// object doesn't confuse brace matching.
func extractGuestToken(html string) string {
	const marker = "window.__guest__="
	idx := strings.Index(html, marker)
	if idx < 0 {
		return ""
	}
	rest := html[idx+len(marker):]
	end := strings.Index(rest, "</script>")
	if end < 0 {
		return ""
	}
	blob := strings.TrimSpace(rest[:end])
	var guest struct {
		Token string `json:"token"`
	}
	// Decode a single JSON value and tolerate any trailing JS (e.g. a trailing
	// semicolon or a second assignment) in the same <script> block.
	if err := json.NewDecoder(strings.NewReader(blob)).Decode(&guest); err != nil {
		return ""
	}
	return guest.Token
}

// ensureGuestToken guarantees a usable bearer token before a command that hits
// an authenticated endpoint. If a token is already resolvable (env var or
// stored credentials) it is a no-op. Otherwise it scrapes the public guest
// token, exports it for the current process, and persists it so future runs
// (including generated endpoint commands) work without re-fetching.
//
// It is a no-op under the Printing Press mock verifier so a verify pass never
// makes a live network call.
func ensureGuestToken(parent context.Context, flags *rootFlags) error {
	if cliutil.IsVerifyEnv() {
		return nil
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	if cfg.AuthHeader() != "" {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	token, err := fetchGuestToken(ctx, flags.timeout)
	if err != nil {
		return fmt.Errorf("bootstrapping Peekaboo guest token: %w (set PEEKABOO_TOKEN manually or run 'peekaboo-pp-cli bootstrap')", err)
	}
	// Make it visible to the config.Load inside flags.newClient() for the rest
	// of this process, and persist it for future invocations.
	_ = os.Setenv("PEEKABOO_TOKEN", token)
	if saveErr := cfg.SaveTokens("", "", token, "", cfg.TokenExpiry); saveErr != nil {
		// Non-fatal: the env override above still authenticates this run.
		fmt.Fprintf(os.Stderr, "warning: could not persist guest token: %v\n", saveErr)
	}
	return nil
}

// EnsureGuestToken makes the CLI's zero-config guest-token contract available
// to the companion MCP server. It is intentionally a no-op when credentials
// already exist, and it keeps verify-mode runs free of live network calls.
func EnsureGuestToken(ctx context.Context, configPath string, timeout time.Duration) error {
	return ensureGuestToken(ctx, &rootFlags{configPath: configPath, timeout: timeout})
}

// authSkipCommands are command path segments that never need the guest token
// (they are local, public, or metadata commands). For everything else the root
// pre-run ensures a token so absorbed endpoint commands work zero-config on a
// fresh install, not just the novel commands that call ensureGuestToken
// directly.
var authSkipCommands = map[string]struct{}{
	"help": {}, "version": {}, "completion": {}, "doctor": {}, "auth": {},
	"bootstrap": {}, "agent-context": {}, "which": {}, "api": {}, "profile": {},
	"feedback": {}, "locations": {}, "teach": {}, "teach-pattern": {},
	"teach-lookup": {}, "teach-playbook": {}, "playbook": {}, "recall": {},
	"learnings": {}, "workflow": {},
}

// maybeEnsureGuestToken bootstraps the guest token before any command that hits
// an authenticated endpoint. It is a no-op for local/public/metadata commands,
// under --dry-run, and when a token is already available. Failures are
// non-fatal here: the command's own error handling surfaces a 401 with a hint
// if the bootstrap could not run.
func maybeEnsureGuestToken(cmd *cobra.Command, flags *rootFlags) {
	if flags.dryRun {
		return
	}
	for _, seg := range strings.Fields(cmd.CommandPath()) {
		if _, skip := authSkipCommands[seg]; skip {
			return
		}
	}
	_ = ensureGuestToken(cmd.Context(), flags)
}

// newBootstrapCmd force-fetches and persists the public guest token. Users
// normally never need to run this — every command auto-bootstraps — but it is
// handy for refreshing a stale/cleared token or verifying connectivity.
func newBootstrapCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "bootstrap",
		Short:       "Fetch and cache Peekaboo's public guest token (zero-config auth)",
		Long:        "Fetch Peekaboo's public guest token from the site and cache it locally so authenticated commands work. Runs automatically on first use; run it manually to refresh a cleared or stale token.",
		Example:     "  peekaboo-pp-cli bootstrap",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and cache the Peekaboo guest token")
				return nil
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 25*time.Second)
			defer cancel()
			token, err := fetchGuestToken(ctx, flags.timeout)
			if err != nil {
				return fmt.Errorf("fetching guest token: %w", err)
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.AuthHeaderVal = ""
			if err := cfg.SaveTokens("", "", token, "", cfg.TokenExpiry); err != nil {
				return configErr(fmt.Errorf("saving guest token: %w", err))
			}
			_ = os.Setenv("PEEKABOO_TOKEN", token)
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"bootstrapped": true, "config_path": cfg.Path})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Guest token cached. Peekaboo commands are ready.")
			return nil
		},
	}
	return cmd
}

// ---- Geo + city resolution ----------------------------------------------

type pkbLocation struct {
	ID        int     `json:"id"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// resolveCity looks up a city by name via the public locations endpoint and
// returns its coordinates and country. Match is case-insensitive; the largest
// (highest entityCount, i.e. first returned) match wins on ties.
func resolveCity(ctx context.Context, flags *rootFlags, city string) (pkbLocation, error) {
	var zero pkbLocation
	city = strings.TrimSpace(city)
	if city == "" {
		return zero, fmt.Errorf("city is required")
	}
	c, err := flags.newClient()
	if err != nil {
		return zero, err
	}
	data, _, err := c.Post(ctx, "/v5/locations", map[string]any{"limit": 1000, "offset": 0})
	if err != nil {
		return zero, fmt.Errorf("fetching locations: %w", err)
	}
	var wrap struct {
		Locations []pkbLocation `json:"locations"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return zero, fmt.Errorf("parsing locations: %w", err)
	}
	want := strings.ToLower(city)
	for _, l := range wrap.Locations {
		if strings.ToLower(l.City) == want {
			return l, nil
		}
	}
	// Fall back to a contains match (e.g. "islamabad" vs "Islamabad ").
	for _, l := range wrap.Locations {
		if strings.Contains(strings.ToLower(l.City), want) {
			return l, nil
		}
	}
	return zero, fmt.Errorf("city %q not found; run 'peekaboo-pp-cli locations list' to see available cities", city)
}

// resolveEntity turns a user-supplied merchant reference into an entity ID.
// A numeric reference is used directly. A name/slug is resolved by scanning the
// city+category merchant listing (requires category > 0). Returns the resolved
// id and its display name.
func resolveEntity(ctx context.Context, flags *rootFlags, merchant string, loc pkbLocation, category, maxScanPages int) (int, string, error) {
	merchant = strings.TrimSpace(merchant)
	if merchant == "" {
		return 0, "", fmt.Errorf("merchant is required (a numeric entity id from 'places list', or a name with --category)")
	}
	if id, err := strconv.Atoi(merchant); err == nil {
		return id, merchant, nil
	}
	if category <= 0 {
		return 0, "", fmt.Errorf("cannot resolve merchant name %q without --category; pass a numeric entity id (see 'peekaboo-pp-cli places list') or add --category", merchant)
	}
	if maxScanPages <= 0 {
		maxScanPages = 5
	}
	entities, _, err := listCityEntities(ctx, flags, loc, category, maxScanPages, 50)
	if err != nil {
		return 0, "", fmt.Errorf("resolving merchant %q: %w", merchant, err)
	}
	want := strings.ToLower(merchant)
	for _, e := range entities {
		if strings.EqualFold(e.Name, merchant) || strings.EqualFold(e.Slug, merchant) {
			return e.ID, e.Name, nil
		}
	}
	for _, e := range entities {
		if strings.Contains(strings.ToLower(e.Name), want) || strings.Contains(strings.ToLower(e.Slug), want) {
			return e.ID, e.Name, nil
		}
	}
	return 0, "", fmt.Errorf("merchant %q not found in %s under that category; try a numeric entity id from 'peekaboo-pp-cli places list'", merchant, loc.City)
}

// mapsDirectionsURL builds the Google Maps directions URL that Peekaboo's
// per-branch "Direction" button opens.
func mapsDirectionsURL(lat, long float64) string {
	return fmt.Sprintf("https://www.google.com/maps?daddr=%s,%s",
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(long, 'f', -1, 64))
}

// haversineKm returns the great-circle distance in kilometers.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// ---- Shared entity + deal types ------------------------------------------

type pkbBranch struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	City          string  `json:"city"`
	Country       string  `json:"country"`
	Address       string  `json:"address"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Distance      string  `json:"distance"`
	Timings       string  `json:"timings"`
	ContactNumber string  `json:"contactNumber"`
	BranchOpenNow string  `json:"branchOpenNow"`
}

type pkbEntity struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Slug    string  `json:"slug"`
	OpenNow bool    `json:"openNow"`
	Rating  float64 `json:"rating"`
	Stats   struct {
		Branches int `json:"branches"`
	} `json:"stats"`
}

type pkbDeal struct {
	DealID           int    `json:"dealId"`
	Title            string `json:"title"`
	PercentageValue  int    `json:"percentageValue"`
	StartDate        string `json:"startDate"`
	EndDate          string `json:"endDate"`
	OrderType        string `json:"orderType"`
	Description      string `json:"description"`
	TargetEntityID   int    `json:"targetEntityId"`
	TargetEntityName string `json:"targetEntityName"`
	SourceEntityID   int    `json:"sourceEntityId"`
	SourceEntityName string `json:"sourceEntityName"`
}

// pkbFetchFailure records a merchant whose deal fetch failed during fan-out so
// aggregates never silently count phantom zeros.
type pkbFetchFailure struct {
	EntityID   int    `json:"entity_id"`
	EntityName string `json:"entity_name"`
	Error      string `json:"error"`
}

// fetchBranches returns all branches of a merchant in a given city, with the
// city's coordinates supplying the user-location query params the endpoint
// requires. The `entity` name query param is an SEO slug the endpoint ignores
// for data, so a placeholder is fine.
func fetchBranches(ctx context.Context, flags *rootFlags, entityID string, loc pkbLocation, limit int) ([]pkbBranch, string, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, "", err
	}
	if limit <= 0 {
		limit = 100
	}
	params := map[string]string{
		"city":     loc.City,
		"country":  loc.Country,
		"entity":   "x",
		"language": "en",
		"lat":      strconv.FormatFloat(loc.Latitude, 'f', -1, 64),
		"long":     strconv.FormatFloat(loc.Longitude, 'f', -1, 64),
		"limit":    strconv.Itoa(limit),
		"offset":   "0",
	}
	path := "/v5/entity/" + entityID + "/branch/_all"
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, "", fmt.Errorf("fetching branches for entity %s: %w", entityID, err)
	}
	var wrap struct {
		Name     string      `json:"name"`
		Branches []pkbBranch `json:"branches"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, "", fmt.Errorf("parsing branches: %w", err)
	}
	return wrap.Branches, wrap.Name, nil
}

// listCityEntities pages through merchants for a city+category, up to
// maxScanPages of pageSize each.
func listCityEntities(ctx context.Context, flags *rootFlags, loc pkbLocation, category, maxScanPages, pageSize int) ([]pkbEntity, int, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, 0, err
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	var all []pkbEntity
	scanned := 0
	for page := 0; page < maxScanPages; page++ {
		// sortType "trending" surfaces deal-bearing merchants first (the site's
		// default). Without it the listing is alphabetical and the long tail of
		// deal-less merchants floods the first pages.
		body := map[string]any{
			"sortType":       "trending",
			"targetEntities": "_all",
			"country":        loc.Country,
			"city":           loc.City,
			"categoryId":     strconv.Itoa(category),
			"lat":            loc.Latitude,
			"long":           loc.Longitude,
			"language":       "en",
			"limit":          pageSize,
			"offset":         page * pageSize,
			"sourceEntityId": "_all",
			"atlId":          "_all",
		}
		data, _, err := c.Post(ctx, "/v8/entities", body)
		if err != nil {
			return all, scanned, fmt.Errorf("fetching merchants page %d: %w", page+1, err)
		}
		var wrap struct {
			NextPage bool        `json:"nextPage"`
			Entities []pkbEntity `json:"entities"`
		}
		if err := json.Unmarshal(data, &wrap); err != nil {
			return all, scanned, fmt.Errorf("parsing merchants page %d: %w", page+1, err)
		}
		if len(wrap.Entities) == 0 {
			break
		}
		all = append(all, wrap.Entities...)
		scanned += len(wrap.Entities)
		if !wrap.NextPage {
			break
		}
	}
	return all, scanned, nil
}

// fetchEntityDeals returns the deals for one merchant.
func fetchEntityDeals(ctx context.Context, flags *rootFlags, loc pkbLocation, entityID int) ([]pkbDeal, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"targetEntityId":  entityID,
		"city":            loc.City,
		"country":         loc.Country,
		"lat":             loc.Latitude,
		"long":            loc.Longitude,
		"language":        "en",
		"offset":          0,
		"limit":           100,
		"associatedDeals": true,
		"card":            "All",
	}
	data, _, err := c.Post(ctx, "/v8/entity/deals", body)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Deals []pkbDeal `json:"deals"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("parsing deals: %w", err)
	}
	return wrap.Deals, nil
}

// dealWithMerchant pairs a deal with the merchant it belongs to.
type dealWithMerchant struct {
	pkbDeal
	MerchantID   int    `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
}

// fanOutCityDeals lists merchants for a city+category and fetches every
// merchant's deals concurrently, preserving per-merchant fetch errors so
// aggregates exclude failures. maxMerchants caps the fan-out width; under live
// dogfood the caller should curtail it.
func fanOutCityDeals(ctx context.Context, flags *rootFlags, loc pkbLocation, category, maxScanPages, maxMerchants int) ([]dealWithMerchant, []pkbFetchFailure, int, error) {
	entities, _, err := listCityEntities(ctx, flags, loc, category, maxScanPages, 50)
	if err != nil && len(entities) == 0 {
		return nil, nil, 0, err
	}
	if maxMerchants > 0 && len(entities) > maxMerchants {
		entities = entities[:maxMerchants]
	}
	// attempted is the number of merchants we actually fetch deals from
	// (post-truncation); it is the honest denominator for partial-failure
	// warnings and the "scanned N merchants" note.
	attempted := len(entities)

	type result struct {
		idx    int
		deals  []pkbDeal
		entity pkbEntity
		err    error
	}
	results := make(chan result, len(entities))
	sem := make(chan struct{}, 8) // bounded concurrency; client limiter paces further
	var wg sync.WaitGroup
	for i, e := range entities {
		wg.Add(1)
		go func(idx int, ent pkbEntity) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			deals, derr := fetchEntityDeals(ctx, flags, loc, ent.ID)
			results <- result{idx: idx, deals: deals, entity: ent, err: derr}
		}(i, e)
	}
	go func() { wg.Wait(); close(results) }()

	ordered := make([][]pkbDeal, len(entities))
	errs := make([]error, len(entities))
	for r := range results {
		ordered[r.idx] = r.deals
		errs[r.idx] = r.err
	}

	out := make([]dealWithMerchant, 0)
	failures := make([]pkbFetchFailure, 0)
	for i, e := range entities {
		if errs[i] != nil {
			failures = append(failures, pkbFetchFailure{EntityID: e.ID, EntityName: e.Name, Error: errs[i].Error()})
			continue
		}
		for _, d := range ordered[i] {
			out = append(out, dealWithMerchant{pkbDeal: d, MerchantID: e.ID, MerchantName: e.Name})
		}
	}
	return out, failures, attempted, err
}

// sortDealsByDiscountDesc sorts in place, biggest discount first.
func sortDealsByDiscountDesc(deals []dealWithMerchant) {
	sort.SliceStable(deals, func(i, j int) bool {
		return deals[i].PercentageValue > deals[j].PercentageValue
	})
}

// pakistanTZ is the zone Peekaboo's timezone-less timestamps are expressed in.
// The API serves Pakistani cities and omits any offset, so reading those values
// as UTC shifts every validity window by five hours -- enough to include or
// exclude a deal a full day early and to skew days_left around the cutoff.
// LoadLocation keeps historical offsets correct where tzdata is available; the
// fixed +05:00 fallback covers builds without it.
var pakistanTZ = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Karachi"); err == nil {
		return loc
	}
	return time.FixedZone("PKT", 5*60*60)
}()

// parseDealTime parses Peekaboo's datetime strings (e.g. "2027-06-16T23:59:00").
// Values carrying an explicit offset are honored as sent; timezone-less values
// are read as Pakistan local time rather than UTC. Returns ok=false on
// empty/unparseable input.
func parseDealTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	// RFC3339 goes first: it is the only layout here that carries its own
	// offset, and that offset has to win over any local interpretation.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, pakistanTZ); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
