// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/spf13/cobra"
)

// pp:data-source hybrid
// Identify is a markerless extension: the synced component mirror produces a
// deterministic candidate set, which the public semantic endpoint may refine.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		// A root persistent hook normally initializes the optional learning store.
		// Dry runs must not touch a database, so bypass that hook only for this
		// command's dry-run path while preserving the root behavior elsewhere.
		rootPreRun := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if cmd.Name() != "identify" || !flags.dryRun {
				return rootPreRun(cmd, args)
			}
			if flags.agent {
				flags.asJSON = true
				flags.compact = true
				flags.noInput = true
				flags.yes = true
				noColor = true
			}
			switch flags.dataSource {
			case "auto", "live", "local":
				return nil
			default:
				return usageErr(fmt.Errorf("invalid --data-source value %q: must be auto, live, or local", flags.dataSource))
			}
		}
		root.AddCommand(newIdentifyCmd(flags))
	})
}

type identifyMeta struct {
	DataSource     string `json:"data_source"`
	SyncedAt       string `json:"synced_at,omitempty"`
	Stale          bool   `json:"stale"`
	Reason         string `json:"reason,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

type identifyCandidate struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Platform       string               `json:"platform"`
	Component      namethatui.Component `json:"component"`
	Part           *namethatui.Part     `json:"part"`
	Score          int                  `json:"score"`
	Reasons        []string             `json:"reasons"`
	HighConfidence bool                 `json:"high_confidence"`
	SourceURL      string               `json:"source_url"`
}

func newIdentifyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var localOnly bool
	cmd := &cobra.Command{
		Use:         "identify <vague-description>",
		Short:       "Identify a UI component or part from a vague description",
		Example:     `  name-that-ui-pp-cli identify "searchable dropdown" --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if isPrintingPressInvalidFixture(args) {
				return usageErr(fmt.Errorf("invalid description fixture"))
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("%s requires <vague-description>", cmd.CommandPath()))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			path := componentDBPath(dbPath)
			if dryRunOK(flags) {
				return componentPrint(cmd, flags, map[string]any{
					"description": args[0], "dry_run": true, "db_path": path, "sqlite_opened": false,
					"meta": identifyMeta{DataSource: "hybrid", Reason: "dry_run"},
					"stages": []any{
						map[string]any{"name": "retrieve", "method": "POST", "path": "/api/search", "body": map[string]any{"q": args[0], "mode": "retrieve", "local": []string{}}},
						map[string]any{"name": "resolve", "method": "POST", "path": "/api/search", "body": map[string]any{"q": args[0], "mode": "resolve", "candidates": []string{}}},
					},
				})
			}
			// Live mode is intentionally useful on a fresh install: semantic
			// retrieval accepts an empty local candidate list, so do not require
			// or open SQLite before making the public API calls.
			if flags.dataSource == "live" && !localOnly {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				retrieve, _, err := c.PostQueryWithParams(cmd.Context(), "/api/search", nil, map[string]any{"q": args[0], "mode": "retrieve", "local": []string{}})
				if err == nil {
					resolve, _, resolveErr := c.PostQueryWithParams(cmd.Context(), "/api/search", nil, map[string]any{"q": args[0], "mode": "resolve", "candidates": identifyResponseIDs(retrieve)})
					if resolveErr == nil {
						return identifyRemotePrint(cmd, flags, args[0], resolve, nil, identifyMeta{DataSource: "live"})
					}
					err = resolveErr
				}
				return classifyAPIError(err, flags)
			}

			items, localMeta, err := componentLoad(cmd, path)
			if err != nil {
				return err
			}
			local := identifyLocalCandidates(items, args[0], limit)
			meta := identifyMeta{DataSource: "local", SyncedAt: localMeta.SyncedAt, Stale: localMeta.Stale}
			if localOnly {
				meta.Reason = "local_only"
				return identifyPrint(cmd, flags, args[0], local, identifyLocalAmbiguous(local), "", meta)
			}
			if flags.dataSource == "local" {
				meta.Reason = "user_requested"
				return identifyPrint(cmd, flags, args[0], local, identifyLocalAmbiguous(local), "", meta)
			}

			candidateIDs := identifyIDs(local)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			retrieve, _, err := c.PostQueryWithParams(cmd.Context(), "/api/search", nil, map[string]any{"q": args[0], "mode": "retrieve", "local": candidateIDs})
			if err == nil {
				resolvedIDs := identifyResponseIDs(retrieve)
				resolve, _, resolveErr := c.PostQueryWithParams(cmd.Context(), "/api/search", nil, map[string]any{"q": args[0], "mode": "resolve", "candidates": resolvedIDs})
				if resolveErr == nil {
					return identifyRemotePrint(cmd, flags, args[0], resolve, items, identifyMeta{DataSource: "live", SyncedAt: localMeta.SyncedAt, Stale: localMeta.Stale})
				}
				err = resolveErr
			}
			if flags.dataSource == "live" {
				return classifyAPIError(err, flags)
			}
			meta.Reason = "live_error"
			meta.FallbackReason = err.Error()
			return identifyPrint(cmd, flags, args[0], local, identifyLocalAmbiguous(local), "", meta)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&limit, "limit", 8, "Maximum local candidates")
	cmd.Flags().BoolVar(&localOnly, "local-only", false, "Use only the locally synced component mirror")
	return cmd
}

