// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/cfpb-complaints/internal/store"
	"github.com/spf13/cobra"
)

func newNovelWatchChangesCmd(flags *rootFlags) *cobra.Command {
	var company, product, state, window string
	var limit int

	cmd := &cobra.Command{
		Use:         "changes --company COMPANY",
		Short:       "Compare the latest bounded cohort with a locally persisted prior observation and report newly observed complaint IDs",
		Example:     "cfpb-complaints-pp-cli watch changes --company 'TRANSUNION INTERMEDIATE HOLDINGS, INC.' --window 30d --agent",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			cohort, duration, err := validateCohort(company, product, state, window, limit)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			start, end := currentCalendarRange(duration)
			response, err := fetchCFPB(ctx, flags, cohortParams(cohort, start, end))
			if err != nil {
				return err
			}
			current := map[string]map[string]string{}
			for _, hit := range response.Hits.Hits {
				id := fmt.Sprint(hit.Source["complaint_id"])
				current[id] = map[string]string{"product": fmt.Sprint(hit.Source["product"]), "issue": fmt.Sprint(hit.Source["issue"])}
			}
			key := strings.Join([]string{strings.ToUpper(company), product, strings.ToUpper(state), window}, "|")
			db, err := store.OpenWithContext(ctx, defaultDBPath("cfpb-complaints-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			previous := map[string]map[string]string{}
			raw, getErr := db.Get("cfpb-complaint-watch", key)
			baseline := errors.Is(getErr, sql.ErrNoRows)
			if getErr != nil && !baseline {
				return getErr
			}
			if getErr == nil {
				if err := json.Unmarshal(raw, &previous); err != nil {
					return err
				}
			}
			var newIDs []string
			products, issues := map[string]bool{}, map[string]bool{}
			if !baseline {
				for id, row := range current {
					if _, ok := previous[id]; !ok {
						newIDs = append(newIDs, id)
						if row["product"] != "" {
							products[row["product"]] = true
						}
						if row["issue"] != "" {
							issues[row["issue"]] = true
						}
					}
				}
			}
			sort.Strings(newIDs)
			next, _ := json.Marshal(current)
			if err := db.Upsert("cfpb-complaint-watch", key, next); err != nil {
				return err
			}
			payload := map[string]any{"cohort": map[string]any{"company": company, "product": product, "state": state, "window": window, "dates": rangeMetadata(start, end), "record_cap": limit}, "baseline_created": baseline, "observed_records": len(current), "total_matching_records": response.Hits.Total.Value, "observation_complete": response.Hits.Total.Value <= limit, "new_complaint_ids": newIDs, "new_products": sortedKeys(products), "new_issues": sortedKeys(issues), "api_meta": response.Meta, "caveats": append(standardCaveats(), "Watch observes at most 100 newest records; narrow filters until observation_complete is true for complete change detection.")}
			rawOut, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				return marshalErr
			}
			return printOutputWithFlagsMeta(cmd.OutOrStdout(), rawOut, flags, map[string]any{"source": "mixed", "provider": "CFPB Consumer Complaint Database + local snapshot"})
		},
	}
	cmd.Flags().StringVar(&company, "company", "", "Exact CFPB company label")
	cmd.Flags().StringVar(&product, "product", "", "Optional exact product label")
	cmd.Flags().StringVar(&state, "state", "", "Optional two-letter state code")
	cmd.Flags().StringVar(&window, "window", "30d", "Bounded observation window")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum newest records observed per run (1-100)")
	return cmd
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
