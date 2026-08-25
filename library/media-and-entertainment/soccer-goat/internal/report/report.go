package report

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sort"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/eafc"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/espn"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/potential"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/store"
)

const potentialUnavailable = "unavailable: Cloudflare (set SOCCER_GOAT_FIFACM_COOKIE for potential)"
const potentialDatasetHint = "unavailable: run 'sync potential' to load the offline dataset, or 'auth login' for live"

// PotentialLooker is the subset of the local store the aggregator uses to fetch
// potential ratings. Kept as an interface so report stays testable without a
// live SQLite store.
type PotentialLooker interface {
	LookupPotential(ctx context.Context, eaID int, name string) (store.PotentialRow, bool, error)
}

type SourceStatus struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type PlayerReport struct {
	Query            string                  `json:"query"`
	Name             string                  `json:"name"`
	TMPlayerID       string                  `json:"tmPlayerId"`
	MarketValue      int64                   `json:"marketValue"`
	MarketValueLabel string                  `json:"marketValueLabel"`
	Club             string                  `json:"club"`
	Position         string                  `json:"position"`
	Foot             string                  `json:"foot"`
	Age              int                     `json:"age"`
	Nationality      string                  `json:"nationality"`
	EAOverall        int                     `json:"eaOverall"`
	Potential        int                     `json:"potential"`
	PotentialSource  string                  `json:"potentialSource"`
	Pace             int                     `json:"pace"`
	Shooting         int                     `json:"shooting"`
	Passing          int                     `json:"passing"`
	Dribbling        int                     `json:"dribbling"`
	Defending        int                     `json:"defending"`
	Physical         int                     `json:"physical"`
	Stats            map[string]int          `json:"stats"`
	EASlug           int                     `json:"eaSlug"`
	ESPN             *espn.Context           `json:"espn"`
	Sources          map[string]SourceStatus `json:"sources"`
}

type TeamReport struct {
	ClubName        string                  `json:"clubName"`
	TMClubID        string                  `json:"tmClubId"`
	SquadValue      int64                   `json:"squadValue"`
	SquadValueLabel string                  `json:"squadValueLabel"`
	Players         []PlayerReport          `json:"players"`
	Sources         map[string]SourceStatus `json:"sources"`
}

// FormatEuros renders a Transfermarkt euro value without locale dependence.
func FormatEuros(value int64) string {
	if value <= 0 {
		return "€0"
	}
	if value >= 1_000_000 {
		return fmt.Sprintf("€%.2fm", float64(value)/1_000_000)
	}
	if value >= 1_000 {
		if value%1_000 == 0 {
			return fmt.Sprintf("€%dk", value/1_000)
		}
		formatted := strconv.FormatFloat(float64(value)/1_000, 'f', 1, 64)
		return "€" + formatted + "k"
	}
	return fmt.Sprintf("€%d", value)
}

// ESPNResolver is the slice of the ESPN client the aggregator needs, expressed
// as an interface so report wiring can be unit-tested with a stub.
type ESPNResolver interface {
	Lookup(ctx context.Context, name string) (espn.Context, bool, error)
	Enrich(ctx context.Context, id string) (*espn.Enrichment, error)
}

type Aggregator struct {
	TM             *client.Client
	EA             *eafc.Client
	Pot            *potential.Client
	ESPN           ESPNResolver
	PotentialStore PotentialLooker
}

func NewAggregator(tm *client.Client) *Aggregator {
	return &Aggregator{
		TM:   tm,
		EA:   eafc.New(),
		Pot:  potential.New(),
		ESPN: espn.New(),
	}
}

// WithPotentialStore wires the local store as the primary (offline) potential
// source. Nil-safe: an aggregator without a store simply falls back to the live
// best-effort path.
func (a *Aggregator) WithPotentialStore(s PotentialLooker) *Aggregator {
	a.PotentialStore = s
	return a
}

