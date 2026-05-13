// PATCH: hand-authored novel-feature file. See .printing-press-patches.json patch id "novel-blsdata-package".
package blsdata

import "strings"

// pp:novel-static-reference
//
// Most BLS series IDs encode seasonal adjustment in the position right after
// the survey prefix. The encoding varies by survey:
//
//   CU  -- position 3 (CUUR vs CUSR): U = NSA, S = SA
//   CW  -- position 3 (CWUR vs CWSR): U = NSA, S = SA
//   CE  -- position 3 (CES vs CEU):   S = SA,  U = NSA (encoded BEFORE the digits)
//   LN  -- position 3 (LNS vs LNU):   S = SA,  U = NSA
//   CI  -- position 3 (CIU vs CIS):   U = NSA, S = SA (ECI inverts: CIU = NSA, CIS = SA)
//   WP  -- position 3 (WPU vs WPS):   U = NSA, S = SA
//   JT  -- position 3 (JTS vs JTU):   S = SA,  U = NSA
//   AP  -- position 3 (APU vs APS):   APU is universally NSA
//
// We expose two helpers:
//   DecodeAdjustment(id) -- returns "seasonal", "nsa", or "unknown"
//   CompareAdjustmentIDs(id) -- returns (saID, nsaID) when both variants
//                                exist, or empty strings if the encoding
//                                cannot be flipped reliably for this prefix.

// DecodeAdjustment classifies a BLS series ID as seasonally adjusted ("seasonal"),
// not seasonally adjusted ("nsa"), or "unknown" when the encoding is
// ambiguous or the prefix is not yet wired.
func DecodeAdjustment(id string) string {
	id = strings.ToUpper(strings.TrimSpace(id))
	if len(id) < 3 {
		return "unknown"
	}
	prefix := id[:2]
	pos3 := id[2]
	switch prefix {
	case "CU", "CW":
		// CUUR/CUSR (CPI-U); CWUR/CWSR (CPI-W)
		switch pos3 {
		case 'U':
			return "nsa"
		case 'S':
			return "seasonal"
		}
	case "LN", "JT", "CE":
		switch pos3 {
		case 'S':
			return "seasonal"
		case 'U':
			return "nsa"
		}
	case "CI":
		// ECI inverts the convention: CIU = NSA, CIS = SA index
		switch pos3 {
		case 'U':
			return "nsa"
		case 'S':
			return "seasonal"
		}
	case "WP":
		switch pos3 {
		case 'U':
			return "nsa"
		case 'S':
			return "seasonal"
		}
	case "AP":
		// APU is universally NSA
		return "nsa"
	}
	return "unknown"
}

// CompareAdjustmentIDs accepts a series ID (which may be SA or NSA) and
// returns the canonical (saID, nsaID) pair when the survey has a documented
// position-3 toggle. Returns ("", "") when the survey doesn't follow the
// pattern, so callers can surface a useful error instead of fabricating an
// ID the API would reject.
//
// For prefixes that share the same prefix letters and only differ in
// position 3 (CU, CW, LN, JT, CE, CI, WP), we toggle position 3.
func CompareAdjustmentIDs(id string) (saID, nsaID string) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if len(id) < 3 {
		return "", ""
	}
	prefix := id[:2]
	pos3 := id[2]
	rest := id[3:]
	switch prefix {
	case "CU", "CW", "CI", "WP":
		// U = NSA, S = SA
		switch pos3 {
		case 'U':
			return prefix + "S" + rest, prefix + "U" + rest
		case 'S':
			return prefix + "S" + rest, prefix + "U" + rest
		}
	case "LN", "JT", "CE":
		// S = SA, U = NSA
		switch pos3 {
		case 'S':
			return prefix + "S" + rest, prefix + "U" + rest
		case 'U':
			return prefix + "S" + rest, prefix + "U" + rest
		}
	}
	return "", ""
}
