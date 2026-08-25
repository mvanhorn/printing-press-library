// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/recipes"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// newTeachRecipeCmd builds the `teach-recipe` subcommand. The
// command writes a single row into search_recipes with
// source='taught'. It coexists with the implicit Extract pipeline
// that fires from `teach` after every successful learning row: this
// command is the explicit, template-author surface for users who
// have a pattern in mind that the implicit pass hasn't (or can't)
// derive from existing teaches.
//
// Typical use case: a user wants to teach the recipe shape BEFORE
// any concrete (Portugal, USA) pairs land in search_learnings, or
// wants to override the implicit inference's strategy choice.
//
// Errors surface to stderr as code-2 usage errors; the silent-on-
// success default matches `teach` and `teach-lookup`.
//
// See docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md
// section U10 for design context.
func newTeachRecipeCmd(flags *rootFlags) *cobra.Command {
	var queryTemplate string
	var resourceTemplate string
	var resourceType string
	var venue string
	var strategy string
	var entityKind string
	var dbPath string
	var exampleQuery string
	var exampleResource string

	cmd := &cobra.Command{
		Use:   "teach-recipe",
		Short: "Author a search_recipes template directly (e.g., world-cup country tickers)",
		Long: `Record a single (query_template, resource_template, strategy) recipe
so the recall layer can substitute new entities into the template at
query time. Idempotent: re-running with the same triple bumps
confidence on the existing row rather than spawning a duplicate.

Use this when you have a templatable pattern in mind that the
implicit recipe extractor hasn't surfaced yet (e.g., before any
concrete teaches exist for the pattern), or when you want to fix
the entity_kind binding that the implicit pass chose.

Strategy values:
  substitute                      Full deterministic ID after substitution.
  substitute-then-search-prefix   Substituted candidate is a prefix; Apply
                                  does a LIKE search in the resources table.

Entity kinds can be table-backed (country_iso2, nfl_team_abbrev, ...)
or computed (lowercase, uppercase, kebab-case, capitalize-first, slug).
List table-backed kinds via 'prediction-goat-pp-cli teach-lookup' help
or by querying entity_lookups directly.`,
		Example: `  prediction-goat-pp-cli teach-recipe \
    --query-template "{entity} cup wins world" \
    --resource-template "KXMENWORLDCUP-26-{entity:country_iso2}" \
    --resource-type kalshi_markets \
    --venue kalshi \
    --strategy substitute \
    --entity-kind country_iso2`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.noLearn || noLearnEnabled() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}

			queryTemplate = strings.TrimSpace(queryTemplate)
			resourceTemplate = strings.TrimSpace(resourceTemplate)
			resourceType = strings.TrimSpace(resourceType)
			strategy = strings.TrimSpace(strategy)
			entityKind = strings.TrimSpace(entityKind)

			if queryTemplate == "" {
				return usageErr(errors.New("--query-template is required"))
			}
			if resourceTemplate == "" {
				return usageErr(errors.New("--resource-template is required"))
			}
			if resourceType == "" {
				return usageErr(errors.New("--resource-type is required"))
			}
			if strategy == "" {
				return usageErr(errors.New("--strategy is required"))
			}
			if entityKind == "" {
				return usageErr(errors.New("--entity-kind is required"))
			}
			switch strategy {
			case recipes.StrategySubstitute, recipes.StrategySubstituteThenSearchPrefix:
			default:
				return usageErr(fmt.Errorf("invalid --strategy %q: must be %q or %q",
					strategy, recipes.StrategySubstitute, recipes.StrategySubstituteThenSearchPrefix))
			}
			// Validate the entity_kind exists either as a computed
			// kind or has at least one row in entity_lookups. This
			// catches a typo like --entity-kind country_iso23 before
			// it lands as a never-resolvable recipe.
			if !lookups.IsComputedKind(entityKind) {
				if dbPath == "" {
					dbPath = defaultDBPath("prediction-goat-pp-cli")
				}
				db, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("open store: %w", err)
				}
				defer db.Close()

				var count int
				if err := db.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups WHERE kind = ?`, entityKind).Scan(&count); err != nil {
					return fmt.Errorf("validate entity_kind: %w", err)
				}
				if count == 0 {
					return usageErr(fmt.Errorf("--entity-kind %q has no rows in entity_lookups and is not a computed kind", entityKind))
				}

				if _, _, err := recipes.Upsert(db.DB(), recipes.Recipe{
					QueryTemplate:    queryTemplate,
					ResourceTemplate: resourceTemplate,
					ResourceType:     resourceType,
					Venue:            venue,
					Strategy:         strategy,
					EntityKind:       entityKind,
					Source:           recipes.SourceTaught,
					ExampleQuery:     exampleQuery,
					ExampleResource:  exampleResource,
				}); err != nil {
					return fmt.Errorf("upsert recipe: %w", err)
				}
				return emitTeachRecipeResult(cmd, flags, queryTemplate, resourceTemplate, strategy, entityKind)
			}
			// Computed-kind branch: open DB only after validation,
			// keeping the validation-error path free of DB side
			// effects.
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			if _, _, err := recipes.Upsert(db.DB(), recipes.Recipe{
				QueryTemplate:    queryTemplate,
				ResourceTemplate: resourceTemplate,
				ResourceType:     resourceType,
				Venue:            venue,
				Strategy:         strategy,
				EntityKind:       entityKind,
				Source:           recipes.SourceTaught,
				ExampleQuery:     exampleQuery,
				ExampleResource:  exampleResource,
			}); err != nil {
				return fmt.Errorf("upsert recipe: %w", err)
			}
			return emitTeachRecipeResult(cmd, flags, queryTemplate, resourceTemplate, strategy, entityKind)
		},
	}
	cmd.Flags().StringVar(&queryTemplate, "query-template", "", "Query template with {entity} slot (e.g. \"{entity} cup wins world\") — required")
	cmd.Flags().StringVar(&resourceTemplate, "resource-template", "", "Resource template with {entity:kind} slot (e.g. \"KXMENWORLDCUP-26-{entity:country_iso2}\") — required")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "Resource type (e.g. kalshi_markets, markets) — required")
	cmd.Flags().StringVar(&venue, "venue", "", "Venue scope: polymarket | kalshi")
	cmd.Flags().StringVar(&strategy, "strategy", "", "substitute | substitute-then-search-prefix — required")
	cmd.Flags().StringVar(&entityKind, "entity-kind", "", "Lookup kind for substitution (country_iso2, lowercase, ...) — required")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	cmd.Flags().StringVar(&exampleQuery, "example-query", "", "Optional example query for diagnostics")
	cmd.Flags().StringVar(&exampleResource, "example-resource", "", "Optional example resource ID for diagnostics")
	return cmd
}

// emitTeachRecipeResult writes the JSON envelope (when --json is
// set) or stays silent. Pulled into a helper because both the
// computed-kind and table-backed-kind branches above share the same
// success shape.
func emitTeachRecipeResult(cmd *cobra.Command, flags *rootFlags, queryTemplate, resourceTemplate, strategy, entityKind string) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"recorded":          true,
			"query_template":    queryTemplate,
			"resource_template": resourceTemplate,
			"strategy":          strategy,
			"entity_kind":       entityKind,
		}, flags)
	}
	return nil
}
