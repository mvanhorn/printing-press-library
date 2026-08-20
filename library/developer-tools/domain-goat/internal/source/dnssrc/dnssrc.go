// Package dnssrc runs DNS heuristics used to corroborate availability.
package dnssrc

import (
	"context"
	"net"
	"strings"
	"time"
)

// Result holds DNS heuristic findings.
type Result struct {
	FQDN      string   `json:"fqdn"`
	NS        []string `json:"ns,omitempty"`
	A         []string `json:"a,omitempty"`
	AAAA      []string `json:"aaaa,omitempty"`
	MX        []string `json:"mx,omitempty"`
	HasAny    bool     `json:"has_any"`
	Available bool     `json:"available"`
}

// Probe runs concurrent A, NS, MX lookups with a 5s deadline.
func Probe(ctx context.Context, fqdn string) (*Result, error) {
	r := &Result{FQDN: fqdn}
	resolver := &net.Resolver{PreferGo: true}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if ns, err := resolver.LookupNS(pctx, fqdn); err == nil {
		for _, n := range ns {
			r.NS = append(r.NS, strings.TrimSuffix(n.Host, "."))
		}
	}
	if ips, err := resolver.LookupIP(pctx, "ip4", fqdn); err == nil {
		for _, ip := range ips {
			r.A = append(r.A, ip.String())
		}
	}
	if ips, err := resolver.LookupIP(pctx, "ip6", fqdn); err == nil {
		for _, ip := range ips {
			r.AAAA = append(r.AAAA, ip.String())
		}
	}
	if mx, err := resolver.LookupMX(pctx, fqdn); err == nil {
		for _, m := range mx {
			r.MX = append(r.MX, strings.TrimSuffix(m.Host, "."))
		}
	}
	r.HasAny = len(r.NS) > 0 || len(r.A) > 0 || len(r.AAAA) > 0 || len(r.MX) > 0
	r.Available = !r.HasAny // heuristic only
	return r, nil
}

// IsRegisteredErr inspects an error to decide whether the failure was a
// legitimate NXDOMAIN. Many resolver errors are network glitches.
func IsRegisteredErr(err error) bool {
	if err == nil {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such host") || strings.Contains(msg, "nxdomain") {
		return false
	}
	return true
}
