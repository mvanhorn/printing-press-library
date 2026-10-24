// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParserVersion is stamped on every capture-log row so stored results can be
// traced to the parser generation that produced them. The response cache
// keys its entries to it too: a body cached by an older parser generation is
// fetched again rather than re-served. Generation 2 requires the documented
// envelope before a result may be verified-empty, and logs pages after their
// typed parse.
const ParserVersion = "appstore-parsers/2"

// Capture outcomes recorded in the capture log (tri-state results).
const (
	OutcomeOK            = "ok"
	OutcomeVerifiedEmpty = "verified-empty"
	OutcomeFailed        = "failed"
)

// Capture is one capture-log row: what was fetched, how Apple answered and
// what the parser made of it.
type Capture struct {
	CapturedAt    time.Time `json:"capturedAt"`
	URL           string    `json:"url"`
	Host          string    `json:"host"`
	Status        int       `json:"status"`
	Outcome       string    `json:"outcome"`
	EntryCount    int       `json:"entryCount"`
	CacheControl  string    `json:"cacheControl,omitempty"`
	XCache        string    `json:"xCache,omitempty"`
	Oldest        string    `json:"oldest,omitempty"`
	Newest        string    `json:"newest,omitempty"`
	ParserVersion string    `json:"parserVersion"`
	Error         string    `json:"error,omitempty"`
}

// CaptureSink receives capture-log rows. The store package implements it
// (store.NewCaptureSink); a nil sink disables logging.
type CaptureSink interface {
	RecordCapture(ctx context.Context, c Capture) error
}

// FetchMeta describes how one typed result was obtained.
type FetchMeta struct {
	URL        string    `json:"url"`
	FinalURL   string    `json:"finalUrl,omitempty"`
	Status     int       `json:"status"`
	FromCache  bool      `json:"fromCache"`
	FetchedAt  time.Time `json:"fetchedAt"`
	Outcome    string    `json:"outcome"`
	EntryCount int       `json:"entryCount"`
	// CacheControl and MaxAge are the response's Cache-Control header and
	// its max-age; Age is its Age header (zero when absent).
	CacheControl string        `json:"cacheControl,omitempty"`
	MaxAge       time.Duration `json:"maxAge,omitempty"`
	Age          time.Duration `json:"age,omitempty"`
	// DataAsOf estimates when Apple produced the data: FetchedAt minus the
	// CDN copy's age (the Age header; for the iTunes Lookup and Search APIs,
	// which send none, 86,400 s minus max-age, since their edge counts
	// max-age down from a day). A Lookup copy can be up to 24 h older than
	// its fetch time, so snapshots record both.
	DataAsOf time.Time `json:"dataAsOf"`
}

// --- Errors ---------------------------------------------------------------

// NotFoundError reports a 404 from Apple, or a lookup that returned nothing
// for the requested id.
type NotFoundError struct {
	URL  string
	What string
}

func (e *NotFoundError) Error() string {
	if e.What != "" {
		return fmt.Sprintf("not found: %s", e.What)
	}
	return fmt.Sprintf("not found: HTTP 404 for %s", e.URL)
}

// HTTPError is a non-success status other than 403/429 (rate limited), 404
// (NotFoundError) and redirects.
type HTTPError struct {
	URL    string
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("apple returned HTTP %d for %s", e.Status, e.URL)
	if b := strings.TrimSpace(e.Body); b != "" {
		msg += ": " + b
	}
	return msg
}

// PageKindError reports a page whose model is not the expected intent kind
// (for example a product URL that rendered a different page).
type PageKindError struct {
	URL  string
	Want string
	Got  string
}

func (e *PageKindError) Error() string {
	return fmt.Sprintf("unexpected App Store page model at %s: want %s, got %q", e.URL, e.Want, e.Got)
}

