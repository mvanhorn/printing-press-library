// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
// Style commands read only the synced NameThatUI style_details mirror.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { root.AddCommand(newStyleCmd(flags)) })
}

type styleMeta struct {
	DataSource string `json:"data_source"`
	SyncedAt   string `json:"synced_at,omitempty"`
	Stale      bool   `json:"stale"`
}

type styleCandidates struct {
	Ambiguous  bool               `json:"ambiguous"`
	Candidates []namethatui.Style `json:"candidates"`
	Meta       styleMeta          `json:"meta"`
}

type styleEvidence struct {
	Field   string `json:"field"`
	Snippet string `json:"snippet"`
	Score   int    `json:"score"`
}

type styleIdentifyCandidate struct {
	Style    namethatui.Style `json:"style"`
	Score    int              `json:"score"`
	Evidence []styleEvidence  `json:"evidence"`
}

func newStyleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "style",
		Short:       "Read locally synced NameThatUI visual styles",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.AddCommand(newStyleIdentifyCmd(flags, &dbPath))
	cmd.AddCommand(newStyleListCmd(flags, &dbPath))
	cmd.AddCommand(newStyleGetCmd(flags, &dbPath))
	cmd.AddCommand(newStyleSignalsCmd(flags, &dbPath))
	cmd.AddCommand(newStyleCompareCmd(flags, &dbPath))
	cmd.AddCommand(newStyleCodeCmd(flags, &dbPath))
	cmd.AddCommand(newStyleCautionsCmd(flags, &dbPath))
	return cmd
}

func styleDBPath(value string) string {
	if value != "" {
		return value
	}
	return defaultDBPath("name-that-ui-pp-cli")
}

func styleDryRun(cmd *cobra.Command, flags *rootFlags, operation string, args []string, dbPath string) error {
	return stylePrint(cmd, flags, map[string]any{
		"dry_run":       true,
		"operation":     operation,
		"args":          nonNilStringSlice(args),
		"data_source":   "local",
		"db_path":       styleDBPath(dbPath),
		"sqlite_opened": false,
	})
}

func styleStore(cmd *cobra.Command, path string) (*store.Store, styleMeta, error) {
	const hint = "run 'name-that-ui-pp-cli sync --resources styles' first"
	if _, err := os.Stat(path); err != nil {
		return nil, styleMeta{}, fmt.Errorf("NameThatUI style mirror is unavailable; %s: %w", hint, err)
	}
	db, err := store.OpenReadOnlyContext(cmd.Context(), path)
	if err != nil {
		return nil, styleMeta{}, fmt.Errorf("opening NameThatUI style mirror; %s: %w", hint, err)
	}
	_, synced, count, err := db.GetSyncState("style_details")
	if err != nil || count == 0 {
		_ = db.Close()
		return nil, styleMeta{}, fmt.Errorf("NameThatUI style mirror is unavailable; %s", hint)
	}
	meta := styleMeta{DataSource: "local"}
	if !synced.IsZero() {
		meta.SyncedAt = synced.UTC().Format(time.RFC3339)
		meta.Stale = time.Since(synced) > flagsMaxAge(cmd)
	}
	return db, meta, nil
}

