package roadside

import (
	"regexp"
	"sort"
	"strings"
)

// CategoryDef is a local classification bucket. The site's own theme slugs are
// internal and undocumented, so categories are matched here by name patterns —
// which directly serves user-facing superlatives (biggest, smallest, tallest,
// weird-food) the site never exposes as a single command.
type CategoryDef struct {
	Name    string         `json:"name"`
	Summary string         `json:"summary"`
	Aliases []string       `json:"aliases,omitempty"`
	re      *regexp.Regexp `json:"-"`
}

// categoryDefs is the ordered classification set. Order controls --list output
// and tie-breaking; the user's four requested categories lead.
var categoryDefs = []CategoryDef{
	{
		Name:    "biggest",
		Summary: "World's largest / giant / colossal things",
		Aliases: []string{"big", "biggest", "largest", "large", "giant", "giants", "huge", "world's-largest", "worlds-largest"},
		re:      regexp.MustCompile(`(?i)\b(world'?s\s+largest|largest|biggest|world'?s\s+biggest|giant|gigantic|colossal|enormous|jumbo|mega|huge|world'?s\s+tallest)\b`),
	},
	{
		Name:    "smallest",
		Summary: "World's smallest / tiniest / miniature things",
		Aliases: []string{"small", "smallest", "tiny", "tiniest", "miniature", "mini", "world's-smallest"},
		re:      regexp.MustCompile(`(?i)\b(world'?s\s+smallest|smallest|tiniest|miniature)\b`),
	},
	{
		Name:    "tallest",
		Summary: "Tallest / highest things",
		Aliases: []string{"tall", "tallest", "highest", "high"},
		re:      regexp.MustCompile(`(?i)\b(world'?s\s+tallest|tallest|highest)\b`),
	},
	{
		Name:    "weird-food",
		Summary: "Giant food, drink, and roadside eats oddities",
		Aliases: []string{"food", "weird-food", "weirdfood", "eats", "drink"},
		re:      regexp.MustCompile(`(?i)\b(donut|doughnut|coffee\s*pot|tea\s*pot|teapot|catsup|ketchup|hot\s*dog|hotdog|peanut|strawberry|pie|cheese|soda|burger|hamburger|ice\s*cream|frying\s*pan|fork|spoon|chicken|rooster|egg|apple|orange|banana|pecan|coffee|milk\s*bottle|popcorn|candy|chili|taco|pierogi|twinkie|spud|potato|tamale|cherry|peach|crab|lobster|shrimp|wienermobile|brat|sausage|cola|root\s*beer|pretzel|pizza|corn|watermelon|pancake|cinnamon|mushroom|onion|pepper|pickle|cake|donuts)\b`),
	},
	{
		Name:    "muffler-men",
		Summary: "Muffler Men, fiberglass giants, Paul Bunyans, Uniroyal Gals",
		Aliases: []string{"muffler", "mufflerman", "muffler-man", "fiberglass", "bunyan"},
		re:      regexp.MustCompile(`(?i)\b(muffler\s*man|muffler\s*men|fiberglass\s+(man|giant|guy)|paul\s+bunyan|uniroyal\s+gal|gemini\s+giant|big\s+john)\b`),
	},
	{
		Name:    "animals",
		Summary: "Giant animals, dinosaurs, and creature statues",
		Aliases: []string{"animal", "animals", "creatures", "beasts"},
		re:      regexp.MustCompile(`(?i)\b(cow|bull|steer|horse|dinosaur|dino|t-?rex|elephant|alligator|gator|crocodile|bison|buffalo|duck|fish|whale|shark|frog|jackrabbit|rabbit|bunny|bear|moose|elk|fox|owl|eagle|pig|hog|boar|snake|serpent|turtle|tortoise|beaver|squirrel|penguin|gorilla|ape|spider|bee|butterfly|prairie\s+dog|armadillo|catfish|trout|walleye|loon|pelican|flamingo|peacock|jackalope|kangaroo|camel|lobster)\b`),
	},
	{
		Name:    "signs",
		Summary: "Neon signs, vintage signage, and roadside marquees",
		Aliases: []string{"sign", "signs", "neon", "signage"},
		re:      regexp.MustCompile(`(?i)\b(sign|signs|neon|billboard|marquee|signage|arrow)\b`),
	},
	{
		Name:    "statues",
		Summary: "Statues, monuments, and oversized figures",
		Aliases: []string{"statue", "statues", "monument", "monuments", "figure"},
		re:      regexp.MustCompile(`(?i)\b(statue|monument|memorial|sculpture|figure|effigy|colossus)\b`),
	},
	{
		Name:    "museums",
		Summary: "Quirky and single-subject museums",
		Aliases: []string{"museum", "museums", "hall-of-fame"},
		re:      regexp.MustCompile(`(?i)\b(museum|hall\s+of\s+fame|gallery)\b`),
	},
}

// Classify returns every category whose pattern matches the attraction name.
func Classify(a Attraction) []string {
	subject := a.Name
	var out []string
	for _, c := range categoryDefs {
		if c.re.MatchString(subject) {
			out = append(out, c.Name)
		}
	}
	return out
}

// NormalizeCategory resolves a user-supplied category token (canonical name or
// alias) to its canonical category name. ok is false when unrecognized.
func NormalizeCategory(input string) (canonical string, ok bool) {
	key := strings.ToLower(strings.TrimSpace(input))
	key = strings.ReplaceAll(key, " ", "-")
	for _, c := range categoryDefs {
		if c.Name == key {
			return c.Name, true
		}
		for _, a := range c.Aliases {
			if a == key {
				return c.Name, true
			}
		}
	}
	return "", false
}

// MatchesCategory reports whether an attraction belongs to a canonical category.
func MatchesCategory(a Attraction, canonical string) bool {
	for _, c := range categoryDefs {
		if c.Name == canonical {
			return c.re.MatchString(a.Name)
		}
	}
	return false
}

// Categories returns the category definitions (without the compiled regexp)
// for the `category --list` output, in canonical order.
func Categories() []CategoryDef {
	out := make([]CategoryDef, 0, len(categoryDefs))
	for _, c := range categoryDefs {
		aliases := append([]string(nil), c.Aliases...)
		sort.Strings(aliases)
		out = append(out, CategoryDef{Name: c.Name, Summary: c.Summary, Aliases: aliases})
	}
	return out
}

// CategoryNames returns the canonical category names in order.
func CategoryNames() []string {
	out := make([]string, 0, len(categoryDefs))
	for _, c := range categoryDefs {
		out = append(out, c.Name)
	}
	return out
}
