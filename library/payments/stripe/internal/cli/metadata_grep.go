// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

type metadataHit struct {
	ResourceType string `json:"resource_type"`
	ID           string `json:"id"`
	Key          string `json:"key"`
	Value        string `json:"value"`
	RelatedID    string `json:"related_id,omitempty"`
}

func newMetadataGrepCmd(flags *rootFlags) *cobra.Command {
	var resourceType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "metadata-grep <key=value>",
		Short: "Search every synced resource's metadata bag for key=value across resource types",
		Long: `Walk the local resources mirror and report every record whose JSON metadata
object contains the given key/value pair. Useful for tracing a Stripe object
back to your internal IDs (e.g. "internal_order_id=42").`,
		Example: `  # Find every object tagged with internal_order_id=42
  stripe-pp-cli metadata-grep internal_order_id=42

  # Restrict to charges
  stripe-pp-cli metadata-grep campaign=summer25 --type charges --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			key, value, ok := strings.Cut(args[0], "=")
			if !ok || key == "" {
				return usageErr(fmt.Errorf("expected <key=value>, got %q", args[0]))
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' first.", path, err))
			}
			defer db.Close()

			hits, err := grepMetadata(db.DB(), resourceType, key, value)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}

	cmd.Flags().StringVar(&resourceType, "type", "", "Restrict the search to a single resource_type")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	return cmd
}

func grepMetadata(db *sql.DB, resourceType, key, value string) ([]metadataHit, error) {
	q := `SELECT resource_type, id, json_extract(data, '$.metadata.' || ?), data
		  FROM resources WHERE json_extract(data,'$.metadata.' || ?) IS NOT NULL`
	args := []any{key, key}
	if resourceType != "" {
		q += ` AND resource_type = ?`
		args = append(args, resourceType)
	}
	rs, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	out := make([]metadataHit, 0)
	for rs.Next() {
		var rt, id, data string
		var got sql.NullString
		if err := rs.Scan(&rt, &id, &got, &data); err != nil {
			return nil, err
		}
		if !got.Valid {
			continue
		}
		// json_extract returns the raw JSON value — strings come back unquoted
		// in the SQLite text output, so no further unwrap is needed.
		if value != "" && got.String != value {
			continue
		}
		hit := metadataHit{
			ResourceType: rt,
			ID:           id,
			Key:          key,
			Value:        got.String,
		}
		// Pull related customer/invoice id when applicable so the consumer
		// has a one-hop reference for the matched record.
		if related, ok := jsonGet(json.RawMessage(data), "customer"); ok {
			hit.RelatedID = related
		} else if related, ok := jsonGet(json.RawMessage(data), "invoice"); ok {
			hit.RelatedID = related
		}
		out = append(out, hit)
	}
	return out, rs.Err()
}
