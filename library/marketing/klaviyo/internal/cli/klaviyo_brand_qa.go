// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const brandColorDistanceThreshold = 12.0
const brandColorOccurrenceLimit = 3

type brandQAOptions struct {
	Enabled        bool
	StrictNeutrals bool
}

func addBrandQAFlags(cmd *cobra.Command, options *brandQAOptions) {
	cmd.Flags().BoolVar(&options.Enabled, "brand", false, "Check colors, logos, and footer against the account brand kit")
	cmd.Flags().BoolVar(&options.StrictNeutrals, "brand-strict-neutrals", false, "Require neutral grays to match the brand palette (with --brand)")
}

type brandQAKit struct {
	Palette              []string `json:"palette"`
	LogoURLs             []string `json:"logo_urls"`
	RGBDistanceThreshold float64  `json:"rgb_distance_threshold"`
	StrictNeutrals       bool     `json:"strict_neutrals"`
	oldLogoURLs          map[string]bool
	organizationWord     string
	hasColors            bool
}

// Brand defaults may be a singleton or a collection; all list endpoints follow links.next.
func brandQAResources(c templateReader, endpoint string, params map[string]string) ([]map[string]any, []map[string]any, error) {
	base, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, err
	}
	var records, included []map[string]any
	seen := map[string]bool{}
	for {
		key := reportPathWithParams(endpoint, params)
		if seen[key] {
			return nil, nil, fmt.Errorf("repeated brand QA page: %s", endpoint)
		}
		seen[key] = true
		raw, err := c.Get(klaviyoRequestContext(c), endpoint, params)
		if err != nil {
			return nil, nil, classifyAPIErrorOnly(err)
		}
		var page struct {
			Data     json.RawMessage  `json:"data"`
			Included []map[string]any `json:"included"`
			Links    struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", endpoint, err)
		}
		data := strings.TrimSpace(string(page.Data))
		var items []map[string]any
		switch {
		case strings.HasPrefix(data, "["):
			err = json.Unmarshal(page.Data, &items)
		case strings.HasPrefix(data, "{"):
			var item map[string]any
			err = json.Unmarshal(page.Data, &item)
			items = append(items, item)
		case data == "null":
		default:
			err = fmt.Errorf("brand QA resource data is missing or invalid")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", endpoint, err)
		}
		records, included = append(records, items...), append(included, page.Included...)
		if page.Links.Next == "" {
			return records, included, nil
		}
		next, err := url.Parse(page.Links.Next)
		if err != nil || strings.TrimRight(next.Path, "/") != strings.TrimRight(base.Path, "/") || next.Fragment != "" {
			return nil, nil, fmt.Errorf("invalid next brand QA page: %q", page.Links.Next)
		}
		endpoint = next.RequestURI()
	}
}

func fetchBrandQAKit(c templateReader, options brandQAOptions) (*brandQAKit, error) {
	kit := &brandQAKit{Palette: []string{}, LogoURLs: []string{}, RGBDistanceThreshold: brandColorDistanceThreshold, StrictNeutrals: options.StrictNeutrals, oldLogoURLs: map[string]bool{}}
	palette := map[string]bool{}
	for _, endpoint := range []string{"/api/brand-colors", "/api/brand-email-defaults", "/api/brand-buttons"} {
		records, _, err := brandQAResources(c, endpoint, nil)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			collectBrandQAColors(record["attributes"], palette)
		}
	}
	kit.hasColors = len(palette) > 0
	palette["#ffffff"], palette["#000000"] = true, true
	for color := range palette {
		kit.Palette = append(kit.Palette, color)
	}
	sort.Strings(kit.Palette)
	logos, included, err := brandQAResources(c, "/api/brand-logos", map[string]string{"include": "image"})
	if err != nil {
		return nil, err
	}
	accounts, _, err := brandQAResources(c, "/api/accounts", nil)
	if err != nil {
		return nil, err
	}
	if len(accounts) > 0 {
		words := strings.Fields(strings.ToLower(stringFromMapPath(accounts[0], "attributes.contact_information.organization_name")))
		if len(words) > 0 {
			kit.organizationWord = words[0]
		}
	}
	images, _, err := brandQAResources(c, "/api/images", map[string]string{"filter": `contains(name,"logo")`, "page[size]": "100"})
	if err != nil {
		return nil, err
	}
	imageURLs := map[string]string{}
	for _, image := range append(append([]map[string]any{}, images...), included...) {
		imageURLs[stringFromMapPath(image, "id")] = stringFromMapPath(image, "attributes.image_url")
	}
	for _, logo := range logos {
		if imageURL := imageURLs[relationshipID(logo, "image")]; imageURL != "" {
			kit.LogoURLs = append(kit.LogoURLs, imageURL)
		}
	}
	kit.LogoURLs = uniqueStrings(kit.LogoURLs)
	sort.Strings(kit.LogoURLs)
	for _, image := range images {
		imageURL := stringFromMapPath(image, "attributes.image_url")
		if imageURL != "" && kit.logoNameMatches(stringFromMapPath(image, "attributes.name")) && !kit.currentLogo(imageURL) {
			kit.oldLogoURLs[imageURL] = true
		}
	}
	return kit, nil
}

