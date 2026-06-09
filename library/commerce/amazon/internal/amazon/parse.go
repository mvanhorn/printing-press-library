package amazon

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// parseCartHTML extracts CartLines from a /gp/cart/view.html response.
//
// Amazon's cart page is HTML-only; we read the per-line data attributes:
//
//	<div data-asin="B0FOO" data-quantity="2" data-item-name="..." ...>
//
// When data-attributes aren't present (account display variant), we fall back
// to a "data-item-asin" + "data-quantity" pair. Best-effort; the canonical
// confirmation that an add succeeded is the cart's row count delta.
func parseCartHTML(body string) []CartLine {
	var lines []CartLine
	for _, re := range []*regexp.Regexp{cartLineRe1, cartLineRe2} {
		matches := re.FindAllStringSubmatch(body, -1)
		for _, m := range matches {
			asin := m[1]
			if asin == "" {
				continue
			}
			line := CartLine{ASIN: asin, Quantity: 1}
			if len(m) > 2 && m[2] != "" {
				if q, err := strconv.Atoi(m[2]); err == nil && q > 0 {
					line.Quantity = q
				}
			}
			if len(m) > 3 && m[3] != "" {
				line.Title = html2text(m[3])
			}
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			break
		}
	}
	return dedupeCartLines(lines)
}

var (
	cartLineRe1 = regexp.MustCompile(`(?s)data-asin="([A-Z0-9]{10})"[^>]*data-quantity="(\d+)"[^>]*data-item-name="([^"]+)"`)
	cartLineRe2 = regexp.MustCompile(`(?s)data-item-asin="([A-Z0-9]{10})"[^>]*data-item-quantity="(\d+)"[^>]*data-item-title="([^"]+)"`)
)

func dedupeCartLines(in []CartLine) []CartLine {
	seen := make(map[string]int, len(in))
	var out []CartLine
	for _, line := range in {
		if idx, ok := seen[line.ASIN]; ok {
			out[idx].Quantity += line.Quantity
			if out[idx].Title == "" {
				out[idx].Title = line.Title
			}
			continue
		}
		seen[line.ASIN] = len(out)
		out = append(out, line)
	}
	return out
}

// extractAccountName scrapes the "Hello, <name>" greeting from the homepage.
// Returns empty string if not found (treat as "logged out" at the caller).
var accountGreetingRe = regexp.MustCompile(`(?i)<span[^>]*id="nav-link-accountList-nav-line-1"[^>]*>([^<]+)<`)

func extractAccountName(body string) string {
	m := accountGreetingRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(html2text(m[1]))
}

// parseSPCTokens extracts the hidden form tokens from the SPC checkout page.
// These tokens (purchase_id, anti-csrf, anti-csrftoken-a2z, etc.) are required
// to round-trip the place-your-order POST.
//
// Returns an error when the page doesn't look like the SPC page (the user
// probably has an empty cart or Amazon redirected to the cart view).
func parseSPCTokens(body string) (map[string]string, error) {
	if !strings.Contains(body, "spc-place-order-button") && !strings.Contains(body, "placeYourOrder") {
		return nil, errors.New("checkout page did not contain the place-order form; the cart may be empty or your account needs attention on amazon.com")
	}
	tokens := make(map[string]string)
	for _, m := range hiddenInputRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		val := m[2]
		if isInterestingToken(name) {
			tokens[name] = html2text(val)
		}
	}
	return tokens, nil
}

var hiddenInputRe = regexp.MustCompile(`(?i)<input[^>]*type="hidden"[^>]*name="([^"]+)"[^>]*value="([^"]*)"`)

func isInterestingToken(name string) bool {
	switch name {
	case "purchase_id", "purchaseId", "pipelineType", "anti-csrftoken-a2z", "ue_back",
		"clientIp", "ref_", "fwcim_session_id", "shippingOption", "session-id":
		return true
	}
	return strings.HasPrefix(name, "purchase") ||
		strings.HasPrefix(name, "merchantId") ||
		strings.HasPrefix(name, "ie")
}

var thankYouOrderRe = regexp.MustCompile(`(?i)order(?:\s*#|\s+number[:\s]+)\s*([0-9]{3}-[0-9]{7}-[0-9]{7})`)

func extractThankYouOrderID(body string) string {
	m := thankYouOrderRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// html2text is a tiny HTML entity decoder for the fields we extract. Good
// enough for &amp; &#39; &quot; &lt; &gt;.
func html2text(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return s
}
