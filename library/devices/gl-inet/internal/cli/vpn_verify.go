// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/spf13/cobra"
)

// vpnVerifyReport is the structured result of a connection-fidelity check.
type vpnVerifyReport struct {
	VPNActive     bool              `json:"vpn_active"`
	ActiveTunnels []string          `json:"active_tunnels"`
	Egress        egressInfo        `json:"egress"`
	STUN          stunInfo          `json:"stun"`
	DNS           dnsLeakInfo       `json:"dns"`
	ExpectCountry string            `json:"expect_country,omitempty"`
	Checks        map[string]string `json:"checks"`
	Warnings      []string          `json:"warnings"`
	Verdict       string            `json:"verdict"`
	Note          string            `json:"note"`
	BrowserTests  browserTests      `json:"browser_leak_tests"`
}

// browserTests points at external deep-leak checkers that need a real browser
// (WebRTC/canvas/font/JS leaks a CLI cannot observe). Surfaced, and optionally
// opened with --open, so the user can corroborate the CLI's verdict.
type browserTests struct {
	Note string   `json:"note"`
	URLs []string `json:"urls"`
}

type egressInfo struct {
	IP               string         `json:"ip"`
	Country          string         `json:"country"`
	Region           string         `json:"region,omitempty"`
	City             string         `json:"city,omitempty"`
	Org              string         `json:"org,omitempty"`
	Sources          []egressSource `json:"sources"`
	IPConsensus      bool           `json:"ip_consensus"`
	CountryConsensus bool           `json:"country_consensus"`
}

// egressSource is one external IP-geolocation provider's view of the egress.
type egressSource struct {
	Source  string `json:"source"`
	IP      string `json:"ip,omitempty"`
	Country string `json:"country,omitempty"`
	Org     string `json:"org,omitempty"`
	Error   string `json:"error,omitempty"`
}

type stunInfo struct {
	IP         string `json:"ip"`
	Consistent bool   `json:"consistent_with_http"`
	Error      string `json:"error,omitempty"`
}

type dnsServer struct {
	IP      string `json:"ip"`
	Country string `json:"country,omitempty"`
	Name    string `json:"name,omitempty"`
}

type dnsLeakInfo struct {
	Servers    []dnsServer `json:"servers"`
	Count      int         `json:"count"`
	Consistent bool        `json:"consistent_with_egress"`
	Error      string      `json:"error,omitempty"`
}

// pp:data-source live
// browserLeakTestURLs are external, browser-based deep-leak checkers that can
// see WebRTC/canvas/font/JS leaks a CLI cannot. Surfaced for corroboration.
var browserLeakTestURLs = []string{
	"https://ipcheck.ing/",
	"https://www.whatismyip.com/",
}

