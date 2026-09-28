package walkerplus

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Search is listing-only; returned overlap days are explicitly unverified.
func (c *Client) Search(ctx context.Context, q Query) (Result, error) {
	return c.discover(ctx, q, false)
}

// Shortlist enriches only max-details candidates and applies evidence constraints.
func (c *Client) Shortlist(ctx context.Context, q Query) (Result, error) {
	return c.discover(ctx, q, true)
}

func (c *Client) discover(ctx context.Context, q Query, enrich bool) (result Result, err error) {
	q, err = NormalizeQuery(q)
	if err != nil {
		return Result{}, err
	}
	if enrich && q.From == "" {
		return Result{}, fmt.Errorf("shortlist requires from and to ISO dates")
	}
	if !enrich && (q.Free || q.Indoor) {
		return Result{}, fmt.Errorf("free and indoor constraints require shortlist detail evidence")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	op := newOperation()
	op.coverage.RequestedPages = q.MaxPages
	result = Result{Query: q, Events: []Event{}}
	defer func() { result.Coverage = c.finish(op) }()
	q, err = c.resolveCity(ctx, q, op)
	if err != nil {
		return result, err
	}
	result.Query = q
	routes := listingRoutes(q)
	if len(routes) > q.MaxPages {
		op.coverage.Incomplete = true
		op.coverage.Truncated = true
		op.coverage.Reasons = append(op.coverage.Reasons, "month routes exceed shared page budget")
	}
	if q.From != "" {
		op.coverage.Reasons = append(op.coverage.Reasons, "native month routes have no year selector; undisclosed editions cannot be discovered")
	}
	next := make([]string, len(routes))
	for i, route := range routes {
		next[i] = route
		if q.Page > 1 {
			next[i] += strconv.Itoa(q.Page) + ".html"
		}
	}
	candidates := []Event{}
	sourceOrder := map[string]int{}
	seen := map[string]bool{}
	// Round-robin first pages across month buckets before deeper pagination.
	for pass := 0; op.coverage.ScannedPages < q.MaxPages; pass++ {
		worked := false
		for i := range next {
			if op.coverage.ScannedPages >= q.MaxPages {
				break
			}
			path := next[i]
			if path == "" {
				continue
			}
			worked = true
			p, fetchErr := c.fetch(ctx, path, op)
			if fetchErr != nil {
				return result, fetchErr
			}
			list, parseErr := parseListing(p)
			if parseErr != nil {
				return result, parseErr
			}
			op.coverage.ScannedPages++
			op.coverage.Routes = append(op.coverage.Routes, p.source.URL)
			if len(routes) == 1 && list.total != nil {
				op.coverage.SourceTotal = list.total
			}
			for _, year := range list.years {
				op.coverage.NativeYearLabels = appendUnique(op.coverage.NativeYearLabels, year)
			}
			next[i] = value(list.next)
			for _, e := range list.events {
				if seen[e.ID] {
					continue
				}
				seen[e.ID] = true
				op.coverage.CandidateCount++
				mayEnrich := enrich && listingFiltersCompatible(e, q)
				if enrich && !mayEnrich {
					op.coverage.ExcludedCount++
					continue
				}
				applyQueryCity(&e, q)
				filtersVerified := matchesLocation(e, q) && categoryMatches(e.Categories, q.Category)
				if !enrich && !filtersVerified {
					op.coverage.ExcludedCount++
					continue
				}
				e.Match = matchEvent(e, q, false)
				if e.Match.State == "excluded" || e.Cancellation != nil {
					op.coverage.ExcludedCount++
					continue
				}
				if !enrich {
					e.Match.Reasons = append(e.Match.Reasons, "source location and category satisfy requested filters")
				}
				candidates = append(candidates, e)
			}
		}
		if !worked {
			break
		}
		_ = pass
	}
	for _, path := range next {
		if path != "" {
			op.coverage.Truncated = true
			op.coverage.NextPage = strptr(c.absolute(path))
			op.coverage.Reasons = appendUnique(op.coverage.Reasons, "listing page budget reached")
			break
		}
	}
	if enrich {
		if q.Sort == "source" {
			for i, e := range candidates {
				sourceOrder[e.ID] = i
			}
		}
		n := len(candidates)
		if n > q.MaxDetails {
			n = q.MaxDetails
			op.coverage.Truncated = true
			op.coverage.Reasons = appendUnique(op.coverage.Reasons, "detail candidate budget reached")
		}
		sortEvents(candidates, q)
		verified := make([]Event, 0, len(candidates))
		unknown := []Event{}
		for _, e := range candidates {
			if matchesLocation(e, q) && categoryMatches(e.Categories, q.Category) {
				verified = append(verified, e)
			} else {
				unknown = append(unknown, e)
			}
		}
		if len(verified) > 0 && len(unknown) > 0 {
			candidates = append(verified, unknown...)
			op.coverage.Reasons = appendUnique(op.coverage.Reasons, "detail selection prioritizes listing-verified location/category matches; candidates with missing evidence use remaining budget")
		}
		details := make([]Event, n)
		errors := make([]error, n)
		var wg sync.WaitGroup
		sem := make(chan struct{}, c.opts.Concurrency)
		for i := 0; i < n; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					errors[i] = ctx.Err()
					return
				}
				details[i], errors[i] = c.event(ctx, "/event/"+candidates[i].ID+"/", candidates[i], op)
			}()
		}
		wg.Wait()
		op.coverage.DetailCount = n
		for i, e := range details {
			if errors[i] != nil {
				return result, errors[i]
			}
			e.Match = matchEvent(e, q, true)
			applyQueryCity(&e, q)
			if !matchesLocation(e, q) || !categoryMatches(e.Categories, q.Category) || e.Match.State == "excluded" || e.Cancellation != nil || (q.Free && e.Admission.Status != "free") || (q.Indoor && (e.Indoor == nil || !*e.Indoor)) {
				op.coverage.ExcludedCount++
				continue
			}
			e.Match.Reasons = append(e.Match.Reasons, "source location and category satisfy requested filters")
			if q.Free {
				e.Match.Reasons = append(e.Match.Reasons, "displayed event admission explicitly free")
			}
			if q.Indoor {
				e.Match.Reasons = append(e.Match.Reasons, "displayed source explicitly identifies unconditional indoor venue")
			}
			result.Events = append(result.Events, e)
		}
	} else {
		result.Events = candidates
	}
	if enrich && q.Sort == "source" {
		sort.SliceStable(result.Events, func(i, j int) bool { return sourceOrder[result.Events[i].ID] < sourceOrder[result.Events[j].ID] })
	}
	sortEvents(result.Events, q)
	if len(result.Events) > q.Limit {
		result.Events = result.Events[:q.Limit]
		op.coverage.Truncated = true
		op.coverage.Reasons = appendUnique(op.coverage.Reasons, "result limit reached")
	}
	op.coverage.ReturnedCount = len(result.Events)
	return result, nil
}

