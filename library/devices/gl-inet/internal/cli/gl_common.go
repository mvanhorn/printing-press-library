// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared helpers for the hand-written GL.iNet command surface: GL JSON-RPC
// access, SSH/UCI access, and the pure parsing/diff/region logic that the
// novel commands build on. Pure functions here are unit-tested in
// gl_common_test.go.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/store"
)

// classifyGLError maps a GL JSON-RPC / SSH error to a typed exit code. Auth
// failures (bad/missing router password) become code 4; "Method not found"
// (firmware-version capability gaps) become a not-found code 3; everything
// else is an API error (code 5).
func classifyGLError(err error, flags *rootFlags) error {
	if err == nil {
		return nil
	}
	var authE *client.GLAuthError
	if errors.As(err, &authE) {
		return authErr(err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "method not found") {
		return notFoundErr(err)
	}
	return apiErr(err)
}

// glClient returns a configured GL client. Thin wrapper so command bodies read
// uniformly and any future cross-cutting setup has one home.
func glClient(flags *rootFlags) (*client.Client, error) {
	return flags.newClient()
}

// glSSHConfig resolves the SSH/UCI transport config from the client base URL.
func glSSHConfig(c *client.Client) (glssh.Config, error) {
	return glssh.ResolveConfig(c.RequestBaseURL())
}

// openSnapshotStore opens the local SQLite store that holds config_snapshots.
func openSnapshotStore(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("gl-inet-pp-cli"))
}

// jsonObjField extracts a named field from a JSON object result as raw JSON.
// Returns nil when the field is absent or the input is not an object.
func jsonObjField(raw json.RawMessage, key string) json.RawMessage {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	return obj[key]
}

// jsonArrayOrSelf returns the named array field if present, else the input.
// Lets a command surface `.clients` / `.res` / `.config_list` arrays while
// degrading gracefully (e.g. under verify short-circuit envelopes).
func jsonArrayField(raw json.RawMessage, key string) json.RawMessage {
	if f := jsonObjField(raw, key); f != nil {
		var arr []json.RawMessage
		if json.Unmarshal(f, &arr) == nil {
			return f
		}
	}
	return json.RawMessage("[]")
}

// shellQuoteSingle single-quotes a string for safe POSIX-shell embedding,
// escaping any embedded single quotes.
func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// --- system info / provenance ---------------------------------------------

// glSystemInfo is the subset of system.get_info the GL commands consume.
type glSystemInfo struct {
	Model           string         `json:"model"`
	FirmwareVersion string         `json:"firmware_version"`
	CountryCode     string         `json:"country_code"`
	BoardInfo       glBoardInfo    `json:"board_info"`
	SoftwareFeature map[string]any `json:"software_feature"`
	HardwareFeature map[string]any `json:"hardware_feature"`
}

type glBoardInfo struct {
	Model          string `json:"model"`
	OpenwrtVersion string `json:"openwrt_version"`
	KernelVersion  string `json:"kernel_version"`
	Hostname       string `json:"hostname"`
	Architecture   string `json:"architecture"`
}

// fetchSystemInfo calls system.get_info and decodes it. Returns a typed struct
// plus the raw JSON for callers that want passthrough.
func fetchSystemInfo(ctx context.Context, c *client.Client) (glSystemInfo, json.RawMessage, error) {
	raw, err := c.Call(ctx, "system", "get_info", nil)
	if err != nil {
		return glSystemInfo{}, nil, err
	}
	var info glSystemInfo
	_ = json.Unmarshal(raw, &info)
	if info.Model == "" {
		info.Model = info.BoardInfo.Model
	}
	return info, raw, nil
}

// parseOpenwrtRelease parses /etc/openwrt_release KEY='value' lines into a map
// with the surrounding quotes stripped.
func parseOpenwrtRelease(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `'"`)
		m[key] = val
	}
	return m
}

// parseLuciVersion extracts the luci version from `opkg list-installed` output
// for the `luci` package (line shape: "luci - <version>").
func parseLuciVersion(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "luci ") && line != "luci" {
			continue
		}
		if i := strings.Index(line, " - "); i >= 0 {
			return strings.TrimSpace(line[i+3:])
		}
	}
	return ""
}

// --- UCI show parsing & diff ----------------------------------------------