func collectBrandQAColors(value any, palette map[string]bool) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "hex" || key == "color" || strings.HasSuffix(key, "_color") {
				if text, ok := child.(string); ok {
					if color, ok := parseBrandQAColor(text); ok {
						palette[color.hex()] = true
					}
				}
			}
			collectBrandQAColors(child, palette)
		}
	case []any:
		for _, child := range value {
			collectBrandQAColors(child, palette)
		}
	}
}

type brandQAColor [3]int

func (c brandQAColor) hex() string { return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]) }

// Compare source RGB channels without compositing alpha against an assumed background.
func parseBrandQAColor(value string) (brandQAColor, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if named, ok := brandQANamedColors[value]; ok {
		value = named
	}
	if strings.HasPrefix(value, "#") {
		hex := value[1:]
		if len(hex) == 3 {
			hex = strings.Repeat(hex[:1], 2) + strings.Repeat(hex[1:2], 2) + strings.Repeat(hex[2:3], 2)
		}
		if len(hex) != 6 && len(hex) != 8 {
			return brandQAColor{}, false
		}
		n, err := strconv.ParseUint(hex, 16, 32)
		if len(hex) == 8 {
			n >>= 8
		}
		return brandQAColor{int(n >> 16), int(n >> 8 & 255), int(n & 255)}, err == nil
	}
	if !strings.HasSuffix(value, ")") || (!strings.HasPrefix(value, "rgb(") && !strings.HasPrefix(value, "rgba(")) {
		return brandQAColor{}, false
	}
	components := strings.Fields(strings.NewReplacer(",", " ", "/", " ").Replace(value[strings.IndexByte(value, '(')+1 : len(value)-1]))
	if len(components) != 3 && len(components) != 4 {
		return brandQAColor{}, false
	}
	var color brandQAColor
	for i, component := range components {
		n, err := strconv.ParseFloat(strings.TrimSuffix(component, "%"), 64)
		max := 255.0
		if i == 3 {
			max = 1
		}
		if strings.HasSuffix(component, "%") {
			n *= max / 100
		}
		if err != nil || math.IsNaN(n) || n < 0 || n > max {
			return brandQAColor{}, false
		}
		if i < 3 {
			color[i] = int(math.Round(n))
		}
	}
	return color, true
}

var brandQANamedColors = map[string]string{
	"black": "#000000", "silver": "#c0c0c0", "gray": "#808080", "grey": "#808080", "white": "#ffffff",
	"maroon": "#800000", "red": "#ff0000", "purple": "#800080", "fuchsia": "#ff00ff", "green": "#008000",
	"lime": "#00ff00", "olive": "#808000", "yellow": "#ffff00", "navy": "#000080", "blue": "#0000ff",
	"teal": "#008080", "aqua": "#00ffff", "orange": "#ffa500", "rebeccapurple": "#663399",
}

