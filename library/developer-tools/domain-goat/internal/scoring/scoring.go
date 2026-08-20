// Package scoring computes brandability scores for domain candidates.
package scoring

import (
	"strings"
	"unicode"
)

// Score is a composite brandability score in 0..100 with components.
type Score struct {
	FQDN          string `json:"fqdn"`
	Label         string `json:"label"`
	TLD           string `json:"tld"`
	Total         int    `json:"total"`
	Length        int    `json:"length"`
	Syllables     int    `json:"syllables"`
	Pronounceable int    `json:"pronounceable"`
	VowelRatio    int    `json:"vowel_ratio"`
	DictWord      bool   `json:"dictionary_word"`
	HackStyle     bool   `json:"hack_style"`
	Palindrome    bool   `json:"palindrome"`
	Repeats       int    `json:"repeats"`
	Hyphens       int    `json:"hyphens"`
	Digits        int    `json:"digits"`
	TLDPrestige   int    `json:"tld_prestige"`
}

// tldPrestige is a curated tier table. Higher is better.
var tldPrestige = map[string]int{
	"com": 100, "net": 60, "org": 60,
	"io": 80, "ai": 90, "app": 70, "dev": 75, "co": 65,
	"studio": 55, "design": 55, "agency": 40, "tech": 50,
	"xyz": 20, "online": 15, "site": 15, "info": 25,
	"biz": 15, "us": 30, "me": 50, "tv": 45, "cc": 25,
}

// commonWords is a tiny built-in dictionary used as a dictionary-match signal.
var commonWords = map[string]bool{
	"apple": true, "art": true, "auto": true, "blue": true, "book": true,
	"brand": true, "build": true, "cake": true, "care": true, "city": true,
	"cloud": true, "code": true, "craft": true, "data": true, "deep": true,
	"design": true, "edge": true, "fast": true, "fire": true, "flow": true,
	"forge": true, "fox": true, "free": true, "fresh": true, "front": true,
	"game": true, "gear": true, "gold": true, "good": true, "green": true,
	"grow": true, "hawk": true, "home": true, "house": true, "hub": true,
	"hunt": true, "idea": true, "jet": true, "lab": true, "land": true,
	"life": true, "light": true, "live": true, "lock": true, "loop": true,
	"love": true, "lumen": true, "make": true, "match": true, "meta": true,
	"mind": true, "moon": true, "next": true, "north": true, "note": true,
	"nova": true, "open": true, "orbit": true, "pad": true, "pages": true,
	"paint": true, "park": true, "path": true, "peak": true, "pen": true,
	"phone": true, "pixel": true, "place": true, "play": true, "plus": true,
	"port": true, "post": true, "press": true, "prime": true, "pro": true,
	"pulse": true, "punk": true, "quick": true, "rain": true, "raw": true,
	"red": true, "rise": true, "river": true, "rock": true, "root": true,
	"safe": true, "scout": true, "sea": true, "shop": true, "shore": true,
	"sign": true, "silver": true, "smart": true, "snow": true, "space": true,
	"spark": true, "star": true, "stay": true, "stone": true, "stream": true,
	"studio": true, "sun": true, "swift": true, "table": true, "tap": true,
	"team": true, "thinker": true, "thrive": true, "tide": true, "tilt": true,
	"time": true, "tone": true, "tower": true, "track": true, "trail": true,
	"tribe": true, "true": true, "tune": true, "wave": true, "way": true,
	"web": true, "well": true, "west": true, "white": true, "wild": true,
	"wing": true, "wise": true, "wolf": true, "wood": true, "work": true,
	"world": true, "yard": true, "zone": true, "zen": true, "kindred": true,
	"novella": true, "voice": true, "verse": true, "ember": true, "atlas": true,
	"haven": true, "harbor": true, "summit": true,
}

// vowels for vowel-ratio calc.
var vowels = "aeiouy"