func newNovelVpnVerifyCmd(flags *rootFlags) *cobra.Command {
	var expectCountry string
	var openBrowser bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify connection fidelity: VPN up, egress country (cross-checked), and no DNS / STUN(WebRTC) leaks",
		Long: "Run a real leak/fidelity check on the router's current connection:\n" +
			"  - public egress IP + geolocation corroborated across several independent providers (ipinfo.io, ip-api.com, ipwho.is, ifconfig.co, Cloudflare trace); disagreement is flagged\n" +
			"  - STUN/UDP public IP vs the HTTP egress IP (the mechanism behind WebRTC IP leaks; a mismatch means UDP is leaking outside the tunnel)\n" +
			"  - DNS leak test (resolves unique probe hostnames and reports which resolvers answered)\n" +
			"  - the router's own VPN client status (is a tunnel actually up)\n" +
			"It also surfaces external browser-based deep-leak checkers (ipcheck.ing, whatismyip.com) for WebRTC/canvas leaks a CLI can't see; pass --open to launch them.\n" +
			"Exits non-zero (8) when a leak or country mismatch is detected.",
		Example:     "  gl-inet-pp-cli vpn verify --expect-country US --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,8"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(out, "would verify connection fidelity (multi-source egress geo, STUN/UDP, DNS leak, VPN status)")
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintln(out, "dry-run: fetch egress geo from multiple providers, STUN public IP, DNS-leak probe, and router VPN status")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rep := vpnVerifyReport{
				ExpectCountry: strings.ToUpper(strings.TrimSpace(expectCountry)),
				Checks:        map[string]string{},
				Note:          "STUN/UDP check covers the network mechanism WebRTC uses to leak IPs; testing an actual browser's WebRTC/canvas/font stack requires a browser — see browser_leak_tests.",
				BrowserTests: browserTests{
					Note: "Open these in the browser you actually use for a full WebRTC/canvas/DNS leak check (re-run with --open to launch them):",
					URLs: browserLeakTestURLs,
				},
			}

			// 1. Router VPN client status (real RPC). Best-effort; absence of a
			// running tunnel is itself a finding, not a hard error.
			if c, err := glClient(flags); err == nil {
				rep.ActiveTunnels = activeRouterTunnels(ctx, c)
				rep.VPNActive = len(rep.ActiveTunnels) > 0
			}

			// 2. Egress IP + geolocation, corroborated across providers.
			rep.Egress = fetchEgressMultiSource(ctx)
			// 3. STUN/UDP public IP.
			rep.STUN = fetchSTUNInfo(ctx)
			// 4. DNS leak test.
			rep.DNS = dnsLeakTest(ctx)

			evaluateVPNVerify(&rep)

			raw, _ := json.Marshal(rep)
			if perr := printOutputWithFlags(out, raw, flags); perr != nil {
				return perr
			}

			// Browser deep-leak tests are a visible side effect: print by
			// default, only launch with --open, and never act under verify.
			if openBrowser && !cliutil.IsVerifyEnv() {
				for _, u := range rep.BrowserTests.URLs {
					if err := openURL(u); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "could not open %s: %v\n", u, err)
					}
				}
			}

			if rep.Verdict == "fail" {
				return &cliError{code: 8, err: fmt.Errorf("connection fidelity check FAILED: %s", strings.Join(rep.Warnings, "; "))}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&expectCountry, "expect-country", "", "Expected egress country (ISO 2-letter, e.g. US); a mismatch fails the check")
	cmd.Flags().BoolVar(&openBrowser, "open", false, "Open the external browser-based leak checkers (ipcheck.ing, whatismyip.com) in your default browser")
	return cmd
}

// openURL launches a URL in the user's default browser, cross-platform.
func openURL(u string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	case "darwin":
		return exec.Command("open", u).Start()
	default:
		return exec.Command("xdg-open", u).Start()
	}
}

