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
	return ResolveHeaterInBoW(cfg, name, "")
}

// ResolveHeaterInBoW is the BoW-aware variant. Hayward's shared-equipment
// pattern means a single heater can appear under both Pool and Spa BoWs
// with identical name, so callers need a way to constrain by BoW name when
// disambiguating. Empty bowFilter matches any BoW.
// PATCH (fix-heater-bow-and-resolver): BoW-aware resolver added so BOW_SHARED_EQUIPMENT ('Gas' under both Pool and Spa) is disambiguable via --bow on heater enable/disable/set-temp.
func ResolveHeaterInBoW(cfg *MspConfig, name, bowFilter string) (poolID, heaterID int, heater Heater, err error) {
	type match struct {
		poolID   int
		heaterID int
		heater   Heater
		bowName  string
	}
	var matches []match
	seen := map[string]bool{} // dedupe shared heaters that appear under multiple BoWs
	nl := strings.ToLower(name)
	bf := strings.ToLower(strings.TrimSpace(bowFilter))
	for _, bow := range cfg.BodiesOfWater {
		if bf != "" && !strings.Contains(strings.ToLower(bow.Name), bf) {
			continue
		}
		bowID := atoiSafe(bow.SystemID)
		for _, h := range bow.Heaters {
			hid := atoiSafe(h.SystemID)
			key := fmt.Sprintf("%d:%d", bowID, hid) // dedupe within (BoW, heater) only
			if seen[key] {
				continue
			}
			if nl == "" {
				seen[key] = true
				matches = append(matches, match{bowID, hid, h, bow.Name})
				continue
			}
			hname := strings.ToLower(h.Name)
			if hname == nl || strings.Contains(hname, nl) {
				seen[key] = true
				matches = append(matches, match{bowID, hid, h, bow.Name})
			}
		}
	}
	switch len(matches) {
	case 0:
		if name == "" {
			suffix := ""
			if bowFilter != "" {
				suffix = fmt.Sprintf(" for BoW %q", bowFilter)
			}
			return 0, 0, Heater{}, fmt.Errorf("no heaters configured for this site%s", suffix)
		}
		return 0, 0, Heater{}, fmt.Errorf("no heater matched %q (use 'config get' to list heaters)", name)
	case 1:
		m := matches[0]
		return m.poolID, m.heaterID, m.heater, nil
	default:
		var names []string
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s in %s", m.heater.Name, m.bowName))
		}
		return 0, 0, Heater{}, fmt.Errorf("heater name %q matched multiple: %s — disambiguate with --bow Pool or --bow Spa", name, strings.Join(names, ", "))
	}
}

// ResolveEquipment finds any equipment item (pump, light, valve, relay) by
// name across all BoWs. Returns the BoW's SystemID + the equipment's
// SystemID. Pass kindFilter to restrict to "pump", "light", "valve",
// "relay", or "" for any.
func ResolveEquipment(cfg *MspConfig, name, kindFilter string) (poolID, equipID int, kind, displayName string, err error) {
	return ResolveEquipmentInBoW(cfg, name, kindFilter, "")
}

