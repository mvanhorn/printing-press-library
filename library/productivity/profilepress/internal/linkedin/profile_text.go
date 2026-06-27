package linkedin

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/browser"
	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/profile"
)

var profileSectionMarkers = []string{
	"Featured",
	"Activity",
	"About",
	"Experience",
	"Education",
	"Licenses & certifications",
	"Skills",
	"Recommendations",
	"Interests",
}

// ParseProfilePageText validates and converts visible LinkedIn page text into
// auditable profile sections. It preserves raw_text even when section extraction
// is necessarily best-effort against LinkedIn's changing UI text.
func ParseProfilePageText(page browser.PageText, expectedName string) ([]profile.Section, string, error) {
	if err := validateLinkedInPage(page, expectedName); err != nil {
		return nil, "", err
	}
	lines := normalizedLines(page.Text)
	sections := []profile.Section{{Name: "raw_text", RawText: strings.Join(lines, "\n"), Normalized: Normalize(strings.Join(lines, " ")), Source: "browser-cdp"}}
	if expectedName != "" {
		if headline := extractHeadline(lines, expectedName); headline != "" {
			sections = append(sections, profile.Section{Name: "headline", RawText: headline, Normalized: Normalize(headline), Source: "browser-cdp"})
		}
	}
	byMarker := extractMarkerSections(lines)
	foundProfileSection := false
	for _, name := range profileSectionMarkers {
		if text := byMarker[name]; text != "" {
			foundProfileSection = true
			sections = append(sections, profile.Section{Name: sectionKey(name), RawText: text, Normalized: Normalize(text), Source: "browser-cdp"})
		}
	}
	if !foundProfileSection {
		return nil, "", fmt.Errorf("captured LinkedIn page for %q but did not find recognizable profile sections", expectedName)
	}
	source := fmt.Sprintf("browser-cdp:%s (%s)", page.URL, page.Title)
	return sections, source, nil
}

func validateLinkedInPage(page browser.PageText, expectedName string) error {
	if strings.TrimSpace(page.Text) == "" {
		return fmt.Errorf("browser page text is empty; ensure the LinkedIn profile is fully loaded")
	}
	u, err := url.Parse(page.URL)
	if err != nil || !strings.Contains(strings.ToLower(u.Host), "linkedin.com") {
		return fmt.Errorf("browser page is not LinkedIn: url=%q title=%q", page.URL, page.Title)
	}
	cleanPath := strings.TrimRight(strings.ToLower(u.EscapedPath()), "/")
	if !strings.HasPrefix(cleanPath, "/in/") || len(cleanPath) <= len("/in/") {
		return fmt.Errorf("browser page does not look like a LinkedIn profile: url=%q title=%q", page.URL, page.Title)
	}
	if expectedName != "" && !strings.Contains(strings.ToLower(page.Text), strings.ToLower(expectedName)) && !strings.Contains(strings.ToLower(page.Title), strings.ToLower(expectedName)) {
		return fmt.Errorf("captured LinkedIn page does not contain expected profile name %q; refusing to snapshot possible stale/wrong tab", expectedName)
	}
	return nil
}

func normalizedLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var lines []string
	prevBlank := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			if !prevBlank {
				lines = append(lines, "")
			}
			prevBlank = true
			continue
		}
		lines = append(lines, line)
		prevBlank = false
	}
	return trimBlankEdges(lines)
}

func extractHeadline(lines []string, expectedName string) string {
	// Prefer an exact profile-name line. Avoid matching the page title
	// "Name | LinkedIn", which appears before the real profile header.
	for i, line := range lines {
		if strings.EqualFold(line, expectedName) {
			if headline := firstHeadlineAfter(lines, i); headline != "" {
				return headline
			}
		}
	}
	lowerName := strings.ToLower(expectedName)
	for i, line := range lines {
		low := strings.ToLower(line)
		if !strings.Contains(low, "linkedin") && strings.Contains(low, lowerName) {
			if headline := firstHeadlineAfter(lines, i); headline != "" {
				return headline
			}
		}
	}
	return ""
}

func firstHeadlineAfter(lines []string, i int) string {
	for j := i + 1; j < len(lines) && j < i+10; j++ {
		candidate := strings.TrimSpace(lines[j])
		low := strings.ToLower(candidate)
		if candidate == "" || isChromeLinkedInChrome(candidate) || strings.Contains(low, "contact info") || strings.Contains(low, "notifications") {
			continue
		}
		if isMarker(candidate) || strings.Contains(low, "followers") || strings.Contains(low, "connections") || strings.EqualFold(candidate, "Resources") || strings.EqualFold(candidate, "Add section") || strings.EqualFold(candidate, "Open to") {
			break
		}
		return candidate
	}
	return ""
}

func extractMarkerSections(lines []string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(lines); i++ {
		marker, ok := canonicalMarker(lines[i])
		if !ok {
			continue
		}
		var body []string
		for j := i + 1; j < len(lines); j++ {
			if canonical, ok := canonicalMarker(lines[j]); ok && canonical != marker {
				break
			}
			if strings.EqualFold(lines[j], "…see more") || strings.EqualFold(lines[j], "show all") {
				continue
			}
			body = append(body, lines[j])
		}
		body = trimBlankEdges(body)
		if len(body) > 0 {
			text := strings.Join(body, "\n")
			if isFooterSection(marker, text) {
				continue
			}
			out[marker] = text
		}
	}
	return out
}

func isFooterSection(marker, text string) bool {
	low := strings.ToLower(text)
	if strings.EqualFold(marker, "About") && strings.Contains(low, "talent solutions") && strings.Contains(low, "linkedin corporation") {
		return true
	}
	return false
}

func canonicalMarker(line string) (string, bool) {
	clean := strings.TrimSpace(strings.TrimSuffix(line, ":"))
	for _, marker := range profileSectionMarkers {
		if strings.EqualFold(clean, marker) {
			return marker, true
		}
	}
	return "", false
}

func isMarker(line string) bool {
	_, ok := canonicalMarker(line)
	return ok
}

func sectionKey(marker string) string {
	key := strings.ToLower(marker)
	key = strings.ReplaceAll(key, " & ", "_")
	key = strings.ReplaceAll(key, " ", "_")
	key = strings.ReplaceAll(key, "-", "_")
	return key
}

func trimBlankEdges(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func isChromeLinkedInChrome(s string) bool {
	low := strings.ToLower(s)
	return low == "home" || low == "my network" || low == "jobs" || low == "messaging" || low == "notifications" || low == "me" || low == "for business" || low == "learning"
}
