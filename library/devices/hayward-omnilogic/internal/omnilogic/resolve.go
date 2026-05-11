package omnilogic

import (
	"fmt"
	"strconv"
	"strings"
)

// ResolveSite returns the MspSystemID + name to use given an optional ID
// hint. If hint is 0, the only site is used; if multiple sites are
// registered with no hint, an error names them.
func ResolveSite(sites []Site, hint int) (Site, error) {
	if hint != 0 {
		for _, s := range sites {
			if s.MspSystemID == hint {
				return s, nil
			}
		}
		return Site{}, fmt.Errorf("no site with MspSystemID=%d (known: %s)", hint, listSites(sites))
	}
	switch len(sites) {
	case 0:
		return Site{}, fmt.Errorf("no sites registered to this account")
	case 1:
		return sites[0], nil
	default:
		return Site{}, fmt.Errorf("multiple sites registered; pass --msp-system-id to choose (known: %s)", listSites(sites))
	}
}

func listSites(sites []Site) string {
	var parts []string
	for _, s := range sites {
		parts = append(parts, fmt.Sprintf("%d=%q", s.MspSystemID, s.BackyardName))
	}
	return strings.Join(parts, ", ")
}

// ResolveHeater finds a heater by name (case-insensitive substring) and
// returns the BoW's SystemID + the heater's SystemID needed for SetHeater*.
// When name is empty and exactly one BoW has a heater, that heater wins.
func ResolveHeater(cfg *MspConfig, name string) (poolID, heaterID int, heater Heater, err error) {
	type match struct {
		poolID   int
		heaterID int
		heater   Heater
	}
	var matches []match
	nl := strings.ToLower(name)
	for _, bow := range cfg.BodiesOfWater {
		bowID := atoiSafe(bow.SystemID)
		for _, h := range bow.Heaters {
			hid := atoiSafe(h.SystemID)
			if nl == "" {
				matches = append(matches, match{bowID, hid, h})
				continue
			}
			hname := strings.ToLower(h.Name)
			if hname == nl || strings.Contains(hname, nl) {
				matches = append(matches, match{bowID, hid, h})
			}
		}
	}
	switch len(matches) {
	case 0:
		if name == "" {
			return 0, 0, Heater{}, fmt.Errorf("no heaters configured for this site")
		}
		return 0, 0, Heater{}, fmt.Errorf("no heater matched %q (use 'config get' to list heaters)", name)
	case 1:
		m := matches[0]
		return m.poolID, m.heaterID, m.heater, nil
	default:
		var names []string
		for _, m := range matches {
			names = append(names, m.heater.Name)
		}
		return 0, 0, Heater{}, fmt.Errorf("heater name %q matched multiple: %s", name, strings.Join(names, ", "))
	}
}

// ResolveEquipment finds any equipment item (pump, light, valve, relay) by
// name across all BoWs. Returns the BoW's SystemID + the equipment's
// SystemID. Pass kindFilter to restrict to "pump", "light", "valve",
// "relay", or "" for any.
func ResolveEquipment(cfg *MspConfig, name, kindFilter string) (poolID, equipID int, kind, displayName string, err error) {
	type match struct {
		poolID int
		eqID   int
		kind   string
		name   string
	}
	var matches []match
	nl := strings.ToLower(name)
	for _, bow := range cfg.BodiesOfWater {
		bowID := atoiSafe(bow.SystemID)
		add := func(k, n, sid string) {
			if (kindFilter == "" || kindFilter == k) && (nl == "" || strings.Contains(strings.ToLower(n), nl)) {
				matches = append(matches, match{bowID, atoiSafe(sid), k, n})
			}
		}
		for _, p := range bow.Pumps {
			add("pump", p.Name, p.SystemID)
		}
		for _, l := range bow.Lights {
			add("light", l.Name, l.SystemID)
		}
		for _, r := range bow.Relays {
			add("relay", r.Name, r.SystemID)
		}
	}
	// Also try backyard-level relays for "relay" or empty kindFilter.
	if kindFilter == "" || kindFilter == "relay" {
		for _, r := range cfg.Relays {
			if nl == "" || strings.Contains(strings.ToLower(r.Name), nl) {
				matches = append(matches, match{0, atoiSafe(r.SystemID), "relay", r.Name})
			}
		}
	}
	switch len(matches) {
	case 0:
		return 0, 0, "", "", fmt.Errorf("no equipment matched %q", name)
	case 1:
		m := matches[0]
		return m.poolID, m.eqID, m.kind, m.name, nil
	default:
		var names []string
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s (%s)", m.name, m.kind))
		}
		return 0, 0, "", "", fmt.Errorf("name %q matched multiple: %s", name, strings.Join(names, ", "))
	}
}

