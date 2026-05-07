// Package criteria implements the no-LLM heuristic that translates a
// free-text criteria string into OSM tag filters and Reddit keyword maps.
// Used by the `goat` standalone compound (no LLM in runtime path) and as a
// fallback by `near` when no agent is orchestrating.
package criteria

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sources"
)

// Match holds the parsed inputs from a free-text criteria string.
type Match struct {
	Intent       sources.Intent
	OSMTags      []OSMTag
	RedditKW     []string  // body-keyword filters
	QualityWords []string  // signals like "vintage", "no tourists", "hidden", "award-winning"
	NegationKW   []string  // explicit avoids — "no tourists", "no chain"
}

// OSMTag is one Overpass tag filter entry.
type OSMTag struct {
	Key   string
	Value string // empty = key existence
}

// keywordTable maps criteria phrases to (intent, OSM tags, Reddit keywords).
// Static lookup so no LLM in the runtime path. Order matters: more specific
// rows come first so they win the intent assignment over generic ones.
// Rows whose intent is empty are quality modifiers — they contribute tags
// and Reddit keywords without claiming the intent slot.
var keywordTable = []struct {
	phrases  []string
	intent   sources.Intent
	osmTags  []OSMTag
	redditKW []string
}{
	// Specific intent rows — most specific FIRST.
	{[]string{"jazz kissaten", "jazz cafe", "jazz bar"}, sources.IntentDrinks, []OSMTag{{"amenity", "bar"}, {"music:jazz", ""}}, []string{"jazz", "kissaten", "vinyl"}},
	{[]string{"viewpoint", "scenic", "photographer", "photo spot", "rooftop", "blue hour", "golden hour"}, sources.IntentViewpoint, []OSMTag{{"tourism", "viewpoint"}}, []string{"viewpoint", "rooftop", "blue hour", "golden hour", "photographer"}},
	{[]string{"kissaten", "vintage cafe", "kissa", "coffee snob", "pour-over", "pour over", "barista", "70s cafe", "old cafe"}, sources.IntentCoffee, []OSMTag{{"amenity", "cafe"}, {"cuisine", "coffee_shop"}}, []string{"kissaten", "pour over", "no chain", "third wave", "beans"}},
	{[]string{"natural wine", "wine bar", "biodynamic"}, sources.IntentDrinks, []OSMTag{{"amenity", "bar"}, {"drink:wine", "yes"}}, []string{"natural wine", "biodynamic", "skin contact"}},
	{[]string{"vintage clothing", "thrift", "vintage shop", "secondhand", "vinyl", "record store", "bookstore"}, sources.IntentShopping, []OSMTag{{"shop", "second_hand"}, {"shop", "music"}, {"shop", "books"}}, []string{"vintage", "vinyl", "rare", "import"}},
	{[]string{"sushi", "omakase"}, sources.IntentFood, []OSMTag{{"cuisine", "sushi"}}, []string{"sushi", "omakase"}},
	{[]string{"ramen"}, sources.IntentFood, []OSMTag{{"cuisine", "ramen"}}, []string{"ramen", "tonkotsu"}},
	{[]string{"hand-pulled noodles", "kalguksu", "noodle shop"}, sources.IntentFood, []OSMTag{{"cuisine", "noodle"}}, []string{"hand-pulled", "noodle", "kalguksu"}},
	{[]string{"pizza", "neapolitan"}, sources.IntentFood, []OSMTag{{"cuisine", "pizza"}}, []string{"pizza", "neapolitan"}},
	{[]string{"bakery", "boulangerie", "bread", "pastry"}, sources.IntentFood, []OSMTag{{"shop", "bakery"}}, []string{"bakery", "bread", "viennoiserie"}},
	{[]string{"michelin", "starred", "tasting menu", "fine dining"}, sources.IntentFood, []OSMTag{{"michelin", ""}}, []string{"michelin", "tasting menu"}},
	{[]string{"history", "historic", "old town", "heritage"}, sources.IntentHistoric, []OSMTag{{"historic", ""}}, []string{"history", "historic"}},
	{[]string{"museum", "gallery", "art"}, sources.IntentCulture, []OSMTag{{"tourism", "museum"}, {"tourism", "gallery"}}, []string{"museum", "exhibit"}},
	{[]string{"temple", "shrine", "church", "cathedral"}, sources.IntentHistoric, []OSMTag{{"amenity", "place_of_worship"}}, []string{"temple", "shrine"}},
	{[]string{"park", "garden", "nature", "trail", "hike"}, sources.IntentNature, []OSMTag{{"leisure", "park"}, {"leisure", "garden"}}, []string{"park", "garden"}},
	{[]string{"cocktail", "speakeasy", "drinks"}, sources.IntentDrinks, []OSMTag{{"amenity", "bar"}}, []string{"cocktail", "bar"}},
	{[]string{"coffee", "espresso", "cappuccino", "cafe"}, sources.IntentCoffee, []OSMTag{{"amenity", "cafe"}}, []string{"coffee", "espresso"}},
	{[]string{"food", "restaurant", "dinner", "lunch"}, sources.IntentFood, []OSMTag{{"amenity", "restaurant"}}, []string{"restaurant"}},

	// Quality modifiers — no intent claim, but they add tags and reddit keywords.
	{[]string{"hidden", "no tourists", "locals only", "underground", "off the beaten"}, "", nil, []string{"locals only", "no tourists", "hidden", "off the beaten"}},
	{[]string{"vintage"}, "", nil, []string{"vintage"}},
	{[]string{"award-winning", "award winning"}, "", nil, []string{"award", "winner"}},
}

