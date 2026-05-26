package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// WorkoutSummary is a single entry from the profile workout list.
type WorkoutSummary struct {
	ID    string
	Title string
	Date  time.Time
}

// Workout is the full detail of a single workout session.
type Workout struct {
	ID        string
	Title     string
	Gym       string
	Date      time.Time
	Exercises []Exercise
}

// Exercise is one exercise block within a workout.
type Exercise struct {
	Name string
	Slug string // derived from href e.g. /exercises/bench-press/ → bench-press
	Sets []Set
}

// Set is one row in an exercise's set table.
type Set struct {
	Number    int     // 0 = warmup
	IsWarmup  bool
	IsPR      bool
	WeightLbs float64 // 0 if not applicable
	Reps      int     // 0 if not applicable
	DurationS int     // 0 if not applicable
}

// colType classifies the rightmost column of an exercise's table header.
type colType int

const (
	colWeightReps colType = iota
	colReps
	colTime
)

var workoutIDRe = regexp.MustCompile(`/workouts/(\d+)/`)
var exerciseSlugRe = regexp.MustCompile(`/exercises/([^/]+)/`)

// ParseProfilePage parses a Gravitus user profile page and returns the list of
// workout summaries visible on that page, plus whether a "next page" link exists.
func ParseProfilePage(htmlStr string) ([]WorkoutSummary, bool, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, false, fmt.Errorf("parsing profile HTML: %w", err)
	}

	var summaries []WorkoutSummary
	hasNextPage := false

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			classes := classSet(n)

			// Each workout card has class "user-workout"
			if classes["user-workout"] {
				s := extractWorkoutSummary(n)
				if s != nil {
					summaries = append(summaries, *s)
				}
				return // don't recurse into the card
			}

			// Detect pagination "Next" link
			if n.Data == "a" {
				href := attr(n, "href")
				text := strings.TrimSpace(nodeText(n))
				if strings.Contains(href, "page=") && (strings.Contains(strings.ToLower(text), "next") || text == "›" || text == "»") {
					hasNextPage = true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return summaries, hasNextPage, nil
}

func extractWorkoutSummary(card *html.Node) *WorkoutSummary {
	s := &WorkoutSummary{}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			classes := classSet(n)

			// The workout link carries both the URL and the title
			if n.Data == "a" && classes["stretched-link"] {
				href := attr(n, "href")
				m := workoutIDRe.FindStringSubmatch(href)
				if len(m) == 2 {
					s.ID = m[1]
				}
				text := strings.ReplaceAll(nodeText(n), "›", "")
				s.Title = strings.TrimSpace(text)
			}

			// Date field
			if classes["started-at"] || classes["workout-date"] {
				t := parseDate(strings.TrimSpace(nodeText(n)))
				if !t.IsZero() {
					s.Date = t
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(card)

	if s.ID == "" {
		return nil
	}
	return s
}

// ParseWorkoutPage parses a Gravitus workout detail page and returns the full
// workout with exercises, sets, reps, weights, and PR markers.
func ParseWorkoutPage(htmlStr, workoutID string) (*Workout, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("parsing workout HTML: %w", err)
	}

	w := &Workout{ID: workoutID}

	// Extract workout title and gym from page header area
	extractWorkoutMeta(doc, w)

	// Extract exercise blocks
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && classSet(n)["exercise-block"] {
			ex := extractExercise(n)
			if ex != nil {
				w.Exercises = append(w.Exercises, *ex)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return w, nil
}

func extractWorkoutMeta(doc *html.Node, w *Workout) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			classes := classSet(n)

			// Workout title: <h1 class="h4 mb-3">Full Body + Core</h1>
			if n.Data == "h1" && w.Title == "" {
				t := strings.TrimSpace(nodeText(n))
				if t != "" {
					w.Title = t
				}
			}

			// Gym: <div class="d-flex flex-wrap gap-3 mb-4 text-muted small">
			//        <span>EōS Fitness</span>
			// Detect by text-muted + small + mb-4 combination
			if n.Data == "div" && classes["text-muted"] && classes["small"] && classes["mb-4"] && w.Gym == "" {
				// First non-empty span child is the gym name
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "span" {
						t := strings.TrimSpace(nodeText(c))
						if t != "" && t != "›" {
							w.Gym = t
							break
						}
					}
				}
			}

			// Fallback: explicit gym-name / location class
			if (classes["gym-name"] || classes["location"] || classes["workout-gym"]) && w.Gym == "" {
				t := strings.TrimSpace(nodeText(n))
				if t != "" {
					w.Gym = t
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
}

func extractExercise(block *html.Node) *Exercise {
	ex := &Exercise{}

	// Extract exercise name and slug from h3 > a.black-link
	var findName func(*html.Node)
	findName = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && classSet(n)["black-link"] {
			href := attr(n, "href")
			m := exerciseSlugRe.FindStringSubmatch(href)
			if len(m) == 2 {
				ex.Slug = m[1]
			}
			name := strings.ReplaceAll(nodeText(n), "›", "")
			ex.Name = strings.TrimSpace(name)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findName(c)
		}
	}
	findName(block)

	if ex.Name == "" {
		return nil
	}

	// Find the table and extract sets
	var findTable func(*html.Node) *html.Node
	findTable = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "table" {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if t := findTable(c); t != nil {
				return t
			}
		}
		return nil
	}

	table := findTable(block)
	if table == nil {
		return ex
	}

	// Determine column type from thead last cell
	ct := extractColType(table)

	// Extract rows from tbody
	var tbody *html.Node
	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "tbody" {
			tbody = c
			break
		}
	}
	if tbody == nil {
		return ex
	}

	setNum := 0
	for row := tbody.FirstChild; row != nil; row = row.NextSibling {
		if row.Type != html.ElementNode || row.Data != "tr" {
			continue
		}
		s := extractSet(row, ct, &setNum)
		if s != nil {
			ex.Sets = append(ex.Sets, *s)
		}
	}

	return ex
}

func extractColType(table *html.Node) colType {
	// Find thead → last td or th
	var thead *html.Node
	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "thead" {
			thead = c
			break
		}
	}
	if thead == nil {
		return colReps
	}

	// Find last cell in the header row
	headerText := ""
	var walkHead func(*html.Node)
	walkHead = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "td" || n.Data == "th") {
			t := strings.TrimSpace(nodeText(n))
			if t != "" && t != "Set" {
				headerText = strings.ToLower(t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkHead(c)
		}
	}
	walkHead(thead)

	if strings.Contains(headerText, "×") || strings.Contains(headerText, "x") || strings.Contains(headerText, "lb") {
		return colWeightReps
	}
	if strings.Contains(headerText, "time") {
		return colTime
	}
	return colReps
}

