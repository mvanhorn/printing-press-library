// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through fetchCPSC in cpsc_novel_support.go

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/cpsc-recalls/internal/store"
	"github.com/spf13/cobra"
)

func newNovelWatchChangesCmd(flags *rootFlags) *cobra.Command {
	var brand, product string

	cmd := &cobra.Command{
		Use:         "changes",
		Short:       "Compare a bounded brand or product recall observation with its prior local snapshot and report material changes.",
		Example:     "cpsc-recalls-pp-cli watch changes --brand Acme --agent",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitCPSCDryRun(cmd, flags, "watch changes", map[string]any{"brand": brand, "product": product, "snapshot_write": true, "material_fields": []string{"RecallDate", "Title", "Description", "URL", "ConsumerContact", "Products", "Hazards", "Remedies", "RemedyOptions", "Injuries", "Incidents", "Manufacturers", "Retailers"}})
			}
			if strings.TrimSpace(brand) == "" && strings.TrimSpace(product) == "" {
				return errors.New("provide --brand or --product")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows, err := newCPSCFetcher(flags).fetch(ctx, url.Values{"Manufacturer": {brand}, "ProductName": {product}})
			if err != nil {
				return err
			}
			current := map[string]json.RawMessage{}
			unidentified := 0
			for _, row := range rows {
				id := strings.TrimSpace(fmt.Sprint(row["RecallID"]))
				if id == "" || id == "<nil>" {
					unidentified++
					continue
				}
				raw, marshalErr := canonicalMaterialRecall(row)
				if marshalErr != nil {
					return marshalErr
				}
				current[id] = raw
			}
			key := strings.ToUpper(brand) + "|" + strings.ToUpper(product)
			db, err := store.OpenWithContext(ctx, defaultDBPath("cpsc-recalls-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			previous := map[string]json.RawMessage{}
			raw, getErr := db.Get("cpsc-recall-watch", key)
			baseline := errors.Is(getErr, sql.ErrNoRows)
			if getErr != nil && !baseline {
				return getErr
			}
			if getErr == nil {
				if err := json.Unmarshal(raw, &previous); err != nil {
					return err
				}
			}
			var added, changed []string
			preservedMissing := 0
			if !baseline {
				for id, value := range current {
					old, ok := previous[id]
					if !ok {
						added = append(added, id)
					} else if string(old) != string(value) {
						changed = append(changed, id)
					}
				}
				for id, old := range previous {
					if _, ok := current[id]; !ok {
						current[id] = old
						preservedMissing++
					}
				}
			}
			sort.Strings(added)
			sort.Strings(changed)
			next, _ := json.Marshal(current)
			if err := db.Upsert("cpsc-recall-watch", key, next); err != nil {
				return err
			}
			return emitCPSC(cmd, flags, "mixed", map[string]any{"brand": brand, "product": product, "baseline_created": baseline, "observed_recall_rows": len(rows), "persisted_recall_ids": len(current), "newly_observed_recall_ids": added, "changed_recall_ids": changed, "removed_recall_ids": nil, "removed_detection_available": false, "preserved_prior_ids_missing_from_response": preservedMissing, "unidentified_rows": unidentified, "observation_completeness": "unknown_provider_array_has_no_total_or_pagination_metadata", "change_definition": "Canonical comparison of documented material recall fields; array ordering is ignored. Missing prior IDs are preserved because the provider does not prove response completeness.", "caveats": cpscCaveats()})
		},
	}
	cmd.Flags().StringVar(&brand, "brand", "", "Manufacturer or brand filter")
	cmd.Flags().StringVar(&product, "product", "", "Product name filter")
	return cmd
}
