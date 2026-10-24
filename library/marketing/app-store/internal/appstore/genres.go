// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Genre is one App Store genre (category). Store is "ios" for the iPhone/iPad
// tree rooted at 36 and "mac" for the Mac App Store tree rooted at 39. The web
// chart pages use the ios ids on every platform, including Mac.
type Genre struct {
	ID       string `json:"genreId"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
	Store    string `json:"store"`
}

// AllGenreID is the root "All Apps" genre used by the overall charts.
const AllGenreID = "36"

// GamesGenreID is the Games genre; its children are the game subgenres.
const GamesGenreID = "6014"

// genreTable is Apple's genre tree from the MZStoreServices genres service
// (archived 2024-12-08, unchanged 2025-02-25) cross-checked against the live
// chart pages of 2026-10-23: 6026 Developer Tools and 6027 Graphics & Design
// are current, and 7003 is Casual (it was Arcade before 2020).
var genreTable = []Genre{
	{ID: "36", Name: "All Apps", Store: "ios"},
	{ID: "6000", Name: "Business", ParentID: "36", Store: "ios"},
	{ID: "6001", Name: "Weather", ParentID: "36", Store: "ios"},
	{ID: "6002", Name: "Utilities", ParentID: "36", Store: "ios"},
	{ID: "6003", Name: "Travel", ParentID: "36", Store: "ios"},
	{ID: "6004", Name: "Sports", ParentID: "36", Store: "ios"},
	{ID: "6005", Name: "Social Networking", ParentID: "36", Store: "ios"},
	{ID: "6006", Name: "Reference", ParentID: "36", Store: "ios"},
	{ID: "6007", Name: "Productivity", ParentID: "36", Store: "ios"},
	{ID: "6008", Name: "Photo & Video", ParentID: "36", Store: "ios"},
	{ID: "6009", Name: "News", ParentID: "36", Store: "ios"},
	{ID: "6010", Name: "Navigation", ParentID: "36", Store: "ios"},
	{ID: "6011", Name: "Music", ParentID: "36", Store: "ios"},
	{ID: "6012", Name: "Lifestyle", ParentID: "36", Store: "ios"},
	{ID: "6013", Name: "Health & Fitness", ParentID: "36", Store: "ios"},
	{ID: "6014", Name: "Games", ParentID: "36", Store: "ios"},
	{ID: "6015", Name: "Finance", ParentID: "36", Store: "ios"},
	{ID: "6016", Name: "Entertainment", ParentID: "36", Store: "ios"},
	{ID: "6017", Name: "Education", ParentID: "36", Store: "ios"},
	{ID: "6018", Name: "Books", ParentID: "36", Store: "ios"},
	{ID: "6020", Name: "Medical", ParentID: "36", Store: "ios"},
	{ID: "6021", Name: "Magazines & Newspapers", ParentID: "36", Store: "ios"},
	{ID: "6022", Name: "Catalogs", ParentID: "36", Store: "ios"},
	{ID: "6023", Name: "Food & Drink", ParentID: "36", Store: "ios"},
	{ID: "6024", Name: "Shopping", ParentID: "36", Store: "ios"},
	{ID: "6025", Name: "Stickers", ParentID: "36", Store: "ios"},
	{ID: "6026", Name: "Developer Tools", ParentID: "36", Store: "ios"},
	{ID: "6027", Name: "Graphics & Design", ParentID: "36", Store: "ios"},
	// Game subgenres (7010 does not exist on iOS).
	{ID: "7001", Name: "Action", ParentID: "6014", Store: "ios"},
	{ID: "7002", Name: "Adventure", ParentID: "6014", Store: "ios"},
	{ID: "7003", Name: "Casual", ParentID: "6014", Store: "ios"},
	{ID: "7004", Name: "Board", ParentID: "6014", Store: "ios"},
	{ID: "7005", Name: "Card", ParentID: "6014", Store: "ios"},
	{ID: "7006", Name: "Casino", ParentID: "6014", Store: "ios"},
	{ID: "7007", Name: "Dice", ParentID: "6014", Store: "ios"},
	{ID: "7008", Name: "Educational", ParentID: "6014", Store: "ios"},
	{ID: "7009", Name: "Family", ParentID: "6014", Store: "ios"},
	{ID: "7011", Name: "Music", ParentID: "6014", Store: "ios"},
	{ID: "7012", Name: "Puzzle", ParentID: "6014", Store: "ios"},
	{ID: "7013", Name: "Racing", ParentID: "6014", Store: "ios"},
	{ID: "7014", Name: "Role-Playing", ParentID: "6014", Store: "ios"},
	{ID: "7015", Name: "Simulation", ParentID: "6014", Store: "ios"},
	{ID: "7016", Name: "Sports", ParentID: "6014", Store: "ios"},
	{ID: "7017", Name: "Strategy", ParentID: "6014", Store: "ios"},
	{ID: "7018", Name: "Trivia", ParentID: "6014", Store: "ios"},
	{ID: "7019", Name: "Word", ParentID: "6014", Store: "ios"},
	// Magazines & Newspapers subgenres.
	{ID: "13001", Name: "News & Politics", ParentID: "6021", Store: "ios"},
	{ID: "13002", Name: "Fashion & Style", ParentID: "6021", Store: "ios"},
	{ID: "13003", Name: "Home & Garden", ParentID: "6021", Store: "ios"},
	{ID: "13004", Name: "Outdoors & Nature", ParentID: "6021", Store: "ios"},
	{ID: "13005", Name: "Sports & Leisure", ParentID: "6021", Store: "ios"},
	{ID: "13006", Name: "Automotive", ParentID: "6021", Store: "ios"},
	{ID: "13007", Name: "Arts & Photography", ParentID: "6021", Store: "ios"},
	{ID: "13008", Name: "Brides & Weddings", ParentID: "6021", Store: "ios"},
	{ID: "13009", Name: "Business & Investing", ParentID: "6021", Store: "ios"},
	{ID: "13010", Name: "Children's Magazines", ParentID: "6021", Store: "ios"},
	{ID: "13011", Name: "Computers & Internet", ParentID: "6021", Store: "ios"},
	{ID: "13012", Name: "Cooking, Food & Drink", ParentID: "6021", Store: "ios"},
	{ID: "13013", Name: "Crafts & Hobbies", ParentID: "6021", Store: "ios"},
	{ID: "13014", Name: "Electronics & Audio", ParentID: "6021", Store: "ios"},
	{ID: "13015", Name: "Entertainment", ParentID: "6021", Store: "ios"},
	{ID: "13017", Name: "Health, Mind & Body", ParentID: "6021", Store: "ios"},
	{ID: "13018", Name: "History", ParentID: "6021", Store: "ios"},
	{ID: "13019", Name: "Literary Magazines & Journals", ParentID: "6021", Store: "ios"},
	{ID: "13020", Name: "Men's Interest", ParentID: "6021", Store: "ios"},
	{ID: "13021", Name: "Movies & Music", ParentID: "6021", Store: "ios"},
	{ID: "13023", Name: "Parenting & Family", ParentID: "6021", Store: "ios"},
	{ID: "13024", Name: "Pets", ParentID: "6021", Store: "ios"},
	{ID: "13025", Name: "Professional & Trade", ParentID: "6021", Store: "ios"},
	{ID: "13026", Name: "Regional News", ParentID: "6021", Store: "ios"},
	{ID: "13027", Name: "Science", ParentID: "6021", Store: "ios"},
	{ID: "13028", Name: "Teens", ParentID: "6021", Store: "ios"},
	{ID: "13029", Name: "Travel & Regional", ParentID: "6021", Store: "ios"},
	{ID: "13030", Name: "Women's Interest", ParentID: "6021", Store: "ios"},
	// Stickers subgenres.
	{ID: "16001", Name: "Emoji & Expressions", ParentID: "6025", Store: "ios"},
	{ID: "16003", Name: "Animals & Nature", ParentID: "6025", Store: "ios"},
	{ID: "16005", Name: "Art", ParentID: "6025", Store: "ios"},
	{ID: "16006", Name: "Celebrations", ParentID: "6025", Store: "ios"},
	{ID: "16007", Name: "Celebrities", ParentID: "6025", Store: "ios"},
	{ID: "16008", Name: "Comics & Cartoons", ParentID: "6025", Store: "ios"},
	{ID: "16009", Name: "Eating & Drinking", ParentID: "6025", Store: "ios"},
	{ID: "16010", Name: "Gaming", ParentID: "6025", Store: "ios"},
	{ID: "16014", Name: "Movies & TV", ParentID: "6025", Store: "ios"},
	{ID: "16015", Name: "Music", ParentID: "6025", Store: "ios"},
	{ID: "16017", Name: "People", ParentID: "6025", Store: "ios"},
	{ID: "16019", Name: "Places & Objects", ParentID: "6025", Store: "ios"},
	{ID: "16021", Name: "Sports & Activities", ParentID: "6025", Store: "ios"},
	{ID: "16025", Name: "Kids & Family", ParentID: "6025", Store: "ios"},
	{ID: "16026", Name: "Fashion", ParentID: "6025", Store: "ios"},
	// Mac App Store tree (legacy Mac RSS feeds and Lookup of Mac-only apps).
	{ID: "39", Name: "Mac App Store", Store: "mac"},
	{ID: "12001", Name: "Business", ParentID: "39", Store: "mac"},
	{ID: "12002", Name: "Developer Tools", ParentID: "39", Store: "mac"},
	{ID: "12003", Name: "Education", ParentID: "39", Store: "mac"},
	{ID: "12004", Name: "Entertainment", ParentID: "39", Store: "mac"},
	{ID: "12005", Name: "Finance", ParentID: "39", Store: "mac"},
	{ID: "12006", Name: "Games", ParentID: "39", Store: "mac"},
	{ID: "12201", Name: "Action", ParentID: "12006", Store: "mac"},
	{ID: "12202", Name: "Adventure", ParentID: "12006", Store: "mac"},
	{ID: "12203", Name: "Casual", ParentID: "12006", Store: "mac"},
	{ID: "12204", Name: "Board", ParentID: "12006", Store: "mac"},
	{ID: "12205", Name: "Card", ParentID: "12006", Store: "mac"},
	{ID: "12206", Name: "Casino", ParentID: "12006", Store: "mac"},
	{ID: "12207", Name: "Dice", ParentID: "12006", Store: "mac"},
	{ID: "12208", Name: "Educational", ParentID: "12006", Store: "mac"},
	{ID: "12209", Name: "Family", ParentID: "12006", Store: "mac"},
	{ID: "12210", Name: "Kids", ParentID: "12006", Store: "mac"},
	{ID: "12211", Name: "Music", ParentID: "12006", Store: "mac"},
	{ID: "12212", Name: "Puzzle", ParentID: "12006", Store: "mac"},
	{ID: "12213", Name: "Racing", ParentID: "12006", Store: "mac"},
	{ID: "12214", Name: "Role Playing", ParentID: "12006", Store: "mac"},
	{ID: "12215", Name: "Simulation", ParentID: "12006", Store: "mac"},
	{ID: "12216", Name: "Sports", ParentID: "12006", Store: "mac"},
	{ID: "12217", Name: "Strategy", ParentID: "12006", Store: "mac"},
	{ID: "12218", Name: "Trivia", ParentID: "12006", Store: "mac"},
	{ID: "12219", Name: "Word", ParentID: "12006", Store: "mac"},
	{ID: "12007", Name: "Health & Fitness", ParentID: "39", Store: "mac"},
	{ID: "12008", Name: "Lifestyle", ParentID: "39", Store: "mac"},
	{ID: "12010", Name: "Medical", ParentID: "39", Store: "mac"},
	{ID: "12011", Name: "Music", ParentID: "39", Store: "mac"},
	{ID: "12012", Name: "News", ParentID: "39", Store: "mac"},
	{ID: "12013", Name: "Photography", ParentID: "39", Store: "mac"},
	{ID: "12014", Name: "Productivity", ParentID: "39", Store: "mac"},
	{ID: "12015", Name: "Reference", ParentID: "39", Store: "mac"},
	{ID: "12016", Name: "Social Networking", ParentID: "39", Store: "mac"},
	{ID: "12017", Name: "Sports", ParentID: "39", Store: "mac"},
	{ID: "12018", Name: "Travel", ParentID: "39", Store: "mac"},
	{ID: "12019", Name: "Utilities", ParentID: "39", Store: "mac"},
	{ID: "12020", Name: "Video", ParentID: "39", Store: "mac"},
	{ID: "12021", Name: "Weather", ParentID: "39", Store: "mac"},
	{ID: "12022", Name: "Graphics & Design", ParentID: "39", Store: "mac"},
}

var genreByID = func() map[string]Genre {
	out := make(map[string]Genre, len(genreTable))
	for _, g := range genreTable {
		out[g.ID] = g
	}
	return out
}()

// genreAliases are the learn aliases from spec.yaml (app_genre and
// game_genre seeds) plus a few chart-page spellings. Keys are normalized with
// normalizeGenreName; values are genre ids.
var genreAliases = map[string]string{
	"all":                    "36",
	"all apps":               "36",
	"overall":                "36",
	"app store":              "36",
	"game":                   "6014",
	"gaming":                 "6014",
	"all games":              "6014",
	"ebooks":                 "6018",
	"dev tools":              "6026",
	"devtools":               "6026",
	"edu":                    "6017",
	"learning":               "6017",
	"fintech":                "6015",
	"banking":                "6015",
	"food":                   "6023",
	"design":                 "6027",
	"graphics":               "6027",
	"health":                 "6013",
	"fitness":                "6013",
	"magazines":              "6021",
	"newspapers":             "6021",
	"maps":                   "6010",
	"photo":                  "6008",
	"video":                  "6008",
	"ecommerce":              "6024",
	"social":                 "6005",
	"social media":           "6005",
	"utility":                "6002",
	"tools":                  "6002",
	"action games":           "7001",
	"adventure games":        "7002",
	"board games":            "7004",
	"card games":             "7005",
	"casual games":           "7003",
	"arcade":                 "7003",
	"casino games":           "7006",
	"dice games":             "7007",
	"educational games":      "7008",
	"family games":           "7009",
	"music games":            "7011",
	"puzzle games":           "7012",
	"puzzles":                "7012",
	"racing games":           "7013",
	"rpg":                    "7014",
	"role playing":           "7014",
	"role playing games":     "7014",
	"sim":                    "7015",
	"simulation games":       "7015",
	"sports games":           "7016",
	"strategy games":         "7017",
	"quiz":                   "7018",
	"trivia games":           "7018",
	"word games":             "7019",
	"mac app store":          "39",
	"magazines & newspapers": "6021",
}

// KidsAgeBand is one Kids chart age band (the chart page's ageBandId).
type KidsAgeBand struct {
	ID   string `json:"ageBandId"`
	Name string `json:"name"`
}

// KidsAgeBands are the Kids chart bands on the iPhone and iPad chart pages
// (genre 36 plus ageBandId). "0" is All Kids.
var KidsAgeBands = []KidsAgeBand{
	{ID: "0", Name: "All Kids"},
	{ID: "10000", Name: "Ages 5 & Under"},
	{ID: "10001", Name: "Ages 6-8"},
	{ID: "10002", Name: "Ages 9-11"},
}

// Genres returns the genre table for one store ("ios", "mac" or "" for both),
// sorted by numeric id.
func Genres(store string) []Genre {
	out := make([]Genre, 0, len(genreTable))
	for _, g := range genreTable {
		if store == "" || g.Store == store {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := strconv.Atoi(out[i].ID)
		b, _ := strconv.Atoi(out[j].ID)
		return a < b
	})
	return out
}

// GenreByID returns the genre for an id from either store.
func GenreByID(id string) (Genre, bool) {
	g, ok := genreByID[strings.TrimSpace(id)]
	return g, ok
}

// GenreName returns the display name for a genre id, or "" when unknown.
func GenreName(id string) string {
	if g, ok := GenreByID(id); ok {
		return g.Name
	}
	return ""
}

// normalizeGenreName lowercases, turns "&" into "and", drops apostrophes and
// folds hyphens, slashes and underscores to spaces so "Role-Playing",
// "role_playing" and "Role Playing" compare equal.
func normalizeGenreName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "’", "")
	s = strings.Map(func(r rune) rune {
		switch r {
		case '-', '_', '/', ',', '.':
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

var normalizedAliases = func() map[string]string {
	out := make(map[string]string, len(genreAliases))
	for k, v := range genreAliases {
		out[normalizeGenreName(k)] = v
	}
	return out
}()

// ResolveGenre resolves a genre id ("6014") or a name/alias ("games",
// "puzzle", "Developer Tools", "rpg"), case-insensitively, against the iOS
// tree the chart pages use. A name shared by an app genre and a game subgenre
// ("music", "sports") resolves to the app genre; prefix it with "games:" or
// "games/" (or suffix " games") for the game subgenre.
func ResolveGenre(s string) (Genre, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Genre{}, fmt.Errorf("genre is empty; use an id such as 6014 or a name such as games")
	}
	if _, err := strconv.Atoi(raw); err == nil {
		if g, ok := genreByID[raw]; ok {
			return g, nil
		}
		return Genre{}, fmt.Errorf("unknown App Store genre id %q; see 'categories list' for valid ids", raw)
	}
	lower := strings.ToLower(raw)
	gamesScope := false
	for _, prefix := range []string{"games:", "games/", "game:", "game/"} {
		if strings.HasPrefix(lower, prefix) {
			gamesScope = true
			raw = raw[len(prefix):]
			break
		}
	}
	name := normalizeGenreName(raw)
	if gamesScope {
		for _, g := range genreTable {
			if g.Store == "ios" && g.ParentID == GamesGenreID && normalizeGenreName(g.Name) == name {
				return g, nil
			}
		}
		if id, ok := normalizedAliases[name+" games"]; ok {
			return genreByID[id], nil
		}
		return Genre{}, fmt.Errorf("unknown game subgenre %q; use a name such as puzzle or an id such as 7012", raw)
	}
	if id, ok := normalizedAliases[name]; ok {
		return genreByID[id], nil
	}
	// Top-level iOS genres first, then game subgenres, then other subgenres.
	for _, pass := range []func(Genre) bool{
		func(g Genre) bool { return g.Store == "ios" && (g.ParentID == "36" || g.ID == "36") },
		func(g Genre) bool { return g.Store == "ios" && g.ParentID == GamesGenreID },
		func(g Genre) bool { return g.Store == "ios" },
	} {
		for _, g := range genreTable {
			if pass(g) && normalizeGenreName(g.Name) == name {
				return g, nil
			}
		}
	}
	return Genre{}, fmt.Errorf("unknown App Store genre %q; use an id such as 6014 or a name such as games, puzzle or developer tools", s)
}

// ResolveAgeBand resolves a Kids age band id ("10000") or name ("5-under",
// "ages 6-8", "6-8", "all kids", "kids") to its ageBandId. Only an explicit
// value ("all", "kids", "all kids" or "0") selects the All Kids chart
// (ageBandId 0): an empty value is an error, because a caller that resolved
// an unset --age-band unconditionally would otherwise turn every overall
// chart into the Kids chart. Callers with an optional flag check for the
// empty value first (the CLI's resolveAgeBand returns no band for it).
func ResolveAgeBand(s string) (KidsAgeBand, error) {
	raw := strings.ToLower(strings.TrimSpace(s))
	switch raw {
	case "":
		return KidsAgeBand{}, fmt.Errorf("kids age band is empty; use all, 5-under, 6-8 or 9-11 (ids 0, 10000, 10001, 10002), or leave the flag unset for the overall chart")
	case "kids", "all", "all kids", "0":
		return KidsAgeBands[0], nil
	case "10000", "5", "5-under", "5 and under", "ages 5 & under", "ages 5 and under", "under 5", "0-5":
		return KidsAgeBands[1], nil
	case "10001", "6-8", "ages 6-8", "6 to 8":
		return KidsAgeBands[2], nil
	case "10002", "9-11", "ages 9-11", "9 to 11":
		return KidsAgeBands[3], nil
	}
	return KidsAgeBand{}, fmt.Errorf("unknown Kids age band %q; use all, 5-under, 6-8 or 9-11 (ids 0, 10000, 10001, 10002)", s)
}

// KidsAgeBandByID returns the Kids band with the given ageBandId ("0",
// "10000", "10001", "10002").
func KidsAgeBandByID(id string) (KidsAgeBand, bool) {
	id = strings.TrimSpace(id)
	for _, b := range KidsAgeBands {
		if b.ID == id {
			return b, true
		}
	}
	return KidsAgeBand{}, false
}