var (
	brandQATagPattern       = regexp.MustCompile(`(?is)<([a-z][\w:-]*)\b(?:[^"'<>]|"[^"]*"|'[^']*')*>`)
	brandQAAttributePattern = regexp.MustCompile(`(?is)\s+([a-z_:][\w:.-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>]+))`)
	brandQAStylePattern     = regexp.MustCompile(`(?is)<style\b[^>]*>(.*?)</style\s*>`)
	brandQADeclaration      = regexp.MustCompile(`(?i)(?:^|[;{])\s*[-\w]+\s*:\s*([^;{}]+)`)
	brandQAColorToken       = regexp.MustCompile(`(?i)#[0-9a-f]+\b|rgba?\([^)]*\)|\b[a-z]+\b`)
	brandQACSSNonColors     = regexp.MustCompile(`(?is)/\*.*?\*/|url\([^)]*\)|"[^"]*"|'[^']*'`)
	brandQACommentPattern   = regexp.MustCompile(`(?s)<!--.*?-->`)
	brandQAOrgTagPattern    = regexp.MustCompile(`(?i)\{\{\s*organization\.(full_address|name|street_address|street_address2|address|address1|address2|city|region|state|zip|postal_code|country)\s*(?:\|[^}]*)?\}\}`)
)

func brandQAAttributes(tag string) map[string]string {
	attrs := map[string]string{}
	for _, match := range brandQAAttributePattern.FindAllStringSubmatch(tag, -1) {
		attrs[strings.ToLower(match[1])] = html.UnescapeString(match[2] + match[3] + match[4])
	}
	return attrs
}

func extractBrandQAColors(htmlBody string) map[string][]string {
	htmlBody = brandQACommentPattern.ReplaceAllStringFunc(htmlBody, func(comment string) string {
		return strings.Repeat("\n", strings.Count(comment, "\n"))
	})
	colors := map[string][]string{}
	add := func(text, location string) {
		text = brandQACSSNonColors.ReplaceAllString(text, " ")
		for _, token := range brandQAColorToken.FindAllString(text, -1) {
			if color, ok := parseBrandQAColor(token); ok && len(colors[color.hex()]) < brandColorOccurrenceLimit {
				colors[color.hex()] = append(colors[color.hex()], location)
			}
		}
	}
	css := func(text, location string) {
		text = brandQACSSNonColors.ReplaceAllString(text, " ")
		for _, declaration := range brandQADeclaration.FindAllStringSubmatch(text, -1) {
			add(declaration[1], location)
		}
	}
	for _, index := range brandQATagPattern.FindAllStringSubmatchIndex(htmlBody, -1) {
		tag, name := htmlBody[index[0]:index[1]], strings.ToLower(htmlBody[index[2]:index[3]])
		attrs := brandQAAttributes(tag)
		location := fmt.Sprintf("<%s> line %d", name, strings.Count(htmlBody[:index[0]], "\n")+1)
		for _, key := range []string{"bgcolor", "color"} {
			if value := attrs[key]; value != "" {
				add(value, location+" "+key+" attribute")
			}
		}
		css(attrs["style"], location+" inline style")
	}
	for _, index := range brandQAStylePattern.FindAllStringSubmatchIndex(htmlBody, -1) {
		css(htmlBody[index[2]:index[3]], fmt.Sprintf("<style> block line %d", strings.Count(htmlBody[:index[0]], "\n")+1))
	}
	return colors
}

func brandQAColorDistance(a, b brandQAColor) float64 {
	distance := 0
	for i := range a {
		distance += (a[i] - b[i]) * (a[i] - b[i])
	}
	return math.Sqrt(float64(distance))
}

func (kit *brandQAKit) nearestColor(color brandQAColor) (string, float64) {
	nearest, distance := "", math.Inf(1)
	for _, hex := range kit.Palette {
		allowed, ok := parseBrandQAColor(hex)
		if d := brandQAColorDistance(color, allowed); ok && d < distance {
			nearest, distance = hex, d
		}
	}
	return nearest, distance
}

func (kit *brandQAKit) currentLogo(src string) bool {
	for _, current := range kit.LogoURLs {
		if src == current {
			return true
		}
	}
	return false
}

func (kit *brandQAKit) logoNameMatches(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "logo") && (kit.organizationWord == "" || strings.Contains(name, kit.organizationWord))
}

