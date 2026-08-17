// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/store"
	"github.com/spf13/cobra"
)

type staleRule struct {
	ObjectID string `json:"objectID"`
	Reason   string `json:"reason"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

type rulesStaleResult struct {
	Index string      `json:"index"`
	Stale []staleRule `json:"stale"`
	Count int         `json:"count"`
}

func newNovelRulesStaleCmd(flags *rootFlags) *cobra.Command {
	var flagIndex string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Find rules that reference attributes missing from the index's searchable attributes or that can never match.",
		Example:     "  algolia-pp-cli rules stale --index algolia_movie_sample_dataset",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rules stale")
			}
			if flagIndex == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--index is required"))
			}
			if flagDB == "" {
				flagDB = defaultDBPath("algolia-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: algolia-pp-cli sync --resources rules to populate the local database.\n", flagDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), rulesStaleResult{Index: flagIndex, Stale: make([]staleRule, 0)}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), flagDB)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "rules") {
				hintIfStale(cmd, db, "rules", flags.maxAge)
			}

			settingsRaw, _ := db.Get("indexes", flagIndex)
			if len(settingsRaw) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: settings for index %q not in local store; run 'algolia-pp-cli sync --resources indexes' before auditing rules\n", flagIndex)
				res := rulesStaleResult{Index: flagIndex, Stale: make([]staleRule, 0)}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Cannot audit rules for %q without synced settings.\n", flagIndex)
				return nil
			}
			settings := unwrapSettingsObject(settingsRaw)
			searchable := stringSliceField(settings, "searchableAttributes")
			facetable := stringSliceField(settings, "attributesForFaceting")

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT data FROM resources WHERE resource_type = 'rules'`)
			if err != nil {
				return fmt.Errorf("querying rules: %w", err)
			}
			var ruleData []json.RawMessage
			for rows.Next() {
				var d string
				if err := rows.Scan(&d); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan rule: %w", err)
				}
				ruleData = append(ruleData, json.RawMessage(d))
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate rules: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close rules: %w", err)
			}

			stale := make([]staleRule, 0)
			for _, raw := range ruleData {
				var rule struct {
					ObjectID   string `json:"objectID"`
					IndexName  string `json:"indexName"`
					Enabled    *bool  `json:"enabled"`
					Conditions []struct {
						Pattern   string `json:"pattern"`
						Context   string `json:"context"`
						Anchoring string `json:"anchoring"`
					} `json:"conditions"`
					Consequence map[string]any `json:"consequence"`
				}
				if json.Unmarshal(raw, &rule) != nil {
					continue
				}
				if rule.IndexName != "" && rule.IndexName != flagIndex {
					continue
				}
				if rule.ObjectID == "" {
					continue
				}
				if rule.Enabled != nil && !*rule.Enabled {
					stale = append(stale, staleRule{ObjectID: rule.ObjectID, Reason: "rule is disabled", Enabled: rule.Enabled})
					continue
				}
				attrs := ruleAttributeRefs(&rule)
				for _, a := range attrs {
					if !containsString(searchable, a) && !containsString(facetable, a) {
						stale = append(stale, staleRule{ObjectID: rule.ObjectID, Reason: fmt.Sprintf("references attribute %q not in searchableAttributes or attributesForFaceting", a), Enabled: rule.Enabled})
						break
					}
				}
			}

			res := rulesStaleResult{Index: flagIndex, Stale: stale, Count: len(stale)}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(stale) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stale rules found for index %q.\n", flagIndex)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d stale rule(s) for index %q:\n", len(stale), flagIndex)
			for _, s := range stale {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — %s\n", s.ObjectID, s.Reason)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagIndex, "index", "", "Index name whose rules to audit")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// ruleAttributeRefs extracts attribute NAMES referenced by a rule's
// consequences — the only rule surface that can reference searchable or
// faceted attributes. Condition patterns and contexts are query text, not
// attribute names; promoted objectIDs are record IDs, not attributes.
func ruleAttributeRefs(rule *struct {
	ObjectID   string `json:"objectID"`
	IndexName  string `json:"indexName"`
	Enabled    *bool  `json:"enabled"`
	Conditions []struct {
		Pattern   string `json:"pattern"`
		Context   string `json:"context"`
		Anchoring string `json:"anchoring"`
	} `json:"conditions"`
	Consequence map[string]any `json:"consequence"`
}) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	// Filter expressions are the canonical attribute references:
	// "genre:scifi" references the genre attribute. Extract the key
	// before the first ':' in each filter operand.
	collectFilters := func(filters any) {
		switch f := filters.(type) {
		case string:
			add(attributeFromFilter(f))
		case []any:
			for _, item := range f {
				if s, ok := item.(string); ok {
					add(attributeFromFilter(s))
				}
			}
		}
	}
	collectFilters(rule.Consequence["filter"])
	collectFilters(rule.Consequence["filters"])
	if cons, ok := rule.Consequence["promote"].([]any); ok {
		for _, p := range cons {
			if pm, ok := p.(map[string]any); ok {
				// promote objects carry an explicit attribute name
				if attr, ok := pm["attribute"].(string); ok {
					add(attr)
				}
			}
		}
	}
	if cons, ok := rule.Consequence["hide"].([]any); ok {
		for _, h := range cons {
			if hm, ok := h.(map[string]any); ok {
				if attr, ok := hm["attribute"].(string); ok {
					add(attr)
				}
			}
		}
	}
	if cons, ok := rule.Consequence["filterPromotes"].([]any); ok {
		for _, fp := range cons {
			if fpm, ok := fp.(map[string]any); ok {
				collectFilters(fpm["filter"])
			}
		}
	}
	return out
}

// attributeFromFilter extracts the attribute name from a filter operand:
// "genre:scifi" -> "genre", "brand:X" -> "brand". Operands without a
// colon (bare terms) are not attribute references.
func attributeFromFilter(filter string) string {
	// Strip negation/score prefixes: "-genre:scifi", "_rating>=4", "rating:>=4"
	s := filter
	for len(s) > 0 && (s[0] == '-' || s[0] == '_' || s[0] == ' ') {
		s = s[1:]
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ':' {
			return s[:i]
		}
		// Numeric/range operators end the attribute name too
		if c == '>' || c == '<' || c == '=' || c == ' ' {
			return s[:i]
		}
	}
	return ""
}

func stringSliceField(m map[string]any, key string) []string {
	var out []string
	if raw, ok := m[key].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func containsString(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}