// ResolveEquipmentInBoW is like ResolveEquipment but constrained to a
// specific Body-of-Water by name (case-insensitive substring; empty matches
// any). Used when the same equipment name appears in both Pool and Spa
// (e.g., both BoWs have a pump literally named "Filter Pump").
func ResolveEquipmentInBoW(cfg *MspConfig, name, kindFilter, bowFilter string) (poolID, equipID int, kind, displayName string, err error) {
	type match struct {
		poolID  int
		eqID    int
		kind    string
		name    string
		bowName string
	}
	var matches []match
	seen := map[string]bool{} // dedupe shared equipment that appears in multiple BoWs
	nl := strings.ToLower(name)
	bf := strings.ToLower(strings.TrimSpace(bowFilter))
	for _, bow := range cfg.BodiesOfWater {
		// Skip BoWs that don't match the filter, if a filter was given.
		if bf != "" && !strings.Contains(strings.ToLower(bow.Name), bf) {
			continue
		}
		bowID := atoiSafe(bow.SystemID)
		bowName := bow.Name
		add := func(k, n, sid string) {
			if (kindFilter == "" || kindFilter == k) && (nl == "" || strings.Contains(strings.ToLower(n), nl)) {
				// Hayward marks shared pumps/heaters/lights as
				// BOW_SHARED_EQUIPMENT and lists them under every BoW
				// that references them. Dedupe by (kind, systemId) so
				// "Filter Pump" resolves to one match, not two.
				key := k + ":" + sid
				if seen[key] {
					return
				}
				seen[key] = true
				matches = append(matches, match{bowID, atoiSafe(sid), k, n, bowName})
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
	// Also try backyard-level relays for "relay" or empty kindFilter
	// (only when no BoW filter was given — backyard relays aren't scoped to a BoW).
	if (kindFilter == "" || kindFilter == "relay") && bf == "" {
		for _, r := range cfg.Relays {
			if nl == "" || strings.Contains(strings.ToLower(r.Name), nl) {
				key := "relay:" + r.SystemID
				if seen[key] {
					continue
				}
				seen[key] = true
				matches = append(matches, match{0, atoiSafe(r.SystemID), "relay", r.Name, ""})
			}
		}
	}
	switch len(matches) {
	case 0:
		hint := ""
		if bf != "" {
			hint = fmt.Sprintf(" in BoW %q", bowFilter)
		}
		return 0, 0, "", "", fmt.Errorf("no equipment matched %q%s", name, hint)
	case 1:
		m := matches[0]
		return m.poolID, m.eqID, m.kind, m.name, nil
	default:
		var names []string
		for _, m := range matches {
			label := fmt.Sprintf("%s (%s", m.name, m.kind)
			if m.bowName != "" {
				label += fmt.Sprintf(" in %s", m.bowName)
			}
			label += ")"
			names = append(names, label)
		}
		return 0, 0, "", "", fmt.Errorf("name %q matched multiple: %s — disambiguate with --bow Pool or --bow Spa", name, strings.Join(names, ", "))
	}
}

// IsVSPPump reports whether the equipment at equipmentID is a variable-speed
// pump. Hayward overloads SetUIEquipmentCmd's IsOn parameter: it expects
// dataType="int" 0-100 for VSPs and dataType="bool" True/False for everything
// else. Callers wiring `equipment on/off` need to know which dialect to send.
//
// Detection signal: the equipment's MSP-config type contains
// FMT_VARIABLE_SPEED or PMP_VARIABLE_SPEED (Hayward's enum prefix for VSPs).
// Falls back to false (treat as standard equipment) when the type can't be
// resolved.
func IsVSPPump(cfg *MspConfig, equipmentID int) bool {
	if cfg == nil {
		return false
	}
	target := equipmentID
	for _, bow := range cfg.BodiesOfWater {
		for _, p := range bow.Pumps {
			if atoiSafe(p.SystemID) == target {
				upper := strings.ToUpper(p.Type)
				return strings.Contains(upper, "VARIABLE_SPEED")
			}
		}
	}
	return false
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

// bumpVerdict folds a new finding into the running verdict. The legal
// terminal states are:
//   - ok       → all readings in range
//   - low      → every out-of-range reading is below safe range
//   - high     → every out-of-range reading is above safe range
//   - mixed    → at least one low AND at least one high (e.g. pH low + ORP high)
//
// Without this, the original implementation kept the first non-"ok" verdict
// and silently dropped every subsequent finding from the verdict string —
// users reading "low" wouldn't see that ORP was also "high", which could
// lead to under-treating the pool. The full reasons list is always in the
// returned reasons slice; the verdict is the one-word summary.
func bumpVerdict(cur, next string) string {
	if cur == next {
		return cur
	}
	if cur == "ok" {
		return next
	}
	// cur and next are both non-ok and differ → mixed direction (low+high).
	return "mixed"
}

// FormatTemp formats an *int temp as "82°F" or "-" when nil.
func FormatTemp(t *int) string {
	if t == nil {
		return "-"
	}
	return strconv.Itoa(*t) + "°F"
}
