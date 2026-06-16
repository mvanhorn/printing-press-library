// Copyright 2026 Nik and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

const (
	docsBaseURL         = "https://code.claude.com"
	maxDocsResponseSize = 10 * 1024 * 1024
	maxVerifyFileSize   = 5 * 1024 * 1024
	maxVerifyFiles      = 10000
	maxVerifyDepth      = 20
)

type docsPage struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Hash    string `json:"hash"`
}

type docsSection struct {
	Page   string `json:"page"`
	Title  string `json:"title"`
	Group  string `json:"group,omitempty"`
	Anchor string `json:"anchor"`
	Level  int    `json:"level"`
	Body   string `json:"body,omitempty"`
	URL    string `json:"url"`
}

type docsSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Page      string   `json:"page"`
	Section   string   `json:"section"`
	Signature string   `json:"signature,omitempty"`
	Anchor    string   `json:"anchor"`
	URL       string   `json:"url"`
	Examples  []string `json:"examples,omitempty"`
}

type docsExample struct {
	Topic    string `json:"topic"`
	Page     string `json:"page"`
	Section  string `json:"section"`
	Language string `json:"language"`
	Code     string `json:"code"`
	URL      string `json:"url"`
}

type docsCorpus struct {
	Pages    []docsPage    `json:"pages"`
	Sections []docsSection `json:"sections"`
	Symbols  []docsSymbol  `json:"symbols"`
	Examples []docsExample `json:"examples"`
}

