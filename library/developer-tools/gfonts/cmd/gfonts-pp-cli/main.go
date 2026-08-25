package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var version = "1.0.0"

const metadataURL = "https://fonts.google.com/metadata/fonts"

// --- Data types (matches actual Google Fonts metadata endpoint) ---

type FontMetadata struct {
	AxisRegistry       []interface{} `json:"axisRegistry"`
	FamilyMetadataList []Font        `json:"familyMetadataList"`
}

type Font struct {
	Family            string                     `json:"family"`
	DisplayName       *string                    `json:"displayName"`
	Category          string                     `json:"category"`
	Subsets           []string                   `json:"subsets"`
	Fonts             map[string]FontVariantMeta `json:"fonts"`
	Designers         []string                   `json:"designers"`
	LastModified      string                     `json:"lastModified"`
	DateAdded         string                     `json:"dateAdded"`
	Popularity        int                        `json:"popularity"`
	Trending          int                        `json:"trending"`
	ColorCapabilities []string                   `json:"colorCapabilities,omitempty"`
	PrimaryScript     string                     `json:"primaryScript,omitempty"`
	IsNoto            bool                       `json:"isNoto"`
	IsOpenSource      bool                       `json:"isOpenSource"`
	IsBrandFont       bool                       `json:"isBrandFont"`
}

type FontVariantMeta struct {
	Thickness  int     `json:"thickness"`
	Slant      int     `json:"slant"`
	Width      int     `json:"width"`
	LineHeight float64 `json:"lineHeight"`
}

type cssEntry struct {
	Variant string
	URL     string
}

// --- Cache ---

var cacheFile = filepath.Join(os.TempDir(), "gfonts-metadata-cache.json")
var cacheTTL = 24 * time.Hour