func (a *Aggregator) ResolvePlayer(ctx context.Context, name string) (*PlayerReport, error) {
	if a == nil || a.TM == nil {
		return nil, fmt.Errorf("transfermarkt client is required")
	}
	path := "/players/search/" + url.PathEscape(name)
	raw, err := a.TM.Get(ctx, path, map[string]string{"page_number": "1"})
	if err != nil {
		return nil, fmt.Errorf("transfermarkt player search %q: %w", name, err)
	}
	var response tmPlayerSearchResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("transfermarkt player search %q: decode response: %w", name, err)
	}
	if len(response.Results) == 0 {
		return nil, fmt.Errorf("player not found: %s", name)
	}

	player := response.Results[0]
	report := newPlayerReport(name, string(player.ID), player.Name, player.Club.Name, player.Position, "", int(player.Age), firstName(player.Nationalities), int64(player.MarketValue))

	a.enrichEAAndPotential(ctx, report)
	if a.ESPN == nil {
		report.Sources["espn"] = SourceStatus{Detail: "unavailable: ESPN client not configured"}
	} else if context, ok, lookupErr := a.ESPN.Lookup(ctx, name); lookupErr != nil {
		report.Sources["espn"] = SourceStatus{Detail: "unavailable: " + lookupErr.Error()}
	} else if ok {
		// A failed enrich never demotes a successful resolve: context stays,
		// espn stays OK, only the stats/splits/recent block is absent.
		if enr, enrErr := a.ESPN.Enrich(ctx, context.AthleteID); enrErr == nil && enr != nil {
			context.Stats = enr.Stats
			context.Splits = enr.Splits
			context.RecentGames = enr.RecentGames
		}
		report.ESPN = &context
		report.Sources["espn"] = SourceStatus{OK: true}
	} else {
		report.Sources["espn"] = SourceStatus{Detail: "unavailable: no ESPN athlete result"}
	}
	return report, nil
}

func (a *Aggregator) ResolveTeam(ctx context.Context, clubName string) (*TeamReport, error) {
	if a == nil || a.TM == nil {
		return nil, fmt.Errorf("transfermarkt client is required")
	}
	searchPath := "/clubs/search/" + url.PathEscape(clubName)
	raw, err := a.TM.Get(ctx, searchPath, map[string]string{"page_number": "1"})
	if err != nil {
		return nil, fmt.Errorf("transfermarkt club search %q: %w", clubName, err)
	}
	var search tmClubSearchResponse
	if err := json.Unmarshal(raw, &search); err != nil {
		return nil, fmt.Errorf("transfermarkt club search %q: decode response: %w", clubName, err)
	}
	if len(search.Results) == 0 {
		return nil, fmt.Errorf("club not found: %s", clubName)
	}

	club := search.Results[0]
	rosterPath := "/clubs/" + url.PathEscape(string(club.ID)) + "/players"
	raw, err = a.TM.Get(ctx, rosterPath, nil)
	if err != nil {
		return nil, fmt.Errorf("transfermarkt club players %q: %w", club.Name, err)
	}
	roster, err := decodeRoster(raw)
	if err != nil {
		return nil, fmt.Errorf("transfermarkt club players %q: decode response: %w", club.Name, err)
	}

	team := &TeamReport{
		ClubName: club.Name,
		TMClubID: string(club.ID),
		Players:  make([]PlayerReport, 0, len(roster)),
		Sources:  make(map[string]SourceStatus, 4),
	}
	team.Sources["transfermarkt"] = SourceStatus{OK: true}
	for _, rosterPlayer := range roster {
		value := int64(rosterPlayer.MarketValue)
		team.SquadValue += value
		player := newPlayerReport(
			rosterPlayer.Name,
			string(rosterPlayer.ID),
			rosterPlayer.Name,
			club.Name,
			rosterPlayer.Position,
			rosterPlayer.Foot,
			int(rosterPlayer.Age),
			firstName(rosterPlayer.Nationality),
			value,
		)
		team.Players = append(team.Players, *player)
	}
	team.SquadValueLabel = FormatEuros(team.SquadValue)

	enrichCount := len(team.Players)
	if cliutil.IsDogfoodEnv() && enrichCount > 5 {
		enrichCount = 5
		for index := enrichCount; index < len(team.Players); index++ {
			team.Players[index].Sources["ea-fc"] = SourceStatus{Detail: "skipped: dogfood enrichment limit"}
			team.Players[index].Sources["potential"] = SourceStatus{Detail: "skipped: dogfood enrichment limit"}
		}
	}
	a.enrichTeamPlayers(ctx, team.Players[:enrichCount])

	eaOK, potentialOK := 0, 0
	for index := 0; index < enrichCount; index++ {
		if team.Players[index].Sources["ea-fc"].OK {
			eaOK++
		}
		if team.Players[index].Sources["potential"].OK {
			potentialOK++
		}
	}
	team.Sources["ea-fc"] = aggregateStatus(eaOK, enrichCount, len(team.Players), cliutil.IsDogfoodEnv())
	team.Sources["potential"] = aggregateStatus(potentialOK, enrichCount, len(team.Players), cliutil.IsDogfoodEnv())
	if potentialOK == 0 {
		status := team.Sources["potential"]
		status.Detail = potentialUnavailable
		team.Sources["potential"] = status
	}

	// ESPN gets its own budget: the existing dogfood cap above is IsDogfoodEnv()
	// gated and does not bound production runs, and ESPN is ~2 calls/player, so a
	// full squad would otherwise serialize dozens of calls. Enrich the top players
	// by market value and mark the rest skipped.
	if a.ESPN == nil {
		for index := range team.Players {
			team.Players[index].Sources["espn"] = SourceStatus{Detail: "unavailable: ESPN client not configured"}
		}
		team.Sources["espn"] = SourceStatus{Detail: "unavailable: ESPN client not configured"}
	} else {
		espnOK, espnAttempted := a.enrichTeamESPN(ctx, team.Players)
		detail := fmt.Sprintf("enriched %d/%d players", espnOK, espnAttempted)
		if espnAttempted < len(team.Players) {
			detail += fmt.Sprintf("; ESPN budget capped enrichment at %d/%d", espnAttempted, len(team.Players))
		}
		team.Sources["espn"] = SourceStatus{OK: espnOK > 0, Detail: detail}
	}
	return team, nil
}

