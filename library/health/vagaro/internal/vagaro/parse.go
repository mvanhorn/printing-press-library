// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package vagaro

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
)

// appDateLayout is Go's reference layout for the "Ddd Mon-DD-YYYY" format the
// availability endpoint expects (e.g. "Fri Jul-24-2026").
const appDateLayout = "Mon Jan-02-2006"

// slotTimeRE matches a clock time like "10:00 AM" or "1:15 PM".
var slotTimeRE = regexp.MustCompile(`\b\d{1,2}:\d{2} [AP]M\b`)

// businessIDJSONRE matches businessID-ish JSON fields. Handles both raw
// (`BusinessID":93458`) and embedded/escaped (`BusinessId\":\"93458\"`) forms.
var businessIDJSONRE = regexp.MustCompile(`(?i)business[_ ]?id\\?"?\s*[:=]\s*\\?"?(\d+)`)

// imageIDRE matches the CDN image pattern `<providerId>_<businessID>_`,
// anchored on a `/` or `_` boundary so both `/43931725_93458_` (path segment)
// and `_43931725_93458_` (embedded) forms are captured.
var imageIDRE = regexp.MustCompile(`[/_](\d+)_(\d+)_`)

// FormatAppDate converts a YYYY-MM-DD date into the "Ddd Mon-DD-YYYY" AppDate
// format the availability endpoint expects.
func FormatAppDate(dateStr string) (string, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return "", fmt.Errorf("invalid date %q (want YYYY-MM-DD): %w", dateStr, err)
	}
	return t.Format(appDateLayout), nil
}

// ParseBusinessID extracts the numeric businessID from SSR HTML. It tallies
// every businessID-ish JSON field (ignoring 0), cross-checks against the
// `_<providerId>_<businessID>_` CDN image pattern, and returns the value with
// the strongest combined signal.
func ParseBusinessID(html string) (string, error) {
	counts := map[string]int{}
	for _, m := range businessIDJSONRE.FindAllStringSubmatch(html, -1) {
		v := m[1]
		if v == "" || v == "0" {
			continue
		}
		counts[v]++
	}
	imageCand := map[string]int{}
	for _, m := range imageIDRE.FindAllStringSubmatch(html, -1) {
		if m[2] != "" && m[2] != "0" {
			imageCand[m[2]]++
		}
	}
	if len(counts) == 0 {
		// No JSON field present: fall back to the image-URL businessID.
		if best := pickMode(imageCand); best != "" {
			return best, nil
		}
		return "", fmt.Errorf("no businessID found in page")
	}
	// Score = 2*JSON frequency, +1 when the value also appears in an image URL.
	best := ""
	bestScore := -1
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	// Deterministic iteration: longer (larger id) first, then lexicographic.
	sortStringsByLenDesc(keys)
	for _, k := range keys {
		score := counts[k] * 2
		if imageCand[k] > 0 {
			score++
		}
		if score > bestScore {
			bestScore = score
			best = k
		}
	}
	return best, nil
}

func sortStringsByLenDesc(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0; j-- {
			a, b := s[j-1], s[j]
			if len(a) < len(b) || (len(a) == len(b) && a < b) {
				s[j-1], s[j] = s[j], s[j-1]
			} else {
				break
			}
		}
	}
}

// metaContentRE extracts <meta property="og:X" content="Y"> values.
func metaContentRE(prop string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)<meta[^>]+property="` + regexp.QuoteMeta(prop) + `"[^>]+content="([^"]*)"`)
}

// breadcrumbRE pulls the "<category> in <City> , <ST>" breadcrumb label.
var breadcrumbRE = regexp.MustCompile(`(?i)"name"\s*:\s*"([^"]+?)\s+in\s+([^",]+?)\s*,\s*([A-Za-z]{2})"`)

