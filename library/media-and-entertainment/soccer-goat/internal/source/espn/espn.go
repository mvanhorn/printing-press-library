package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/cliutil"
)

const (
	// searchURL is ESPN's multi-sport site search. Root cause of the prior 100%
	// miss: the old code queried common/v3/search with sport=soccer, and that
	// endpoint returns count:0 for a sport= filter (it wants type=player). We use
	// the richer search/v2 endpoint, which is live and returns typed player rows.
	searchURL = "https://site.web.api.espn.com/apis/search/v2"
	// overviewURLFmt returns an athlete's season statistics, per-competition
	// splits, and recent-match gameLog in a single call.
	overviewURLFmt = "https://site.web.api.espn.com/apis/common/v3/sports/soccer/all/athletes/%s/overview"
	desktopChrome  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	maxRetries     = 2
	// recentGamesLimit caps how many recent matches we surface from the gameLog.
	recentGamesLimit = 5
)

// Context is the player context ESPN exposes: identity plus optional season
// stats, per-competition splits, and recent-match form when enrichment ran.
type Context struct {
	DisplayName string         `json:"displayName"`
	Team        string         `json:"team"`
	League      string         `json:"league"`
	Link        string         `json:"link"`
	AthleteID   string         `json:"athleteId,omitempty"`
	Stats       *SeasonStats   `json:"stats,omitempty"`
	Splits      []StatSplit    `json:"splits,omitempty"`
	RecentGames []GameLogEntry `json:"recentGames,omitempty"`
}

// SeasonStats is the named-field view of ESPN's column-model stat line. Values
// are aggregated by stat name, never by column position (ESPN reorders columns).
type SeasonStats struct {
	Appearances    int `json:"appearances,omitempty"`
	Starts         int `json:"starts,omitempty"`
	Goals          int `json:"goals"`
	Assists        int `json:"assists"`
	Shots          int `json:"shots,omitempty"`
	ShotsOnTarget  int `json:"shotsOnTarget,omitempty"`
	YellowCards    int `json:"yellowCards,omitempty"`
	RedCards       int `json:"redCards,omitempty"`
	FoulsCommitted int `json:"foulsCommitted,omitempty"`
	FoulsSuffered  int `json:"foulsSuffered,omitempty"`
	Offsides       int `json:"offsides,omitempty"`
}

// StatSplit is one per-competition / per-team row of the season stats.
type StatSplit struct {
	DisplayName string      `json:"displayName"`
	TeamID      string      `json:"teamId,omitempty"`
	TeamSlug    string      `json:"teamSlug,omitempty"`
	LeagueID    string      `json:"leagueId,omitempty"`
	LeagueSlug  string      `json:"leagueSlug,omitempty"`
	Stats       SeasonStats `json:"stats"`
}

// GameLogEntry is one recent match: metadata (from gameLog.events) joined with
// that match's stat line (from gameLog.statistics[].events, keyed by eventId).
type GameLogEntry struct {
	Date     string `json:"date,omitempty"`
	Opponent string `json:"opponent,omitempty"`
	AtVs     string `json:"atVs,omitempty"`
	Score    string `json:"score,omitempty"`
	Result   string `json:"result,omitempty"`
	League   string `json:"league,omitempty"`
	Goals    int    `json:"goals,omitempty"`
	Assists  int    `json:"assists,omitempty"`
}

// Enrichment bundles the pieces Enrich returns; the caller assigns them onto a
// resolved Context.
type Enrichment struct {
	Stats       *SeasonStats
	Splits      []StatSplit
	RecentGames []GameLogEntry
}

type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 12 * time.Second},
		limiter: cliutil.NewAdaptiveLimiter(3),
	}
}

// searchResponse mirrors the ESPN search/v2 shape: results grouped by type,
// each carrying a contents list of the actual hits.
type searchResponse struct {
	Results []struct {
		Type     string `json:"type"`
		Contents []struct {
			DisplayName       string `json:"displayName"`
			Subtitle          string `json:"subtitle"`
			UID               string `json:"uid"`
			DefaultLeagueSlug string `json:"defaultLeagueSlug"`
			Sport             string `json:"sport"`
			Link              struct {
				Web string `json:"web"`
			} `json:"link"`
		} `json:"contents"`
	} `json:"results"`
}