func brandQAFooter(htmlBody string) (bool, string) {
	htmlBody = renderedHTMLSource(htmlBody)
	name, address := false, false
	for _, tag := range brandQAOrgTagPattern.FindAllStringSubmatch(htmlBody, -1) {
		switch strings.ToLower(tag[1]) {
		case "full_address":
			return true, "Found {{ organization.full_address }}."
		case "name":
			name = true
		default:
			address = true
		}
	}
	if name && address {
		return true, "Found {{ organization.name }} plus an organization address tag."
	}
	for _, tag := range brandQATagPattern.FindAllStringSubmatch(htmlBody, -1) {
		attrs := brandQAAttributes(tag[0])
		if strings.EqualFold(tag[1], "footer") || strings.EqualFold(attrs["role"], "contentinfo") || strings.Contains(strings.ToLower(attrs["id"]+" "+attrs["class"]+" "+attrs["data-testid"]+" "+attrs["data-block-type"]), "footer") {
			return true, "Found a footer block: <footer>, role=contentinfo, or an id/class/data-testid/data-block-type containing footer."
		}
	}
	return false, "No footer detected: expected {{ organization.full_address }}, or {{ organization.name }} plus an organization address tag (street_address, street_address2, address, address1, address2, city, region, state, zip, postal_code, country), or a <footer>/role=contentinfo block or id/class/data-testid/data-block-type containing footer; comments and non-rendered HTML do not count."
}

func (kit *brandQAKit) availabilityFindings() []map[string]any {
	if !kit.hasColors && len(kit.LogoURLs) == 0 {
		return []map[string]any{{"check": "brand_colors", "status": "warn", "detail": "Brand kit has no colors or logos; brand color and logo checks skipped. Footer detection still runs."}}
	}
	findings := []map[string]any{}
	if !kit.hasColors {
		findings = append(findings, map[string]any{"check": "brand_colors", "status": "warn", "detail": "Brand kit has no colors; brand color check skipped."})
	}
	if len(kit.LogoURLs) == 0 {
		findings = append(findings, map[string]any{"check": "brand_logo", "status": "warn", "detail": "Brand kit has no current logo URLs; brand logo check skipped."})
	}
	return findings
}

func (kit *brandQAKit) checkHTML(htmlBody string) []map[string]any {
	findings := []map[string]any{}
	if kit.hasColors {
		colors := extractBrandQAColors(htmlBody)
		keys := make([]string, 0, len(colors))
		for hex := range colors {
			keys = append(keys, hex)
		}
		sort.Strings(keys)
		for _, hex := range keys {
			color, _ := parseBrandQAColor(hex)
			nearest, distance := kit.nearestColor(color)
			if distance <= brandColorDistanceThreshold || (!kit.StrictNeutrals && color[0] == color[1] && color[1] == color[2]) {
				continue
			}
			findings = append(findings, map[string]any{"check": "brand_colors", "status": "warn", "color": hex, "nearest_brand_color": nearest, "rgb_distance": distance, "occurrences": colors[hex], "detail": fmt.Sprintf("Off-brand color %s; nearest allowed brand color %s (RGB distance %.1f exceeds %.0f), at %s.", hex, nearest, distance, brandColorDistanceThreshold, strings.Join(colors[hex], "; "))})
		}
	}
	if len(kit.LogoURLs) > 0 {
		seen := map[string]bool{}
		for _, tag := range brandQATagPattern.FindAllStringSubmatch(renderedHTMLSource(htmlBody), -1) {
			if !strings.EqualFold(tag[1], "img") {
				continue
			}
			attrs := brandQAAttributes(tag[0])
			src := attrs["src"]
			imageURL, _ := url.Parse(src)
			filename := ""
			if imageURL != nil {
				filename, _ = url.PathUnescape(path.Base(imageURL.Path))
			}
			if src == "" || seen[src] || kit.currentLogo(src) || (!kit.oldLogoURLs[src] && !kit.logoNameMatches(attrs["alt"]) && !kit.logoNameMatches(filename)) {
				continue
			}
			seen[src] = true
			findings = append(findings, map[string]any{"check": "brand_logo", "status": "warn", "url": src, "current_logo_urls": kit.LogoURLs, "detail": fmt.Sprintf("Old logo URL %s; current brand logo URL(s): %s.", src, strings.Join(kit.LogoURLs, ", "))})
		}
	}
	status := "pass"
	present, detail := brandQAFooter(htmlBody)
	if !present {
		status = "fail"
	}
	return append(findings, map[string]any{"check": "brand_footer", "status": status, "detail": detail})
}