// evaluateVPNVerify derives the per-check status, warnings, and overall verdict.
func evaluateVPNVerify(rep *vpnVerifyReport) {
	// Egress country
	if rep.Egress.IP == "" {
		rep.Checks["egress"] = "unknown"
		rep.Warnings = append(rep.Warnings, "could not determine public egress IP")
	} else if rep.ExpectCountry != "" && !strings.EqualFold(rep.Egress.Country, rep.ExpectCountry) {
		rep.Checks["egress_country"] = "fail"
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("egress country %q != expected %q", rep.Egress.Country, rep.ExpectCountry))
	} else {
		rep.Checks["egress_country"] = "ok"
	}

	// Cross-source agreement: providers disagreeing on country/IP can mean a
	// split-tunnel, a load-balanced exit, or one source mislocating the VPN.
	if rep.Egress.IP != "" && !rep.Egress.CountryConsensus {
		rep.Checks["egress_sources"] = "warn"
		rep.Warnings = append(rep.Warnings, "external geolocation providers disagree on the egress country — review egress.sources")
	} else if rep.Egress.IP != "" {
		rep.Checks["egress_sources"] = "ok"
	}

	// VPN active
	if rep.VPNActive {
		rep.Checks["vpn_active"] = "ok"
	} else {
		rep.Checks["vpn_active"] = "warn"
		rep.Warnings = append(rep.Warnings, "no VPN client reported running on the router")
	}

	// STUN consistency (WebRTC-style UDP leak)
	switch {
	case rep.STUN.Error != "" || rep.STUN.IP == "":
		rep.Checks["stun_udp"] = "unknown"
	case rep.Egress.IP != "" && rep.STUN.IP == rep.Egress.IP:
		rep.STUN.Consistent = true
		rep.Checks["stun_udp"] = "ok"
	default:
		rep.STUN.Consistent = false
		rep.Checks["stun_udp"] = "fail"
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("STUN/UDP public IP %s differs from HTTP egress %s — UDP traffic is leaking outside the tunnel (WebRTC-style leak)", rep.STUN.IP, rep.Egress.IP))
	}

	// DNS leak
	switch {
	case rep.DNS.Error != "" || rep.DNS.Count == 0:
		rep.Checks["dns_leak"] = "unknown"
	default:
		leak := false
		for _, s := range rep.DNS.Servers {
			if rep.Egress.Country != "" && s.Country != "" && !strings.EqualFold(s.Country, rep.Egress.Country) {
				leak = true
			}
		}
		rep.DNS.Consistent = !leak
		if leak {
			rep.Checks["dns_leak"] = "fail"
			rep.Warnings = append(rep.Warnings, "a DNS resolver in a different country than the VPN egress answered — possible DNS leak")
		} else {
			rep.Checks["dns_leak"] = "ok"
		}
	}

	// Overall verdict: fail if any check failed; warn if only warnings; pass otherwise.
	rep.Verdict = "pass"
	for _, v := range rep.Checks {
		if v == "fail" {
			rep.Verdict = "fail"
			break
		}
	}
	if rep.Verdict != "fail" {
		for _, v := range rep.Checks {
			if v == "warn" {
				rep.Verdict = "warn"
				break
			}
		}
	}
}

// activeRouterTunnels returns the names of VPN tunnels the router reports as
// running. Firmware 4.x exposes a unified `vpn-client.get_status` whose
// status_list holds the enabled tunnel(s); older firmware used per-module
// get_status, which is tried as a fallback. Version-aware by construction.
func activeRouterTunnels(ctx context.Context, c interface {
	Call(context.Context, string, string, any) (json.RawMessage, error)
}) []string {
	var active []string
	// Preferred (4.x): unified vpn-client.get_status -> {status_list:[{enabled,type,peer_name,...}]}
	if data, err := c.Call(ctx, "vpn-client", "get_status", nil); err == nil {
		var resp struct {
			StatusList []map[string]any `json:"status_list"`
		}
		if json.Unmarshal(data, &resp) == nil {
			for _, st := range resp.StatusList {
				enabled, _ := st["enabled"].(bool)
				if !enabled {
					continue
				}
				name := firstString(st, "peer_name", "name", "group_name", "main_server", "server")
				if t := firstString(st, "type"); t != "" {
					if name == "" {
						name = t
					} else {
						name = name + " (" + t + ")"
					}
				}
				if name == "" {
					name = "vpn"
				}
				active = append(active, name)
			}
		}
		if len(active) > 0 {
			return active
		}
	}
	// Fallback (older firmware): per-module get_status.
	for _, m := range []string{"wg-client", "ovpn-client"} {
		data, err := c.Call(ctx, m, "get_status", nil)
		if err != nil {
			continue
		}
		var st map[string]any
		if json.Unmarshal(data, &st) != nil {
			continue
		}
		if statusLooksConnected(st) {
			name := firstString(st, "name", "group_name", "main_server", "server")
			if name == "" {
				name = m
			}
			active = append(active, name)
		}
	}
	return active
}