// EnvelopeError reports a 200 response whose body is not the documented
// envelope of the surface that was asked for: an empty object, null, an
// error object, or another feed's body. It is a failed capture, never a
// verified-empty one, because only a well-formed envelope can prove that
// Apple has no data.
type EnvelopeError struct {
	Surface string
	Reason  string
}

func (e *EnvelopeError) Error() string {
	return fmt.Sprintf("unexpected %s response: %s", e.Surface, e.Reason)
}

// IsEnvelopeError reports whether err is (or wraps) an EnvelopeError.
func IsEnvelopeError(err error) bool {
	var e *EnvelopeError
	return errors.As(err, &e)
}

// ErrOptInRequired is returned for the robots-disallowed surfaces (the web
// search page, apps.apple.com/api and the other disallowed apps.apple.com
// paths) until the caller enables them explicitly.
var ErrOptInRequired = errors.New("this App Store surface is opt-in only (robots-disallowed); enable it with an explicit --source flag")

// IsNotFound reports whether err is (or wraps) a NotFoundError.
func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}

// --- Lookup / Search ------------------------------------------------------

// LookupQuery selects apps for the iTunes Lookup API. Set exactly one of IDs,
// BundleIDs or ArtistID.
type LookupQuery struct {
	IDs       []string
	BundleIDs []string
	ArtistID  string
	Country   string
	Entity    string
	Limit     int
	Lang      string
}

// LookupArtist is the developer wrapper row of a developer lookup.
type LookupArtist struct {
	ArtistID      int64  `json:"artistId"`
	ArtistName    string `json:"artistName"`
	ArtistType    string `json:"artistType,omitempty"`
	ArtistLinkURL string `json:"artistLinkUrl,omitempty"`
}

// LookupResponse is the merged result of one or more Lookup batches. Results
// keep the input order; Missing lists requested ids (or bundle ids) Apple
// returned nothing for in this storefront.
//
// FetchedAt is when Apple served the oldest batch (a batch from the local
// cache keeps its original fetch time) and FromCache is true when any batch
// came from the cache. Each result also carries its own batch's FetchedAt
// and FromCache: stamp snapshots with those, never with the wall clock.
type LookupResponse struct {
	Country string         `json:"country"`
	Results []LookupResult `json:"results"`
	Missing []string       `json:"missing"`
	// Unresolved lists the ids (or bundle ids) a failed batch left unasked;
	// set only together with an error from Lookup.
	Unresolved []string      `json:"unresolved,omitempty"`
	Artist     *LookupArtist `json:"artist,omitempty"`
	Requests   int           `json:"requests"`
	Meta       []FetchMeta   `json:"-"`
	FetchedAt  time.Time     `json:"-"`
	FromCache  bool          `json:"-"`
}

// observe folds one batch's fetch metadata into FetchedAt and FromCache.
func (r *LookupResponse) observe(m FetchMeta) {
	if !m.FetchedAt.IsZero() && (r.FetchedAt.IsZero() || m.FetchedAt.Before(r.FetchedAt)) {
		r.FetchedAt = m.FetchedAt
	}
	if m.FromCache {
		r.FromCache = true
	}
}

// SearchQuery is an iTunes Search API query for software.
type SearchQuery struct {
	Term    string
	Country string
	Entity  string // software (default) | iPadSoftware | desktopSoftware | macSoftware
	Limit   int    // 1..200
	Lang    string
}

// SearchResponse is the ordered Search API result list. FetchedAt is when
// Apple served it (a cached answer keeps its original fetch time).
type SearchResponse struct {
	Term        string         `json:"term"`
	Country     string         `json:"country"`
	Entity      string         `json:"entity"`
	ResultCount int            `json:"resultCount"`
	Results     []LookupResult `json:"results"`
	Meta        FetchMeta      `json:"-"`
	FetchedAt   time.Time      `json:"-"`
	FromCache   bool           `json:"-"`
}

