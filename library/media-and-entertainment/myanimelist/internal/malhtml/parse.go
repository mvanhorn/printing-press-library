// Package malhtml parses MyAnimeList's server-rendered HTML into typed records.
//
// MyAnimeList publishes no anonymous JSON API for most of its data. Title pages,
// statistics, episode tables, cast lists, seasonal charts, rankings, and person
// pages are server-rendered HTML, so this package is the single place where the
// site's markup idioms are understood:
//
//   - label/value pairs anchored on <span class="dark_text">Label:</span>
//   - the score-stats table (class="score-label score-N" + "(N votes)")
//   - episode rows carrying data-sort-key="episode-aired"
//   - related-entry tiles under <h2 id="related_entries">
//   - ranking rows (tr.ranking-list) and seasonal tiles (js-anime-category-producer)
//
// Every parser is anchored to those idioms and returns an error instead of a
// silently half-populated record when the expected structure is absent. That
// matters because MyAnimeList answers unknown sub-routes with the *base* page
// and an HTTP 200, so "did we parse anything?" is the only reliable signal that
// the right document was fetched.
//
// Handlers: a MAL redesign is the known failure mode for website-derived
// tooling (the archived ryukinix/mal CLI died exactly this way). Keep parser
// fixtures in parse_test.go current, and prefer adding a new anchored pattern
// over loosening an existing one.
package malhtml

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	tagRE       = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]*>`)
	hiddenRE    = regexp.MustCompile(`(?s)<span[^>]*style="[^"]*display:\s*none[^"]*"[^>]*>.*?</span>`)
	spaceRE     = regexp.MustCompile(`[\s\x{00a0}]+`)
	commaRE     = regexp.MustCompile(`\s*,\s*`)
	titleTagRE  = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	h1RE        = regexp.MustCompile(`(?s)<h1[^>]*class="[^"]*h1[^"]*"[^>]*>(.*?)</h1>`)
	h1AnyRE     = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	scoreLabRE  = regexp.MustCompile(`class="score-label"[^>]*>\s*([0-9.]+)`)
	numRE       = regexp.MustCompile(`[0-9][0-9,]*`)
	genreLinkRE = regexp.MustCompile(`href="/anime/genre/\d+/[^"]*"[^>]*>([^<]+)<`)
	prodLinkRE  = regexp.MustCompile(`/anime/producer/\d+/[^"]*"[^>]*>([^<]+)<`)
)

// textField returns the text of the value rendered after a dark_text label.
// MyAnimeList wraps those pairs in <div class="spaceit_pad">, so the value ends
// at the closing </div>.
func textField(page, label string) string {
	re, err := regexp.Compile(`(?s)<span class="dark_text">` + regexp.QuoteMeta(label) + `</span>(.*?)</div>`)
	if err != nil {
		return ""
	}
	m := re.FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return cleanText(m[1])
}

// cleanText strips tags and collapses whitespace on an extracted fragment.
func cleanText(s string) string {
	s = hiddenRE.ReplaceAllString(s, "")
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(strings.Trim(s, "|"))
}

// flatText returns the whole document as one collapsed text line, used for
// rank/popularity/member style values that are not div-anchored.
func flatText(page string) string {
	return cleanText(page)
}

// firstNumber pulls the first integer out of a fragment ("#50" -> 50).
func firstNumber(s string) int {
	m := numRE.FindString(s)
	if m == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m, ",", ""))
	if err != nil {
		return 0
	}
	return n
}

