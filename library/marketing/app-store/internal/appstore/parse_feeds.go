// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// --- Envelope helpers -----------------------------------------------------
//
// A 200 is "verified empty" only when its body is the documented envelope of
// the surface that was asked for and that envelope holds no rows. Anything
// else (an empty object, null, an error object, another feed's body) is an
// *EnvelopeError, which the endpoints log as a failed capture.

// decodeJSONObject decodes a body that must be a JSON object. null, arrays
// and scalars are envelope errors; malformed JSON is a decode error.
func decodeJSONObject(body []byte, surface string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, &EnvelopeError{Surface: surface, Reason: "the body is empty"}
	}
	if trimmed[0] != '{' {
		var probe any
		if err := json.Unmarshal(trimmed, &probe); err != nil {
			return nil, fmt.Errorf("decoding %s response: %w", surface, err)
		}
		if probe == nil {
			return nil, &EnvelopeError{Surface: surface, Reason: "the body is null"}
		}
		return nil, &EnvelopeError{Surface: surface, Reason: "the body is not a JSON object"}
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &top); err != nil {
		return nil, fmt.Errorf("decoding %s response: %w", surface, err)
	}
	return top, nil
}

// decodeJSONAny is decodeJSONObject for the parsers that walk map[string]any.
func decodeJSONAny(body []byte, surface string) (map[string]any, error) {
	if _, err := decodeJSONObject(body, surface); err != nil {
		return nil, err
	}
	var top map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(body), &top); err != nil {
		return nil, fmt.Errorf("decoding %s response: %w", surface, err)
	}
	return top, nil
}

func jsonKind(raw json.RawMessage) byte {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 {
		return 0
	}
	return t[0]
}

func isJSONArray(raw json.RawMessage) bool  { return jsonKind(raw) == '[' }
func isJSONObject(raw json.RawMessage) bool { return jsonKind(raw) == '{' }
func isJSONNull(raw json.RawMessage) bool   { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

// jsonText renders a JSON value as text: a string as-is, anything else as
// its compact JSON ("" for null or absent).
func jsonText(raw json.RawMessage) string {
	if len(raw) == 0 || isJSONNull(raw) {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(bytes.TrimSpace(raw))
}

// ParseLookupBody splits a Lookup or Search body into software results and
// the artist wrapper row (developer lookups return one first). The body must
// be the documented envelope {"resultCount": n, "results": [...]}: an empty
// object, null, an {"errorMessage": ...} object, or a resultCount above zero
// with no results is an *EnvelopeError, so a wrong-shaped 200 can never pass
// for "Apple has no such app".
func ParseLookupBody(body []byte) ([]LookupResult, *LookupArtist, int, error) {
	const surface = "iTunes Lookup/Search"
	top, err := decodeJSONObject(body, surface)
	if err != nil {
		return nil, nil, 0, err
	}
	if msg := strings.TrimSpace(jsonText(top["errorMessage"])); msg != "" {
		return nil, nil, 0, &EnvelopeError{Surface: surface, Reason: "Apple answered with an error: " + msg}
	}
	rcRaw, hasCount := top["resultCount"]
	resRaw, hasResults := top["results"]
	if !hasCount || !hasResults {
		return nil, nil, 0, &EnvelopeError{Surface: surface, Reason: "the body has no {resultCount, results} envelope"}
	}
	var resultCount int
	if err := json.Unmarshal(rcRaw, &resultCount); err != nil {
		return nil, nil, 0, &EnvelopeError{Surface: surface, Reason: "resultCount is not a number"}
	}
	if !isJSONArray(resRaw) {
		return nil, nil, 0, &EnvelopeError{Surface: surface, Reason: "results is not an array"}
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(resRaw, &rows); err != nil {
		return nil, nil, 0, fmt.Errorf("decoding iTunes lookup results: %w", err)
	}
	if resultCount > 0 && len(rows) == 0 {
		return nil, nil, 0, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("resultCount is %d but results is empty", resultCount)}
	}
	results := make([]LookupResult, 0, len(rows))
	var artist *LookupArtist
	for _, raw := range rows {
		var probe struct {
			WrapperType   string `json:"wrapperType"`
			ArtistType    string `json:"artistType"`
			ArtistName    string `json:"artistName"`
			ArtistLinkURL string `json:"artistLinkUrl"`
			ArtistID      int64  `json:"artistId"`
		}
		_ = json.Unmarshal(raw, &probe)
		if probe.WrapperType == "artist" {
			if artist == nil {
				artist = &LookupArtist{ArtistID: probe.ArtistID, ArtistName: probe.ArtistName, ArtistType: probe.ArtistType, ArtistLinkURL: CanonicalAppURL(probe.ArtistLinkURL)}
			}
			continue
		}
		var r LookupResult
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, nil, 0, fmt.Errorf("decoding iTunes lookup result: %w", err)
		}
		if r.WrapperType != "" && r.WrapperType != "software" {
			continue
		}
		results = append(results, r)
	}
	return results, artist, resultCount, nil
}

// --- Legacy RSS feeds -------------------------------------------------------

type rssLabel struct {
	Label      string            `json:"label"`
	Attributes map[string]string `json:"attributes"`
}

func (l *rssLabel) UnmarshalJSON(b []byte) error {
	var obj struct {
		Label      json.RawMessage   `json:"label"`
		Attributes map[string]string `json:"attributes"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		// Some feeds send a bare string.
		var s string
		if err2 := json.Unmarshal(b, &s); err2 == nil {
			l.Label = s
			return nil
		}
		return err
	}
	l.Attributes = obj.Attributes
	if len(obj.Label) > 0 {
		var s string
		if err := json.Unmarshal(obj.Label, &s); err == nil {
			l.Label = s
		} else {
			l.Label = strings.Trim(string(obj.Label), `"`)
		}
	}
	return nil
}

