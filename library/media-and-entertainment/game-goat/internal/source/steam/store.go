// store.go — typed keyless Steam store-CATALOG client.
//
// steam.go answers per-title questions (storesearch -> appid, appreviews,
// appdetails). This file adds ENUMERATION: plural search, filtered pagination,
// and full typed app records — which is what makes questions like "every free
// demo on Steam right now" expressible. You cannot filter what you cannot
// enumerate.
//
// Four keyless Valve store services on api.steampowered.com power it:
//   - IStoreQueryService/SearchSuggestions  text -> ranked store items
//   - IStoreQueryService/Query              filtered, PAGINATED enumeration
//   - IStoreBrowseService/GetItems          batch appids -> full records
//   - IStoreService/GetTagList              tagid -> human tag name
//
// None of them appear on https://partner.steamgames.com/doc/api. The documented
// catalog endpoint there, IStoreService/GetAppList, requires a Steam Web API key
// and cannot filter by demo, tag, or price. These store services are keyless and
// take a country_code + language context, so the app is localisable like the
// IsThereAnyDeal path. The keys in the JSON payloads are lowercase snake_case
// (they are protobuf fields serialised through the web API's input_json
// convention), which is why the wire structs carry explicit json tags.

package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAPIBaseURL hosts Valve's keyless store services (distinct from
	// DefaultBaseURL, the storefront host used by storesearch/appdetails).
	DefaultAPIBaseURL     = "https://api.steampowered.com"
	searchSuggestionsPath = "/IStoreQueryService/SearchSuggestions/v1/"
	queryPath             = "/IStoreQueryService/Query/v1/"
	getItemsPath          = "/IStoreBrowseService/GetItems/v1/"
	getTagListPath        = "/IStoreService/GetTagList/v1/"

	// MaxPageSize caps one service page (Valve accepts up to 100).
	MaxPageSize = 100

	// MaxSearchBatch caps SearchSuggestions max_results.
	MaxSearchBatch = 1000

	// MaxItemsPerRequest caps GetItems appids per request. The edge rejects
	// query strings above ~8192 bytes (about 265 ids), so batch well below it.
	MaxItemsPerRequest = 200

	// earlyAccessTagID is the store's "Early Access" tag. Steam models early
	// access as a tag, not as an app type.
	earlyAccessTagID = 493
	// tagCount is how many weighted tags the service is asked to return per item.
	tagCount = 10
	// parentTagCount is how many weighted tags the demos parent lookup asks
	// for. Steam caps an item's tags at 20, and a full game's tags stand in for
	// its demo's own tags (demos almost never carry tags).
	parentTagCount = 20

	// storeItemAssetBase prefixes the asset_url_format field of GetItems.
	storeItemAssetBase = "https://shared.akamai.steamstatic.com/store_item_assets/"
)

// ErrAmbiguousApp: the title matched several equally plausible store apps
// (remakes, re-releases, editions). Callers report the candidates instead of
// guessing; a wrong match presents another game's reviews and price.
var ErrAmbiguousApp = errors.New("steam: title matched multiple store apps")

// ErrPartialLookup: some GetItems chunks in a batched full-game lookup failed
// while others succeeded. The returned map carries the successful chunks; the
// error names the last failing chunk so callers mark the source as partial
// (steam_parent) without discarding the rows they did resolve.
var ErrPartialLookup = errors.New("steam: some full-game lookups failed")

// ErrTagNamesUnavailable: the store tag dictionary could not be fetched, so
// tag NAMES are missing even though the tag ids are present. Non-fatal: the
// summaries stay usable and callers mark steam_tags rather than failing.
var ErrTagNamesUnavailable = errors.New("steam: store tag names unavailable")

// ErrAppHidden: the app exists but is hidden from anonymous store requests
// (age or region gate). It is deliberately its own message rather than an
// ErrAppNotFound wrap: wrapping made the CLI report a hidden app as "no Steam
// app matched the title", which is false. Is() keeps it a not-found for
// existing handling and the CLI's not-found exit code; callers that can
// distinguish the cases report the store record and demo state as unknown
// instead of empty.
var ErrAppHidden error = hiddenAppError{}

// hiddenAppError is the comparable sentinel type behind ErrAppHidden.
type hiddenAppError struct{}

func (hiddenAppError) Error() string {
	return "steam: app is hidden from anonymous store requests (age or region gate)"
}

func (hiddenAppError) Is(target error) bool {
	return target == ErrAppNotFound
}

// hiddenAppNotFoundError carries the app id(s) into the hidden-app message
// while keeping ErrAppHidden in the Unwrap chain, so errors.Is works for both
// the hidden sentinel and ErrAppNotFound.
type hiddenAppNotFoundError struct{ ids string }

func (e hiddenAppNotFoundError) Error() string {
	return fmt.Sprintf("steam: app %s is hidden from anonymous store requests (age or region gate); its store record and demo state are unknown", e.ids)
}

func (e hiddenAppNotFoundError) Unwrap() error { return ErrAppHidden }

// AppType is the typed app taxonomy the store services expose. Steam models
// "free to play" and "early access" as ATTRIBUTES (is_free, the Early Access
// tag), not as types, so those are fields on StoreItem rather than AppType
// values.
type AppType string