// Lookup resolves the best-matching soccer athlete for a name via search/v2.
// It accepts a row only when it is soccer AND its name matches the query, so an
// ambiguous or common name never silently resolves to the wrong athlete (whose
// id would then feed the soccer-only overview path and return junk).
func (c *Client) Lookup(ctx context.Context, name string) (Context, bool, error) {
	if cliutil.IsVerifyEnv() {
		return Context{}, false, nil
	}
	query := url.Values{}
	query.Set("query", name)
	query.Set("limit", "5")
	target := searchURL + "?" + query.Encode()
	body, err := c.get(ctx, target)
	if err != nil {
		return Context{}, false, fmt.Errorf("espn lookup %q: %w", name, err)
	}
	return parseLookup(body, name)
}

// parseLookup applies the resolution guard to a search/v2 payload. Split from
// Lookup so the guard/id-extraction logic is unit-testable against fixtures.
func parseLookup(body []byte, name string) (Context, bool, error) {
	var response searchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return Context{}, false, fmt.Errorf("espn lookup %q: decode response: %w", name, err)
	}

	wantTokens := nameTokens(name)
	for _, result := range response.Results {
		if !strings.EqualFold(result.Type, "player") {
			continue
		}
		for _, item := range result.Contents {
			if !isSoccer(item.Sport, item.DefaultLeagueSlug) {
				continue
			}
			if !nameMatches(wantTokens, item.DisplayName) {
				continue
			}
			link := item.Link.Web
			if strings.HasPrefix(link, "/") {
				link = "https://www.espn.com" + link
			}
			return Context{
				DisplayName: strings.TrimSpace(item.DisplayName),
				Team:        strings.TrimSpace(item.Subtitle),
				League:      strings.TrimSpace(item.DefaultLeagueSlug),
				Link:        link,
				AthleteID:   athleteID(item.UID, link),
			}, true, nil
		}
	}
	return Context{}, false, nil
}

// isSoccer keeps non-soccer athletes (search/v2 has no sport filter) out of the
// resolution, since their id would 404/500 the soccer-only overview endpoint.
func isSoccer(sport, leagueSlug string) bool {
	if strings.EqualFold(sport, "soccer") {
		return true
	}
	// Some rows omit sport but carry a soccer league slug (e.g. fifa.world, eng.1).
	return sport == "" && leagueSlug != ""
}

// athleteID extracts ESPN's numeric athlete id from a uid ("s:600~a:355061")
// or, failing that, from the player link path (".../id/355061/...").
func athleteID(uid, link string) string {
	if idx := strings.Index(uid, "~a:"); idx >= 0 {
		return strings.TrimSpace(uid[idx+3:])
	}
	if idx := strings.Index(link, "/id/"); idx >= 0 {
		rest := link[idx+4:]
		if end := strings.IndexByte(rest, '/'); end >= 0 {
			return rest[:end]
		}
		return rest
	}
	return ""
}

// nameTokens normalizes a query into lowercase, diacritic-folded tokens of
// length >= 3 (the significant name parts to match on).
func nameTokens(name string) []string {
	var tokens []string
	for _, tok := range strings.Fields(normalizeName(name)) {
		if len(tok) >= 3 {
			tokens = append(tokens, tok)
		}
	}
	return tokens
}

// nameMatches accepts a display name when any significant query token appears in
// it (surname-level match), or the whole query is a substring. Empty query
// tokens (all short) fall back to accepting, so single short names still resolve.
func nameMatches(wantTokens []string, displayName string) bool {
	got := normalizeName(displayName)
	if len(wantTokens) == 0 {
		return true
	}
	for _, tok := range wantTokens {
		if strings.Contains(got, tok) {
			return true
		}
	}
	return false
}

var diacriticFolder = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ä", "a", "ã", "a", "å", "a", "ā", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e", "ē", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "ī", "i",
	"ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o", "ø", "o", "ō", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u", "ū", "u",
	"ç", "c", "ñ", "n", "ß", "ss", "š", "s", "ž", "z", "ć", "c", "č", "c",
)

