// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestParseUCIShow(t *testing.T) {
	t.Parallel()
	in := "wireless.mt798111=wifi-device\nwireless.mt798111.country='IT'\n\nnetwork.lan.proto='static'\n# comment-without-eq\n"
	got := parseUCIShow(in)
	if got["wireless.mt798111"] != "wifi-device" {
		t.Errorf("section decl: got %q", got["wireless.mt798111"])
	}
	if got["wireless.mt798111.country"] != "'IT'" {
		t.Errorf("option raw rhs: got %q", got["wireless.mt798111.country"])
	}
	if _, ok := got["# comment-without-eq"]; ok {
		t.Errorf("line without '=' should be skipped")
	}
	if len(got) != 3 {
		t.Errorf("want 3 keys, got %d (%v)", len(got), got)
	}
}

func TestUCIDisplayValue(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"'IT'":        "IT",
		"wifi-device": "wifi-device",
		"'a' 'b'":     "'a' 'b'", // list stays as-is
		`'it'\''s'`:   "it's",    // escaped single quote unescaped
	}
	for in, want := range cases {
		if got := uciDisplayValue(in); got != want {
			t.Errorf("uciDisplayValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComputeUCIDiff(t *testing.T) {
	t.Parallel()
	from := map[string]string{
		"wireless.r.country": "'US'",
		"network.lan.proto":  "'static'",
		"removed.only.opt":   "'x'",
	}
	to := map[string]string{
		"wireless.r.country": "'IT'",     // changed
		"network.lan.proto":  "'static'", // same
		"added.new.opt":      "'y'",      // added
	}
	d := computeUCIDiff(from, to)
	if len(d.Changed) != 1 || d.Changed[0].Key != "wireless.r.country" || d.Changed[0].Old != "US" || d.Changed[0].New != "IT" {
		t.Errorf("changed wrong: %+v", d.Changed)
	}
	if len(d.Added) != 1 || d.Added[0].Key != "added.new.opt" {
		t.Errorf("added wrong: %+v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0].Key != "removed.only.opt" {
		t.Errorf("removed wrong: %+v", d.Removed)
	}
	if d.Empty() {
		t.Errorf("diff should not be empty")
	}
}

func TestUCIApplyCommands(t *testing.T) {
	t.Parallel()
	cur := map[string]string{
		"wireless.r.country": "'US'",
		"network.lan.gone":   "'1'",
	}
	snap := map[string]string{
		"wireless.r.country": "'IT'",
		"firewall.z.input":   "'ACCEPT'",
	}
	cmds, pkgs := uciApplyCommands(cur, snap)
	joined := strings.Join(cmds, "\n")
	// Keys are shell-quoted to defend against glob metacharacters in anonymous
	// section names (e.g. wireless.@wifi-iface[0].ssid).
	if !strings.Contains(joined, "uci set 'wireless.r.country'='IT'") {
		t.Errorf("missing changed set: %v", cmds)
	}
	if !strings.Contains(joined, "uci set 'firewall.z.input'='ACCEPT'") {
		t.Errorf("missing added set: %v", cmds)
	}
	if !strings.Contains(joined, "uci delete 'network.lan.gone'") {
		t.Errorf("missing delete: %v", cmds)
	}
	wantPkgs := map[string]bool{"wireless": true, "network": true, "firewall": true}
	if len(pkgs) != 3 {
		t.Errorf("want 3 packages, got %v", pkgs)
	}
	for _, p := range pkgs {
		if !wantPkgs[p] {
			t.Errorf("unexpected package %q", p)
		}
	}
}

func TestUCIApplyCommandsRejectsUnsafeKey(t *testing.T) {
	t.Parallel()
	cur := map[string]string{}
	snap := map[string]string{"wireless.r.opt; rm -rf /": "'x'"}
	cmds, _ := uciApplyCommands(cur, snap)
	if len(cmds) != 0 {
		t.Errorf("unsafe key must be skipped, got %v", cmds)
	}
}

func TestChannel24Region(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cc      string
		ch      int
		allowed bool
	}{
		{"US", 11, true},
		{"US", 12, false},
		{"US", 13, false},
		{"IT", 13, true},
		{"DE", 13, true},
		{"", 13, true}, // unknown -> world default 13
		{"JP", 14, true},
		{"IT", 14, false},
	}
	for _, tc := range cases {
		if got := channel24Allowed(tc.cc, tc.ch); got != tc.allowed {
			t.Errorf("channel24Allowed(%q,%d) = %v, want %v", tc.cc, tc.ch, got, tc.allowed)
		}
	}
	if got := countryAllowing24Channel(13); got != "IT" {
		t.Errorf("countryAllowing24Channel(13) = %q, want IT", got)
	}
	if got := countryAllowing24Channel(11); got != "US" {
		t.Errorf("countryAllowing24Channel(11) = %q, want US", got)
	}
	if got := countryAllowing24Channel(14); got != "JP" {
		t.Errorf("countryAllowing24Channel(14) = %q, want JP", got)
	}
}

func TestMapWanMode(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"ethernet":  "router",
		"cable":     "router",
		"repeater":  "repeater",
		"wifi":      "repeater",
		"tethering": "tethering",
		"usb":       "tethering",
	}
	for in, want := range cases {
		got, ok := mapWanMode(in)
		if !ok || got != want {
			t.Errorf("mapWanMode(%q) = (%q,%v), want (%q,true)", in, got, ok, want)
		}
	}
	if _, ok := mapWanMode("bogus"); ok {
		t.Errorf("mapWanMode(bogus) should be !ok")
	}
}

func TestValidCountryCode(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"IT", "US", "DE"} {
		if !validCountryCode(ok) {
			t.Errorf("validCountryCode(%q) should be true", ok)
		}
	}
	for _, bad := range []string{"it", "USA", "I", "1T", ""} {
		if validCountryCode(bad) {
			t.Errorf("validCountryCode(%q) should be false", bad)
		}
	}
}

func TestSnapshotModelMismatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		snap, cur string
		force     bool
		want      bool
	}{
		{"mt3000", "mt3000", false, false},
		{"mt3000", "mt1300", false, true},
		{"mt3000", "mt1300", true, false}, // force overrides
		{"", "mt1300", false, false},      // unknown snapshot model
		{"mt3000", "", false, false},      // unknown device model
	}
	for _, tc := range cases {
		if got := snapshotModelMismatch(tc.snap, tc.cur, tc.force); got != tc.want {
			t.Errorf("snapshotModelMismatch(%q,%q,%v) = %v, want %v", tc.snap, tc.cur, tc.force, got, tc.want)
		}
	}
}

func TestParseOpenwrtReleaseAndLuci(t *testing.T) {
	t.Parallel()
	rel := "DISTRIB_RELEASE='21.02.0'\nDISTRIB_TARGET='mediatek/filogic'\nDISTRIB_ARCH='aarch64_cortex-a53'\n"
	m := parseOpenwrtRelease(rel)
	if m["DISTRIB_RELEASE"] != "21.02.0" || m["DISTRIB_TARGET"] != "mediatek/filogic" {
		t.Errorf("parseOpenwrtRelease = %v", m)
	}
	if got := parseLuciVersion("luci - git-23.001.12345\nother - 1.0\n"); got != "git-23.001.12345" {
		t.Errorf("parseLuciVersion = %q", got)
	}
}

func TestParseIwinfo(t *testing.T) {
	t.Parallel()
	scan := `Cell 01 - Address: AA:BB:CC:DD:EE:FF
          ESSID: "CafeNet"
          Mode: Master  Channel: 13
          Signal: -60 dBm  Quality: 50/70
Cell 02 - Address: 11:22:33:44:55:66
          ESSID: "Other"
          Mode: Master  Channel: 6
          Signal: -40 dBm`
	aps := parseIwinfoScan(scan)
	if len(aps) != 2 {
		t.Fatalf("want 2 APs, got %d (%+v)", len(aps), aps)
	}
	if aps[0].SSID != "CafeNet" || aps[0].Channel != 13 || aps[0].Signal != -60 {
		t.Errorf("ap0 wrong: %+v", aps[0])
	}
	counts := channelCounts(aps)
	if counts[13] != 1 || counts[6] != 1 {
		t.Errorf("channelCounts = %v", counts)
	}

	info := parseIwinfoInfo(`wlan0     ESSID: "CafeNet"
          Mode: Client  Channel: 36 (5.180 GHz)
          Signal: -50 dBm  Noise: -95 dBm
          Bit Rate: 433.3 MBit/s`)
	if info.ESSID != "CafeNet" || info.Channel != 36 || info.Signal != -50 || info.Mode != "Client" {
		t.Errorf("info wrong: %+v", info)
	}
	if info.BitRate < 433.0 || info.BitRate > 434.0 {
		t.Errorf("bitrate wrong: %v", info.BitRate)
	}
}