const (
	AppTypeGame       AppType = "game"
	AppTypeDemo       AppType = "demo"
	AppTypeMod        AppType = "mod"
	AppTypeDLC        AppType = "dlc"
	AppTypeSoundtrack AppType = "soundtrack"
	AppTypeSoftware   AppType = "software"
	AppTypeVideo      AppType = "video"
	AppTypeHardware   AppType = "hardware"
	// AppTypeBundle is recognised as a flag value but is not enumerable: the
	// keyless store services expose no bundle filter (a bundle query returns
	// zero rows), so the commands reject it with that explanation.
	AppTypeBundle AppType = "bundle"
	// AppTypeOther is anything the taxonomy does not model.
	AppTypeOther AppType = "other"
)

// appTypeFromService maps the service "type" integer to an AppType. Values
// verified live against IStoreQueryService/Query and IStoreBrowseService/GetItems.
func appTypeFromService(t int) AppType {
	switch t {
	case 0:
		return AppTypeGame
	case 1:
		return AppTypeDemo
	case 2:
		return AppTypeMod
	case 4:
		return AppTypeDLC
	case 6:
		return AppTypeSoftware
	case 7:
		return AppTypeVideo
	case 10:
		return AppTypeHardware
	case 11:
		return AppTypeSoundtrack
	default:
		return AppTypeOther
	}
}

// ParseAppType validates a --type flag value. "music" is accepted as a synonym
// for the store's soundtrack type.
func ParseAppType(s string) (AppType, error) {
	switch AppType(strings.ToLower(strings.TrimSpace(s))) {
	case AppTypeGame, AppTypeDemo, AppTypeMod, AppTypeDLC, AppTypeSoftware, AppTypeVideo, AppTypeHardware, AppTypeBundle:
		return AppType(strings.ToLower(strings.TrimSpace(s))), nil
	case AppTypeSoundtrack, "music", "soundtracks":
		return AppTypeSoundtrack, nil
	case "":
		return "", fmt.Errorf("empty app type")
	default:
		return "", fmt.Errorf("unknown app type %q; valid types: game, demo, dlc, soundtrack, software, video, mod, hardware, bundle", s)
	}
}

// ParseAppTypes splits a comma-separated --type value.
func ParseAppTypes(csv string) ([]AppType, error) {
	var out []AppType
	for _, part := range strings.Split(csv, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		t, err := ParseAppType(part)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no app type given; valid types: game, demo, dlc, soundtrack, software, video, mod, hardware")
	}
	return out, nil
}

// typeFilters maps AppType values onto the service's type_filters booleans.
// AppTypeBundle has no keyless filter and is reported as an error.
func typeFilters(types []AppType) (map[string]bool, error) {
	filters := map[string]bool{}
	for _, t := range types {
		switch t {
		case AppTypeGame:
			filters["include_games"] = true
		case AppTypeDemo:
			filters["include_demos"] = true
		case AppTypeMod:
			filters["include_mods"] = true
		case AppTypeDLC:
			filters["include_dlc"] = true
		case AppTypeSoundtrack:
			filters["include_music"] = true
		case AppTypeSoftware:
			filters["include_software"] = true
		case AppTypeVideo:
			filters["include_video"] = true
		case AppTypeHardware:
			filters["include_hardware"] = true
		case AppTypeBundle:
			return nil, fmt.Errorf("steam: bundles are not enumerable: the keyless store services expose no bundle filter (only a Steam Web API key or the storefront HTML does)")
		default:
			return nil, fmt.Errorf("steam: unsupported app type %q", string(t))
		}
	}
	return filters, nil
}

// PlatformSupport is the per-OS availability block GetItems returns.
type PlatformSupport struct {
	Windows         bool `json:"windows"`
	Mac             bool `json:"mac"`
	Linux           bool `json:"linux"`
	SteamDeckCompat int  `json:"steam_deck_compat,omitempty"`
}

// Tag is a weighted store tag. Name is filled from GetTagList when that call
// succeeds; an empty Name still leaves the id usable.
type Tag struct {
	ID     int    `json:"id"`
	Name   string `json:"name,omitempty"`
	Weight int    `json:"weight,omitempty"`
}

// StorePrice is the current best purchase option, in the country's currency.
type StorePrice struct {
	FinalCents        int    `json:"final_cents"`
	OriginalCents     int    `json:"original_cents"`
	Formatted         string `json:"formatted,omitempty"`
	OriginalFormatted string `json:"original_formatted,omitempty"`
	DiscountPercent   int    `json:"discount_percent,omitempty"`
}

// StoreItem is the full typed store record shared by search, browse, and get.
type StoreItem struct {
	AppID               int64           `json:"app_id"`
	Name                string          `json:"name"`
	Type                AppType         `json:"type"`
	IsFree              bool            `json:"is_free,omitempty"`
	ComingSoon          bool            `json:"coming_soon,omitempty"`
	EarlyAccess         bool            `json:"early_access,omitempty"`
	ReleaseDate         string          `json:"release_date,omitempty"`
	OriginalReleaseDate string          `json:"original_release_date,omitempty"`
	Platforms           PlatformSupport `json:"platforms"`
	Tags                []Tag           `json:"tags,omitempty"`
	Publishers          []string        `json:"publishers,omitempty"`
	Developers          []string        `json:"developers,omitempty"`
	Franchises          []string        `json:"franchises,omitempty"`
	ShortDescription    string          `json:"short_description,omitempty"`
	Price               *StorePrice     `json:"price,omitempty"`
	ParentAppID         int64           `json:"parent_app_id,omitempty"`
	DemoAppIDs          []int64         `json:"demo_app_ids,omitempty"`
	StoreURL            string          `json:"store_url,omitempty"`
	HeaderImageURL      string          `json:"header_image_url,omitempty"`
}