// LookupResult is one software result from the iTunes Lookup or Search API:
// the 44 documented software fields as typed values, plus Raw with every key
// exactly as Apple sent it.
type LookupResult struct {
	WrapperType                        string   `json:"wrapperType"`
	Kind                               string   `json:"kind"`
	TrackID                            int64    `json:"trackId"`
	TrackName                          string   `json:"trackName"`
	TrackCensoredName                  string   `json:"trackCensoredName,omitempty"`
	TrackViewURL                       string   `json:"trackViewUrl"`
	BundleID                           string   `json:"bundleId"`
	ArtistID                           int64    `json:"artistId"`
	ArtistName                         string   `json:"artistName"`
	ArtistViewURL                      string   `json:"artistViewUrl,omitempty"`
	SellerName                         string   `json:"sellerName,omitempty"`
	SellerURL                          string   `json:"sellerUrl,omitempty"`
	Price                              float64  `json:"price"`
	FormattedPrice                     string   `json:"formattedPrice"`
	Currency                           string   `json:"currency"`
	AverageUserRating                  float64  `json:"averageUserRating"`
	UserRatingCount                    int64    `json:"userRatingCount"`
	AverageUserRatingForCurrentVersion float64  `json:"averageUserRatingForCurrentVersion"`
	UserRatingCountForCurrentVersion   int64    `json:"userRatingCountForCurrentVersion"`
	Version                            string   `json:"version"`
	ReleaseNotes                       string   `json:"releaseNotes,omitempty"`
	CurrentVersionReleaseDate          string   `json:"currentVersionReleaseDate,omitempty"`
	ReleaseDate                        string   `json:"releaseDate,omitempty"`
	Description                        string   `json:"description,omitempty"`
	PrimaryGenreID                     int64    `json:"primaryGenreId"`
	PrimaryGenreName                   string   `json:"primaryGenreName"`
	GenreIDs                           []string `json:"genreIds"`
	Genres                             []string `json:"genres"`
	ContentAdvisoryRating              string   `json:"contentAdvisoryRating,omitempty"`
	TrackContentRating                 string   `json:"trackContentRating,omitempty"`
	Advisories                         []string `json:"advisories"`
	MinimumOSVersion                   string   `json:"minimumOsVersion,omitempty"`
	FileSizeBytes                      string   `json:"fileSizeBytes,omitempty"`
	LanguageCodesISO2A                 []string `json:"languageCodesISO2A"`
	SupportedDevices                   []string `json:"supportedDevices"`
	Features                           []string `json:"features"`
	IsGameCenterEnabled                bool     `json:"isGameCenterEnabled"`
	IsVppDeviceBasedLicensingEnabled   bool     `json:"isVppDeviceBasedLicensingEnabled"`
	ArtworkURL60                       string   `json:"artworkUrl60,omitempty"`
	ArtworkURL100                      string   `json:"artworkUrl100,omitempty"`
	ArtworkURL512                      string   `json:"artworkUrl512,omitempty"`
	ScreenshotURLs                     []string `json:"screenshotUrls"`
	IPadScreenshotURLs                 []string `json:"ipadScreenshotUrls"`
	AppleTVScreenshotURLs              []string `json:"appletvScreenshotUrls"`

	// Raw holds every key of the result exactly as Apple sent it.
	Raw map[string]json.RawMessage `json:"-"`

	// FetchedAt is when Apple served the response this result came from (a
	// result from the local cache keeps its original fetch time); FromCache
	// says it was served from the cache. Zero for results parsed from bytes.
	// store.RecordLookupSnapshot stamps the snapshot with FetchedAt.
	FetchedAt time.Time `json:"-"`
	FromCache bool      `json:"-"`
	// DataAsOf is when Apple produced this result (FetchMeta.DataAsOf of its
	// batch): the Lookup CDN serves per-URL copies up to 24 h old.
	DataAsOf time.Time `json:"-"`
}

// AppID returns the track id as a decimal string ("" when absent).
func (r LookupResult) AppID() string {
	if r.TrackID == 0 {
		return ""
	}
	return strconv.FormatInt(r.TrackID, 10)
}

