// Package closedsignal consolidates every "is this place permanently closed"
// detection rule used by the dispatcher's kill-gate. Keeping detection
// rules in one package means adding a new locale/source rule is a single
// edit, not a hunt across per-source clients.
//
// Detection sources:
//   - Tabelog (JP):       "閉店", "営業終了" in HTML body
//   - Naver (KR):         "폐업" in HTML body
//   - Le Fooding (FR):    "fermé définitivement"
//   - OSM:                disused:amenity=*, opening_hours=closed
//   - Google Places:      business_status CLOSED_PERMANENTLY (drop) /
//     CLOSED_TEMPORARILY (warn)
//   - Recent reviews:     "RIP", "permanently closed", "closed in YYYY"
//
// Recent-review keywords are deliberately conservative — they catch the
// strongest English signals and the brief's named locale phrases. They
// are not a sentiment classifier.
package closedsignal

import (
	"strings"
	"time"
)

// Verdict is the outcome of a single closed-signal check.
type Verdict struct {
	// Closed is true when the source returned a high-confidence closed
	// signal. Closed entries are dropped by the dispatcher kill-gate.
	Closed bool

	// Temporary is true when the signal explicitly says "temporarily
	// closed" (Google CLOSED_TEMPORARILY, Tabelog "現在お休み"). These
	// are surfaced as warnings, not dropped.
	Temporary bool

	// Source is the source slug or descriptor that produced the signal
	// (e.g. "tabelog", "naver", "google.business_status", "osm.disused",
	// "review-keyword").
	Source string

	// Evidence is the verbatim string that matched. Kept short (<=200
	// chars) so it can be embedded in CLI output as a citation.
	Evidence string
}

// Open is the conventional zero-value verdict: not closed.
var Open = Verdict{}

// CheckTabelogHTML scans Tabelog page HTML for permanent-close strings.
// Tabelog uses 閉店 ("closed shop") for permanent closure and
// 営業終了 ("business ended") in the same context.
func CheckTabelogHTML(html string) Verdict {
	if html == "" {
		return Open
	}
	for _, kw := range []string{"閉店", "営業終了"} {
		if idx := strings.Index(html, kw); idx >= 0 {
			return Verdict{Closed: true, Source: "tabelog", Evidence: kw}
		}
	}
	// Tabelog's "現在お休み" means "currently on break" — temporary.
	if strings.Contains(html, "現在お休み") {
		return Verdict{Temporary: true, Source: "tabelog", Evidence: "現在お休み"}
	}
	return Open
}

// CheckNaverHTML scans a Naver Map / Naver Blog page for "폐업"
// ("closed business").
func CheckNaverHTML(html string) Verdict {
	if html == "" {
		return Open
	}
	if strings.Contains(html, "폐업") {
		return Verdict{Closed: true, Source: "naver", Evidence: "폐업"}
	}
	return Open
}

// CheckLeFoodingHTML scans Le Fooding for "fermé définitivement"
// (permanently closed). The Le Fooding editorial team marks these
// explicitly when delisting a place.
func CheckLeFoodingHTML(html string) Verdict {
	if html == "" {
		return Open
	}
	lc := strings.ToLower(html)
	if strings.Contains(lc, "fermé définitivement") || strings.Contains(lc, "fermé definitivement") {
		return Verdict{Closed: true, Source: "lefooding", Evidence: "fermé définitivement"}
	}
	return Open
}

// CheckOSMTags reports closed when an OSM element has disused:amenity=*
// or opening_hours=closed (or off). The map is the parsed OSM tag map
// for one element.
func CheckOSMTags(tags map[string]string) Verdict {
	if tags == nil {
		return Open
	}
	for k := range tags {
		if strings.HasPrefix(k, "disused:") {
			return Verdict{Closed: true, Source: "osm.disused", Evidence: k + "=" + tags[k]}
		}
	}
	if v := strings.ToLower(tags["opening_hours"]); v == "closed" || v == "off" {
		return Verdict{Closed: true, Source: "osm.opening_hours", Evidence: "opening_hours=" + v}
	}
	return Open
}

// CheckGoogleBusinessStatus maps the Google Places business_status enum to
// a Verdict. Empty string returns Open (the field was absent / pre-filtered).
func CheckGoogleBusinessStatus(status string) Verdict {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "CLOSED_PERMANENTLY":
		return Verdict{Closed: true, Source: "google.business_status", Evidence: "CLOSED_PERMANENTLY"}
	case "CLOSED_TEMPORARILY":
		return Verdict{Temporary: true, Source: "google.business_status", Evidence: "CLOSED_TEMPORARILY"}
	default:
		return Open
	}
}

// CheckReviewText scans a free-text review/blog/reddit body for
// permanent-close keywords. Conservative: only fires on strong signals.
// 'recencyMonths' is informational; the caller is expected to pass only
// reviews from the last 12 months per brief.
func CheckReviewText(body string, recencyMonths int) Verdict {
	if body == "" {
		return Open
	}
	// Wrap with spaces so token-bounded matchers like " rip " catch the
	// word at the start or end of the body too.
	lc := " " + strings.ToLower(body) + " "
	signals := []string{
		"permanently closed",
		"permanently shut",
		"closed for good",
		"closed down",
		"rest in peace",
		" rip ",
		" rip,",
		" rip.",
		"shuttered for good",
		"now closed",
	}
	for _, s := range signals {
		if strings.Contains(lc, s) {
			ev := s
			if len(ev) > 64 {
				ev = ev[:64]
			}
			return Verdict{Closed: true, Source: "review-keyword", Evidence: strings.TrimSpace(ev)}
		}
	}
	return Open
}

// Combine returns the strongest verdict from a slice. Closed beats
// Temporary; first-Closed wins ties (callers that pass verdicts in
// trust order get trust-ordered evidence). Empty input returns Open.
func Combine(verdicts []Verdict) Verdict {
	first := Open
	var firstClosed *Verdict
	var firstTemp *Verdict
	for i := range verdicts {
		v := verdicts[i]
		if v.Closed && firstClosed == nil {
			firstClosed = &v
		}
		if v.Temporary && firstTemp == nil {
			firstTemp = &v
		}
	}
	switch {
	case firstClosed != nil:
		return *firstClosed
	case firstTemp != nil:
		return *firstTemp
	default:
		return first
	}
}

// Recent returns true if t is within the last `months` months. Used by
// the dispatcher when filtering reviews before passing to CheckReviewText.
func Recent(t time.Time, months int) bool {
	if t.IsZero() || months <= 0 {
		return false
	}
	cutoff := time.Now().AddDate(0, -months, 0)
	return t.After(cutoff)
}