// normalizeName lowercases, folds common Latin diacritics, and keeps only
// letters, digits, and spaces so name matching is accent-insensitive.
func normalizeName(s string) string {
	s = diacriticFolder.Replace(strings.ToLower(strings.TrimSpace(s)))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '\'':
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// overviewResponse mirrors the ESPN athlete overview: season stats + splits, and
// a gameLog whose match metadata (events map) is separate from its per-match
// stat lines (statistics[].events, keyed by eventId).
type overviewResponse struct {
	Statistics struct {
		Names  []string `json:"names"`
		Splits []struct {
			DisplayName string   `json:"displayName"`
			TeamID      string   `json:"teamId"`
			TeamSlug    string   `json:"teamSlug"`
			LeagueID    string   `json:"leagueId"`
			LeagueSlug  string   `json:"leagueSlug"`
			Stats       []string `json:"stats"`
		} `json:"splits"`
	} `json:"statistics"`
	GameLog struct {
		Statistics []struct {
			Names  []string `json:"names"`
			Events []struct {
				EventID string   `json:"eventId"`
				Stats   []string `json:"stats"`
			} `json:"events"`
		} `json:"statistics"`
		Events map[string]struct {
			GameDate   string `json:"gameDate"`
			AtVs       string `json:"atVs"`
			Score      string `json:"score"`
			GameResult string `json:"gameResult"`
			LeagueName string `json:"leagueName"`
			Opponent   struct {
				DisplayName string `json:"displayName"`
			} `json:"opponent"`
		} `json:"events"`
	} `json:"gameLog"`
}

// Enrich fetches an athlete's overview and returns season stats, per-competition
// splits, and recent-match form. A missing/failed payload yields (nil, nil) so
// the caller keeps the already-resolved context (ESPN stays "ok", stats absent).
func (c *Client) Enrich(ctx context.Context, id string) (*Enrichment, error) {
	if cliutil.IsVerifyEnv() || strings.TrimSpace(id) == "" {
		return nil, nil
	}
	body, err := c.get(ctx, fmt.Sprintf(overviewURLFmt, url.PathEscape(id)))
	if err != nil {
		return nil, err
	}
	return parseOverview(body, id)
}

// parseOverview builds the Enrichment from an athlete overview payload. Split
// from Enrich so the stat aggregation and gameLog join are unit-testable.
func parseOverview(body []byte, id string) (*Enrichment, error) {
	var overview overviewResponse
	if err := json.Unmarshal(body, &overview); err != nil {
		return nil, fmt.Errorf("espn enrich %q: decode response: %w", id, err)
	}

	enr := &Enrichment{}
	names := overview.Statistics.Names
	total := SeasonStats{}
	haveSplits := false
	for _, split := range overview.Statistics.Splits {
		stats := statsFromNames(names, split.Stats)
		if isTotalRow(split.DisplayName, split.TeamID, split.LeagueID) {
			// A pre-summed total row: use it as the aggregate but don't list or
			// re-sum it (that would double-count the real competition rows).
			if !haveSplits {
				total = stats
			}
			continue
		}
		enr.Splits = append(enr.Splits, StatSplit{
			DisplayName: split.DisplayName,
			TeamID:      split.TeamID,
			TeamSlug:    split.TeamSlug,
			LeagueID:    split.LeagueID,
			LeagueSlug:  split.LeagueSlug,
			Stats:       stats,
		})
		addStats(&total, stats)
		haveSplits = true
	}
	if haveSplits || hasAnyStat(total) {
		agg := total
		enr.Stats = &agg
	}

	enr.RecentGames = recentGames(overview)
	if enr.Stats == nil && len(enr.Splits) == 0 && len(enr.RecentGames) == 0 {
		return nil, nil
	}
	return enr, nil
}

// recentGames joins gameLog match metadata with each match's stat line and
// returns the most-recent matches first.
func recentGames(overview overviewResponse) []GameLogEntry {
	if len(overview.GameLog.Events) == 0 {
		return nil
	}
	// Build eventId -> stat map from the gameLog stat table (first block).
	perGame := map[string]map[string]string{}
	if len(overview.GameLog.Statistics) > 0 {
		block := overview.GameLog.Statistics[0]
		for _, ev := range block.Events {
			perGame[ev.EventID] = zipNames(block.Names, ev.Stats)
		}
	}
	var games []GameLogEntry
	for id, meta := range overview.GameLog.Events {
		entry := GameLogEntry{
			Date:     meta.GameDate,
			Opponent: strings.TrimSpace(meta.Opponent.DisplayName),
			AtVs:     meta.AtVs,
			Score:    meta.Score,
			Result:   meta.GameResult,
			League:   meta.LeagueName,
		}
		if stats := perGame[id]; stats != nil {
			entry.Goals = atoiSafe(stats["totalGoals"])
			entry.Assists = atoiSafe(stats["goalAssists"])
		}
		games = append(games, entry)
	}
	sort.Slice(games, func(i, j int) bool { return games[i].Date > games[j].Date })
	if len(games) > recentGamesLimit {
		games = games[:recentGamesLimit]
	}
	return games
}

// statsFromNames maps ESPN's name->value columns onto SeasonStats fields by
// name (never by column index — ESPN reorders columns between athletes).
func statsFromNames(names, values []string) SeasonStats {
	m := zipNames(names, values)
	return SeasonStats{
		Appearances:    atoiSafe(m["appearances"]),
		Starts:         atoiSafe(m["starts"]),
		Goals:          atoiSafe(m["totalGoals"]),
		Assists:        atoiSafe(m["goalAssists"]),
		Shots:          atoiSafe(m["totalShots"]),
		ShotsOnTarget:  atoiSafe(m["shotsOnTarget"]),
		YellowCards:    atoiSafe(m["yellowCards"]),
		RedCards:       atoiSafe(m["redCards"]),
		FoulsCommitted: atoiSafe(m["foulsCommitted"]),
		FoulsSuffered:  atoiSafe(m["foulsSuffered"]),
		Offsides:       atoiSafe(m["offsides"]),
	}
}

func addStats(total *SeasonStats, s SeasonStats) {
	total.Appearances += s.Appearances
	total.Starts += s.Starts
	total.Goals += s.Goals
	total.Assists += s.Assists
	total.Shots += s.Shots
	total.ShotsOnTarget += s.ShotsOnTarget
	total.YellowCards += s.YellowCards
	total.RedCards += s.RedCards
	total.FoulsCommitted += s.FoulsCommitted
	total.FoulsSuffered += s.FoulsSuffered
	total.Offsides += s.Offsides
}

func hasAnyStat(s SeasonStats) bool {
	return s != SeasonStats{}
}

// isTotalRow detects a pre-summed aggregate row so it is neither listed as a
// competition split nor summed into the total.
func isTotalRow(displayName, teamID, leagueID string) bool {
	if teamID == "" && leagueID == "" {
		return true
	}
	name := strings.ToLower(displayName)
	return strings.Contains(name, "total") ||
		strings.Contains(name, "all competition") ||
		strings.Contains(name, "career")
}

func zipNames(names, values []string) map[string]string {
	m := make(map[string]string, len(names))
	for i, n := range names {
		if i < len(values) {
			m[n] = values[i]
		}
	}
	return m
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func (c *Client) get(ctx context.Context, target string) ([]byte, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", desktopChrome)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			retryAfter := cliutil.RetryAfter(resp)
			if attempt == maxRetries {
				return nil, &cliutil.RateLimitError{URL: target, RetryAfter: retryAfter, Body: bodySnippet(body)}
			}
			if err := wait(ctx, retryAfter); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GET %s returned HTTP %d: %s", target, resp.StatusCode, bodySnippet(body))
		}
		c.limiter.OnSuccess()
		return body, nil
	}
	return nil, fmt.Errorf("GET %s failed", target)
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func bodySnippet(body []byte) string {
	const max = 512
	text := strings.TrimSpace(string(body))
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}
