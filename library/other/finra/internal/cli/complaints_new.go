// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelComplaintsNewCmd(flags *rootFlags) *cobra.Command {
	var flagFirm string
	var flagSince string
	var flagGroup string
	var flagName string

	cmd := &cobra.Command{
		Use:   "new",
		Short: "See 4530 customer complaint filings for a firm within a recent time window",
		Long: "See 4530 customer complaint filings for a firm filed within a recent time window, without\n" +
			"re-reading the full history.\n\n" +
			"--since accepts a duration like 24h, 7d, or 1w (default 7d).\n\n" +
			"Requires a FINRA credential entitled for firm data — a basic-tier credential will receive a\n" +
			"403 with a clear permission-denied message.",
		Example:     "--firm 19847 --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--firm=19847", "pp:requires-tier": "entitled"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s/%s filings for firm %s since %s\n", flagGroup, flagName, flagFirm, flagSince)
				return nil
			}
			if strings.TrimSpace(flagFirm) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--firm is required"))
			}

			since := flagSince
			if strings.TrimSpace(since) == "" {
				since = "7d"
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			cutoff := now.Add(-window)
			path := replacePathParam(replacePathParam("/data/group/{group}/name/{name}", "group", flagGroup), "name", flagName)
			body := map[string]any{
				// firmCrdNumber is the confirmed identifier field for this
				// dataset. No dateRangeFilters is sent server-side: the
				// filing-date field name is unconfirmed, so --since (when
				// provided) is applied client-side below instead via
				// findRecordDate/filterRecordsByDateWindow.
				"compareFilters": []map[string]any{
					{"fieldName": "firmCrdNumber", "fieldValue": flagFirm, "compareType": "EQUAL"},
				},
			}
			data, _, err := c.PostQueryWithParams(ctx, path, nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			filings, err := parseDatasetRecords(data)
			if err != nil {
				return apiErr(fmt.Errorf("parsing %s/%s response: %w", flagGroup, flagName, err))
			}
			// submissionDate is the confirmed filing-date field for 4530FILINGS
			// (per /metadata); preferred over the generic date-key scan since a
			// filing record also carries discoveryDate, and picking the wrong
			// one alphabetically would filter by the wrong semantic date.
			filings = filterRecordsByDateWindowPreferField(filings, "submissionDate", cutoff, now)

			view := complaintsNewView{
				FirmCRD:    flagFirm,
				Since:      since,
				NewFilings: filings,
				Count:      len(filings),
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%d new 4530 filing(s) for firm %s since %s\n", view.Count, view.FirmCRD, view.Since)
				for _, f := range filings {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", summarizeFiling(f))
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagFirm, "firm", "", "Firm CRD number to check for new 4530 customer complaint filings (required)")
	cmd.Flags().StringVar(&flagSince, "since", "7d", "How far back to look for new filings (e.g. 24h, 7d, 1w)")
	cmd.Flags().StringVar(&flagGroup, "group", "FIRM", "Dataset group for 4530 Customer Complaints (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().StringVar(&flagName, "name", "4530FILINGS", "Dataset name for 4530 Customer Complaints (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	return cmd
}

type complaintsNewView struct {
	FirmCRD    string           `json:"firm_crd"`
	Since      string           `json:"since"`
	NewFilings []map[string]any `json:"new_filings"`
	Count      int              `json:"count"`
}

// summarizeFiling renders a best-effort one-line human summary of a filing
// record. The exact field names for this dataset are unconfirmed, so it
// looks for an id-like, a date-like, and a description-like key rather than
// assuming a specific schema.
func summarizeFiling(rec map[string]any) string {
	id := firstMatchingValue(rec, isIDKey)
	date := firstMatchingValue(rec, func(k string) bool { return strings.Contains(strings.ToLower(k), "date") })
	desc := firstMatchingValue(rec, func(k string) bool { return strings.Contains(strings.ToLower(k), "description") })

	parts := make([]string, 0, 3)
	if id != "" {
		parts = append(parts, "id="+id)
	}
	if date != "" {
		parts = append(parts, "date="+date)
	}
	if desc != "" {
		parts = append(parts, desc)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%v", rec)
	}
	return strings.Join(parts, " ")
}

// firstMatchingValue returns the string form of the value at the first key
// (in sorted order) for which match returns true, or "" if no key matches.
// Go map iteration order is randomized per run, so candidate keys are
// collected and sorted before picking — this keeps the chosen key (and thus
// the rendered summary) stable across runs when a record has more than one
// key satisfying match.
func firstMatchingValue(rec map[string]any, match func(key string) bool) string {
	var candidates []string
	for k := range rec {
		if match(k) {
			candidates = append(candidates, k)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	return fmt.Sprintf("%v", rec[candidates[0]])
}

// isIDKey reports whether k looks like an identifier field. A plain
// substring check for "id" is too broad — it also matches unrelated fields
// such as "confidential", "provided", or "residentState" — so this instead
// requires a word-boundary match: the key is exactly "id" (case-insensitive),
// ends in a capitalized "Id"/"ID" suffix (filingId, complaint_ID), or ends in
// an underscore/hyphen-separated "id" (complaint_id, complaint-id).
func isIDKey(k string) bool {
	if strings.EqualFold(k, "id") {
		return true
	}
	if strings.HasSuffix(k, "Id") || strings.HasSuffix(k, "ID") {
		return true
	}
	lk := strings.ToLower(k)
	return strings.HasSuffix(lk, "_id") || strings.HasSuffix(lk, "-id")
}
