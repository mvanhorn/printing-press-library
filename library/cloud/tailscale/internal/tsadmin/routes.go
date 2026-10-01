// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

// Package tsadmin holds the pure logic behind the Tailscale admin novel
// commands: route set arithmetic, key-expiry classification, device
// selectors, policy-file patching, line diffs, and share filtering. Nothing
// here performs I/O, so every rule is covered by table tests.
package tsadmin

import (
	"fmt"
	"net/netip"
	"strings"
)

// The exit-node route pair. Tailscale treats a device as an exit node only
// when both are advertised and enabled.
const (
	ExitRouteV4 = "0.0.0.0/0"
	ExitRouteV6 = "::/0"
)

// ExitRoutes returns the exit-node route pair in canonical order.
func ExitRoutes() []string { return []string{ExitRouteV4, ExitRouteV6} }

// IsExitRoute reports whether r is one half of the exit-node pair.
func IsExitRoute(r string) bool { return r == ExitRouteV4 || r == ExitRouteV6 }

// NormalizeRoute parses a CIDR and returns its canonical string. A prefix
// with host bits set is rejected rather than silently masked, because the
// caller almost certainly typed the wrong network.
func NormalizeRoute(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty route")
	}
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return "", fmt.Errorf("route %q is not a CIDR prefix (want e.g. 192.168.1.0/24)", s)
	}
	masked := p.Masked()
	if masked != p {
		return "", fmt.Errorf("route %q has host bits set; did you mean %s?", s, masked.String())
	}
	return masked.String(), nil
}

// NormalizeRoutes validates every route, then returns them normalized and
// de-duplicated in first-occurrence order.
func NormalizeRoutes(in []string) ([]string, error) {
	for _, r := range in {
		if _, err := NormalizeRoute(r); err != nil {
			return nil, err
		}
	}
	return canonicalSet(in), nil
}

// canonicalSet normalizes routes reported by the API. Unparseable values are
// kept verbatim so they are never dropped from a write.
func canonicalSet(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, r := range in {
		n, err := NormalizeRoute(r)
		if err != nil {
			n = strings.TrimSpace(r)
		}
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func toSet(in []string) map[string]bool {
	m := make(map[string]bool, len(in))
	for _, r := range in {
		m[r] = true
	}
	return m
}

func minus(a, b []string) []string {
	bs := toSet(b)
	out := make([]string, 0)
	for _, r := range a {
		if !bs[r] {
			out = append(out, r)
		}
	}
	return out
}

// SameRouteSet reports whether two route lists hold the same routes,
// ignoring order and duplicates.
func SameRouteSet(a, b []string) bool {
	ac, bc := canonicalSet(a), canonicalSet(b)
	if len(ac) != len(bc) {
		return false
	}
	bs := toSet(bc)
	for _, r := range ac {
		if !bs[r] {
			return false
		}
	}
	return true
}

// RoutePlan is the result of computing a new enabled-route set. After is the
// complete list that must be sent, because the API replaces the whole set.
type RoutePlan struct {
	Before       []string `json:"before"`
	After        []string `json:"after"`
	Added        []string `json:"added"`
	Removed      []string `json:"removed"`
	Advertised   []string `json:"advertised"`
	Unadvertised []string `json:"unadvertised_requested"`
	NotEnabled   []string `json:"not_enabled_requested"`
	NoChange     bool     `json:"no_change"`
}

// AddedExitRoutes counts how many halves of the exit-node pair the plan adds.
func (p RoutePlan) AddedExitRoutes() int {
	n := 0
	for _, r := range p.Added {
		if IsExitRoute(r) {
			n++
		}
	}
	return n
}

func newRoutePlan(advertised, enabled []string) RoutePlan {
	return RoutePlan{Before: canonicalSet(enabled), Advertised: canonicalSet(advertised), Added: []string{}, Removed: []string{}, Unadvertised: []string{}, NotEnabled: []string{}}
}

// PlanApprove returns enabled ∪ requested, keeping every route that is
// already enabled. Unadvertised lists requested routes the device does not
// advertise, so the caller can refuse or warn.
func PlanApprove(advertised, enabled, requested []string) RoutePlan {
	plan := newRoutePlan(advertised, enabled)
	req := canonicalSet(requested)
	advSet := toSet(plan.Advertised)
	after := append([]string{}, plan.Before...)
	beforeSet := toSet(plan.Before)
	for _, r := range req {
		if !advSet[r] {
			plan.Unadvertised = append(plan.Unadvertised, r)
		}
		if !beforeSet[r] {
			after = append(after, r)
			plan.Added = append(plan.Added, r)
			beforeSet[r] = true
		}
	}
	plan.After = after
	plan.NoChange = len(plan.Added) == 0
	return plan
}

// PlanUnapprove returns enabled minus requested. NotEnabled lists requested
// routes that were not enabled to begin with.
func PlanUnapprove(advertised, enabled, requested []string) RoutePlan {
	plan := newRoutePlan(advertised, enabled)
	req := canonicalSet(requested)
	reqSet := toSet(req)
	beforeSet := toSet(plan.Before)
	after := make([]string, 0, len(plan.Before))
	for _, r := range plan.Before {
		if reqSet[r] {
			plan.Removed = append(plan.Removed, r)
			continue
		}
		after = append(after, r)
	}
	for _, r := range req {
		if !beforeSet[r] {
			plan.NotEnabled = append(plan.NotEnabled, r)
		}
	}
	plan.After = after
	plan.NoChange = len(plan.Removed) == 0
	return plan
}

// Exit-node states reported by ComputeRouteState.
const (
	ExitNone                 = "none"
	ExitApproved             = "approved"
	ExitPending              = "pending"
	ExitPartial              = "partial"
	ExitApprovedNotAdvertise = "approved-not-advertised"
)

// RouteState summarizes one device's routes.
type RouteState struct {
	Advertised []string `json:"advertised"`
	Enabled    []string `json:"enabled"`
	Pending    []string `json:"pending"`
	Stale      []string `json:"stale"`
	Subnets    []string `json:"subnets_advertised"`
	ExitNode   string   `json:"exit_node"`
}

// ComputeRouteState derives pending (advertised but not enabled), stale
// (enabled but not advertised), and exit-node state.
func ComputeRouteState(advertised, enabled []string) RouteState {
	adv := canonicalSet(advertised)
	en := canonicalSet(enabled)
	st := RouteState{Advertised: adv, Enabled: en, Pending: minus(adv, en), Stale: minus(en, adv), Subnets: []string{}}
	for _, r := range adv {
		if !IsExitRoute(r) {
			st.Subnets = append(st.Subnets, r)
		}
	}
	advSet, enSet := toSet(adv), toSet(en)
	advExit := 0
	enExit := 0
	for _, r := range ExitRoutes() {
		if advSet[r] {
			advExit++
		}
		if enSet[r] {
			enExit++
		}
	}
	switch {
	case advExit == 0 && enExit == 0:
		st.ExitNode = ExitNone
	case advExit == 2 && enExit == 2:
		st.ExitNode = ExitApproved
	case advExit == 0 && enExit == 2:
		st.ExitNode = ExitApprovedNotAdvertise
	case advExit > 0 && enExit == 0:
		st.ExitNode = ExitPending
	default:
		st.ExitNode = ExitPartial
	}
	return st
}