func statusLooksConnected(st map[string]any) bool {
	for _, k := range []string{"status", "state"} {
		if v, ok := st[k]; ok {
			s := strings.ToLower(fmt.Sprint(v))
			if strings.Contains(s, "disconnect") {
				continue
			}
			if strings.Contains(s, "connect") || s == "1" || s == "running" || s == "online" {
				return true
			}
			if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
				return true
			}
		}
	}
	if v, ok := st["enable"]; ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
		if fmt.Sprint(v) == "1" {
			return true
		}
	}
	return false
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// fetchEgressMultiSource queries several independent IP-geolocation providers
// concurrently and returns the consensus IP/country plus each provider's raw
// view. Cross-checking guards against a single source being wrong, stale, or
// mislocating a VPN exit; disagreement is surfaced for the verdict.
func fetchEgressMultiSource(ctx context.Context) egressInfo {
	fetchers := []func(context.Context) egressSource{
		fetchEgressIPInfo,
		fetchEgressIPApiCo,
		fetchEgressIPWho,
		fetchEgressIfconfig,
		fetchEgressCloudflare,
	}
	results := make([]egressSource, len(fetchers))
	var wg sync.WaitGroup
	for i, fn := range fetchers {
		wg.Add(1)
		go func(i int, fn func(context.Context) egressSource) {
			defer wg.Done()
			results[i] = fn(ctx)
		}(i, fn)
	}
	wg.Wait()

	info := egressInfo{Sources: results}
	ipCounts := map[string]int{}
	ccCounts := map[string]int{}
	for _, s := range results {
		if s.Error != "" {
			continue
		}
		if s.IP != "" {
			ipCounts[s.IP]++
		}
		if s.Country != "" {
			ccCounts[strings.ToUpper(s.Country)]++
		}
	}
	info.IP, info.IPConsensus = topAgreement(ipCounts)
	info.Country, info.CountryConsensus = topAgreement(ccCounts)
	// Fill region/city/org from the richest source matching the consensus IP
	// (ipinfo first), else the first non-error source.
	for _, s := range results {
		if s.Error == "" && (s.IP == info.IP || info.IP == "") && s.Org != "" {
			info.Org = s.Org
			break
		}
	}
	return info
}

// topAgreement returns the most common value and whether every contributing
// source agreed on it (no dissenting non-empty values).
func topAgreement(counts map[string]int) (string, bool) {
	best, bestN, total := "", 0, 0
	for v, n := range counts {
		total += n
		if n > bestN {
			best, bestN = v, n
		}
	}
	if best == "" {
		return "", false
	}
	return best, bestN == total
}

func getEgressJSON(ctx context.Context, url string, v any) error {
	cctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return json.Unmarshal(body, v)
}

func fetchEgressIPInfo(ctx context.Context) egressSource {
	s := egressSource{Source: "ipinfo.io"}
	var raw struct{ IP, Country, Org string }
	if err := getEgressJSON(ctx, "https://ipinfo.io/json", &raw); err != nil {
		s.Error = err.Error()
		return s
	}
	s.IP, s.Country, s.Org = raw.IP, raw.Country, raw.Org
	return s
}

// fetchEgressIPApiCo queries ipapi.co over HTTPS. ip-api.com's free tier is
// HTTP-only, which a hostile-LAN MITM (the exact threat this tool checks for)
// could spoof; every egress source here must be TLS-protected so a single
// network attacker can't poison the consensus.
func fetchEgressIPApiCo(ctx context.Context) egressSource {
	s := egressSource{Source: "ipapi.co"}
	var raw struct {
		IP      string `json:"ip"`
		Country string `json:"country"` // 2-letter ISO code
		Org     string `json:"org"`
	}
	if err := getEgressJSON(ctx, "https://ipapi.co/json/", &raw); err != nil {
		s.Error = err.Error()
		return s
	}
	s.IP, s.Country, s.Org = raw.IP, raw.Country, raw.Org
	return s
}

