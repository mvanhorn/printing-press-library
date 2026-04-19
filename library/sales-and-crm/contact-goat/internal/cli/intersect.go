// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// intersect: find people who appear in BOTH your LinkedIn 1st-degree network
// AND your Happenstance friends list. These are the highest-signal warm
// intros: people you already know on both graphs.
//
// Implementation strategy:
//   1. Pull Happenstance friends from /api/friends/list (or a warm store cache).
//   2. For each friend, probe LinkedIn via `search_people` with name + primary
//      company and check whether a 1st-degree match is surfaced. The
//      linkedin-scraper-mcp result set includes a distance/connection-degree
//      hint for each row; when absent we fall back to "name match on a single
//      top result" as a probabilistic proxy.
//   3. Cache the joined view in p2_cache for 24h to avoid burning LinkedIn
//      scraper time on repeat runs.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/linkedin"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/store"

	"github.com/spf13/cobra"
)

const intersectCacheKey = "transcendence.intersect.v1"
const intersectCacheTTL = 24 * time.Hour

// IntersectMatch is the output shape of a single intersected person.
type IntersectMatch struct {
	Name             string   `json:"name"`
	LinkedInURL      string   `json:"linkedin_url,omitempty"`
	HappenstanceUUID string   `json:"happenstance_uuid,omitempty"`
	ConnectionCount  int      `json:"connection_count"`
	Company          string   `json:"company,omitempty"`
	Sources          []string `json:"sources"`
}

func newIntersectCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var skipLI bool

	cmd := &cobra.Command{
		Use:   "intersect",
		Short: "Find people in BOTH your LinkedIn 1st-degree AND Happenstance friends",
		Long: `Find the highest-signal warm intros: people who appear in BOTH your
LinkedIn 1st-degree network AND your Happenstance friends list.

Results are ranked by Happenstance connection_count and cached locally for
24 hours to avoid repeatedly spinning up the LinkedIn scraper.`,
		Example: `  contact-goat-pp-cli intersect
  contact-goat-pp-cli intersect --limit 20 --json
  contact-goat-pp-cli intersect --no-linkedin   # skip LI probe, list HP friends only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Happenstance is the primary data source — required.
			c, err := flags.newClientRequireCookies("happenstance")
			if err != nil {
				return err
			}

			// Cache first unless --no-cache.
			if !flags.noCache {
				if cached := readIntersectCache(); cached != nil {
					return emitIntersectMatches(cmd, flags, filterLimit(cached, limit))
				}
			}

			// Pull Happenstance friends.
			friendsData, err := c.Get("/api/friends/list", nil)
			if err != nil {
				return classifyAPIError(err)
			}
			friendsData = extractResponseData(friendsData)
			friends, err := parseFriends(friendsData)
			if err != nil {
				return fmt.Errorf("parsing friends list: %w", err)
			}
			if len(friends) == 0 {
				fmt.Fprintln(os.Stderr, "no Happenstance friends returned — is your cookie still valid?")
				fmt.Fprintln(os.Stderr, "Run: contact-goat-pp-cli auth status")
				return nil
			}

			var matches []IntersectMatch
			if skipLI {
				// No LinkedIn probe: list all HP friends that have a linkedin_url
				// stamped in their profile (these are provably also on LI).
				for _, f := range friends {
					if f.LinkedInURL == "" {
						continue
					}
					matches = append(matches, IntersectMatch{
						Name:             f.Name,
						LinkedInURL:      f.LinkedInURL,
						HappenstanceUUID: f.UUID,
						ConnectionCount:  f.ConnectionCount,
						Company:          f.Company,
						Sources:          []string{"hp_friend", "li_url_known"},
					})
				}
			} else {
				matches, err = probeLinkedInForFriends(cmd.Context(), flags, friends, limit)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: LinkedIn probe failed: %v\n", err)
					fmt.Fprintln(os.Stderr, "falling back to hp_friend-only results (re-run with --no-cache to retry).")
					for _, f := range friends {
						matches = append(matches, IntersectMatch{
							Name:             f.Name,
							LinkedInURL:      f.LinkedInURL,
							HappenstanceUUID: f.UUID,
							ConnectionCount:  f.ConnectionCount,
							Company:          f.Company,
							Sources:          []string{"hp_friend"},
						})
					}
				}
			}

			// Sort by connection_count desc then name asc.
			sort.Slice(matches, func(i, j int) bool {
				if matches[i].ConnectionCount != matches[j].ConnectionCount {
					return matches[i].ConnectionCount > matches[j].ConnectionCount
				}
				return matches[i].Name < matches[j].Name
			})

			// Cache and emit.
			writeIntersectCache(matches)
			return emitIntersectMatches(cmd, flags, filterLimit(matches, limit))
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of intersected people to return")
	cmd.Flags().BoolVar(&skipLI, "no-linkedin", false, "Skip LinkedIn probe and list Happenstance friends with known LinkedIn URLs")
	return cmd
}

// hpFriend is the subset of /api/friends/list fields we care about here.
type hpFriend struct {
	Name            string
	UUID            string
	LinkedInURL     string
	Company         string
	ConnectionCount int
}

func parseFriends(data json.RawMessage) ([]hpFriend, error) {
	// Try array directly.
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		// Try wrapped envelope.
		var obj map[string]json.RawMessage
		if err2 := json.Unmarshal(data, &obj); err2 != nil {
			return nil, err
		}
		for _, k := range []string{"friends", "data", "results", "items"} {
			if raw, ok := obj[k]; ok {
				if err3 := json.Unmarshal(raw, &arr); err3 == nil {
					break
				}
			}
		}
	}
	out := make([]hpFriend, 0, len(arr))
	for _, m := range arr {
		f := hpFriend{
			Name:            str(m["name"]),
			UUID:            str(firstNonNil(m, "uuid", "id")),
			LinkedInURL:     str(firstNonNil(m, "linkedin_url", "linkedinUrl", "linkedin")),
			Company:         str(firstNonNil(m, "primary_company", "company", "employer")),
			ConnectionCount: toInt(firstNonNil(m, "connection_count", "connectionCount", "num_connections")),
		}
		if f.Name == "" {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

func str(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func firstNonNil(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	case string:
		var n int
		fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}

func probeLinkedInForFriends(parentCtx context.Context, flags *rootFlags, friends []hpFriend, limit int) ([]IntersectMatch, error) {
	if len(friends) == 0 {
		return nil, nil
	}
	// Pre-flight: login check. If LinkedIn MCP isn't set up, don't even spawn.
	if ok, _ := linkedin.IsLoggedIn(); !ok {
		return nil, fmt.Errorf("linkedin-mcp not logged in. %s", linkedin.LoginHint())
	}

	ctx, cancel := signalCtx(parentCtx)
	defer cancel()
	probeCap := limit * 2
	if probeCap < len(friends) {
		friends = friends[:probeCap]
	}

	client, err := spawnLIClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if _, err := client.Initialize(ctx, linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version}); err != nil {
		return nil, fmt.Errorf("initialize linkedin-mcp: %w", err)
	}

	out := make([]IntersectMatch, 0, len(friends))
	for _, f := range friends {
		if parentCtx.Err() != nil {
			break
		}
		query := f.Name
		if f.Company != "" {
			query = f.Name + " " + f.Company
		}
		callCtx, callCancel := context.WithTimeout(ctx, flags.timeout)
		result, callErr := client.CallTool(callCtx, linkedin.ToolNames.SearchPeople, map[string]any{
			"keywords": query,
			"limit":    3,
		})
		callCancel()
		if callErr != nil {
			fmt.Fprintf(os.Stderr, "warning: li probe for %q failed: %v\n", f.Name, callErr)
			continue
		}
		hits := parseLISearchHits(linkedin.TextPayload(result))
		// Accept a hit as "1st-degree" when the result explicitly marks
		// degree == 1 OR distance == "DISTANCE_1" OR, absent those flags, the
		// top result's name closely matches ours (heuristic; noted in Sources).
		var matched *liSearchHit
		for i := range hits {
			if hits[i].isFirstDegree() {
				matched = &hits[i]
				break
			}
		}
		heuristic := false
		if matched == nil && len(hits) > 0 {
			if nameMatch(hits[0].Name, f.Name) {
				matched = &hits[0]
				heuristic = true
			}
		}
		if matched == nil {
			continue
		}
		sources := []string{"li_1deg", "hp_friend"}
		if heuristic {
			sources = []string{"li_name_match", "hp_friend"}
		}
		out = append(out, IntersectMatch{
			Name:             f.Name,
			LinkedInURL:      firstNonEmpty(matched.URL, f.LinkedInURL),
			HappenstanceUUID: f.UUID,
			ConnectionCount:  f.ConnectionCount,
			Company:          firstNonEmpty(matched.Company, f.Company),
			Sources:          sources,
		})
	}
	return out, nil
}

type liSearchHit struct {
	Name     string
	URL      string
	Company  string
	Distance string
	Degree   int
}

// isFirstDegree applies a lenient interpretation of the linkedin-scraper-mcp
// result so we don't miss legitimate matches when the server changes labels.
func (h liSearchHit) isFirstDegree() bool {
	if h.Degree == 1 {
		return true
	}
	d := strings.ToLower(h.Distance)
	return d == "distance_1" || d == "1st" || d == "first"
}

// parseLISearchHits handles both a JSON array and a JSON object with a common
// wrapper field. The LinkedIn MCP traditionally emits text blocks that are
// JSON; we tolerate either shape here.
func parseLISearchHits(payload string) []liSearchHit {
	if payload == "" {
		return nil
	}
	var arr []map[string]any
	if json.Unmarshal([]byte(payload), &arr) != nil {
		var obj map[string]json.RawMessage
		if json.Unmarshal([]byte(payload), &obj) != nil {
			return nil
		}
		for _, k := range []string{"results", "people", "data", "items"} {
			if raw, ok := obj[k]; ok {
				if json.Unmarshal(raw, &arr) == nil {
					break
				}
			}
		}
	}
	hits := make([]liSearchHit, 0, len(arr))
	for _, m := range arr {
		hit := liSearchHit{
			Name:     str(firstNonNil(m, "name", "full_name", "fullName")),
			URL:      str(firstNonNil(m, "linkedin_url", "profile_url", "url")),
			Company:  str(firstNonNil(m, "company", "current_company", "employer")),
			Distance: str(firstNonNil(m, "distance", "connection_degree")),
			Degree:   toInt(firstNonNil(m, "degree", "connection_degree_number")),
		}
		if hit.Name != "" {
			hits = append(hits, hit)
		}
	}
	return hits
}

func nameMatch(a, b string) bool {
	la, lb := strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	if la == "" || lb == "" {
		return false
	}
	if la == lb {
		return true
	}
	// Tolerate middle-initial / name-order variations by comparing sorted tokens.
	return strings.Fields(la)[0] == strings.Fields(lb)[0] && strings.Fields(la)[len(strings.Fields(la))-1] == strings.Fields(lb)[len(strings.Fields(lb))-1]
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func emitIntersectMatches(cmd *cobra.Command, flags *rootFlags, matches []IntersectMatch) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(matches)
	}
	if len(matches) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no intersected people found)")
		return nil
	}
	items := make([]map[string]any, 0, len(matches))
	for _, m := range matches {
		items = append(items, map[string]any{
			"name":             m.Name,
			"company":          m.Company,
			"connection_count": m.ConnectionCount,
			"linkedin_url":     m.LinkedInURL,
			"sources":          strings.Join(m.Sources, ","),
		})
	}
	return printAutoTable(cmd.OutOrStdout(), items)
}

func filterLimit(m []IntersectMatch, limit int) []IntersectMatch {
	if limit <= 0 || len(m) <= limit {
		return m
	}
	return m[:limit]
}

// ---- cache helpers ----

func readIntersectCache() []IntersectMatch {
	s, err := openP2Store()
	if err != nil || s == nil {
		return nil
	}
	defer s.Close()
	raw, ok := s.P2CacheGet(intersectCacheKey)
	if !ok {
		return nil
	}
	var out []IntersectMatch
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func writeIntersectCache(m []IntersectMatch) {
	if len(m) == 0 {
		return
	}
	s, err := openP2Store()
	if err != nil || s == nil {
		return
	}
	defer s.Close()
	b, _ := json.Marshal(m)
	if err := s.P2CacheSet(intersectCacheKey, b, intersectCacheTTL); err != nil {
		fmt.Fprintf(os.Stderr, "warning: intersect cache write failed: %v\n", err)
	}
}

// openP2Store opens the SQLite store, creating the directory + schema if needed.
// Returns (nil, nil) only on disk errors that aren't file-not-found.
func openP2Store() (*store.Store, error) {
	dbPath := defaultDBPath("contact-goat-pp-cli")
	return store.Open(dbPath)
}
