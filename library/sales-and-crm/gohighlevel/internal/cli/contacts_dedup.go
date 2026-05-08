// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

type dedupGroup struct {
	Key        string   `json:"key"`
	By         string   `json:"by"`
	Count      int      `json:"count"`
	IDs        []string `json:"ids"`
	Names      []string `json:"names"`
	LocationID string   `json:"location_id"`
}

func newContactsDedupCmd(flags *rootFlags) *cobra.Command {
	var by string
	var location string
	var apply bool
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "dedup",
		Short:       "Find duplicate contacts in the local store by phone or email",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Group contacts by normalized phone, email, or both, returning every
group with more than one contact. With --apply (default off), the
command would call the contacts merge endpoint per pair; this version
prints the merge plan only.

Run 'gohighlevel-pp-cli sync' first; this query is local-only.
`,
		Example: strings.Trim(`
  # Dedup by phone OR email across all locations
  gohighlevel-pp-cli contacts dedup --by phone,email --json

  # Dedup just by email in one location
  gohighlevel-pp-cli contacts dedup --by email --location loc_abc123
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gohighlevel-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gohighlevel-pp-cli sync' first.", err)
			}
			defer db.Close()

			byParts := map[string]bool{}
			for _, p := range strings.Split(by, ",") {
				p = strings.TrimSpace(strings.ToLower(p))
				if p != "" {
					byParts[p] = true
				}
			}
			if len(byParts) == 0 {
				byParts["phone"] = true
				byParts["email"] = true
			}

			where := []string{"1=1"}
			argv := []any{}
			if location != "" {
				where = append(where, "json_extract(data, '$.locationId') = ?")
				argv = append(argv, location)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), fmt.Sprintf(`
				SELECT id,
					COALESCE(json_extract(data, '$.phone'), '') AS phone,
					COALESCE(json_extract(data, '$.email'), '') AS email,
					COALESCE(json_extract(data, '$.fullNameLowerCase'), json_extract(data, '$.firstName') || ' ' || json_extract(data, '$.lastName'), '') AS name,
					COALESCE(json_extract(data, '$.locationId'), '') AS loc
				FROM contacts WHERE %s
			`, strings.Join(where, " AND ")), argv...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			byPhone := map[string]*dedupGroup{}
			byEmail := map[string]*dedupGroup{}
			phoneRe := regexp.MustCompile(`[^0-9]+`)

			for rows.Next() {
				var id, phone, email, name, loc string
				if scanErr := rows.Scan(&id, &phone, &email, &name, &loc); scanErr != nil {
					continue
				}
				if byParts["phone"] && phone != "" {
					norm := phoneRe.ReplaceAllString(phone, "")
					if len(norm) >= 7 {
						g, ok := byPhone[norm]
						if !ok {
							g = &dedupGroup{Key: norm, By: "phone", LocationID: loc}
							byPhone[norm] = g
						}
						g.IDs = append(g.IDs, id)
						g.Names = append(g.Names, strings.TrimSpace(name))
						g.Count++
					}
				}
				if byParts["email"] && email != "" {
					norm := strings.ToLower(strings.TrimSpace(email))
					g, ok := byEmail[norm]
					if !ok {
						g = &dedupGroup{Key: norm, By: "email", LocationID: loc}
						byEmail[norm] = g
					}
					g.IDs = append(g.IDs, id)
					g.Names = append(g.Names, strings.TrimSpace(name))
					g.Count++
				}
			}

			var groups []*dedupGroup
			for _, g := range byPhone {
				if g.Count > 1 {
					groups = append(groups, g)
				}
			}
			for _, g := range byEmail {
				if g.Count > 1 {
					groups = append(groups, g)
				}
			}
			if limit > 0 && len(groups) > limit {
				groups = groups[:limit]
			}

			result := struct {
				Mode       string        `json:"mode"`
				By         []string      `json:"by"`
				GroupCount int           `json:"group_count"`
				Groups     []*dedupGroup `json:"groups"`
				Apply      bool          `json:"apply"`
			}{
				Mode:       "report-only",
				By:         sortedKeys(byParts),
				GroupCount: len(groups),
				Groups:     groups,
				Apply:      apply,
			}
			if apply {
				result.Mode = "would-apply"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Dedup — %d duplicate group(s)\n", len(groups))
			for _, g := range groups {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s — %d contacts: %v\n", g.By, g.Key, g.Count, g.IDs)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&by, "by", "phone,email", "Dedup keys: phone, email, or both (comma-separated)")
	cmd.Flags().StringVar(&location, "location", "", "Location id (default: all)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Plan and call merge endpoint (currently always reports planned ops)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max groups to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}