// espnTeamBudget bounds how many players in a squad report get ESPN enrichment
// (top N by market value). ESPN is ~2 calls/player through a rate limiter, so an
// uncapped squad would add tens of seconds of latency.
const espnTeamBudget = 12

// enrichTeamESPN resolves + enriches ESPN for the top-value players (up to the
// budget), marking the rest "skipped: espn enrichment limit". Returns the number
// of successful enrichments and the number attempted.
func (a *Aggregator) enrichTeamESPN(ctx context.Context, players []PlayerReport) (okCount, attempted int) {
	if len(players) == 0 {
		return 0, 0
	}
	budget := espnEnrichIndices(players)
	inBudget := make(map[int]bool, len(budget))
	for _, index := range budget {
		inBudget[index] = true
	}
	for index := range players {
		if !inBudget[index] {
			players[index].Sources["espn"] = SourceStatus{Detail: "skipped: espn enrichment limit"}
		}
	}

	workers := len(budget)
	if workers > 6 {
		workers = 6
	}
	jobs := make(chan int)
	var group sync.WaitGroup
	var mu sync.Mutex
	group.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer group.Done()
			for index := range jobs {
				if a.resolveESPNPlayer(ctx, &players[index]) {
					mu.Lock()
					okCount++
					mu.Unlock()
				}
			}
		}()
	}
	for _, index := range budget {
		jobs <- index
	}
	close(jobs)
	group.Wait()
	return okCount, len(budget)
}

// resolveESPNPlayer resolves and enriches ESPN for a single player, setting its
// source status. Returns true on a successful resolve.
func (a *Aggregator) resolveESPNPlayer(ctx context.Context, player *PlayerReport) bool {
	context, ok, err := a.ESPN.Lookup(ctx, player.Name)
	if err != nil {
		player.Sources["espn"] = SourceStatus{Detail: "unavailable: " + err.Error()}
		return false
	}
	if !ok {
		player.Sources["espn"] = SourceStatus{Detail: "unavailable: no ESPN athlete result"}
		return false
	}
	if enr, enrErr := a.ESPN.Enrich(ctx, context.AthleteID); enrErr == nil && enr != nil {
		context.Stats = enr.Stats
		context.Splits = enr.Splits
		context.RecentGames = enr.RecentGames
	}
	player.ESPN = &context
	player.Sources["espn"] = SourceStatus{OK: true}
	return true
}

// espnEnrichIndices returns the indices of the highest-value players to enrich,
// capped at the budget (and further capped to 5 under dogfood for speed).
func espnEnrichIndices(players []PlayerReport) []int {
	indices := make([]int, len(players))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(a, b int) bool {
		return players[indices[a]].MarketValue > players[indices[b]].MarketValue
	})
	limit := espnTeamBudget
	if cliutil.IsDogfoodEnv() && limit > 5 {
		limit = 5
	}
	if limit > len(indices) {
		limit = len(indices)
	}
	return indices[:limit]
}