func extractSet(row *html.Node, ct colType, setNum *int) *Set {
	// Collect all td cells in this row
	var cells []*html.Node
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			cells = append(cells, c)
		}
	}
	if len(cells) < 2 {
		return nil
	}

	s := &Set{}

	// First cell: set number or "W"
	firstText := strings.TrimSpace(nodeText(cells[0]))
	if strings.EqualFold(firstText, "W") {
		s.IsWarmup = true
		s.Number = 0
	} else {
		n, err := strconv.Atoi(firstText)
		if err != nil {
			return nil // skip non-numeric, non-warmup rows
		}
		s.Number = n
		*setNum = n
	}

	// Check any cell for PR badge
	for _, cell := range cells {
		text := strings.ToLower(nodeText(cell))
		if strings.Contains(text, "pr") {
			// Make sure it's actually a PR badge, not just "pr" in some other text
			classes := classSet(cell)
			if classes["pr"] || classes["badge"] || strings.TrimSpace(text) == "pr" {
				s.IsPR = true
			}
		}
		// Check child nodes for PR badge class (actual HTML uses class="pr-badge")
		var checkPR func(*html.Node)
		checkPR = func(n *html.Node) {
			if n.Type == html.ElementNode {
				c := classSet(n)
				if c["pr-badge"] || c["pr"] || c["personal-record"] {
					s.IsPR = true
				}
			}
			for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
				checkPR(ch)
			}
		}
		checkPR(cell)
	}

	// Last cell: the value — get text from the first span or the cell itself
	lastCell := cells[len(cells)-1]
	valueText := ""

	// Try to find a span first (matching Cheerio's td:last-child span)
	var findSpan func(*html.Node) string
	findSpan = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "span" {
			t := strings.TrimSpace(nodeText(n))
			// Skip PR badge spans
			if !strings.EqualFold(t, "PR") && t != "" {
				return t
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := findSpan(c); v != "" {
				return v
			}
		}
		return ""
	}
	valueText = findSpan(lastCell)
	if valueText == "" {
		valueText = strings.TrimSpace(nodeText(lastCell))
	}

	if valueText == "" {
		return nil
	}

	// Parse the value based on column type
	switch ct {
	case colWeightReps:
		// Format: "25x10" or "50x10"
		re := regexp.MustCompile(`^([\d.]+)[xX×](\d+)$`)
		m := re.FindStringSubmatch(strings.TrimSpace(valueText))
		if m == nil {
			return nil
		}
		w, _ := strconv.ParseFloat(m[1], 64)
		r, _ := strconv.Atoi(m[2])
		s.WeightLbs = w
		s.Reps = r

	case colTime:
		// Format: "5:00" or "0:20"
		re := regexp.MustCompile(`^(\d+):(\d{2})$`)
		m := re.FindStringSubmatch(strings.TrimSpace(valueText))
		if m == nil {
			return nil
		}
		mins, _ := strconv.Atoi(m[1])
		secs, _ := strconv.Atoi(m[2])
		s.DurationS = mins*60 + secs

	case colReps:
		r, err := strconv.Atoi(strings.TrimSpace(valueText))
		if err != nil {
			return nil
		}
		s.Reps = r
	}

	return s
}

