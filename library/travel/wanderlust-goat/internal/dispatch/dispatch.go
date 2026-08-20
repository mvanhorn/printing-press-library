package dispatch

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/closedsignal"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/criteria"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/googleplaces"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/regions"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/walking"
)

// Plan describes the user's intent for one near/goat invocation.
type Plan struct {
	Anchor        string
	Resolved      AnchorResolution
	Criteria      string
	Identity      string
	RadiusMinutes float64
	SeedLimit     int
	IncludedTypes []string // Google Places type filter (cafe, restaurant, etc.)
	UseLLM        bool
}

// Candidate is one Stage-1 seed enriched with Stage-2 evidence and
// Stage-3 score. Exposed verbatim by the `why` command.
type Candidate struct {
	Place         googleplaces.Place           `json:"place"`
	WalkingMin    float64                      `json:"walking_minutes"`
	StageHits     map[string][]sourcetypes.Hit `json:"hits"`
	StubReasons   map[string]string            `json:"stubbed_sources,omitempty"`
	SourceErrors  map[string]string            `json:"source_errors,omitempty"`
	ClosedSignals []closedsignal.Verdict       `json:"closed_signals,omitempty"`
	Score         Score                        `json:"score"`
}

// Score is the trust-weighted breakdown for one candidate. Components
// follow the brief: Google base + LocaleBoost (+0.10 in country) +
// NotabilityBoost (+0.05 if Wikipedia) + RedditBoost (+0.05 if 10+/3+).
type Score struct {
	Total           float64 `json:"total"`
	GoogleBase      float64 `json:"google_base"`
	LocaleBoost     float64 `json:"locale_boost"`
	NotabilityBoost float64 `json:"notability_boost"`
	RedditBoost     float64 `json:"reddit_boost"`
	CriteriaMatch   float64 `json:"criteria_match"`
}

// Result is one ranked output row.
type Result struct {
	Name           string                       `json:"name"`
	Lat            float64                      `json:"lat"`
	Lng            float64                      `json:"lng"`
	Address        string                       `json:"address"`
	WalkingMinutes float64                      `json:"walking_minutes"`
	Score          Score                        `json:"score"`
	Sources        []string                     `json:"sources"`
	Evidence       []string                     `json:"evidence"`
	Why            string                       `json:"why"`
	BusinessStatus string                       `json:"business_status,omitempty"`
	GoogleMapsURI  string                       `json:"google_maps_uri,omitempty"`
	Hits           map[string][]sourcetypes.Hit `json:"hits,omitempty"`
}

// Trace captures per-source diagnostic info for the `coverage` and `why`
// commands.
type Trace struct {
	Region       regions.Region
	SeedCount    int
	StageHits    map[string]int
	StubsSkipped map[string]string // slug → stub reason
	Errors       map[string]string // slug → error message
}

// SeedClient is the interface Stage-1 must satisfy. The real path uses
// internal/googleplaces; tests inject a fake.
type SeedClient interface {
	NearbySearch(ctx context.Context, lat, lng, radiusMeters float64, includedTypes []string, maxResults int, languageCode string) ([]googleplaces.Place, error)
}

// Dispatcher runs the two-stage funnel.
type Dispatcher struct {
	Seed     SeedClient
	Registry *Registry
}

// New returns a Dispatcher wired to the default seed (Google Places) and
// the default registry. Returns ErrMissingAPIKey when no GOOGLE_PLACES_API_KEY
// is set; callers should surface this as exit code 4 (auth).
func New() (*Dispatcher, error) {
	g, err := googleplaces.NewClient()
	if err != nil {
		return nil, err
	}
	return &Dispatcher{Seed: g, Registry: DefaultRegistry()}, nil
}

// NewWithSeed lets tests inject a fake SeedClient.
func NewWithSeed(seed SeedClient, reg *Registry) *Dispatcher {
	if reg == nil {
		reg = DefaultRegistry()
	}
	return &Dispatcher{Seed: seed, Registry: reg}
}

