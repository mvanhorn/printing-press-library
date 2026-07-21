// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/spf13/cobra"
)

// pp:data-source local
// Recommend is a markerless extension over the synced component mirror. It
// deliberately does not call the semantic endpoint: ranking is reproducible
// from the upstream records stored in SQLite.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		rootPreRun := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if !dryRunOK(flags) || (cmd.Name() != "recommend" && cmd.Name() != "ask") {
				return rootPreRun(cmd, args)
			}
			if flags.agent {
				flags.asJSON = true
				flags.compact = true
				flags.noInput = true
				flags.yes = true
				noColor = true
			}
			return validateDataSourceStrategy(flags, "local")
		}
		root.AddCommand(newRecommendCmd(flags))
	})
}

type recommendationEvidence struct {
	Field   string `json:"field"`
	Snippet string `json:"snippet"`
	Score   int    `json:"score"`
}

type recommendationCandidate struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	Platform      string                   `json:"platform"`
	Slug          string                   `json:"slug"`
	Description   string                   `json:"description"`
	Tagline       string                   `json:"tagline"`
	Score         int                      `json:"score"`
	Evidence      []recommendationEvidence `json:"evidence"`
	StatesOrParts []namethatui.Part        `json:"states_or_parts"`
	FrameworkAPIs []namethatui.API         `json:"framework_apis"`
	RelatedRefs   []any                    `json:"related_refs"`
	Prompt        string                   `json:"prompt"`
	DebugPrompt   string                   `json:"debug_prompt"`
	WatchOuts     []string                 `json:"watch_outs"`
	SourceURL     string                   `json:"source_url"`
}

type localReadProvenance struct {
	DataSource string `json:"data_source"`
	Resource   string `json:"resource"`
}

type localFreshness struct {
	SyncedAt string `json:"synced_at,omitempty"`
	Stale    bool   `json:"stale"`
}

type recommendResponse struct {
	Scenario     string                    `json:"scenario"`
	Framework    string                    `json:"framework"`
	Limit        int                       `json:"limit"`
	Ambiguous    bool                      `json:"ambiguous"`
	Choice       *recommendationCandidate  `json:"choice"`
	Alternatives []recommendationCandidate `json:"alternatives"`
	SourceURLs   []string                  `json:"source_urls"`
	Provenance   localReadProvenance       `json:"provenance"`
	Freshness    localFreshness            `json:"freshness"`
}

