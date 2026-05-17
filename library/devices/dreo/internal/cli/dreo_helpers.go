// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the hand-written Dreo novel commands: resolving the
// WebSocket host from the configured REST base URL, opening the local
// store, refreshing the device catalog from the API, parsing CLI human
// inputs, and mapping flag values to Dreo WS control-frame params.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/dreoauth"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/dreows"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/store"
)

// dreoWSHost returns the WS host for the given REST base URL.
// Falls back to the US host if the base is unusual.
func dreoWSHost(baseURL string) string {
	if strings.Contains(baseURL, "-eu.") {
		return "wsb-eu.dreo-tech.com"
	}
	return "wsb-us.dreo-tech.com"
}

// openStore returns a *store.Store at the default path.
func openStore() (*store.Store, error) {
	return store.Open(store.DefaultPath())
}

// loadAccessToken returns the access token from config, or "" if not set.
func loadAccessToken(flags *rootFlags) (string, *config.Config, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return "", nil, err
	}
	return cfg.AccessToken, cfg, nil
}

// requireToken returns the cached access token, or — if none exists but
// credentials are reachable via env vars or the persisted config — runs
// the OAuth exchange to mint one. Mirrors the REST client's lazyLogin
// path so the WebSocket-only commands (`watch`, `sensors record`) work
// out-of-the-box for users who only have DREO_USERNAME/DREO_PASSWORD set
// and haven't run an authenticated REST command yet in this session.
func requireToken(flags *rootFlags) (string, *config.Config, error) {
	tok, cfg, err := loadAccessToken(flags)
	if err != nil {
		return "", nil, configErr(err)
	}
	if tok != "" {
		return tok, cfg, nil
	}
	if cfg.DreoUsername == "" || cfg.DreoPassword == "" {
		return "", nil, authErr(fmt.Errorf("not authenticated; export DREO_USERNAME and DREO_PASSWORD or run `dreo-pp-cli auth login`"))
	}
	// Lazy login: same flow as Client.lazyLogin, but inlined here because
	// the WebSocket commands don't otherwise instantiate a REST client.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, lerr := dreoauth.Login(ctx, cfg.BaseURL, cfg.DreoUsername, cfg.DreoPassword)
	if lerr != nil {
		return "", nil, authErr(fmt.Errorf("lazy login: %w", lerr))
	}
	cfg.AccessToken = resp.AccessToken
	cfg.Region = resp.Region
	// Best-effort persist; ignore errors so a read-only HOME (dogfood
	// --live scoped tempdir) doesn't kill the WS connect.
	_ = cfg.SaveTokens("", "", resp.AccessToken, "", time.Time{})
	return resp.AccessToken, cfg, nil
}

// connectWS dials the Dreo websocket using the active credential.
func connectWS(ctx context.Context, flags *rootFlags) (*dreows.Conn, error) {
	tok, cfg, err := requireToken(flags)
	if err != nil {
		return nil, err
	}
	return dreows.Connect(ctx, dreoWSHost(cfg.BaseURL), tok)
}

// dreoListEnvelope mirrors GET /api/v2/user-device/device/list response.
type dreoListEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List []map[string]any `json:"list"`
	} `json:"data"`
}

// fetchAndCacheDevices calls the device-list endpoint and writes the
// catalog into the local store. Returns the parsed devices.
//
// Always uses a non-dry-run client: helper commands like `set` and `bulk`
// resolve a device by sn/name even under --dry-run, and a dry-run-shaped
// device list ({"dry_run": true}) would otherwise short-circuit the lookup.
func fetchAndCacheDevices(ctx context.Context, flags *rootFlags) ([]store.Device, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	c.DryRun = false // dry-run only gates the user-visible mutation
	params := map[string]string{
		"currentPage": "1",
		"pageSize":    "100",
	}
	raw, err := c.Get("/api/v2/user-device/device/list", params)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	devs, err := parseDeviceList(raw)
	if err != nil {
		return nil, err
	}
	st, err := openStore()
	if err != nil {
		return nil, err
	}
	defer st.Close()
	for _, d := range devs {
		if err := st.UpsertDevice(ctx, d); err != nil {
			return nil, err
		}
	}
	return devs, nil
}