// Run executes Stage 1 → Stage 2 → Stage 3 and returns a ranked slice
// plus a trace. Top N is determined by the caller; Run returns every
// surviving candidate sorted by score descending.
func (d *Dispatcher) Run(ctx context.Context, p Plan) ([]Result, Trace, error) {
	tr := Trace{
		StageHits:    map[string]int{},
		StubsSkipped: map[string]string{},
		Errors:       map[string]string{},
	}
	if p.RadiusMinutes <= 0 {
		p.RadiusMinutes = 15
	}
	if p.SeedLimit <= 0 || p.SeedLimit > 20 {
		p.SeedLimit = 20
	}

	// Stage 1: seed.
	cands, err := d.stage1Seed(ctx, p)
	if err != nil {
		return nil, tr, err
	}
	tr.SeedCount = len(cands)
	if len(cands) == 0 {
		return nil, tr, nil
	}

	// Stage 2: deep research.
	region := regions.Lookup(p.Resolved.Country)
	tr.Region = region
	d.stage2Research(ctx, p, region, cands, &tr)

	// Apply closed-signal kill-gate. Drop verdicts where any source
	// said Closed; keep Temporary as warnings.
	kept := make([]*Candidate, 0, len(cands))
	for _, c := range cands {
		v := closedsignal.Combine(c.ClosedSignals)
		if v.Closed {
			continue
		}
		kept = append(kept, c)
	}

	// Stage 3: rank.
	d.stage3Rank(p, kept)

	// Build []Result.
	out := make([]Result, 0, len(kept))
	for _, c := range kept {
		out = append(out, candidateToResult(c))
	}
	return out, tr, nil
}

func (d *Dispatcher) stage1Seed(ctx context.Context, p Plan) ([]*Candidate, error) {
	radiusMeters := walking.MetersFromMinutes(p.RadiusMinutes)
	if radiusMeters < 100 {
		radiusMeters = 100
	}
	places, err := d.Seed.NearbySearch(ctx, p.Resolved.Lat, p.Resolved.Lng, radiusMeters, p.IncludedTypes, p.SeedLimit, "en")
	if err != nil {
		return nil, fmt.Errorf("stage1: %w", err)
	}
	out := make([]*Candidate, 0, len(places))
	anchor := walking.LatLng{Lat: p.Resolved.Lat, Lng: p.Resolved.Lng}
	for _, pl := range places {
		c := &Candidate{
			Place:        pl,
			WalkingMin:   walking.MinutesBetween(anchor, walking.LatLng{Lat: pl.Lat, Lng: pl.Lng}),
			StageHits:    map[string][]sourcetypes.Hit{},
			StubReasons:  map[string]string{},
			SourceErrors: map[string]string{},
		}
		if v := closedsignal.CheckGoogleBusinessStatus(pl.BusinessStatus); v.Closed || v.Temporary {
			c.ClosedSignals = append(c.ClosedSignals, v)
		}
		out = append(out, c)
	}
	return out, nil
}

func (d *Dispatcher) stage2Research(ctx context.Context, p Plan, region regions.Region, cands []*Candidate, tr *Trace) {
	if len(region.LocalReviewSites) == 0 {
		return
	}
	city := p.Resolved.City
	if city == "" {
		city = extractCity(p.Resolved.Display)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex // protects cands' shared maps
	for _, c := range cands {
		c := c
		for _, slug := range region.LocalReviewSites {
			cli := d.Registry.Get(slug)
			if cli == nil {
				mu.Lock()
				c.SourceErrors[slug] = "client not registered"
				mu.Unlock()
				continue
			}
			wg.Add(1)
			go func(slug string, cli sourcetypes.Client) {
				defer wg.Done()
				hits, err := cli.LookupByName(ctx, c.Place.DisplayName, city, 3)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					if errors.Is(err, sourcetypes.ErrNotImplemented) {
						c.StubReasons[slug] = sourcetypes.StubReason(cli)
						tr.StubsSkipped[slug] = sourcetypes.StubReason(cli)
					} else {
						c.SourceErrors[slug] = err.Error()
						tr.Errors[slug] = err.Error()
					}
					return
				}
				if len(hits) == 0 {
					return
				}
				c.StageHits[slug] = hits
				tr.StageHits[slug] = tr.StageHits[slug] + len(hits)
				// Run CheckClosed on the first hit if the client supports it.
				if checker, ok := cli.(closedChecker); ok {
					v := checker.CheckClosed(ctx, hits[0])
					if v.Closed || v.Temporary {
						c.ClosedSignals = append(c.ClosedSignals, v)
					}
				}
			}(slug, cli)
		}
	}
	wg.Wait()
}