func firstFloat(s string) float64 {
	m := regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`).FindString(s)
	if m == "" {
		return 0
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0
	}
	return f
}

// splitList splits a "a, b, c" list field into trimmed items.
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" || s == "-" || s == "None" {
		return nil
	}
	parts := commaRE.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && p != "-" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Related is one entry in a title's Related Entries table.
type Related struct {
	Relation string `json:"relation"`
	Kind     string `json:"kind"`
	ID       int    `json:"id"`
	Title    string `json:"title"`
}

// Detail is a parsed anime or manga detail page.
type Detail struct {
	Kind          string    `json:"kind"`
	ID            int       `json:"id"`
	Title         string    `json:"title,omitempty"`
	TitleJapanese string    `json:"title_japanese,omitempty"`
	TitleEnglish  string    `json:"title_english,omitempty"`
	Synonyms      string    `json:"synonyms,omitempty"`
	Type          string    `json:"type,omitempty"`
	Episodes      int       `json:"episodes,omitempty"`
	Volumes       int       `json:"volumes,omitempty"`
	Chapters      int       `json:"chapters,omitempty"`
	Status        string    `json:"status,omitempty"`
	Aired         string    `json:"aired,omitempty"`
	Published     string    `json:"published,omitempty"`
	Premiered     string    `json:"premiered,omitempty"`
	Broadcast     string    `json:"broadcast,omitempty"`
	Producers     []string  `json:"producers,omitempty"`
	Licensors     []string  `json:"licensors,omitempty"`
	Studios       []string  `json:"studios,omitempty"`
	Source        string    `json:"source,omitempty"`
	Genres        []string  `json:"genres,omitempty"`
	Themes        []string  `json:"themes,omitempty"`
	Demographics  []string  `json:"demographics,omitempty"`
	Duration      string    `json:"duration,omitempty"`
	Rating        string    `json:"rating,omitempty"`
	Score         float64   `json:"score,omitempty"`
	ScoredBy      int       `json:"scored_by,omitempty"`
	Rank          int       `json:"rank,omitempty"`
	Popularity    int       `json:"popularity,omitempty"`
	Members       int       `json:"members,omitempty"`
	Favorites     int       `json:"favorites,omitempty"`
	Synopsis      string    `json:"synopsis,omitempty"`
	Background    string    `json:"background,omitempty"`
	ExternalLinks []string  `json:"external_links,omitempty"`
	Related       []Related `json:"related,omitempty"`
}

var relatedEntryRE = regexp.MustCompile(`(?s)<div class="entry[^"]*">(.*?)</div>\s*</div>\s*</div>`)
var relatedLinkRE = regexp.MustCompile(`href="https://myanimelist\.net/(anime|manga)/(\d+)/[^"]*"[^>]*>([^<]*)<`)
var relatedRelRE = regexp.MustCompile(`<span(?: class="[^"]*")?\s*>([A-Za-z][A-Za-z /-]{2,30})</span>`)

// ParseDetail parses an anime or manga detail page.
func ParseDetail(kind string, id int, page string) (*Detail, error) {
	d := &Detail{Kind: kind, ID: id}
	if m := h1RE.FindStringSubmatch(page); m != nil {
		d.Title = cleanText(m[1])
	}
	if d.Title == "" {
		if m := titleTagRE.FindStringSubmatch(page); m != nil {
			d.Title = stripSiteSuffix(cleanText(m[1]))
		}
	}
	if d.Title == "" {
		if m := h1AnyRE.FindStringSubmatch(page); m != nil {
			d.Title = cleanText(m[1])
		}
	}
	if d.Title == "" {
		return nil, fmt.Errorf("no title found in %s page %d: the document is not a title page (MyAnimeList serves the base page for unknown sub-routes, so check the fetched URL)", kind, id)
	}
	d.TitleJapanese = textField(page, "Japanese:")
	d.TitleEnglish = textField(page, "English:")
	d.Synonyms = textField(page, "Synonyms:")
	d.Type = textField(page, "Type:")
	d.Status = textField(page, "Status:")
	d.Aired = textField(page, "Aired:")
	d.Published = textField(page, "Published:")
	d.Premiered = textField(page, "Premiered:")
	d.Broadcast = textField(page, "Broadcast:")
	d.Source = textField(page, "Source:")
	d.Duration = textField(page, "Duration:")
	d.Rating = textField(page, "Rating:")
	d.Episodes = firstNumber(textField(page, "Episodes:"))
	d.Volumes = firstNumber(textField(page, "Volumes:"))
	d.Chapters = firstNumber(textField(page, "Chapters:"))
	d.Genres = splitList(textField(page, "Genres:"))
	d.Themes = splitList(textField(page, "Themes:"))
	d.Demographics = splitList(textField(page, "Demographic:"))
	if len(d.Demographics) == 0 {
		d.Demographics = splitList(textField(page, "Demographics:"))
	}
	d.Producers = splitList(textField(page, "Producers:"))
	d.Licensors = splitList(textField(page, "Licensors:"))
	d.Studios = splitList(textField(page, "Studios:"))
	d.Score = firstFloat(textField(page, "Score:"))
	if d.Score == 0 {
		if m := scoreLabRE.FindStringSubmatch(page); m != nil {
			d.Score, _ = strconv.ParseFloat(m[1], 64)
		}
	}
	flat := flatText(page)
	d.ScoredBy = firstNumber(matchAfter(flat, "scored by"))
	d.Rank = firstNumber(matchAfter(flat, "Ranked:"))
	d.Popularity = firstNumber(matchAfter(flat, "Popularity:"))
	d.Members = firstNumber(matchAfter(flat, "Members "))
	if d.Members == 0 {
		d.Members = firstNumber(matchAfter(flat, "Members:"))
	}
	d.Favorites = firstNumber(matchAfter(flat, "Favorites:"))
	if m := regexp.MustCompile(`(?s)<p itemprop="description">(.*?)</p>`).FindStringSubmatch(page); m != nil {
		d.Synopsis = cleanText(m[1])
	}
	if m := regexp.MustCompile(`(?s)<span itemprop="description">(.*?)</span>`).FindStringSubmatch(page); m != nil && d.Synopsis == "" {
		d.Synopsis = cleanText(m[1])
	}
	d.Related = parseRelated(page)
	d.ExternalLinks = parseExternalLinks(page)
	return d, nil
}