func newRecommendCmd(flags *rootFlags) *cobra.Command {
	var dbPath, framework string
	var limit int
	cmd := &cobra.Command{
		Use:         "recommend <scenario>",
		Short:       "Recommend locally synced UI components for a scenario",
		Example:     `  name-that-ui-pp-cli recommend "choose one option from a searchable list" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if isPrintingPressInvalidFixture(args) {
				return usageErr(fmt.Errorf("invalid scenario fixture"))
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("%s requires <scenario>", cmd.CommandPath()))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			path := componentDBPath(dbPath)
			if dryRunOK(flags) {
				return recommendPrint(cmd, flags, map[string]any{
					"scenario": args[0], "framework": framework, "limit": limit, "dry_run": true,
					"db_path": path, "sqlite_opened": false, "ambiguous": false, "choice": nil,
					"alternatives": []recommendationCandidate{}, "source_urls": []string{},
					"provenance": localReadProvenance{DataSource: "local", Resource: "components"},
					"freshness":  localFreshness{},
				})
			}
			items, meta, err := componentLoad(cmd, path)
			if err != nil {
				return err
			}
			allCandidates := recommendLocalCandidates(items, args[0], framework)
			ambiguous := len(allCandidates) > 1 && allCandidates[0].Score-allCandidates[1].Score <= 8
			candidates := allCandidates
			if len(candidates) > limit {
				candidates = candidates[:limit]
			}
			response := recommendResponse{
				Scenario: args[0], Framework: framework, Limit: limit,
				Alternatives: []recommendationCandidate{}, SourceURLs: []string{},
				Provenance: localReadProvenance{DataSource: "local", Resource: "components"},
				Freshness:  localFreshness{SyncedAt: meta.SyncedAt, Stale: meta.Stale},
			}
			if len(candidates) > 0 {
				response.Choice = &candidates[0]
				response.Ambiguous = ambiguous
				response.SourceURLs = append(response.SourceURLs, candidates[0].SourceURL)
				if len(candidates) > 1 {
					response.Alternatives = append(response.Alternatives, candidates[1:]...)
					for _, candidate := range candidates[1:] {
						response.SourceURLs = append(response.SourceURLs, candidate.SourceURL)
					}
				}
			}
			return recommendPrint(cmd, flags, response)
		},
	}
	cmd.Flags().StringVar(&framework, "framework", "", "Filter framework API mappings")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum recommendations")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func recommendLocalCandidates(items []namethatui.Component, scenario, framework string) []recommendationCandidate {
	// Share identify's compound-token normalization so a user-written phrase
	// such as "combo box" can match the canonical one-word `combobox` slug.
	tokens := identifyQueryTokens(scenario)
	if len(tokens) == 0 {
		return []recommendationCandidate{}
	}
	candidates := make([]recommendationCandidate, 0, len(items))
	for _, component := range items {
		candidate := recommendationCandidate{
			ID: component.ID, Name: component.Name, Platform: component.Platform, Slug: component.Slug,
			Description: component.Description, Tagline: component.Tagline, Evidence: []recommendationEvidence{},
			StatesOrParts: nonNilParts(component.Parts), FrameworkAPIs: componentAPIs(component.API, framework),
			RelatedRefs: componentRelated(items, component.Related), Prompt: component.Prompt,
			DebugPrompt: component.DebugPrompt, WatchOuts: upstreamWatchOuts(component), SourceURL: component.SourceURL,
		}
		addEvidence := func(field, text string, weight int) {
			matches := identifyTokenMatches(tokens, text)
			if matches == 0 {
				return
			}
			score := matches * weight
			if componentNormalize(text) == componentNormalize(scenario) {
				score += weight
			}
			candidate.Score += score
			candidate.Evidence = append(candidate.Evidence, recommendationEvidence{Field: field, Snippet: styleSnippet(text, tokens), Score: score})
		}
		addEvidence("name", component.Name, 30)
		for _, alias := range component.AKA {
			addEvidence("alias", alias, 26)
		}
		for _, phrase := range component.Fuzzy {
			addEvidence("fuzzy_phrase", phrase, 22)
		}
		addEvidence("description", component.Description, 7)
		addEvidence("tagline", component.Tagline, 10)
		for _, part := range component.Parts {
			addEvidence("part", part.Name, 18)
			addEvidence("part_id", part.ID, 16)
			addEvidence("part_description", part.Description, 6)
			addEvidence("part_api", part.API, 16)
		}
		for _, api := range componentAPIs(component.API, framework) {
			addEvidence("api_symbol", api.Symbol, 16)
			addEvidence("api_note", api.Note, 6)
		}
		candidate.Score += identifyCompoundSlugBonus(component, tokens)
		if candidate.Score > 0 {
			candidates = append(candidates, candidate)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func upstreamWatchOuts(component namethatui.Component) []string {
	// A watch-out is only useful when it remains attributable. The upstream
	// debug prompt is retained verbatim; no generated advice is mixed in.
	if strings.TrimSpace(component.DebugPrompt) != "" {
		return []string{component.DebugPrompt}
	}
	return []string{}
}

func recommendPrint(cmd *cobra.Command, flags *rootFlags, value any) error {
	if wantsMachineOutput(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), value, flags)
	}
	response, ok := value.(recommendResponse)
	if !ok {
		return componentPrint(cmd, flags, value)
	}
	if response.Choice == nil {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No local component recommendation matched the scenario.")
		return err
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) — score %d\n", response.Choice.Name, response.Choice.Platform, response.Choice.Score); err != nil {
		return err
	}
	for _, evidence := range response.Choice.Evidence {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s\n", evidence.Field, evidence.Snippet); err != nil {
			return err
		}
	}
	if response.Ambiguous {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Close local scores; review the alternatives before choosing.")
		return err
	}
	return nil
}