func loadMetadata() (*FontMetadata, error) {
	if info, err := os.Stat(cacheFile); err == nil {
		if time.Since(info.ModTime()) < cacheTTL {
			data, err := os.ReadFile(cacheFile)
			if err == nil {
				var meta FontMetadata
				if json.Unmarshal(data, &meta) == nil && len(meta.FamilyMetadataList) > 0 {
					return &meta, nil
				}
			}
		}
	}

	fmt.Fprintf(os.Stderr, "Fetching font metadata from Google Fonts...\n")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(metadataURL)
	if err != nil {
		return nil, fmt.Errorf("fetch metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	raw := strings.TrimSpace(string(body))
	if strings.HasPrefix(raw, ")]}'") {
		if idx := strings.Index(raw, "\n"); idx >= 0 {
			raw = raw[idx+1:]
		}
	}

	var meta FontMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}

	os.WriteFile(cacheFile, []byte(raw), 0644)
	return &meta, nil
}

func (f *Font) displayName() string {
	if f.DisplayName != nil && *f.DisplayName != "" {
		return *f.DisplayName
	}
	return f.Family
}

func (f *Font) variantList() []string {
	variants := make([]string, 0, len(f.Fonts))
	for v := range f.Fonts {
		variants = append(variants, v)
	}
	sortVariants(variants)
	return variants
}

func sortVariants(vars []string) {
	order := map[string]int{
		"100": 1, "200": 2, "300": 3, "regular": 4, "500": 5, "600": 6,
		"700": 7, "800": 8, "900": 9,
		"100italic": 10, "200italic": 11, "300italic": 12, "italic": 13,
		"500italic": 14, "600italic": 15, "700italic": 16, "800italic": 17, "900italic": 18,
	}
	sort.Slice(vars, func(i, j int) bool {
		oi, okI := order[vars[i]]
		oj, okJ := order[vars[j]]
		if okI && okJ {
			return oi < oj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return vars[i] < vars[j]
	})
}

func findFont(meta *FontMetadata, query string) *Font {
	q := strings.ToLower(strings.TrimSpace(query))
	for i := range meta.FamilyMetadataList {
		if strings.ToLower(meta.FamilyMetadataList[i].Family) == q {
			return &meta.FamilyMetadataList[i]
		}
	}
	for i := range meta.FamilyMetadataList {
		if strings.HasPrefix(strings.ToLower(meta.FamilyMetadataList[i].Family), q) {
			return &meta.FamilyMetadataList[i]
		}
	}
	for i := range meta.FamilyMetadataList {
		if strings.Contains(strings.ToLower(meta.FamilyMetadataList[i].Family), q) {
			return &meta.FamilyMetadataList[i]
		}
	}
	return nil
}

func searchFonts(meta *FontMetadata, query string) []Font {
	q := strings.ToLower(strings.TrimSpace(query))
	var results []Font
	for _, f := range meta.FamilyMetadataList {
		name := strings.ToLower(f.Family)
		cat := strings.ToLower(f.Category)
		designer := ""
		if len(f.Designers) > 0 {
			designer = strings.ToLower(strings.Join(f.Designers, " "))
		}
		if strings.Contains(name, q) || strings.Contains(cat, q) || strings.Contains(designer, q) {
			results = append(results, f)
		}
	}
	return results
}

// categoryKey lowercases a category and collapses '-', '_', and spaces so
// help slugs like "sans-serif" match stored names like "Sans Serif".
func categoryKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '-' || r == '_' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func categoriesMatch(stored, query string) bool {
	if stored == query {
		return true
	}
	storedKey := categoryKey(stored)
	queryKey := categoryKey(query)
	if queryKey == "" {
		return false
	}
	return storedKey == queryKey
}

func filterByCategory(meta *FontMetadata, category string) []Font {
	if categoryKey(category) == "" {
		return nil
	}
	var results []Font
	for _, f := range meta.FamilyMetadataList {
		if categoriesMatch(f.Category, category) {
			results = append(results, f)
		}
	}
	return results
}

func joinItalicWeights(weights []string) string {
	var parts []string
	for _, w := range weights {
		parts = append(parts, "1,"+w)
	}
	return strings.Join(parts, ";")
}

func fetchFontCSS(family string, variants []string) ([]cssEntry, error) {
	var normalWeights, italicWeights []string
	for _, v := range variants {
		if v == "regular" {
			normalWeights = append(normalWeights, "400")
		} else if v == "italic" {
			italicWeights = append(italicWeights, "400")
		} else if strings.HasSuffix(v, "italic") {
			w := strings.TrimSuffix(v, "italic")
			italicWeights = append(italicWeights, w)
		} else if strings.HasSuffix(v, "i") && len(v) > 1 {
			w := strings.TrimSuffix(v, "i")
			italicWeights = append(italicWeights, w)
		} else {
			normalWeights = append(normalWeights, v)
		}
	}

	escaped := strings.ReplaceAll(family, " ", "+")
	var parts []string
	if len(normalWeights) > 0 {
		parts = append(parts, fmt.Sprintf("family=%s:wght@%s", escaped, strings.Join(normalWeights, ";")))
	}
	if len(italicWeights) > 0 {
		parts = append(parts, fmt.Sprintf("family=%s:ital,wght@%s", escaped, joinItalicWeights(italicWeights)))
	}

	if len(parts) == 0 {
		parts = append(parts, "family="+escaped)
	}

	cssURL := "https://fonts.googleapis.com/css2?" + strings.Join(parts, "&")
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", cssURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("CSS fetch failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading CSS response: %w", err)
	}
	css := string(body)

	re := regexp.MustCompile(`@font-face\s*\{[^}]+\}`)
	urlRe := regexp.MustCompile(`src:\s*url\(([^)]+)\)`)
	weightRe := regexp.MustCompile(`font-weight:\s*(\d+)`)
	styleRe := regexp.MustCompile(`font-style:\s*(\w+)`)

	var entries []cssEntry
	for _, block := range re.FindAllString(css, -1) {
		urlMatch := urlRe.FindStringSubmatch(block)
		weightMatch := weightRe.FindStringSubmatch(block)
		styleMatch := styleRe.FindStringSubmatch(block)

		if urlMatch == nil {
			continue
		}

		fontURL := urlMatch[1]
		weight := "400"
		if weightMatch != nil {
			weight = weightMatch[1]
		}
		style := "normal"
		if styleMatch != nil {
			style = styleMatch[1]
		}

		variant := weight
		if style == "italic" {
			if weight == "400" {
				variant = "italic"
			} else {
				variant = weight + "italic"
			}
		} else if weight == "400" {
			variant = "regular"
		}

		entries = append(entries, cssEntry{Variant: variant, URL: fontURL})
	}
	return entries, nil
}

// --- Commands ---

func cmdSearch(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gfonts search <query>")
		os.Exit(1)
	}
	query := strings.Join(args, " ")

	meta, err := loadMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	results := searchFonts(meta, query)
	if len(results) == 0 {
		fmt.Printf("No fonts found matching %q\n", query)
		return
	}

	fmt.Printf("Found %d font(s) matching %q:\n\n", len(results), query)
	limit := 25
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		f := results[i]
		designers := ""
		if len(f.Designers) > 0 {
			designers = " — " + strings.Join(f.Designers, ", ")
		}
		fmt.Printf("  %-40s [%s]  %d variants%s\n", f.Family, f.Category, len(f.Fonts), designers)
	}
	if len(results) > limit {
		fmt.Printf("\n  ... and %d more. Refine your search.\n", len(results)-limit)
	}
}

