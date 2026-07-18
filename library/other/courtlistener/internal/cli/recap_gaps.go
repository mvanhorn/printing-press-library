// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through clGet in courtlistener_novel_support.go

package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelRecapGapsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "recap-gaps DOCKET_ID",
		Short:       "Classify bounded docket document records by CourtListener availability fields without implying complete PACER coverage.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			response, err := clGet(ctx, flags, "/recap-documents/", url.Values{"docket_entry__docket": {args[0]}, "page_size": {"100"}}, true)
			if err != nil {
				return err
			}
			counts := map[string]int{"available": 0, "metadata_only": 0, "unavailable": 0, "ambiguous": 0}
			var rows []map[string]any
			for _, doc := range clResults(response) {
				status := "ambiguous"
				availability, availabilityKnown := parseCLBool(doc["is_available"])
				pacer := fmt.Sprint(doc["pacer_doc_id"])
				if !availabilityKnown {
					status = "ambiguous"
				} else if availability {
					status = "available"
				} else if pacer != "" && pacer != "<nil>" {
					status = "metadata_only"
				} else {
					status = "unavailable"
				}
				counts[status]++
				rows = append(rows, map[string]any{"id": doc["id"], "status": status, "is_available": doc["is_available"], "filepath_local": doc["filepath_local"], "pacer_doc_id": doc["pacer_doc_id"], "page_count": doc["page_count"]})
			}
			return emitCL(cmd, flags, "live", map[string]any{"docket_id": args[0], "counts": counts, "observed_documents": len(rows), "complete_observation": response["next"] == nil, "documents": rows, "next": response["next"], "caveats": clCaveats()})
		},
	}
	return cmd
}

func parseCLBool(value any) (bool, bool) {
	if value == nil {
		return false, false
	}
	s := strings.TrimSpace(fmt.Sprint(value))
	if s == "" || s == "<nil>" {
		return false, false
	}
	parsed, err := strconv.ParseBool(s)
	return parsed, err == nil
}