func styleLoad(cmd *cobra.Command, path string) ([]namethatui.Style, styleMeta, error) {
	db, meta, err := styleStore(cmd, path)
	if err != nil {
		return nil, meta, err
	}
	defer db.Close()
	raw, err := db.List("style_details", 0)
	if err != nil {
		return nil, meta, err
	}
	items := make([]namethatui.Style, 0, len(raw))
	for _, item := range raw {
		var style namethatui.Style
		if json.Unmarshal(item, &style) == nil {
			style.Signals = nonNilSignals(style.Signals)
			style.Sections = nonNilSections(style.Sections)
			items = append(items, style)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	return items, meta, nil
}

func nonNilSignals(v []namethatui.Signal) []namethatui.Signal {
	if v == nil {
		return []namethatui.Signal{}
	}
	return v
}

func nonNilSections(v []namethatui.Section) []namethatui.Section {
	if v == nil {
		return []namethatui.Section{}
	}
	return v
}

func nonNilStyles(v []namethatui.Style) []namethatui.Style {
	if v == nil {
		return []namethatui.Style{}
	}
	return v
}

func nonNilStringSlice(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func styleNormalize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

func styleResolve(items []namethatui.Style, query string) (*namethatui.Style, []namethatui.Style) {
	exact := make([]namethatui.Style, 0)
	for _, style := range items {
		if style.Slug == query || style.Name == query {
			exact = append(exact, style)
		}
	}
	if len(exact) == 1 {
		return &exact[0], nil
	}
	if len(exact) > 1 {
		return nil, exact
	}

	q := styleNormalize(query)
	normalized := make([]namethatui.Style, 0)
	for _, style := range items {
		if styleNormalize(style.Slug) == q || styleNormalize(style.Name) == q {
			normalized = append(normalized, style)
		}
	}
	if len(normalized) == 1 {
		return &normalized[0], nil
	}
	if len(normalized) > 1 {
		return nil, normalized
	}
	if len(q) < 3 {
		return nil, []namethatui.Style{}
	}
	fuzzy := make([]namethatui.Style, 0)
	for _, style := range items {
		if strings.Contains(styleNormalize(style.Slug), q) || strings.Contains(styleNormalize(style.Name), q) {
			fuzzy = append(fuzzy, style)
		}
	}
	if len(fuzzy) == 1 {
		return &fuzzy[0], nil
	}
	return nil, fuzzy
}

func newStyleListCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List locally synced visual styles", Example: "  name-that-ui-pp-cli style list --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return styleDryRun(cmd, flags, "style.list", args, *dbPath)
		}
		if limit < 0 {
			return usageErr(fmt.Errorf("--limit must be zero or greater"))
		}
		items, meta, err := styleLoad(cmd, styleDBPath(*dbPath))
		if err != nil {
			return err
		}
		if limit > 0 && len(items) > limit {
			items = items[:limit]
		}
		return stylePrint(cmd, flags, map[string]any{"results": nonNilStyles(items), "meta": meta})
	}}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum results (0 for all)")
	return cmd
}

func newStyleGetCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newStyleReadCmd(flags, dbPath, &cobra.Command{Use: "get <style>", Short: "Get a visual style", Example: "  name-that-ui-pp-cli style get glassmorphism --agent"}, "get", func(style *namethatui.Style, meta styleMeta) map[string]any {
		return map[string]any{"result": style, "source_url": style.SourceURL, "meta": meta}
	})
}

func newStyleSignalsCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newStyleReadCmd(flags, dbPath, &cobra.Command{Use: "signals <style>", Short: "Get upstream style signals", Example: "  name-that-ui-pp-cli style signals glassmorphism --agent"}, "signals", func(style *namethatui.Style, meta styleMeta) map[string]any {
		return map[string]any{"style": style, "signals": nonNilSignals(style.Signals), "source_url": style.SourceURL, "meta": meta}
	})
}

func newStyleReadCmd(flags *rootFlags, dbPath *string, cmd *cobra.Command, operation string, result func(*namethatui.Style, styleMeta) map[string]any) *cobra.Command {
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if isPrintingPressInvalidFixture(args) {
			return usageErr(fmt.Errorf("invalid style fixture"))
		}
		if dryRunOK(flags) {
			return styleDryRun(cmd, flags, "style."+operation, args, *dbPath)
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("%s requires <slug-or-name>", cmd.CommandPath()))
		}
		items, meta, err := styleLoad(cmd, styleDBPath(*dbPath))
		if err != nil {
			return err
		}
		style, candidates := styleResolve(items, args[0])
		if style == nil {
			return stylePrint(cmd, flags, styleCandidates{Ambiguous: len(candidates) > 1, Candidates: nonNilStyles(candidates), Meta: meta})
		}
		return stylePrint(cmd, flags, result(style, meta))
	}
	return cmd
}

func newStyleIdentifyCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return &cobra.Command{Use: "identify <description>", Short: "Rank locally synced styles by upstream evidence", Example: `  name-that-ui-pp-cli style identify "frosted translucent cards" --agent`, RunE: func(cmd *cobra.Command, args []string) error {
		if isPrintingPressInvalidFixture(args) {
			return usageErr(fmt.Errorf("invalid style fixture"))
		}
		if dryRunOK(flags) {
			return styleDryRun(cmd, flags, "style.identify", args, *dbPath)
		}
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
			return usageErr(fmt.Errorf("%s requires <description>", cmd.CommandPath()))
		}
		items, meta, err := styleLoad(cmd, styleDBPath(*dbPath))
		if err != nil {
			return err
		}
		candidates := styleIdentify(items, args[0])
		return stylePrint(cmd, flags, map[string]any{"description": args[0], "candidates": candidates, "meta": meta})
	}}
}

