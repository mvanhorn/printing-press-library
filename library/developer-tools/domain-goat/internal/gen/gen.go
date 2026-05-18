// Package gen contains offline domain-name generators: affix, mix, blend,
// portmanteau, rhyme, hack-style, and dnstwist-style permutations.
package gen

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// DefaultPrefixes is a curated prefix list used for "get-", "my-" style combos.
var DefaultPrefixes = []string{"get", "my", "go", "use", "try", "join", "the", "with"}

// DefaultSuffixes is a curated suffix list.
var DefaultSuffixes = []string{"hq", "ly", "io", "app", "hub", "labs", "studio", "kit", "now", "co"}

// Common small TLDs used by Hack-style generators (for `kub.es`, `del.icio.us`).
var hackTLDs = []string{"al", "ar", "as", "at", "be", "by", "ca", "ch", "cl", "co",
	"cz", "de", "dk", "do", "ee", "es", "eu", "fi", "fr", "gg", "gr", "hk",
	"hm", "ht", "id", "ie", "il", "im", "in", "io", "ir", "is", "it",
	"je", "jp", "kg", "kr", "kz", "la", "li", "lt", "lu", "lv", "ly",
	"md", "me", "mg", "mn", "ms", "mu", "mx", "my", "no", "nu", "nz",
	"pl", "pt", "qa", "ro", "rs", "ru", "se", "sg", "sh", "si", "sk",
	"st", "sx", "to", "tr", "tv", "us", "vc", "ws"}

// Pair generates name × tld matrix.
func Pair(names []string, tlds []string) []string {
	out := make([]string, 0, len(names)*len(tlds))
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" {
			continue
		}
		for _, t := range tlds {
			t = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(t, ".")))
			if t == "" {
				continue
			}
			out = append(out, n+"."+t)
		}
	}
	return out
}

// Affix produces prefixed + suffixed variants. If lists are nil/empty, uses defaults.
func Affix(seed string, prefixes, suffixes []string) []string {
	if seed == "" {
		return nil
	}
	if len(prefixes) == 0 {
		prefixes = DefaultPrefixes
	}
	if len(suffixes) == 0 {
		suffixes = DefaultSuffixes
	}
	set := map[string]struct{}{}
	for _, p := range prefixes {
		set[strings.ToLower(p)+seed] = struct{}{}
		set[strings.ToLower(p)+"-"+seed] = struct{}{}
	}
	for _, s := range suffixes {
		set[seed+strings.ToLower(s)] = struct{}{}
		set[seed+"-"+strings.ToLower(s)] = struct{}{}
	}
	return toSorted(set)
}

// Blend creates portmanteau-style merges of two seeds: overlap-merge by shared
// letters, plus simple half+half blends.
func Blend(a, b string) []string {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	set := map[string]struct{}{}
	if a == "" || b == "" {
		return nil
	}
	// overlap merge: largest suffix of a that's prefix of b
	for n := minInt(len(a), len(b)); n >= 1; n-- {
		if a[len(a)-n:] == b[:n] {
			set[a+b[n:]] = struct{}{}
			break
		}
	}
	// half/half
	if len(a) >= 2 && len(b) >= 2 {
		set[a[:len(a)/2+1]+b[len(b)/2:]] = struct{}{}
		set[a[:2]+b] = struct{}{}
		set[a+b[len(b)-2:]] = struct{}{}
	}
	return toSorted(set)
}

// Mix combines multiple seeds with prefixes and suffixes between them.
func Mix(seeds []string) []string {
	if len(seeds) < 1 {
		return nil
	}
	set := map[string]struct{}{}
	for i := range seeds {
		for j := range seeds {
			if i == j {
				continue
			}
			a := strings.ToLower(seeds[i])
			b := strings.ToLower(seeds[j])
			set[a+b] = struct{}{}
			set[a+"-"+b] = struct{}{}
		}
	}
	return toSorted(set)
}

// Hack returns hack-style splits: take a word like "kubes", try splitting at
// every internal point and check if the suffix is a known short TLD.
func Hack(word string) []string {
	word = strings.ToLower(strings.TrimSpace(word))
	if word == "" {
		return nil
	}
	hackSet := map[string]struct{}{}
	for _, tld := range hackTLDs {
		if strings.HasSuffix(word, tld) && len(word) > len(tld) {
			label := word[:len(word)-len(tld)]
			hackSet[label+"."+tld] = struct{}{}
		}
	}
	// Multi-segment splits like del.icio.us — try 3-way split where each segment is non-empty
	if len(word) >= 5 {
		for i := 1; i < len(word)-3; i++ {
			for _, tld := range []string{"us", "io", "be", "to"} {
				if len(word)-i-2 < 1 {
					continue
				}
				if strings.HasSuffix(word, tld) {
					mid := word[i : len(word)-len(tld)]
					if mid == "" {
						continue
					}
					hackSet[word[:i]+"."+mid+"."+tld] = struct{}{}
				}
			}
		}
	}
	return toSorted(hackSet)
}