// uciKeyRE pins a uci dotted key to a shell- and SQL-safe shape so reconstructed
// `uci set`/`uci delete` commands cannot inject. Covers anonymous sections
// (pkg.@type[0].opt) and named sections alike.
var uciKeyRE = regexp.MustCompile(`^[A-Za-z0-9_]+(?:\.[A-Za-z0-9_@\[\]-]+){1,2}$`)

// parseUCIShow parses `uci show` dotted output into key -> raw-RHS map. The
// value is kept exactly as uci printed it (single-quoted for options, bare for
// section-type declarations) so diffs compare consistently and apply can
// reconstruct shell-safe `uci set key=rhs` commands.
func parseUCIShow(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := line[eq+1:]
		if key == "" {
			continue
		}
		m[key] = val
	}
	return m
}

// uciPackage returns the package (first dotted segment) of a uci key.
func uciPackage(key string) string {
	if i := strings.IndexByte(key, '.'); i >= 0 {
		return key[:i]
	}
	return key
}

// uciDisplayValue strips one pair of surrounding single quotes and unescapes
// the uci `'\”` sequence for human-readable display. Lists (multiple quoted
// tokens) are returned roughly as-is.
func uciDisplayValue(rhs string) string {
	s := rhs
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' && !strings.Contains(s[1:len(s)-1], "' '") {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, `'\''`, `'`)
	}
	return s
}

// uciChange is one option-level difference between two uci configurations.
type uciChange struct {
	Key string `json:"key"`
	Old string `json:"old,omitempty"`
	New string `json:"new,omitempty"`
}

// uciDiff is the structured difference between an old (from) and new (to)
// configuration. Added = present in `to` only; Removed = present in `from`
// only; Changed = present in both with differing values (Old from `from`,
// New from `to`).
type uciDiff struct {
	Added   []uciChange `json:"added"`
	Removed []uciChange `json:"removed"`
	Changed []uciChange `json:"changed"`
}

// Empty reports whether the diff has no differences.
func (d uciDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// computeUCIDiff diffs two parsed uci-show maps. `from` is the baseline/old,
// `to` is the target/new. Display values are unquoted for readability.
func computeUCIDiff(from, to map[string]string) uciDiff {
	var d uciDiff
	for k, nv := range to {
		ov, ok := from[k]
		if !ok {
			d.Added = append(d.Added, uciChange{Key: k, New: uciDisplayValue(nv)})
		} else if ov != nv {
			d.Changed = append(d.Changed, uciChange{Key: k, Old: uciDisplayValue(ov), New: uciDisplayValue(nv)})
		}
	}
	for k, ov := range from {
		if _, ok := to[k]; !ok {
			d.Removed = append(d.Removed, uciChange{Key: k, Old: uciDisplayValue(ov)})
		}
	}
	sortChanges(d.Added)
	sortChanges(d.Removed)
	sortChanges(d.Changed)
	return d
}

func sortChanges(c []uciChange) {
	sort.Slice(c, func(i, j int) bool { return c[i].Key < c[j].Key })
}

// uciApplyCommands builds the ordered `uci` commands that transform the current
// configuration (cur) into the target snapshot (snap), grouped so each touched
// package gets a trailing `uci commit`. Returns the command list and the set of
// touched packages. Keys that fail the safe-key regex are skipped.
//
// snapRaw/curRaw are the raw-RHS maps from parseUCIShow.
func uciApplyCommands(curRaw, snapRaw map[string]string) (cmds []string, packages []string) {
	// from = current, to = snapshot: added/changed -> uci set; removed -> delete.
	diff := computeRawDiff(curRaw, snapRaw)
	touched := map[string]bool{}
	add := func(key, cmd string) {
		if !uciKeyRE.MatchString(key) {
			return
		}
		cmds = append(cmds, cmd)
		touched[uciPackage(key)] = true
	}
	// Sets for added + changed (value comes from snapshot raw RHS).
	for _, ch := range append(append([]uciChange{}, diff.Added...), diff.Changed...) {
		rhs := snapRaw[ch.Key]
		add(ch.Key, fmt.Sprintf("uci set %s=%s", shellQuoteSingle(ch.Key), rhs))
	}
	for _, ch := range diff.Removed {
		add(ch.Key, fmt.Sprintf("uci delete %s", shellQuoteSingle(ch.Key)))
	}
	for p := range touched {
		packages = append(packages, p)
	}
	sort.Strings(packages)
	return cmds, packages
}

// computeRawDiff is computeUCIDiff over raw values without display unquoting,
// used by apply so command reconstruction sees exact RHS strings.
func computeRawDiff(from, to map[string]string) uciDiff {
	var d uciDiff
	for k, nv := range to {
		ov, ok := from[k]
		if !ok {
			d.Added = append(d.Added, uciChange{Key: k})
		} else if ov != nv {
			d.Changed = append(d.Changed, uciChange{Key: k})
		}
	}
	for k := range from {
		if _, ok := to[k]; !ok {
			d.Removed = append(d.Removed, uciChange{Key: k})
		}
	}
	sortChanges(d.Added)
	sortChanges(d.Removed)
	sortChanges(d.Changed)
	return d
}

// grepUCILines returns the `key=value` lines from uci-show text that contain
// term (case-insensitive) anywhere in the line.
func grepUCILines(out, term string) []string {
	term = strings.ToLower(term)
	var matches []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(line), term) {
			matches = append(matches, line)
		}
	}
	return matches
}