func identifyLocalCandidates(items []namethatui.Component, description string, limit int) []identifyCandidate {
	tokens := identifyQueryTokens(description)
	if len(tokens) == 0 {
		return []identifyCandidate{}
	}
	results := make([]identifyCandidate, 0)
	for _, component := range items {
		if candidate := identifyRank(component, nil, tokens, description); candidate.Score > 0 {
			results = append(results, candidate)
		}
		for i := range component.Parts {
			part := component.Parts[i]
			if candidate := identifyRank(component, &part, tokens, description); candidate.Score > 0 {
				results = append(results, candidate)
			}
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].ID < results[j].ID
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func identifyRank(component namethatui.Component, part *namethatui.Part, tokens []string, description string) identifyCandidate {
	id := component.ID
	url := component.SourceURL
	if part != nil {
		id += "#" + part.ID
		url += "#" + part.ID
	}
	candidate := identifyCandidate{ID: id, Name: component.Name, Platform: component.Platform, Component: component, Part: part, Reasons: []string{}, SourceURL: url}
	add := func(field, value string, weight int) {
		matches := identifyTokenMatches(tokens, value)
		if matches == 0 {
			return
		}
		score := matches * weight
		fieldTokens := styleTokens(value)
		// A multi-word description should not be dominated by a single,
		// generic token from an otherwise unrelated component. Whole aliases
		// such as "combo box" receive a deterministic bonus, while an
		// isolated token receives only a small signal.
		if len(tokens) > 1 && len(fieldTokens) == 1 {
			score = max(1, score/4)
		}
		if len(fieldTokens) > 1 && matches == len(fieldTokens) && strings.Contains(componentNormalize(description), componentNormalize(value)) {
			score += weight * 3
		}
		if componentNormalize(value) == componentNormalize(description) {
			score += weight
		}
		candidate.Score += score
		candidate.Reasons = append(candidate.Reasons, field)
	}
	if part == nil {
		add("name", component.Name, 28)
		for _, alias := range component.AKA {
			add("alias", alias, 24)
		}
		for _, phrase := range component.Fuzzy {
			add("fuzzy_phrase", phrase, 20)
		}
		add("description", component.Description, 6)
		add("tagline", component.Tagline, 8)
		for _, api := range component.API {
			add("api_symbol", api.Symbol, 16)
		}
		candidate.Score += identifyCompoundSlugBonus(component, tokens)
	} else {
		add("part", part.Name, 22)
		add("part_id", part.ID, 18)
		add("part_description", part.Description, 6)
		add("part_api", part.API, 16)
	}
	candidate.HighConfidence = candidate.Score >= 30 && (containsReason(candidate.Reasons, "name") || containsReason(candidate.Reasons, "alias") || containsReason(candidate.Reasons, "fuzzy_phrase") || containsReason(candidate.Reasons, "part") || containsReason(candidate.Reasons, "part_id") || containsReason(candidate.Reasons, "api_symbol") || containsReason(candidate.Reasons, "part_api"))
	return candidate
}

// identifyQueryTokens adds joined adjacent words alongside the ordinary token
// stream. Public records commonly spell a canonical slug as one word
// ("combobox") while people write it as two ("combo box"). Keeping both
// representations lets the ranker reward the canonical slug without losing
// the individual-word evidence used for other UI descriptions.
func identifyQueryTokens(description string) []string {
	tokens := styleTokens(description)
	seen := make(map[string]struct{}, len(tokens)*2)
	for _, token := range tokens {
		seen[token] = struct{}{}
	}
	// Only join adjacent words from the original query. Iterating to the
	// growing slice length would recursively join the synthetic tokens we append
	// below and never terminate for any multi-word description.
	originalLen := len(tokens)
	for i := 0; i+1 < originalLen; i++ {
		joined := tokens[i] + tokens[i+1]
		if len(joined) < 5 {
			continue
		}
		if _, exists := seen[joined]; exists {
			continue
		}
		seen[joined] = struct{}{}
		tokens = append(tokens, joined)
	}
	return tokens
}

func identifyCompoundSlugBonus(component namethatui.Component, tokens []string) int {
	slug := componentNormalize(component.Slug)
	if slug == "" {
		_, slug, _ = strings.Cut(component.ID, "/")
		slug = componentNormalize(slug)
	}
	if slug == "" {
		return 0
	}
	for _, token := range tokens {
		if token == slug && len(token) >= 5 {
			return 180
		}
	}
	return 0
}

func identifyTokenMatches(tokens []string, text string) int {
	fields := map[string]struct{}{}
	for _, token := range styleTokens(text) {
		fields[token] = struct{}{}
	}
	matches := 0
	for _, token := range tokens {
		if _, ok := fields[token]; ok {
			matches++
		}
	}
	return matches
}

func containsReason(reasons []string, target string) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}

func identifyIDs(candidates []identifyCandidate) []string {
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.ID)
	}
	return ids
}

