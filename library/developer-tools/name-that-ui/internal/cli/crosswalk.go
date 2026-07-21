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
// Crosswalks only translate terminology present in the local component mirror.
func newNovelCrosswalkCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "crosswalk <concept>",
		Short:       "See one UI concept across plain language, component parts, AppKit, SwiftUI, and ARIA or HTML terminology.",
		Example:     "name-that-ui-pp-cli crosswalk \"menu bar extra\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("%s requires <concept>", cmd.CommandPath()))
			}
			if args[0] == "__printing_press_invalid__" {
				return usageErr(fmt.Errorf("invalid concept fixture"))
			}
			if dryRunOK(flags) {
				return crosswalkPrint(cmd, flags, map[string]any{
					"dry_run":       true,
					"operation":     "crosswalk",
					"concept":       args[0],
					"data_source":   "local",
					"db_path":       componentDBPath(dbPath),
					"sqlite_opened": false,
					"candidates":    []crosswalkCandidate{},
					"matrix":        []crosswalkMatrixRow{},
					"source_urls":   []string{},
				})
			}
			items, meta, err := componentLoad(cmd, componentDBPath(dbPath))
			if err != nil {
				return err
			}
			candidates := crosswalkCandidates(items, args[0])
			matrix := make([]crosswalkMatrixRow, 0, len(candidates))
			urls := make([]string, 0, len(candidates))
			for _, candidate := range candidates {
				matrix = append(matrix, crosswalkMatrix(candidate))
				urls = append(urls, candidate.SourceURLs...)
			}
			return crosswalkPrint(cmd, flags, map[string]any{
				"concept":     args[0],
				"ambiguous":   crosswalkAmbiguous(candidates),
				"candidates":  candidates,
				"matrix":      matrix,
				"source_urls": crosswalkURLs(urls),
				"provenance":  map[string]any{"data_source": "local", "component": meta},
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

type crosswalkCandidate struct {
	Score         int                 `json:"score"`
	Evidence      []crosswalkEvidence `json:"evidence"`
	Component     map[string]string   `json:"component"`
	Part          *crosswalkPart      `json:"part,omitempty"`
	PlainLanguage []string            `json:"plain_language"`
	APIs          []namethatui.API    `json:"apis"`
	SourceURLs    []string            `json:"source_urls"`
}

type crosswalkEvidence struct {
	Field string `json:"field"`
	Value string `json:"value"`
	Score int    `json:"score"`
}

type crosswalkPart struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	API         string `json:"api"`
	Description string `json:"description"`
}

type crosswalkFrameworkTerms struct {
	Framework string   `json:"framework"`
	Terms     []string `json:"terms"`
}

type crosswalkMatrixRow struct {
	PlainLanguage      []string                  `json:"plain_language"`
	CanonicalComponent string                    `json:"canonical_component"`
	CanonicalPart      string                    `json:"canonical_part"`
	AppKit             []string                  `json:"appkit"`
	SwiftUI            []string                  `json:"swiftui"`
	ARIA               []string                  `json:"aria"`
	HTML               []string                  `json:"html"`
	Other              []crosswalkFrameworkTerms `json:"other"`
	SourceURLs         []string                  `json:"source_urls"`
}

func crosswalkCandidates(items []namethatui.Component, concept string) []crosswalkCandidate {
	result := make([]crosswalkCandidate, 0)
	for _, component := range items {
		componentEvidence := crosswalkComponentEvidence(component, concept)
		if len(componentEvidence) > 0 {
			result = append(result, newCrosswalkCandidate(component, nil, componentEvidence))
		}
		for index := range component.Parts {
			part := &component.Parts[index]
			evidence := crosswalkPartEvidence(*part, concept)
			if len(evidence) > 0 {
				result = append(result, newCrosswalkCandidate(component, part, evidence))
			}
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		if result[i].Component["id"] != result[j].Component["id"] {
			return result[i].Component["id"] < result[j].Component["id"]
		}
		return crosswalkCandidatePartName(result[i]) < crosswalkCandidatePartName(result[j])
	})
	return result
}

func newCrosswalkCandidate(component namethatui.Component, part *namethatui.Part, evidence []crosswalkEvidence) crosswalkCandidate {
	score := 0
	for _, item := range evidence {
		if item.Score > score {
			score = item.Score
		}
	}
	candidate := crosswalkCandidate{
		Score:         score,
		Evidence:      evidence,
		Component:     map[string]string{"id": component.ID, "platform": component.Platform, "slug": component.Slug, "name": component.Name},
		PlainLanguage: crosswalkURLs(append(append([]string{component.Name}, component.AKA...), component.Fuzzy...)),
		APIs:          nonNilAPIs(component.API),
		SourceURLs:    crosswalkURLs([]string{component.SourceURL}),
	}
	if part != nil {
		candidate.Part = &crosswalkPart{ID: part.ID, Name: part.Name, API: part.API, Description: part.Description}
		candidate.PlainLanguage = crosswalkURLs(append(candidate.PlainLanguage, part.Name))
	}
	return candidate
}

func crosswalkComponentEvidence(component namethatui.Component, concept string) []crosswalkEvidence {
	evidence := make([]crosswalkEvidence, 0)
	appendCrosswalkEvidence(&evidence, "component_name", component.Name, concept, 100, 55)
	appendCrosswalkEvidence(&evidence, "component_slug", component.Slug, concept, 96, 50)
	appendCrosswalkEvidence(&evidence, "component_id", component.ID, concept, 96, 50)
	for _, alias := range component.AKA {
		appendCrosswalkEvidence(&evidence, "component_alias", alias, concept, 90, 45)
	}
	for _, fuzzy := range component.Fuzzy {
		appendCrosswalkEvidence(&evidence, "component_fuzzy", fuzzy, concept, 80, 40)
	}
	for _, api := range component.API {
		appendCrosswalkEvidence(&evidence, "api_framework", api.Framework, concept, 60, 30)
		appendCrosswalkEvidence(&evidence, "api_symbol", api.Symbol, concept, 85, 45)
	}
	return evidence
}

func crosswalkPartEvidence(part namethatui.Part, concept string) []crosswalkEvidence {
	evidence := make([]crosswalkEvidence, 0)
	appendCrosswalkEvidence(&evidence, "part_name", part.Name, concept, 88, 46)
	appendCrosswalkEvidence(&evidence, "part_api", part.API, concept, 82, 42)
	return evidence
}

func appendCrosswalkEvidence(destination *[]crosswalkEvidence, field, value, concept string, exactScore, fuzzyScore int) {
	value = strings.TrimSpace(value)
	query := componentNormalize(concept)
	normalized := componentNormalize(value)
	if query == "" || normalized == "" {
		return
	}
	if query == normalized {
		*destination = append(*destination, crosswalkEvidence{Field: field, Value: value, Score: exactScore})
		return
	}
	if len(query) >= 3 && (strings.Contains(normalized, query) || strings.Contains(query, normalized)) {
		*destination = append(*destination, crosswalkEvidence{Field: field, Value: value, Score: fuzzyScore})
	}
}

func crosswalkMatrix(candidate crosswalkCandidate) crosswalkMatrixRow {
	row := crosswalkMatrixRow{
		PlainLanguage:      crosswalkURLs(candidate.PlainLanguage),
		CanonicalComponent: candidate.Component["name"],
		CanonicalPart:      crosswalkCandidatePartName(candidate),
		AppKit:             []string{},
		SwiftUI:            []string{},
		ARIA:               []string{},
		HTML:               []string{},
		Other:              []crosswalkFrameworkTerms{},
		SourceURLs:         crosswalkURLs(candidate.SourceURLs),
	}
	other := map[string][]string{}
	for _, api := range candidate.APIs {
		framework := strings.TrimSpace(api.Framework)
		if framework == "" || strings.TrimSpace(api.Symbol) == "" {
			continue
		}
		switch strings.ToLower(framework) {
		case "appkit":
			row.AppKit = append(row.AppKit, api.Symbol)
		case "swiftui":
			row.SwiftUI = append(row.SwiftUI, api.Symbol)
		case "aria":
			row.ARIA = append(row.ARIA, api.Symbol)
		case "html":
			row.HTML = append(row.HTML, api.Symbol)
		default:
			other[framework] = append(other[framework], api.Symbol)
		}
	}
	row.AppKit = crosswalkURLs(row.AppKit)
	row.SwiftUI = crosswalkURLs(row.SwiftUI)
	row.ARIA = crosswalkURLs(row.ARIA)
	row.HTML = crosswalkURLs(row.HTML)
	frameworks := make([]string, 0, len(other))
	for framework := range other {
		frameworks = append(frameworks, framework)
	}
	sort.Strings(frameworks)
	for _, framework := range frameworks {
		row.Other = append(row.Other, crosswalkFrameworkTerms{Framework: framework, Terms: crosswalkURLs(other[framework])})
	}
	return row
}

func crosswalkCandidatePartName(candidate crosswalkCandidate) string {
	if candidate.Part == nil {
		return ""
	}
	return candidate.Part.Name
}

func crosswalkAmbiguous(candidates []crosswalkCandidate) bool {
	return len(candidates) > 1 && candidates[0].Score == candidates[1].Score
}

func crosswalkURLs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			if _, exists := seen[value]; !exists {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	sort.Strings(result)
	return result
}

func crosswalkPrint(cmd *cobra.Command, flags *rootFlags, value any) error {
	return componentPrint(cmd, flags, value)
}