// closedChecker is implemented by every real Stage-2 source.
type closedChecker interface {
	CheckClosed(ctx context.Context, hit sourcetypes.Hit) closedsignal.Verdict
}

func (d *Dispatcher) stage3Rank(p Plan, cands []*Candidate) {
	region := regions.Lookup(p.Resolved.Country)
	for _, c := range cands {
		s := Score{}
		// Google base: rating × log(1 + count/100), bounded 0..1.
		if c.Place.Rating > 0 {
			s.GoogleBase = (c.Place.Rating / 5.0) * 0.6
			if c.Place.UserRatingCount > 0 {
				// Add up to +0.4 for review volume (saturating curve).
				s.GoogleBase += min1(float64(c.Place.UserRatingCount)/500.0) * 0.4
			}
		}
		// Locale boost: +0.10 when at least one in-country regional source
		// produced a hit.
		for _, slug := range region.LocalReviewSites {
			if hits, ok := c.StageHits[slug]; ok && len(hits) > 0 {
				s.LocaleBoost = 0.10
				break
			}
		}
		// Criteria match: keyword overlap of criteria string with title +
		// snippet of any hit.
		if p.Criteria != "" {
			s.CriteriaMatch = scoreCriteriaMatch(p.Criteria, c)
		}
		// Notability + Reddit are wired by the cli/ commands when they
		// have those signals; the dispatcher never burns paid budget on
		// unsolicited Wikipedia/Reddit lookups. The cli wrapper sets these
		// fields directly when running with --rich.
		s.Total = s.GoogleBase + s.LocaleBoost + s.NotabilityBoost + s.RedditBoost + 0.5*s.CriteriaMatch
		c.Score = s
	}
	sort.SliceStable(cands, func(i, j int) bool {
		return cands[i].Score.Total > cands[j].Score.Total
	})
}

func scoreCriteriaMatch(crit string, c *Candidate) float64 {
	if c == nil {
		return 0
	}
	m := criteria.Parse(crit)
	keys := append([]string{}, m.RedditKW...)
	keys = append(keys, m.QualityWords...)
	// Tokenize the raw criteria as a fallback for phrases the table didn't
	// cover — split on whitespace, drop trivial tokens.
	for _, w := range strings.Fields(strings.ToLower(crit)) {
		if len(w) > 3 {
			keys = append(keys, w)
		}
	}
	if len(keys) == 0 {
		return 0
	}
	body := strings.ToLower(c.Place.DisplayName) + " " +
		strings.ToLower(c.Place.Address) + " " +
		strings.ToLower(c.Place.PrimaryType) + " " +
		strings.ToLower(strings.Join(c.Place.Types, " "))
	for _, hits := range c.StageHits {
		for _, h := range hits {
			body += " " + strings.ToLower(h.Title) + " " + strings.ToLower(h.Snippet)
		}
	}
	uniq := map[string]bool{}
	for _, k := range keys {
		uniq[k] = true
	}
	hits := 0
	for k := range uniq {
		if strings.Contains(body, k) {
			hits++
		}
	}
	if len(uniq) == 0 {
		return 0
	}
	return float64(hits) / float64(len(uniq))
}

