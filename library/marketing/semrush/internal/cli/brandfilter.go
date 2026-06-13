// Brand-filter helpers — strips branded keywords (self or competitor) from
// KMT results. The use case is "non-branded keyword research" deliverables
// where any phrase that mentions a brand (the client's or a competitor's) is
// noise. Two inputs:
//
//   - The target domain (--domain) auto-derives self-brand tokens (e.g.
//     "nationaltiles.com.au" → ["nationaltiles", "national tiles"])
//   - An explicit --exclude list of competitor brand terms
//
// Matching is case-insensitive, substring-against the keyword phrase.
package cli

import (
	"regexp"
	"strings"
)

// brandFilterConfig is the resolved filter for one research call.
type brandFilterConfig struct {
	terms []string // lowercase tokens to exclude (substring match against the phrase)
}

func newBrandFilter(domain string, excludeSelf bool, excludeList string) *brandFilterConfig {
	cfg := &brandFilterConfig{}
	if excludeSelf && domain != "" {
		cfg.terms = append(cfg.terms, deriveBrandTokens(domain)...)
	}
	if excludeList != "" {
		for _, raw := range strings.Split(excludeList, ",") {
			t := strings.ToLower(strings.TrimSpace(raw))
			if t != "" {
				cfg.terms = append(cfg.terms, t)
			}
		}
	}
	cfg.terms = dedupeStrings(cfg.terms)
	return cfg
}

// deriveBrandTokens strips a domain to the brand-relevant root and produces
// a small set of candidate tokens to check against keyword phrases.
//
// "nationaltiles.com.au"   → ["nationaltiles", "national tiles", "nationaltile"]
// "www.kickassproducts.com.au" → ["kickassproducts", "kickass products", "kickassproduct"]
// "client.com"             → ["client"]
//
// We split CamelCase runs and well-known concatenated brand shapes by
// inserting spaces between repeated lowercase clusters. Imperfect, but it
// catches the common case the user flagged (the self-brand keyword written
// as either "nationaltiles" or "national tiles").
func deriveBrandTokens(domain string) []string {
	host := strings.ToLower(domain)
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "www.")
	// Strip ETLDs (very simple — handles .com, .co.uk, .com.au, etc.)
	parts := strings.Split(host, ".")
	if len(parts) == 0 {
		return nil
	}
	root := parts[0]
	if root == "" {
		return nil
	}
	tokens := []string{root}
	// If the root is a concatenated compound (no separators), try to split it
	// on common boundary heuristics: known suffixes like "tiles", "store",
	// "shop", "supply", "group", "products", "online" tend to be the
	// rightmost half.
	suffixes := []string{
		"tiles", "store", "stores", "shop", "shops", "supply", "supplies",
		"group", "products", "online", "direct", "central", "marketplace",
		"warehouse", "outlet", "depot", "world", "express", "club", "hub",
	}
	for _, suf := range suffixes {
		if root != suf && strings.HasSuffix(root, suf) {
			prefix := strings.TrimSuffix(root, suf)
			if prefix != "" {
				tokens = append(tokens, prefix+" "+suf)
				tokens = append(tokens, prefix) // brand-stem alone (e.g. "national")
			}
		}
	}
	// Singular form (drop trailing s) as a low-cost catch — only for words >4 chars
	if len(root) > 4 && strings.HasSuffix(root, "s") {
		tokens = append(tokens, strings.TrimSuffix(root, "s"))
	}
	return dedupeStrings(tokens)
}

// matches returns true if the phrase contains any of the brand tokens as a
// whole-word substring. Whole-word means token surrounded by either string
// boundary or non-letter/non-digit chars — this prevents "national" from
// matching "international" but does match "national tiles" and "nationaltiles".
func (b *brandFilterConfig) matches(phrase string) (bool, string) {
	if b == nil || len(b.terms) == 0 || phrase == "" {
		return false, ""
	}
	p := strings.ToLower(phrase)
	for _, t := range b.terms {
		if isWholeWordSubstring(p, t) {
			return true, t
		}
	}
	return false, ""
}

// isWholeWordSubstring is a permissive whole-word check. For multi-word
// tokens (e.g. "national tiles") we treat the entire token as the substring
// and check the boundary chars before/after the match.
func isWholeWordSubstring(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] != needle {
			continue
		}
		// boundary check before
		if i > 0 {
			prev := haystack[i-1]
			if isWordChar(prev) {
				continue
			}
		}
		// boundary check after
		end := i + len(needle)
		if end < len(haystack) {
			next := haystack[end]
			if isWordChar(next) {
				continue
			}
		}
		return true
	}
	return false
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// kmtRowPhrase extracts the keyword phrase string from a row map, tolerant
// of upstream key casing.
func kmtRowPhrase(row map[string]any) string {
	for _, k := range []string{"phrase", "Keyword", "keyword"} {
		if v, ok := row[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// applyBrandFilter walks rows and drops any whose phrase matches a brand
// token. Returns filtered rows and the count dropped.
func applyBrandFilter(rows []map[string]any, b *brandFilterConfig) ([]map[string]any, int) {
	if b == nil || len(b.terms) == 0 {
		return rows, 0
	}
	out := make([]map[string]any, 0, len(rows))
	dropped := 0
	for _, r := range rows {
		phrase := kmtRowPhrase(r)
		if hit, _ := b.matches(phrase); hit {
			dropped++
			continue
		}
		out = append(out, r)
	}
	return out, dropped
}

// keepWordRegexp is a compiled-once whitespace-collapse helper. (Not used in
// the brand-filter path itself but useful for downstream normalizers.)
var keepWordRegexp = regexp.MustCompile(`\s+`)