// rssEntries handles the legacy feed quirk where one entry arrives as an
// object instead of a one-element array, and no entries as a missing key.
type rssEntries []map[string]json.RawMessage

func (e *rssEntries) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*e = nil
		return nil
	}
	if b[0] == '{' {
		var one map[string]json.RawMessage
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*e = rssEntries{one}
		return nil
	}
	var many []map[string]json.RawMessage
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*e = many
	return nil
}

type rssFeed struct {
	Feed struct {
		ID      rssLabel   `json:"id"`
		Updated rssLabel   `json:"updated"`
		Entry   rssEntries `json:"entry"`
	} `json:"feed"`
	// hasMeta records whether the feed object carries the Atom metadata
	// (id, updated, author or title) Apple sends on every feed, empty ones
	// included. Only such a feed can prove "no entries".
	hasMeta bool
}

func rssField(entry map[string]json.RawMessage, key string) rssLabel {
	var l rssLabel
	if raw, ok := entry[key]; ok {
		_ = json.Unmarshal(raw, &l)
	}
	return l
}

// decodeRSS decodes a legacy iTunes RSS JSON feed. The body must be an
// object with a "feed" object; {} , null, {"feed": null} and other shapes
// are envelope errors.
func decodeRSS(body []byte, surface string) (*rssFeed, error) {
	top, err := decodeJSONObject(body, surface)
	if err != nil {
		return nil, err
	}
	rawFeed, ok := top["feed"]
	if !ok || !isJSONObject(rawFeed) {
		return nil, &EnvelopeError{Surface: surface, Reason: "the body has no feed object"}
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(rawFeed, &meta); err != nil {
		return nil, fmt.Errorf("decoding %s feed: %w", surface, err)
	}
	var f rssFeed
	if err := json.Unmarshal(bytes.TrimSpace(body), &f); err != nil {
		return nil, fmt.Errorf("decoding %s feed: %w", surface, err)
	}
	for _, k := range []string{"id", "updated", "author", "title"} {
		if v, ok := meta[k]; ok && !isJSONNull(v) {
			f.hasMeta = true
			break
		}
	}
	return &f, nil
}

// rssFeedID splits a legacy feed id such as
// "https://itunes.apple.com/us/rss/customerreviews/page=1/id=1/sortby=mostrecent/json"
// into its feed name ("customerreviews") and key=value segments. Both are
// lower-cased; the name is "" when the id is not an /rss/ URL.
func rssFeedID(label string) (string, map[string]string) {
	l := strings.ToLower(strings.TrimSpace(label))
	idx := strings.Index(l, "/rss/")
	if idx < 0 {
		return "", nil
	}
	rest := l[idx+len("/rss/"):]
	if cut := strings.IndexAny(rest, "?#"); cut >= 0 {
		rest = rest[:cut]
	}
	parts := strings.Split(rest, "/")
	params := map[string]string{}
	for _, p := range parts[1:] {
		if k, v, ok := strings.Cut(p, "="); ok {
			params[k] = v
		}
	}
	return parts[0], params
}

const customerReviewsFeed = "customerreviews"

var developerIDRe = regexp.MustCompile(`/id(\d+)`)

// ParseChartFeed parses a legacy RSS chart (topfreeapplications etc.).
// Entries are ranked by their position in the feed starting at 1. A body
// that is not a chart feed (no feed object, a customer-reviews feed, entries
// without app ids) is an *EnvelopeError; an empty chart needs the feed's
// metadata to count as verified-empty.
func ParseChartFeed(body []byte) (updated string, entries []ChartEntry, err error) {
	return parseChartFeed(body, "")
}

// parseChartFeed is ParseChartFeed that also rejects the body of another
// known chart feed than wantFeed (when wantFeed is set).
func parseChartFeed(body []byte, wantFeed string) (string, []ChartEntry, error) {
	const surface = "iTunes RSS chart"
	f, err := decodeRSS(body, surface)
	if err != nil {
		return "", nil, err
	}
	if kind, _ := rssFeedID(f.Feed.ID.Label); kind != "" {
		if kind == customerReviewsFeed {
			return "", nil, &EnvelopeError{Surface: surface, Reason: "the body is a customer-reviews feed (" + f.Feed.ID.Label + ")"}
		}
		if want := strings.ToLower(strings.TrimSpace(wantFeed)); want != "" && LegacyChartFeeds[kind] && kind != want {
			return "", nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("the body is the %s feed, not %s", kind, want)}
		}
	}
	entries := make([]ChartEntry, 0, len(f.Feed.Entry))
	for i, e := range f.Feed.Entry {
		if _, ok := e["im:rating"]; ok {
			return "", nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("entry %d is a customer review, not a chart row", i+1)}
		}
		id := rssField(e, "id")
		price := rssField(e, "im:price")
		artist := rssField(e, "im:artist")
		cat := rssField(e, "category")
		rel := rssField(e, "im:releaseDate")
		amount, _ := strconv.ParseFloat(price.Attributes["amount"], 64)
		ce := ChartEntry{
			// The feed position, so a skipped row leaves a gap instead of
			// shifting every later rank up.
			Rank:        i + 1,
			AppID:       id.Attributes["im:id"],
			BundleID:    id.Attributes["im:bundleId"],
			Name:        cleanText(rssField(e, "im:name").Label),
			Artist:      cleanText(artist.Label),
			Price:       cleanText(price.Label),
			Amount:      amount,
			Currency:    price.Attributes["currency"],
			GenreID:     cat.Attributes["im:id"],
			Genre:       cat.Attributes["label"],
			ReleaseDate: normalizeDate(rel.Label),
			URL:         CanonicalAppURL(id.Label),
		}
		if m := developerIDRe.FindStringSubmatch(artist.Attributes["href"]); m != nil {
			ce.ArtistID = m[1]
		}
		if ce.AppID == "" {
			if m := developerIDRe.FindStringSubmatch(id.Label); m != nil {
				ce.AppID = m[1]
			}
		}
		if ce.AppID == "" {
			continue
		}
		entries = append(entries, ce)
	}
	if len(f.Feed.Entry) > 0 && len(entries) == 0 {
		return "", nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("none of the %d entries carries an app id", len(f.Feed.Entry))}
	}
	if len(f.Feed.Entry) == 0 && !f.hasMeta {
		return "", nil, &EnvelopeError{Surface: surface, Reason: "the feed has no entries and no feed metadata (id, updated, author), so it cannot prove an empty chart"}
	}
	return normalizeDate(f.Feed.Updated.Label), entries, nil
}

