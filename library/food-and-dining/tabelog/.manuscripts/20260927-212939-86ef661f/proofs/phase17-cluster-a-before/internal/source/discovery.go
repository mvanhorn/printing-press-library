package source

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"tabelog-pp-cli/internal/domain"
)

//go:embed catalogs.json
var catalogsJSON []byte

type catalog struct {
	CapturedAt  time.Time       `json:"captured_at"`
	Prefectures []domain.Choice `json:"prefectures"`
	Cuisines    []domain.Choice `json:"cuisines"`
	Areas       []domain.Choice `json:"areas"`
}

var bootstrap = func() catalog {
	var v catalog
	if e := json.Unmarshal(catalogsJSON, &v); e != nil {
		panic(e)
	}
	return v
}()
var areaCode = regexp.MustCompile(`^[AC][0-9]{4,6}$`)
var thresholds = []int{0, 1000, 2000, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 20000, 30000, 40000, 50000, 60000, 80000, 100000}

func BudgetThresholds() []int { return append([]int(nil), thresholds[1:]...) }

func BudgetCode(yen int) (int, error) {
	for i, n := range thresholds {
		if yen == n {
			return i, nil
		}
	}
	return 0, fail("usage", fmt.Sprintf("unsupported budget threshold %d; use 1000,2000,3000,4000,5000,6000,8000,10000,15000,20000,30000,40000,50000,60000,80000,100000 JPY", yen))
}
func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func knownPrefecture(slug string) bool {
	for _, c := range bootstrap.Prefectures {
		if c.Prefecture == slug {
			return true
		}
	}
	return false
}
func choiceMatches(c domain.Choice, q string) bool {
	q = norm(q)
	return q == norm(c.Name) || q == norm(c.Selector) || q == norm(c.Genre) || strings.Contains(norm(c.Name), q)
}
func dedupeChoices(rows []domain.Choice) []domain.Choice {
	out := make([]domain.Choice, 0)
	seen := map[string]bool{}
	for _, c := range rows {
		key := c.Kind + ":" + c.Selector
		if !seen[key] {
			seen[key] = true
			out = append(out, c)
		}
	}
	return out
}

func (c *Client) Lookup(ctx context.Context, q, kind string) ([]domain.Choice, error) {
	if kind != "" && kind != "area" && kind != "station" && kind != "prefecture" {
		return nil, fail("usage", "--kind must be area, station, or prefecture")
	}
	out := make([]domain.Choice, 0)
	for _, row := range append(append([]domain.Choice{}, bootstrap.Prefectures...), bootstrap.Areas...) {
		if (kind == "" || kind == row.Kind) && choiceMatches(row, q) {
			out = append(out, row)
		}
	}
	if q != "" && kind != "prefecture" {
		rows, e := c.Suggestions(ctx, q)
		if e != nil {
			return nil, e
		}
		for _, row := range rows {
			if row.Kind != "cuisine" && row.Kind != "restaurant" && (kind == "" || kind == row.Kind) {
				out = append(out, row)
			}
		}
		c.saveChoices(rows)
	}
	return dedupeChoices(out), nil
}

func (c *Client) Cuisines(ctx context.Context, q string) ([]domain.Choice, error) {
	out := make([]domain.Choice, 0)
	for _, row := range bootstrap.Cuisines {
		if q == "" || choiceMatches(row, q) {
			out = append(out, row)
		}
	}
	if q != "" && len(out) == 0 {
		rows, e := c.Suggestions(ctx, q)
		if e != nil {
			return nil, e
		}
		c.saveChoices(rows)
		for _, row := range rows {
			if row.Kind == "cuisine" {
				out = append(out, row)
			}
		}
	}
	return dedupeChoices(out), nil
}

func (c *Client) saveChoices(rows []domain.Choice) {
	if c.CacheDir == "" || c.Mode == "local" {
		return
	}
	path := filepath.Join(c.CacheDir, ".choices.json")
	m := map[string]domain.Choice{}
	if b, e := os.ReadFile(path); e == nil && len(b) <= 1<<20 {
		_ = json.Unmarshal(b, &m)
	}
	if len(m) > 1024 {
		m = map[string]domain.Choice{}
	}
	for _, r := range rows {
		m[r.Selector] = r
	}
	b, e := json.Marshal(m)
	if e != nil {
		return
	}
	if os.MkdirAll(c.CacheDir, 0700) == nil {
		_ = os.WriteFile(path, b, 0600)
	}
}

