// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/spf13/cobra"
)

const novelMaxFileBytes int64 = 2 << 20

// pp:data-source local
// Lint intentionally uses only the synced NameThatUI component mirror.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		rootPreRun := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if !flags.dryRun || (cmd.Name() != "lint" && cmd.Name() != "inventory") {
				return rootPreRun(cmd, args)
			}
			if flags.agent {
				flags.asJSON, flags.compact, flags.noInput, flags.yes, noColor = true, true, true, true, true
			}
			switch flags.dataSource {
			case "auto", "live", "local":
				return nil
			default:
				return usageErr(fmt.Errorf("invalid --data-source value %q: must be auto, live, or local", flags.dataSource))
			}
		}
	})
}

func newNovelLintCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "lint <path>",
		Short:       "Find colloquial or ambiguous UI terms in prose and return canonical",
		Example:     "name-that-ui-pp-cli lint ./SPEC.md --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("%s requires exactly one <path>", cmd.CommandPath()))
			}
			path := args[0]
			mirror := componentDBPath(dbPath)
			if dryRunOK(flags) {
				return componentPrint(cmd, flags, lintResponse{Path: path, DBPath: mirror, DryRun: true, FileRead: false, SQLiteOpened: false, Findings: []lintFinding{}})
			}
			body, err := readNovelTextFile(path)
			if err != nil {
				return err
			}
			items, meta, err := componentLoad(cmd, mirror)
			if err != nil {
				return err
			}
			return componentPrint(cmd, flags, lintResponse{Path: path, DBPath: mirror, DataSource: meta.DataSource, Findings: lintFindings(body, items)})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

type lintResponse struct {
	Path         string        `json:"path"`
	DBPath       string        `json:"db_path"`
	DataSource   string        `json:"data_source,omitempty"`
	DryRun       bool          `json:"dry_run,omitempty"`
	FileRead     bool          `json:"file_read"`
	SQLiteOpened bool          `json:"sqlite_opened"`
	Findings     []lintFinding `json:"findings"`
}

type lintFinding struct {
	Line                int                  `json:"line"`
	Column              int                  `json:"column"`
	MatchedPhrase       string               `json:"matched_phrase"`
	MatchKind           string               `json:"match_kind"`
	CanonicalCandidates []canonicalCandidate `json:"canonical_candidates"`
	Ambiguous           bool                 `json:"ambiguous"`
	SourceURLs          []string             `json:"source_urls"`
}