// Missing listing evidence can be checked by bounded detail enrichment.
// Explicit conflicting facts cannot consume that detail budget.
func listingFiltersCompatible(e Event, q Query) bool {
	if q.Prefecture != "" {
		if code := value(e.Location.PrefectureCode); code != "" && code != q.Prefecture {
			return false
		}
		if p, ok := lookup(prefectures, value(e.Location.PrefectureJA)); ok && p.Code != q.Prefecture {
			return false
		}
	}
	if q.City != "" {
		actual := value(e.Location.CityJA)
		cityMatches := actual == q.cityName || (strings.HasSuffix(q.cityName, "市") && strings.HasPrefix(actual, q.cityName) && strings.HasSuffix(actual, "区"))
		parentOnly := strings.HasSuffix(actual, "市") && strings.HasSuffix(q.cityName, "区") && (!strings.Contains(q.cityName, "市") || strings.HasPrefix(q.cityName, actual))
		if code := value(e.Location.CityCode); code != "" && q.Prefecture != "" && !strings.HasPrefix(code, q.Prefecture) {
			return false
		}
		if !cityMatches && !parentOnly {
			if code := value(e.Location.CityCode); code != "" && code != q.City {
				return false
			}
			for _, suffix := range []string{"市", "区", "町", "村"} {
				if actual != "" && strings.HasSuffix(actual, suffix) {
					return false
				}
			}
		}
	}
	return q.Category == "" || len(e.Categories) == 0 || categoryMatches(e.Categories, q.Category)
}