func (c *Client) cachedChoice(selector string) (domain.Choice, bool) {
	if c.CacheDir == "" {
		return domain.Choice{}, false
	}
	b, e := os.ReadFile(filepath.Join(c.CacheDir, ".choices.json"))
	if e != nil || len(b) > 1<<20 {
		return domain.Choice{}, false
	}
	m := map[string]domain.Choice{}
	if json.Unmarshal(b, &m) != nil {
		return domain.Choice{}, false
	}
	v, ok := m[selector]
	return v, ok
}

func locationURL(raw string) (domain.Choice, error) {
	if e := ValidateSourceURL(raw); e != nil {
		return domain.Choice{}, e
	}
	u, _ := url.Parse(raw)
	if strings.TrimRight(u.Path, "/") == "/en/rstLst" {
		q := u.Query()
		pref := q.Get("pal")
		if !knownPrefecture(pref) {
			return domain.Choice{}, fail("usage", "source location URL requires a verified pal prefecture")
		}
		path := "/en/" + pref + "/"
		for _, key := range []string{"LstPrf", "LstAre"} {
			v := q.Get(key)
			if v == "" {
				continue
			}
			if !areaCode.MatchString(v) {
				return domain.Choice{}, fail("usage", "source location URL has an invalid area code")
			}
			path += v + "/"
		}
		u.Path = path + "rstLst/"
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || !knownPrefecture(parts[1]) {
		return domain.Choice{}, fail("usage", "--area URL must identify a verified English prefecture/area listing")
	}
	ch := domain.Choice{Name: parts[1], Kind: "prefecture", Prefecture: parts[1], Exact: true}
	for i, p := range parts[2:] {
		if p == "rstLst" {
			break
		}
		if !areaCode.MatchString(p) || i > 1 {
			return domain.Choice{}, fail("usage", "--area URL must identify geography rather than a restaurant/detail section")
		}
		if i == 0 {
			ch.Area1 = p
		} else {
			ch.Area2 = p
		}
		ch.Kind = "area"
	}
	q := url.Values{}
	q.Set("pal", ch.Prefecture)
	if ch.Area1 != "" {
		q.Set("LstPrf", ch.Area1)
	}
	if ch.Area2 != "" {
		q.Set("LstAre", ch.Area2)
	}
	if station := u.Query().Get("station_id"); station != "" {
		if _, e := strconv.Atoi(station); e != nil {
			return domain.Choice{}, fail("usage", "invalid station_id in area URL")
		}
		q.Set("station_id", station)
		ch.Kind = "station"
		ch.StationID = station
	}
	// A native route is stable and needs no lookup. Source queries retain only geographic fields.
	path := "/en/" + ch.Prefecture + "/"
	if ch.Area1 != "" {
		path += ch.Area1 + "/"
	}
	if ch.Area2 != "" {
		path += ch.Area2 + "/"
	}
	path += "rstLst/"
	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""
	if ch.StationID != "" {
		u.RawQuery = q.Encode()
	}
	ch.URL = u.String()
	ch.Selector = ch.URL
	for _, a := range bootstrap.Areas {
		if a.Area1 == ch.Area1 && a.Area2 == ch.Area2 && a.Prefecture == ch.Prefecture {
			ch.Name = a.Name
		}
	}
	return ch, nil
}

func (c *Client) ResolveArea(ctx context.Context, input string) (domain.Choice, error) {
	if strings.HasPrefix(input, "http:") || strings.HasPrefix(input, "https:") {
		return locationURL(input)
	}
	for _, row := range bootstrap.Prefectures {
		if norm(input) == row.Prefecture || norm(input) == norm(row.Name) {
			row.URL = strings.TrimRight(row.URL, "/") + "/rstLst/"
			return row, nil
		}
	}
	if row, ok := c.cachedChoice(input); ok && row.Kind != "cuisine" && row.Kind != "restaurant" {
		return row, nil
	}
	rows, e := c.Lookup(ctx, input, "")
	if e != nil {
		return domain.Choice{}, e
	}
	exact := make([]domain.Choice, 0)
	for _, r := range rows {
		if r.Exact || norm(r.Name) == norm(input) {
			exact = append(exact, r)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(rows) == 1 {
		return rows[0], nil
	}
	if len(rows) == 0 {
		return domain.Choice{}, fail("not_found", "no source location found; use areas QUERY or a canonical English area URL")
	}
	return domain.Choice{}, &Error{Kind: "ambiguous", Message: "location is ambiguous; choose an area/station URL or typed selector from areas", Choices: rows, Meta: c.Meta()}
}

func (c *Client) ResolveCuisine(ctx context.Context, input string) (string, error) {
	if input == "" {
		return "", nil
	}
	for _, r := range bootstrap.Cuisines {
		if norm(input) == norm(r.Genre) || norm(input) == norm(r.Selector) {
			return r.Genre, nil
		}
	}
	if row, ok := c.cachedChoice(input); ok && row.Kind == "cuisine" {
		return row.Genre, nil
	}
	rows, e := c.Cuisines(ctx, input)
	if e != nil {
		return "", e
	}
	if len(rows) == 1 {
		return rows[0].Genre, nil
	}
	if len(rows) == 0 {
		return "", fail("not_found", "no source category found; use cuisines QUERY")
	}
	return "", &Error{Kind: "ambiguous", Message: "cuisine is ambiguous; choose the category's canonical genre code", Choices: rows, Meta: c.Meta()}
}

type FindOptions struct {
	Area      string
	Cuisine   string
	Meal      string
	BudgetMin int
	BudgetMax int
	Keyword   string
	Limit     int
	MaxPages  int
}

func (o FindOptions) Validate() error {
	if strings.TrimSpace(o.Area) == "" {
		return fail("usage", "find requires --area; use areas QUERY for typed location choices")
	}
	if o.Limit < 1 || o.Limit > 50 {
		return fail("usage", "--limit must be between1 and50")
	}
	if o.MaxPages < 1 || o.MaxPages > 5 {
		return fail("usage", "--max-pages must be between1 and5")
	}
	if o.Meal != "" && o.Meal != "lunch" && o.Meal != "dinner" {
		return fail("usage", "--meal must be lunch or dinner")
	}
	if o.BudgetMin != 0 || o.BudgetMax != 0 {
		if o.Meal == "" {
			return fail("usage", "budget filters require explicit --meal lunch or dinner")
		}
		if _, e := BudgetCode(o.BudgetMin); e != nil {
			return e
		}
		if _, e := BudgetCode(o.BudgetMax); e != nil {
			return e
		}
		if o.BudgetMax != 0 && o.BudgetMin > o.BudgetMax {
			return fail("usage", "--budget-min must not exceed --budget-max")
		}
	}
	return nil
}

func listingURL(area domain.Choice, genre string, o FindOptions) (string, error) {
	u, e := url.Parse(area.URL)
	if e != nil {
		return "", e
	}
	q := u.Query()
	q.Set("SrtT", "rt")
	if genre != "" {
		if strings.Contains(u.Path, "/rstLst/") && u.Path != "/en/rstLst/" {
			u.Path = strings.SplitN(u.Path, "/rstLst/", 2)[0] + "/rstLst/" + genre + "/"
		} else {
			q.Set("genre_name", genre)
		}
	}
	if o.Keyword != "" {
		q.Set("sw", o.Keyword)
	}
	if o.Meal != "" {
		v := "2"
		if o.Meal == "lunch" {
			v = "1"
		}
		q.Set("RdoCosTp", v)
	}
	if o.BudgetMin != 0 {
		v, _ := BudgetCode(o.BudgetMin)
		q.Set("LstCos", strconv.Itoa(v))
	}
	if o.BudgetMax != 0 {
		v, _ := BudgetCode(o.BudgetMax)
		q.Set("LstCosT", strconv.Itoa(v))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

type effectiveRoute struct {
	prefecture, area1, area2, station, genre string
	query                                    url.Values
}

func decodeRoute(raw string) (effectiveRoute, bool) {
	u, e := url.Parse(raw)
	if e != nil {
		return effectiveRoute{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	r := effectiveRoute{query: u.Query()}
	if len(parts) < 2 || parts[0] != "en" {
		return r, false
	}
	idx := 1
	if parts[idx] != "rstLst" {
		r.prefecture = parts[idx]
		idx++
	}
	for idx < len(parts) && parts[idx] != "rstLst" {
		p := parts[idx]
		if strings.HasPrefix(p, "R") {
			r.station = strings.TrimPrefix(p, "R")
		} else if r.area1 == "" {
			r.area1 = p
		} else if r.area2 == "" {
			r.area2 = p
		} else {
			return r, false
		}
		idx++
	}
	if idx >= len(parts) {
		return r, false
	}
	idx++
	if idx < len(parts) {
		if _, e := strconv.Atoi(parts[idx]); e != nil {
			r.genre = parts[idx]
		}
	}
	if r.prefecture == "" {
		r.prefecture = r.query.Get("pal")
	}
	if r.area1 == "" {
		r.area1 = r.query.Get("LstPrf")
	}
	if r.area2 == "" {
		r.area2 = r.query.Get("LstAre")
	}
	if r.station == "" {
		r.station = r.query.Get("station_id")
	}
	if r.genre == "" {
		r.genre = r.query.Get("genre_name")
	}
	return r, r.prefecture != ""
}
func validateListing(l Listing, area domain.Choice, genre string, o FindOptions) error {
	// Next and sort links describe the effective complete query. Ancestor,
	// sidebar and footer links cannot prove its geographic level or filters.
	var current effectiveRoute
	known := false
	if l.NextURL != "" {
		current, known = decodeRoute(l.NextURL)
	}
	if !known {
		walk(l.Document, func(n *html.Node) {
			if known || n.Data != "a" || !class(n, "navi-rstlst__label") {
				return
			}
			if r, ok := decodeRoute(attr(n, "href")); ok {
				current = r
				known = true
			}
		})
	}
	if !known {
		best := -1
		walk(l.Document, func(n *html.Node) {
			if n.Data != "a" || !class(n, "list-condition__condition-target") {
				return
			}
			if r, ok := decodeRoute(attr(n, "href")); ok {
				score := 1
				if r.area1 != "" {
					score++
				}
				if r.area2 != "" {
					score++
				}
				if r.station != "" {
					score++
				}
				if score > best {
					best = score
					current = r
					known = true
				}
			}
		})
	}
	matched := known && current.prefecture == area.Prefecture
	if area.StationID != "" {
		matched = matched && current.station == area.StationID
	} else {
		matched = matched && current.area1 == area.Area1 && current.area2 == area.Area2 && current.station == ""
	}
	if !matched {
		return fail("constraint_mismatch", "source returned different geographic constraints or silently narrowed the requested area")
	}
	if current.genre != genre {
		return fail("constraint_mismatch", "source returned different cuisine/category constraints")
	}
	min, _ := BudgetCode(o.BudgetMin)
	max, _ := BudgetCode(o.BudgetMax)
	actualMin, _ := strconv.Atoi(current.query.Get("LstCos"))
	actualMax, _ := strconv.Atoi(current.query.Get("LstCosT"))
	if actualMin != min || actualMax != max {
		return fail("constraint_mismatch", "source returned different meal-budget constraints")
	}
	if norm(current.query.Get("sw")) != norm(o.Keyword) {
		verifiedEmptyKeyword := len(l.Items) == 0 && current.query.Get("sw") == "" && o.Keyword != "" && byClass(l.Document, "rstlist-notfound") != nil && strings.Contains(norm(l.Condition), norm(o.Keyword))
		if !verifiedEmptyKeyword {
			return fail("constraint_mismatch", "source returned different keyword constraints")
		}
	}
	if o.Meal != "" && (o.BudgetMin != 0 || o.BudgetMax != 0) {
		if !strings.Contains(strings.ToLower(l.Condition), o.Meal) {
			return fail("constraint_mismatch", "source did not preserve the requested meal budget filter")
		}
		for _, bound := range []int{o.BudgetMin, o.BudgetMax} {
			if bound != 0 && !strings.Contains(l.Condition, fmt.Sprintf("%s", withCommas(bound))) {
				return fail("constraint_mismatch", "source did not confirm the requested budget threshold")
			}
		}
	}
	if o.Keyword != "" && !strings.Contains(strings.ToLower(l.Condition), strings.ToLower(o.Keyword)) {
		return fail("constraint_mismatch", "source did not confirm the requested keyword")
	}
	return nil
}
func withCommas(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func (c *Client) Find(ctx context.Context, o FindOptions) ([]domain.Restaurant, []domain.Restaurant, domain.Meta, error) {
	if e := o.Validate(); e != nil {
		return nil, nil, domain.Meta{}, e
	}
	area, e := c.ResolveArea(ctx, o.Area)
	if e != nil {
		return nil, nil, domain.Meta{}, e
	}
	genre, e := c.ResolveCuisine(ctx, o.Cuisine)
	if e != nil {
		return nil, nil, domain.Meta{}, e
	}
	raw, e := listingURL(area, genre, o)
	if e != nil {
		return nil, nil, domain.Meta{}, e
	}
	selected := make([]domain.Restaurant, 0)
	all := make([]domain.Restaurant, 0)
	seen := map[string]bool{}
	urls := map[string]bool{}
	areaCriteria := map[string]any{"name": area.Name, "kind": area.Kind, "prefecture": area.Prefecture}
	if area.Area1 != "" {
		areaCriteria["area1"] = area.Area1
	}
	if area.Area2 != "" {
		areaCriteria["area2"] = area.Area2
	}
	if area.StationID != "" {
		areaCriteria["station_id"] = area.StationID
	}
	meta := domain.Meta{SourceSort: "highest_rated", SourceSurface: "listing", BudgetSource: "listing", Criteria: map[string]any{"area": areaCriteria, "cuisine": genre, "meal": o.Meal, "budget_min": o.BudgetMin, "budget_max": o.BudgetMax, "keyword": o.Keyword, "limit": o.Limit, "max_pages": o.MaxPages}, SourceURL: raw}
	next := raw
	for page := 0; page < o.MaxPages && next != "" && len(selected) < o.Limit; page++ {
		if urls[next] {
			return nil, nil, domain.Meta{}, fail("parser_drift", "source pagination repeated a previously fetched page")
		}
		urls[next] = true
		b, t, e := c.Fetch(ctx, next, 15*time.Minute)
		if e != nil {
			return nil, nil, domain.Meta{}, e
		}
		l, e := ParseListing(b, next, t)
		if e != nil {
			return nil, nil, domain.Meta{}, e
		}
		if e = validateListing(l, area, genre, o); e != nil {
			return nil, nil, domain.Meta{}, e
		}
		if e = c.cacheWrite(next, b, t); e != nil {
			return nil, nil, domain.Meta{}, e
		}
		meta.Pages++
		meta.Scanned += len(l.Items)
		if meta.FetchedAt.IsZero() || t.Before(meta.FetchedAt) {
			meta.FetchedAt = t
		}
		for _, r := range l.Items {
			if seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			all = append(all, r)
			if len(selected) < o.Limit {
				selected = append(selected, r)
			}
		}
		next = l.NextURL
	}
	stats := c.Meta()
	meta.Source = stats.Source
	meta.Requests = stats.Requests
	meta.Bytes = stats.Bytes
	meta.Stale = stats.Stale
	meta.Returned = len(selected)
	meta.AgeSeconds = int64(time.Since(meta.FetchedAt).Seconds())
	meta.HasMore = next != "" || len(all) > len(selected)
	meta.NextURL = next
	meta.Coverage = "source_pages"
	meta.OmittedFromScanned = meta.Scanned - meta.Returned
	if next != "" {
		meta.NextURLScope = "after_scanned_pages"
	}
	if len(selected) == 0 {
		meta.Note = "no candidates in the fetched source result; no broader source-wide absence is implied"
	}
	return selected, all, meta, nil
}
