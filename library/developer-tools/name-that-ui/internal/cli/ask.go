// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
// Ask is intentionally a lexical router. It never invokes a model, shell, or
// provider endpoint; a route either returns parsed terms or local candidates.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newAskCmd(flags))
	})
}

type askRoute struct {
	Name             string
	Confidence       string
	Reason           string
	SuggestedCommand string
	ParsedTerms      []string
	CandidateQuery   string
}

const askCandidateSummaryLimit = 5

func newAskCmd(flags *rootFlags) *cobra.Command {
	var dbPath, framework string
	cmd := &cobra.Command{
		Use:         "ask <natural-language-intent>",
		Short:       "Route a UI intent using explicit lexical cues",
		Example:     `  name-that-ui-pp-cli ask "what do you call a searchable dropdown?" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if isPrintingPressInvalidFixture(args) {
				return usageErr(fmt.Errorf("invalid intent fixture"))
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("%s requires <natural-language-intent>", cmd.CommandPath()))
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			route := lexicalAskRoute(args[0], framework)
			path := componentDBPath(dbPath)
			base := map[string]any{
				"intent": args[0], "route": route.Name, "confidence": route.Confidence, "reason": route.Reason,
				"suggested_command": route.SuggestedCommand, "parsed_terms": route.ParsedTerms, "candidate_query": route.CandidateQuery,
				"candidates": []recommendationCandidate{}, "style_candidates": []styleIdentifyCandidate{},
				"provenance": localReadProvenance{DataSource: "local", Resource: "none"}, "freshness": localFreshness{},
			}
			if dryRunOK(flags) {
				base["dry_run"] = true
				base["db_path"] = path
				base["sqlite_opened"] = false
				return askPrint(cmd, flags, base)
			}
			switch route.Name {
			case "identify", "recommend":
				items, meta, err := componentLoad(cmd, path)
				if err != nil {
					return err
				}
				base["candidates"] = capAskCandidates(recommendLocalCandidates(items, route.CandidateQuery, framework))
				base["provenance"] = localReadProvenance{DataSource: "local", Resource: "components"}
				base["freshness"] = localFreshness{SyncedAt: meta.SyncedAt, Stale: meta.Stale}
			case "style_identify":
				items, meta, err := styleLoad(cmd, styleDBPath(dbPath))
				if err != nil {
					return err
				}
				base["style_candidates"] = capAskStyleCandidates(styleIdentify(items, route.CandidateQuery))
				base["provenance"] = localReadProvenance{DataSource: "local", Resource: "style_details"}
				base["freshness"] = localFreshness{SyncedAt: meta.SyncedAt, Stale: meta.Stale}
			}
			return askPrint(cmd, flags, base)
		},
	}
	cmd.Flags().StringVar(&framework, "framework", "", "Filter component API mappings for candidate routes")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

var (
	compareIntentPattern    = regexp.MustCompile(`(?i)^\s*compare\s+(.+?)\s+(?:vs\.?|versus|with|and|to)\s+(.+?)\s*$`)
	differenceIntentPattern = regexp.MustCompile(`(?i)difference\s+between\s+(.+?)\s+and\s+(.+?)\s*$`)
	versusIntentPattern     = regexp.MustCompile(`(?i)^\s*(.+?)\s+(?:vs\.?|versus)\s+(.+?)\s*$`)
	translateIntentPattern  = regexp.MustCompile(`(?i)^\s*translate\s+(.+?)\s+(?:to|into|for)\s+(.+?)\s*$`)
)

func lexicalAskRoute(intent, framework string) askRoute {
	trimmed := strings.TrimSpace(intent)
	lower := strings.ToLower(trimmed)
	if matches := translateIntentPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
		terms := cleanAskTerms(matches[1], matches[2])
		return askRoute{Name: "translate", Confidence: "high", Reason: "matched explicit lexical cue: translate … to/into/for", ParsedTerms: terms, SuggestedCommand: "component api " + posixShellQuote(terms[0]) + " --framework " + posixShellQuote(terms[1])}
	}
	for _, pattern := range []*regexp.Regexp{compareIntentPattern, differenceIntentPattern, versusIntentPattern} {
		if matches := pattern.FindStringSubmatch(trimmed); len(matches) == 3 {
			terms := cleanAskTerms(matches[1], matches[2])
			return askRoute{Name: "component_compare", Confidence: "high", Reason: "matched explicit lexical comparison cue", ParsedTerms: terms, SuggestedCommand: "component compare " + posixShellQuote(terms[0]) + " " + posixShellQuote(terms[1])}
		}
	}
	if containsAny(lower, "style", "aesthetic", "visual language", "look and feel", "glassmorphism", "brutalism", "skeuomorphism", "minimalism") {
		return askRoute{Name: "style_identify", Confidence: "high", Reason: "matched explicit lexical style cue", ParsedTerms: []string{}, CandidateQuery: askSubject(trimmed, "identify", "a", "an", "the"), SuggestedCommand: "style identify " + posixShellQuote(trimmed)}
	}
	if containsAny(lower, "recommend", "should i use", "which component", "best component", "need a") {
		return askRoute{Name: "recommend", Confidence: "high", Reason: "matched explicit lexical recommendation cue", ParsedTerms: []string{}, CandidateQuery: askSubject(trimmed, "recommend", "should i use", "which component", "best component", "need a", "need"), SuggestedCommand: askRecommendationCommand(trimmed, framework)}
	}
	if containsAny(lower, "identify", "what is", "what do you call", "called", "name for") {
		return askRoute{Name: "identify", Confidence: "high", Reason: "matched explicit lexical identification cue", ParsedTerms: []string{}, CandidateQuery: askSubject(trimmed, "what do you call", "what is", "identify", "called", "name for"), SuggestedCommand: "identify " + posixShellQuote(trimmed)}
	}
	return askRoute{Name: "identify", Confidence: "low", Reason: "no recognized lexical cue; defaulted to identify without semantic inference", ParsedTerms: []string{}, CandidateQuery: trimmed, SuggestedCommand: "identify " + posixShellQuote(trimmed)}
}

// askSubject removes routing language before the deterministic local ranker
// sees a request. Without this, common words like "recommend" and "what" can
// make unrelated records look relevant merely because their descriptions are
// verbose.
func askSubject(intent string, prefixes ...string) string {
	value := strings.TrimSpace(intent)
	lower := strings.ToLower(value)
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			value = strings.TrimSpace(value[len(prefix):])
			break
		}
	}
	value = strings.TrimSpace(strings.Trim(value, " ?!.,:;\"'"))
	for _, article := range []string{"a ", "an ", "the "} {
		if strings.HasPrefix(strings.ToLower(value), article) {
			value = strings.TrimSpace(value[len(article):])
			break
		}
	}
	if value == "" {
		return strings.TrimSpace(intent)
	}
	return value
}

func capAskCandidates(candidates []recommendationCandidate) []recommendationCandidate {
	if len(candidates) <= askCandidateSummaryLimit {
		return candidates
	}
	return candidates[:askCandidateSummaryLimit]
}

func capAskStyleCandidates(candidates []styleIdentifyCandidate) []styleIdentifyCandidate {
	if len(candidates) <= askCandidateSummaryLimit {
		return candidates
	}
	return candidates[:askCandidateSummaryLimit]
}

func cleanAskTerms(values ...string) []string {
	terms := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.Trim(strings.TrimSpace(value), "\"'"))
		terms = append(terms, value)
	}
	return terms
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func askRecommendationCommand(intent, framework string) string {
	if framework == "" {
		return "recommend " + posixShellQuote(intent)
	}
	return "recommend " + posixShellQuote(intent) + " --framework " + posixShellQuote(framework)
}

// posixShellQuote emits one deterministic POSIX-shell word. Single quotes are
// the least-surprising portable form; an embedded quote is represented by
// ending the quoted word, inserting a literal quote, then resuming it.
func posixShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func askPrint(cmd *cobra.Command, flags *rootFlags, value any) error {
	if wantsMachineOutput(flags) {
		if flags.compact {
			value = compactAskValue(value)
		}
		return printJSONFiltered(cmd.OutOrStdout(), value, flags)
	}
	result, ok := value.(map[string]any)
	if !ok {
		return componentPrint(cmd, flags, value)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n%s\n", result["route"], result["confidence"], result["reason"], result["suggested_command"])
	return err
}

// compactAskValue is deliberately command-specific: generic object compaction
// preserves nested candidate records, which can make `ask --agent` emit an
// entire local component corpus. Agent mode instead returns routing evidence
// and a bounded candidate summary; fetch full records explicitly with the
// advertised follow-up command.
func compactAskValue(value any) any {
	base, ok := value.(map[string]any)
	if !ok {
		return value
	}
	compact := map[string]any{}
	for _, key := range []string{"intent", "route", "confidence", "reason", "suggested_command", "parsed_terms", "candidate_query", "provenance", "freshness", "dry_run", "db_path", "sqlite_opened"} {
		if item, found := base[key]; found {
			compact[key] = item
		}
	}
	if candidates, ok := base["candidates"].([]recommendationCandidate); ok {
		summaries := make([]map[string]any, 0, len(candidates))
		for _, candidate := range candidates {
			summaries = append(summaries, map[string]any{"id": candidate.ID, "name": candidate.Name, "platform": candidate.Platform, "score": candidate.Score, "source_url": candidate.SourceURL})
		}
		compact["candidates"] = summaries
	} else if candidates, found := base["candidates"]; found {
		compact["candidates"] = candidates
	}
	if candidates, ok := base["style_candidates"].([]styleIdentifyCandidate); ok {
		summaries := make([]map[string]any, 0, len(candidates))
		for _, candidate := range candidates {
			summaries = append(summaries, map[string]any{"id": candidate.Style.ID, "name": candidate.Style.Name, "score": candidate.Score, "source_url": candidate.Style.SourceURL})
		}
		compact["style_candidates"] = summaries
	} else if candidates, found := base["style_candidates"]; found {
		compact["style_candidates"] = candidates
	}
	compact["candidate_detail_hint"] = "Run component get <id> or style get <id> for a full local record."
	return compact
}
