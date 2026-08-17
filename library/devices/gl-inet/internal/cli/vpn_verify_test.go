// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestEvaluateVPNVerify(t *testing.T) {
	cases := []struct {
		name        string
		rep         vpnVerifyReport
		wantVerdict string
		wantCheck   map[string]string
	}{
		{
			name: "clean pass",
			rep: vpnVerifyReport{
				ExpectCountry: "US",
				VPNActive:     true,
				Egress:        egressInfo{IP: "1.2.3.4", Country: "US", CountryConsensus: true, IPConsensus: true},
				STUN:          stunInfo{IP: "1.2.3.4"},
				DNS:           dnsLeakInfo{Servers: []dnsServer{{IP: "1.2.3.5", Country: "US"}}, Count: 1},
				Checks:        map[string]string{},
			},
			wantVerdict: "pass",
			wantCheck:   map[string]string{"egress_country": "ok", "vpn_active": "ok", "stun_udp": "ok", "dns_leak": "ok"},
		},
		{
			name: "stun udp leak fails",
			rep: vpnVerifyReport{
				VPNActive: true,
				Egress:    egressInfo{IP: "1.2.3.4", Country: "US"},
				STUN:      stunInfo{IP: "9.9.9.9"},
				DNS:       dnsLeakInfo{Servers: []dnsServer{{IP: "1.2.3.5", Country: "US"}}, Count: 1},
				Checks:    map[string]string{},
			},
			wantVerdict: "fail",
			wantCheck:   map[string]string{"stun_udp": "fail"},
		},
		{
			name: "country mismatch fails",
			rep: vpnVerifyReport{
				ExpectCountry: "US",
				VPNActive:     true,
				Egress:        egressInfo{IP: "1.2.3.4", Country: "JP"},
				STUN:          stunInfo{IP: "1.2.3.4"},
				Checks:        map[string]string{},
			},
			wantVerdict: "fail",
			wantCheck:   map[string]string{"egress_country": "fail"},
		},
		{
			name: "dns leak different country fails",
			rep: vpnVerifyReport{
				VPNActive: true,
				Egress:    egressInfo{IP: "1.2.3.4", Country: "US"},
				STUN:      stunInfo{IP: "1.2.3.4"},
				DNS:       dnsLeakInfo{Servers: []dnsServer{{IP: "8.8.8.8", Country: "DE"}}, Count: 1},
				Checks:    map[string]string{},
			},
			wantVerdict: "fail",
			wantCheck:   map[string]string{"dns_leak": "fail"},
		},
		{
			name: "no vpn warns only",
			rep: vpnVerifyReport{
				VPNActive: false,
				Egress:    egressInfo{IP: "1.2.3.4", Country: "US"},
				STUN:      stunInfo{IP: "1.2.3.4"},
				DNS:       dnsLeakInfo{Servers: []dnsServer{{IP: "1.2.3.5", Country: "US"}}, Count: 1},
				Checks:    map[string]string{},
			},
			wantVerdict: "warn",
			wantCheck:   map[string]string{"vpn_active": "warn"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := tc.rep
			evaluateVPNVerify(&rep)
			if rep.Verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q (warnings: %v)", rep.Verdict, tc.wantVerdict, rep.Warnings)
			}
			for k, want := range tc.wantCheck {
				if rep.Checks[k] != want {
					t.Errorf("check[%q] = %q, want %q", k, rep.Checks[k], want)
				}
			}
		})
	}
}

func TestStatusLooksConnected(t *testing.T) {
	if !statusLooksConnected(map[string]any{"status": "connected"}) {
		t.Error("expected connected for status=connected")
	}
	if !statusLooksConnected(map[string]any{"enable": true}) {
		t.Error("expected connected for enable=true")
	}
	if statusLooksConnected(map[string]any{"status": "disconnected"}) {
		t.Error("expected not-connected for status=disconnected")
	}
}