func TestParsePingLatency(t *testing.T) {
	t.Parallel()
	out := "PING 1.1.1.1: 56 data bytes\n64 bytes from 1.1.1.1: seq=0 ttl=58 time=12.3 ms\nround-trip min/avg/max = 11.0/12.3/13.5 ms"
	ms, ok := parsePingLatency(out)
	if !ok || ms < 12.0 || ms > 12.6 {
		t.Errorf("parsePingLatency = (%v,%v)", ms, ok)
	}
	if !pingReachable(out) {
		t.Errorf("pingReachable should be true")
	}
	if pingReachable("2 packets transmitted, 0 packets received") {
		t.Errorf("pingReachable should be false on 0 received")
	}
}

func TestParseIfaceBlocksAndPickSta(t *testing.T) {
	t.Parallel()
	gather := `##IFACE wlan0
          ESSID: "VenueAP"
          Mode: Client  Channel: 6
          Signal: -55 dBm
##IFACE wlan1
          ESSID: ""
          Mode: Master  Channel: 36
##PING
64 bytes from 1.1.1.1: time=10 ms`
	ifaces := parseIfaceBlocks(gather)
	if len(ifaces) != 2 {
		t.Fatalf("want 2 ifaces, got %d (%v)", len(ifaces), ifaces)
	}
	name, info := pickStaInterface(ifaces)
	if name != "wlan0" || info.Mode != "Client" {
		t.Errorf("pickSta = %q %+v, want wlan0/Client", name, info)
	}
	if !has5GHzRadio(ifaces) {
		t.Errorf("has5GHzRadio should be true (wlan1 ch36)")
	}
}

func TestExtractRadioCountriesAndDevices(t *testing.T) {
	t.Parallel()
	show := parseUCIShow("wireless.mt798111=wifi-device\nwireless.mt798111.country='IT'\nwireless.mt798112=wifi-device\nwireless.mt798112.country='IT'\nwireless.default=wifi-iface\n")
	countries := extractRadioCountries(show)
	if countries["mt798111"] != "IT" || countries["mt798112"] != "IT" {
		t.Errorf("extractRadioCountries = %v", countries)
	}
	devices := extractWifiDevices(show)
	if len(devices) != 2 || devices[0] != "mt798111" || devices[1] != "mt798112" {
		t.Errorf("extractWifiDevices = %v", devices)
	}
}

func TestGrepUCILines(t *testing.T) {
	t.Parallel()
	out := "wireless.r.country='IT'\nnetwork.lan.proto='static'\nfirewall.z.name='lan'"
	m := grepUCILines(out, "country")
	if len(m) != 1 || !strings.Contains(m[0], "country") {
		t.Errorf("grep by key = %v", m)
	}
	m = grepUCILines(out, "static")
	if len(m) != 1 {
		t.Errorf("grep by value = %v", m)
	}
}

func TestFindAPChannel(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"res":[{"ssid":"Cafe","channel":11},{"ssid":"Other","channel":6}]}`)
	ch, ok := findAPChannel(raw, "Cafe")
	if !ok || ch != 11 {
		t.Errorf("findAPChannel = (%d,%v), want (11,true)", ch, ok)
	}
	if _, ok := findAPChannel(raw, "Missing"); ok {
		t.Errorf("findAPChannel(Missing) should be !ok")
	}
}

func TestDetectRepeaterConnected(t *testing.T) {
	t.Parallel()
	if c, ssid := detectRepeaterConnected([]byte(`{"ssid":"Cafe","status":"connected"}`)); !c || ssid != "Cafe" {
		t.Errorf("connected case = (%v,%q)", c, ssid)
	}
	if c, _ := detectRepeaterConnected([]byte(`{"status":"disconnected"}`)); c {
		t.Errorf("disconnected should be false")
	}
	if c, _ := detectRepeaterConnected([]byte(`{}`)); c {
		t.Errorf("empty should be false")
	}
}
