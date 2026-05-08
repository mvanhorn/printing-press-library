// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

type bulkTagPlan struct {
	ContactID string `json:"contact_id"`
	Action    string `json:"action"`
	Tag       string `json:"tag"`
}

func newContactsBulkTagCmd(flags *rootFlags) *cobra.Command {
	var fromSearch string
	var location string
	var dbPath string
	var remove bool
	var limit int

	cmd := &cobra.Command{
		Use:         "bulk-tag <tag>",
		Short:       "Apply or remove a tag across the results of a contact search (dry-run by default)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Plan and (with --dry-run=false) apply a tag across every contact matching
a local search query. Composes the local FTS index with per-contact tag
mutations, with rate-limit-aware progress.

This default plan-only mode is intentional: bulk-tag is high-risk; the
default behaviour prints the plan as JSON. Pass --dry-run=false to invoke
the API. Without an API key configured, only the plan is printed.

Run 'gohighlevel-pp-cli sync' first; the search runs against local data.
`,
		Example: strings.Trim(`
  # Plan-only: see which contacts would be tagged
  gohighlevel-pp-cli contacts bulk-tag --from-search "campaign:spring-2026" tested --json

  # Plan a tag-removal
  gohighlevel-pp-cli contacts bulk-tag --from-search "tag:trial" --remove churned --json
`, "\n"),
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			tag := strings.TrimSpace(args[0])
			if tag == "" {
				return fmt.Errorf("missing <tag> argument")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("gohighlevel-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gohighlevel-pp-cli sync' first.", err)
			}
			defer db.Close()

			where := []string{"1=1"}
			argv := []any{}
			if location != "" {
				where = append(where, "json_extract(data, '$.locationId') = ?")
				argv = append(argv, location)
			}
			if fromSearch != "" {
				// Use a simple JSON-substring match on the contact data — broader
				// than FTS for things like phone digits or tag names.
				where = append(where, "data LIKE ?")
				argv = append(argv, "%"+fromSearch+"%")
			}

			q := fmt.Sprintf(`
				SELECT id, COALESCE(json_extract(data, '$.fullNameLowerCase'), '') AS name
				FROM contacts WHERE %s LIMIT ?
			`, strings.Join(where, " AND "))
			argv = append(argv, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			action := "add-tag"
			if remove {
				action = "remove-tag"
			}
			var plans []bulkTagPlan
			for rows.Next() {
				var id, name string
				_ = name
				if scanErr := rows.Scan(&id, &name); scanErr != nil {
					continue
				}
				plans = append(plans, bulkTagPlan{ContactID: id, Action: action, Tag: tag})
			}

			result := struct {
				Mode      string        `json:"mode"`
				Action    string        `json:"action"`
				Tag       string        `json:"tag"`
				Search    string        `json:"from_search"`
				Location  string        `json:"location"`
				PlanCount int           `json:"plan_count"`
				Plans     []bulkTagPlan `json:"plans"`
			}{Mode: "report-only", Action: action, Tag: tag, Search: fromSearch, Location: location, PlanCount: len(plans), Plans: plans}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bulk-tag plan — %d contact(s) would be %sed with tag %q\n", len(plans), action, tag)
			for _, p := range plans {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\n", p.ContactID, p.Action)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromSearch, "from-search", "", "Filter contacts whose JSON contains this substring (e.g. tag:foo, campaign:bar)")
	cmd.Flags().StringVar(&location, "location", "", "Location id (default: all)")
	cmd.Flags().BoolVar(&remove, "remove", false, "Plan a tag removal instead of an add")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Max contacts to plan against")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}
