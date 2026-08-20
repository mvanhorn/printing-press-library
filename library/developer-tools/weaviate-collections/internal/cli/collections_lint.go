// Copyright 2026 SomSamantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: collections lint.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type lintFinding struct {
	Severity string `json:"severity"`
	Field    string `json:"field"`
	Message  string `json:"message"`
}

// pp:data-source live
func newNovelCollectionsLintCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "lint <className>",
		Short:       "Flag risky collection configs: no vectorizer set, replication factor of 1, unindexed high-cardinality properties.",
		Example:     "  weaviate-collections-pp-cli collections lint Article",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would lint collection config")
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("className is required"))
			}
			className := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			cls, err := fetchOneClass(ctx, flags, className)
			if err != nil {
				return err
			}
			findings := lintCollection(cls)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(findings) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: no issues found\n", className)
					return nil
				}
				for _, f := range findings {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s\n", f.Severity, f.Field, f.Message)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"collection": className,
				"findings":   findings,
				"clean":      len(findings) == 0,
			}, flags)
		},
	}
	return cmd
}

func lintCollection(cls map[string]any) []lintFinding {
	findings := make([]lintFinding, 0)

	vectorizer, _ := cls["vectorizer"].(string)
	_, hasVectorConfig := cls["vectorConfig"]
	if (vectorizer == "" || vectorizer == "none") && !hasVectorConfig {
		findings = append(findings, lintFinding{
			Severity: "warning",
			Field:    "vectorizer",
			Message:  "no vectorizer configured; objects will need vectors supplied manually",
		})
	}

	if rc, ok := cls["replicationConfig"].(map[string]any); ok {
		if factor, ok := rc["factor"].(float64); ok && factor <= 1 {
			findings = append(findings, lintFinding{
				Severity: "warning",
				Field:    "replicationConfig.factor",
				Message:  "replication factor is 1; no fault tolerance against node loss",
			})
		}
	} else {
		findings = append(findings, lintFinding{
			Severity: "info",
			Field:    "replicationConfig",
			Message:  "no replicationConfig set; defaults to factor 1 (no fault tolerance)",
		})
	}

	if props, ok := cls["properties"].([]any); ok {
		for _, p := range props {
			pm, ok := p.(map[string]any)
			if !ok {
				continue
			}
			name, _ := pm["name"].(string)
			dataTypes, _ := pm["dataType"].([]any)
			isText := false
			for _, dt := range dataTypes {
				if s, ok := dt.(string); ok && (s == "text" || s == "text[]") {
					isText = true
				}
			}
			if !isText {
				continue
			}
			filterable, hasFilterable := pm["indexFilterable"].(bool)
			searchable, hasSearchable := pm["indexSearchable"].(bool)
			if hasFilterable && hasSearchable && !filterable && !searchable {
				findings = append(findings, lintFinding{
					Severity: "warning",
					Field:    fmt.Sprintf("properties[%s]", name),
					Message:  "text property has both indexFilterable and indexSearchable disabled; it cannot be filtered or searched",
				})
			}
		}
	}

	if mt, ok := cls["multiTenancyConfig"].(map[string]any); ok {
		if enabled, _ := mt["enabled"].(bool); enabled {
			if autoCreate, _ := mt["autoTenantCreation"].(bool); !autoCreate {
				findings = append(findings, lintFinding{
					Severity: "info",
					Field:    "multiTenancyConfig.autoTenantCreation",
					Message:  "multi-tenancy enabled without autoTenantCreation; writes to unknown tenants will fail",
				})
			}
		}
	}

	return findings
}
