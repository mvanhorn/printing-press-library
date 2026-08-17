// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through echoGet in echo_novel_support.go

package cli

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelResolveFacilityCmd(flags *rootFlags) *cobra.Command {
	var name, frs, state, city, zip string

	cmd := &cobra.Command{
		Use:         "facility",
		Short:       "Rank bounded facility matches while preserving FRS identifiers, addresses, program status fields, and query provenance.",
		Example:     "epa-echo-pp-cli resolve facility --name 'CAPITAL EXXON' --state RI --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" && strings.TrimSpace(frs) == "" {
				return errors.New("provide --name or --frs")
			}
			params := url.Values{"p_fn": {name}, "p_fntype": {"CONTAINS"}, "p_frs": {frs}, "p_st": {state}, "p_ct": {city}, "p_zip": {zip}}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			summary, facilities, err := searchEcho(ctx, flags, params)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(facilities))
			query := map[string]string{"name": name, "frs": frs, "state": state, "city": city, "zip": zip}
			for _, facility := range facilities {
				rows = append(rows, rankedFacility(facility, query))
			}
			sort.SliceStable(rows, func(i, j int) bool {
				left, right := rows[i]["match_score"].(int), rows[j]["match_score"].(int)
				if left != right {
					return left > right
				}
				return fmt.Sprint(rows[i]["registry_id"]) < fmt.Sprint(rows[j]["registry_id"])
			})
			total := queryRowCount(summary)
			return emitECHO(cmd, flags, "live", map[string]any{"query": query, "summary": summary, "coverage": map[string]any{"total_matches": total, "returned": len(rows), "truncated": total > len(rows), "limit": 100}, "matches": rows, "ranking": "Deterministic exact/contains field agreement; score is match quality only, not environmental risk. Verify FRS ID and address before selection.", "caveats": echoCaveats()})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Facility name substring")
	cmd.Flags().StringVar(&frs, "frs", "", "Exact 12-digit FRS registry ID")
	cmd.Flags().StringVar(&state, "state", "", "Two-letter state code")
	cmd.Flags().StringVar(&city, "city", "", "City")
	cmd.Flags().StringVar(&zip, "zip", "", "ZIP code")
	return cmd
}