func fetchEgressIPWho(ctx context.Context) egressSource {
	s := egressSource{Source: "ipwho.is"}
	var raw struct {
		IP          string `json:"ip"`
		CountryCode string `json:"country_code"`
		Connection  struct {
			Isp, Org string
		} `json:"connection"`
	}
	if err := getEgressJSON(ctx, "https://ipwho.is/", &raw); err != nil {
		s.Error = err.Error()
		return s
	}
	s.IP, s.Country = raw.IP, raw.CountryCode
	s.Org = raw.Connection.Isp
	if s.Org == "" {
		s.Org = raw.Connection.Org
	}
	return s
}

func fetchEgressIfconfig(ctx context.Context) egressSource {
	s := egressSource{Source: "ifconfig.co"}
	var raw struct {
		IP         string `json:"ip"`
		CountryISO string `json:"country_iso"`
		ASNOrg     string `json:"asn_org"`
	}
	if err := getEgressJSON(ctx, "https://ifconfig.co/json", &raw); err != nil {
		s.Error = err.Error()
		return s
	}
	s.IP, s.Country, s.Org = raw.IP, raw.CountryISO, raw.ASNOrg
	return s
}

// fetchEgressCloudflare reads Cloudflare's edge trace (the same data point
// ipcheck.ing and similar tools use). It also reports whether Cloudflare WARP
// is in play, which would itself be a separate tunnel.
func fetchEgressCloudflare(ctx context.Context) egressSource {
	s := egressSource{Source: "cloudflare-trace"}
	cctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, "https://1.1.1.1/cdn-cgi/trace", nil)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
	fields := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			fields[k] = v
		}
	}
	s.IP, s.Country = fields["ip"], strings.ToUpper(fields["loc"])
	org := "Cloudflare edge " + fields["colo"]
	if fields["warp"] == "on" {
		org += " (WARP on)"
	}
	s.Org = strings.TrimSpace(org)
	if s.IP == "" {
		s.Error = "no ip in cloudflare trace"
	}
	return s
}

// stunMagicCookie is the fixed STUN magic cookie (RFC 5389).
const stunMagicCookie uint32 = 0x2112A442

// fetchSTUNInfo performs a real STUN binding request (RFC 5389) over UDP using
// only the standard library, and returns the server-reflexive public IP. A
// result differing from the HTTP egress indicates UDP is leaking outside the
// VPN tunnel — the mechanism behind WebRTC IP leaks. Implemented with stdlib
// (no pion/stun) so the CLI carries no DTLS dependency.
func fetchSTUNInfo(ctx context.Context) stunInfo {
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(cctx, "udp4", "stun.l.google.com:19302")
	if err != nil {
		return stunInfo{Error: "stun dial: " + err.Error()}
	}
	defer conn.Close()
	if deadline, ok := cctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	// Binding Request: 20-byte header, no attributes.
	req := make([]byte, 20)
	binary.BigEndian.PutUint16(req[0:], 0x0001) // message type: Binding Request
	binary.BigEndian.PutUint16(req[2:], 0x0000) // message length
	binary.BigEndian.PutUint32(req[4:], stunMagicCookie)
	txID := req[8:20]
	if _, err := rand.Read(txID); err != nil {
		return stunInfo{Error: "stun txid: " + err.Error()}
	}
	if _, err := conn.Write(req); err != nil {
		return stunInfo{Error: "stun write: " + err.Error()}
	}

	resp := make([]byte, 1024)
	n, err := conn.Read(resp)
	if err != nil {
		return stunInfo{Error: "stun read: " + err.Error()}
	}
	ip, perr := parseSTUNMappedIP(resp[:n], txID)
	if perr != "" {
		return stunInfo{Error: perr}
	}
	return stunInfo{IP: ip}
}