func styleIdentify(items []namethatui.Style, description string) []styleIdentifyCandidate {
	queryTokens := styleTokens(description)
	if len(queryTokens) == 0 {
		return []styleIdentifyCandidate{}
	}
	candidates := make([]styleIdentifyCandidate, 0, len(items))
	for _, style := range items {
		evidence := make([]styleEvidence, 0)
		appendEvidence := func(field, text string, weight int) {
			if score := styleEvidenceScore(queryTokens, description, text, weight); score > 0 {
				evidence = append(evidence, styleEvidence{Field: field, Snippet: styleSnippet(text, queryTokens), Score: score})
			}
		}
		appendEvidence("style_name", style.Name, 30)
		for _, signal := range style.Signals {
			appendEvidence("signal_name", signal.Name, 16)
			appendEvidence("signal_description", signal.Description, 9)
		}
		for _, section := range style.Sections {
			appendEvidence("section_heading", section.Heading, 12)
			appendEvidence("section_text", section.Text, 5)
		}
		score := 0
		for _, item := range evidence {
			score += item.Score
		}
		if score > 0 {
			candidates = append(candidates, styleIdentifyCandidate{Style: style, Score: score, Evidence: evidence})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Style.Name < candidates[j].Style.Name
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func styleTokens(s string) []string {
	stopWords := map[string]struct{}{"a": {}, "an": {}, "and": {}, "as": {}, "at": {}, "for": {}, "from": {}, "in": {}, "is": {}, "of": {}, "on": {}, "or": {}, "the": {}, "to": {}, "ui": {}, "with": {}}
	seen := map[string]struct{}{}
	tokens := make([]string, 0)
	for _, token := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(token) < 3 {
			continue
		}
		if _, stop := stopWords[token]; stop {
			continue
		}
		if _, exists := seen[token]; !exists {
			seen[token] = struct{}{}
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func styleEvidenceScore(queryTokens []string, description, text string, weight int) int {
	fieldTokens := make(map[string]struct{})
	for _, token := range styleTokens(text) {
		fieldTokens[token] = struct{}{}
	}
	matches := 0
	for _, token := range queryTokens {
		if _, ok := fieldTokens[token]; ok {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}
	score := matches * weight
	query := styleNormalize(description)
	field := styleNormalize(text)
	if len(query) >= 4 && strings.Contains(field, query) {
		score += weight * 2
	}
	return score
}

func styleSnippet(text string, queryTokens []string) string {
	const max = 220
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	start := -1
	for _, token := range queryTokens {
		if index := strings.Index(lower, token); index >= 0 && (start < 0 || index < start) {
			start = index
		}
	}
	if start < 0 || len(trimmed) <= max {
		if len(trimmed) <= max {
			return trimmed
		}
		return trimmed[:max-1] + "…"
	}
	from := start - 70
	if from < 0 {
		from = 0
	}
	to := from + max
	if to > len(trimmed) {
		to = len(trimmed)
		from = maxInt(0, to-max)
	}
	snippet := trimmed[from:to]
	if from > 0 {
		snippet = "…" + snippet
	}
	if to < len(trimmed) {
		snippet += "…"
	}
	return snippet
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func newStyleCompareCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return &cobra.Command{Use: "compare <left> <right>", Short: "Compare two locally synced styles", Example: "  name-that-ui-pp-cli style compare glassmorphism minimalism --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return styleDryRun(cmd, flags, "style.compare", args, *dbPath)
		}
		if len(args) != 2 {
			return usageErr(fmt.Errorf("%s requires <left> <right>", cmd.CommandPath()))
		}
		items, meta, err := styleLoad(cmd, styleDBPath(*dbPath))
		if err != nil {
			return err
		}
		left, leftCandidates := styleResolve(items, args[0])
		right, rightCandidates := styleResolve(items, args[1])
		if left == nil || right == nil {
			return stylePrint(cmd, flags, map[string]any{"left_candidates": nonNilStyles(leftCandidates), "right_candidates": nonNilStyles(rightCandidates), "meta": meta})
		}
		return stylePrint(cmd, flags, map[string]any{
			"left":                        left,
			"right":                       right,
			"signal_overlap":              styleOverlap(styleSignalNames(left), styleSignalNames(right)),
			"signal_differences":          styleDifferences(styleSignalNames(left), styleSignalNames(right)),
			"section_heading_overlap":     styleOverlap(styleSectionHeadings(left), styleSectionHeadings(right)),
			"section_heading_differences": styleDifferences(styleSectionHeadings(left), styleSectionHeadings(right)),
			"meta":                        meta,
		})
	}}
}

func styleSignalNames(style *namethatui.Style) []string {
	values := make([]string, 0, len(style.Signals))
	for _, signal := range style.Signals {
		values = append(values, signal.Name)
	}
	return values
}

func styleSectionHeadings(style *namethatui.Style) []string {
	values := make([]string, 0, len(style.Sections))
	for _, section := range style.Sections {
		values = append(values, section.Heading)
	}
	return values
}

func styleOverlap(left, right []string) []string {
	rightByKey := map[string]struct{}{}
	for _, value := range right {
		rightByKey[styleNormalize(value)] = struct{}{}
	}
	values := make([]string, 0)
	for _, value := range left {
		if _, ok := rightByKey[styleNormalize(value)]; ok {
			values = append(values, value)
		}
	}
	return sortedUniqueStrings(values)
}

func styleDifferences(left, right []string) map[string][]string {
	rightByKey := map[string]struct{}{}
	leftByKey := map[string]struct{}{}
	for _, value := range right {
		rightByKey[styleNormalize(value)] = struct{}{}
	}
	for _, value := range left {
		leftByKey[styleNormalize(value)] = struct{}{}
	}
	leftOnly := make([]string, 0)
	for _, value := range left {
		if _, ok := rightByKey[styleNormalize(value)]; !ok {
			leftOnly = append(leftOnly, value)
		}
	}
	rightOnly := make([]string, 0)
	for _, value := range right {
		if _, ok := leftByKey[styleNormalize(value)]; !ok {
			rightOnly = append(rightOnly, value)
		}
	}
	return map[string][]string{"left_only": sortedUniqueStrings(leftOnly), "right_only": sortedUniqueStrings(rightOnly)}
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := styleNormalize(value)
		if key == "" {
			key = value
		}
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func newStyleCodeCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newStyleSectionsCmd(flags, dbPath, &cobra.Command{Use: "code <style>", Short: "Get upstream code and implementation sections", Example: "  name-that-ui-pp-cli style code glassmorphism --agent"}, "code", styleCodeSection, "No upstream sections have headings matching code, implementation, or starting points.")
}

func newStyleCautionsCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	return newStyleSectionsCmd(flags, dbPath, &cobra.Command{Use: "cautions <style>", Short: "Get upstream accessibility and caution sections", Example: "  name-that-ui-pp-cli style cautions glassmorphism --agent"}, "cautions", styleCautionSection, "")
}

func newStyleSectionsCmd(flags *rootFlags, dbPath *string, cmd *cobra.Command, operation string, matches func(string) bool, noMatchReason string) *cobra.Command {
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if isPrintingPressInvalidFixture(args) {
			return usageErr(fmt.Errorf("invalid style fixture"))
		}
		if dryRunOK(flags) {
			return styleDryRun(cmd, flags, "style."+operation, args, *dbPath)
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("%s requires <slug-or-name>", cmd.CommandPath()))
		}
		items, meta, err := styleLoad(cmd, styleDBPath(*dbPath))
		if err != nil {
			return err
		}
		style, candidates := styleResolve(items, args[0])
		if style == nil {
			return stylePrint(cmd, flags, styleCandidates{Ambiguous: len(candidates) > 1, Candidates: nonNilStyles(candidates), Meta: meta})
		}
		sections := make([]namethatui.Section, 0)
		for _, section := range style.Sections {
			if matches(section.Heading) {
				sections = append(sections, section)
			}
		}
		// Keep the full upstream record out of this response: `sections` is the
		// complete, intentionally filtered upstream content for this command.
		output := map[string]any{"style": styleSummary(style), "sections": sections, "source_url": style.SourceURL, "meta": meta}
		if len(sections) == 0 && noMatchReason != "" {
			output["reason"] = noMatchReason
		}
		return stylePrint(cmd, flags, output)
	}
	return cmd
}

func styleSummary(style *namethatui.Style) map[string]string {
	return map[string]string{"id": style.ID, "slug": style.Slug, "name": style.Name, "source_url": style.SourceURL}
}

func styleCodeSection(heading string) bool {
	heading = strings.ToLower(heading)
	return strings.Contains(heading, "code") || strings.Contains(heading, "implementation") || strings.Contains(heading, "starting point")
}

func styleCautionSection(heading string) bool {
	heading = strings.ToLower(heading)
	return strings.Contains(heading, "accessibility") || strings.Contains(heading, "misuse") || strings.Contains(heading, "caution") || strings.Contains(heading, "avoid")
}

func stylePrint(cmd *cobra.Command, flags *rootFlags, value any) error {
	if wantsMachineOutput(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), value, flags)
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return err
}