var reRadioCountry = regexp.MustCompile(`^wireless\.([^.]+)\.country$`)
var reWifiDevice = regexp.MustCompile(`^wireless\.([^.]+)$`)

// extractWifiDevices returns the radio device names (sections of type
// wifi-device) from a parsed `uci show wireless` map.
func extractWifiDevices(show map[string]string) []string {
	var devices []string
	for k, v := range show {
		if uciDisplayValue(v) != "wifi-device" {
			continue
		}
		if m := reWifiDevice.FindStringSubmatch(k); m != nil {
			devices = append(devices, m[1])
		}
	}
	sort.Strings(devices)
	return devices
}

// buildRegionSetScript builds the uci commands that set each radio's regulatory
// country and reload wifi.
func buildRegionSetScript(radios []string, cc string) string {
	var parts []string
	for _, r := range radios {
		parts = append(parts, fmt.Sprintf("uci set wireless.%s.country=%s", r, shellQuoteSingle(cc)))
	}
	parts = append(parts, "uci commit wireless", "wifi reload")
	return strings.Join(parts, " ; ")
}

// snapshotModelMismatch reports whether applying a snapshot should be refused
// on the provenance gate: true only when both models are known, they differ,
// and the operator did not pass --force.
func snapshotModelMismatch(snapModel, curModel string, force bool) bool {
	if force {
		return false
	}
	if snapModel == "" || curModel == "" {
		return false
	}
	return snapModel != curModel
}

// mapWanMode translates a user-facing WAN source to the GL netmode value.
// ethernet/cable/wired -> router; repeater/wifi -> repeater; tethering/usb ->
// tethering (passed literally). Returns (mode, true) on a known source.
func mapWanMode(arg string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "ethernet", "cable", "wired", "router":
		return "router", true
	case "repeater", "wifi", "wisp":
		return "repeater", true
	case "tethering", "usb", "modem", "tether":
		return "tethering", true
	default:
		return "", false
	}
}