// Compute returns a Score for a domain (or label).
func Compute(fqdn string) Score {
	fqdn = strings.ToLower(strings.TrimSpace(fqdn))
	label, tld := splitOnFirstDot(fqdn)
	if label == "" {
		label = fqdn
	}
	s := Score{
		FQDN:  fqdn,
		Label: label,
		TLD:   tld,
	}
	s.Length = len(label)
	s.Syllables = countSyllables(label)
	s.VowelRatio = vowelRatioPct(label)
	s.DictWord = commonWords[label] || containsCommonWord(label)
	s.HackStyle = isHackStyle(fqdn)
	s.Palindrome = isPalindrome(label)
	s.Repeats = countRepeats(label)
	s.Hyphens = strings.Count(label, "-")
	s.Digits = countDigits(label)
	s.Pronounceable = pronounceabilityScore(label)
	s.TLDPrestige = tldPrestige[tld]

	// Composite — clamp components to 0..100, weight, sum
	lengthScore := lengthQualityScore(s.Length)
	pronScore := s.Pronounceable
	vowelScore := scoreVowelRatio(s.VowelRatio)
	dictScore := 0
	if s.DictWord {
		dictScore = 70
	}
	hackBonus := 0
	if s.HackStyle {
		hackBonus = 60
	}
	palindromeBonus := 0
	if s.Palindrome && s.Length >= 4 {
		palindromeBonus = 50
	}
	hyphenPenalty := s.Hyphens * 15
	digitPenalty := s.Digits * 8
	tldScore := s.TLDPrestige

	// Weighted composite — domain experts care most about length + pronounceability + tld.
	raw := (lengthScore*30 + pronScore*25 + vowelScore*10 + dictScore*10 + hackBonus*8 + palindromeBonus*2 + tldScore*15) / 100
	raw -= hyphenPenalty + digitPenalty
	if raw < 0 {
		raw = 0
	}
	if raw > 100 {
		raw = 100
	}
	s.Total = raw
	return s
}

// lengthQualityScore returns 0..100, peaking at 4-7 chars.
func lengthQualityScore(n int) int {
	switch {
	case n <= 3:
		return 60
	case n <= 5:
		return 100
	case n <= 7:
		return 90
	case n <= 9:
		return 65
	case n <= 12:
		return 40
	case n <= 16:
		return 20
	default:
		return 5
	}
}

// scoreVowelRatio peaks near 40-50%.
func scoreVowelRatio(pct int) int {
	if pct >= 35 && pct <= 55 {
		return 100
	}
	if pct >= 25 && pct < 35 {
		return 70
	}
	if pct > 55 && pct <= 70 {
		return 65
	}
	return 30
}

func vowelRatioPct(s string) int {
	if len(s) == 0 {
		return 0
	}
	n := 0
	for _, c := range s {
		if strings.ContainsRune(vowels, c) {
			n++
		}
	}
	return n * 100 / len(s)
}

func countSyllables(s string) int {
	// Simple heuristic: count vowel groups.
	n := 0
	prev := false
	for _, c := range s {
		isV := strings.ContainsRune(vowels, c)
		if isV && !prev {
			n++
		}
		prev = isV
	}
	if n == 0 {
		n = 1
	}
	return n
}

// pronounceabilityScore returns 0..100 based on consonant clusters.
func pronounceabilityScore(s string) int {
	if s == "" {
		return 0
	}
	cluster := 0
	maxClus := 0
	hardClus := 0
	for _, c := range s {
		if !unicode.IsLetter(c) {
			cluster = 0
			continue
		}
		if strings.ContainsRune(vowels, c) {
			if cluster > maxClus {
				maxClus = cluster
			}
			cluster = 0
		} else {
			cluster++
			if cluster >= 4 {
				hardClus++
			}
		}
	}
	if cluster > maxClus {
		maxClus = cluster
	}
	score := 100 - maxClus*10 - hardClus*15
	if score < 0 {
		score = 0
	}
	return score
}

func countRepeats(s string) int {
	n := 0
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			n++
		}
	}
	return n
}

func countDigits(s string) int {
	n := 0
	for _, c := range s {
		if unicode.IsDigit(c) {
			n++
		}
	}
	return n
}

func isPalindrome(s string) bool {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		if r[i] != r[j] {
			return false
		}
	}
	return len(r) > 1
}

// hackWords supplements commonWords with hack-style targets that aren't
// general-purpose enough to live in the brandability dictionary.
var hackWords = map[string]bool{
	"delicious": true, "kubes": true, "instagrm": true, "goal": true,
	"flickr": true, "tumblr": true, "spotify": true,
}

func isHackStyle(fqdn string) bool {
	// fqdn like del.icio.us or kub.es — the union ignoring dots spells a word.
	noDots := strings.ReplaceAll(fqdn, ".", "")
	if commonWords[noDots] || hackWords[noDots] {
		return true
	}
	// word.tld where tld is a 2-letter ccTLD AND the joined string is a word.
	parts := strings.Split(fqdn, ".")
	if len(parts) >= 2 {
		joined := strings.Join(parts, "")
		if commonWords[joined] || hackWords[joined] {
			return true
		}
	}
	return false
}

func containsCommonWord(s string) bool {
	for w := range commonWords {
		if len(w) >= 4 && strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func splitOnFirstDot(s string) (label, tld string) {
	idx := strings.Index(s, ".")
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}
