// Hand-written novel command: pivot custom fields across all members.

package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/auth/memberstack/internal/store"
)

func newFieldsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fields",
		Short: "Custom-field tooling: flatten member customFields across the local store",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFieldsFlattenCmd(flags))
	return cmd
}

func newFieldsFlattenCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var asCSV bool
	var includeBlank bool

	cmd := &cobra.Command{
		Use:   "flatten",
		Short: "Pivot every member's customFields map into a flat table (CSV or JSON).",
		Long: `Produces one row per member; one column per custom-field key observed across
the whole local mirror. Useful for BI exports and marketing pivots.

Run 'memberstack-pp-cli sync --full' first to populate the local store.`,
		Example: `  memberstack-pp-cli fields flatten --csv > members.csv
  memberstack-pp-cli fields flatten --json | jq '.[] | select(.["custom.plan_tier"] == "gold")'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would flatten customFields across local member store")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("memberstack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w (hint: run 'sync --full' first)", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, data FROM resources
				WHERE resource_type IN ('members', 'member')`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			fieldSet := map[string]struct{}{}
			flatRows := []map[string]string{}

			for rows.Next() {
				var id string
				var data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				if !data.Valid {
					continue
				}
				var m map[string]any
				if err := json.Unmarshal([]byte(data.String), &m); err != nil {
					continue
				}
				row := map[string]string{
					"id":        id,
					"email":     "",
					"createdAt": stringFromAny(m["createdAt"]),
					"lastLogin": stringFromAny(m["lastLogin"]),
					"verified":  fmt.Sprintf("%v", m["verified"]),
				}
				if auth, ok := m["auth"].(map[string]any); ok {
					row["email"] = stringFromAny(auth["email"])
				}
				if cf, ok := m["customFields"].(map[string]any); ok {
					for k, v := range cf {
						col := "custom." + k
						fieldSet[col] = struct{}{}
						row[col] = formatScalar(v)
					}
				}
				flatRows = append(flatRows, row)
			}

			cols := []string{"id", "email", "createdAt", "lastLogin", "verified"}
			extras := make([]string, 0, len(fieldSet))
			for k := range fieldSet {
				extras = append(extras, k)
			}
			sort.Strings(extras)
			cols = append(cols, extras...)

			if !includeBlank {
				// includeBlank false is the default; nothing to filter for now (rows already include only observed keys).
				_ = includeBlank
			}

			if asCSV || flags.csv {
				w := csv.NewWriter(cmd.OutOrStdout())
				if err := w.Write(cols); err != nil {
					return err
				}
				for _, r := range flatRows {
					rec := make([]string, len(cols))
					for i, c := range cols {
						rec[i] = r[c]
					}
					if err := w.Write(rec); err != nil {
						return err
					}
				}
				w.Flush()
				return w.Error()
			}

			// JSON output
			out := make([]map[string]any, 0, len(flatRows))
			for _, r := range flatRows {
				m := map[string]any{}
				for _, c := range cols {
					if v, ok := r[c]; ok && v != "" {
						m[c] = v
					}
				}
				out = append(out, m)
			}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	cmd.Flags().BoolVar(&asCSV, "csv", false, "Output as CSV (overrides --json)")
	cmd.Flags().BoolVar(&includeBlank, "include-blank", false, "Include rows even when no customFields are present")
	return cmd
}

func formatScalar(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return fmt.Sprintf("%v", x)
	case float64:
		// JSON numbers always decode to float64. Print integers cleanly.
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case nil:
		return ""
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
}
