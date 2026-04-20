// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Flagship helpers: shared utilities for the 5 flagship compound-query
// commands (warm-intro, coverage, prospect, dossier, budget). These helpers
// parse LinkedIn MCP text payloads and Happenstance JSON envelopes into a
// common `flagshipPerson` shape so the feature commands can rank and emit
// results without re-implementing the glue each time.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/deepline"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/linkedin"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/store"
)

// flagshipPerson is the common shape used by warm-intro / coverage / prospect
// / dossier. It is intentionally a flat, JSON-safe record rather than a Go
// struct with typed fields — downstream output is --json by default, and the
// set of fields varies by feature.
type flagshipPerson struct {
	Name             string         `json:"name"`
	LinkedInURL      string         `json:"linkedin_url,omitempty"`
	HappenstanceUUID string         `json:"happenstance_uuid,omitempty"`
	Title            string         `json:"title,omitempty"`
	Company          string         `json:"company,omitempty"`
	Location         string         `json:"location,omitempty"`
	ImageURL         string         `json:"image_url,omitempty"`
	ConnectionCount  int            `json:"connection_count,omitempty"`
	Sources          []string       `json:"sources,omitempty"`
	Rationale        string         `json:"rationale,omitempty"`
	Relationship     string         `json:"relationship,omitempty"`
	MutualCount      int            `json:"mutual_count,omitempty"`
	Bridges          []bridgeRef    `json:"bridges,omitempty"`
	Score            float64        `json:"score,omitempty"`
	Raw              any            `json:"raw,omitempty"`
}

// bridgeRef is flagshipPerson's render-time projection of a client.Bridge.
// Kept separate from the client type so the JSON output schema is stable
// even if the canonical Bridge struct gains internal fields. Zero
// AffinityScore is a valid weak-signal marker; renderers must treat it as
// "bridge exists but mention-only" rather than filtering it out.
type bridgeRef struct {
	Name             string  `json:"name,omitempty"`
	HappenstanceUUID string  `json:"happenstance_uuid,omitempty"`
	AffinityScore    float64 `json:"affinity_score"`
	Kind             string  `json:"kind,omitempty"`
}

// bridgesToFlagship projects a slice of canonical client.Bridge entries
// onto the flagshipPerson render shape. Returns nil when the input is
// empty so JSON output omits the field.
func bridgesToFlagship(in []client.Bridge) []bridgeRef {
	if len(in) == 0 {
		return nil
	}
	out := make([]bridgeRef, 0, len(in))
	for _, b := range in {
		out = append(out, bridgeRef{
			Name:             b.Name,
			HappenstanceUUID: b.HappenstanceUUID,
			AffinityScore:    b.AffinityScore,
			Kind:             b.Kind,
		})
	}
	return out
}

// topFriendBridge returns the highest-affinity friend bridge from the
// slice, or (zero, false) if none exist. Self-graph bridges are
// ignored so "via <your own contacts>" never leaks into renderer prose.
func topFriendBridge(bridges []client.Bridge) (client.Bridge, bool) {
	var top client.Bridge
	found := false
	for _, b := range bridges {
		if b.Kind == client.BridgeKindSelfGraph {
			continue
		}
		if !found || b.AffinityScore > top.AffinityScore {
			top = b
			found = true
		}
	}
	return top, found
}

// hasSelfGraphBridge reports whether the slice contains at least one
// self-graph bridge (meaning the person sits in the user's own synced
// LinkedIn/Gmail contacts bucket on the bearer surface).
func hasSelfGraphBridge(bridges []client.Bridge) bool {
	for _, b := range bridges {
		if b.Kind == client.BridgeKindSelfGraph {
			return true
		}
	}
	return false
}

// bearerRationale formats a human-readable rationale string for a
// bearer-surface person based on the graph signal the API returned.
// The decision table:
//
//   - One+ friend bridge with affinity > 0 -> "via <top name> (affinity X.X)"
//   - One+ friend bridge with all affinities 0 -> weak-signal string
//   - Only self-graph bridges -> "in your synced graph"
//   - No bridges at all -> "Happenstance bearer (no graph match)"
//
// Exported (lowercase, package-scoped) so every bearer-path command in
// this package formats consistently. If two commands disagree, fix them
// here, not inline at the call site.
func bearerRationale(bridges []client.Bridge) string {
	top, ok := topFriendBridge(bridges)
	if ok {
		if top.AffinityScore > 0 {
			return fmt.Sprintf("via %s (affinity %.1f)", top.Name, top.AffinityScore)
		}
		return "Happenstance bearer (weak signal, no graph affinity)"
	}
	if hasSelfGraphBridge(bridges) {
		return "in your synced graph"
	}
	return "Happenstance bearer (no graph match)"
}