// ReleasesInYear reports whether the item's release date falls in year.
func (i StoreItem) ReleasesInYear(year int) bool {
	if year <= 0 || len(i.ReleaseDate) < 4 {
		return false
	}
	y, err := strconv.Atoi(i.ReleaseDate[:4])
	return err == nil && y == year
}

// ---------------------------------------------------------------------------
// Options and results
// ---------------------------------------------------------------------------

// SearchOptions configures Search. Limit is capped at MaxPageSize.
type SearchOptions struct {
	Types []AppType
	Limit int
}

// BrowseOptions configures Browse. Start/Count paginate; the rest filter.
type BrowseOptions struct {
	Types        []AppType
	FreeOnly     bool
	TagIDs       []int // every listed tag is required (AND across tags)
	ComingSoon   bool
	ReleasedOnly bool
	Start        int
	Count        int
	// SkipTagNames skips the GetTagList request that fills Tag.Name. Callers
	// that only need tag ids (or no tags) save that request.
	SkipTagNames bool
}

// SearchPageOptions configures SearchPage: the same filters as Browse but over
// the text-search endpoint, which reports a total but ignores offsets.
type SearchPageOptions struct {
	Types        []AppType
	TagIDs       []int // every listed tag is required (AND across tags)
	ComingSoon   bool  // unreleased items only (server-side coming_soon_only)
	ReleasedOnly bool  // released items only (server-side released_only)
	Limit        int   // capped at MaxSearchBatch; defaults to 100
}

// Page is one page of a paginated browse. Count is the number of items
// actually returned in this page.
type Page struct {
	Total int         `json:"total"`
	Start int         `json:"start"`
	Count int         `json:"count"`
	Items []StoreItem `json:"items"`
}

// HasMore reports whether a further page exists after this one.
func (p Page) HasMore() bool { return p.Start+p.Count < p.Total }

// Truncated reports whether the service matched more records than this page
// returned. It is the text-search view of HasMore: the endpoint has no offset,
// so the only honest signal is total > returned.
func (p *Page) Truncated() bool { return p.Total > len(p.Items) }

// ---------------------------------------------------------------------------
// Wire shapes (the services serialise protobuf fields as lowercase snake_case)
// ---------------------------------------------------------------------------

type storeReleaseWire struct {
	SteamReleaseDate         int64 `json:"steam_release_date"`
	OriginalSteamReleaseDate int64 `json:"original_steam_release_date"`
	IsEarlyAccess            bool  `json:"is_early_access"`
	ComingSoon               bool  `json:"coming_soon"`
	IsComingSoon             bool  `json:"is_coming_soon"`
}

type storeNamedWire struct {
	Name string `json:"name"`
}

type storeTagWire struct {
	TagID  int `json:"tagid"`
	Weight int `json:"weight"`
}

type storeRelatedWire struct {
	ParentAppID int64   `json:"parent_appid"`
	DemoAppID   []int64 `json:"demo_appid"`
	Demos       []struct {
		AppID int64 `json:"appid"`
	} `json:"demos"`
}

type storePlatformsWire struct {
	Windows   bool `json:"windows"`
	Mac       bool `json:"mac"`
	Linux     bool `json:"linux"`
	SteamDeck int  `json:"steam_deck_compat_category"`
}

type storePurchaseOptionWire struct {
	FinalPriceInCents      string `json:"final_price_in_cents"`
	OriginalPriceInCents   string `json:"original_price_in_cents"`
	FormattedFinalPrice    string `json:"formatted_final_price"`
	FormattedOriginalPrice string `json:"formatted_original_price"`
	DiscountPct            int    `json:"discount_pct"`
}

type storeAssetsWire struct {
	AssetURLFormat string `json:"asset_url_format"`
	Header         string `json:"header"`
}

type storeItemWire struct {
	ItemType     int                `json:"item_type"`
	ID           int64              `json:"id"`
	AppID        int64              `json:"appid"`
	Success      int                `json:"success"`
	Visible      bool               `json:"visible"`
	Name         string             `json:"name"`
	Type         int                `json:"type"`
	IsFree       bool               `json:"is_free"`
	StoreURLPath string             `json:"store_url_path"`
	TagIDs       []int              `json:"tagids"`
	Tags         []storeTagWire     `json:"tags"`
	Release      storeReleaseWire   `json:"release"`
	Platforms    storePlatformsWire `json:"platforms"`
	BasicInfo    struct {
		ShortDescription string           `json:"short_description"`
		Publishers       []storeNamedWire `json:"publishers"`
		Developers       []storeNamedWire `json:"developers"`
		Franchises       []storeNamedWire `json:"franchises"`
	} `json:"basic_info"`
	RelatedItems       storeRelatedWire         `json:"related_items"`
	BestPurchaseOption *storePurchaseOptionWire `json:"best_purchase_option"`
	Assets             storeAssetsWire          `json:"assets"`
}

type storeMetaWire struct {
	TotalMatchingRecords int `json:"total_matching_records"`
	Start                int `json:"start"`
	Count                int `json:"count"`
}

type storeResponseWire struct {
	Response struct {
		Metadata   storeMetaWire   `json:"metadata"`
		StoreItems []storeItemWire `json:"store_items"`
	} `json:"response"`
}

type storeTagListWire struct {
	Response struct {
		Tags []struct {
			TagID int    `json:"tagid"`
			Name  string `json:"name"`
		} `json:"tags"`
	} `json:"response"`
}

// ---------------------------------------------------------------------------
// Enumeration
// ---------------------------------------------------------------------------