func cmdList(args []string) {
	var category, sortField string
	limit := 20

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--category", "-c":
			if i+1 < len(args) {
				category = args[i+1]
				i++
			}
		case "--sort", "-s":
			if i+1 < len(args) {
				sortField = args[i+1]
				i++
			}
		case "--limit", "-n":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &limit)
				i++
			}
		}
	}

	meta, err := loadMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fonts := meta.FamilyMetadataList
	if category != "" {
		fonts = filterByCategory(meta, category)
	}

	switch sortField {
	case "alpha", "name":
		sort.Slice(fonts, func(i, j int) bool { return fonts[i].Family < fonts[j].Family })
	case "date", "modified":
		sort.Slice(fonts, func(i, j int) bool { return fonts[i].LastModified > fonts[j].LastModified })
	case "trending":
		sort.Slice(fonts, func(i, j int) bool { return fonts[i].Trending < fonts[j].Trending })
	default:
		sort.Slice(fonts, func(i, j int) bool { return fonts[i].Popularity < fonts[j].Popularity })
	}

	if limit > len(fonts) {
		limit = len(fonts)
	}

	catLabel := ""
	if category != "" {
		catLabel = fmt.Sprintf(" [%s]", category)
	}
	fmt.Printf("Google Fonts%s (%d showing of %d):\n\n", catLabel, limit, len(fonts))

	for i := 0; i < limit; i++ {
		f := fonts[i]
		designers := ""
		if len(f.Designers) > 0 {
			designers = " — " + strings.Join(f.Designers, ", ")
		}
		fmt.Printf("  %3d. %-40s %s  %d variants%s\n", i+1, f.Family, f.Category, len(f.Fonts), designers)
	}
}

func cmdInfo(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gfonts info <font-family>")
		os.Exit(1)
	}
	query := strings.Join(args, " ")

	meta, err := loadMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	font := findFont(meta, query)
	if font == nil {
		fmt.Fprintf(os.Stderr, "Font %q not found.\n", query)
		os.Exit(1)
	}

	variants := font.variantList()

	fmt.Printf("Font: %s\n", font.Family)
	fmt.Printf("Category: %s\n", font.Category)
	if len(font.Designers) > 0 {
		fmt.Printf("Designer(s): %s\n", strings.Join(font.Designers, ", "))
	}
	fmt.Printf("Popularity Rank: #%d\n", font.Popularity)
	fmt.Printf("Trending Rank: #%d\n", font.Trending)
	fmt.Printf("Last Modified: %s\n", font.LastModified)
	fmt.Printf("Date Added: %s\n", font.DateAdded)
	fmt.Printf("Open Source: %v\n", font.IsOpenSource)
	fmt.Printf("Variants (%d): %s\n", len(variants), strings.Join(variants, ", "))
	fmt.Printf("Subsets (%d): %s\n", len(font.Subsets), strings.Join(font.Subsets, ", "))
	if font.PrimaryScript != "" {
		fmt.Printf("Primary Script: %s\n", font.PrimaryScript)
	}
	if len(font.ColorCapabilities) > 0 {
		fmt.Printf("Color Capabilities: %v\n", font.ColorCapabilities)
	}

	fmt.Printf("\nFetch download URLs: gfonts download %q\n", font.Family)
}

