package omnilogic

import "strings"

// LightShows is the full ColorLogic show catalog as documented by Hayward
// and the omnilogic-python wrapper. V2 lights accept the full list; V1
// lights only the first 9. Numbering matches Hayward's controller firmware.
var LightShows = []LightShow{
	{0, "Off", false},
	{1, "Voodoo Lounge", false},
	{2, "Deep Blue Sea", false},
	{3, "Royal Blue", false},
	{4, "Afternoon Skies", false},
	{5, "Aqua Green", false},
	{6, "Emerald", false},
	{7, "Cloud White", false},
	{8, "Warm Red", false},
	{9, "Flamingo", false},
	{10, "Vivid Violet", false},
	{11, "Sangria", false},
	{12, "Twilight", false},
	{13, "Tranquility", false},
	{14, "Gemstone", false},
	{15, "USA!", false},
	{16, "Mardi Gras", false},
	{17, "Cool Cabaret", false},
	{18, "Yellow", true},
	{19, "Orange", true},
	{20, "Gold", true},
	{21, "Mint", true},
	{22, "Teal", true},
	{23, "Burnt Orange", true},
	{24, "Pure White", true},
	{25, "Crisp White", true},
	{26, "Warm White", true},
	{27, "Bright Yellow", true},
}

// ResolveShow matches a user-supplied show name or numeric ID against the
// catalog. Names match case-insensitively and ignore non-alphanumeric chars
// (so "deep-blue-sea", "Deep Blue Sea", and "deepbluesea" all match).
func ResolveShow(input string) (LightShow, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return LightShow{}, false
	}
	// Numeric ID first.
	if n := atoiSafe(input); n >= 0 {
		for _, s := range LightShows {
			if s.ID == n {
				return s, true
			}
		}
		return LightShow{}, false
	}
	wanted := normalizeShow(input)
	for _, s := range LightShows {
		if normalizeShow(s.Name) == wanted {
			return s, true
		}
	}
	return LightShow{}, false
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func normalizeShow(s string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	return out.String()
}