// Search returns ranked store items for a text term.
//
// Contrast with ResolveAppID's storefront storesearch endpoint, which caps at
// 10 results: this service returns up to MaxPageSize. It IGNORES an offset,
// though, so text search has no second page — use Browse when you need
// pagination. An empty result set is the typed ErrAppNotFound.
func (c *Client) Search(ctx context.Context, term string, opts SearchOptions) ([]StoreItem, error) {
	items, err := c.search(ctx, term, opts.Types, opts.Limit)
	if err != nil {
		return nil, err
	}
	c.attachTagNames(ctx, items)
	return items, nil
}

// search is Search without the tag-name enrichment, for callers that only need
// identity (resolution) and should not spend a request on names.
func (c *Client) search(ctx context.Context, term string, types []AppType, limit int) ([]StoreItem, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, fmt.Errorf("steam: empty search term")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	if len(types) == 0 {
		types = []AppType{AppTypeGame}
	}
	filters, err := typeFilters(types)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"search_term":  term,
		"max_results":  limit,
		"context":      storeContext(c),
		"data_request": storeDataRequest(),
		"filters":      map[string]any{"type_filters": filters},
	}
	var wire storeResponseWire
	if err := c.serviceGet(ctx, searchSuggestionsPath, payload, &wire); err != nil {
		return nil, err
	}
	items := convertItems(wire.Response.StoreItems)
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrAppNotFound, term)
	}
	return items, nil
}

// SearchPage returns ranked store items for a text term together with the
// service's total match count. Unlike Search, an empty result is a legitimate
// answer (an empty Page, not an error), and unlike Browse the endpoint ignores
// offsets: Total may exceed the returned items, which Truncated reports.
func (c *Client) SearchPage(ctx context.Context, term string, opts SearchPageOptions) (*Page, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, fmt.Errorf("steam: empty search term")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > MaxSearchBatch {
		limit = MaxSearchBatch
	}
	types := opts.Types
	if len(types) == 0 {
		types = []AppType{AppTypeGame}
	}
	tf, err := typeFilters(types)
	if err != nil {
		return nil, err
	}
	filters := map[string]any{"type_filters": tf}
	if opts.ComingSoon {
		filters["coming_soon_only"] = true
	}
	if opts.ReleasedOnly {
		filters["released_only"] = true
	}
	if len(opts.TagIDs) > 0 {
		// One group per tag: the service ORs within a group and ANDs across
		// them, so every listed tag is required. Same shape as Browse.
		groups := make([]map[string]any, 0, len(opts.TagIDs))
		for _, id := range opts.TagIDs {
			groups = append(groups, map[string]any{"tagids": []int{id}})
		}
		filters["tagids_must_match"] = groups
	}
	payload := map[string]any{
		"search_term":  term,
		"max_results":  limit,
		"context":      storeContext(c),
		"data_request": storeDataRequest(),
		"filters":      filters,
	}
	var wire storeResponseWire
	if err := c.serviceGet(ctx, searchSuggestionsPath, payload, &wire); err != nil {
		return nil, err
	}
	items := convertItems(wire.Response.StoreItems)
	return &Page{
		Total: wire.Response.Metadata.TotalMatchingRecords,
		Start: wire.Response.Metadata.Start,
		Count: len(items),
		Items: items,
	}, nil
}

// Browse walks the store catalog with filters and real pagination. An empty
// page is a legitimate answer (unlike Search), so only transport and decode
// failures return an error.
func (c *Client) Browse(ctx context.Context, opts BrowseOptions) (*Page, error) {
	count := opts.Count
	if count <= 0 {
		count = 20
	}
	if count > MaxPageSize {
		count = MaxPageSize
	}
	start := opts.Start
	if start < 0 {
		start = 0
	}
	if len(opts.Types) == 0 {
		opts.Types = []AppType{AppTypeGame}
	}
	tf, err := typeFilters(opts.Types)
	if err != nil {
		return nil, err
	}
	filters := map[string]any{"type_filters": tf}
	if opts.FreeOnly {
		filters["price_filters"] = map[string]any{"only_free_items": true}
	}
	if opts.ComingSoon {
		filters["coming_soon_only"] = true
	}
	if opts.ReleasedOnly {
		filters["released_only"] = true
	}
	if len(opts.TagIDs) > 0 {
		// The service ORs tagids within one group and ANDs across groups, so
		// one group per tag makes every listed tag required.
		groups := make([]map[string]any, 0, len(opts.TagIDs))
		for _, id := range opts.TagIDs {
			groups = append(groups, map[string]any{"tagids": []int{id}})
		}
		filters["tagids_must_match"] = groups
	}
	payload := map[string]any{
		"query": map[string]any{
			"start":   start,
			"count":   count,
			"filters": filters,
		},
		"context":      storeContext(c),
		"data_request": storeDataRequest(),
	}
	var wire storeResponseWire
	if err := c.serviceGet(ctx, queryPath, payload, &wire); err != nil {
		return nil, err
	}
	items := convertItems(wire.Response.StoreItems)
	if !opts.SkipTagNames {
		c.attachTagNames(ctx, items)
	}
	return &Page{
		Total: wire.Response.Metadata.TotalMatchingRecords,
		Start: wire.Response.Metadata.Start,
		Count: len(items),
		Items: items,
	}, nil
}