func cmdDownload(args []string) {
	var outDir, variant string
	var showOnly bool
	var fontArgs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output", "-o":
			if i+1 < len(args) {
				outDir = args[i+1]
				i++
			}
		case "--variant", "-v":
			if i+1 < len(args) {
				variant = args[i+1]
				i++
			}
		case "--show", "-s":
			showOnly = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				fontArgs = append(fontArgs, args[i])
			}
		}
	}
	if len(fontArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gfonts download <font-family> [--variant regular] [--output ./fonts] [--show]")
		os.Exit(1)
	}
	query := strings.Join(fontArgs, " ")

	meta, err := loadMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	font := findFont(meta, query)
	if font == nil {
		fmt.Fprintf(os.Stderr, "Font %q not found.\n", query)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Fetching CSS for %s...\n", font.Family)
	entries, err := fetchFontCSS(font.Family, font.variantList())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching CSS: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "No font files found.\n")
		os.Exit(1)
	}

	if variant != "" {
		var filtered []cssEntry
		for _, e := range entries {
			if e.Variant == variant {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			avail := make([]string, len(entries))
			for i, e := range entries {
				avail[i] = e.Variant
			}
			fmt.Fprintf(os.Stderr, "Variant %q not found. Available: %s\n", variant, strings.Join(avail, ", "))
			os.Exit(1)
		}
		entries = filtered
	}

	if showOnly {
		fmt.Printf("Font: %s\n\nAvailable files:\n", font.Family)
		for _, e := range entries {
			fmt.Printf("  %-12s %s\n", e.Variant, e.URL)
		}
		return
	}

	if outDir == "" {
		outDir = strings.ReplaceAll(font.Family, " ", "-")
	}
	os.MkdirAll(outDir, 0755)

	client := &http.Client{Timeout: 60 * time.Second}
	downloaded := 0
	for _, e := range entries {
		ext := ".ttf"
		if strings.Contains(e.URL, ".woff2") {
			ext = ".woff2"
		} else if strings.Contains(e.URL, ".woff") {
			ext = ".woff"
		}
		filename := strings.ReplaceAll(font.Family, " ", "-") + "-" + e.Variant + ext
		outPath := filepath.Join(outDir, filename)

		fmt.Fprintf(os.Stderr, "Downloading %s...\n", filename)
		resp, err := client.Get(e.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			fmt.Fprintf(os.Stderr, "  Error: HTTP %d\n", resp.StatusCode)
			continue
		}

		os.WriteFile(outPath, body, 0644)
		downloaded++
		fmt.Printf("  ✓ %s\n", outPath)
	}

	fmt.Printf("\nDownloaded %d file(s) to %s/\n", downloaded, outDir)
}

func cmdTrending(args []string) {
	meta, err := loadMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fonts := make([]Font, len(meta.FamilyMetadataList))
	copy(fonts, meta.FamilyMetadataList)
	sort.Slice(fonts, func(i, j int) bool { return fonts[i].Trending < fonts[j].Trending })

	limit := 15
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil {
			if n < 1 {
				n = 1
			}
			limit = n
		}
	}
	if limit > len(fonts) {
		limit = len(fonts)
	}

	fmt.Printf("Top %d trending Google Fonts:\n\n", limit)
	for i := 0; i < limit; i++ {
		f := fonts[i]
		designers := ""
		if len(f.Designers) > 0 {
			designers = " — " + strings.Join(f.Designers, ", ")
		}
		fmt.Printf("  %3d. %-40s [%s]  %d variants%s\n", i+1, f.Family, f.Category, len(f.Fonts), designers)
	}
}

func cmdCategories(args []string) {
	meta, err := loadMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	counts := map[string]int{}
	for _, f := range meta.FamilyMetadataList {
		counts[f.Category]++
	}

	type catCount struct {
		name  string
		count int
	}
	var cats []catCount
	for name, count := range counts {
		cats = append(cats, catCount{name, count})
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].count > cats[j].count })

	fmt.Printf("Font Categories (%d total fonts):\n\n", len(meta.FamilyMetadataList))
	for _, cc := range cats {
		fmt.Printf("  %-20s %d fonts\n", cc.name, cc.count)
	}
}

func cmdRandom(args []string) {
	category := ""
	for i := 0; i < len(args); i++ {
		if (args[i] == "--category" || args[i] == "-c") && i+1 < len(args) {
			category = args[i+1]
			i++
		}
	}

	meta, err := loadMetadata()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fonts := meta.FamilyMetadataList
	if category != "" {
		fonts = filterByCategory(meta, category)
		if len(fonts) == 0 {
			fmt.Printf("No fonts in category %q\n", category)
			return
		}
	}

	if len(fonts) == 0 {
		fmt.Println("No fonts available")
		return
	}

	idx := int(time.Now().UnixNano()) % len(fonts)
	if idx < 0 {
		idx = -idx
	}
	f := fonts[idx]

	designers := ""
	if len(f.Designers) > 0 {
		designers = " by " + strings.Join(f.Designers, ", ")
	}
	fmt.Printf("🎲 %s [%s]%s — %d variants (popularity #%d)\n", f.Family, f.Category, designers, len(f.Fonts), f.Popularity)
	fmt.Printf("   Try: gfonts download %q\n", f.Family)
}