// validCountryCode reports whether cc is a 2-letter uppercase ISO country code.
func validCountryCode(cc string) bool {
	if len(cc) != 2 {
		return false
	}
	for _, r := range cc {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// extractRadioCountries pulls per-radio regulatory country codes from a parsed
// `uci show wireless` map.
func extractRadioCountries(show map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range show {
		if m := reRadioCountry.FindStringSubmatch(k); m != nil {
			out[m[1]] = uciDisplayValue(v)
		}
	}
	return out
}

// --- regulatory-domain channel reference -----------------------------------
//
// pp:novel-static-reference
// 2.4GHz channel availability by regulatory domain. The exact problem this
// targets: a router set to a US regdomain cannot see/join an AP on 2.4GHz
// channel 12 or 13, which most of Europe (and the rest of the world) uses.
// US/Canada/Taiwan cap at channel 11; Japan allows 14; everywhere else 13.
var country24MaxChannel = map[string]int{
	"US": 11, "CA": 11, "TW": 11,
	"JP": 14,
}

const defaultCountry24MaxChannel = 13

// allowed24MaxChannel returns the highest 2.4GHz channel a regulatory domain
// permits. Unknown/empty country codes fall back to the world-common 13.
func allowed24MaxChannel(cc string) int {
	cc = strings.ToUpper(strings.TrimSpace(cc))
	if v, ok := country24MaxChannel[cc]; ok {
		return v
	}
	return defaultCountry24MaxChannel
}

// channel24Allowed reports whether a 2.4GHz channel is usable under cc.
func channel24Allowed(cc string, ch int) bool {
	if ch < 1 || ch > 14 {
		return false
	}
	return ch <= allowed24MaxChannel(cc)
}

// countryAllowing24Channel names an example country whose regdomain permits the
// given 2.4GHz channel, for the "set region to e.g. IT" remediation hint.
func countryAllowing24Channel(ch int) string {
	switch {
	case ch <= 11:
		return "US"
	case ch <= 13:
		return "IT"
	default:
		return "JP"
	}
}

// findAPChannel recursively searches a decoded JSON value (e.g. a repeater.scan
// result of unknown exact shape) for an object whose ssid matches and returns
// its channel. Returns (0, false) when not found.
func findAPChannel(raw json.RawMessage, ssid string) (int, bool) {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return 0, false
	}
	return walkForChannel(v, ssid)
}

func walkForChannel(v any, ssid string) (int, bool) {
	switch t := v.(type) {
	case map[string]any:
		if s, ok := t["ssid"].(string); ok && s == ssid {
			if ch, ok := numField(t, "channel"); ok {
				return ch, true
			}
		}
		for _, child := range t {
			if ch, ok := walkForChannel(child, ssid); ok {
				return ch, true
			}
		}
	case []any:
		for _, child := range t {
			if ch, ok := walkForChannel(child, ssid); ok {
				return ch, true
			}
		}
	}
	return 0, false
}

func numField(m map[string]any, key string) (int, bool) {
	switch n := m[key].(type) {
	case float64:
		return int(n), true
	case string:
		if v := atoiSafe(n); v > 0 {
			return v, true
		}
	}
	return 0, false
}

// detectRepeaterConnected interprets a repeater.get_status result. Returns
// whether the STA is joined to a source AP and the SSID it is on (if any).
func detectRepeaterConnected(raw json.RawMessage) (bool, string) {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false, ""
	}
	ssid, _ := m["ssid"].(string)
	if s, ok := m["status"].(string); ok {
		if strings.EqualFold(s, "connected") {
			return true, ssid
		}
		if strings.EqualFold(s, "disconnected") {
			return false, ssid
		}
	}
	if c, ok := m["connected"].(bool); ok {
		return c, ssid
	}
	return strings.TrimSpace(ssid) != "", ssid
}

// --- iwinfo scan/info parsing ----------------------------------------------

type scanAP struct {
	SSID    string `json:"ssid"`
	Channel int    `json:"channel"`
	Signal  int    `json:"signal_dbm"`
}

var (
	reESSID   = regexp.MustCompile(`ESSID:\s*"([^"]*)"`)
	reChannel = regexp.MustCompile(`Channel:\s*(\d+)`)
	reSignal  = regexp.MustCompile(`Signal:\s*(-?\d+)\s*dBm`)
	reBitRate = regexp.MustCompile(`Bit Rate:\s*([0-9.]+)`)
	reMode    = regexp.MustCompile(`Mode:\s*(\w+)`)
)

// parseIwinfoScan parses `iwinfo <iface> scan` output into a list of APs.
// Each "Cell" block contributes one entry; partial blocks are tolerated.
func parseIwinfoScan(out string) []scanAP {
	var aps []scanAP
	blocks := regexp.MustCompile(`(?m)^\s*Cell `).Split(out, -1)
	for _, b := range blocks {
		if !strings.Contains(b, "ESSID") && !strings.Contains(b, "Channel") {
			continue
		}
		ap := scanAP{}
		if m := reESSID.FindStringSubmatch(b); m != nil {
			ap.SSID = m[1]
		}
		if m := reChannel.FindStringSubmatch(b); m != nil {
			ap.Channel = atoiSafe(m[1])
		}
		if m := reSignal.FindStringSubmatch(b); m != nil {
			ap.Signal = atoiSafe(m[1])
		}
		if ap.SSID == "" && ap.Channel == 0 {
			continue
		}
		aps = append(aps, ap)
	}
	return aps
}

// iwinfoInfo holds the fields uplink cares about from `iwinfo <iface> info`.
type iwinfoInfo struct {
	ESSID   string  `json:"essid"`
	Channel int     `json:"channel"`
	Signal  int     `json:"signal_dbm"`
	BitRate float64 `json:"bitrate_mbits"`
	Mode    string  `json:"mode"`
}