// reviewsFeedRequest is what a customer-reviews page was requested for; the
// parser rejects a feed whose id names another app, page or sort.
type reviewsFeedRequest struct {
	AppID string
	Page  int
	Sort  string
}

// ParseReviewsFeed parses one customer-reviews RSS page. Only entries that
// carry im:rating are reviews (some captures lead with an app-metadata entry).
// A body that is not a customer-reviews feed (no feed object, another feed's
// id, more than one entry without im:rating) is an *EnvelopeError; an empty
// page needs the feed's metadata to count as verified-empty.
func ParseReviewsFeed(body []byte) ([]Review, error) {
	return parseReviewsFeed(body, reviewsFeedRequest{})
}

func parseReviewsFeed(body []byte, req reviewsFeedRequest) ([]Review, error) {
	const surface = "customer-reviews RSS"
	f, err := decodeRSS(body, surface)
	if err != nil {
		return nil, err
	}
	if kind, params := rssFeedID(f.Feed.ID.Label); kind != "" {
		if kind != customerReviewsFeed {
			return nil, &EnvelopeError{Surface: surface, Reason: "the body is the " + kind + " feed, not customer reviews"}
		}
		if got := params["id"]; req.AppID != "" && got != "" && got != req.AppID {
			return nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("the feed is for app %s, not %s", got, req.AppID)}
		}
		if got := params["page"]; req.Page > 0 && got != "" && got != strconv.Itoa(req.Page) {
			return nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("the feed is page %s, not page %d", got, req.Page)}
		}
		if got := params["sortby"]; req.Sort != "" && got != "" && got != strings.ToLower(req.Sort) {
			return nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("the feed is sorted by %s, not %s", got, req.Sort)}
		}
	}
	out := make([]Review, 0, len(f.Feed.Entry))
	nonReviews, idless := 0, 0
	for _, e := range f.Feed.Entry {
		if _, ok := e["im:rating"]; !ok {
			nonReviews++
			continue
		}
		rating, _ := strconv.Atoi(strings.TrimSpace(rssField(e, "im:rating").Label))
		voteSum, _ := strconv.Atoi(strings.TrimSpace(rssField(e, "im:voteSum").Label))
		voteCount, _ := strconv.Atoi(strings.TrimSpace(rssField(e, "im:voteCount").Label))
		author := ""
		if raw, ok := e["author"]; ok {
			var a struct {
				Name rssLabel `json:"name"`
			}
			_ = json.Unmarshal(raw, &a)
			author = cleanText(a.Name.Label)
		}
		r := Review{
			ID:         strings.TrimSpace(rssField(e, "id").Label),
			Rating:     rating,
			Version:    strings.TrimSpace(rssField(e, "im:version").Label),
			Title:      cleanText(rssField(e, "title").Label),
			Body:       cleanMultiline(rssField(e, "content").Label),
			Author:     author,
			ReviewedAt: normalizeDate(rssField(e, "updated").Label),
			VoteSum:    voteSum,
			VoteCount:  voteCount,
		}
		if r.ID == "" {
			idless++
			continue
		}
		out = append(out, r)
	}
	if nonReviews > 1 {
		// A reviews page leads with at most one app-metadata entry; a run of
		// entries without ratings is another feed (a chart, say).
		return nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("%d of %d entries carry no im:rating; this is not a customer-reviews feed", nonReviews, len(f.Feed.Entry))}
	}
	if len(out) == 0 && idless > 0 {
		return nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("none of the %d review entries carries a review id", idless)}
	}
	if len(out) == 0 && !f.hasMeta {
		return nil, &EnvelopeError{Surface: surface, Reason: "the feed has no reviews and no feed metadata (id, updated, author), so it cannot prove an empty page"}
	}
	return out, nil
}