func parseDeviceList(raw json.RawMessage) ([]store.Device, error) {
	// Some Dreo endpoints return the bare `data` shape; some wrap with `{code,msg,data:{list:[...]}}`.
	// Handle both.
	var env dreoListEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && env.Data.List != nil {
		return convertDeviceList(env.Data.List), nil
	}
	// Bare-array fallback
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return convertDeviceList(arr), nil
	}
	return nil, errors.New("could not parse device list response")
}

func convertDeviceList(rows []map[string]any) []store.Device {
	out := make([]store.Device, 0, len(rows))
	for _, r := range rows {
		raw, _ := json.Marshal(r)
		// Dreo's device list does not expose an `online` field;
		// status is derived from the state endpoint's mixed.connected.
		// Default to true here so freshly-synced devices aren't
		// misreported as offline before any state call has run.
		online := true
		if _, has := r["online"]; has {
			online = asBool(r, "online")
		}
		d := store.Device{
			Sn:        asString(r, "sn", "deviceSn"),
			Name:      asString(r, "deviceName", "name"),
			Model:     asString(r, "model", "productModel"),
			Room:      asString(r, "roomName", "room"),
			ProductID: asInt(r, "productId"),
			Online:    online,
			Raw:       raw,
			UpdatedAt: time.Now(),
		}
		if d.Sn == "" {
			continue
		}
		out = append(out, d)
	}
	return out
}

// deviceCacheTTL bounds how long a synced device catalog can be served
// without a refresh. Past this age the cache is treated as stale and
// listCachedOrFetch falls through to the live endpoint. The window is
// generous because device-list churn on a single Dreo account is rare
// (devices are added/removed manually in the app), but bounded so that
// `bulk --all`, `scene save`, and `rooms` never silently operate on a
// snapshot from weeks ago when the account has changed.
const deviceCacheTTL = 1 * time.Hour

// listCachedOrFetch returns devices from the cache when at least one row
// exists AND every row was updated within deviceCacheTTL. If the cache
// is empty, stale, or forceLive is set, it refetches from the live API
// and writes through the store.
func listCachedOrFetch(ctx context.Context, flags *rootFlags, forceLive bool) ([]store.Device, error) {
	if !forceLive {
		st, err := openStore()
		if err == nil {
			defer st.Close()
			devs, err := st.ListDevices(ctx)
			if err == nil && len(devs) > 0 {
				oldest := time.Now()
				for _, d := range devs {
					if d.UpdatedAt.Before(oldest) {
						oldest = d.UpdatedAt
					}
				}
				if time.Since(oldest) <= deviceCacheTTL {
					return devs, nil
				}
				// Cache is stale: fall through to live fetch.
			}
		}
	}
	return fetchAndCacheDevices(ctx, flags)
}

// fetchDeviceState fetches /api/user-device/device/state and writes it to the store.
// The state response wraps fields in a `mixed` sub-object (Dreo's packaging quirk);
// flattenState pulls those up so callers see a flat state map.
func fetchDeviceState(ctx context.Context, flags *rootFlags, c *client.Client, sn string) (map[string]any, error) {
	// GetNoCache bypasses the 5-minute on-disk HTTP cache so the
	// `--live` paths in sensors/alerts actually hit the API on every
	// invocation. Going through c.Get would let a 4-minute-old cached
	// response satisfy --live silently, which contradicts the flag's
	// documented "fetch fresh state for every device" contract.
	raw, err := c.GetNoCache("/api/user-device/device/state", map[string]string{"deviceSn": sn})
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var env struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Data != nil {
		flat := flattenState(env.Data)
		st, _ := openStore()
		if st != nil {
			defer st.Close()
			data, _ := json.Marshal(flat)
			_ = st.UpsertDeviceState(ctx, sn, data, time.Now())
		}
		return flat, nil
	}
	// Bare-object fallback
	var bare map[string]any
	if err := json.Unmarshal(raw, &bare); err == nil {
		return flattenState(bare), nil
	}
	return nil, errors.New("could not parse device state response")
}