// Items returns full typed records for a batch of appids in one request. It is
// the "app details as a full typed record" path: one request for many apps,
// unlike the storefront appdetails endpoint. When no appid yields a record the
// typed ErrAppNotFound is returned; when the only records for the requested ids
// are hidden from anonymous requests (age or region gate), ErrAppHidden is
// returned instead so callers report the record and demo state as unknown.
func (c *Client) Items(ctx context.Context, appIDs []int64) ([]StoreItem, error) {
	if len(appIDs) == 0 {
		return nil, nil
	}
	var items []StoreItem
	var lastErr error
	var hidden []int64
	for _, chunk := range chunkIDs(appIDs, MaxItemsPerRequest) {
		wires, hiddenIDs, err := c.getItemsChunk(ctx, chunk, storeDataRequest())
		if err != nil {
			lastErr = err
			continue
		}
		hidden = append(hidden, hiddenIDs...)
		if len(wires) == 0 {
			lastErr = fmt.Errorf("%w: app %s (store service returned no record)", ErrAppNotFound, joinIDs(chunk))
			continue
		}
		for _, w := range wires {
			items = append(items, w.toStoreItem())
		}
	}
	if len(items) == 0 {
		if len(hidden) > 0 {
			return nil, hiddenAppNotFoundError{ids: joinIDs(hidden)}
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("%w: app %s (store service returned no record)", ErrAppNotFound, joinIDs(appIDs))
	}
	c.attachTagNames(ctx, items)
	return items, nil
}

// getItemsChunk issues one GetItems request for appIDs and returns the wire
// records the service actually returned (success == 1 and visible) plus the
// appids the service answered as hidden (success != 1 or visible false). Items,
// AppSummaries, and DemoLinks all go through it; they differ only in
// the data_request they need and in how they treat the hidden ids.
func (c *Client) getItemsChunk(ctx context.Context, appIDs []int64, dataRequest map[string]any) ([]storeItemWire, []int64, error) {
	ids := make([]map[string]any, 0, len(appIDs))
	for _, id := range appIDs {
		ids = append(ids, map[string]any{"appid": id})
	}
	payload := map[string]any{
		"ids":          ids,
		"context":      storeContext(c),
		"data_request": dataRequest,
	}
	var wire storeResponseWire
	if err := c.serviceGet(ctx, getItemsPath, payload, &wire); err != nil {
		return nil, nil, err
	}
	out := make([]storeItemWire, 0, len(wire.Response.StoreItems))
	var hidden []int64
	for _, w := range wire.Response.StoreItems {
		// Only EResult 1 (OK) is a real record. success:15 and visible:false
		// mark an app hidden from anonymous requests (age or region gate);
		// counting it as found yields an empty name and a false has_demo.
		if w.Success != 1 || !w.Visible {
			id := w.AppID
			if id == 0 {
				id = w.ID
			}
			if id != 0 {
				hidden = append(hidden, id)
			}
			continue
		}
		out = append(out, w)
	}
	return out, hidden, nil
}

// Item returns the full typed record for exactly one appid.
func (c *Client) Item(ctx context.Context, appID int64) (*StoreItem, error) {
	items, err := c.Items(ctx, []int64{appID})
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].AppID == appID {
			return &items[i], nil
		}
	}
	return &items[0], nil
}

// AppSummary is the demos parent-lookup record: the full game's display name
// plus its store tags. Demos rarely carry tags of their own, so the parent's
// tags are what makes a demos row useful; name and tags arrive in the same
// GetItems request.
type AppSummary struct {
	Name string `json:"name"`
	Tags []Tag  `json:"tags,omitempty"`
}

// AppSummaries maps appids to their store name AND tags in ONE GetItems
// request per MaxItemsPerRequest chunk, asking for include_tag_count (Steam
// caps an item's tags at 20). Ids that yield no record are absent, matching
// Items. Tag.Name is filled from the cached tag dictionary — the same one
// a demos page already fetched for its own rows — so the parent lookup adds no
// request.
//
// A non-nil map returned alongside a non-nil error means PARTIAL data: some
// requests succeeded and the map carries their summaries. ErrPartialLookup
// marks whole chunks that failed; ErrTagNamesUnavailable marks a failed tag
// dictionary (names missing, ids retained). Both can be joined. Only when
// every chunk fails does it return (nil, err) as before.
func (c *Client) AppSummaries(ctx context.Context, ids []int64) (map[int64]AppSummary, error) {
	unique := dedupePositiveIDs(ids)
	summaries := make(map[int64]AppSummary, len(unique))
	chunks := chunkIDs(unique, MaxItemsPerRequest)
	requested := false
	var lastErr error
	for _, chunk := range chunks {
		wires, _, err := c.getItemsChunk(ctx, chunk, map[string]any{
			"include_basic_info": true,
			"include_tag_count":  parentTagCount,
		})
		if err != nil {
			lastErr = err
			continue
		}
		requested = true
		for _, w := range wires {
			appID := w.AppID
			if appID == 0 {
				appID = w.ID
			}
			if appID == 0 {
				continue
			}
			summaries[appID] = AppSummary{Name: w.Name, Tags: tagsFrom(w)}
		}
	}
	if len(chunks) > 0 && !requested {
		return nil, lastErr
	}
	var partialErr error
	if lastErr != nil {
		partialErr = fmt.Errorf("%w: %w", ErrPartialLookup, lastErr)
	}
	var tagErr error
	if err := c.attachSummaryTagNames(ctx, summaries); err != nil {
		tagErr = fmt.Errorf("%w: %w", ErrTagNamesUnavailable, err)
	}
	return summaries, errors.Join(partialErr, tagErr)
}

