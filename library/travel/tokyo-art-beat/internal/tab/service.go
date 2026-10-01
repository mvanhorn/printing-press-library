package tab

import (
	"context"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const eventSelect = "sys.id,sys.updatedAt,fields.slug,fields.venue,fields.eventName,fields.artists,fields.scheduleStartsOn,fields.scheduleEndsOn,fields.scheduleEndDateUnfix,fields.permanentShow,fields.categories"
const eventDetailSelect = eventSelect + ",fields.fee,fields.coupon,fields.reservation,fields.description,fields.showsWebpage,fields.openingHoursOpens,fields.openingHoursCloses,fields.closedDays,fields.hideClosedDays,fields.scheduleSpecialCases,fields.additionalInfoOnOpeningHoursdays,fields.hideScheduleSpecialCases"
const venueSelect = "sys.id,sys.updatedAt,fields.slug,fields.fullName,fields.address,fields.geoInfo,fields.localArea,fields.venueType,fields.venueStatus"
const venueDetailSelect = venueSelect + ",fields.homePage,fields.admissionFee,fields.howtoAccess,fields.description,fields.openingHoursOpens,fields.openingHoursCloses,fields.closedDays,fields.hideClosedDays,fields.scheduleSpecialCases,fields.additionalInfoOnOpeningHoursdays,fields.hideScheduleSpecialCases"

type Search struct {
	Query        string `json:"query,omitempty"`
	Artist       string `json:"artist,omitempty"`
	Area         string `json:"area,omitempty"`
	Category     string `json:"category,omitempty"`
	Venue        string `json:"venue,omitempty"`
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
	Relation     string `json:"relation,omitempty"`
	Status       string `json:"status,omitempty"`
	Sort         string `json:"sort,omitempty"`
	Limit        int    `json:"limit"`
	Offset       int    `json:"offset"`
	MaxScanPages int    `json:"max_scan_pages,omitempty"`
}
type Pagination struct {
	Offset     int  `json:"offset"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	Returned   int  `json:"returned"`
	NextOffset *int `json:"next_offset"`
}
type Meta struct {
	SchemaVersion string      `json:"schema_version"`
	Coverage      any         `json:"coverage"`
	Source        string      `json:"source"`
	Timezone      string      `json:"timezone"`
	AsOfDate      string      `json:"as_of_date"`
	Partial       bool        `json:"partial"`
	Truncated     bool        `json:"truncated"`
	Pagination    *Pagination `json:"pagination,omitempty"`
	Ranking       string      `json:"ranking,omitempty"`
	Scope         any         `json:"scope,omitempty"`
	Stats         Stats       `json:"stats"`
	Freshness     []Fetch     `json:"freshness"`
	Warnings      []string    `json:"warnings"`
}
type Result struct {
	Meta    Meta     `json:"meta"`
	Results any      `json:"results"`
	Errors  []*Error `json:"errors"`
}

func (c *Client) result(items any, p *Pagination, errors []*Error, warnings []string, ranking string) Result {
	if errors == nil {
		errors = []*Error{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	source := "live"
	if c.Stats.Requests == 0 && c.Stats.CacheHits > 0 {
		source = "cache"
	} else if c.Stats.CacheHits > 0 {
		source = "mixed"
	}
	return Result{Meta: Meta{SchemaVersion: "1", Coverage: map[string]any{"languages": "public en-US and ja-JP fields; omissions are null, no translations inferred", "membership": "public coupon indicators only; redemption and member-only data excluded", "ticket_inventory": "unknown; listings do not establish availability", "source": "undocumented published website feed", "date_filters": "bounded date spans; missing boundaries may be excluded"}, Source: source, Timezone: "Asia/Tokyo", AsOfDate: Today(), Partial: len(errors) > 0, Truncated: p != nil && p.NextOffset != nil, Pagination: p, Ranking: ranking, Stats: c.Summary(), Freshness: c.Fetches, Warnings: warnings}, Results: items, Errors: errors}
}
func qbase(typ, selectFields string, limit, offset, include int) url.Values {
	q := url.Values{"content_type": {typ}, "locale": {"*"}, "limit": {strconv.Itoa(limit)}, "skip": {strconv.Itoa(offset)}, "include": {strconv.Itoa(include)}}
	if selectFields != "" {
		q.Set("select", selectFields)
	}
	return q
}
func page(f Feed, returned int) *Pagination {
	p := &Pagination{f.Skip, f.Limit, f.Total, returned, nil}
	n := f.Skip + len(f.Items)
	if n < f.Total && len(f.Items) > 0 {
		p.NextOffset = &n
	}
	return p
}
func ValidateSearch(s Search) error {
	if s.MaxScanPages < 0 || s.MaxScanPages > 5 {
		return Fail("invalid_scan_cap", "--max-scan-pages must be 1..5", 2)
	}
	if s.Limit < 1 || s.Limit > 50 {
		return Fail("invalid_limit", "--limit must be between 1 and 50", 2)
	}
	if s.Offset < 0 || s.Offset > 100000 {
		return Fail("invalid_offset", "--offset must be between 0 and 100000", 2)
	}
	if s.From != "" {
		if _, e := ParseDate(s.From); e != nil {
			return e
		}
	}
	if s.To != "" {
		if _, e := ParseDate(s.To); e != nil {
			return e
		}
	}
	if s.To != "" && s.From == "" {
		return Fail("invalid_window", "--to requires --from", 2)
	}
	if s.To != "" && s.To < s.From {
		return Fail("invalid_window", "--to must be on or after --from", 2)
	}
	if s.Relation != "" && s.Relation != "overlap" && s.Relation != "starts" && s.Relation != "ends" {
		return Fail("invalid_relation", "--relation must be overlap, starts, or ends", 2)
	}
	if s.Relation != "" && s.Relation != "overlap" && s.From == "" {
		return Fail("invalid_window", "--relation starts/ends requires --from", 2)
	}
	if s.Status != "" && s.Status != "active" && s.Status != "upcoming" && s.Status != "past" && s.Status != "all" {
		return Fail("invalid_status", "--status must be active, upcoming, past or all", 2)
	}
	if s.From != "" && s.Status != "" && s.Status != "all" {
		return Fail("invalid_window", "Use a trip window or --status, rather than combining relative status with dates", 2)
	}
	if s.Sort != "" && s.Sort != "starts" && s.Sort != "ends" && s.Sort != "updated" {
		return Fail("invalid_sort", "--sort must be starts, ends or updated; popularity is not exposed", 2)
	}
	return nil
}
func dateQuery(q url.Values, s Search) {
	today := Today()
	if s.From != "" {
		to := s.To
		if to == "" {
			to = s.From
		}
		switch s.Relation {
		case "starts":
			q.Set("fields.scheduleStartsOn[gte]", s.From)
			q.Set("fields.scheduleStartsOn[lte]", to)
		case "ends":
			q.Set("fields.scheduleEndsOn[gte]", s.From)
			q.Set("fields.scheduleEndsOn[lte]", to)
		default:
			q.Set("fields.scheduleStartsOn[lte]", to)
			q.Set("fields.scheduleEndsOn[gte]", s.From)
		}
	} else {
		switch s.Status {
		case "all":
		case "past":
			q.Set("fields.scheduleEndsOn[lt]", today)
		case "active":
			q.Set("fields.scheduleStartsOn[lte]", today)
			q.Set("fields.scheduleEndsOn[gte]", today)
		case "upcoming":
			q.Set("fields.scheduleStartsOn[gt]", today)
		default:
			q.Set("fields.scheduleEndsOn[gte]", today)
		}
	}
	switch s.Sort {
	case "ends":
		q.Set("order", "fields.scheduleEndsOn")
	case "updated":
		q.Set("order", "-sys.updatedAt")
	default:
		q.Set("order", "fields.scheduleStartsOn")
	}
}
func (c *Client) Catalog(ctx context.Context, kind, query string, limit, offset int) (Result, error) {
	typ := map[string]string{"areas": "localArea", "categories": "eventCategory", "types": "venueType"}[kind]
	if typ == "" {
		return Result{}, Fail("invalid_catalog", "Catalog must be areas, categories or types", 2)
	}
	if limit < 1 || limit > 300 || offset < 0 {
		return Result{}, Fail("invalid_pagination", "Catalog --limit must be 1..300 and --offset nonnegative", 2)
	}
	q := qbase(typ, "sys.id,fields.name", limit, offset, 0)
	q.Set("order", "sys.id")
	if query != "" {
		q.Set("query", query)
	}
	f, err := c.Query(ctx, q)
	if err != nil {
		return Result{}, err
	}
	rs := []Ref{}
	for _, e := range f.Items {
		rs = append(rs, ref(e))
	}
	return c.result(rs, page(f, len(rs)), nil, nil, "source ID ascending"), nil
}
func (c *Client) catalogEntries(ctx context.Context, typ string) (map[string]Entry, error) {
	q := qbase(typ, "sys.id,fields.name", 300, 0, 0)
	f, err := c.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	if f.Total > len(f.Items) {
		return nil, Fail("catalog_truncated", "Reference catalog exceeds bounded lookup size; use a source ID", 5)
	}
	return index(f.Items), nil
}
func resolveName(input string, idx map[string]Entry) (string, error) {
	if _, ok := idx[input]; ok {
		return input, nil
	}
	exact, partial := []Ref{}, []Ref{}
	needle := strings.ToLower(strings.TrimSpace(input))
	for _, e := range idx {
		r := ref(e)
		isExact, isPartial := false, false
		for _, p := range []*string{r.Name.EN, r.Name.JA} {
			if p == nil {
				continue
			}
			name := strings.ToLower(*p)
			isExact = isExact || name == needle
			isPartial = isPartial || strings.Contains(name, needle)
		}
		if isExact {
			exact = append(exact, r)
		} else if isPartial {
			partial = append(partial, r)
		}
	}
	found := exact
	if len(found) == 0 {
		found = partial
	}
	sort.Slice(found, func(i, j int) bool { return found[i].ID < found[j].ID })
	if len(found) == 1 {
		return found[0].ID, nil
	}
	e := Fail("unknown_filter", "No catalog name matches "+input+"; use catalogs or a source ID", 2)
	if len(found) > 1 {
		e.Code = "ambiguous_filter"
		e.Message = "Ambiguous name " + input + "; use an exact name or source ID"
		if len(found) > 10 {
			found = found[:10]
		}
		e.Candidates = found
	}
	return "", e
}
func (c *Client) eventQuery(ctx context.Context, s Search) (url.Values, error) {
	if err := ValidateSearch(s); err != nil {
		return nil, err
	}
	q := qbase("event", eventSelect, s.Limit, s.Offset, 0)
	dateQuery(q, s)
	if s.Query != "" {
		q.Set("query", s.Query)
	}
	if s.Artist != "" {
		if s.Query == "" {
			q.Set("query", s.Artist)
		}
	}
	if s.Area != "" {
		idx, err := c.catalogEntries(ctx, "localArea")
		if err != nil {
			return nil, err
		}
		id, err := resolveName(s.Area, idx)
		if err != nil {
			return nil, err
		}
		q.Set("fields.venue.sys.contentType.sys.id", "venue")
		q.Set("fields.venue.fields.localArea.sys.id", id)
	}
	if s.Category != "" {
		idx, err := c.catalogEntries(ctx, "eventCategory")
		if err != nil {
			return nil, err
		}
		id, err := resolveName(s.Category, idx)
		if err != nil {
			return nil, err
		}
		q.Set("fields.categories.sys.id", id)
	}
	if s.Venue != "" {
		id, err := c.resolveVenue(ctx, s.Venue)
		if err != nil {
			return nil, err
		}
		q.Set("fields.venue.sys.id", id)
	}
	return q, nil
}
func (c *Client) resolveVenue(ctx context.Context, input string) (string, error) {
	// Exact opaque IDs are checked with an ID query before treating the input as a name.
	q := qbase("venue", "sys.id,fields.fullName", 1, 0, 0)
	q.Set("sys.id", input)
	f, err := c.Query(ctx, q)
	if err != nil {
		return "", err
	}
	if len(f.Items) == 1 {
		return f.Items[0].Sys.ID, nil
	}
	q = qbase("venue", "sys.id,fields.fullName", 20, 0, 0)
	q.Set("query", input)
	f, err = c.Query(ctx, q)
	if err != nil {
		return "", err
	}
	idx := map[string]Entry{}
	for _, e := range f.Items {
		e.Fields["name"] = e.Fields["fullName"]
		idx[e.Sys.ID] = e
	}
	id, err := resolveName(input, idx)
	if f.Total > len(f.Items) && err == nil {
		return "", Fail("ambiguous_filter", "Venue search was truncated; use a source ID from venues search", 2)
	}
	return id, err
}
func asError(err error) *Error { return Classify(err) }
func (c *Client) venuesByIDs(ctx context.Context, ids []string, detail bool) (map[string]Venue, error) {
	out := map[string]Venue{}
	if len(ids) == 0 {
		return out, nil
	}
	sort.Strings(ids)
	ids = unique(ids)
	sel := venueSelect
	if detail {
		sel = venueDetailSelect
	}
	q := qbase("venue", sel, len(ids), 0, 1)
	q.Set("sys.id[in]", strings.Join(ids, ","))
	f, err := c.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	idx := index(f.Includes.Entry)
	for _, e := range f.Items {
		out[e.Sys.ID] = makeVenue(e, idx, detail)
	}
	return out, nil
}
func unique(ss []string) []string {
	out := []string{}
	for _, s := range ss {
		if s != "" && (len(out) == 0 || s != out[len(out)-1]) {
			out = append(out, s)
		}
	}
	return out
}
func (c *Client) eventResults(ctx context.Context, f Feed, detail bool) ([]Event, []*Error) {
	ids := []string{}
	for _, e := range f.Items {
		if id := linkID(value(e, "venue")); id != "" {
			ids = append(ids, id)
		}
	}
	errs := []*Error{}
	vs, err := c.venuesByIDs(ctx, ids, detail)
	if err != nil {
		errs = append(errs, asError(err))
		vs = map[string]Venue{}
	}
	cats := map[string]Entry{}
	if len(f.Items) > 0 {
		cats, err = c.catalogEntries(ctx, "eventCategory")
		if err != nil {
			errs = append(errs, asError(err))
			cats = map[string]Entry{}
		}
	}
	es := []Event{}
	for _, e := range f.Items {
		es = append(es, makeEvent(e, vs, cats, Today(), detail))
		id := linkID(value(e, "venue"))
		if id != "" {
			if _, ok := vs[id]; !ok {
				errs = append(errs, Fail("unresolved_venue", "Linked venue "+id+" is unavailable", 5))
			}
		}
	}
	return es, errs
}
func (c *Client) SearchEvents(ctx context.Context, s Search) (Result, error) {
	q, err := c.eventQuery(ctx, s)
	if err != nil {
		return Result{}, err
	}
	f, err := c.Query(ctx, q)
	if err != nil {
		return Result{}, err
	}
	scanned := len(f.Items)
	scanPages := 1
	next := f.Skip + len(f.Items)
	capped := false
	if s.Artist != "" {
		candidates := f.Items
		maxPages := s.MaxScanPages
		if maxPages == 0 {
			maxPages = 3
		}
		matching := func(e Entry) bool { return artistMatches(e, s.Artist) }
		count := 0
		for _, e := range candidates {
			if matching(e) {
				count++
			}
		}
		for count < s.Limit && next < f.Total && scanPages < maxPages {
			q.Set("skip", strconv.Itoa(next))
			q.Set("limit", "50")
			more, er := c.Query(ctx, q)
			if er != nil {
				return Result{}, er
			}
			scanPages++
			scanned += len(more.Items)
			next = more.Skip + len(more.Items)
			if len(more.Items) == 0 {
				break
			}
			for _, e := range more.Items {
				if matching(e) {
					count++
				}
			}
			candidates = append(candidates, more.Items...)
		}
		f.Items = []Entry{}
		for i, e := range candidates {
			if matching(e) {
				f.Items = append(f.Items, e)
				if len(f.Items) == s.Limit {
					next = s.Offset + i + 1
					capped = next < f.Total
					break
				}
			}
		}

	}
	es, errs := c.eventResults(ctx, f, false)
	sortEvents(es, s.Sort)
	p := &Pagination{Offset: s.Offset, Limit: s.Limit, Total: f.Total, Returned: len(es)}
	if next < f.Total {
		p.NextOffset = &next
	}
	warnings := []string{"Date filters compare overall source date spans; weekly/exceptional closures require event detail. Listing is not ticket availability.", "The source honors one sort key; ID tiebreaks apply within returned pages. Source index changes can shift pagination."}
	if s.Artist != "" {
		warnings = append(warnings, "Artist filtering is local over bounded full-text source candidates; total counts candidates, not artist matches. Use --max-scan-pages to widen a zero-match scan.")
	}
	r := c.result(es, p, errs, warnings, "source "+q.Get("order")+"; ID tiebreak within page")
	r.Meta.Truncated = r.Meta.Truncated || capped
	r.Meta.Partial = r.Meta.Partial || (s.Artist != "" && (next < f.Total || capped))
	r.Meta.Scope = map[string]any{"filters": s, "scanned_events": scanned, "scan_pages": scanPages, "candidate_total": f.Total}
	return r, nil
}
func artistMatches(e Entry, needle string) bool {
	n := names(e, "artists")
	needle = strings.ToLower(strings.TrimSpace(needle))
	for _, p := range []*string{n.EN, n.JA} {
		if p != nil && strings.Contains(strings.ToLower(*p), needle) {
			return true
		}
	}
	return false
}
func sortEvents(es []Event, key string) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i].Starts, es[j].Starts
		switch key {
		case "ends":
			a, b = es[i].Ends, es[j].Ends
		case "updated":
			a, b = es[i].UpdatedAt, es[j].UpdatedAt
		}
		if a == nil || b == nil {
			if a == nil && b != nil {
				return false
			}
			if a != nil && b == nil {
				return true
			}
		} else if *a != *b {
			if key == "updated" {
				return *a > *b
			}
			return *a < *b
		}
		return es[i].ID < es[j].ID
	})
}

func identity(input string, kind string) (string, string, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err != nil || u.Host != "www.tokyoartbeat.com" && u.Host != "tokyoartbeat.com" {
			return "", "", Fail("invalid_identity", "Expected a Tokyo Art Beat source URL or ID", 2)
		}
		prefix := "/" + kind + "/-/"
		p := strings.TrimPrefix(u.Path, "/en")
		if !strings.HasPrefix(p, prefix) {
			return "", "", Fail("invalid_identity", "Expected a modern /"+kind+"/-/ source URL; archived legacy URLs are unsupported", 2)
		}
		slug := strings.TrimPrefix(p, prefix)
		if slug == "" {
			return "", "", Fail("invalid_identity", "Source URL is missing its edition slug", 2)
		}
		return "fields.slug", slug, nil
	}
	if input == "" || strings.ContainsAny(input, "\n\r,") || len(input) > 250 {
		return "", "", Fail("invalid_identity", "A source ID or modern source URL is required", 2)
	}
	return "sys.id", input, nil
}
func (c *Client) EventDetail(ctx context.Context, input, on string) (Result, error) {
	key, id, err := identity(input, "events")
	if err != nil {
		return Result{}, err
	}
	if on != "" {
		if _, err = ParseDate(on); err != nil {
			return Result{}, err
		}
	}
	q := qbase("event", eventDetailSelect, 2, 0, 0)
	q.Set(key, id)
	f, err := c.Query(ctx, q)
	if err != nil {
		return Result{}, err
	}
	if len(f.Items) == 0 {
		return Result{}, Fail("not_found", "No public event matches that source identity", 4)
	}
	if len(f.Items) > 1 {
		return Result{}, Fail("ambiguous_identity", "Edition URL matched multiple events; use a source ID", 2)
	}
	es, errs := c.eventResults(ctx, f, true)
	if on != "" {
		d, _ := AssessDay(es[0], on)
		es[0].Day = &d
	}
	r := c.result(es[0], nil, errs, []string{"Event fees and hours are separate from venue defaults. Last admission and ticket links are null/empty when the published feed has no structured value. Official links are source-provided; ticket availability is unknown."}, "")
	return r, nil
}
func (c *Client) SearchVenues(ctx context.Context, query, area, typ string, limit, offset int) (Result, error) {
	if limit < 1 || limit > 50 || offset < 0 || offset > 100000 {
		return Result{}, Fail("invalid_pagination", "Venue --limit must be 1..50 and --offset 0..100000", 2)
	}
	q := qbase("venue", venueSelect, limit, offset, 1)
	q.Set("order", "fields.fullName")
	if query != "" {
		q.Set("query", query)
	}
	for _, x := range []struct{ input, ctype, key string }{{area, "localArea", "fields.localArea.sys.id"}, {typ, "venueType", "fields.venueType.sys.id"}} {
		if x.input != "" {
			idx, err := c.catalogEntries(ctx, x.ctype)
			if err != nil {
				return Result{}, err
			}
			id, err := resolveName(x.input, idx)
			if err != nil {
				return Result{}, err
			}
			q.Set(x.key, id)
		}
	}
	f, err := c.Query(ctx, q)
	if err != nil {
		return Result{}, err
	}
	idx := index(f.Includes.Entry)
	vs := []Venue{}
	for _, e := range f.Items {
		vs = append(vs, makeVenue(e, idx, false))
	}
	return c.result(vs, page(f, len(vs)), nil, nil, "source venue name then ID"), nil
}
func (c *Client) VenueDetail(ctx context.Context, input string) (Result, error) {
	key, id, err := identity(input, "venues")
	if err != nil {
		return Result{}, err
	}
	q := qbase("venue", venueDetailSelect, 2, 0, 1)
	q.Set(key, id)
	f, err := c.Query(ctx, q)
	if err != nil {
		return Result{}, err
	}
	if len(f.Items) == 0 {
		return Result{}, Fail("not_found", "No public venue matches that source identity", 4)
	}
	if len(f.Items) > 1 {
		return Result{}, Fail("ambiguous_identity", "Venue URL matched multiple entries; use a source ID", 2)
	}
	v := makeVenue(f.Items[0], index(f.Includes.Entry), true)
	return c.result(v, nil, nil, []string{"Venue source status and hours may differ from event schedules; inspect event overrides and confirm official exceptional closures."}, ""), nil
}
func (c *Client) Nearby(ctx context.Context, center Geo, radius float64, candidates int, s Search) (Result, error) {
	if math.IsNaN(center.Lat) || math.IsNaN(center.Lon) || math.IsInf(center.Lat, 0) || math.IsInf(center.Lon, 0) || math.IsNaN(radius) || math.IsInf(radius, 0) || center.Lat < -90 || center.Lat > 90 || center.Lon < -180 || center.Lon > 180 {
		return Result{}, Fail("invalid_coordinates", "--lat must be -90..90 and --lon -180..180, with finite values", 2)
	}
	if radius <= 0 || radius > 50 || candidates < 1 || candidates > 100 {
		return Result{}, Fail("invalid_nearby", "--radius-km must be >0..50 and --candidates 1..100", 2)
	}
	if math.Abs(center.Lat) > 85 {
		return Result{}, Fail("invalid_coordinates", "Nearby supports latitudes between -85 and 85", 2)
	}
	if s.Offset != 0 {
		return Result{}, Fail("invalid_offset", "Nearby shortlists use --candidates, not --offset", 2)
	}
	q, err := c.eventQuery(ctx, s)
	if err != nil {
		return Result{}, err
	}
	// Coordinate operators are ignored by the source. Filter a bounded candidate page locally.
	vq := qbase("venue", venueSelect, candidates, 0, 1)
	if s.Area != "" {
		idx, er := c.catalogEntries(ctx, "localArea")
		if er != nil {
			return Result{}, er
		}
		id, er := resolveName(s.Area, idx)
		if er != nil {
			return Result{}, er
		}
		vq.Set("fields.localArea.sys.id", id)
	}
	vq.Set("order", "sys.id")
	vf, err := c.Query(ctx, vq)
	if err != nil {
		return Result{}, err
	}
	idx := index(vf.Includes.Entry)
	vs := map[string]Venue{}
	ids := []string{}
	dist := map[string]float64{}
	for _, e := range vf.Items {
		v := makeVenue(e, idx, false)
		if v.Coordinates != nil {
			d := Distance(center, *v.Coordinates)
			if d <= radius {
				vs[v.ID] = v
				ids = append(ids, v.ID)
				dist[v.ID] = d
			}
		}
	}
	rs := []Event{}
	errs := []*Error{}
	eventTruncated := false
	if len(ids) > 0 {
		sort.Strings(ids)
		q.Set("fields.venue.sys.id[in]", strings.Join(ids, ","))
		q.Set("limit", "100")
		q.Set("skip", "0")
		f, er := c.Query(ctx, q)
		if er != nil {
			return Result{}, er
		}
		eventTruncated = f.Total > len(f.Items)
		cats, er := c.catalogEntries(ctx, "eventCategory")
		if er != nil {
			errs = append(errs, asError(er))
			cats = map[string]Entry{}
		}
		for _, e := range f.Items {
			if s.Artist != "" && !artistMatches(e, s.Artist) {
				continue
			}
			id := linkID(value(e, "venue"))
			if d, ok := dist[id]; ok {
				ev := makeEvent(e, vs, cats, Today(), false)
				d = math.Round(d*1000) / 1000
				ev.DistanceKM = &d
				rs = append(rs, ev)
			}
		}
	}
	sort.Slice(rs, func(i, j int) bool {
		if *rs[i].DistanceKM != *rs[j].DistanceKM {
			return *rs[i].DistanceKM < *rs[j].DistanceKM
		}
		return rs[i].ID < rs[j].ID
	})
	trimmed := len(rs) > s.Limit
	if trimmed {
		rs = rs[:s.Limit]
	}
	r := c.result(rs, nil, errs, []string{"Straight-line distance from public coordinates; no walking time. Ranking covers fetched candidates, not every venue in Japan. Coordinate filters are local; use --area and --candidates to widen or focus coverage. Listing does not imply ticket availability."}, "straight-line distance then source ID, among fetched candidates")
	r.Meta.Truncated = trimmed || vf.Total > len(vf.Items) || eventTruncated
	r.Meta.Partial = r.Meta.Partial || vf.Total > len(vf.Items) || eventTruncated
	if len(rs) == 0 {
		r.Meta.Warnings = append(r.Meta.Warnings, "No matches among scanned venue candidates; use --area and increase --candidates. This does not establish absence of nearby exhibitions.")
	}
	r.Meta.Scope = map[string]any{"center": center, "radius_km": radius, "venue_candidates_total": vf.Total, "venue_candidates_fetched": len(vf.Items), "candidate_limit": candidates, "event_candidate_limit": 100, "event_candidates_truncated": eventTruncated, "limit": s.Limit}
	return r, nil
}

func (c *Client) Compare(ctx context.Context, inputs []string, on string) (Result, error) {
	if len(inputs) < 2 || len(inputs) > 4 {
		return Result{}, Fail("invalid_compare", "Compare requires 2..4 exhibition source IDs or URLs", 2)
	}
	if on != "" {
		if _, err := ParseDate(on); err != nil {
			return Result{}, err
		}
	}
	results := []Event{}
	errs := []*Error{}
	for _, input := range inputs {
		r, err := c.EventDetail(ctx, input, on)
		if err != nil {
			e := Classify(err)
			copy := *e
			copy.Input = input
			errs = append(errs, &copy)
			continue
		}
		results = append(results, r.Results.(Event))
		errs = append(errs, r.Errors...)
	}
	if len(results) == 0 {
		exit := errs[0].Exit
		for _, e := range errs {
			if e.Exit != exit {
				exit = 5
				break
			}
		}
		failure := Fail("compare_failed", "No exhibitions could be inspected; inspect error.failures for each input and its correction", exit)
		failure.Failures = errs
		return Result{}, failure
	}
	r := c.result(results, nil, errs, []string{"Comparison preserves exhibition and venue fees/hours separately. Ticket availability is unknown."}, "input order, successful fetches only")
	return r, nil
}

func (c *Client) Source(ctx context.Context, typ string, limit, include int) (Result, error) {
	if limit < 1 || limit > 10 || include != 0 {
		return Result{}, Fail("invalid_source", "Diagnostic source requires --limit 1..10 and --include 0", 2)
	}
	sel := ""
	switch typ {
	case "event":
		sel = eventSelect
	case "venue":
		sel = venueSelect
	case "localArea", "eventCategory", "venueType":
		sel = "sys.id,fields.name"
	default:
		return Result{}, Fail("invalid_source", "Diagnostic content type must be event, venue, localArea, eventCategory or venueType", 2)
	}
	f, e := c.Query(ctx, qbase(typ, sel, limit, 0, 0))
	if e != nil {
		return Result{}, e
	}
	return c.result(f, page(f, len(f.Items)), nil, nil, "source default"), nil
}