// Rhyme generates simple rhyming candidates by replacing the leading consonant
// cluster with common alternatives.
func Rhyme(word string) []string {
	word = strings.ToLower(strings.TrimSpace(word))
	if word == "" {
		return nil
	}
	starts := []string{"b", "br", "c", "cl", "cr", "d", "dr", "f", "fl", "fr",
		"g", "gl", "gr", "h", "j", "k", "l", "m", "n", "p", "pl", "pr",
		"qu", "r", "s", "sh", "sl", "sn", "sp", "st", "t", "tr", "v", "w"}
	// Find end of leading consonant cluster
	cut := 0
	for cut < len(word) {
		c := word[cut]
		if !strings.ContainsRune("bcdfghjklmnpqrstvwxz", rune(c)) {
			break
		}
		cut++
	}
	tail := word[cut:]
	set := map[string]struct{}{}
	for _, s := range starts {
		if s == word[:cut] {
			continue
		}
		set[s+tail] = struct{}{}
	}
	return toSorted(set)
}

// Permutation types (dnstwist-style).
type Permutation struct {
	FQDN   string `json:"fqdn"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
}

// Permute generates similar-name variations for a FQDN.
func Permute(fqdn string, kinds []string) []Permutation {
	fqdn = strings.ToLower(strings.TrimSpace(fqdn))
	if fqdn == "" {
		return nil
	}
	label, tld := splitFirstDot(fqdn)
	if tld == "" {
		tld = "com"
	}
	if len(kinds) == 0 {
		kinds = []string{"omission", "insertion", "replacement", "transposition",
			"repetition", "vowel-swap", "hyphenation", "addition", "tld-swap"}
	}
	set := map[string]string{}
	for _, k := range kinds {
		switch k {
		case "omission":
			for i := range label {
				v := label[:i] + label[i+1:]
				if v != "" && v != label {
					set[v+"."+tld] = k
				}
			}
		case "insertion":
			for i := 0; i <= len(label); i++ {
				for c := 'a'; c <= 'z'; c++ {
					v := label[:i] + string(c) + label[i:]
					if v != label {
						set[v+"."+tld] = k
					}
				}
			}
		case "replacement":
			for i := range label {
				for c := 'a'; c <= 'z'; c++ {
					if rune(label[i]) == c {
						continue
					}
					v := label[:i] + string(c) + label[i+1:]
					set[v+"."+tld] = k
				}
			}
		case "transposition":
			for i := 0; i+1 < len(label); i++ {
				if label[i] == label[i+1] {
					continue
				}
				v := label[:i] + string(label[i+1]) + string(label[i]) + label[i+2:]
				set[v+"."+tld] = k
			}
		case "repetition":
			for i := range label {
				v := label[:i] + string(label[i]) + label[i:]
				set[v+"."+tld] = k
			}
		case "vowel-swap":
			vowels := "aeiou"
			for i := range label {
				if !strings.ContainsRune(vowels, rune(label[i])) {
					continue
				}
				for _, v := range vowels {
					if rune(label[i]) == v {
						continue
					}
					alt := label[:i] + string(v) + label[i+1:]
					set[alt+"."+tld] = k
				}
			}
		case "hyphenation":
			for i := 1; i < len(label); i++ {
				v := label[:i] + "-" + label[i:]
				set[v+"."+tld] = k
			}
		case "addition":
			for c := 'a'; c <= 'z'; c++ {
				set[label+string(c)+"."+tld] = k
				set[string(c)+label+"."+tld] = k
			}
		case "tld-swap":
			for _, alt := range []string{"com", "net", "org", "io", "ai", "app", "dev",
				"co", "studio", "design", "agency", "tech", "xyz", "site", "me"} {
				if alt == tld {
					continue
				}
				set[label+"."+alt] = k
			}
		case "homoglyph":
			// Limited cyrillic homoglyph swaps to avoid IDN encoding failures.
			swaps := map[byte]string{'a': "а", 'e': "е", 'o': "о", 'p': "р", 'c': "с"}
			for i := range label {
				if alt, ok := swaps[label[i]]; ok {
					v := label[:i] + alt + label[i+1:]
					set[v+"."+tld] = k
				}
			}
		case "bitsquatting":
			for i := range label {
				for b := 0; b < 7; b++ {
					alt := byte(label[i]) ^ byte(1<<b)
					if alt < 'a' || alt > 'z' {
						continue
					}
					v := label[:i] + string(alt) + label[i+1:]
					set[v+"."+tld] = k
				}
			}
		case "subdomain":
			for i := 1; i < len(label)-1; i++ {
				v := label[:i] + "." + label[i:]
				set[v+"."+tld] = k
			}
		}
	}
	out := make([]Permutation, 0, len(set))
	for v, kind := range set {
		// drop the original
		if v == fqdn {
			continue
		}
		// drop pathological lengths
		if utf8.RuneCountInString(v) > 80 {
			continue
		}
		out = append(out, Permutation{FQDN: v, Kind: kind, Source: "permute"})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].FQDN < out[j].FQDN
	})
	return out
}

func toSorted(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func splitFirstDot(s string) (a, b string) {
	idx := strings.Index(s, ".")
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