// BusinessProfile is the structured business detail parsed from SSR HTML.
type BusinessProfile struct {
	Slug        string  `json:"slug,omitempty"`
	BusinessID  string  `json:"business_id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Category    string  `json:"category,omitempty"`
	City        string  `json:"city,omitempty"`
	State       string  `json:"state,omitempty"`
	Rating      float64 `json:"rating,omitempty"`
	ReviewCount int     `json:"review_count,omitempty"`
	PriceRange  string  `json:"price_range,omitempty"`
	Address     string  `json:"address,omitempty"`
	Phone       string  `json:"phone,omitempty"`
}

// ParseBusinessProfile extracts the business profile from SSR HTML. Name and
// category/city/state come from og:* meta tags and the JSON-LD breadcrumb;
// businessID from ParseBusinessID. Rating/address/phone are not exposed in
// parseable form on the SSR page and are left zero when absent.
func ParseBusinessProfile(html string) BusinessProfile {
	var p BusinessProfile
	if id, err := ParseBusinessID(html); err == nil {
		p.BusinessID = id
	}
	if m := metaContentRE("og:title").FindStringSubmatch(html); m != nil {
		p.Name = cleanBusinessName(cliutil.CleanText(m[1]))
	}
	if m := metaContentRE("og:description").FindStringSubmatch(html); m != nil {
		p.Description = cliutil.CleanText(m[1])
	}
	if m := breadcrumbRE.FindStringSubmatch(html); m != nil {
		p.Category = cliutil.CleanText(m[1])
		p.City = cliutil.CleanText(m[2])
		p.State = strings.ToUpper(cliutil.CleanText(m[3]))
	}
	return p
}

// cleanBusinessName trims Vagaro's "<Name> - <City ST> | Vagaro" title suffix
// down to the business name.
func cleanBusinessName(title string) string {
	if i := strings.Index(title, " | "); i >= 0 {
		title = title[:i]
	}
	if i := strings.LastIndex(title, " - "); i >= 0 {
		title = title[:i]
	}
	return strings.TrimSpace(title)
}

// ServiceRow is a flattened service entry.
type ServiceRow struct {
	ServiceID    int64  `json:"service_id"`
	ServiceTitle string `json:"title"`
	PriceText    string `json:"price_text"`
	PriceCents   int    `json:"price_cents,omitempty"`
	Category     string `json:"category,omitempty"`
}

type serviceCategory struct {
	ServiceCategoryTitle string `json:"ServiceCategoryTitle"`
	ServiceList          []struct {
		ServiceID    int64   `json:"ServiceID"`
		ServiceTitle string  `json:"ServiceTitle"`
		PriceText    string  `json:"PriceText"`
		Price        float64 `json:"Price"`
	} `json:"ServiceList"`
}

type servicesEnvelope struct {
	Services []serviceCategory `json:"Services"`
}

// htmlTagRE matches an HTML tag. Promo-priced services return PriceText as a
// markup fragment ("<span ...>$60.00</span><span ...>$54.00</span>") rather
// than a bare "$52.00", so tags are stripped for display.
var htmlTagRE = regexp.MustCompile(`<[^>]*>`)

// wsRE collapses runs of whitespace introduced by tag removal.
var wsRE = regexp.MustCompile(`\s+`)

// cleanPriceText strips HTML tags from a PriceText value and collapses the
// resulting whitespace so promo prices render readably.
func cleanPriceText(s string) string {
	if !strings.Contains(s, "<") {
		return cliutil.CleanText(s)
	}
	stripped := htmlTagRE.ReplaceAllString(s, " ")
	stripped = wsRE.ReplaceAllString(stripped, " ")
	return cliutil.CleanText(stripped)
}

// ParseServices flattens the getshopdetailcompositeservice payload into rows.
// PriceCents is taken from the numeric Price field (authoritative and stable
// across promo markup); PriceText is the cleaned display string.
func ParseServices(d json.RawMessage) []ServiceRow {
	var env servicesEnvelope
	if err := json.Unmarshal(d, &env); err != nil {
		return []ServiceRow{}
	}
	out := make([]ServiceRow, 0)
	for _, cat := range env.Services {
		category := cliutil.CleanText(cat.ServiceCategoryTitle)
		for _, s := range cat.ServiceList {
			out = append(out, ServiceRow{
				ServiceID:    s.ServiceID,
				ServiceTitle: cliutil.CleanText(s.ServiceTitle),
				PriceText:    cleanPriceText(s.PriceText),
				PriceCents:   int(s.Price*100 + 0.5),
				Category:     category,
			})
		}
	}
	return out
}

// Provider is a service provider (staff member).
type Provider struct {
	ServiceProviderID int64  `json:"provider_id"`
	Name              string `json:"name"`
}

type staffEnvelope struct {
	ServiceProviders []struct {
		ServiceProviderID int64  `json:"ServiceProviderID"`
		FirstName         string `json:"FirstName"`
		LastName          string `json:"LastName"`
	} `json:"ServiceProviders"`
}

// ParseStaff extracts providers from the getshopdetailcompositestaff payload.
func ParseStaff(d json.RawMessage) []Provider {
	var env staffEnvelope
	if err := json.Unmarshal(d, &env); err != nil {
		return []Provider{}
	}
	out := make([]Provider, 0, len(env.ServiceProviders))
	for _, sp := range env.ServiceProviders {
		name := strings.TrimSpace(cliutil.CleanText(sp.FirstName) + " " + cliutil.CleanText(sp.LastName))
		out = append(out, Provider{ServiceProviderID: sp.ServiceProviderID, Name: name})
	}
	return out
}

// Review is a single customer review.
type Review struct {
	ReviewID int64   `json:"review_id"`
	Author   string  `json:"author"`
	Rating   float64 `json:"rating"`
	Text     string  `json:"text,omitempty"`
	Date     string  `json:"date,omitempty"`
	Provider string  `json:"provider,omitempty"`
}

type reviewRaw struct {
	ReviewID              int64   `json:"ReviewID"`
	Reviewer              string  `json:"Reviewer"`
	AverageRank           float64 `json:"AverageRank"`
	ServiceProviderReview string  `json:"ServiceProviderReview"`
	VenueReview           string  `json:"VenueReview"`
	PublishedDate         string  `json:"PublishedDate"`
	CreatedDateFormat     string  `json:"CreatedDateFormat"`
	ServiceProviderName   string  `json:"ServiceProviderName"`
}

// ParseReviews extracts reviews from the getreviews payload, which is a bare
// JSON array.
func ParseReviews(d json.RawMessage) []Review {
	var raws []reviewRaw
	if err := json.Unmarshal(d, &raws); err != nil {
		return []Review{}
	}
	out := make([]Review, 0, len(raws))
	for _, r := range raws {
		text := cliutil.CleanText(r.ServiceProviderReview)
		if text == "" {
			text = cliutil.CleanText(r.VenueReview)
		}
		date := cliutil.CleanText(r.PublishedDate)
		if date == "" {
			date = cliutil.CleanText(r.CreatedDateFormat)
		}
		out = append(out, Review{
			ReviewID: r.ReviewID,
			Author:   cliutil.CleanText(r.Reviewer),
			Rating:   r.AverageRank,
			Text:     text,
			Date:     date,
			Provider: cliutil.CleanText(r.ServiceProviderName),
		})
	}
	return out
}

// SlotGroup is a set of available times for one provider on one date.
type SlotGroup struct {
	Date       string   `json:"date,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	ProviderID string   `json:"provider_id,omitempty"`
	Times      []string `json:"times"`
}