// --- Agent-context (machine-readable CLI description for agents) ---
// The live dogfood runner enumerates this command tree, so the command list
// here must stay in sync with the switch dispatch in main().

type agentContextCommand struct {
	Name        string                `json:"name"`
	Use         string                `json:"use,omitempty"`
	Short       string                `json:"short,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	Flags       []agentContextFlag    `json:"flags,omitempty"`
	Subcommands []agentContextCommand `json:"subcommands,omitempty"`
}

type agentContextFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Usage   string `json:"usage,omitempty"`
	Default string `json:"default,omitempty"`
}

func agentContextPaths() map[string]string {
	home, _ := os.UserHomeDir()
	return map[string]string{
		"config_dir": filepath.Join(home, ".config", "gfonts"),
		"data_dir":   filepath.Join(home, ".local", "share", "gfonts"),
		"state_dir":  filepath.Join(home, ".local", "state", "gfonts"),
		"cache_dir":  os.TempDir(),
	}
}

func cmdAgentContext(args []string) {
	pretty := false
	for _, a := range args {
		if a == "--pretty" {
			pretty = true
		}
	}
	ctx := map[string]any{
		"schema_version": "4",
		"cli": map[string]string{
			"name":        "gfonts-pp-cli",
			"description": "Search, browse, and download fonts from Google Fonts. No API key required.",
			"version":     version,
		},
		"auth": map[string]any{
			"mode":     "none",
			"env_vars": []any{},
		},
		"paths": agentContextPaths(),
		"commands": []agentContextCommand{
			{Name: "search", Use: "search <query>", Short: "Search fonts by name, category, or designer",
				Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<query>=Inter"}},
			{Name: "list", Use: "list [--category <category>] [--sort <sort>] [--limit <limit>]", Short: "List fonts",
				Annotations: map[string]string{"mcp:read-only": "true"},
				Flags: []agentContextFlag{
					{Name: "category", Type: "string", Usage: "filter by category (serif, sans-serif, display, etc.)"},
					{Name: "sort", Type: "string", Usage: "sort by: popularity (default), alpha, date, trending", Default: "popularity"},
					{Name: "limit", Type: "int", Usage: "number of results (default 20)", Default: "20"},
				}},
			{Name: "info", Use: "info <font>", Short: "Show detailed font info",
				Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<font>=Inter"}},
			{Name: "download", Use: "download <font> [--variant <variant>] [--output <dir>] [--show]", Short: "Download font files",
				Annotations: map[string]string{"pp:happy-args": "<font>=Inter;--variant=regular;--output=/tmp/gfonts-dogfood-download"},
				Flags: []agentContextFlag{
					{Name: "variant", Type: "string", Usage: "download specific variant (e.g. regular, 700, italic)"},
					{Name: "output", Type: "string", Usage: "output directory (default: font name)"},
					{Name: "show", Type: "bool", Usage: "show URLs without downloading"},
				}},
			{Name: "trending", Use: "trending", Short: "Show popular/trending fonts",
				Annotations: map[string]string{"mcp:read-only": "true"}},
			{Name: "categories", Use: "categories", Short: "List all font categories",
				Annotations: map[string]string{"mcp:read-only": "true"}},
			{Name: "random", Use: "random [--category <category>]", Short: "Pick a random font",
				Annotations: map[string]string{"mcp:read-only": "true"},
				Flags: []agentContextFlag{
					{Name: "category", Type: "string", Usage: "only pick from this category"},
				}},
		},
		"available_profiles":           []string{},
		"feedback_endpoint_configured": false,
		"learn_protocol":               "",
	}
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	_ = enc.Encode(ctx)
}

// commandHelp returns the per-command help text. Each block keeps a
// "Usage:" line and an "Examples:" section (with example lines starting at
// the binary name) because the live dogfood runner parses those. Do not
// mention "--json" here: the runner treats its presence as JSON support and
// would probe the command for JSON output this CLI does not produce.

func commandHelp(cmd string) string {
	switch cmd {
	case "search":
		return `Usage:
  gfonts search <query>

Search fonts by name, category, or designer.

Options:
  (none)

Examples:
  gfonts search "Inter"
  gfonts search "Playfair Display"
  gfonts search "sans-serif"`
	case "list":
		return `Usage:
  gfonts list [--category <category>] [--sort <sort>] [--limit <limit>]

List fonts with optional category filter, sort order, and result limit.

Options:
  --category, -c    Filter by category (serif, sans-serif, display, etc.)
  --sort, -s        Sort by: popularity (default), alpha, date, trending
  --limit, -n       Number of results (default 20)

Examples:
  gfonts list --category sans-serif --sort trending --limit 10
  gfonts list --sort alpha --limit 5`
	case "info":
		return `Usage:
  gfonts info <font>

Show detailed metadata for a specific font family.

Options:
  (none)

Examples:
  gfonts info "Inter"
  gfonts info "Playfair Display"`
	case "download":
		return `Usage:
  gfonts download <font> [--variant <variant>] [--output <dir>] [--show]

Download font files to a local directory.

Options:
  --variant, -v    Download a specific variant (e.g. regular, 700, italic)
  --output, -o     Output directory (default: the font name)
  --show, -s       Show download URLs without downloading

Examples:
  gfonts download "Inter" --variant regular --output ./my-fonts
  gfonts download "Playfair Display" --show`
	case "trending":
		return `Usage:
  gfonts trending

Show the most trending Google Fonts right now.

Options:
  (none)

Examples:
  gfonts trending`
	case "categories":
		return `Usage:
  gfonts categories

List all font categories with font counts.

Options:
  (none)

Examples:
  gfonts categories`
	case "random":
		return `Usage:
  gfonts random [--category <category>]

Pick a random font, optionally restricted to a category.

Options:
  --category, -c    Only pick from this category (serif, sans-serif, display, etc.)

Examples:
  gfonts random --category display
  gfonts random`
	default:
		return ""
	}
}

func printCommandHelp(cmd string) {
	if text := commandHelp(cmd); text != "" {
		fmt.Println(text)
		return
	}
	printUsage()
}

func printUsage() {
	fmt.Printf(`gfonts — Google Fonts CLI (v%s)

Search, browse, and download fonts from Google Fonts. No API key required.

Commands:
  search <query>              Search fonts by name, category, or designer
  list                        List fonts (--category, --sort, --limit)
  info <font>                 Show detailed font info
  download <font>             Download font files (--variant, --output, --show)
  trending                    Show popular/trending fonts
  categories                  List all font categories
  random                      Pick a random font (--category)

Options (list):
  --category, -c    Filter by category (serif, sans-serif, display, etc.)
  --sort, -s        Sort by: popularity (default), alpha, date, trending
  --limit, -n       Number of results (default 20)

Options (download):
  --variant, -v     Download specific variant (e.g. regular, 700, italic)
  --output, -o      Output directory (default: font name)
  --show, -s        Show URLs without downloading

Examples:
  gfonts search "serif"
  gfonts list --category sans-serif --sort trending --limit 10
  gfonts info "Playfair Display"
  gfonts download "Inter" --variant regular --output ./my-fonts
  gfonts download "Inter" --show
  gfonts trending
  gfonts random --category display
`, version)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Handle --version/-v flags anywhere in args
	for _, a := range os.Args[1:] {
		if a == "--version" {
			fmt.Printf("gfonts %s\n", version)
			os.Exit(0)
		}
	}

	// Per-command help: gfonts <cmd> --help prints that command's help.
	if cmd != "help" && cmd != "--help" && cmd != "-h" {
		for _, a := range args {
			if a == "--help" || a == "-h" {
				printCommandHelp(cmd)
				os.Exit(0)
			}
		}
	}

	switch cmd {
	case "search":
		cmdSearch(args)
	case "list", "ls":
		cmdList(args)
	case "info", "show":
		cmdInfo(args)
	case "download", "get", "dl":
		cmdDownload(args)
	case "trending", "popular":
		cmdTrending(args)
	case "categories", "cats":
		cmdCategories(args)
	case "random", "rand":
		cmdRandom(args)
	case "agent-context":
		cmdAgentContext(args)
	case "version":
		fmt.Printf("gfonts %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}
