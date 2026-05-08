// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

type reconcileResult struct {
	Source   string              `json:"source"`
	Key      string              `json:"key"`
	Source_n int                 `json:"source_count"`
	Local_n  int                 `json:"local_count"`
	Created  []map[string]string `json:"created"`
	Updated  []map[string]string `json:"updated"`
	Missing  []map[string]string `json:"missing"`
	Extra    []map[string]string `json:"extra"`
}

func newContactsReconcileCmd(flags *rootFlags) *cobra.Command {
	var sourcePath string
	var key string
	var location string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "reconcile",
		Short:       "Diff a source CSV against synced contacts on a key column",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Compare the contents of a source CSV against the synced local contacts
table on a key column. Output four buckets:
  created — present in source AND local
  updated — same key, but the source row's tracked fields differ
  missing — present in source but not in local (failed to land)
  extra   — present in local but not in source (out-of-band data)

Useful after a bulk migration. Pure local; doesn't call the API.
`,
		Example: strings.Trim(`
  # Reconcile a migration CSV against synced contacts
  gohighlevel-pp-cli contacts reconcile --source migration.csv --key email --json

  # Single location, key by phone
  gohighlevel-pp-cli contacts reconcile --source phones.csv --key phone --location loc_abc123
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if sourcePath == "" {
				return fmt.Errorf("missing --source <csv-path>")
			}
			if key == "" {
				return fmt.Errorf("missing --key <field>")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gohighlevel-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gohighlevel-pp-cli sync' first.", err)
			}
			defer db.Close()

			f, err := os.Open(sourcePath)
			if err != nil {
				return fmt.Errorf("opening source CSV: %w", err)
			}
			defer f.Close()

			r := csv.NewReader(f)
			r.FieldsPerRecord = -1
			rows, err := r.ReadAll()
			if err != nil {
				return fmt.Errorf("reading CSV: %w", err)
			}
			if len(rows) < 2 {
				return fmt.Errorf("CSV has no data rows")
			}
			header := rows[0]
			keyIdx := -1
			for i, h := range header {
				if strings.EqualFold(strings.TrimSpace(h), key) {
					keyIdx = i
					break
				}
			}
			if keyIdx == -1 {
				return fmt.Errorf("key column %q not found in CSV header %v", key, header)
			}

			// Build source set
			sourceSet := map[string]map[string]string{}
			for _, row := range rows[1:] {
				if keyIdx >= len(row) {
					continue
				}
				k := strings.ToLower(strings.TrimSpace(row[keyIdx]))
				if k == "" {
					continue
				}
				rec := map[string]string{}
				for i, v := range row {
					if i < len(header) {
						rec[header[i]] = v
					}
				}
				sourceSet[k] = rec
			}

			// Build local set
			localSet := map[string]map[string]string{}
			where := []string{"1=1"}
			argv := []any{}
			if location != "" {
				where = append(where, "json_extract(data, '$.locationId') = ?")
				argv = append(argv, location)
			}
			localPath := "$." + key
			q := fmt.Sprintf(`
				SELECT id, COALESCE(json_extract(data, '%s'), '') AS k,
				       COALESCE(json_extract(data, '$.email'), '') AS email,
				       COALESCE(json_extract(data, '$.phone'), '') AS phone,
				       COALESCE(json_extract(data, '$.firstName'), '') AS first_name,
				       COALESCE(json_extract(data, '$.lastName'), '') AS last_name
				FROM contacts WHERE %s
			`, localPath, strings.Join(where, " AND "))
			localRows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("local query: %w", err)
			}
			defer localRows.Close()
			for localRows.Next() {
				var id, k, email, phone, fn, ln string
				if scanErr := localRows.Scan(&id, &k, &email, &phone, &fn, &ln); scanErr != nil {
					continue
				}
				kl := strings.ToLower(strings.TrimSpace(k))
				if kl == "" {
					continue
				}
				localSet[kl] = map[string]string{
					"id": id, "email": email, "phone": phone, "first_name": fn, "last_name": ln, key: k,
				}
			}

			res := reconcileResult{
				Source:   sourcePath,
				Key:      key,
				Source_n: len(sourceSet),
				Local_n:  len(localSet),
			}
			for k, srcRec := range sourceSet {
				if _, ok := localSet[k]; ok {
					// Compare matched record fields where both sides have them
					if differs(srcRec, localSet[k], []string{"email", "phone", "first_name", "last_name"}) {
						res.Updated = append(res.Updated, map[string]string{key: k})
					} else {
						res.Created = append(res.Created, map[string]string{key: k})
					}
				} else {
					res.Missing = append(res.Missing, map[string]string{key: k})
				}
			}
			for k := range localSet {
				if _, ok := sourceSet[k]; !ok {
					res.Extra = append(res.Extra, map[string]string{key: k})
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Reconcile — source=%d local=%d\n", res.Source_n, res.Local_n)
			fmt.Fprintf(cmd.OutOrStdout(), "  created (key match, fields match):  %d\n", len(res.Created))
			fmt.Fprintf(cmd.OutOrStdout(), "  updated (key match, fields differ): %d\n", len(res.Updated))
			fmt.Fprintf(cmd.OutOrStdout(), "  missing (in source, not in local):  %d\n", len(res.Missing))
			fmt.Fprintf(cmd.OutOrStdout(), "  extra   (in local, not in source):  %d\n", len(res.Extra))
			return nil
		},
	}

	cmd.Flags().StringVar(&sourcePath, "source", "", "Path to source CSV (header row required)")
	cmd.Flags().StringVar(&key, "key", "email", "Column to join on (must exist in CSV header AND in contact data)")
	cmd.Flags().StringVar(&location, "location", "", "Location id (default: all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

func differs(a, b map[string]string, fields []string) bool {
	for _, f := range fields {
		av := strings.TrimSpace(strings.ToLower(a[f]))
		bv := strings.TrimSpace(strings.ToLower(b[f]))
		if av != "" && bv != "" && av != bv {
			return true
		}
	}
	return false
}