func identifyLocalAmbiguous(candidates []identifyCandidate) bool {
	if len(candidates) < 2 || !candidates[0].HighConfidence || !candidates[1].HighConfidence {
		return false
	}
	return candidates[0].Score-candidates[1].Score <= 8
}

func identifyResponseIDs(data json.RawMessage) []string {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return []string{}
	}
	seen := map[string]struct{}{}
	ids := []string{}
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			if id, ok := v["id"].(string); ok && id != "" {
				if _, exists := seen[id]; !exists {
					seen[id] = struct{}{}
					ids = append(ids, id)
				}
			}
			for _, key := range []string{"results", "candidates", "data", "items"} {
				if nested, ok := v[key]; ok {
					walk(nested)
				}
			}
		}
	}
	walk(value)
	return ids
}

func identifyRemotePrint(cmd *cobra.Command, flags *rootFlags, description string, raw json.RawMessage, items []namethatui.Component, meta identifyMeta) error {
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decode semantic resolve response: %w", err)
	}
	byID := identifyComponentIndex(items)
	switch value := result.(type) {
	case map[string]any:
		for _, key := range []string{"results", "candidates", "data", "items"} {
			if candidates, ok := value[key].([]any); ok {
				value[key] = identifyEnrichRemote(candidates, byID)
			}
		}
		value["description"] = description
		value["meta"] = meta
		return componentPrintWithAgentSource(cmd, flags, value, meta.DataSource)
	case []any:
		return componentPrintWithAgentSource(cmd, flags, map[string]any{"description": description, "results": identifyEnrichRemote(value, byID), "ambiguous": false, "clarification": "", "meta": meta}, meta.DataSource)
	default:
		return componentPrintWithAgentSource(cmd, flags, map[string]any{"description": description, "results": []any{}, "remote": result, "ambiguous": false, "clarification": "", "meta": meta}, meta.DataSource)
	}
}