func (c *Client) resolveCity(ctx context.Context, q Query, op *operation) (Query, error) {
	if q.City == "" || q.cityPath != "" {
		return q, nil
	}
	pref, _ := lookup(prefectures, q.Prefecture)
	before := op.coverage.RequestCount
	p, err := c.fetch(ctx, pref.Path, op)
	op.coverage.CatalogRequests += op.coverage.RequestCount - before
	op.coverage.CatalogRoutes = append(op.coverage.CatalogRoutes, c.absolute(pref.Path))
	if err != nil {
		return q, err
	}
	doc, err := parseDoc(p.body)
	if err != nil {
		return q, err
	}
	cities := cityLinks(doc)
	if len(cities) == 0 {
		if _, err = parseListing(p); err != nil {
			return q, err
		}
	}
	filtered := []CatalogItem{}
	for _, city := range cities {
		if value(city.PrefectureCode) == q.Prefecture {
			filtered = append(filtered, city)
		}
	}
	matches := []CatalogItem{}
	for _, item := range filtered {
		if _, ok := lookup([]CatalogItem{item}, q.City); ok {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return q, fmt.Errorf("%w: city %q not found in this prefecture; use areas --prefecture", ErrInvalidQuery, q.City)
	}
	if len(matches) > 1 {
		return q, fmt.Errorf("%w: city %q is ambiguous in this prefecture; use a city code from areas --prefecture", ErrInvalidQuery, q.City)
	}
	city := matches[0]
	q.City = city.Code
	q.cityPath = city.Path
	q.cityName = city.NameJA
	return q, nil
}

func applyQueryCity(e *Event, q Query) {
	if q.City == "" {
		return
	}
	actual := value(e.Location.CityJA)
	matches := actual == q.cityName
	if strings.HasSuffix(q.cityName, "市") && strings.HasPrefix(actual, q.cityName) {
		ward := strings.TrimPrefix(actual, q.cityName)
		matches = matches || (ward != "" && strings.HasSuffix(ward, "区"))
	}
	if q.cityName != "" && matches {
		e.Location.CityCode = strptr(q.City)
	} else {
		e.Location.CityCode = nil
	}
}

func (c *Client) Event(ctx context.Context, idOrURL string) (e Event, err error) {
	path, err := c.eventPath(idOrURL)
	if err != nil {
		return Event{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	op := newOperation()
	defer c.finish(op)
	e, err = c.event(ctx, path, emptyEvent(eventIDFromPath(path), "", c.absolute(path)), op)
	if err == nil {
		op.coverage.DetailCount = 1
		op.coverage.ReturnedCount = 1
	}
	return e, err
}

func (c *Client) event(ctx context.Context, path string, e Event, op *operation) (Event, error) {
	p, err := c.fetch(ctx, path, op)
	if err != nil {
		return Event{}, err
	}
	links, err := parseDetail(p, &e)
	if err != nil {
		return Event{}, err
	}
	done := map[string]bool{path: true}
	queue := []string{}
	// data before price: specific displayed admission wins over overview/schema.
	for _, suffix := range []string{"data.html", "price.html"} {
		for _, link := range links {
			if strings.HasSuffix(link, suffix) {
				queue = appendUnique(queue, link)
			}
		}
	}
	for len(queue) > 0 {
		link := queue[0]
		queue = queue[1:]
		if done[link] {
			continue
		}
		done[link] = true
		p, err = c.fetch(ctx, link, op)
		if err != nil {
			return Event{}, err
		}
		more, err := parseDetail(p, &e)
		if err != nil {
			return Event{}, err
		}
		for _, l := range more {
			if !done[l] {
				queue = appendUnique(queue, l)
			}
		}
	}
	if e.Location.PrefectureCode == nil {
		code := e.ID[:6]
		if p, ok := lookup(prefectures, code); ok {
			e.Location.PrefectureCode = strptr(code)
			e.Location.PrefectureJA = strptr(p.NameJA)
		}
	}
	parseSchedule(&e)
	return e, nil
}

func (c *Client) Categories() CatalogResult {
	return CatalogResult{Items: append([]CatalogItem{}, categoryItems...), SourceURL: c.absolute("/event_list/"), Coverage: emptyCoverage()}
}
func (c *Client) Areas(ctx context.Context, prefecture string) (result CatalogResult, err error) {
	result = CatalogResult{Items: []CatalogItem{}, SourceURL: c.absolute("/event_list/"), Coverage: emptyCoverage()}
	if prefecture == "" {
		result.Items = append(result.Items, prefectures...)
		return result, nil
	}
	p, ok := lookup(prefectures, prefecture)
	if !ok {
		return result, fmt.Errorf("unknown prefecture %q; use areas", prefecture)
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	op := newOperation()
	defer func() { result.Coverage = c.finish(op) }()
	page, err := c.fetch(ctx, p.Path, op)
	if err != nil {
		return result, err
	}
	list, err := parseListing(page)
	if err != nil {
		return result, err
	}
	result.SourceURL = page.source.URL
	result.Items = append(result.Items, p)
	seen := map[string]bool{}
	for _, city := range list.cities {
		if value(city.PrefectureCode) == p.Code && !seen[city.Code] {
			result.Items = append(result.Items, city)
			seen[city.Code] = true
		}
	}
	op.coverage.ScannedPages = 1
	op.coverage.Routes = append(op.coverage.Routes, page.source.URL)
	op.coverage.ReturnedCount = len(result.Items)
	return result, nil
}