func parseIwinfoInfo(out string) iwinfoInfo {
	var info iwinfoInfo
	if m := reESSID.FindStringSubmatch(out); m != nil {
		info.ESSID = m[1]
	}
	if m := reChannel.FindStringSubmatch(out); m != nil {
		info.Channel = atoiSafe(m[1])
	}
	if m := reSignal.FindStringSubmatch(out); m != nil {
		info.Signal = atoiSafe(m[1])
	}
	if m := reBitRate.FindStringSubmatch(out); m != nil {
		info.BitRate = atofSafe(m[1])
	}
	if m := reMode.FindStringSubmatch(out); m != nil {
		info.Mode = m[1]
	}
	return info
}

func atoiSafe(s string) int {
	n := 0
	neg := false
	for i, r := range s {
		if i == 0 && r == '-' {
			neg = true
			continue
		}
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		return -n
	}
	return n
}

func atofSafe(s string) float64 {
	var whole, frac float64
	var div float64 = 1
	dot := false
	for _, r := range s {
		if r == '.' {
			dot = true
			continue
		}
		if r < '0' || r > '9' {
			break
		}
		if dot {
			div *= 10
			frac = frac*10 + float64(r-'0')
		} else {
			whole = whole*10 + float64(r-'0')
		}
	}
	return whole + frac/div
}

// parseIfaceBlocks parses the uplink gather output (blocks delimited by
// "##IFACE <name>") into a per-interface iwinfo info map.
func parseIfaceBlocks(text string) map[string]iwinfoInfo {
	out := map[string]iwinfoInfo{}
	parts := strings.Split(text, "##IFACE ")
	for _, p := range parts[1:] {
		nl := strings.IndexByte(p, '\n')
		if nl < 0 {
			continue
		}
		iface := strings.TrimSpace(p[:nl])
		if i := strings.Index(iface, "##PING"); i >= 0 {
			iface = strings.TrimSpace(iface[:i])
		}
		if iface == "" {
			continue
		}
		body := p[nl+1:]
		if i := strings.Index(body, "##PING"); i >= 0 {
			body = body[:i]
		}
		out[iface] = parseIwinfoInfo(body)
	}
	return out
}

// pickStaInterface chooses the repeater STA interface from a parsed iface map:
// prefer one in Client/Sta mode, else the one with the strongest signal and a
// non-empty ESSID. Returns the iface name and its info.
func pickStaInterface(ifaces map[string]iwinfoInfo) (string, iwinfoInfo) {
	bestName, bestInfo := "", iwinfoInfo{}
	bestScore := -1 << 30
	names := make([]string, 0, len(ifaces))
	for n := range ifaces {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		info := ifaces[n]
		score := 0
		switch strings.ToLower(info.Mode) {
		case "client", "sta":
			score += 100000
		}
		if info.ESSID != "" {
			score += 1000
		}
		score += info.Signal // less-negative is better
		if score > bestScore {
			bestScore = score
			bestName, bestInfo = n, info
		}
	}
	return bestName, bestInfo
}

// channelCounts tallies how many scanned APs sit on each channel.
func channelCounts(aps []scanAP) map[int]int {
	counts := map[int]int{}
	for _, ap := range aps {
		if ap.Channel > 0 {
			counts[ap.Channel]++
		}
	}
	return counts
}

// has5GHzRadio reports whether any interface is on a 5GHz channel (>14).
func has5GHzRadio(ifaces map[string]iwinfoInfo) bool {
	for _, info := range ifaces {
		if info.Channel > 14 {
			return true
		}
	}
	return false
}

// parsePingLatency extracts the average round-trip ms from `ping` output.
// Returns (ms, true) when an avg figure is found.
func parsePingLatency(out string) (float64, bool) {
	// Busybox/iputils: "round-trip min/avg/max = 1.2/3.4/5.6 ms" or
	// "rtt min/avg/max/mdev = 1.2/3.4/5.6/0.1 ms".
	re := regexp.MustCompile(`=\s*[0-9.]+/([0-9.]+)/`)
	if m := re.FindStringSubmatch(out); m != nil {
		return atofSafe(m[1]), true
	}
	return 0, false
}

// pingReachable reports whether ping output indicates at least one reply.
func pingReachable(out string) bool {
	if strings.Contains(out, "bytes from") {
		return true
	}
	m := regexp.MustCompile(`(\d+) (?:packets )?received`).FindStringSubmatch(out)
	if m == nil {
		return false
	}
	return atoiSafe(m[1]) > 0
}