// CanonicalURL returns trackViewUrl without Apple's "?uo=4" tracking suffix.
func (r LookupResult) CanonicalURL() string { return CanonicalAppURL(r.TrackViewURL) }

// UnmarshalJSON decodes leniently: a field whose type differs from the
// documented one (a price sent as a string, say) is coerced or left zero
// instead of failing the whole response.
func (r *LookupResult) UnmarshalJSON(data []byte) error {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var generic map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return err
	}
	str := func(k string) string { return strOf(generic[k]) }
	strs := func(k string) []string {
		out := make([]string, 0)
		for _, v := range asArray(generic[k]) {
			if s := strOf(v); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	*r = LookupResult{
		WrapperType:                        str("wrapperType"),
		Kind:                               str("kind"),
		TrackID:                            intOf(generic["trackId"]),
		TrackName:                          str("trackName"),
		TrackCensoredName:                  str("trackCensoredName"),
		TrackViewURL:                       str("trackViewUrl"),
		BundleID:                           str("bundleId"),
		ArtistID:                           intOf(generic["artistId"]),
		ArtistName:                         str("artistName"),
		ArtistViewURL:                      str("artistViewUrl"),
		SellerName:                         str("sellerName"),
		SellerURL:                          str("sellerUrl"),
		Price:                              floatOf(generic["price"]),
		FormattedPrice:                     str("formattedPrice"),
		Currency:                           str("currency"),
		AverageUserRating:                  floatOf(generic["averageUserRating"]),
		UserRatingCount:                    intOf(generic["userRatingCount"]),
		AverageUserRatingForCurrentVersion: floatOf(generic["averageUserRatingForCurrentVersion"]),
		UserRatingCountForCurrentVersion:   intOf(generic["userRatingCountForCurrentVersion"]),
		Version:                            str("version"),
		ReleaseNotes:                       str("releaseNotes"),
		CurrentVersionReleaseDate:          str("currentVersionReleaseDate"),
		ReleaseDate:                        str("releaseDate"),
		Description:                        str("description"),
		PrimaryGenreID:                     intOf(generic["primaryGenreId"]),
		PrimaryGenreName:                   str("primaryGenreName"),
		GenreIDs:                           strs("genreIds"),
		Genres:                             strs("genres"),
		ContentAdvisoryRating:              str("contentAdvisoryRating"),
		TrackContentRating:                 str("trackContentRating"),
		Advisories:                         strs("advisories"),
		MinimumOSVersion:                   str("minimumOsVersion"),
		FileSizeBytes:                      str("fileSizeBytes"),
		LanguageCodesISO2A:                 strs("languageCodesISO2A"),
		SupportedDevices:                   strs("supportedDevices"),
		Features:                           strs("features"),
		IsGameCenterEnabled:                boolOf(generic["isGameCenterEnabled"]),
		IsVppDeviceBasedLicensingEnabled:   boolOf(generic["isVppDeviceBasedLicensingEnabled"]),
		ArtworkURL60:                       str("artworkUrl60"),
		ArtworkURL100:                      str("artworkUrl100"),
		ArtworkURL512:                      str("artworkUrl512"),
		ScreenshotURLs:                     strs("screenshotUrls"),
		IPadScreenshotURLs:                 strs("ipadScreenshotUrls"),
		AppleTVScreenshotURLs:              strs("appletvScreenshotUrls"),
		Raw:                                raw,
	}
	return nil
}

// LookupFieldNames are the 44 documented software result keys.
var LookupFieldNames = []string{
	"advisories", "appletvScreenshotUrls", "artistId", "artistName", "artistViewUrl",
	"artworkUrl100", "artworkUrl512", "artworkUrl60", "averageUserRating",
	"averageUserRatingForCurrentVersion", "bundleId", "contentAdvisoryRating", "currency",
	"currentVersionReleaseDate", "description", "features", "fileSizeBytes", "formattedPrice",
	"genreIds", "genres", "ipadScreenshotUrls", "isGameCenterEnabled",
	"isVppDeviceBasedLicensingEnabled", "kind", "languageCodesISO2A", "minimumOsVersion", "price",
	"primaryGenreId", "primaryGenreName", "releaseDate", "releaseNotes", "screenshotUrls",
	"sellerName", "sellerUrl", "supportedDevices", "trackCensoredName", "trackContentRating",
	"trackId", "trackName", "trackViewUrl", "userRatingCount", "userRatingCountForCurrentVersion",
	"version", "wrapperType",
}

// --- Feeds ----------------------------------------------------------------

// ChartEntry is one row of a legacy iTunes RSS chart feed.
type ChartEntry struct {
	Rank        int     `json:"rank"`
	AppID       string  `json:"appId"`
	BundleID    string  `json:"bundleId,omitempty"`
	Name        string  `json:"name"`
	Artist      string  `json:"artist,omitempty"`
	ArtistID    string  `json:"artistId,omitempty"`
	Price       string  `json:"price,omitempty"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency,omitempty"`
	GenreID     string  `json:"genreId,omitempty"`
	Genre       string  `json:"genre,omitempty"`
	ReleaseDate string  `json:"releaseDate,omitempty"`
	URL         string  `json:"url,omitempty"`
}

// ChartFeed is a parsed legacy RSS chart. FetchedAt is when Apple served it
// (a cached feed keeps its original fetch time): the chart snapshot time.
type ChartFeed struct {
	Country   string       `json:"country"`
	Feed      string       `json:"feed"`
	GenreID   string       `json:"genreId,omitempty"`
	Limit     int          `json:"limit"`
	Updated   string       `json:"updated,omitempty"`
	Entries   []ChartEntry `json:"entries"`
	Outcome   string       `json:"outcome"`
	Meta      FetchMeta    `json:"-"`
	FetchedAt time.Time    `json:"-"`
	FromCache bool         `json:"-"`
}

// Review is one customer review from the RSS feed, the web see-all page or
// the opt-in /api reviews resource.
type Review struct {
	ID                string `json:"reviewId"`
	Rating            int    `json:"rating"`
	Version           string `json:"version,omitempty"`
	Title             string `json:"title"`
	Body              string `json:"body"`
	Author            string `json:"author"`
	ReviewedAt        string `json:"reviewedAt"`
	VoteSum           int    `json:"voteSum"`
	VoteCount         int    `json:"voteCount"`
	DeveloperResponse string `json:"developerResponse,omitempty"`
	Edited            bool   `json:"edited,omitempty"`
}

// Review sort orders accepted by the RSS feed.
const (
	SortMostRecent  = "mostrecent"
	SortMostHelpful = "mosthelpful"
)

// ReviewsFeed is one page of the customer-reviews RSS feed. FetchedAt is when
// Apple served it (a cached page keeps its original fetch time).
type ReviewsFeed struct {
	Country    string    `json:"country"`
	AppID      string    `json:"appId"`
	Page       int       `json:"page"`
	Sort       string    `json:"sort"`
	Reviews    []Review  `json:"reviews"`
	Outcome    string    `json:"outcome"`
	Oldest     string    `json:"oldest,omitempty"`
	Newest     string    `json:"newest,omitempty"`
	EntryCount int       `json:"entryCount"`
	Meta       FetchMeta `json:"-"`
	FetchedAt  time.Time `json:"-"`
	FromCache  bool      `json:"-"`
}

// HintsResult is the search-hints (autocomplete) answer for one term.
type HintsResult struct {
	Term        string    `json:"term"`
	Country     string    `json:"country"`
	Suggestions []string  `json:"suggestions"`
	Meta        FetchMeta `json:"-"`
	FetchedAt   time.Time `json:"-"`
	FromCache   bool      `json:"-"`
}