// bridgeAffinityBonus converts a slice of bridges into an additive
// ranking bonus, scaled so coarse tier ordering from scoreForRelationship
// stays intact for typical affinities but strong graph signal (observed
// up to ~300) can still push a 2nd-degree row past a 1st-degree row
// with a weak source. Scale: log1p(max_friend_affinity). A typical
// non-zero friend bridge sits in the 10-100 range, yielding a bonus of
// 2.4-4.6 — enough to distinguish adjacent bearer rows, not enough to
// leap tiers. Zero or self-graph-only inputs return 0.
func bridgeAffinityBonus(bridges []bridgeRef) float64 {
	var top float64
	for _, b := range bridges {
		if b.Kind == client.BridgeKindSelfGraph {
			continue
		}
		if b.AffinityScore > top {
			top = b.AffinityScore
		}
	}
	if top <= 0 {
		return 0
	}
	return math.Log1p(top)
}

// bearerScore picks the score a bearer-surface row should sort by.
// Priority: max friend-bridge affinity (if > 0) > traits score > 0. The
// traits fallback keeps bearer rows with no graph signal comparable to
// each other via the API's WeightedTraitsScore, which is what the old
// bearer projection relied on exclusively.
func bearerScore(bridges []client.Bridge, traitsScore float64) float64 {
	top, ok := topFriendBridge(bridges)
	if ok && top.AffinityScore > 0 {
		return top.AffinityScore
	}
	return traitsScore
}

// personLookupKey returns the best available unique key (linkedin URL, else
// Happenstance UUID, else normalized name+company).
func (p *flagshipPerson) dedupKey() string {
	if p.LinkedInURL != "" {
		return "li:" + canonicalLinkedInURL(p.LinkedInURL)
	}
	if p.HappenstanceUUID != "" {
		return "hp:" + p.HappenstanceUUID
	}
	return "nm:" + strings.ToLower(strings.TrimSpace(p.Name)) + "|" + strings.ToLower(strings.TrimSpace(p.Company))
}

// canonicalLinkedInURL strips protocol, www, and trailing slashes so
// "https://linkedin.com/in/x/" and "https://www.linkedin.com/in/x" dedupe.
func canonicalLinkedInURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.TrimSuffix(s, "/")
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
		return host + strings.TrimSuffix(u.Path, "/")
	}
	return s
}