// attachSummaryTagNames fills Tag.Name on each summary from the cached tag
// dictionary. It returns the dictionary error so AppSummaries can mark
// steam_tags: a dictionary failure leaves the ids usable but the names
// unknown.
func (c *Client) attachSummaryTagNames(ctx context.Context, summaries map[int64]AppSummary) error {
	if len(summaries) == 0 {
		return nil
	}
	names, err := c.tagNames(ctx)
	if err != nil {
		return err
	}
	for id, summary := range summaries {
		applyTagNames(summary.Tags, names)
		summaries[id] = summary
	}
	return nil
}

// DemoLinks maps each appid to the demo appids GetItems reports for it. A found
// app without demos maps to a non-nil empty slice; ids with no record are
// absent.
func (c *Client) DemoLinks(ctx context.Context, ids []int64) (map[int64][]int64, error) {
	unique := dedupePositiveIDs(ids)
	links := make(map[int64][]int64, len(unique))
	chunks := chunkIDs(unique, MaxItemsPerRequest)
	requested := false
	var lastErr error
	for _, chunk := range chunks {
		wires, _, err := c.getItemsChunk(ctx, chunk, map[string]any{
			"include_basic_info":    true,
			"include_related_items": true,
		})
		if err != nil {
			lastErr = err
			continue
		}
		requested = true
		for _, w := range wires {
			appID := w.AppID
			if appID == 0 {
				appID = w.ID
			}
			if appID == 0 {
				continue
			}
			demos := demoAppIDs(w.RelatedItems)
			if demos == nil {
				demos = []int64{}
			}
			links[appID] = demos
		}
	}
	if len(chunks) > 0 && !requested {
		return nil, lastErr
	}
	return links, nil
}

// ---------------------------------------------------------------------------
// Tag names
// ---------------------------------------------------------------------------

// ResolveTag accepts a store tag name ("Roguelike") or numeric id and returns
// the tagid. Unknown input is an error naming close matches, so a typo never
// silently widens a browse.
func (c *Client) ResolveTag(ctx context.Context, nameOrID string) (int, error) {
	names, err := c.tagNames(ctx)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(nameOrID)
	if s == "" {
		return 0, fmt.Errorf("steam: empty tag")
	}
	if id, cerr := strconv.Atoi(s); cerr == nil {
		if _, ok := names[id]; !ok {
			return 0, fmt.Errorf("steam: unknown tag id %d", id)
		}
		return id, nil
	}
	target := normalizeName(s)
	for id, name := range names {
		if normalizeName(name) == target {
			return id, nil
		}
	}
	var close []string
	for _, name := range names {
		if strings.Contains(normalizeName(name), target) {
			close = append(close, name)
		}
	}
	sort.Strings(close)
	if len(close) > 5 {
		close = close[:5]
	}
	if len(close) > 0 {
		return 0, fmt.Errorf("steam: unknown tag %q; close matches: %s", nameOrID, strings.Join(close, ", "))
	}
	return 0, fmt.Errorf("steam: unknown tag %q", nameOrID)
}

// tagNames fetches and caches the store tag dictionary for this client's
// language. One request per process: the dictionary is large and stable.
func (c *Client) tagNames(ctx context.Context) (map[int]string, error) {
	c.tagMu.Lock()
	if c.tagCache != nil {
		cached := c.tagCache
		c.tagMu.Unlock()
		return cached, nil
	}
	c.tagMu.Unlock()

	endpoint := c.apiBase() + getTagListPath + "?language=" + url.QueryEscape(c.languageName())
	body, err := c.doer.get(ctx, endpoint, c.headers())
	if err != nil {
		return nil, err
	}
	var wire storeTagListWire
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("steam: parsing GetTagList response: %w", err)
	}
	names := make(map[int]string, len(wire.Response.Tags))
	for _, t := range wire.Response.Tags {
		names[t.TagID] = t.Name
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("steam: store tag list came back empty")
	}
	c.tagMu.Lock()
	c.tagCache = names
	c.tagMu.Unlock()
	return names, nil
}

// AttachTagNames fills Tag.Name on items from the cached tag dictionary. It is
// the exported seam for callers (such as the demos --title path) whose endpoint
// does not attach names itself. The dictionary is shared with ResolveTag, so a
// caller that already resolved a --tag pays no extra request.
func (c *Client) AttachTagNames(ctx context.Context, items []StoreItem) {
	c.attachTagNames(ctx, items)
}

// attachTagNames fills Tag.Name from the tag dictionary. A failure is never
// fatal: ids stay usable and the record still ships.
func (c *Client) attachTagNames(ctx context.Context, items []StoreItem) {
	if len(items) == 0 {
		return
	}
	names, err := c.tagNames(ctx)
	if err != nil {
		return
	}
	for i := range items {
		applyTagNames(items[i].Tags, names)
	}
}

// applyTagNames copies names onto tags by id; ids absent from the dictionary
// keep an empty Name and stay usable.
func applyTagNames(tags []Tag, names map[int]string) {
	for j := range tags {
		if name, ok := names[tags[j].ID]; ok {
			tags[j].Name = name
		}
	}
}

// ---------------------------------------------------------------------------
// Store-aware identity resolution
// ---------------------------------------------------------------------------