func (a *Aggregator) enrichTeamPlayers(ctx context.Context, players []PlayerReport) {
	if len(players) == 0 {
		return
	}
	workers := len(players)
	if workers > 6 {
		workers = 6
	}
	jobs := make(chan int)
	var group sync.WaitGroup
	group.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer group.Done()
			for index := range jobs {
				a.enrichEAAndPotential(ctx, &players[index])
			}
		}()
	}
	for index := range players {
		jobs <- index
	}
	close(jobs)
	group.Wait()
}

func (a *Aggregator) enrichEAAndPotential(ctx context.Context, report *PlayerReport) {
	if a.EA == nil {
		report.Sources["ea-fc"] = SourceStatus{Detail: "unavailable: EA FC client not configured"}
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	player, ok, err := a.EA.Best(ctx, report.Name)
	if err != nil {
		report.Sources["ea-fc"] = SourceStatus{Detail: "unavailable: " + err.Error()}
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	if !ok {
		report.Sources["ea-fc"] = SourceStatus{Detail: "unavailable: no EA FC player match"}
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	// EA is searched by name only, so for common or duplicated names the first
	// hit can be a different player. Verify the EA match against the identity we
	// already trust from Transfermarkt (club, then nationality) before merging
	// its rating/potential — otherwise team boards and divergence rankings could
	// combine one player's TM value with another player's EA rating. When we have
	// no TM signal to compare, accept the match (best-effort); the TM value is
	// correct regardless.
	if detail, consistent := eaMatchConsistent(report, player); !consistent {
		report.Sources["ea-fc"] = SourceStatus{Detail: detail}
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	report.EAOverall = player.Overall
	report.Pace = player.Pace
	report.Shooting = player.Shooting
	report.Passing = player.Passing
	report.Dribbling = player.Dribbling
	report.Defending = player.Defending
	report.Physical = player.Physical
	report.Stats = cloneStats(player.Stats)
	report.EASlug = player.ID
	report.Sources["ea-fc"] = SourceStatus{OK: true}

	// Potential, tier 1: the bundled/synced dataset (offline, primary). Joined on
	// the EA id, which the dataset shares exactly, with a normalized-name
	// fallback. This is the path that makes potential populate by default.
	if a.PotentialStore != nil {
		if row, ok, err := a.PotentialStore.LookupPotential(ctx, report.EASlug, report.Name); err == nil && ok {
			report.Potential = row.Potential
			report.PotentialSource = row.Source
			report.Sources["potential"] = SourceStatus{OK: true}
			return
		}
	}

	// Potential, tier 2: live best-effort (Cloudflare) fallback for players the
	// dataset missed, only when a clearance cookie is configured.
	if a.Pot != nil && report.EASlug > 0 {
		if rating, ratingOK, _ := a.Pot.ByEAID(ctx, report.EASlug); ratingOK {
			report.Potential = rating.Potential
			report.PotentialSource = rating.Source
			report.Sources["potential"] = SourceStatus{OK: true}
			return
		}
	}
	report.Sources["potential"] = SourceStatus{Detail: potentialDatasetHint}
}

// clubNoiseTokens are common club-name affixes that carry no identifying
// signal, so they are dropped before comparing a Transfermarkt club against an
// EA team label ("SL Benfica" vs "Benfica", "Real Madrid CF" vs "Real Madrid").
var clubNoiseTokens = map[string]bool{
	"fc": true, "cf": true, "sc": true, "ac": true, "afc": true, "cd": true,
	"sl": true, "ss": true, "us": true, "rc": true, "sv": true, "ud": true,
	"club": true, "de": true, "the": true, "b": true, "ii": true, "1": true,
}

// clubTokens normalizes a club/team name to its significant lowercase tokens.
func clubTokens(name string) map[string]bool {
	tokens := make(map[string]bool)
	for _, raw := range strings.Fields(strings.ToLower(name)) {
		cleaned := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, raw)
		if cleaned == "" || clubNoiseTokens[cleaned] {
			continue
		}
		tokens[cleaned] = true
	}
	return tokens
}

// eaMatchConsistent decides whether an EA search hit plausibly refers to the
// same player the Transfermarkt spine resolved. It returns (detail, false) with
// a human-readable reason when the match should be rejected.
//
// EA is searched by name only, so the club is the one strong discriminator we
// have for telling a real match from a namesake. The guard therefore requires a
// positive club agreement: both sides must name a club AND their significant
// tokens must overlap (after dropping affix noise like SL/FC/CF). Any weaker
// state is rejected:
//   - clubs disagree -> almost certainly a different player who shares the name;
//   - either club missing -> we cannot verify, so we refuse to merge on the name
//     alone rather than risk attaching a namesake's rating.
//
// Nationality is deliberately not consulted (too many players share one, so
// "same country, different club" is exactly the collision this guard catches).
// On any rejection the report keeps its correct Transfermarkt market value and
// simply marks the best-effort EA rating/potential "unavailable" — the safe
// direction. In practice both sources populate the club on virtually every
// active player, so this rarely costs coverage; the residual cost (a real
// mid-cycle transfer, or a club-less free agent) is an honest "unavailable"
// instead of a silent mismerge.
func eaMatchConsistent(report *PlayerReport, player *eafc.Player) (string, bool) {
	tmTokens := clubTokens(report.Club)
	eaTokens := clubTokens(player.Team)
	if len(tmTokens) == 0 || len(eaTokens) == 0 {
		return fmt.Sprintf("unavailable: unverifiable name match (missing club to confirm EA team %q against TM club %q)", player.Team, report.Club), false
	}
	clubAgrees := false
	for token := range tmTokens {
		if eaTokens[token] {
			clubAgrees = true
			break
		}
	}
	if !clubAgrees {
		return fmt.Sprintf("unavailable: ambiguous name match (EA team %q vs TM club %q)", player.Team, report.Club), false
	}
	// Club is the same, but a same-club teammate can still be returned by EA's
	// name search (two similarly-named players at one club). Require a second
	// positive signal: the EA hit's own name must share a token with the
	// Transfermarkt name. EA is searched by that name, so a correct hit almost
	// always agrees; a fuzzy same-club namesake (different surname) does not.
	if !nameAffirms(report.Name, player) {
		return fmt.Sprintf("unavailable: same-club name mismatch (EA %q vs TM %q at %q)", player.DisplayName(), report.Name, report.Club), false
	}
	return "", true
}

// nameTokens splits a personal name into significant lowercase, diacritic-folded
// tokens (dropping one/two-letter fragments like initials). It reuses the
// store's normalization so folding matches the potential-lookup path.
func nameTokens(s string) []string {
	out := make([]string, 0, 4)
	for _, tok := range strings.Fields(store.NormalizePotentialName(s)) {
		if len(tok) >= 3 {
			out = append(out, tok)
		}
	}
	return out
}

// tokenAffirms reports whether tok matches any token in set, either exactly or
// prefix-tolerantly (the shorter token, >=4 chars, is a prefix of the longer) so
// short/nick forms still match ("rodri" affirms "rodrigo").
func tokenAffirms(tok string, set []string) bool {
	for _, s := range set {
		if tok == s {
			return true
		}
		if len(tok) >= 4 && strings.HasPrefix(s, tok) {
			return true
		}
		if len(s) >= 4 && strings.HasPrefix(tok, s) {
			return true
		}
	}
	return false
}

// nameAffirms reports whether the EA player's name and the Transfermarkt name
// agree on the *surname*, not merely a shared first name. A shared first name
// alone ("João Silva" vs same-club "João Santos") is exactly the collision this
// rejects; the discriminator is the last (surname) token, checked in both
// directions and prefix-tolerantly so nick/short forms still match.
func nameAffirms(tmName string, player *eafc.Player) bool {
	tm := nameTokens(tmName)
	ea := nameTokens(player.DisplayName() + " " + player.FirstName + " " + player.LastName + " " + player.CommonName)
	if len(tm) == 0 || len(ea) == 0 {
		return false
	}
	return tokenAffirms(tm[len(tm)-1], ea) || tokenAffirms(ea[len(ea)-1], tm)
}

func newPlayerReport(query, id, name, club, position, foot string, age int, nationality string, value int64) *PlayerReport {
	return &PlayerReport{
		Query:            query,
		Name:             name,
		TMPlayerID:       id,
		MarketValue:      value,
		MarketValueLabel: FormatEuros(value),
		Club:             club,
		Position:         position,
		Foot:             foot,
		Age:              age,
		Nationality:      nationality,
		Stats:            make(map[string]int),
		Sources: map[string]SourceStatus{
			"transfermarkt": {OK: true},
			"ea-fc":         {Detail: "unavailable: not attempted"},
			"potential":     {Detail: potentialUnavailable},
			"espn":          {Detail: "unavailable: not attempted"},
		},
	}
}

func cloneStats(stats map[string]int) map[string]int {
	cloned := make(map[string]int, len(stats))
	for key, value := range stats {
		cloned[key] = value
	}
	return cloned
}

func aggregateStatus(successes, attempted, total int, dogfood bool) SourceStatus {
	detail := fmt.Sprintf("enriched %d/%d players", successes, attempted)
	if dogfood && attempted < total {
		detail += fmt.Sprintf("; dogfood capped enrichment at %d/%d", attempted, total)
	}
	return SourceStatus{OK: successes > 0, Detail: detail}
}

type flexibleString string
type flexibleInt int
type marketValue int64
type nameList []string

func (value *flexibleString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*value = ""
		return nil
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		*value = flexibleString(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*value = flexibleString(number.String())
	return nil
}

func (value *flexibleInt) UnmarshalJSON(data []byte) error {
	parsed, err := parseInt64(data)
	*value = flexibleInt(parsed)
	return err
}

func (value *marketValue) UnmarshalJSON(data []byte) error {
	parsed, err := parseMarketValue(data)
	*value = marketValue(parsed)
	return err
}

func (names *nameList) UnmarshalJSON(data []byte) error {
	var stringsList []string
	if err := json.Unmarshal(data, &stringsList); err == nil {
		*names = nameList(stringsList)
		return nil
	}
	var objects []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &objects); err != nil {
		return err
	}
	result := make([]string, 0, len(objects))
	for _, object := range objects {
		if object.Name != "" {
			result = append(result, object.Name)
		}
	}
	*names = nameList(result)
	return nil
}

func firstName(names nameList) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func parseInt64(data []byte) (int64, error) {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "" || text == "null" {
		return 0, nil
	}
	if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
		return integer, nil
	}
	float, err := strconv.ParseFloat(text, 64)
	return int64(float), err
}

func parseMarketValue(data []byte) (int64, error) {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "" || text == "null" || text == "-" {
		return 0, nil
	}
	if value, err := strconv.ParseInt(text, 10, 64); err == nil {
		return value, nil
	}
	normalized := strings.ToLower(strings.NewReplacer("€", "", ",", "", " ", "").Replace(text))
	multiplier := float64(1)
	switch {
	case strings.HasSuffix(normalized, "bn"):
		multiplier = 1_000_000_000
		normalized = strings.TrimSuffix(normalized, "bn")
	case strings.HasSuffix(normalized, "b"):
		multiplier = 1_000_000_000
		normalized = strings.TrimSuffix(normalized, "b")
	case strings.HasSuffix(normalized, "m"):
		multiplier = 1_000_000
		normalized = strings.TrimSuffix(normalized, "m")
	case strings.HasSuffix(normalized, "k"):
		multiplier = 1_000
		normalized = strings.TrimSuffix(normalized, "k")
	}
	number, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, err
	}
	return int64(number * multiplier), nil
}

type tmPlayerSearchResponse struct {
	Results []tmSearchPlayer `json:"results"`
}

type tmSearchPlayer struct {
	ID   flexibleString `json:"id"`
	Name string         `json:"name"`
	Club struct {
		Name string `json:"name"`
	} `json:"club"`
	Position      string      `json:"position"`
	Age           flexibleInt `json:"age"`
	Nationalities nameList    `json:"nationalities"`
	MarketValue   marketValue `json:"marketValue"`
}

type tmClubSearchResponse struct {
	Results []tmClubResult `json:"results"`
}

type tmClubResult struct {
	ID   flexibleString `json:"id"`
	Name string         `json:"name"`
}

type tmRosterPlayer struct {
	ID          flexibleString `json:"id"`
	Name        string         `json:"name"`
	Position    string         `json:"position"`
	Age         flexibleInt    `json:"age"`
	Nationality nameList       `json:"nationality"`
	Foot        string         `json:"foot"`
	MarketValue marketValue    `json:"marketValue"`
}

func decodeRoster(data []byte) ([]tmRosterPlayer, error) {
	players := make([]tmRosterPlayer, 0)
	if err := json.Unmarshal(data, &players); err == nil {
		if players == nil {
			players = make([]tmRosterPlayer, 0)
		}
		return players, nil
	}
	var wrapped struct {
		Players []tmRosterPlayer `json:"players"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return players, err
	}
	if wrapped.Players == nil {
		wrapped.Players = make([]tmRosterPlayer, 0)
	}
	return wrapped.Players, nil
}
