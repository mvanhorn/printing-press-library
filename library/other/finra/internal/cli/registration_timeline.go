// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/cliutil"
	"github.com/spf13/cobra"
)

// registrationTimelineSource names one of the three datasets joined by
// `registration timeline`. A CRD-keyed identifier field is very likely
// present across all three given FINRA's registration data model, so
// server-side compareFilters narrowing is used here (unlike the
// symbol/CUSIP commands, which cannot assume a field name).
type registrationTimelineSource struct {
	key   string
	group string
	name  string
}

// pp:data-source live
func newNovelRegistrationTimelineCmd(flags *rootFlags) *cobra.Command {
	var flagCrd string

	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Full chronological registration-status history for one person, joining Composite Individual",
		Long: "Full chronological registration-status history for one person, joining Composite Individual,\n" +
			"Firm Registration Status History, and Individual Delta records by CRD number.\n\n" +
			"Requires a FINRA credential entitled for registration/firm data — a basic-tier credential\n" +
			"will receive a 403 with a clear permission-denied message.",
		Example:     "--crd 1234567 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--crd=1000001", "pp:requires-tier": "entitled"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch composite individual, firm registration status history, and individual delta records for CRD %s\n", flagCrd)
				return nil
			}
			if strings.TrimSpace(flagCrd) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--crd is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			sources := []registrationTimelineSource{
				{key: "composite_individual", group: "REGISTRATION", name: "COMPOSITEINDIVIDUAL"},
				{key: "firm_registration_status_history", group: "FIRM", name: "FIRMREGISTRATIONSTATUSHISTORY"},
				{key: "individual_delta", group: "REGISTRATION", name: "INDIVIDUALDELTA"},
			}

			results, errs := cliutil.FanoutRun(ctx, sources,
				func(s registrationTimelineSource) string { return s.key },
				func(fctx context.Context, s registrationTimelineSource) ([]map[string]any, error) {
					return fetchRegistrationDatasetByCRD(fctx, c, s, flagCrd)
				},
			)

			view := registrationTimelineView{CRD: flagCrd}
			var entries []registrationTimelineEntry
			for _, r := range results {
				for _, rec := range r.Value {
					entry := registrationTimelineEntry{Source: r.Source, Record: rec}
					if d, ok := findRecordDate(rec); ok {
						entry.Date = d.Format("2006-01-02")
						entries = append(entries, entry)
					} else {
						view.Undated = append(view.Undated, entry)
					}
				}
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Date < entries[j].Date })
			view.Timeline = entries
			for _, e := range errs {
				view.FetchFailures = append(view.FetchFailures, fixedincomeFetchFailure{Dataset: e.Source, Error: e.Err.Error()})
			}
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "CRD %s: %d dated event(s), %d undated\n", view.CRD, len(view.Timeline), len(view.Undated))
				for _, e := range view.Timeline {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s [%s]\n", e.Date, e.Source)
				}
				for _, f := range view.FetchFailures {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: fetch failed: %s\n", f.Dataset, f.Error)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCrd, "crd", "", "CRD number to build a registration timeline for (required)")
	return cmd
}

type registrationTimelineEntry struct {
	Source string         `json:"source"`
	Date   string         `json:"date,omitempty"`
	Record map[string]any `json:"record"`
}

type registrationTimelineView struct {
	CRD           string                      `json:"crd"`
	Timeline      []registrationTimelineEntry `json:"timeline"`
	Undated       []registrationTimelineEntry `json:"undated,omitempty"`
	FetchFailures []fixedincomeFetchFailure   `json:"fetch_failures,omitempty"`
}

func fetchRegistrationDatasetByCRD(ctx context.Context, c *client.Client, s registrationTimelineSource, crd string) ([]map[string]any, error) {
	path := replacePathParam(replacePathParam("/data/group/{group}/name/{name}", "group", s.group), "name", s.name)
	body := map[string]any{
		"compareFilters": []map[string]any{
			{"fieldName": "crdNumber", "fieldValue": crd, "compareType": "EQUAL"},
		},
	}
	data, _, err := c.PostQueryWithParams(ctx, path, nil, body)
	if err != nil {
		return nil, err
	}
	records, err := parseDatasetRecords(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s/%s response: %w", s.group, s.name, err)
	}
	return records, nil
}