type searchHit struct {
	Title   string `json:"title"`
	Page    string `json:"page"`
	Kind    string `json:"kind"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Score   int    `json:"score"`
}

var knownDocsPages = []docsPage{
	{Key: "index", Title: "Claude Code docs index", Path: "/docs/llms.txt"},
	{Key: "python", Title: "Agent SDK reference - Python", Path: "/docs/en/agent-sdk/python.md"},
	{Key: "overview", Title: "Agent SDK overview", Path: "/docs/en/agent-sdk/overview.md"},
	{Key: "quickstart", Title: "Agent SDK quickstart", Path: "/docs/en/agent-sdk/quickstart.md"},
	{Key: "custom-tools", Title: "Custom tools", Path: "/docs/en/agent-sdk/custom-tools.md"},
	{Key: "sessions", Title: "Sessions", Path: "/docs/en/agent-sdk/sessions.md"},
	{Key: "permissions", Title: "Permissions", Path: "/docs/en/agent-sdk/permissions.md"},
	{Key: "structured-outputs", Title: "Structured outputs", Path: "/docs/en/agent-sdk/structured-outputs.md"},
	{Key: "mcp", Title: "MCP", Path: "/docs/en/agent-sdk/mcp.md"},
}

func newDocsReadCmd(flags *rootFlags) *cobra.Command {
	var pageKey string
	var section string
	cmd := &cobra.Command{
		Use:     "read [page-or-topic]",
		Short:   "Read a Claude Agent SDK docs page or section",
		Example: "  claude-agent-sdk-python-docs-pp-cli read --page python --section ClaudeSDKClient",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch Claude Agent SDK docs")
				return nil
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("--data-source local is not supported by read; run sync/search for local store access"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			corpus, err := loadDocsCorpus(ctx)
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			view := selectReadView(corpus, pageKey, section, query)
			if errText, ok := view["error"].(string); ok && errText != "" {
				return notFoundErr(fmt.Errorf("%s", errText))
			}
			return emitDocsView(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&pageKey, "page", "", "docs page key such as python, quickstart, custom-tools, sessions, permissions, structured-outputs, or mcp")
	cmd.Flags().StringVar(&section, "section", "", "section title or symbol to extract")
	return cmd
}

func newDocsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var typeFilter string
	cmd := &cobra.Command{
		Use:     "search <query>",
		Short:   "Search Claude Agent SDK Python docs",
		Example: "  claude-agent-sdk-python-docs-pp-cli search \"ClaudeAgentOptions\" --type pages --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search Claude Agent SDK docs")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("query is required"))
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("--data-source local is not supported by this docs search; omit it to query live Markdown"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			corpus, err := loadDocsCorpus(ctx)
			if err != nil {
				return err
			}
			hits := searchDocs(corpus, strings.Join(args, " "), typeFilter, limit)
			return emitDocsView(cmd, flags, map[string]any{"query": strings.Join(args, " "), "items": hits})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum search results to return")
	cmd.Flags().StringVar(&typeFilter, "type", "", "restrict search to pages, sections, symbols, or examples")
	return cmd
}

func newDocsSymbolCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var all bool
	cmd := &cobra.Command{
		Use:     "symbol <name>",
		Short:   "Look up a Python Agent SDK symbol",
		Example: "  claude-agent-sdk-python-docs-pp-cli symbol ClaudeSDKClient --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would look up a documented SDK symbol")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("symbol name is required"))
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("--data-source local is not supported by symbol; omit it to query live Markdown"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			corpus, err := loadDocsCorpus(ctx)
			if err != nil {
				return err
			}
			matches := findSymbols(corpus.Symbols, strings.Join(args, " "), kind)
			if len(matches) == 0 {
				return notFoundErr(fmt.Errorf("symbol %q not found", strings.Join(args, " ")))
			}
			if !all && len(matches) > 1 {
				matches = matches[:1]
			}
			return emitDocsView(cmd, flags, map[string]any{"query": strings.Join(args, " "), "symbols": matches})
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "symbol kind filter such as functions, classes, types, hooks, or tool inputs")
	cmd.Flags().BoolVar(&all, "all", false, "return all matching symbols")
	return cmd
}

func newDocsExamplesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var language string
	cmd := &cobra.Command{
		Use:     "examples [topic]",
		Short:   "Extract documented Python Agent SDK examples",
		Example: "  claude-agent-sdk-python-docs-pp-cli examples \"custom tools\" --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would extract documented examples")
				return nil
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("--data-source local is not supported by examples; omit it to query live Markdown"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			corpus, err := loadDocsCorpus(ctx)
			if err != nil {
				return err
			}
			examples := filterExamples(corpus.Examples, strings.Join(args, " "), language, limit)
			if strings.Join(args, " ") != "" && len(examples) == 0 {
				return notFoundErr(fmt.Errorf("no examples found for %q", strings.Join(args, " ")))
			}
			return emitDocsView(cmd, flags, map[string]any{"topic": strings.Join(args, " "), "examples": examples})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum examples to return")
	cmd.Flags().StringVar(&language, "language", "python", "code language filter")
	return cmd
}

func newDocsGuideCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "guide [topic]",
		Short:   "Find supporting Agent SDK guide pages",
		Example: "  claude-agent-sdk-python-docs-pp-cli guide sessions --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find related Agent SDK guide pages")
				return nil
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("--data-source local is not supported by guide; omit it to query live Markdown"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			corpus, err := loadDocsCorpus(ctx)
			if err != nil {
				return err
			}
			hits := searchDocs(corpus, strings.Join(args, " "), "pages", limit)
			if strings.Join(args, " ") != "" && len(hits) == 0 {
				return notFoundErr(fmt.Errorf("no guide pages found for %q", strings.Join(args, " ")))
			}
			return emitDocsView(cmd, flags, map[string]any{"topic": strings.Join(args, " "), "guides": hits})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 8, "maximum guide pages to return")
	return cmd
}

func runDocsContext(cmd *cobra.Command, flags *rootFlags, topic string, limit int) error {
	if topic == "" {
		_ = cmd.Usage()
		return usageErr(fmt.Errorf("topic is required"))
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	corpus, err := loadDocsCorpus(ctx)
	if err != nil {
		return err
	}
	sections := topSections(corpus.Sections, topic, limit)
	examples := filterExamples(corpus.Examples, topic, "", limit)
	symbols := findSymbols(corpus.Symbols, topic, "")
	if len(symbols) > limit {
		symbols = symbols[:limit]
	}
	if len(sections) == 0 && len(examples) == 0 && len(symbols) == 0 {
		return notFoundErr(fmt.Errorf("no docs context found for %q", topic))
	}
	view := map[string]any{
		"topic":     topic,
		"sections":  sections,
		"symbols":   symbols,
		"examples":  examples,
		"citations": citationsForSections(sections),
	}
	return emitDocsView(cmd, flags, view)
}

func runDocsRecipe(cmd *cobra.Command, flags *rootFlags, topic string, limit int) error {
	if topic == "" {
		_ = cmd.Usage()
		return usageErr(fmt.Errorf("topic is required"))
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	corpus, err := loadDocsCorpus(ctx)
	if err != nil {
		return err
	}
	examples := filterExamples(corpus.Examples, topic, "python", limit)
	symbols := findSymbols(corpus.Symbols, topic, "")
	if len(examples) == 0 && len(symbols) == 0 {
		return notFoundErr(fmt.Errorf("no recipe material found for %q", topic))
	}
	view := map[string]any{
		"topic":    topic,
		"symbols":  symbols,
		"examples": examples,
		"recipe":   composeRecipe(topic, symbols, examples),
	}
	return emitDocsView(cmd, flags, view)
}

func runDocsMap(cmd *cobra.Command, flags *rootFlags, kind string) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	corpus, err := loadDocsCorpus(ctx)
	if err != nil {
		return err
	}
	wanted := splitCSV(kind)
	grouped := map[string][]docsSymbol{}
	for _, sym := range corpus.Symbols {
		k := normalizeKind(sym.Kind)
		if strings.Contains(strings.ToLower(sym.Name), "options") {
			k = "options"
		}
		if len(wanted) > 0 && !containsString(wanted, k) && !containsString(wanted, sym.Kind) {
			continue
		}
		grouped[k] = append(grouped[k], sym)
	}
	for _, key := range wanted {
		if _, ok := grouped[key]; !ok {
			grouped[key] = []docsSymbol{}
		}
	}
	return emitDocsView(cmd, flags, map[string]any{"kind": kind, "groups": grouped})
}

func runDocsCoverageExamples(cmd *cobra.Command, flags *rootFlags) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	corpus, err := loadDocsCorpus(ctx)
	if err != nil {
		return err
	}
	withExamples := make([]docsSymbol, 0)
	withoutExamples := make([]docsSymbol, 0)
	for _, sym := range corpus.Symbols {
		if len(sym.Examples) > 0 {
			withExamples = append(withExamples, sym)
		} else {
			withoutExamples = append(withoutExamples, sym)
		}
	}
	return emitDocsView(cmd, flags, map[string]any{
		"symbols_with_examples":    withExamples,
		"symbols_without_examples": withoutExamples,
		"examples_total":           len(corpus.Examples),
	})
}

func runDocsAuditLinks(cmd *cobra.Command, flags *rootFlags) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	corpus, err := loadDocsCorpus(ctx)
	if err != nil {
		return err
	}
	anchors := map[string]bool{}
	for _, sec := range corpus.Sections {
		anchors[sec.Page+"#"+sec.Anchor] = true
	}
	var broken []map[string]string
	linkRe := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	for _, page := range corpus.Pages {
		for _, match := range linkRe.FindAllStringSubmatch(page.Content, -1) {
			target := match[1]
			if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "mailto:") || !strings.Contains(target, "#") {
				continue
			}
			parts := strings.SplitN(target, "#", 2)
			anchor := slugify(parts[1])
			if anchor == "" {
				continue
			}
			pageKey := page.Key
			if parts[0] != "" {
				pageKey = pageKeyFromPath(parts[0])
			}
			if !anchors[pageKey+"#"+anchor] {
				broken = append(broken, map[string]string{"page": page.Key, "target": target})
			}
		}
	}
	return emitDocsView(cmd, flags, map[string]any{"broken_links": broken, "checked_pages": len(corpus.Pages)})
}

func runDocsDiff(cmd *cobra.Command, flags *rootFlags, since string) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	corpus, err := loadDocsCorpus(ctx)
	if err != nil {
		return err
	}
	current := map[string]string{}
	for _, page := range corpus.Pages {
		current["page:"+page.Key] = page.Hash
	}
	for _, sec := range corpus.Sections {
		current["section:"+sec.Page+"#"+sec.Anchor] = hashString(sec.Body)
	}
	baselinePath := filepath.Join(os.TempDir(), "claude-agent-sdk-python-docs-pp-cli-baseline.json")
	var baseline map[string]string
	if data, err := os.ReadFile(baselinePath); err == nil { // #nosec G304 -- baselinePath is constructed from os.TempDir plus a fixed filename.
		_ = json.Unmarshal(data, &baseline)
	}
	added, changed, removed := diffHashes(baseline, current)
	return emitDocsView(cmd, flags, map[string]any{
		"since":          since,
		"baseline_path":  baselinePath,
		"baseline_found": baseline != nil,
		"added":          added,
		"changed":        changed,
		"removed":        removed,
		"note":           "write the current hash map to baseline_path to use it as a future baseline",
	})
}

func runDocsVerify(cmd *cobra.Command, flags *rootFlags, target string) error {
	if target == "" {
		_ = cmd.Usage()
		return usageErr(fmt.Errorf("path is required"))
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	corpus, err := loadDocsCorpus(ctx)
	if err != nil {
		return err
	}
	documented := map[string]docsSymbol{}
	for _, sym := range corpus.Symbols {
		documented[sym.Name] = sym
	}
	found, err := scanPythonSDKNames(target)
	if err != nil {
		return err
	}
	var unknown []string
	var known []docsSymbol
	for _, name := range found {
		if sym, ok := documented[name]; ok {
			known = append(known, sym)
		} else {
			unknown = append(unknown, name)
		}
	}
	return emitDocsView(cmd, flags, map[string]any{
		"path":               target,
		"documented_symbols": known,
		"unknown_symbols":    unknown,
		"status":             map[bool]string{true: "pass", false: "warn"}[len(unknown) == 0],
	})
}

func loadDocsCorpus(ctx context.Context) (docsCorpus, error) {
	pages := make([]docsPage, len(knownDocsPages))
	var wg sync.WaitGroup
	errs := make(chan error, len(knownDocsPages))
	for i, page := range knownDocsPages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			content, err := fetchDocsPath(ctx, page.Path)
			if err != nil {
				errs <- err
				return
			}
			page.URL = docsBaseURL + page.Path
			page.Content = content
			page.Hash = hashString(content)
			if title := firstMarkdownTitle(content); title != "" {
				page.Title = title
			}
			pages[i] = page
		}()
	}
	wg.Wait()
	close(errs)
	var fetchErrs []error
	for err := range errs {
		fetchErrs = append(fetchErrs, err)
	}
	if len(fetchErrs) > 0 {
		return docsCorpus{}, errors.Join(fetchErrs...)
	}
	for i := range pages {
		if pages[i].Content == "" {
			return docsCorpus{}, fmt.Errorf("fetching %s: empty docs page", knownDocsPages[i].Path)
		}
	}
	corpus := docsCorpus{Pages: pages}
	for _, page := range pages {
		sections := parseSections(page)
		corpus.Sections = append(corpus.Sections, sections...)
		corpus.Examples = append(corpus.Examples, extractExamples(page, sections)...)
	}
	corpus.Symbols = extractSymbols(corpus.Sections, corpus.Examples)
	return corpus, nil
}

func fetchDocsPath(ctx context.Context, path string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, docsBaseURL+path, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetching %s: %w", path, err)
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxDocsResponseSize+1))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if len(data) > maxDocsResponseSize {
			return "", fmt.Errorf("fetching %s: response exceeded %d bytes", path, maxDocsResponseSize)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return string(data), nil
		}
		lastErr = fmt.Errorf("fetching %s: HTTP %d", path, resp.StatusCode)
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			break
		}
	}
	return "", lastErr
}

func parseSections(page docsPage) []docsSection {
	var sections []docsSection
	var current *docsSection
	var group string
	scanner := bufio.NewScanner(strings.NewReader(page.Content))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if level, title, ok := parseHeading(line); ok {
			if current != nil {
				sections = append(sections, *current)
			}
			if level == 2 {
				group = title
			}
			current = &docsSection{
				Page:   page.Key,
				Title:  title,
				Group:  group,
				Anchor: slugify(title),
				Level:  level,
				URL:    page.URL + "#" + slugify(title),
			}
			continue
		}
		if current != nil {
			current.Body += line + "\n"
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections
}

func parseHeading(line string) (int, string, bool) {
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(line[level+1:]), true
}

func extractExamples(page docsPage, sections []docsSection) []docsExample {
	var examples []docsExample
	lines := strings.Split(page.Content, "\n")
	section := ""
	anchor := ""
	inCode := false
	lang := ""
	var code []string
	for _, line := range lines {
		if _, title, ok := parseHeading(line); ok {
			section = title
			anchor = slugify(title)
		}
		if strings.HasPrefix(line, "```") {
			if !inCode {
				inCode = true
				lang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				if idx := strings.IndexByte(lang, ' '); idx >= 0 {
					lang = lang[:idx]
				}
				code = nil
				continue
			}
			inCode = false
			if len(code) > 0 {
				examples = append(examples, docsExample{
					Topic:    section,
					Page:     page.Key,
					Section:  section,
					Language: lang,
					Code:     strings.Join(code, "\n"),
					URL:      page.URL + "#" + anchor,
				})
			}
			continue
		}
		if inCode {
			code = append(code, line)
		}
	}
	return examples
}