// mergePeople merges a slice of flagshipPerson records: later entries for the
// same key add their sources to the earlier entry and update empty fields.
func mergePeople(in []flagshipPerson) []flagshipPerson {
	byKey := map[string]*flagshipPerson{}
	order := []string{}
	for i := range in {
		p := in[i]
		key := p.dedupKey()
		if key == "nm:|" {
			continue // un-identifiable, drop
		}
		if existing, ok := byKey[key]; ok {
			existing.Sources = dedupStrings(append(existing.Sources, p.Sources...))
			if existing.Title == "" {
				existing.Title = p.Title
			}
			if existing.Company == "" {
				existing.Company = p.Company
			}
			if existing.Location == "" {
				existing.Location = p.Location
			}
			if existing.ImageURL == "" {
				existing.ImageURL = p.ImageURL
			}
			if existing.LinkedInURL == "" {
				existing.LinkedInURL = p.LinkedInURL
			}
			if existing.HappenstanceUUID == "" {
				existing.HappenstanceUUID = p.HappenstanceUUID
			}
			if p.ConnectionCount > existing.ConnectionCount {
				existing.ConnectionCount = p.ConnectionCount
			}
			// Bridges are a bearer-surface signal. When a cookie-path row
			// and a bearer-path row dedupe to the same person, the cookie
			// row wins on most fields but the bearer bridges are still
			// useful context — no overlap with the cookie Referrers
			// chain, which lives in the untyped Raw field today. Merge
			// them on so downstream renderers see both.
			if len(p.Bridges) > 0 && len(existing.Bridges) == 0 {
				existing.Bridges = p.Bridges
			}
			continue
		}
		byKey[key] = &p
		order = append(order, key)
	}
	out := make([]flagshipPerson, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

func dedupStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// fetchHappenstanceFriends calls /api/friends/list and returns parsed
// flagshipPerson records tagged with the "hp_friend" source.
func fetchHappenstanceFriends(c *client.Client) ([]flagshipPerson, error) {
	raw, err := c.Get("/api/friends/list", nil)
	if err != nil {
		return nil, err
	}
	// Envelope: may be { "status":"success","data":[...]}, may be [...] directly.
	payload := extractResponseData(raw)

	var arr []map[string]any
	if err := json.Unmarshal(payload, &arr); err != nil {
		// Some endpoints wrap under {"friends":[...]}.
		var obj map[string]json.RawMessage
		if uerr := json.Unmarshal(payload, &obj); uerr != nil {
			return nil, fmt.Errorf("parsing friends/list: %w", err)
		}
		for _, key := range []string{"friends", "results", "items"} {
			if inner, ok := obj[key]; ok {
				if err := json.Unmarshal(inner, &arr); err == nil {
					break
				}
			}
		}
	}
	out := make([]flagshipPerson, 0, len(arr))
	for _, f := range arr {
		p := flagshipPerson{
			Name:             getStr(f, "name", "full_name", "display_name", "friend_name"),
			HappenstanceUUID: getStr(f, "uuid", "id", "friend_id", "user_id"),
			LinkedInURL:      getStr(f, "linkedin_url", "linkedinUrl", "linkedin"),
			Title:            getStr(f, "title", "headline", "job_title"),
			Company:          getStr(f, "company", "current_company", "employer"),
			Location:         getStr(f, "location", "city"),
			ImageURL:         getStr(f, "image_url", "imageUrl", "avatar"),
			ConnectionCount:  coerceCount(f, "connection_count", "connectionCount", "connections"),
			Sources:          []string{"hp_friend"},
			Raw:              f,
		}
		if p.Name == "" && p.HappenstanceUUID == "" && p.LinkedInURL == "" {
			continue
		}
		if p.Name == "" {
			p.Name = "(unnamed friend)"
		}
		out = append(out, p)
	}
	return out, nil
}

// searchPeopleArgs builds the MCP args map for the LinkedIn
// `search_people` tool. Exposed as a pure function so tests can assert
// the shape (notably: the absence of a `limit` key — the MCP tool has
// no `limit` parameter and rejects requests that include it with a
// pydantic `Unexpected keyword argument` error).
func searchPeopleArgs(keywords, location string) map[string]any {
	args := map[string]any{"keywords": keywords}
	if location != "" {
		args["location"] = location
	}
	return args
}

// fetchLinkedInSearchPeople spawns the LinkedIn MCP, runs search_people with
// the given keywords, and parses the JSON text payload into flagshipPerson
// records tagged "li_search". `limit` is applied client-side after parsing
// (the MCP's search_people tool does not accept a server-side limit).
func fetchLinkedInSearchPeople(ctx context.Context, keywords, location string, limit int) ([]flagshipPerson, error) {
	if keywords == "" {
		return nil, errors.New("fetchLinkedInSearchPeople: keywords required")
	}
	client, err := spawnLIClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if _, err := client.Initialize(ctx, linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version}); err != nil {
		return nil, fmt.Errorf("linkedin mcp initialize: %w", err)
	}

	res, err := client.CallTool(ctx, linkedin.ToolNames.SearchPeople, searchPeopleArgs(keywords, location))
	if err != nil {
		return nil, err
	}
	people := parseLIPeoplePayload(linkedin.TextPayload(res), "li_search")
	if limit > 0 && len(people) > limit {
		people = people[:limit]
	}
	return people, nil
}

// fetchLinkedInPerson fetches a single LinkedIn profile by URL or slug.
// Returns the parsed flagshipPerson and the raw JSON payload for dossiers.
func fetchLinkedInPerson(ctx context.Context, linkedinURL string, sections []string) (*flagshipPerson, json.RawMessage, error) {
	if linkedinURL == "" {
		return nil, nil, errors.New("fetchLinkedInPerson: linkedin_url required")
	}
	client, err := spawnLIClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()
	if _, err := client.Initialize(ctx, linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version}); err != nil {
		return nil, nil, fmt.Errorf("linkedin mcp initialize: %w", err)
	}
	args := map[string]any{"linkedin_url": linkedinURL}
	if len(sections) > 0 {
		args["sections"] = sections
	}
	res, err := client.CallTool(ctx, linkedin.ToolNames.GetPerson, args)
	if err != nil {
		return nil, nil, err
	}
	payload := linkedin.TextPayload(res)
	raw := json.RawMessage(payload)
	if !json.Valid([]byte(payload)) {
		raw = nil
	}
	var obj map[string]any
	if raw != nil {
		_ = json.Unmarshal(raw, &obj)
	}
	if obj == nil {
		return &flagshipPerson{Name: linkedinURL, LinkedInURL: linkedinURL, Sources: []string{"li_profile"}}, raw, nil
	}
	p := personFromLIObject(obj, "li_profile")
	if p.LinkedInURL == "" {
		p.LinkedInURL = linkedinURL
	}
	return &p, raw, nil
}

