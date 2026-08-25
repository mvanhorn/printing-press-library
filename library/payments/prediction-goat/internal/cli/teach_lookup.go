// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// newTeachLookupCmd builds the `teach-lookup` subcommand. The
// command writes a single (kind, canonical, value) row into the
// entity_lookups table with source='taught' (or whatever --source
// the user passes). It exists so users and downstream tooling can
// extend the seeded reference data without waiting for a binary
// release that adds the entry to the seed files.
//
// Unlike `teach`, this command is meant to be human-invoked: errors
// surface on stderr (not a teach.log file), and a usage error
// returns code 2 the way the rest of the human CLI does. The
// silent-on-success behavior matches `teach` because the typical
// invocation is inside a one-liner script that may chain into a
// re-run of a prediction-market query.
//
// See docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md
// section U9 for the broader design context.
func newTeachLookupCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var canonical string
	var value string
	var source string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "teach-lookup",
		Short: "Add a canonical -> value entry to the entity_lookups table (e.g., Curaçao -> CW)",
		Long: `Record a single (kind, canonical, value) mapping so recipe substitution
can resolve the canonical token to the value at query time. Idempotent:
re-running with the same triple is a no-op (PK conflict, silenced).

Use this when a country, team, or other reference entity is missing from
the seeded data, or when you want to override a seeded alias for your own
domain. Example: a Polymarket slug uses "USA" instead of "United-States"
in a particular topic — teach-lookup --kind country_alt --canonical
"United States" --value "USA" lets the recipe engine pick that variant.

Computed kinds (lowercase, uppercase, kebab-case, capitalize-first, slug)
cannot be taught — they are pure string transforms with no table backing.`,
		Example: `  prediction-goat-pp-cli teach-lookup \
    --kind country_iso2 \
    --canonical "Bosnia and Herzegovina" \
    --value BA`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Match `teach`'s noLearn / dry-run gating so disabling
			// the learning subsystem disables manual lookup teaches
			// too. A user who explicitly set NO_LEARN almost
			// certainly does not want a side-channel write surface
			// going around it.
			if flags.noLearn || noLearnEnabled() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}

			kind = strings.TrimSpace(kind)
			canonical = strings.TrimSpace(canonical)
			value = strings.TrimSpace(value)

			if kind == "" {
				return usageErr(errors.New("--kind is required"))
			}
			if canonical == "" {
				return usageErr(errors.New("--canonical is required"))
			}
			if value == "" {
				return usageErr(errors.New("--value is required"))
			}
			if lookups.IsComputedKind(kind) {
				return usageErr(fmt.Errorf("--kind %q is a computed kind (resolved by string transform); not table-backed", kind))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			if err := lookups.Upsert(db.DB(), kind, canonical, value, source); err != nil {
				return fmt.Errorf("upsert lookup: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"recorded":  true,
					"kind":      kind,
					"canonical": canonical,
					"value":     value,
					"source":    source,
				}, flags)
			}
			// Default: silent on success, like teach.
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Lookup kind (e.g. country_iso2, nfl_team_abbrev) — required")
	cmd.Flags().StringVar(&canonical, "canonical", "", "Canonical entity name (e.g. \"Bosnia and Herzegovina\") — required")
	cmd.Flags().StringVar(&value, "value", "", "Resolved value (e.g. \"BA\") — required")
	cmd.Flags().StringVar(&source, "source", "taught", "Provenance tag: seeded | taught | inferred")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}
