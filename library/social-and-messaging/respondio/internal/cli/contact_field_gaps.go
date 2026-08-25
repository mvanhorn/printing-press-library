// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command contact field-gaps: find contacts missing a custom field value.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/internal/store"
	"github.com/spf13/cobra"
)

type fieldGapRow struct {
	ID          int    `json:"id"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Missing     bool   `json:"missing"`
	CustomField string `json:"custom_field"`
	Value       any    `json:"value,omitempty"`
}

// pp:data-source local

func newNovelContactFieldGapsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var fieldName string
	var value string
	var limit int

	cmd := &cobra.Command{
		Use:         "field-gaps",
		Short:       "Find contacts missing a custom field value (e.g. no orderId or region).",
		Long:        "Scans synced contacts and reports those missing the named custom field, or (with --value) those whose field differs from the value.",
		Example:     "  respondio-pp-cli contact field-gaps --name orderId --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contact field-gaps")
			}
			if fieldName == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--name is required (the custom field to check)"))
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("respondio-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: respondio-pp-cli sync --resources contact --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]fieldGapRow, 0), flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No synced contacts yet.")
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'contact'`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			var datas [][]byte
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan contact: %w", err)
				}
				datas = append(datas, data)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate contacts: %w", err)
			}
			_ = rows.Close()

			results := make([]fieldGapRow, 0)
			for _, raw := range datas {
				var c map[string]any
				if err := json.Unmarshal(raw, &c); err != nil {
					continue
				}
				foundVal, found := customFieldValue(c, fieldName)
				row := fieldGapRow{
					ID: intNum(c["id"]), FirstName: str(c["firstName"]), LastName: str(c["lastName"]),
					Email: str(c["email"]), Phone: str(c["phone"]), CustomField: fieldName,
				}
				if value != "" {
					// match contacts whose field does not equal the given value
					if found && fmt.Sprint(foundVal) == value {
						continue
					}
					if !found {
						row.Missing = true
					}
					results = append(results, row)
				} else {
					if !found {
						row.Missing = true
						results = append(results, row)
					}
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "id=%d %v %v (%v missing=%v)\n", r.ID, r.Email, r.Phone, r.CustomField, r.Missing)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fieldName, "name", "", "custom field name to check")
	cmd.Flags().StringVar(&value, "value", "", "expected value; flag contacts whose field differs")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum contacts to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func customFieldValue(c map[string]any, fieldName string) (any, bool) {
	cf, ok := c["custom_fields"].([]any)
	if !ok {
		return nil, false
	}
	for _, e := range cf {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if str(m["name"]) == fieldName {
			return m["value"], true
		}
	}
	return nil, false
}

func intNum(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n := 0
		for _, r := range t {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		return n
	}
	return 0
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