func extractSymbols(sections []docsSection, examples []docsExample) []docsSymbol {
	var symbols []docsSymbol
	exByName := map[string][]string{}
	for _, ex := range examples {
		for _, name := range possibleSymbolNames(ex.Code) {
			exByName[name] = append(exByName[name], ex.Code)
		}
	}
	for _, sec := range sections {
		if sec.Page != "python" || sec.Level > 3 {
			continue
		}
		name := symbolNameFromTitle(sec.Title)
		if name == "" {
			continue
		}
		sig := firstCodeLine(sec.Body)
		symbols = append(symbols, docsSymbol{
			Name:      name,
			Kind:      sec.Group,
			Page:      sec.Page,
			Section:   sec.Title,
			Signature: sig,
			Anchor:    sec.Anchor,
			URL:       sec.URL,
			Examples:  exByName[name],
		})
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].Name < symbols[j].Name })
	return symbols
}

func selectReadView(corpus docsCorpus, pageKey, section, query string) map[string]any {
	if pageKey == "" && query != "" {
		pageKey = pageKeyFromPath(query)
	}
	if pageKey == "" {
		pageKey = "python"
	}
	for _, page := range corpus.Pages {
		if page.Key != pageKey {
			continue
		}
		view := map[string]any{"page": page}
		if section != "" || query != "" {
			needle := section
			if needle == "" {
				needle = query
			}
			var matches []docsSection
			for _, sec := range corpus.Sections {
				if sec.Page == page.Key && strings.Contains(strings.ToLower(sec.Title+" "+sec.Body), strings.ToLower(needle)) {
					matches = append(matches, sec)
				}
			}
			view["sections"] = matches
			page.Content = ""
			view["page"] = page
		}
		return view
	}
	return map[string]any{"page": pageKey, "error": "page not found"}
}