// reviewSpan returns the oldest and newest reviewedAt values.
func reviewSpan(reviews []Review) (oldest, newest string) {
	for _, r := range reviews {
		if r.ReviewedAt == "" {
			continue
		}
		if oldest == "" || r.ReviewedAt < oldest {
			oldest = r.ReviewedAt
		}
		if newest == "" || r.ReviewedAt > newest {
			newest = r.ReviewedAt
		}
	}
	return oldest, newest
}

// --- Search hints plist -----------------------------------------------------

// ParseHintsPlist reads the search-hints plist and returns the hint terms in
// order. The root dict must carry a "hints" array (empty when Apple has no
// suggestions); a dict without it is an *EnvelopeError, not "no hints".
func ParseHintsPlist(body []byte) ([]string, error) {
	const surface = "search hints"
	root, err := decodePlist(body)
	if err != nil {
		return nil, err
	}
	dict, ok := root.(map[string]any)
	if !ok {
		return nil, &EnvelopeError{Surface: surface, Reason: "the plist root is not a dict"}
	}
	raw, ok := dict["hints"]
	if !ok {
		return nil, &EnvelopeError{Surface: surface, Reason: "the plist has no hints array"}
	}
	hints, ok := raw.([]any)
	if !ok {
		return nil, &EnvelopeError{Surface: surface, Reason: "hints is not an array"}
	}
	out := make([]string, 0, len(hints))
	for _, h := range hints {
		term := strings.TrimSpace(strOf(asObject(h)["term"]))
		if term != "" {
			out = append(out, term)
		}
	}
	if len(hints) > 0 && len(out) == 0 {
		return nil, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("none of the %d hints carries a term", len(hints))}
	}
	return out, nil
}