// TotalVolumeLbs calculates the total volume for a workout (weight × reps summed).
func (w *Workout) TotalVolumeLbs() float64 {
	var total float64
	for _, ex := range w.Exercises {
		for _, s := range ex.Sets {
			if s.WeightLbs > 0 && s.Reps > 0 {
				total += s.WeightLbs * float64(s.Reps)
			}
		}
	}
	return total
}

// ExercisesJSON returns the exercises formatted as the JSON string that the
// dashboard's LiftingSession.exercises field expects:
// [{"name":"Bench Press","sets":[{"reps":10,"weight_lbs":135},...]}]
func (w *Workout) ExercisesJSON() string {
	type setOut struct {
		Reps      *int     `json:"reps,omitempty"`
		WeightLbs *float64 `json:"weight_lbs,omitempty"`
		DurationS *int     `json:"duration_s,omitempty"`
	}
	type exOut struct {
		Name string   `json:"name"`
		Sets []setOut `json:"sets"`
	}
	var out []exOut
	for _, ex := range w.Exercises {
		e := exOut{Name: ex.Name}
		for _, s := range ex.Sets {
			so := setOut{}
			if s.Reps > 0 {
				r := s.Reps
				so.Reps = &r
			}
			if s.WeightLbs > 0 {
				wl := s.WeightLbs
				so.WeightLbs = &wl
			}
			if s.DurationS > 0 {
				d := s.DurationS
				so.DurationS = &d
			}
			e.Sets = append(e.Sets, so)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return "[]"
	}

	// Manual JSON encoding to avoid importing encoding/json in this package
	var sb strings.Builder
	sb.WriteString("[")
	for i, e := range out {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"name":`)
		sb.WriteString(jsonStr(e.Name))
		sb.WriteString(`,"sets":[`)
		for j, s := range e.Sets {
			if j > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("{")
			first := true
			if s.Reps != nil {
				sb.WriteString(fmt.Sprintf(`"reps":%d`, *s.Reps))
				first = false
			}
			if s.WeightLbs != nil {
				if !first {
					sb.WriteString(",")
				}
				sb.WriteString(fmt.Sprintf(`"weight_lbs":%g`, *s.WeightLbs))
				first = false
			}
			if s.DurationS != nil {
				if !first {
					sb.WriteString(",")
				}
				sb.WriteString(fmt.Sprintf(`"duration_s":%d`, *s.DurationS))
			}
			sb.WriteString("}")
		}
		sb.WriteString("]}")
	}
	sb.WriteString("]")
	return sb.String()
}

// PRs returns all personal record sets across all exercises.
func (w *Workout) PRs() []PR {
	var prs []PR
	for _, ex := range w.Exercises {
		for _, s := range ex.Sets {
			if s.IsPR {
				prs = append(prs, PR{
					ExerciseName: ex.Name,
					ExerciseSlug: ex.Slug,
					WeightLbs:    s.WeightLbs,
					Reps:         s.Reps,
					Date:         w.Date,
				})
			}
		}
	}
	return prs
}

// PR represents a personal record from a workout.
type PR struct {
	ExerciseName string
	ExerciseSlug string
	WeightLbs    float64
	Reps         int
	Date         time.Time
}

// Estimated1RM returns the Epley formula estimate for 1RM given weight and reps.
func Estimated1RM(weightLbs float64, reps int) float64 {
	if reps <= 0 || weightLbs <= 0 {
		return 0
	}
	if reps == 1 {
		return weightLbs
	}
	return weightLbs * (1 + float64(reps)/30.0)
}

// Best1RMSet returns the set with the highest estimated 1RM across an exercise's sets.
func (ex *Exercise) Best1RMSet() *Set {
	var best *Set
	var bestEst float64
	for i := range ex.Sets {
		s := &ex.Sets[i]
		if s.IsWarmup {
			continue
		}
		est := Estimated1RM(s.WeightLbs, s.Reps)
		if est > bestEst {
			bestEst = est
			best = s
		}
	}
	return best
}

// --- Helpers ---

func classSet(n *html.Node) map[string]bool {
	classes := make(map[string]bool)
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				classes[c] = true
			}
		}
	}
	return classes
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(nodeText(c))
	}
	return sb.String()
}

var dateFormats = []string{
	time.RFC3339,          // "2026-05-15T22:03:07+00:00" — actual Gravitus format
	time.RFC3339Nano,      // with nanoseconds
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"January 2, 2006",
	"Jan 2, 2006",
	"2006-01-02",
	"01/02/2006",
	"1/2/2006",
	"January 02, 2006",
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, f := range dateFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func jsonStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return `"` + s + `"`
}
