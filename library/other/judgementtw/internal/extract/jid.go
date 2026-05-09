// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package extract

import (
	"fmt"
	"strconv"
	"strings"
)

// JID represents the components of a Taiwan Judicial Yuan judgment ID.
//
// Format: <Court4chars>,<Year>,<CaseChar>,<No>,<Date YYYYMMDD>,<Check>
// Example: TPSM,115,台抗,703,20260430,1
//
// The first 3 characters of Court are the court code (e.g. TPS = 最高法院);
// the 4th character is the case-type code: M (criminal), V (civil),
// A (administrative), P (disciplinary), C (constitutional).
type JID struct {
	Raw      string // original JID string
	Court    string // 3-letter court code, e.g. "TPS"
	CaseType string // 1-letter case type, e.g. "M"
	Year     int    // ROC year (民國), e.g. 115
	CaseChar string // 字別, e.g. "台抗", "毒抗", "訴"
	No       int    // case number, e.g. 703
	JDate    string // YYYYMMDD (Gregorian)
	Check    int    // check digit
}

// Parse splits a JID string into its components. Returns an error when the
// JID does not have exactly 6 comma-separated parts.
func Parse(raw string) (*JID, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty JID")
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 6 {
		return nil, fmt.Errorf("invalid JID %q: expected 6 comma-separated parts, got %d", raw, len(parts))
	}
	courtAndType := parts[0]
	if len(courtAndType) < 3 {
		return nil, fmt.Errorf("invalid JID %q: court+type prefix too short", raw)
	}
	year, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid JID %q: year %q is not an integer", raw, parts[1])
	}
	no, err := strconv.Atoi(strings.TrimSpace(parts[3]))
	if err != nil {
		return nil, fmt.Errorf("invalid JID %q: number %q is not an integer", raw, parts[3])
	}
	check, err := strconv.Atoi(strings.TrimSpace(parts[5]))
	if err != nil {
		return nil, fmt.Errorf("invalid JID %q: check %q is not an integer", raw, parts[5])
	}
	// Court code is 3 characters; case type is the 4th character. Some court codes
	// are exactly 3 chars (e.g. TPS, IPC); others are 4 (e.g. TPHM where H is the
	// branch). For robustness we treat the first 3 chars as Court and the rest of
	// `courtAndType` as CaseType.
	court := courtAndType[:3]
	caseType := courtAndType[3:]
	return &JID{
		Raw:      raw,
		Court:    court,
		CaseType: caseType,
		Year:     year,
		CaseChar: parts[2],
		No:       no,
		JDate:    parts[4],
		Check:    check,
	}, nil
}

// CaseTypeName returns the long-form Chinese name for a case-type code.
func CaseTypeName(code string) string {
	switch code {
	case "M":
		return "刑事"
	case "V":
		return "民事"
	case "A":
		return "行政"
	case "P":
		return "懲戒"
	case "C":
		return "憲法"
	default:
		return code
	}
}

// CaseTypeEnglish returns the English name for a case-type code.
func CaseTypeEnglish(code string) string {
	switch code {
	case "M":
		return "criminal"
	case "V":
		return "civil"
	case "A":
		return "administrative"
	case "P":
		return "disciplinary"
	case "C":
		return "constitutional"
	default:
		return code
	}
}

// CaseTypeFromEnglish converts a long-form name (any case) to its 1-letter code.
// Returns the input unchanged when no match is found, so callers can pass either
// "criminal" or "M" interchangeably.
func CaseTypeFromEnglish(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "criminal", "刑事", "m":
		return "M"
	case "civil", "民事", "v":
		return "V"
	case "administrative", "行政", "a":
		return "A"
	case "disciplinary", "懲戒", "p":
		return "P"
	case "constitutional", "憲法", "c":
		return "C"
	default:
		return s
	}
}

// CaseRoot is a stable identifier for a single matter across appellate stages.
// It is the (court tier, year, case-character) triple — appeals share these
// three properties even when the court and number change.
type CaseRoot struct {
	CourtTier string
	Year      int
	CaseChar  string
}

// Root extracts the appeal-chain key from a JID. CourtTier is the first
// character of the court code (T = 臺/Taiwan, K = 福建Kinmen, J = 司法院 special)
// concatenated with the case-type letter, which is enough to keep different
// case types apart while merging across court branches.
func (j *JID) Root() CaseRoot {
	tier := ""
	if len(j.Court) > 0 {
		tier = j.Court[:1]
	}
	return CaseRoot{
		CourtTier: tier + j.CaseType,
		Year:      j.Year,
		CaseChar:  j.CaseChar,
	}
}

// CourtHierarchyRank returns an integer indicating the court level for sorting
// appeal chains: 0 = district, 1 = high, 2 = supreme/constitutional, etc.
//
// Lower rank means lower court (where the case originated).
func CourtHierarchyRank(courtCode string) int {
	if len(courtCode) < 3 {
		return 0
	}
	prefix := courtCode[:3]
	switch prefix {
	case "JCC", "TPS", "TPA": // 憲法法庭, 最高法院, 最高行政法院
		return 3
	case "TPP", "TPJ", "TPC", "TPU": // 懲戒法院, 司法院特別庭
		return 3
	case "TPH", "TCH", "TNH", "KSH", "HLH", "KMH": // 高等法院 + 分院
		return 2
	case "TPB", "TCB", "KSB", "IPC": // 高等行政法院, 智慧財產
		return 2
	case "TPT", "TCT", "KST": // 高等行政法院 地方庭
		return 1
	}
	// District courts (...D) end in D; KSY = 高雄少年及家事
	if strings.HasSuffix(prefix, "D") || prefix == "KSY" {
		return 0
	}
	return 0
}
