package edeka

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var umlauts = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "Ä", "ae", "Ö", "oe", "Ü", "ue")
var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func tokens(s string) []string {
	s = nonWord.ReplaceAllString(umlauts.Replace(strings.ToLower(s)), " ")
	return strings.Fields(s)
}

// MatchScore rates how well a product title matches a free-text query
// (0 = no query word found).
func MatchScore(query, title string) float64 {
	q := tokens(query)
	t := tokens(title)
	if len(q) == 0 || len(t) == 0 {
		return 0
	}
	joined := " " + strings.Join(t, " ") + " "
	score := 0.0
	hits := 0
	for _, w := range q {
		switch {
		case strings.Contains(joined, " "+w+" "):
			score += 3
			hits++
		case strings.Contains(joined, " "+w):
			score += 2
			hits++
		case strings.Contains(joined, w):
			score += 1
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	if hits == len(q) {
		score += 3
	}
	if strings.HasPrefix(strings.Join(t, " "), strings.Join(q, " ")) {
		score += 2
	}
	// shorter titles are closer to what was asked for
	score -= float64(len(t)) * 0.1
	return score
}

// nonFoodPaths are category roots of non-grocery assortments; they only win
// when the query names them.
var nonFoodPaths = []string{"/Drogerie-EDEKA/", "/Haushalt-Freizeit/", "/Tabak-Zigaretten-EDEKA/", "/Angebote/Drogerie/", "/Angebote/Haushalt-Freizeit/"}

// IsNonFood reports whether a product URL sits in a non-grocery category.
func IsNonFood(u string) bool {
	for _, p := range nonFoodPaths {
		if strings.Contains(u, p) {
			return true
		}
	}
	return false
}

// categoryScore rewards a product whose category path names a query word
// (".../Milch/...", ".../Butter-Fette/...").
func categoryScore(query, u string) float64 {
	i := strings.Index(u, ".edeka.shop/")
	if i >= 0 {
		u = u[i+len(".edeka.shop/"):]
	}
	if j := strings.LastIndex(u, "/"); j >= 0 {
		u = u[:j]
	}
	path := " " + strings.Join(tokens(u), " ") + " "
	score := 0.0
	for _, w := range tokens(query) {
		if strings.Contains(path, " "+w+" ") || strings.Contains(path, " "+w) {
			score += 2
		}
	}
	return score
}

// BestMatch picks the product that best matches the query. Weight articles
// are only chosen when nothing else matches (they need a weight); non-grocery
// articles lose against groceries unless the query names their category; the
// shop's own ranking breaks ties.
func BestMatch(query string, products []Product) (Product, bool) {
	type cand struct {
		p Product
		s float64
		i int
	}
	var cs []cand
	for i, p := range products {
		s := MatchScore(query, p.Title)
		if s <= 0 || p.ID == "" || !WordMatch(query, p.Title) {
			continue
		}
		cat := categoryScore(query, p.URL)
		s += cat
		if p.WeightArticle {
			s -= 5
		}
		if IsNonFood(p.URL) && cat == 0 {
			s -= 4
		}
		s -= float64(i) * 0.05
		cs = append(cs, cand{p, s, i})
	}
	if len(cs) == 0 {
		return Product{}, false
	}
	sort.SliceStable(cs, func(a, b int) bool {
		if cs[a].s != cs[b].s {
			return cs[a].s > cs[b].s
		}
		return cs[a].i < cs[b].i
	})
	return cs[0].p, true
}

// WordMatch reports whether a query word equals, starts or ends a title word
// (German compounds: "milch" matches "Vollmilch"). Words under three letters
// are ignored. Unlike MatchScore it ignores matches inside a word
// ("press" does not match "Espresso").
func WordMatch(query, title string) bool {
	tt := tokens(title)
	for _, w := range tokens(query) {
		if len(w) < 3 {
			continue
		}
		for _, t := range tt {
			if t == w || strings.HasPrefix(t, w) || (len(w) >= 4 && strings.HasSuffix(t, w)) {
				return true
			}
		}
	}
	return false
}

// Relevance rates a product for a query: title words, category path words and
// a penalty for non-grocery articles the query does not name. It can be
// negative; callers sort by it and keep the shop's order for ties.
func Relevance(query string, p Product) float64 {
	cat := categoryScore(query, p.URL)
	s := MatchScore(query, p.Title) + cat
	if IsNonFood(p.URL) && cat == 0 {
		s -= 4
	}
	return s
}

// groceryAlias maps everyday words to the names German shops must use
// (plant drinks may not be called "Milch").
var groceryAlias = map[string]string{"hafermilch": "haferdrink", "sojamilch": "sojadrink", "mandelmilch": "mandeldrink", "reismilch": "reisdrink", "kokosmilch": "kokosmilch", "dinkelmilch": "dinkeldrink"}

// AlternativeQuery returns a second query to try when the first only finds
// non-grocery articles, or "".
func AlternativeQuery(query string) string {
	ws := tokens(query)
	changed := false
	for i, w := range ws {
		if a, ok := groceryAlias[w]; ok && a != w {
			ws[i] = a
			changed = true
		}
	}
	if !changed {
		return ""
	}
	return strings.Join(ws, " ")
}

var qtyPrefix = regexp.MustCompile(`^\s*(\d{1,2})\s*(?:x|×|\*)?\s+(.+)$`)
var qtySuffix = regexp.MustCompile(`^(.+?)\s+(?:x\s*)?(\d{1,2})\s*x?\s*$`)

// ParseListLine reads "2x kaffee", "2 kaffee", "kaffee x2" or "kaffee" into
// quantity and text.
func ParseListLine(line string) (int, string) {
	line = strings.TrimSpace(line)
	if m := qtyPrefix.FindStringSubmatch(line); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n > 0 {
			return n, strings.TrimSpace(m[2])
		}
	}
	if m := qtySuffix.FindStringSubmatch(line); m != nil {
		n, _ := strconv.Atoi(m[2])
		if n > 0 {
			return n, strings.TrimSpace(m[1])
		}
	}
	return 1, line
}

// LooksLikeArticleID reports whether s is a shop article id such as
// 5915407005MXST (digits followed by a market suffix).
func LooksLikeArticleID(s string) bool {
	return regexp.MustCompile(`^[0-9]{4,}[A-Z]{2,6}$`).MatchString(strings.TrimSpace(s))
}