// splitTitleYear separates a trailing "(YYYY)" from a title, as RAWG writes
// remake years. No suffix yields year 0.
func splitTitleYear(title string) (string, int) {
	trimmed := strings.TrimSpace(title)
	if !strings.HasSuffix(trimmed, ")") {
		return trimmed, 0
	}
	open := strings.LastIndex(trimmed, "(")
	if open < 0 {
		return trimmed, 0
	}
	digits := trimmed[open+1 : len(trimmed)-1]
	if len(digits) != 4 {
		return trimmed, 0
	}
	year, err := strconv.Atoi(digits)
	if err != nil || year < 1900 || year > 2099 {
		return trimmed, 0
	}
	return strings.TrimSpace(trimmed[:open]), year
}

// ResolveAppIDWithHint maps a title (optionally pinned to a release year) to one
// appid using the plural store search.
//
// It tolerates the "(YYYY)" suffix RAWG appends to remakes (RAWG calls the 2016
// reboot "DOOM (2016)"; the Steam store calls it plain "DOOM"), restricts the
// candidate set to games, and breaks name collisions by release year. When the
// candidates stay indistinguishable it returns ErrAmbiguousApp rather than
// guessing, because presenting another edition's reviews and price as the
// requested game's is worse than reporting no Steam data.
//
// A supplied year is binding: whether passed explicitly or parsed from a
// "(YYYY)" suffix, a candidate whose release year falls outside the one-year
// window is never accepted, even when it is the only name match.
func (c *Client) ResolveAppIDWithHint(ctx context.Context, title string, year int) (int64, error) {
	base, suffixYear := splitTitleYear(title)
	if year == 0 {
		year = suffixYear
	}
	if base == "" {
		base = strings.TrimSpace(title)
	}
	items, err := c.search(ctx, base, []AppType{AppTypeGame}, MaxPageSize)
	if err != nil {
		return 0, err
	}
	matches := exactMatches(items, base)
	if len(matches) == 0 {
		return 0, fmt.Errorf("%w: %q (no exact store match among %d games)", ErrAppNotFound, title, len(items))
	}
	if year > 0 {
		if picked, ok := pickByYear(matches, year); ok {
			return picked.AppID, nil
		}
		if !anyWithinAYear(matches, year) {
			return 0, fmt.Errorf("%w: %q (no exact store match released within a year of %d; candidates: %s)", ErrAppNotFound, title, year, describeItems(matches))
		}
		return 0, ambiguousError(title, matches)
	}
	if len(matches) == 1 {
		return matches[0].AppID, nil
	}
	return 0, ambiguousError(title, matches)
}

// exactMatches filters items whose name folds to the requested title.
func exactMatches(items []StoreItem, title string) []StoreItem {
	want := normalizeName(title)
	var out []StoreItem
	for _, item := range items {
		if normalizeName(item.Name) == want {
			out = append(out, item)
		}
	}
	return out
}

// pickByYear prefers an exact release-year match and accepts a single candidate
// within one year. Anything else stays ambiguous.
func pickByYear(matches []StoreItem, year int) (StoreItem, bool) {
	var exact []StoreItem
	var near []StoreItem
	for _, m := range matches {
		if m.ReleasesInYear(year) {
			exact = append(exact, m)
			continue
		}
		if withinAYear(m, year) {
			near = append(near, m)
		}
	}
	if len(exact) == 1 {
		return exact[0], true
	}
	if len(exact) == 0 && len(near) == 1 {
		return near[0], true
	}
	return StoreItem{}, false
}

// withinAYear reports whether an item's release year is within one year of the
// requested year, which absorbs store/release date drift.
func withinAYear(item StoreItem, year int) bool {
	if len(item.ReleaseDate) < 4 {
		return false
	}
	y, err := strconv.Atoi(item.ReleaseDate[:4])
	if err != nil {
		return false
	}
	delta := y - year
	return delta >= -1 && delta <= 1
}

// anyWithinAYear reports whether any candidate falls in the requested year or
// within one year of it. When pickByYear fails, this distinguishes genuine
// ambiguity from a year with no plausible match.
func anyWithinAYear(matches []StoreItem, year int) bool {
	for _, m := range matches {
		if m.ReleasesInYear(year) || withinAYear(m, year) {
			return true
		}
	}
	return false
}

func ambiguousError(title string, matches []StoreItem) error {
	return fmt.Errorf("%w: %q — candidates: %s (pin with --year or pass an appid directly)",
		ErrAmbiguousApp, title, describeItems(matches))
}

func describeItems(items []StoreItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		date := item.ReleaseDate
		if date == "" {
			date = "no release date"
		}
		parts = append(parts, fmt.Sprintf("%d %s (%s)", item.AppID, item.Name, date))
	}
	return strings.Join(parts, "; ")
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func convertItems(wires []storeItemWire) []StoreItem {
	if len(wires) == 0 {
		return nil
	}
	items := make([]StoreItem, 0, len(wires))
	for _, w := range wires {
		items = append(items, w.toStoreItem())
	}
	return items
}

