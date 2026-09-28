package ikyu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/cliutil"
	"golang.org/x/net/html"
)

// decodeSSR reads only Nuxt data, and bounds recursive reference expansion.
func decodeSSR(body []byte) (map[string]any, error) {
	marker := []byte(`id="__NUXT_DATA__"`)
	start := bytes.Index(body, marker)
	if start < 0 {
		return nil, &SchemaError{Message: "missing __NUXT_DATA__"}
	}
	start += bytes.IndexByte(body[start:], '>') + 1
	end := bytes.Index(body[start:], []byte("</script>"))
	if end < 0 {
		return nil, &SchemaError{Message: "unterminated Nuxt data"}
	}
	dec := json.NewDecoder(bytes.NewReader(body[start : start+end]))
	dec.UseNumber()
	var table []any
	if err := dec.Decode(&table); err != nil {
		return nil, &SchemaError{Message: "invalid Nuxt reference table"}
	}
	memo := map[int]any{}
	active := map[int]bool{}
	var ref func(int, int) (any, error)
	ref = func(i, depth int) (any, error) {
		if depth > 200 {
			return nil, fmt.Errorf("Nuxt reference depth exceeded")
		}
		if i < 0 {
			if i == -1 || i == -2 {
				return nil, nil
			}
			return nil, fmt.Errorf("unsupported Nuxt numeric sentinel")
		}
		if i >= len(table) {
			return nil, fmt.Errorf("Nuxt reference out of bounds")
		}
		if active[i] {
			return nil, fmt.Errorf("cyclic Nuxt data")
		}
		if v, ok := memo[i]; ok {
			return v, nil
		}
		active[i] = true
		defer delete(active, i)
		var out any
		idx := func(v any) (int, error) {
			n, ok := v.(json.Number)
			if !ok {
				return 0, fmt.Errorf("Nuxt index is not an integer")
			}
			a, e := strconv.Atoi(string(n))
			return a, e
		}
		switch node := table[i].(type) {
		case map[string]any:
			m := map[string]any{}
			for k, v := range node {
				j, e := idx(v)
				if e != nil {
					return nil, e
				}
				x, e := ref(j, depth+1)
				if e != nil {
					return nil, e
				}
				m[k] = x
			}
			out = m
		case []any:
			if len(node) > 0 {
				if tag, ok := node[0].(string); ok {
					if (tag == "ShallowReactive" || tag == "Reactive" || tag == "Ref" || tag == "ShallowRef") && len(node) == 2 {
						j, e := idx(node[1])
						if e != nil {
							return nil, e
						}
						x, e := ref(j, depth+1)
						if e != nil {
							return nil, e
						}
						out = x
					} else {
						return nil, fmt.Errorf("unsupported Nuxt tag %s", tag)
					}
				} else {
					a := make([]any, 0, len(node))
					for _, v := range node {
						j, e := idx(v)
						if e != nil {
							return nil, e
						}
						x, e := ref(j, depth+1)
						if e != nil {
							return nil, e
						}
						a = append(a, x)
					}
					out = a
				}
			} else {
				out = []any{}
			}
		default:
			out = node
		}
		memo[i] = out
		return out, nil
	}
	// Select the data reference before hydration, omitting state and review caches.
	if len(table) < 4 {
		return nil, &SchemaError{Message: "short Nuxt table"}
	}
	root, ok := table[1].(map[string]any)
	if !ok {
		return nil, &SchemaError{Message: "missing Nuxt root"}
	}
	n, ok := root["data"].(json.Number)
	if !ok {
		return nil, &SchemaError{Message: "missing Nuxt data reference"}
	}
	index, _ := strconv.Atoi(string(n))
	data, err := ref(index, 0)
	if err != nil {
		return nil, &SchemaError{Message: err.Error()}
	}
	m, ok := data.(map[string]any)
	if !ok {
		return nil, &SchemaError{Message: "Nuxt data is not an object"}
	}
	return m, nil
}
func operation(data map[string]any, name string) (map[string]any, map[string]any, error) {
	for key, v := range data {
		end := strings.LastIndex(key, "}")
		if end < 0 {
			continue
		}
		var meta struct {
			Name      string         `json:"o"`
			Variables map[string]any `json:"v"`
		}
		if json.Unmarshal([]byte(key[:end+1]), &meta) != nil || meta.Name != name || strings.HasSuffix(key, "-skip") {
			continue
		}
		value, ok := v.(map[string]any)
		if !ok {
			return nil, nil, &SchemaError{Message: "operation " + name + " has no data"}
		}
		return value, meta.Variables, nil
	}
	return nil, nil, &SchemaError{Message: "missing SSR operation " + name}
}