// stripSiteSuffix removes the site's own title decorations so the fallback
// <title> parse yields the work's name only.
func stripSiteSuffix(t string) string {
	for _, suffix := range []string{" - MyAnimeList.net", " - MyAnimeList", " | Manga", " | Anime"} {
		t = strings.TrimSuffix(t, suffix)
	}
	return strings.TrimSpace(t)
}

// matchAfter returns a bounded window of flat text following a marker so
// firstNumber picks up that marker's value rather than an earlier one.
func matchAfter(flat, marker string) string {
	i := strings.Index(flat, marker)
	if i < 0 {
		return ""
	}
	end := i + len(marker) + 40
	if end > len(flat) {
		end = len(flat)
	}
	return flat[i+len(marker) : end]
}

// parseRelated walks the Related Entries tiles.
func parseRelated(page string) []Related {
	start := strings.Index(page, `id="related_entries"`)
	if start < 0 {
		return nil
	}
	section := page[start:]
	if end := strings.Index(section, `id="`); end > 0 {
		section = section[:end]
	}
	blocks := strings.Split(section, `<div class="entry`)
	out := make([]Related, 0, 4)
	seenRel := map[int]bool{}
	for _, b := range blocks[1:] {
		var kind, title string
		id := 0
		for _, m := range relatedLinkRE.FindAllStringSubmatch(b, -1) {
			if strings.TrimSpace(m[3]) == "" {
				continue
			}
			id, _ = strconv.Atoi(m[2])
			kind, title = m[1], cleanText(m[3])
			break
		}
		if id == 0 {
			continue
		}
		if seenRel[id] {
			continue
		}
		seenRel[id] = true
		out = append(out, Related{Relation: relationLabel(b), Kind: kind, ID: id, Title: title})
	}
	// Table form: <td class="ar fw-n">Sequel:</td> ... <a href="/anime/9135/...">
	for _, row := range entriesRowRE.FindAllStringSubmatch(section, -1) {
		rel := ""
		if lm := entriesLabelRE.FindStringSubmatch(row[1]); lm != nil {
			rel = titleCase(strings.TrimSpace(lm[1]))
		} else if lm := relatedRelRE.FindStringSubmatch(row[1]); lm != nil {
			rel = titleCase(strings.TrimSpace(strings.TrimSuffix(lm[1], ":")))
		}
		for _, m := range relatedLinkRE.FindAllStringSubmatch(row[1], -1) {
			if strings.TrimSpace(m[3]) == "" {
				continue
			}
			id, err := strconv.Atoi(m[2])
			if err != nil || seenRel[id] {
				continue
			}
			seenRel[id] = true
			if !malRelationLabels[strings.ToLower(rel)] {
				rel = ""
			}
			out = append(out, Related{Relation: rel, Kind: m[1], ID: id, Title: cleanText(m[3])})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var extLinkRE = regexp.MustCompile(`href="(https?://[^"]+)"[^>]*target="_blank"`)

// malRelationLabels is MyAnimeList's fixed relation vocabulary. Matching only
// these keeps UI text such as "View All" out of the relation column.
var malRelationLabels = map[string]bool{
	"adaptation": true, "alternative setting": true, "alternative version": true,
	"character": true, "full story": true, "other": true, "parent story": true,
	"prequel": true, "sequel": true, "side story": true, "spin-off": true,
	"summary": true,
}

// relationDivRE matches the container MyAnimeList renders the relation inside:
// <div class="relation">Sequel (TV)</div>.
var relationDivRE = regexp.MustCompile(`(?s)<div class="relation">(.*?)</div>`)

// entriesRowRE splits the table form of the related-entries block, whose rows
// carry the relation in a <td class="ar fw-n"> cell.
var entriesRowRE = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)

var entriesLabelRE = regexp.MustCompile(`<td class="ar fw-n">\s*([A-Za-z][A-Za-z /-]{2,30}?):?\s*</td>`)

// relationLabel finds the relation label inside one related-entry tile. The
// label carries the media type in parentheses ("Sequel (TV)"), which is not
// part of the relation.
func relationLabel(tile string) string {
	if m := relationDivRE.FindStringSubmatch(tile); m != nil {
		label := cleanText(m[1])
		if i := strings.Index(label, "("); i > 0 {
			label = strings.TrimSpace(label[:i])
		}
		if malRelationLabels[strings.ToLower(label)] {
			return titleCase(label)
		}
	}
	for _, m := range relatedRelRE.FindAllStringSubmatch(tile, -1) {
		candidate := strings.ToLower(strings.TrimSpace(m[1]))
		if malRelationLabels[candidate] {
			return titleCase(strings.TrimSpace(candidate))
		}
	}
	return ""
}

func parseExternalLinks(page string) []string {
	start := strings.Index(page, `id="external_links"`)
	if start < 0 {
		return nil
	}
	section := page[start:]
	if end := strings.Index(section, `id="`); end > 0 {
		section = section[:end]
	}
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	for _, m := range extLinkRE.FindAllStringSubmatch(section, -1) {
		u := m[1]
		if strings.Contains(u, "myanimelist.net") || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Bucket is one score-distribution row.
type Bucket struct {
	Score   int     `json:"score"`
	Percent float64 `json:"percent"`
	Votes   int     `json:"votes"`
}

// Stats is a parsed statistics page.
type Stats struct {
	Kind        string   `json:"kind"`
	ID          int      `json:"id"`
	Title       string   `json:"title,omitempty"`
	Score       float64  `json:"score,omitempty"`
	ScoredBy    int      `json:"scored_by,omitempty"`
	Buckets     []Bucket `json:"score_distribution,omitempty"`
	Watching    int      `json:"watching,omitempty"`
	Completed   int      `json:"completed,omitempty"`
	OnHold      int      `json:"on_hold,omitempty"`
	Dropped     int      `json:"dropped,omitempty"`
	PlanToWatch int      `json:"plan_to_watch,omitempty"`
	Total       int      `json:"total,omitempty"`
}

var bucketRE = regexp.MustCompile(`(?s)class="score-label score-(\d+)"[^>]*>\s*(\d+)\s*</td>.*?([0-9.]+)%;?.*?\(([0-9,]+)\s*votes?\)`)

// ParseStats parses an anime or manga statistics page.
func ParseStats(kind string, id int, page string) (*Stats, error) {
	s := &Stats{Kind: kind, ID: id}
	if m := h1RE.FindStringSubmatch(page); m != nil {
		s.Title = cleanText(m[1])
	}
	if s.Title == "" {
		if m := titleTagRE.FindStringSubmatch(page); m != nil {
			s.Title = stripSiteSuffix(cleanText(m[1]))
		}
	}
	for _, m := range bucketRE.FindAllStringSubmatch(page, -1) {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, fmt.Errorf("statistics page for %s %d has an unreadable score row (%q): the markup may have changed", kind, id, m[0][:min(len(m[0]), 40)])
		}
		pct, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			return nil, fmt.Errorf("statistics page for %s %d has an unreadable percentage (%q)", kind, id, m[3])
		}
		s.Buckets = append(s.Buckets, Bucket{Score: n, Percent: pct, Votes: firstNumber(m[4])})
	}
	s.Watching = firstNumber(textField(page, "Watching:"))
	s.Completed = firstNumber(textField(page, "Completed:"))
	s.OnHold = firstNumber(textField(page, "On-Hold:"))
	s.Dropped = firstNumber(textField(page, "Dropped:"))
	s.PlanToWatch = firstNumber(textField(page, "Plan to Watch:"))
	s.Total = firstNumber(textField(page, "Total:"))
	s.Score = firstFloat(textField(page, "Score:"))
	s.ScoredBy = firstNumber(matchAfter(flatText(page), "scored by"))
	if len(s.Buckets) == 0 && s.Total == 0 {
		return nil, fmt.Errorf("no statistics found in %s page %d: MyAnimeList returns the base title page for an unknown stats route, so check the URL uses the three-segment form /%s/{id}/_/stats", kind, id, kind)
	}
	return s, nil
}

// Episode is one row of a title's episode table.
type Episode struct {
	Number        int     `json:"number"`
	Title         string  `json:"title,omitempty"`
	TitleJapanese string  `json:"title_japanese,omitempty"`
	Aired         string  `json:"aired,omitempty"`
	PollAverage   float64 `json:"poll_average,omitempty"`
	Replies       int     `json:"replies,omitempty"`
	URL           string  `json:"url,omitempty"`
}

var (
	epRowRE   = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	epLinkRE  = regexp.MustCompile(`href="(https://myanimelist\.net/anime/\d+/[^"]*/episode/(\d+))"`)
	epAiredRE = regexp.MustCompile(`([A-Z][a-z]{2} \d{1,2}, \d{4})`)
	epPollRE  = regexp.MustCompile(`average\s*</?[^>]*>?\s*([0-9.]+)`)
	epJpRE    = regexp.MustCompile(`\(([^()]*[\p{Han}\p{Hiragana}\p{Katakana}][^()]*)\)`)
)

// ParseEpisodes parses a title's episode table.
func ParseEpisodes(id int, page string) ([]Episode, error) {
	if !strings.Contains(page, "episode-aired") && !strings.Contains(page, "/episode/") {
		return nil, fmt.Errorf("no episode table found for anime %d: MyAnimeList serves the base title page for an unknown episode route, so check the URL uses /anime/{id}/_/episode", id)
	}
	out := make([]Episode, 0, 24)
	seen := map[int]bool{}
	for _, row := range epRowRE.FindAllStringSubmatch(page, -1) {
		m := epLinkRE.FindStringSubmatch(row[1])
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		if n == 0 || seen[n] {
			continue
		}
		seen[n] = true
		text := cleanText(row[1])
		ep := Episode{Number: n, URL: m[1]}
		if a := epAiredRE.FindString(text); a != "" {
			ep.Aired = a
		}
		if p := epPollRE.FindStringSubmatch(row[1]); p != nil {
			ep.PollAverage, _ = strconv.ParseFloat(p[1], 64)
		}
		if p := epPollRE.FindStringSubmatch(text); p == nil && ep.PollAverage == 0 {
			if v := regexp.MustCompile(`average ([0-9.]+)`).FindStringSubmatch(text); v != nil {
				ep.PollAverage, _ = strconv.ParseFloat(v[1], 64)
			}
		}
		if jp := epJpRE.FindStringSubmatch(text); jp != nil {
			ep.TitleJapanese = strings.TrimSpace(jp[1])
		}
		// The English title is the text between the episode number and the
		// Japanese title; it is the longest alphabetic run in the row.
		parts := strings.Split(text, "|")
		best := ""
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if len(p) > len(best) && strings.IndexFunc(p, func(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' }) >= 0 && !strings.Contains(p, "Vote") {
				best = p
			}
		}
		ep.Title = best
		if nums := regexp.MustCompile(`([0-9]{2,5})\s*$`).FindStringSubmatch(text); nums != nil {
			ep.Replies = firstNumber(nums[1])
		}
		out = append(out, ep)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("episode table for anime %d parsed zero rows; the markup may have changed", id)
	}
	return out, nil
}

// VA is a voice actor credited for a character.
type VA struct {
	PersonID int    `json:"person_id"`
	Person   string `json:"person"`
	Language string `json:"language"`
}

// CharacterRole is one row of a Characters & Staff page's character table.
type CharacterRole struct {
	CharacterID int    `json:"character_id"`
	Character   string `json:"character"`
	Role        string `json:"role,omitempty"`
	VAs         []VA   `json:"voice_actors,omitempty"`
}

// StaffCredit is one row of the staff table on the same page.
type StaffCredit struct {
	PersonID int    `json:"person_id"`
	Person   string `json:"person"`
	Role     string `json:"role,omitempty"`
}

var (
	charLinkRE = regexp.MustCompile(`href="(?:https://myanimelist\.net)?/character/(\d+)/[^"]*"[^>]*>([^<]+)<`)
	persLinkRE = regexp.MustCompile(`href="(?:https://myanimelist\.net)?/people/(\d+)/[^"]*"[^>]*>([^<]+)<`)
	staffTabRE = regexp.MustCompile(`(?s)<h2[^>]*>\s*Staff\s*</h2>(.*)$`)
	roleRE     = regexp.MustCompile(`(?i)^(main|supporting|background)$`)
)

// titleCase upper-cases the first letter of an enum-ish label.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// ParseCharacters parses a "Characters & Staff" page into its two tables.
func ParseCharacters(page string) ([]CharacterRole, []StaffCredit, error) {
	if !strings.Contains(page, "/character/") {
		return nil, nil, fmt.Errorf("no character table found: MyAnimeList serves the base title page for an unknown sub-route, so check the URL uses /{kind}/{id}/_/characters")
	}
	charSection := page
	staffSection := ""
	if m := staffTabRE.FindStringSubmatchIndex(page); m != nil {
		charSection = page[:m[0]]
		staffSection = page[m[0]:]
	}
	chars := make([]CharacterRole, 0, 12)
	for _, tbl := range strings.Split(charSection, "<table") {
		cm := charLinkRE.FindStringSubmatch(tbl)
		if cm == nil {
			continue
		}
		cid, _ := strconv.Atoi(cm[1])
		role := ""
		if rm := relatedRelRE.FindStringSubmatch(tbl); rm != nil {
			if roleRE.MatchString(strings.TrimSpace(rm[1])) {
				role = titleCase(strings.TrimSpace(rm[1]))
			}
		}
		cr := CharacterRole{CharacterID: cid, Character: cleanText(cm[2]), Role: role}
		vms := persLinkRE.FindAllStringSubmatch(tbl, -1)
		for i, vm := range vms {
			pid, _ := strconv.Atoi(vm[1])
			lang := "Japanese"
			if i > 0 {
				lang = "English"
			}
			cr.VAs = append(cr.VAs, VA{PersonID: pid, Person: cleanText(vm[2]), Language: lang})
		}
		chars = append(chars, cr)
	}
	staff := make([]StaffCredit, 0, 8)
	for _, tbl := range strings.Split(staffSection, "<table") {
		pm := persLinkRE.FindStringSubmatch(tbl)
		if pm == nil {
			continue
		}
		pid, _ := strconv.Atoi(pm[1])
		role := ""
		if rm := relatedRelRE.FindStringSubmatch(tbl); rm != nil {
			role = strings.TrimSpace(rm[1])
		}
		staff = append(staff, StaffCredit{PersonID: pid, Person: cleanText(pm[2]), Role: role})
	}
	if len(chars) == 0 && len(staff) == 0 {
		return nil, nil, fmt.Errorf("characters page parsed zero rows; the markup may have changed")
	}
	return chars, staff, nil
}

// SeasonEntry is one tile in a seasonal chart.
type SeasonEntry struct {
	Category  string   `json:"category,omitempty"`
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	MediaType string   `json:"media_type,omitempty"`
	StartDate string   `json:"start_date,omitempty"`
	Members   int      `json:"members,omitempty"`
	Score     float64  `json:"score,omitempty"`
	Genres    []string `json:"genres,omitempty"`
	Synopsis  string   `json:"synopsis,omitempty"`
}

var (
	seasonCatRE   = regexp.MustCompile(`(?s)<div class="anime-header">([^<]+)</div>`)
	seasonTitleRE = regexp.MustCompile(`(?s)<h2 class="h2_anime_title"><a href="https://myanimelist\.net/anime/(\d+)/[^"]*"[^>]*>([^<]+)</a>`)
	jsNumRE       = func(class string) *regexp.Regexp {
		return regexp.MustCompile(`class="js-` + class + `">([^<]*)<`)
	}
	seasonItemRE = regexp.MustCompile(`<span class="item">([^<]*)</span>`)
	seasonGenRE  = regexp.MustCompile(`/anime/genre/\d+/[^"]*"[^>]*>([^<]+)<`)
)

// ParseSeason parses a seasonal chart page into flat entries with their
// category (TV (New), TV (Continuing), ONA, OVA, Movie, Special).
func ParseSeason(page string) ([]SeasonEntry, error) {
	if !strings.Contains(page, "seasonal-anime") {
		return nil, fmt.Errorf("no seasonal tiles found: check the season path is /anime/season/{year}/{season} or /anime/season/{current,later,archive}")
	}
	out := make([]SeasonEntry, 0, 32)
	// Walk tile start offsets so each tile inherits the last category header.
	type pos struct {
		at  int
		cat string
		tok string
	}
	marks := make([]pos, 0, 64)
	for _, m := range seasonCatRE.FindAllStringSubmatchIndex(page, -1) {
		marks = append(marks, pos{at: m[0], cat: strings.TrimSpace(page[m[2]:m[3]])})
	}
	for _, m := range regexp.MustCompile(`js-anime-category-producer`).FindAllStringIndex(page, -1) {
		marks = append(marks, pos{at: m[0], tok: "tile"})
	}
	// Sort by offset (simple insertion sort keeps this dependency-free).
	for i := 1; i < len(marks); i++ {
		for j := i; j > 0 && marks[j].at < marks[j-1].at; j-- {
			marks[j], marks[j-1] = marks[j-1], marks[j]
		}
	}
	cat := ""
	for i, mk := range marks {
		if mk.tok != "tile" {
			cat = mk.cat
			continue
		}
		end := len(page)
		if i+1 < len(marks) {
			end = marks[i+1].at
		}
		tile := page[mk.at:end]
		tm := seasonTitleRE.FindStringSubmatch(tile)
		if tm == nil {
			continue
		}
		id, _ := strconv.Atoi(tm[1])
		e := SeasonEntry{ID: id, Title: cleanText(tm[2]), Category: cat}
		if m := jsNumRE("members").FindStringSubmatch(tile); m != nil {
			e.Members = firstNumber(m[1])
		}
		if m := jsNumRE("score").FindStringSubmatch(tile); m != nil {
			e.Score, _ = strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
		}
		if m := jsNumRE("start_date").FindStringSubmatch(tile); m != nil {
			d := strings.TrimSpace(m[1])
			if len(d) == 8 {
				e.StartDate = d[0:4] + "-" + d[4:6] + "-" + d[6:8]
			}
		}
		if m := seasonItemRE.FindStringSubmatch(tile); m != nil && e.StartDate == "" {
			e.StartDate = cleanText(m[1])
		}
		seen := map[string]bool{}
		for _, g := range seasonGenRE.FindAllStringSubmatch(tile, -1) {
			gname := cleanText(g[1])
			if gname != "" && !seen[gname] {
				seen[gname] = true
				e.Genres = append(e.Genres, gname)
			}
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("seasonal chart parsed zero tiles; the markup may have changed")
	}
	return out, nil
}

// RankedTitle is one row of a top-anime/top-manga ranking.
type RankedTitle struct {
	Rank      int     `json:"rank"`
	ID        int     `json:"id"`
	Title     string  `json:"title"`
	MediaType string  `json:"media_type,omitempty"`
	StartDate string  `json:"start_date,omitempty"`
	Score     float64 `json:"score,omitempty"`
	Members   int     `json:"members,omitempty"`
}

var (
	rankRowRE  = regexp.MustCompile(`(?s)<tr class="ranking-list">(.*?)</tr>`)
	rankLinkRE = regexp.MustCompile(`href="https://myanimelist\.net/(anime|manga)/(\d+)/[^"]*"[^>]*>([^<]+)<`)
	rankNumRE  = regexp.MustCompile(`class="rank[^"]*"[^>]*>\s*<span[^>]*>\s*(\d+)`)
)

// ParseRanking parses a top-anime or top-manga page.
func ParseRanking(page string) ([]RankedTitle, error) {
	rows := rankRowRE.FindAllStringSubmatch(page, -1)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no ranking rows found: check the list type against `ranking anime --type` / `ranking manga --type`")
	}
	out := make([]RankedTitle, 0, len(rows))
	for i, r := range rows {
		row := r[1]
		m := rankLinkRE.FindStringSubmatch(row)
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[2])
		rt := RankedTitle{ID: id, Title: cleanText(m[3]), Rank: i + 1}
		if rm := rankNumRE.FindStringSubmatch(row); rm != nil {
			rt.Rank = firstNumber(rm[1])
		}
		if sm := regexp.MustCompile(`class="score[^"]*"[^>]*>(?:\s*<[^>]*>)*\s*([0-9.]+)`).FindStringSubmatch(row); sm != nil {
			rt.Score, _ = strconv.ParseFloat(sm[1], 64)
		}
		text := cleanText(row)
		if tm := regexp.MustCompile(`\b(TV|Movie|OVA|ONA|Special|Music|CM|PV|Manga|Novel|One-shot|Manhwa|Manhua|Doujin|Light Novel)\b`).FindStringSubmatch(text); tm != nil {
			rt.MediaType = tm[1]
		}
		if dm := epAiredRE.FindStringSubmatch(text); dm != nil {
			rt.StartDate = dm[1]
		} else if dm := regexp.MustCompile(`([A-Z][a-z]{2} \d{4})`).FindStringSubmatch(text); dm != nil {
			rt.StartDate = dm[1]
		}
		if mm := regexp.MustCompile(`([0-9][0-9,]{3,})\s*members`).FindStringSubmatch(text); mm != nil {
			rt.Members = firstNumber(mm[1])
		}
		out = append(out, rt)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ranking rows parsed zero titles; the markup may have changed")
	}
	return out, nil
}

// PersonCredit is one animeography or staff entry on a person page.
type PersonCredit struct {
	Kind  string `json:"kind"`
	ID    int    `json:"id"`
	Title string `json:"title"`
	Role  string `json:"role,omitempty"`
}

// Person is a parsed voice-actor/staff page.
type Person struct {
	ID             int            `json:"id"`
	Name           string         `json:"name,omitempty"`
	NameJapanese   string         `json:"name_japanese,omitempty"`
	Favorites      int            `json:"favorites,omitempty"`
	About          string         `json:"about,omitempty"`
	VoiceRoles     []PersonCredit `json:"voice_roles,omitempty"`
	StaffRoles     []PersonCredit `json:"staff_roles,omitempty"`
	AlternateNames []string       `json:"alternate_names,omitempty"`
}

var personAnimeRE = regexp.MustCompile(`href="https://myanimelist\.net/anime/(\d+)/[^"]*"[^>]*>([^<]+)<`)

// ParsePerson parses a person page into voice roles and staff roles.
func ParsePerson(id int, page string) (*Person, error) {
	p := &Person{ID: id}
	if m := h1RE.FindStringSubmatch(page); m != nil {
		p.Name = cleanText(m[1])
	}
	if p.Name == "" {
		return nil, fmt.Errorf("no name found on person page %d: check the id", id)
	}
	p.NameJapanese = textField(page, "Japanese:")
	p.Favorites = firstNumber(textField(page, "Member Favorites:"))
	if m := regexp.MustCompile(`(?s)<div class="js-collapse-about">(.*?)</div>`).FindStringSubmatch(page); m != nil {
		p.About = cleanText(m[1])
	}
	voiceSection := page
	staffSection := ""
	if m := regexp.MustCompile(`(?s)<h2[^>]*>\s*Staff Positions\s*</h2>`).FindStringSubmatchIndex(page); m != nil {
		voiceSection = page[:m[0]]
		staffSection = page[m[0]:]
	}
	seen := map[string]bool{}
	for _, m := range personAnimeRE.FindAllStringSubmatch(voiceSection, -1) {
		aid, _ := strconv.Atoi(m[1])
		p.VoiceRoles = append(p.VoiceRoles, PersonCredit{Kind: "anime", ID: aid, Title: cleanText(m[2])})
		seen[m[1]] = true
	}
	for _, m := range personAnimeRE.FindAllStringSubmatch(staffSection, -1) {
		aid, _ := strconv.Atoi(m[1])
		if seen[m[1]] {
			continue
		}
		p.StaffRoles = append(p.StaffRoles, PersonCredit{Kind: "anime", ID: aid, Title: cleanText(m[2])})
	}
	if len(p.VoiceRoles) == 0 && len(p.StaffRoles) == 0 {
		return nil, fmt.Errorf("person page %d parsed zero credits; the markup may have changed", id)
	}
	return p, nil
}