func (w storeItemWire) toStoreItem() StoreItem {
	appID := w.AppID
	if appID == 0 {
		appID = w.ID
	}
	item := StoreItem{
		AppID:            appID,
		Name:             w.Name,
		Type:             appTypeFromService(w.Type),
		IsFree:           w.IsFree,
		ComingSoon:       w.Release.ComingSoon || w.Release.IsComingSoon,
		ReleaseDate:      unixDate(w.Release.SteamReleaseDate),
		Publishers:       namedList(w.BasicInfo.Publishers),
		Developers:       namedList(w.BasicInfo.Developers),
		Franchises:       namedList(w.BasicInfo.Franchises),
		ShortDescription: w.BasicInfo.ShortDescription,
		ParentAppID:      w.RelatedItems.ParentAppID,
		DemoAppIDs:       demoAppIDs(w.RelatedItems),
		Tags:             tagsFrom(w),
		Platforms: PlatformSupport{
			Windows:         w.Platforms.Windows,
			Mac:             w.Platforms.Mac,
			Linux:           w.Platforms.Linux,
			SteamDeckCompat: w.Platforms.SteamDeck,
		},
	}
	item.OriginalReleaseDate = unixDate(w.Release.OriginalSteamReleaseDate)
	item.EarlyAccess = w.Release.IsEarlyAccess || hasTag(item.Tags, earlyAccessTagID)
	if w.StoreURLPath != "" {
		item.StoreURL = DefaultBaseURL + "/" + strings.TrimLeft(w.StoreURLPath, "/")
	}
	item.HeaderImageURL = headerImageURL(w.Assets)
	if p := w.BestPurchaseOption; p != nil {
		item.Price = &StorePrice{
			FinalCents:        atoiOrZero(p.FinalPriceInCents),
			OriginalCents:     atoiOrZero(p.OriginalPriceInCents),
			Formatted:         p.FormattedFinalPrice,
			OriginalFormatted: p.FormattedOriginalPrice,
			DiscountPercent:   p.DiscountPct,
		}
	}
	return item
}

func namedList(in []storeNamedWire) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, n := range in {
		if strings.TrimSpace(n.Name) != "" {
			out = append(out, n.Name)
		}
	}
	return out
}

// tagsFrom merges the weighted tags array with the bare tagids array, keeping
// the weighted entry when both carry the same id.
func tagsFrom(w storeItemWire) []Tag {
	seen := map[int]bool{}
	var out []Tag
	for _, t := range w.Tags {
		if t.TagID == 0 || seen[t.TagID] {
			continue
		}
		seen[t.TagID] = true
		out = append(out, Tag{ID: t.TagID, Weight: t.Weight})
	}
	for _, id := range w.TagIDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Tag{ID: id})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func demoAppIDs(related storeRelatedWire) []int64 {
	if len(related.DemoAppID) == 0 && len(related.Demos) == 0 {
		return nil
	}
	seen := map[int64]bool{}
	var out []int64
	for _, id := range related.DemoAppID {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, d := range related.Demos {
		if d.AppID == 0 || seen[d.AppID] {
			continue
		}
		seen[d.AppID] = true
		out = append(out, d.AppID)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func hasTag(tags []Tag, id int) bool {
	for _, t := range tags {
		if t.ID == id {
			return true
		}
	}
	return false
}

// headerImageURL expands the GetItems asset template into a capsule URL.
func headerImageURL(a storeAssetsWire) string {
	if a.AssetURLFormat == "" || a.Header == "" {
		return ""
	}
	return storeItemAssetBase + strings.ReplaceAll(a.AssetURLFormat, "${FILENAME}", a.Header)
}

// unixDate renders a service unix timestamp as YYYY-MM-DD (UTC); 0 is empty.
func unixDate(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02")
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func joinIDs(ids []int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

// chunkIDs splits ids into consecutive slices of at most size entries.
func chunkIDs(ids []int64, size int) [][]int64 {
	if size <= 0 {
		size = MaxItemsPerRequest
	}
	var chunks [][]int64
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[start:end])
	}
	return chunks
}

// dedupePositiveIDs drops non-positive appids and duplicates, preserving order.
func dedupePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// ---------------------------------------------------------------------------
// Transport
// ---------------------------------------------------------------------------

// storeContext is the locale context every store service call carries.
func storeContext(c *Client) map[string]string {
	return map[string]string{
		"language":     c.languageName(),
		"country_code": c.countryCode(),
	}
}

// storeDataRequest asks for every field StoreItem models.
func storeDataRequest() map[string]any {
	return map[string]any{
		"include_basic_info":    true,
		"include_release":       true,
		"include_platforms":     true,
		"include_assets":        true,
		"include_related_items": true,
		"include_tag_count":     tagCount,
	}
}

// languageName is the store locale, defaulting when the client was built with
// an empty config.
func (c *Client) languageName() string {
	if c != nil && strings.TrimSpace(c.Language) != "" {
		return c.Language
	}
	return DefaultLanguage
}

// countryCode is the ISO 3166-1 alpha-2 storefront region.
func (c *Client) countryCode() string {
	if c != nil && strings.TrimSpace(c.Country) != "" {
		return strings.ToUpper(strings.TrimSpace(c.Country))
	}
	return DefaultCountry
}

// apiBase is the store-services host, defaulting when unset so a Client built
// as a literal still works.
func (c *Client) apiBase() string {
	if c != nil && strings.TrimSpace(c.APIBaseURL) != "" {
		return strings.TrimRight(c.APIBaseURL, "/")
	}
	return DefaultAPIBaseURL
}

// serviceGet issues a keyless store-service GET. The request payload travels
// URL-encoded in the input_json query parameter, which is how these services
// accept structured arguments over GET, and the reply is the {"response":...}
// envelope. Every call goes through adaptiveDoer, so it obeys the same rate
// limit and 429/5xx retry policy as the storefront endpoints.
func (c *Client) serviceGet(ctx context.Context, path string, payload any, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("steam: encoding %s request: %w", path, err)
	}
	endpoint := c.apiBase() + path + "?input_json=" + url.QueryEscape(string(raw))
	body, err := c.doer.get(ctx, endpoint, c.headers())
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("steam: parsing %s response: %w", path, err)
	}
	return nil
}