// ResolveChlor finds the (single) chlorinator on a BoW. Pool-level call so
// returns poolID + chlorID.
func ResolveChlor(cfg *MspConfig, bowName string) (poolID, chlorID int, err error) {
	type match struct {
		poolID  int
		chlorID int
		bowName string
	}
	var matches []match
	nl := strings.ToLower(bowName)
	for _, bow := range cfg.BodiesOfWater {
		if bow.Chlorinator == nil {
			continue
		}
		if bowName == "" || strings.Contains(strings.ToLower(bow.Name), nl) {
			matches = append(matches, match{atoiSafe(bow.SystemID), atoiSafe(bow.Chlorinator.SystemID), bow.Name})
		}
	}
	switch len(matches) {
	case 0:
		return 0, 0, fmt.Errorf("no chlorinator found%s", filterSuffix(bowName))
	case 1:
		return matches[0].poolID, matches[0].chlorID, nil
	default:
		var names []string
		for _, m := range matches {
			names = append(names, m.bowName)
		}
		return 0, 0, fmt.Errorf("multiple chlorinators found; pass --bow to choose: %s", strings.Join(names, ", "))
	}
}

func filterSuffix(s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf(" for BoW %q", s)
}

// ChemistryVerdict assigns a traffic-light verdict from a set of chemistry
// readings. Thresholds follow standard Trouble Free Pool guidance.
func ChemistryVerdict(ph *float64, orp, salt *int) (verdict string, reasons []string) {
	verdict = "ok"
	if ph != nil {
		switch {
		case *ph < 7.2:
			verdict = bumpVerdict(verdict, "low")
			reasons = append(reasons, fmt.Sprintf("pH low (%.2f, want 7.2-7.8)", *ph))
		case *ph > 7.8:
			verdict = bumpVerdict(verdict, "high")
			reasons = append(reasons, fmt.Sprintf("pH high (%.2f, want 7.2-7.8)", *ph))
		}
	}
	if orp != nil {
		switch {
		case *orp < 650:
			verdict = bumpVerdict(verdict, "low")
			reasons = append(reasons, fmt.Sprintf("ORP low (%d mV, want 650-750)", *orp))
		case *orp > 800:
			verdict = bumpVerdict(verdict, "high")
			reasons = append(reasons, fmt.Sprintf("ORP high (%d mV)", *orp))
		}
	}
	if salt != nil {
		switch {
		case *salt < 2700:
			verdict = bumpVerdict(verdict, "low")
			reasons = append(reasons, fmt.Sprintf("salt low (%d ppm, want 2700-3400)", *salt))
		case *salt > 3500:
			verdict = bumpVerdict(verdict, "high")
			reasons = append(reasons, fmt.Sprintf("salt high (%d ppm)", *salt))
		}
	}
	if ph == nil && orp == nil && salt == nil {
		verdict = "unknown"
	}
	return verdict, reasons
}

func bumpVerdict(cur, next string) string {
	if cur == "ok" {
		return next
	}
	return cur
}

// FormatTemp formats an *int temp as "82°F" or "-" when nil.
func FormatTemp(t *int) string {
	if t == nil {
		return "-"
	}
	return strconv.Itoa(*t) + "°F"
}