var destinationPath = regexp.MustCompile(`^/[a-z][a-z0-9-]*/[0-9]{6,8}/(?:[a-z][a-z0-9]*/)*$`)
var onsenPath = regexp.MustCompile(`^/onsen/[0-9]{6}/$`)

func parseDestinations(body []byte) ([]Destination, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, &SchemaError{Message: "invalid destination HTML"}
	}
	items := map[string]Destination{}
	var textNode func(*html.Node) string
	textNode = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return strings.TrimSpace(n.Data)
		}
		if n.Type == html.ElementNode && (n.Data == "svg" || n.Data == "script" || n.Data == "style") {
			return ""
		}
		for x := n.FirstChild; x != nil; x = x.NextSibling {
			if text := textNode(x); text != "" {
				return text
			}
		}
		return ""
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}
				u, e := url.Parse(a.Val)
				if e != nil || u.Host != "" && u.Host != "www.ikyu.com" {
					continue
				}
				if !destinationPath.MatchString(u.Path) && !onsenPath.MatchString(u.Path) {
					continue
				}
				name := strings.Join(strings.Fields(textNode(n)), " ")
				if name == "" || len(name) > 100 {
					continue
				}
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				basePath := "/" + parts[0] + "/" + parts[1] + "/"
				d := Destination{ID: parts[1], Name: cleanSourceText(name), Path: basePath, ObservedPath: u.Path}
				observedName := cliutil.CleanText(textAll(n))
				d.ObservedName = &observedName
				if u.Path != basePath {
					note := "Source navigation link contains filters; the unfiltered base destination is used for search."
					d.FilterRemovalNote = &note
				}
				switch basePath {
				case "/tokyo/140000/":
					d.Name = "東京"
					d.Aliases = []string{"tokyo", "東京"}
				case "/hakone/160418/":
					d.Name = "箱根"
					d.Aliases = []string{"hakone", "箱根"}
				}
				if old, ok := items[d.Path]; !ok || len(d.Name) < len(old.Name) {
					items[d.Path] = d
				}
			}
		}
		for x := n.FirstChild; x != nil; x = x.NextSibling {
			walk(x)
		}
	}
	walk(doc)
	out := make([]Destination, 0, len(items))
	for _, d := range items {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	if len(out) == 0 {
		return nil, &SchemaError{Message: "no source destination links"}
	}
	return out, nil
}
func (c *Client) Destinations(ctx context.Context, query string, limit, offset int) (DestinationsResult, error) {
	if err := bounds(limit, offset); err != nil {
		return DestinationsResult{}, err
	}
	raw, f, err := c.fetch(ctx, http.MethodGet, "/", nil, staticTTL)
	if err != nil {
		return DestinationsResult{}, err
	}
	all, err := parseDestinations(raw)
	if err != nil {
		return DestinationsResult{}, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	matches := []Destination{}
	for _, d := range all {
		match := q == "" || strings.Contains(strings.ToLower(d.Name), q) || d.ID == q || d.Path == q
		for _, a := range d.Aliases {
			match = match || strings.Contains(strings.ToLower(a), q)
		}
		if match {
			matches = append(matches, d)
		}
	}
	total := len(matches)
	end := offset + limit
	if end > total {
		end = total
	}
	data := []Destination{}
	if offset < total {
		data = matches[offset:end]
	}
	return DestinationsResult{Data: data, Pagination: page(limit, offset, len(data), len(data), total), Freshness: f}, nil
}
func (c *Client) resolveDestination(ctx context.Context, input string) (Destination, Freshness, error) {
	raw, f, err := c.fetch(ctx, http.MethodGet, "/", nil, staticTTL)
	if err != nil {
		return Destination{}, Freshness{}, err
	}
	all, err := parseDestinations(raw)
	if err != nil {
		return Destination{}, Freshness{}, err
	}
	q := strings.ToLower(strings.TrimSpace(input))
	var matches []Destination
	for _, d := range all {
		match := strings.ToLower(d.Name) == q || d.ID == q || d.Path == q
		for _, a := range d.Aliases {
			match = match || strings.ToLower(a) == q
		}
		if match {
			matches = append(matches, d)
		}
	}
	if len(matches) != 1 {
		return Destination{}, Freshness{}, fmt.Errorf("destination %q is unknown or ambiguous; use stay destinations and an exact source ID/path", input)
	}
	return matches[0], f, nil
}
func (c *Client) Search(ctx context.Context, r SearchRequest) (SearchResult, error) {
	if err := ValidateStay(r.Stay, c.opts.Now()); err != nil {
		return SearchResult{}, err
	}
	if err := bounds(r.Limit, r.Offset); err != nil {
		return SearchResult{}, err
	}
	if err := validatePreferences(r.Preferences); err != nil {
		return SearchResult{}, err
	}
	dest, catalogFreshness, err := c.resolveDestination(ctx, r.Destination)
	if err != nil {
		return SearchResult{}, err
	}
	link, _ := CanonicalURL("00000000", "", "", &r.Stay)
	u, _ := url.Parse(link)
	pageNumber := r.Offset/20 + 1
	path := dest.Path
	if pageNumber > 1 {
		path += "p" + strconv.Itoa(pageNumber) + "/"
	}
	path += "?" + u.RawQuery
	raw, f, err := c.fetch(ctx, http.MethodGet, path, nil, availabilityTTL)
	if err != nil {
		return SearchResult{}, err
	}
	data, err := decodeSSR(raw)
	if err != nil {
		return SearchResult{}, err
	}
	value, vars, err := operation(data, "ListPageDataIkyu")
	if err != nil {
		return SearchResult{}, err
	}
	input, ok := vars["searchAccommodationsInput"].(map[string]any)
	if !ok {
		return SearchResult{}, &SchemaError{Message: "missing search condition echo"}
	}
	field := "areaIds"
	if strings.HasPrefix(dest.Path, "/onsen/") {
		field = "springGroundIds"
	}
	expected, ok := input[field].([]any)
	matched := false
	for _, id := range expected {
		matched = matched || id == dest.ID
	}
	if !ok || !matched {
		return SearchResult{}, &SchemaError{Message: "source normalized destination"}
	}
	if err := verifySearchStay(input, r.Stay); err != nil {
		return SearchResult{}, err
	}
	list, ok := value["listPageIkyu"].(map[string]any)
	if !ok {
		return SearchResult{}, &SchemaError{Message: "missing destination list"}
	}
	encoded, _ := json.Marshal(list["accommodations"])
	var conn connection[rawProperty]
	if err := json.Unmarshal(encoded, &conn); err != nil || conn.Total == nil {
		return SearchResult{}, &SchemaError{Message: "missing property connection"}
	}
	out := SearchResult{Dependencies: map[string]Freshness{"destination_catalog": catalogFreshness}, Data: []Property{}, Stay: r.Stay, Freshness: f, Coverage: coverage(r.Preferences, "local filtering of one source page and at most three room previews per property")}
	skip := r.Offset % 20
	scanned := 0
	for i, e := range conn.Edges {
		if i < skip {
			continue
		}
		if scanned >= r.Limit {
			break
		}
		scanned++
		p, err := normalizeProperty(&e.Node)
		if err != nil {
			return SearchResult{}, err
		}
		if e.Node.Amount == nil {
			out.Coverage.Unknown = append(out.Coverage.Unknown, p.ID+": dated price absent")
			if !hasPreferences(r.Preferences) {
				gap := "Dated price and availability are unverified; inspect exact room-plan detail for the requested stay."
				p.DetailGap = &gap
				p.URL, _ = CanonicalURL(p.ID, "", "", &r.Stay)
				out.Data = append(out.Data, p)
			}
			continue
		}
		if err := verifyAmount(*e.Node.Amount, r.Stay, false); err != nil {
			return SearchResult{}, err
		}
		if hasPreferences(r.Preferences) {
			matched, room, err := qualifyingPreview(e.Node, r.Stay, r.Preferences)
			if err != nil {
				return SearchResult{}, err
			}
			if matched == nil {
				out.Coverage.Unknown = append(out.Coverage.Unknown, p.ID+": no single observed room-plan proves all requested criteria; remaining previews/details are uninspected")
				continue
			}
			price := normalizePrice(*matched)
			p.Price = &price
			p.Match = &PreviewMatch{RoomID: room.ID, PlanID: matched.Plan.ID, Meal: matched.Plan.Meal, PointVariation: matched.Plan.PointVariation, DateEchoVerified: false}
		}
		if p.Match != nil {
			p.URL, _ = CanonicalURL(p.ID, p.Match.RoomID, p.Match.PlanID, &r.Stay)
		} else {
			p.URL, _ = CanonicalURL(p.ID, "", "", &r.Stay)
		}
		out.Data = append(out.Data, p)
	}
	out.Pagination = page(r.Limit, r.Offset, scanned, len(out.Data), *conn.Total)
	if len(out.Data) == 0 {
		out.Coverage.Note = "No matches among scanned properties in this source page; use --offset to inspect another page. Unknown room evidence does not count as a match."
	}
	return out, nil
}
func verifySearchStay(input map[string]any, s Stay) error {
	num := func(k string) (int, bool) {
		v, ok := input[k]
		if !ok {
			return 0, false
		}
		switch n := v.(type) {
		case json.Number:
			i, e := strconv.Atoi(string(n))
			return i, e == nil
		case float64:
			return int(n), float64(int(n)) == n
		}
		return 0, false
	}
	date, _ := input["checkInDate"].(string)
	if date != s.CheckIn {
		return &SchemaError{Message: "source normalized check-in date"}
	}
	for k, want := range map[string]int{"peopleCount": s.Adults, "roomCount": s.Rooms, "lodgingCount": nights(s)} {
		got, ok := num(k)
		if !ok || got != want {
			return &SchemaError{Message: "source normalized " + k}
		}
	}
	for i, k := range []string{"childACount", "childBCount", "childCCount", "childDCount", "childECount", "childFCount"} {
		got, ok := num(k)
		if (!ok && s.Children[i] != 0) || got != s.Children[i] {
			return &SchemaError{Message: "source normalized " + k}
		}
	}
	return nil
}

func hasPreferences(p Preferences) bool {
	return p.MinBudget != nil || p.MaxBudget != nil || len(p.Meals) > 0 || p.OutdoorBath || p.HotSpringBath || p.Nonsmoking || p.MinSizeM2 != nil
}
func qualifyingPreview(p rawProperty, s Stay, prefs Preferences) (*rawAmount, *rawRoom, error) {
	if p.SearchRooms == nil || p.SearchRooms.Rooms == nil {
		return nil, nil, nil
	}
	roomPrefs := prefs
	roomPrefs.MinBudget = nil
	roomPrefs.MaxBudget = nil
	roomPrefs.Meals = nil
	for _, edge := range p.SearchRooms.Rooms.Edges {
		raw := edge.Node
		room, err := normalizeRoom(raw, p.ID, &s)
		if err != nil {
			return nil, nil, err
		}
		if !roomMatches(room, roomPrefs) {
			continue
		}
		if raw.Amounts == nil {
			continue
		}
		for _, amount := range raw.Amounts.Edges {
			a := amount.Node
			if a.Plan == nil || a.Plan.ID == "" {
				continue
			}
			if err := validateID("source plan ID", a.Plan.ID); err != nil {
				return nil, nil, &SchemaError{Message: err.Error()}
			}
			meal := len(prefs.Meals) == 0
			for _, m := range prefs.Meals {
				meal = meal || m == a.Plan.Meal.Code
			}
			if !meal || !priceMatches(normalizePrice(a), prefs) {
				continue
			}
			if err := verifyAmount(a, s, false); err != nil {
				return nil, nil, err
			}
			return &a, &raw, nil
		}
	}
	return nil, nil, nil
}

func textAll(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data + " "
	}
	var s strings.Builder
	for x := n.FirstChild; x != nil; x = x.NextSibling {
		s.WriteString(textAll(x))
	}
	return strings.Join(strings.Fields(s.String()), " ")
}