// parseSTUNMappedIP extracts the IPv4 address from a STUN Binding Success
// response's XOR-MAPPED-ADDRESS (0x0020), falling back to MAPPED-ADDRESS
// (0x0001). Returns the IP string, or an error description.
func parseSTUNMappedIP(msg, txID []byte) (string, string) {
	if len(msg) < 20 {
		return "", "stun response too short"
	}
	// Header: type(2) length(2) cookie(4) txid(12). Attributes follow.
	attrLen := int(binary.BigEndian.Uint16(msg[2:4]))
	pos := 20
	end := 20 + attrLen
	if end > len(msg) {
		end = len(msg)
	}
	for pos+4 <= end {
		atype := binary.BigEndian.Uint16(msg[pos : pos+2])
		alen := int(binary.BigEndian.Uint16(msg[pos+2 : pos+4]))
		vstart := pos + 4
		vend := vstart + alen
		if vend > len(msg) {
			break
		}
		val := msg[vstart:vend]
		switch atype {
		case 0x0020: // XOR-MAPPED-ADDRESS
			if ip, ok := decodeMappedAddress(val, true); ok {
				return ip, ""
			}
		case 0x0001: // MAPPED-ADDRESS (legacy)
			if ip, ok := decodeMappedAddress(val, false); ok {
				return ip, ""
			}
		}
		// Attributes are padded to 4-byte boundaries.
		pos = vend
		if pad := alen % 4; pad != 0 {
			pos += 4 - pad
		}
	}
	return "", "no mapped address in stun response"
}

// decodeMappedAddress decodes a (XOR-)MAPPED-ADDRESS attribute value for IPv4.
// Layout: reserved(1) family(1) port(2) address(4). When xor is true the
// address bytes are XOR'd with the magic cookie.
func decodeMappedAddress(val []byte, xor bool) (string, bool) {
	if len(val) < 8 || val[1] != 0x01 { // family 0x01 = IPv4
		return "", false
	}
	addr := make([]byte, 4)
	copy(addr, val[4:8])
	if xor {
		cookie := make([]byte, 4)
		binary.BigEndian.PutUint32(cookie, stunMagicCookie)
		for i := range addr {
			addr[i] ^= cookie[i]
		}
	}
	return net.IPv4(addr[0], addr[1], addr[2], addr[3]).String(), true
}

// dnsLeakTest runs the standard dnsleaktest-style probe: resolve several unique
// hostnames under a per-run id, then ask the test service which resolvers
// performed the upstream lookups. Honest, real DNS resolution through whatever
// resolver the router hands the host.
func dnsLeakTest(ctx context.Context) dnsLeakInfo {
	id := dnsLeakID()
	if id == "" {
		return dnsLeakInfo{Error: "could not generate test id"}
	}
	// Trigger DNS resolutions of unique probe hostnames. Failures are expected
	// (the names don't resolve to A records); the side effect — the upstream
	// resolver querying bash.ws — is what the test records.
	resolver := net.Resolver{}
	for i := 0; i < 4; i++ {
		rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, _ = resolver.LookupHost(rctx, fmt.Sprintf("%d.%s.bash.ws", i, id))
		cancel()
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	url := "https://bash.ws/dnsleak/test/" + id + "?json"
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return dnsLeakInfo{Error: err.Error()}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return dnsLeakInfo{Error: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<18))
	var rows []struct {
		IP, Country, ASN, Name, Type string
	}
	if json.Unmarshal(body, &rows) != nil {
		return dnsLeakInfo{Error: "could not parse dns leak result"}
	}
	var info dnsLeakInfo
	for _, r := range rows {
		if strings.EqualFold(r.Type, "dns") {
			info.Servers = append(info.Servers, dnsServer{IP: r.IP, Country: r.Country, Name: r.Name})
		}
	}
	info.Count = len(info.Servers)
	return info
}

// dnsLeakID returns a short unique id for a DNS leak run. Uses the monotonic
// clock plus a process-stable component; uniqueness across concurrent runs is
// not required for a single user's manual check.
func dnsLeakID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