func identifyComponentIndex(items []namethatui.Component) map[string]namethatui.Component {
	indexed := make(map[string]namethatui.Component, len(items))
	for _, component := range items {
		indexed[component.ID] = component
	}
	return indexed
}

func identifyEnrichRemote(candidates []any, components map[string]namethatui.Component) []any {
	enriched := make([]any, 0, len(candidates))
	for _, raw := range candidates {
		candidate, ok := raw.(map[string]any)
		if !ok {
			enriched = append(enriched, raw)
			continue
		}
		enriched = append(enriched, identifyNormalizeRemoteCandidate(candidate, components))
	}
	return enriched
}

// identifyNormalizeRemoteCandidate gives live semantic results the same
// stable top-level candidate paths as local results. The semantic endpoint
// returns IDs, not component descriptions; when a fresh install has no local
// mirror, derive only the canonical NameThatUI URL and ID-addressable metadata.
func identifyNormalizeRemoteCandidate(candidate map[string]any, components map[string]namethatui.Component) map[string]any {
	id, _ := candidate["id"].(string)
	componentID, partID, hasPart := strings.Cut(id, "#")
	candidate["part"] = nil
	if component, found := components[componentID]; found {
		candidate["component"] = component
		candidate["name"] = component.Name
		candidate["platform"] = component.Platform
		candidate["source_url"] = component.SourceURL
		if hasPart {
			for i := range component.Parts {
				if component.Parts[i].ID == partID {
					candidate["part"] = component.Parts[i]
					candidate["source_url"] = component.SourceURL + "#" + partID
					break
				}
			}
		}
		return candidate
	}

	platform, slug, ok := identifyCanonicalID(componentID)
	if !ok {
		candidate["name"] = ""
		candidate["platform"] = ""
		candidate["source_url"] = ""
		return candidate
	}
	if name, _ := candidate["name"].(string); strings.TrimSpace(name) == "" {
		candidate["name"] = identifyDisplayName(slug)
	}
	candidate["platform"] = platform
	candidate["source_url"] = "https://namethatui.com/" + platform + "/" + slug
	if hasPart && identifyCanonicalSegment(partID) {
		candidate["part"] = map[string]any{"id": partID}
		candidate["source_url"] = candidate["source_url"].(string) + "#" + partID
	}
	return candidate
}

func identifyCanonicalID(id string) (platform, slug string, ok bool) {
	platform, slug, found := strings.Cut(id, "/")
	return platform, slug, found && identifyCanonicalSegment(platform) && identifyCanonicalSegment(slug)
}

func identifyCanonicalSegment(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !(unicode.IsLower(r) || unicode.IsDigit(r) || r == '-') {
			return false
		}
	}
	return true
}

func identifyDisplayName(slug string) string {
	words := strings.FieldsFunc(slug, func(r rune) bool { return r == '-' })
	for i := range words {
		if words[i] != "" {
			words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
		}
	}
	return strings.Join(words, " ")
}

func identifyPrint(cmd *cobra.Command, flags *rootFlags, description string, candidates []identifyCandidate, ambiguous bool, clarification string, meta identifyMeta) error {
	return componentPrint(cmd, flags, map[string]any{
		"description":   description,
		"results":       candidates,
		"ambiguous":     ambiguous,
		"clarification": clarification,
		"meta":          meta,
	})
}