// fetchLinkedInSidebar calls get_sidebar ("People also viewed") for a target.
func fetchLinkedInSidebar(ctx context.Context, personURL string) ([]flagshipPerson, error) {
	client, err := spawnLIClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if _, err := client.Initialize(ctx, linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version}); err != nil {
		return nil, err
	}
	res, err := client.CallTool(ctx, linkedin.ToolNames.Sidebar, map[string]any{"person_url": personURL})
	if err != nil {
		return nil, err
	}
	return parseLIPeoplePayload(linkedin.TextPayload(res), "li_sidebar"), nil
}

// parseLIPeoplePayload turns a raw MCP text payload into flagshipPerson records.
// The LinkedIn MCP returns either a JSON array of person objects or an object
// containing a "results" / "people" array. We try both.
func parseLIPeoplePayload(payload string, sourceTag string) []flagshipPerson {
	if payload == "" {
		return nil
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(payload), &arr); err == nil {
		return parseLIPeopleSlice(arr, sourceTag)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &obj); err == nil {
		for _, key := range []string{"results", "people", "items", "data"} {
			if inner, ok := obj[key]; ok {
				var slice []map[string]any
				if err := json.Unmarshal(inner, &slice); err == nil {
					return parseLIPeopleSlice(slice, sourceTag)
				}
			}
		}
	}
	return nil
}

func parseLIPeopleSlice(items []map[string]any, sourceTag string) []flagshipPerson {
	out := make([]flagshipPerson, 0, len(items))
	for _, item := range items {
		out = append(out, personFromLIObject(item, sourceTag))
	}
	return out
}

func personFromLIObject(item map[string]any, sourceTag string) flagshipPerson {
	name := getStr(item, "name", "full_name", "fullName", "displayName")
	if name == "" {
		first := getStr(item, "first_name", "firstName")
		last := getStr(item, "last_name", "lastName")
		name = strings.TrimSpace(first + " " + last)
	}
	company := getStr(item, "company", "current_company", "companyName", "company_name")
	if company == "" {
		// Sometimes nested under experience[0].company.
		if exp, ok := item["experience"].([]any); ok && len(exp) > 0 {
			if first, ok := exp[0].(map[string]any); ok {
				company = getStr(first, "company", "company_name", "companyName")
			}
		}
	}
	return flagshipPerson{
		Name:        name,
		LinkedInURL: getStr(item, "linkedin_url", "linkedinUrl", "profile_url", "url"),
		Title:       getStr(item, "title", "headline", "current_title", "job_title"),
		Company:     company,
		Location:    getStr(item, "location", "city", "geo"),
		ImageURL:    getStr(item, "image_url", "imageUrl", "profile_picture", "avatar"),
		Sources:     []string{sourceTag},
		Raw:         item,
	}
}