// decodePlist decodes an XML property list into map[string]any, []any,
// string, int64, float64, bool and nil (date and data become strings).
func decodePlist(body []byte) (any, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = false
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("empty plist")
			}
			return nil, fmt.Errorf("decoding plist: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "plist" {
				continue
			}
			return decodePlistValue(dec, se)
		}
	}
}

func decodePlistValue(dec *xml.Decoder, se xml.StartElement) (any, error) {
	switch se.Name.Local {
	case "dict":
		out := map[string]any{}
		var key string
		for {
			tok, err := dec.Token()
			if err != nil {
				return nil, fmt.Errorf("decoding plist dict: %w", err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "key" {
					var k string
					if err := dec.DecodeElement(&k, &t); err != nil {
						return nil, err
					}
					key = k
					continue
				}
				v, err := decodePlistValue(dec, t)
				if err != nil {
					return nil, err
				}
				out[key] = v
			case xml.EndElement:
				if t.Name.Local == "dict" {
					return out, nil
				}
			}
		}
	case "array":
		out := make([]any, 0)
		for {
			tok, err := dec.Token()
			if err != nil {
				return nil, fmt.Errorf("decoding plist array: %w", err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				v, err := decodePlistValue(dec, t)
				if err != nil {
					return nil, err
				}
				out = append(out, v)
			case xml.EndElement:
				if t.Name.Local == "array" {
					return out, nil
				}
			}
		}
	case "true", "false":
		if err := dec.Skip(); err != nil {
			return nil, err
		}
		return se.Name.Local == "true", nil
	default:
		var s string
		if err := dec.DecodeElement(&s, &se); err != nil {
			return nil, err
		}
		switch se.Name.Local {
		case "integer":
			n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err != nil {
				return s, nil
			}
			return n, nil
		case "real":
			f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				return s, nil
			}
			return f, nil
		}
		return s, nil
	}
}