type availSPData struct {
	AvailableTime       string `json:"AvailableTime"`
	ServiceProviderID   int64  `json:"ServiceProviderID"`
	ServiceProviderName string `json:"ServiceProviderName"`
}

type availGroup struct {
	AppDate             string        `json:"AppDate"`
	ServicepPoviderData []availSPData `json:"ServicepPoviderData"`
}

// ExtractSlots parses the (already .d-unwrapped) availability payload into
// per-provider slot groups. The normal payload is a JSON array of booking
// groups; when the endpoint instead returns an HTML/text fragment inside .d,
// slot times are regex-extracted and stamped with the query's date (and
// provider, when a single one was requested) so callers still compare against
// the correct day rather than matching a clock label on any date. An empty or
// no-availability response yields an empty (non-nil) slice.
//
// queryDate is the AppDate sent with the request (appDateLayout, e.g.
// "Mon Jan-02-2006"); queryProvider is the requested csvSPID.
func ExtractSlots(d json.RawMessage, queryDate, queryProvider string) []SlotGroup {
	var groups []availGroup
	if err := json.Unmarshal(d, &groups); err == nil && len(groups) > 0 {
		out := make([]SlotGroup, 0, len(groups))
		for _, g := range groups {
			for _, sp := range g.ServicepPoviderData {
				times := splitTimes(sp.AvailableTime)
				if len(times) == 0 {
					continue
				}
				sg := SlotGroup{
					Date:     cliutil.CleanText(g.AppDate),
					Provider: cliutil.CleanText(sp.ServiceProviderName),
					Times:    times,
				}
				if sp.ServiceProviderID != 0 {
					sg.ProviderID = strconv.FormatInt(sp.ServiceProviderID, 10)
				}
				out = append(out, sg)
			}
		}
		return out
	}
	// Fallback: HTML/text fragment carries no structured date/provider. Stamp
	// the known query date so date-aware callers (slotOpen, earliestSlot) don't
	// match a clock label against the wrong day; attach the provider only when a
	// single one was requested.
	times := extractSlotTimes(string(d))
	if len(times) == 0 {
		return []SlotGroup{}
	}
	sg := SlotGroup{Date: cliutil.CleanText(queryDate), Times: times}
	if p := strings.TrimSpace(queryProvider); p != "" && !strings.Contains(p, ",") {
		sg.ProviderID = p
	}
	return []SlotGroup{sg}
}

// slotDateLayouts are the date formats the availability payload's AppDate
// field has been observed to use across the structured and week-start forms.
var slotDateLayouts = []string{
	"2 Jan 2006",
	"02 Jan 2006",
	"Mon Jan-02-2006",
	"Jan-02-2006",
	"2006-01-02",
	"Jan 2, 2006",
}

// ParseSlotDateTime combines a slot group's date string and a "10:00 AM" clock
// time into a single time.Time so callers can rank availability by soonest.
// Returns ok=false when the date cannot be parsed (HTML-fragment fallback
// groups carry no date); a parseable date with an unparseable time yields the
// date at midnight so date-level ranking still works.
func ParseSlotDateTime(dateStr, timeStr string) (time.Time, bool) {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}, false
	}
	var day time.Time
	parsed := false
	for _, layout := range slotDateLayouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			day = t
			parsed = true
			break
		}
	}
	if !parsed {
		return time.Time{}, false
	}
	if clock, err := time.Parse("3:04 PM", strings.TrimSpace(timeStr)); err == nil {
		return time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, time.UTC), true
	}
	return day, true
}

// splitTimes splits an "AvailableTime" CSV into validated, de-duplicated
// clock times in original order.
func splitTimes(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, part := range strings.Split(csv, ",") {
		t := strings.TrimSpace(part)
		if !slotTimeRE.MatchString(t) {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// extractSlotTimes regex-extracts distinct clock times from an HTML/text blob.
func extractSlotTimes(text string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, t := range slotTimeRE.FindAllString(text, -1) {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}