type canonicalCandidate struct {
	Component string `json:"component"`
	Part      string `json:"part,omitempty"`
	Framework string `json:"framework,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	SourceURL string `json:"source_url"`
}

type novelTerm struct {
	Phrase    string
	Kind      string
	Candidate canonicalCandidate
}

func readNovelTextFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory; lint requires one prose, specification, or prompt file", path)
	}
	if info.Size() > novelMaxFileBytes {
		return nil, fmt.Errorf("%s exceeds the 2 MiB limit", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if !utf8.Valid(body) || containsNUL(body) {
		return nil, fmt.Errorf("%s is binary; lint accepts text only", path)
	}
	return body, nil
}

func containsNUL(body []byte) bool { return strings.IndexByte(string(body), 0) >= 0 }

func lintFindings(body []byte, items []namethatui.Component) []lintFinding {
	terms := componentTerms(items, true, true)
	grouped := map[string]*lintFinding{}
	for _, term := range terms {
		caseSensitive := term.Kind == "api" || term.Kind == "part_api"
		for _, at := range boundedPhraseOffsets(string(body), term.Phrase, caseSensitive) {
			if lintContextSuppressed(term, string(body), at, at+len(term.Phrase)) {
				continue
			}
			if (term.Kind == "api" || term.Kind == "part_api") && lowInformationAPISymbol(term.Phrase) && !apiCodeContext(string(body), at, at+len(term.Phrase)) {
				continue
			}
			line, column := offsetLineColumn(body, at)
			key := fmt.Sprintf("%d:%d:%s", line, column, strings.ToLower(term.Phrase))
			finding := grouped[key]
			if finding == nil {
				finding = &lintFinding{Line: line, Column: column, MatchedPhrase: string(body[at : at+len(term.Phrase)]), MatchKind: term.Kind, CanonicalCandidates: []canonicalCandidate{}, SourceURLs: []string{}}
				grouped[key] = finding
			}
			if matchKindRank(term.Kind) < matchKindRank(finding.MatchKind) {
				finding.MatchKind = term.Kind
			}
			appendCandidate(finding, term.Candidate)
		}
	}
	findings := make([]lintFinding, 0, len(grouped))
	for _, finding := range grouped {
		finalizeFinding(finding)
		findings = append(findings, *finding)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Line == findings[j].Line {
			return findings[i].Column < findings[j].Column
		}
		return findings[i].Line < findings[j].Line
	})
	return findings
}

// lintContextSuppressed avoids treating common product/editor and auth prose
// as a UI reference. It intentionally targets only the overloaded terms, so
// real UI mentions such as `Editor(...)` and `Cursor { ... }` still pass
// through the normal API-syntax check.
func lintContextSuppressed(term novelTerm, text string, start, end int) bool {
	if (term.Kind == "api" || term.Kind == "part_api") && apiCodeContext(text, start, end) {
		return false
	}
	phrase := strings.ToLower(strings.TrimSpace(term.Phrase))
	lineStart := strings.LastIndex(text[:start], "\n") + 1
	lineEnd := len(text)
	if offset := strings.Index(text[end:], "\n"); offset >= 0 {
		lineEnd = end + offset
	}
	line := strings.ToLower(text[lineStart:lineEnd])
	switch phrase {
	case "cursor", "editor":
		return strings.Contains(line, "editor") || strings.Contains(line, " ide") || strings.Contains(line, "codebase") || strings.Contains(line, "repository") || strings.Contains(line, "terminal") || strings.Contains(line, "codex") || strings.Contains(line, "gemini cli") || strings.Contains(line, "github copilot") || strings.Contains(line, "claude code") || strings.Contains(line, "other agents")
	case "token":
		return strings.Contains(line, "auth") || strings.Contains(line, "credential") || strings.Contains(line, "bearer") || strings.Contains(line, "api key") || strings.Contains(line, "oauth") || strings.Contains(line, "account") || strings.Contains(line, "cookies") || strings.Contains(line, "browser session")
	case "auth":
		return strings.Contains(line, "auth token") || strings.Contains(line, "token auth") || strings.Contains(line, "authentication token")
	}
	return false
}

// Low-information framework symbols such as SwiftUI List appear constantly in
// ordinary prose. Retain them only when nearby syntax makes an API reference
// clear, so real `List { ... }` and `List(...)` usages remain detectable.
func lowInformationAPISymbol(symbol string) bool {
	_, found := map[string]struct{}{
		"list": {}, "view": {}, "text": {}, "image": {}, "menu": {}, "table": {}, "form": {}, "group": {}, "section": {}, "label": {},
	}[strings.ToLower(strings.TrimSpace(symbol))]
	return found
}

func apiCodeContext(text string, start, end int) bool {
	before := strings.TrimSpace(text[:start])
	after := strings.TrimSpace(text[end:])
	if strings.HasSuffix(before, ".") || strings.HasSuffix(before, "`") || strings.HasPrefix(after, "`") {
		return true
	}
	return strings.HasPrefix(after, "(") || strings.HasPrefix(after, "{") || strings.HasPrefix(after, "[") || strings.HasPrefix(after, "<")
}

func matchKindRank(kind string) int {
	switch kind {
	case "component":
		return 0
	case "alias":
		return 1
	case "fuzzy_phrase":
		return 2
	case "part":
		return 3
	case "api":
		return 4
	case "part_api":
		return 5
	default:
		return 6
	}
}

func componentTerms(items []namethatui.Component, includeFuzzy, includeAPI bool) []novelTerm {
	terms := []novelTerm{}
	for _, component := range items {
		candidate := canonicalCandidate{Component: component.Name, SourceURL: component.SourceURL}
		terms = appendNovelTerm(terms, component.Name, "component", candidate)
		terms = appendNovelTerm(terms, component.Slug, "component", candidate)
		for _, alias := range component.AKA {
			terms = appendNovelTerm(terms, alias, "alias", candidate)
		}
		if includeFuzzy {
			for _, phrase := range component.Fuzzy {
				terms = appendNovelTerm(terms, phrase, "fuzzy_phrase", candidate)
			}
		}
		if includeAPI {
			for _, api := range component.API {
				terms = appendAPITerm(terms, api.Symbol, "api", canonicalCandidate{Component: component.Name, Framework: api.Framework, Symbol: api.Symbol, SourceURL: component.SourceURL})
			}
		}
		for _, part := range component.Parts {
			partCandidate := canonicalCandidate{Component: component.Name, Part: part.Name, Symbol: part.API, SourceURL: component.SourceURL}
			terms = appendNovelTerm(terms, part.Name, "part", partCandidate)
			terms = appendNovelTerm(terms, part.ID, "part", partCandidate)
			if includeAPI {
				terms = appendAPITerm(terms, part.API, "part_api", partCandidate)
			}
		}
	}
	sort.Slice(terms, func(i, j int) bool {
		if strings.EqualFold(terms[i].Phrase, terms[j].Phrase) {
			if terms[i].Candidate.Component == terms[j].Candidate.Component {
				return terms[i].Kind < terms[j].Kind
			}
			return terms[i].Candidate.Component < terms[j].Candidate.Component
		}
		return strings.ToLower(terms[i].Phrase) < strings.ToLower(terms[j].Phrase)
	})
	return terms
}

func appendNovelTerm(terms []novelTerm, phrase, kind string, candidate canonicalCandidate) []novelTerm {
	if !meaningfulPhrase(phrase) {
		return terms
	}
	return append(terms, novelTerm{Phrase: phrase, Kind: kind, Candidate: candidate})
}

func appendAPITerm(terms []novelTerm, phrase, kind string, candidate canonicalCandidate) []novelTerm {
	if !apiShapedSymbol(phrase) {
		return terms
	}
	return appendNovelTerm(terms, phrase, kind, candidate)
}

func apiShapedSymbol(symbol string) bool {
	for _, r := range strings.TrimSpace(symbol) {
		if unicode.IsUpper(r) || unicode.IsDigit(r) || !unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func meaningfulPhrase(phrase string) bool {
	letters := 0
	for _, r := range phrase {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letters++
		}
	}
	return letters >= 3
}

func boundedPhraseOffsets(text, phrase string, caseSensitive bool) []int {
	if !meaningfulPhrase(phrase) {
		return []int{}
	}
	haystack, needle := text, phrase
	if !caseSensitive {
		haystack, needle = strings.ToLower(text), strings.ToLower(phrase)
	}
	offsets := []int{}
	for from := 0; from < len(haystack); {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			break
		}
		at := from + i
		end := at + len(needle)
		if tokenBoundary(text, at, end) {
			offsets = append(offsets, at)
		}
		from = at + len(needle)
	}
	return offsets
}

func tokenBoundary(text string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:start])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return false
		}
	}
	if end < len(text) {
		r, _ := utf8.DecodeRuneInString(text[end:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return false
		}
	}
	return true
}

func offsetLineColumn(body []byte, offset int) (int, int) {
	line, column := 1, 1
	for i := 0; i < offset; {
		if body[i] == '\n' {
			line, column = line+1, 1
			i++
			continue
		}
		_, size := utf8.DecodeRune(body[i:])
		column++
		i += size
	}
	return line, column
}

func appendCandidate(finding *lintFinding, candidate canonicalCandidate) {
	for i, existing := range finding.CanonicalCandidates {
		if existing.Component == candidate.Component && existing.Part == candidate.Part && existing.SourceURL == candidate.SourceURL {
			if existing.Symbol == "" && candidate.Symbol != "" {
				finding.CanonicalCandidates[i] = candidate
			}
			return
		}
	}
	finding.CanonicalCandidates = append(finding.CanonicalCandidates, candidate)
}

func finalizeFinding(finding *lintFinding) {
	sort.Slice(finding.CanonicalCandidates, func(i, j int) bool {
		left, right := finding.CanonicalCandidates[i], finding.CanonicalCandidates[j]
		if left.Component == right.Component {
			return left.Part < right.Part
		}
		return left.Component < right.Component
	})
	seen := map[string]bool{}
	for _, candidate := range finding.CanonicalCandidates {
		if !seen[candidate.SourceURL] {
			seen[candidate.SourceURL] = true
			finding.SourceURLs = append(finding.SourceURLs, candidate.SourceURL)
		}
	}
	sort.Strings(finding.SourceURLs)
	finding.Ambiguous = len(finding.CanonicalCandidates) > 1
}

func novelRelativePath(root, path string) string {
	if root == path {
		return filepath.Base(path)
	}
	value, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.ToSlash(value)
}