func searchDocs(corpus docsCorpus, query, typeFilter string, limit int) []searchHit {
	if limit <= 0 {
		limit = 10
	}
	var hits []searchHit
	add := func(title, page, kind, url, body string) {
		score := scoreText(query, title+" "+body)
		if query != "" && score == 0 {
			return
		}
		hits = append(hits, searchHit{Title: title, Page: page, Kind: kind, URL: url, Snippet: snippet(body, query), Score: score})
	}
	if typeFilter == "" || typeFilter == "pages" {
		for _, p := range corpus.Pages {
			add(p.Title, p.Key, "page", p.URL, p.Content)
		}
	}
	if typeFilter == "" || typeFilter == "sections" {
		for _, s := range corpus.Sections {
			add(s.Title, s.Page, "section", s.URL, s.Body)
		}
	}
	if typeFilter == "" || typeFilter == "symbols" {
		for _, s := range corpus.Symbols {
			add(s.Name, s.Page, normalizeKind(s.Kind), s.URL, s.Signature+" "+s.Section)
		}
	}
	if typeFilter == "" || typeFilter == "examples" {
		for _, e := range corpus.Examples {
			add(e.Topic, e.Page, "example", e.URL, e.Code)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func topSections(sections []docsSection, topic string, limit int) []docsSection {
	hits := make([]searchHit, 0, len(sections))
	byURL := map[string]docsSection{}
	for _, sec := range sections {
		score := scoreText(topic, sec.Title+" "+sec.Body)
		if score == 0 {
			continue
		}
		hits = append(hits, searchHit{URL: sec.URL, Score: score})
		byURL[sec.URL] = sec
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if limit <= 0 {
		limit = 5
	}
	var out []docsSection
	for _, hit := range hits {
		out = append(out, byURL[hit.URL])
		if len(out) >= limit {
			break
		}
	}
	return out
}

func findSymbols(symbols []docsSymbol, query, kind string) []docsSymbol {
	query = strings.ToLower(strings.Trim(query, "` "))
	var out []docsSymbol
	for _, sym := range symbols {
		if kind != "" && !strings.Contains(strings.ToLower(sym.Kind), strings.ToLower(kind)) && normalizeKind(sym.Kind) != kind {
			continue
		}
		name := strings.ToLower(sym.Name)
		if query == "" || name == query || strings.Contains(name, query) || strings.Contains(strings.ToLower(sym.Section+" "+sym.Signature), query) {
			out = append(out, sym)
		}
	}
	return out
}

func filterExamples(examples []docsExample, topic, language string, limit int) []docsExample {
	if limit <= 0 {
		limit = 10
	}
	type scoredExample struct {
		example docsExample
		score   int
	}
	var scored []scoredExample
	for _, ex := range examples {
		if language != "" && ex.Language != "" && !strings.EqualFold(ex.Language, language) {
			continue
		}
		score := scoreText(topic, ex.Topic+" "+ex.Code)
		if topic != "" && score == 0 {
			continue
		}
		scored = append(scored, scoredExample{example: ex, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	out := make([]docsExample, 0, min(limit, len(scored)))
	for _, item := range scored {
		out = append(out, item.example)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func emitDocsView(cmd *cobra.Command, flags *rootFlags, v any) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), v, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func scanPythonSDKNames(target string) ([]string, error) {
	var files []string
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(targetAbs, path)
			if relErr == nil && strings.Count(rel, string(os.PathSeparator)) > maxVerifyDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && strings.HasSuffix(path, ".py") {
				if len(files) >= maxVerifyFiles {
					return fmt.Errorf("verify file limit exceeded: more than %d Python files", maxVerifyFiles)
				}
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = append(files, target)
	}
	names := map[string]bool{}
	importRe := regexp.MustCompile(`from\s+claude_agent_sdk\s+import\s+([^\n#]+)`)
	qualifiedRe := regexp.MustCompile(`claude_agent_sdk\.([A-Za-z_][A-Za-z0-9_]*)`)
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		if info.Size() > maxVerifyFileSize {
			return nil, fmt.Errorf("verifying %s: file exceeds %d bytes", file, maxVerifyFileSize)
		}
		data, err := os.ReadFile(file) // #nosec G304 -- verify intentionally scans caller-selected Python files with size and count bounds.
		if err != nil {
			return nil, err
		}
		text := string(data)
		for _, m := range importRe.FindAllStringSubmatch(text, -1) {
			for _, part := range strings.Split(m[1], ",") {
				name := strings.TrimSpace(strings.Split(part, " as ")[0])
				if name != "" && name != "*" {
					names[name] = true
				}
			}
		}
		for _, m := range qualifiedRe.FindAllStringSubmatch(text, -1) {
			names[m[1]] = true
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func diffHashes(old, current map[string]string) (added, changed, removed []string) {
	if old == nil {
		for key := range current {
			added = append(added, key)
		}
		sort.Strings(added)
		return added, nil, nil
	}
	for key, val := range current {
		if oldVal, ok := old[key]; !ok {
			added = append(added, key)
		} else if oldVal != val {
			changed = append(changed, key)
		}
	}
	for key := range old {
		if _, ok := current[key]; !ok {
			removed = append(removed, key)
		}
	}
	sort.Strings(added)
	sort.Strings(changed)
	sort.Strings(removed)
	return added, changed, removed
}

func composeRecipe(topic string, symbols []docsSymbol, examples []docsExample) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", topic)
	if len(symbols) > 0 {
		fmt.Fprintln(&b, "## Relevant documented symbols")
		for _, sym := range symbols {
			fmt.Fprintf(&b, "- `%s` from %s\n", sym.Name, sym.URL)
		}
		fmt.Fprintln(&b)
	}
	if len(examples) > 0 {
		fmt.Fprintln(&b, "## Documented example scaffold")
		fmt.Fprintln(&b, "```python")
		fmt.Fprintln(&b, examples[0].Code)
		fmt.Fprintln(&b, "```")
		fmt.Fprintf(&b, "\nSource: %s\n", examples[0].URL)
	}
	return b.String()
}

func citationsForSections(sections []docsSection) []map[string]string {
	out := make([]map[string]string, 0, len(sections))
	for _, sec := range sections {
		out = append(out, map[string]string{"title": sec.Title, "url": sec.URL})
	}
	return out
}

func scoreText(query, text string) int {
	query = strings.ToLower(query)
	text = strings.ToLower(text)
	score := 0
	for _, tok := range strings.FieldsFunc(query, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}) {
		if tok == "" {
			continue
		}
		score += strings.Count(text, tok)
	}
	if query != "" && strings.Contains(text, query) {
		score += 10
	}
	return score
}

func snippet(text, query string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= 240 {
		return text
	}
	q := strings.ToLower(strings.TrimSpace(query))
	idx := -1
	if q != "" {
		idx = strings.Index(strings.ToLower(text), q)
	}
	if idx < 0 {
		return text[:240]
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := start + 240
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}

func symbolNameFromTitle(title string) string {
	if strings.Contains(title, "`") {
		parts := strings.Split(title, "`")
		if len(parts) >= 2 {
			return strings.TrimSuffix(parts[1], "()")
		}
	}
	clean := strings.TrimSpace(title)
	if strings.Contains(clean, " ") {
		return ""
	}
	if clean == "" || strings.HasPrefix(clean, "Example") {
		return ""
	}
	return strings.TrimSuffix(clean, "()")
}

func possibleSymbolNames(code string) []string {
	re := regexp.MustCompile(`\b[A-Z][A-Za-z0-9_]+\b|\b[a-z_]+\(`)
	seen := map[string]bool{}
	for _, m := range re.FindAllString(code, -1) {
		name := strings.TrimSuffix(m, "(")
		seen[name] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	return out
}

func firstCodeLine(body string) string {
	inCode := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "```") {
			inCode = !inCode
			continue
		}
		if inCode && strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func firstMarkdownTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if level, title, ok := parseHeading(line); ok && level == 1 {
			return title
		}
	}
	return ""
}

func slugify(s string) string {
	s = strings.ToLower(strings.Trim(s, "` "))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func normalizeKind(kind string) string {
	k := strings.ToLower(kind)
	switch {
	case strings.Contains(k, "function"):
		return "functions"
	case strings.Contains(k, "class"):
		return "classes"
	case strings.Contains(k, "message"):
		return "messages"
	case strings.Contains(k, "hook"):
		return "hooks"
	case strings.Contains(k, "tool"):
		return "tools"
	case strings.Contains(k, "type"):
		return "types"
	default:
		return slugify(kind)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func pageKeyFromPath(s string) string {
	s = strings.TrimSpace(strings.Trim(s, "`"))
	s = strings.TrimPrefix(s, "/docs/en/agent-sdk/")
	s = strings.TrimPrefix(s, "https://code.claude.com/docs/en/agent-sdk/")
	s = strings.TrimSuffix(s, ".md")
	s = strings.TrimSuffix(s, "/")
	s = strings.ReplaceAll(s, "_", "-")
	for _, page := range knownDocsPages {
		if s == page.Key || strings.Contains(s, page.Key) {
			return page.Key
		}
	}
	return s
}
