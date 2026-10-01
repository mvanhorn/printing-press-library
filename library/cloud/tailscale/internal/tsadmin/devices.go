// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package tsadmin

import (
	"math"
	"net/netip"
	"sort"
	"strings"
	"time"
)

// Device is the subset of the API device object the novel commands use.
type Device struct {
	ID                 string   `json:"id"`
	NodeID             string   `json:"nodeId"`
	Name               string   `json:"name"`
	Hostname           string   `json:"hostname"`
	Addresses          []string `json:"addresses"`
	User               string   `json:"user"`
	OS                 string   `json:"os"`
	Tags               []string `json:"tags"`
	Created            string   `json:"created"`
	Expires            string   `json:"expires"`
	KeyExpiryDisabled  bool     `json:"keyExpiryDisabled"`
	LastSeen           string   `json:"lastSeen"`
	ConnectedToControl bool     `json:"connectedToControl"`
	Authorized         bool     `json:"authorized"`
	IsExternal         bool     `json:"isExternal"`
	UpdateAvailable    bool     `json:"updateAvailable"`
	ClientVersion      string   `json:"clientVersion"`
	AdvertisedRoutes   []string `json:"advertisedRoutes"`
	EnabledRoutes      []string `json:"enabledRoutes"`
}

// MachineName returns the first label of the MagicDNS name, which is the
// unique machine name the admin console shows. OS hostnames are often
// generic ("localhost") or repeated across machines.
func (d Device) MachineName() string {
	first, _, _ := strings.Cut(strings.TrimSuffix(d.Name, "."), ".")
	return first
}

// Label returns the most human-friendly unique identifier for a device.
func (d Device) Label() string {
	if m := d.MachineName(); m != "" {
		return m
	}
	if d.Hostname != "" {
		return d.Hostname
	}
	return d.NodeID
}

// PreferredID returns nodeId when present, else the legacy numeric id.
func (d Device) PreferredID() string {
	if d.NodeID != "" {
		return d.NodeID
	}
	return d.ID
}

// FirstIPv4 returns the device's Tailscale IPv4 address, or its first
// address when it has no IPv4.
func (d Device) FirstIPv4() string {
	for _, a := range d.Addresses {
		if ip, err := netip.ParseAddr(a); err == nil && ip.Is4() {
			return a
		}
	}
	if len(d.Addresses) > 0 {
		return d.Addresses[0]
	}
	return ""
}

// MatchDevices resolves a selector against a device list. Tiers are tried in
// order and the first tier with any match wins: exact nodeId/id, exact
// Tailscale address, hostname (case-insensitive), full MagicDNS name, then
// the first label of the MagicDNS name. More than one result means the
// selector is ambiguous.
func MatchDevices(devs []Device, sel string) []Device {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return nil
	}
	lsel := strings.ToLower(strings.TrimSuffix(sel, "."))
	tiers := []func(Device) bool{
		func(d Device) bool { return d.NodeID == sel || d.ID == sel },
		func(d Device) bool {
			for _, a := range d.Addresses {
				if a == sel {
					return true
				}
			}
			return false
		},
		func(d Device) bool { return strings.EqualFold(d.Hostname, sel) },
		func(d Device) bool { return strings.ToLower(strings.TrimSuffix(d.Name, ".")) == lsel },
		func(d Device) bool {
			first, _, _ := strings.Cut(strings.ToLower(d.Name), ".")
			return first != "" && first == lsel
		},
	}
	for _, tier := range tiers {
		var out []Device
		for _, d := range devs {
			if tier(d) {
				out = append(out, d)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// Expiry statuses.
const (
	ExpiryNever    = "never"
	ExpiryExpired  = "expired"
	ExpiryExpiring = "expiring"
	ExpiryOK       = "ok"
	ExpiryUnknown  = "unknown"
)

// ParseAPITime parses an API timestamp. The zero time ("0001-01-01T00:00:00Z")
// and empty strings report ok=false.
func ParseAPITime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, false
	}
	if t.Year() <= 1 {
		return time.Time{}, false
	}
	return t, true
}

// ClassifyExpiry returns the status and whether the item is flagged for the
// given window (in days). Items with expiry disabled are never flagged.
func ClassifyExpiry(expires string, disabled bool, within int, now time.Time) (status string, daysLeft *int, flagged bool) {
	if disabled {
		return ExpiryNever, nil, false
	}
	t, ok := ParseAPITime(expires)
	if !ok {
		return ExpiryUnknown, nil, false
	}
	d := int(math.Floor(t.Sub(now).Hours() / 24))
	switch {
	case !t.After(now):
		return ExpiryExpired, &d, true
	case d <= within:
		return ExpiryExpiring, &d, true
	default:
		return ExpiryOK, &d, false
	}
}

// ExpiryItem is one row of the expiry report.
type ExpiryItem struct {
	Kind              string   `json:"kind"`
	ID                string   `json:"id"`
	Machine           string   `json:"machine,omitempty"`
	Hostname          string   `json:"hostname,omitempty"`
	Name              string   `json:"name,omitempty"`
	User              string   `json:"user,omitempty"`
	OS                string   `json:"os,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	KeyType           string   `json:"key_type,omitempty"`
	Description       string   `json:"description,omitempty"`
	Expires           string   `json:"expires,omitempty"`
	DaysLeft          *int     `json:"days_left"`
	KeyExpiryDisabled bool     `json:"key_expiry_disabled"`
	LastSeen          string   `json:"last_seen,omitempty"`
	Status            string   `json:"status"`
	Flagged           bool     `json:"flagged"`
	Self              bool     `json:"self,omitempty"`
}

// SortExpiry orders items soonest-first; items without a day count go last.
func SortExpiry(items []ExpiryItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].DaysLeft, items[j].DaysLeft
		switch {
		case a == nil && b == nil:
			return items[i].Hostname+items[i].Description < items[j].Hostname+items[j].Description
		case a == nil:
			return false
		case b == nil:
			return true
		default:
			return *a < *b
		}
	})
}
