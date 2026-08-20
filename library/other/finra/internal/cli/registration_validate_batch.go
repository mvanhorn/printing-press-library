// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/cliutil"
	"github.com/spf13/cobra"
)

const registrationValidateBatchConcurrency = 5

// maxRegistrationValidateBatchFileBytes bounds the size of a --file input.
// A CRD batch file is a plain list of numeric CRDs (or a small CSV column of
// them); even a worst-case list of many thousands of CRDs stays well under
// this, so a file above it is almost certainly the wrong file rather than a
// legitimately huge batch, and shouldn't be loaded fully into memory.
const maxRegistrationValidateBatchFileBytes = 10 * 1024 * 1024 // 10 MB

// pp:data-source live
func newNovelRegistrationValidateBatchCmd(flags *rootFlags) *cobra.Command {
	var flagFile string

	cmd := &cobra.Command{
		Use:   "validate-batch",
		Short: "Validate many CRDs from a file in one call instead of checking them one at a time.",
		Long: "Validate many CRDs from a file in one call instead of checking them one at a time.\n\n" +
			"--file accepts either a plain newline-delimited list of CRDs, or a CSV file with a header row\n" +
			"containing a \"crd\" column (case-insensitive).\n\n" +
			"Requires a FINRA credential entitled for registration data — a basic-tier credential will\n" +
			"receive a 403 with a clear permission-denied message.",
		Example:     "--file crds.csv --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--file=/tmp/finra-happy-crds.txt", "pp:requires-tier": "entitled"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would validate CRDs listed in %s\n", flagFile)
				return nil
			}
			if strings.TrimSpace(flagFile) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required"))
			}

			info, err := os.Stat(flagFile)
			if err != nil {
				return usageErr(fmt.Errorf("reading --file %q: %w", flagFile, err))
			}
			if info.Size() > maxRegistrationValidateBatchFileBytes {
				return usageErr(fmt.Errorf("--file %q is %d bytes, which is too large for a CRD batch file (max %d bytes); split it into smaller batches", flagFile, info.Size(), maxRegistrationValidateBatchFileBytes))
			}
			raw, err := os.ReadFile(flagFile) // #nosec G304 -- flagFile is the CLI operator's own --file argument, not attacker-controlled input
			if err != nil {
				return usageErr(fmt.Errorf("reading --file %q: %w", flagFile, err))
			}
			crds, err := parseCRDFile(raw)
			if err != nil {
				return usageErr(fmt.Errorf("parsing --file %q: %w", flagFile, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			results, _ := cliutil.FanoutRun(ctx, crds,
				func(crd string) string { return crd },
				func(fctx context.Context, crd string) (crdValidationResult, error) {
					return validateOneCRD(fctx, c, crd), nil
				},
				cliutil.WithConcurrency(registrationValidateBatchConcurrency),
			)

			view := registrationValidateBatchView{Total: len(crds)}
			bySource := map[string]crdValidationResult{}
			for _, r := range results {
				bySource[r.Source] = r.Value
			}
			for _, crd := range crds {
				res := bySource[crd]
				view.Results = append(view.Results, res)
				switch res.Status {
				case "valid":
					view.Valid++
				case "not_found":
					view.NotFound++
				default:
					view.Errors++
				}
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "total=%d valid=%d not_found=%d errors=%d\n", view.Total, view.Valid, view.NotFound, view.Errors)
				for _, r := range view.Results {
					if r.Status != "valid" {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s%s\n", r.CRD, r.Status, formatDetail(r.Detail))
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Path to a file of CRDs to validate: one per line, or CSV with a 'crd' column (required)")
	return cmd
}

func formatDetail(detail string) string {
	if detail == "" {
		return ""
	}
	return ": " + detail
}

type crdValidationResult struct {
	CRD    string `json:"crd"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type registrationValidateBatchView struct {
	Total    int                   `json:"total"`
	Valid    int                   `json:"valid"`
	NotFound int                   `json:"not_found"`
	Errors   int                   `json:"errors"`
	Results  []crdValidationResult `json:"results"`
}

// parseCRDFile parses either a CSV file with a header row containing a
// "crd" column (case-insensitive), or a plain newline-delimited list of
// CRDs. Blank lines are skipped and values are trimmed.
func parseCRDFile(raw []byte) ([]string, error) {
	text := string(raw)
	firstLine := text
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		firstLine = text[:idx]
	}
	if firstLineHasCRDColumn(firstLine) {
		return parseCRDCSV(text)
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

// firstLineHasCRDColumn reports whether firstLine looks like a CSV header
// row that should route into CSV parsing. Two cases route to CSV: (a) an
// exact "crd" column (case-insensitive, comma-split), or (b) a comma-
// separated line that merely contains "crd" as a substring of some field
// (e.g. "name,crd_number") — kept for a clear "no exact crd column found"
// error instead of silently mis-parsing an evidently CSV-shaped header. A
// line with no comma is never routed to CSV: a plain newline-delimited
// batch's first CRD value is a single bare token, and a bare substring
// check against it (the prior implementation) could misroute a numeric
// or comment token that happens to contain the letters "crd".
func firstLineHasCRDColumn(firstLine string) bool {
	fields := strings.Split(firstLine, ",")
	if len(fields) < 2 {
		return false
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), "crd") {
			return true
		}
	}
	return false
}

func parseCRDCSV(text string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	crdCol := -1
	for i, h := range rows[0] {
		if strings.EqualFold(strings.TrimSpace(h), "crd") {
			crdCol = i
			break
		}
	}
	if crdCol == -1 {
		return nil, fmt.Errorf("no 'crd' column found in CSV header: %v", rows[0])
	}
	var out []string
	for _, row := range rows[1:] {
		if crdCol >= len(row) {
			continue
		}
		v := strings.TrimSpace(row[crdCol])
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// validateOneCRD checks a single CRD via the confirmed GET-by-id pattern.
// It never returns an error itself — per-CRD outcomes (valid, not_found,
// error) are captured in the result so one bad CRD never aborts the batch.
func validateOneCRD(ctx context.Context, c *client.Client, crd string) crdValidationResult {
	path := "/data/group/{group}/name/{name}/id/{id}"
	path = replacePathParam(path, "group", "REGISTRATION")
	path = replacePathParam(path, "name", "REGISTRATIONVALIDATIONINDIVIDUAL")
	path = replacePathParam(path, "id", crd)

	_, err := c.Get(ctx, path, nil)
	if err == nil {
		return crdValidationResult{CRD: crd, Status: "valid"}
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		return crdValidationResult{CRD: crd, Status: "not_found"}
	}
	return crdValidationResult{CRD: crd, Status: "error", Detail: err.Error()}
}
