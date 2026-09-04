// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Splitwise novel command port.
// Preserved outside generated files.
// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newResolveCmd(flags *rootFlags) *cobra.Command {
	var resolveType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "resolve <name>",
		Short: "Resolve a friend, group, or category name to its Splitwise id from the local store (no network)",
		Long: strings.Trim(`
Resolve a friend, group, or category NAME to its Splitwise id, matching the local store (no network).

Use this command whenever you need an id for a name: the returned records are authoritative — every id-taking
command (ledger, settle-up, get-friend, get-group, get-expenses --group-id) accepts the id directly, so there is no
need to re-check the result with get-friend, get-friends, search, or sql.
Do NOT use get-friends, get-groups, search, or sql to look up an id by name; use this command instead.
Do NOT use it for balances or debts; use 'balances' or 'debts' instead.
An empty result means nothing matched (exit 0): run 'sync' first if the store is stale, or try --type to narrow.
`, "\n"),
		Example: "  splitwise-pp-cli resolve \"Alex Kim\" --agent",
		// no-error-path-probe: a name matching nothing is a valid empty result
		// (exit 0, empty list), not an error, so the generic invalid-argument
		// probe does not apply.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "resolve")
			}
			if len(args) == 0 {
				return novelErr(cmd, flags, usageErr(errors.New("name argument is required")))
			}
			name := joinNameArgs(args)

			db, err := openSplitwiseStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			resolveOne := func(kind, input string) (map[string]any, error) {
				var fields []string
				switch kind {
				case "friend":
					fields = []string{"first_name", "last_name"}
					id, err := db.ResolveByName("get-friends", input, fields...)
					if err != nil {
						return nil, err
					}
					return map[string]any{"type": "friend", "id": id, "name_input": input}, nil
				case "group":
					id, err := db.ResolveByName("get-groups", input, "name")
					if err != nil {
						return nil, err
					}
					return map[string]any{"type": "group", "id": id, "name_input": input}, nil
				case "category":
					id, err := db.ResolveByName("get-categories", input, "name")
					if err != nil {
						return nil, err
					}
					return map[string]any{"type": "category", "id": id, "name_input": input}, nil
				default:
					return nil, fmt.Errorf("unsupported type %q", kind)
				}
			}

			if resolveType != "" {
				item, err := resolveOne(resolveType, name)
				if err != nil {
					return err
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return flags.emitStructured(cmd, item)
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "TYPE\tID\tNAME INPUT")
				_, _ = fmt.Fprintf(tw, "%v\t%v\t%v\n", item["type"], item["id"], item["name_input"])
				return tw.Flush()
			}

			results := make([]map[string]any, 0)
			for _, kind := range []string{"friend", "group", "category"} {
				item, err := resolveOne(kind, name)
				if err != nil {
					continue
				}
				results = append(results, item)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.emitStructured(cmd, results)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "TYPE\tID\tNAME INPUT")
			for _, item := range results {
				_, _ = fmt.Fprintf(tw, "%v\t%v\t%v\n", item["type"], item["id"], item["name_input"])
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&resolveType, "type", "", "Type to resolve: friend|group|category")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