// quietSignal phrases that indicate empty/quiet hours in Reddit body matches
// (used by `quiet-hour`).
var quietSignals = []string{"dead before", "empty weekday", "quiet on", "never crowded", "almost empty", "rarely busy", "no line"}

// QuietSignals returns the keyword set for quiet-hour body matching.
func QuietSignals() []string {
	out := make([]string, len(quietSignals))
	copy(out, quietSignals)
	return out
}

// negationPhrases the user might write to express avoid signals.
var negationPhrases = []string{"no tourists", "no chain", "no scene", "no instagram", "no influencer", "not touristy", "not a chain"}

// Parse turns a free-text criteria string into a Match. Best-effort:
// unmatched text contributes to QualityWords and RedditKW so the ranker
// still has signal even if no static row matched.
func Parse(criteria string) Match {
	lower := strings.ToLower(criteria)
	m := Match{}

	// Detect negations first so we don't accidentally promote them as positive intent.
	for _, neg := range negationPhrases {
		if strings.Contains(lower, neg) {
			m.NegationKW = append(m.NegationKW, neg)
		}
	}

	tagSeen := map[string]bool{}
	kwSeen := map[string]bool{}

	for _, row := range keywordTable {
		for _, phrase := range row.phrases {
			if !strings.Contains(lower, phrase) {
				continue
			}
			if m.Intent == "" {
				m.Intent = row.intent
			}
			for _, tag := range row.osmTags {
				key := tag.Key + "=" + tag.Value
				if !tagSeen[key] {
					m.OSMTags = append(m.OSMTags, tag)
					tagSeen[key] = true
				}
			}
			for _, kw := range row.redditKW {
				if !kwSeen[kw] {
					m.RedditKW = append(m.RedditKW, kw)
					kwSeen[kw] = true
				}
			}
		}
	}

	// QualityWords: short words (≥4 chars) that didn't match a phrase.
	for _, word := range strings.Fields(lower) {
		word = strings.Trim(word, ".,!?;:")
		if len(word) >= 4 && !kwSeen[word] && !isStopWord(word) {
			m.QualityWords = append(m.QualityWords, word)
		}
	}

	if m.Intent == "" {
		m.Intent = sources.IntentFood // safe default
	}

	return m
}

var stopWords = map[string]bool{
	"with": true, "from": true, "that": true, "this": true, "have": true, "will": true,
	"want": true, "like": true, "near": true, "into": true, "some": true, "very": true,
	"much": true, "many": true, "more": true, "most": true,
}

func isStopWord(w string) bool { return stopWords[w] }