// getStr returns the first non-empty string value in m for the given keys.
func getStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := fmt.Sprintf("%v", v)
			s = strings.TrimSpace(s)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func coerceCount(m map[string]any, keys ...string) int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case float64:
			return int(x)
		case int:
			return x
		case int64:
			return int(x)
		case string:
			// Strip commas, "+", etc.
			cleaned := strings.TrimSpace(x)
			cleaned = strings.ReplaceAll(cleaned, ",", "")
			cleaned = strings.TrimSuffix(cleaned, "+")
			var n int
			_, _ = fmt.Sscanf(cleaned, "%d", &n)
			return n
		case json.Number:
			i, _ := x.Int64()
			return int(i)
		}
	}
	return 0
}

// hydrateMutualHints annotates a list of people with mutual/network hints
// based on whether they appear in the user's Happenstance friends list.
// Modifies in place and returns the same slice.
func hydrateMutualHints(people []flagshipPerson, friends []flagshipPerson) []flagshipPerson {
	if len(friends) == 0 {
		return people
	}
	index := map[string]int{}
	for i, f := range friends {
		if f.LinkedInURL != "" {
			index["li:"+canonicalLinkedInURL(f.LinkedInURL)] = i
		}
		if f.HappenstanceUUID != "" {
			index["hp:"+f.HappenstanceUUID] = i
		}
		if f.Name != "" {
			index["nm:"+strings.ToLower(strings.TrimSpace(f.Name))] = i
		}
	}
	for i := range people {
		p := &people[i]
		hit := -1
		if p.LinkedInURL != "" {
			if idx, ok := index["li:"+canonicalLinkedInURL(p.LinkedInURL)]; ok {
				hit = idx
			}
		}
		if hit < 0 && p.HappenstanceUUID != "" {
			if idx, ok := index["hp:"+p.HappenstanceUUID]; ok {
				hit = idx
			}
		}
		if hit < 0 && p.Name != "" {
			if idx, ok := index["nm:"+strings.ToLower(strings.TrimSpace(p.Name))]; ok {
				hit = idx
			}
		}
		if hit >= 0 {
			p.Sources = dedupStrings(append(p.Sources, "hp_friend"))
			if p.ConnectionCount == 0 && friends[hit].ConnectionCount > 0 {
				p.ConnectionCount = friends[hit].ConnectionCount
			}
		}
	}
	return people
}

// persistPeople upserts a slice of flagshipPerson records into the local
// store. Non-fatal on individual row failures — warnings go to stderr.
func persistPeople(people []flagshipPerson) {
	dbPath := defaultDBPath("contact-goat-pp-cli")
	s, err := store.Open(dbPath)
	if err != nil {
		return
	}
	defer s.Close()
	for _, p := range people {
		if p.Name == "" {
			continue
		}
		// The bearer surface returns name + title + company but no
		// LinkedIn URL or Happenstance UUID. The local store keys on
		// one of those, so bearer-only rows can't be persisted. Skip
		// silently rather than spamming a warning per row.
		if p.LinkedInURL == "" && p.HappenstanceUUID == "" {
			continue
		}
		data := map[string]any{
			"full_name":         p.Name,
			"linkedin_url":      p.LinkedInURL,
			"happenstance_uuid": p.HappenstanceUUID,
			"title":             p.Title,
			"company":           p.Company,
			"location":          p.Location,
			"image_url":         p.ImageURL,
			"sources":           shortSources(p.Sources),
		}
		if _, err := s.UpsertPerson(data); err != nil {
			fmt.Fprintf(os.Stderr, "warning: upsert person %q failed: %v\n", p.Name, err)
		}
	}
}

// shortSources converts feature-level source tags (li_search, hp_friend, ...)
// to the compact CSV tokens expected by the `people.sources` column
// (li/hp/dl). Non-matching tags are preserved verbatim so we keep the
// richer tag history for the in-memory output.
func shortSources(tags []string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range tags {
		var short string
		switch {
		case strings.HasPrefix(t, "li"):
			short = "li"
		case strings.HasPrefix(t, "hp"):
			short = "hp"
		case strings.HasPrefix(t, "dl"):
			short = "dl"
		default:
			short = t
		}
		if seen[short] {
			continue
		}
		seen[short] = true
		out = append(out, short)
	}
	return strings.Join(out, ",")
}