// flattenState merges Dreo's `mixed` sub-object (and `deviceInfo`) into the
// top level so callers can read temperature/humidity/pm25/poweron etc.
// without knowing the wrapper. Top-level keys win on conflict (the state
// API sometimes mirrors a few fields at both levels).
//
// Every state field in `mixed` is wrapped: `{state: <value>, timestamp: <unix>}`.
// We unwrap the `state` value so callers see scalar/object data directly.
// The timestamp is preserved alongside as `<field>_timestamp` for novel
// commands like alerts that want freshness signals.
func flattenState(m map[string]any) map[string]any {
	out := map[string]any{}
	unwrap := func(target map[string]any, key string, val any) {
		if obj, ok := val.(map[string]any); ok {
			if state, hasState := obj["state"]; hasState {
				target[key] = state
				if ts, hasTS := obj["timestamp"]; hasTS && ts != nil {
					target[key+"_timestamp"] = ts
				}
				return
			}
		}
		target[key] = val
	}
	if mixed, ok := m["mixed"].(map[string]any); ok {
		for k, v := range mixed {
			unwrap(out, k, v)
		}
	}
	if info, ok := m["deviceInfo"].(map[string]any); ok {
		for k, v := range info {
			if _, already := out[k]; !already {
				unwrap(out, k, v)
			}
		}
	}
	for k, v := range m {
		if k == "mixed" || k == "deviceInfo" {
			continue
		}
		if _, already := out[k]; !already {
			out[k] = v
		}
	}
	return out
}

// asString reads the first non-empty string at the given keys.
func asString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			}
		}
	}
	return ""
}

func asInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return int(t)
			case int:
				return t
			case string:
				if n, err := strconv.Atoi(t); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func asBool(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case bool:
				return t
			case float64:
				return t != 0
			case string:
				return t == "true" || t == "1" || t == "online"
			}
		}
	}
	return false
}

func asFloat(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return t, true
			case int:
				return float64(t), true
			case string:
				if n, err := strconv.ParseFloat(t, 64); err == nil {
					return n, true
				}
			}
		}
	}
	return 0, false
}

// parseOnOff turns "on"/"off"/"true"/"false" into a bool.
func parseOnOff(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "1", "yes", "y":
		return true, nil
	case "off", "false", "0", "no", "n":
		return false, nil
	}
	return false, fmt.Errorf("expected on|off, got %q", s)
}

// parseDuration is time.ParseDuration plus shorthand "30s","2h","15m".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty duration")
	}
	return time.ParseDuration(s)
}

// resolveDeviceByQuery looks up a device sn or name in the cache; on a
// miss it refreshes from the API and retries once.
func resolveDeviceByQuery(ctx context.Context, flags *rootFlags, query string) (*store.Device, error) {
	st, err := openStore()
	if err != nil {
		return nil, err
	}
	defer st.Close()
	dev, err := st.GetDevice(ctx, query)
	if err == nil {
		return dev, nil
	}
	// Refresh and retry
	if _, ferr := fetchAndCacheDevices(ctx, flags); ferr != nil {
		return nil, ferr
	}
	return st.GetDevice(ctx, query)
}

// matchesDeviceType reports whether a device matches a coarse type filter.
// Accepts the prefix model code (e.g. "HTF") or a category name like
// "tower-fan", "purifier", "humidifier", "heater", "ac".
func matchesDeviceType(d store.Device, t string) bool {
	if t == "" {
		return true
	}
	model := strings.ToUpper(d.Model)
	key := strings.ToLower(strings.TrimSpace(t))
	if strings.HasPrefix(model, strings.ToUpper(key)) {
		return true
	}
	switch key {
	case "tower-fan", "tower":
		return strings.HasPrefix(model, "HTF")
	case "fan":
		return strings.HasPrefix(model, "HTF") || strings.HasPrefix(model, "HPF") || strings.HasPrefix(model, "HCF") || strings.HasPrefix(model, "HSH")
	case "purifier", "air-purifier":
		return strings.HasPrefix(model, "HAP") || strings.HasPrefix(model, "HPP") || strings.HasPrefix(model, "HKO")
	case "humidifier":
		return strings.HasPrefix(model, "HHM") || strings.HasPrefix(model, "HHU")
	case "heater":
		return strings.HasPrefix(model, "HSH") || strings.HasPrefix(model, "HCH") || strings.HasPrefix(model, "HCT")
	case "ac", "air-conditioner":
		return strings.HasPrefix(model, "HAC") || strings.HasPrefix(model, "WHAC")
	}
	return false
}
