// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — `location resolve` primitive
// exposing the resolver as an MCP tool. Agents can resolve a free-
// form location string up-front, see the typed GeoContext or
// disambiguation envelope, then pass --location to subsequent
// commands with confidence. R8.

import (
	"github.com/spf13/cobra"
)

// newLocationCmd builds the `location` command tree. v1 ships only
// the `resolve` sub-command; `list`, `set`, `unset`, `current` are
// deferred per Scope Boundaries.
func newLocationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "location",
		Short: "Location resolver and disambiguation primitives",
		Long: "Resolve free-form location strings into a typed GeoContext.\n\n" +
			"When the input is ambiguous (e.g., bare 'bellevue' matches WA/NE/KY),\n" +
			"the resolver returns a structured disambiguation envelope instead of\n" +
			"silently picking one. Agents calling this CLI should consult their\n" +
			"conversation context to disambiguate before re-running with a more\n" +
			"specific input.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newLocationResolveCmd(flags))
	return cmd
}

// newLocationResolveCmd is the agent-facing primitive: resolve a
// free-form location string into either a typed GeoContext (HIGH/
// MEDIUM tier — agent uses the centroid + radius downstream) or a
// disambiguation envelope (LOW tier — agent disambiguates from
// conversation context or asks the user).
func newLocationResolveCmd(flags *rootFlags) *cobra.Command {
	var flagAcceptAmbiguous bool

	cmd := &cobra.Command{
		Use:   "resolve <input>",
		Short: "Resolve a free-form location string",
		Long: "Accepts bare city ('bellevue'), city+state ('bellevue, wa'),\n" +
			"metro qualifier ('seattle metro'), or coordinates ('47.62,-122.20').\n" +
			"On success emits a GeoContext JSON; on ambiguity emits a\n" +
			"needs_clarification envelope (exit 0 either way — both are valid\n" +
			"responses, not errors).",
		Example: "  table-reservation-goat-pp-cli location resolve 'bellevue, wa' --json\n" +
			"  table-reservation-goat-pp-cli location resolve bellevue --batch-accept-ambiguous",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]
			gc, envelope, err := ResolveLocation(input, ResolveOptions{
				Source:          SourceExplicitFlag,
				AcceptAmbiguous: flagAcceptAmbiguous,
			})
			if err != nil {
				return err
			}
			if envelope != nil {
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			if gc == nil {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]string{
					"error_kind": ErrorKindLocationUnknown,
					"reason":     "empty input — provide a location string (e.g. 'bellevue, wa')",
				}, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), gc, flags)
		},
	}
	cmd.Flags().BoolVar(&flagAcceptAmbiguous, "batch-accept-ambiguous", false,
		"BATCH-ONLY escape hatch: force a pick on ambiguous input instead of "+
			"returning an envelope. Interactive agents must NOT set this — it defeats "+
			"the disambiguation contract.")
	return cmd
}