// extractCity is a heuristic fallback for when the resolved Anchor has no
// City field. Walks comma-split parts from the end, skipping postcodes,
// country names, and admin-region suffixes (Prefecture, Region, Ward,
// Subprefecture, etc.) — the part that looked like the city in v1 was
// often the postcode.
func extractCity(display string) string {
	parts := strings.Split(display, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p == "" {
			continue
		}
		low := strings.ToLower(p)
		if extractCityCountries[low] {
			continue
		}
		if extractCityPostcodeRe.MatchString(p) {
			continue
		}
		skip := false
		for _, suf := range extractCitySkipSuffixes {
			if low == suf || strings.HasSuffix(low, " "+suf) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		return p
	}
	return strings.TrimSpace(display)
}

var extractCityPostcodeRe = regexp.MustCompile(`^[〒]?\s*[\dA-Z]{2,5}([- ]?[\dA-Z]{0,4})?$`)

var extractCitySkipSuffixes = []string{
	"region", "prefecture", "subprefecture", "province", "state",
	"district", "county", "ward", "republic", "territory", "oblast",
}

var extractCityCountries = map[string]bool{
	"japan": true, "france": true, "korea": true, "south korea": true,
	"germany": true, "italy": true, "spain": true, "united kingdom": true,
	"uk": true, "england": true, "scotland": true, "wales": true,
	"ireland": true, "china": true, "switzerland": true, "austria": true,
	"australia": true, "canada": true, "mexico": true, "brazil": true,
	"united states": true, "usa": true, "netherlands": true, "belgium": true,
	"portugal": true, "greece": true, "thailand": true, "vietnam": true,
	"singapore": true, "malaysia": true, "indonesia": true, "philippines": true,
	"new zealand": true, "norway": true, "sweden": true, "denmark": true,
	"finland": true, "iceland": true, "poland": true, "czechia": true,
}

func min1(x float64) float64 {
	if x > 1 {
		return 1
	}
	return x
}

func candidateToResult(c *Candidate) Result {
	src := make([]string, 0, len(c.StageHits)+1)
	src = append(src, "google.places")
	for s := range c.StageHits {
		src = append(src, s)
	}
	sort.Strings(src)

	var ev []string
	for _, hits := range c.StageHits {
		for _, h := range hits {
			ev = append(ev, fmt.Sprintf("%s: %s", h.Source, h.Title))
		}
	}
	r := Result{
		Name:           c.Place.DisplayName,
		Lat:            c.Place.Lat,
		Lng:            c.Place.Lng,
		Address:        c.Place.Address,
		WalkingMinutes: roundTo(c.WalkingMin, 1),
		Score:          c.Score,
		Sources:        src,
		Evidence:       ev,
		BusinessStatus: c.Place.BusinessStatus,
		GoogleMapsURI:  c.Place.GoogleMapsURI,
		Hits:           c.StageHits,
		Why:            buildWhy(c),
	}
	return r
}

func buildWhy(c *Candidate) string {
	parts := []string{}
	if c.Place.Rating > 0 {
		parts = append(parts, fmt.Sprintf("Google %.1f★ (%d reviews)", c.Place.Rating, c.Place.UserRatingCount))
	}
	for slug, hits := range c.StageHits {
		if len(hits) > 0 {
			parts = append(parts, fmt.Sprintf("%s match", slug))
		}
	}
	if c.Score.LocaleBoost > 0 {
		parts = append(parts, "in-country boost")
	}
	if v := closedsignal.Combine(c.ClosedSignals); v.Temporary {
		parts = append(parts, "WARNING temporarily closed")
	}
	if len(parts) == 0 {
		return "Google Places seed; no Stage-2 enrichment"
	}
	return strings.Join(parts, "; ")
}

func roundTo(v float64, digits int) float64 {
	mult := 1.0
	for i := 0; i < digits; i++ {
		mult *= 10
	}
	return float64(int(v*mult+0.5)) / mult
}