// rankPeople sorts people by a composite score: mutual count → connection
// count → source strength → name.
func rankPeople(in []flagshipPerson) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].Score != in[j].Score {
			return in[i].Score > in[j].Score
		}
		if in[i].MutualCount != in[j].MutualCount {
			return in[i].MutualCount > in[j].MutualCount
		}
		if in[i].ConnectionCount != in[j].ConnectionCount {
			return in[i].ConnectionCount > in[j].ConnectionCount
		}
		return strings.ToLower(in[i].Name) < strings.ToLower(in[j].Name)
	})
}

// sourceStrength assigns a score (higher = warmer) to a source tag used for
// ranking flagship results.
func sourceStrength(tag string) float64 {
	switch tag {
	case "hp_friend":
		return 5.0
	case "hp_graph_1deg":
		return 5.0 // you know them directly — equal to an HP top connector
	case "hp_graph_2deg":
		return 4.5 // concrete 2nd-degree path at the target company
	case "li_1deg":
		return 4.0
	case "li_search", "li_profile":
		return 2.5
	case "li_sidebar":
		return 2.0
	case "hp_network":
		return 3.0
	case "hp_graph_3deg":
		return 1.5 // public hit, no concrete path
	case "li_2deg":
		return 1.5
	case "dl_apollo":
		return 1.0
	}
	return 0.5
}

// deeplinePersonEnrich calls Deepline person-enrich for a given LinkedIn URL.
// Charges credits; logs the call to deepline_log. Returns parsed JSON.
func deeplinePersonEnrich(ctx context.Context, apiKey, linkedinURL string) (json.RawMessage, int, error) {
	client := deepline.NewClient(apiKey)
	payload := map[string]any{"linkedin_url": linkedinURL}
	cost, _ := client.EstimateCost(deepline.ToolPersonEnrich, payload)
	hash := hashPayload(payload)
	if err := client.ValidateKey(); err != nil {
		logDeeplineSafely(deepline.ToolPersonEnrich, hash, cost, "auth-error")
		return nil, 0, err
	}
	res, err := client.Execute(ctx, deepline.ToolPersonEnrich, payload)
	if err != nil {
		logDeeplineSafely(deepline.ToolPersonEnrich, hash, cost, "error")
		return nil, 0, err
	}
	logDeeplineSafely(deepline.ToolPersonEnrich, hash, cost, "ok")
	return res, cost, nil
}

// deeplineApolloSearch calls Deepline apollo-people-search for a given
// title/location/industry/limit combination. Charges credits; logs.
func deeplineApolloSearch(ctx context.Context, apiKey string, payload map[string]any) (json.RawMessage, int, error) {
	client := deepline.NewClient(apiKey)
	cost, _ := client.EstimateCost(deepline.ToolApolloPeopleSearch, payload)
	hash := hashPayload(payload)
	if err := client.ValidateKey(); err != nil {
		logDeeplineSafely(deepline.ToolApolloPeopleSearch, hash, cost, "auth-error")
		return nil, 0, err
	}
	res, err := client.Execute(ctx, deepline.ToolApolloPeopleSearch, payload)
	if err != nil {
		logDeeplineSafely(deepline.ToolApolloPeopleSearch, hash, cost, "error")
		return nil, 0, err
	}
	logDeeplineSafely(deepline.ToolApolloPeopleSearch, hash, cost, "ok")
	return res, cost, nil
}

// parseDeeplinePeople pulls a list of flagshipPerson records out of a
// Deepline apollo-people-search response. Handles both {data:[...]} and
// {results:[...]} shapes.
func parseDeeplinePeople(raw json.RawMessage) []flagshipPerson {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]json.RawMessage
	var items []map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{"data", "results", "people", "items"} {
			if inner, ok := obj[key]; ok {
				if err := json.Unmarshal(inner, &items); err == nil && len(items) > 0 {
					break
				}
			}
		}
	}
	if len(items) == 0 {
		_ = json.Unmarshal(raw, &items)
	}
	out := make([]flagshipPerson, 0, len(items))
	for _, item := range items {
		p := personFromLIObject(item, "dl_apollo")
		// Deepline payloads sometimes surface email/phone; ignore here (dossier does that).
		out = append(out, p)
	}
	return out
}

// nowISO returns an RFC3339 timestamp string for diagnostics.
func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
