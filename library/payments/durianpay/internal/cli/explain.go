// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local

// explainResult is the decoded SNAP response code, emitted as JSON with
// --json or rendered as a human table otherwise.
type explainResult struct {
	Code        string `json:"code"`
	HTTPCode    string `json:"http_status"`
	ServiceCode string `json:"service_code"`
	CaseCode    string `json:"case_code"`
	Message     string `json:"meaning"`
	Description string `json:"description"`
	Service     string `json:"service,omitempty"`
	ServicePath string `json:"service_path,omitempty"`
	Outcome     string `json:"outcome"`
	Fix         string `json:"likely_fix,omitempty"`
}

// splitSnapCode splits a 7-digit SNAP response code into its HTTP (3),
// service (2), and case (2) components.
func splitSnapCode(code string) (httpCode, serviceCode, caseCode string, err error) {
	code = strings.TrimSpace(code)
	if len(code) != 7 {
		return "", "", "", fmt.Errorf("invalid SNAP response code %q: expected 7 digits (HTTP+service+case, e.g. 4001801)", code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return "", "", "", fmt.Errorf("invalid SNAP response code %q: must be all digits", code)
		}
	}
	return code[:3], code[3:5], code[5:7], nil
}

// snapFixHint returns a likely-fix hint for the well-known failure cases.
func snapFixHint(httpCode, caseCode string) string {
	switch {
	case httpCode == "401" && caseCode == "01":
		return "Access token is invalid or expired. Mint a fresh B2B token: durianpay-pp-cli snap token --mint"
	case httpCode == "401" && caseCode == "03":
		return "Token missing from the request. Mint and attach a B2B token: durianpay-pp-cli snap token --mint"
	case httpCode == "401":
		return "Signature or credentials rejected. Inspect the exact string-to-sign and headers: durianpay-pp-cli snap sign --debug"
	case httpCode == "409":
		return "X-EXTERNAL-ID (or partnerReferenceNo) was reused. Generate a fresh unique X-EXTERNAL-ID per request."
	case httpCode == "404":
		return "Transaction or account not found. Verify the reference number / account number you sent."
	case httpCode == "400":
		return "Request validation failed. Check the named field's format and that all mandatory fields are present."
	case httpCode == "403":
		return "Transaction expired or the account is inactive. Re-create the transaction or use an active account."
	case httpCode == "500" || httpCode == "504":
		return "Durianpay-side error/timeout. Keep the transaction as processing and re-inquire status before retrying."
	}
	return ""
}

func buildExplainResult(code string) (explainResult, error) {
	httpCode, serviceCode, caseCode, err := splitSnapCode(code)
	if err != nil {
		return explainResult{}, err
	}
	c, ok := lookupSnapCase(httpCode, serviceCode, caseCode)
	if !ok {
		return explainResult{}, fmt.Errorf("unknown SNAP response code %q: no case matches HTTP %s / case %s (run 'durianpay-pp-cli explain' with no args for examples)", code, httpCode, caseCode)
	}
	res := explainResult{
		Code:        code,
		HTTPCode:    httpCode,
		ServiceCode: serviceCode,
		CaseCode:    caseCode,
		Message:     c.Message,
		Description: c.Description,
		Fix:         snapFixHint(httpCode, caseCode),
	}
	if c.Success {
		res.Outcome = "success"
	} else {
		res.Outcome = "error"
	}
	if svc, ok := lookupSnapService(serviceCode); ok {
		res.Service = svc.Name
		res.ServicePath = svc.Path
	} else {
		res.Service = "unknown service code " + serviceCode
	}
	return res, nil
}

func newNovelExplainCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "explain <code>",
		Short:       "Decode any SNAP response code offline — meaning, originating service, HTTP status, and the likely fix.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "4001801"},
		Long: `Decode a 7-digit SNAP response code offline.

A SNAP response code is HTTP code (3) + service code (2) + case code (2).
For example 2001800 = HTTP 200 + service 18 (Bank Transfer) + case 00.

This is a static, offline lookup — it never calls the API.`,
		Example: strings.TrimLeft(`
  durianpay-pp-cli explain 4001801
  durianpay-pp-cli explain 2001800
  durianpay-pp-cli explain 4091100 --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags != nil && flags.dataSource == "live" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("explain is an offline static lookup with no live equivalent; drop --data-source live"))
			}
			if dryRunOK(flags) {
				return nil
			}
			res, err := buildExplainResult(args[0])
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Code:        %s\n", res.Code)
			fmt.Fprintf(out, "HTTP status: %s\n", res.HTTPCode)
			fmt.Fprintf(out, "Service:     %s (code %s)\n", res.Service, res.ServiceCode)
			if res.ServicePath != "" {
				fmt.Fprintf(out, "Path:        %s\n", res.ServicePath)
			}
			fmt.Fprintf(out, "Outcome:     %s\n", res.Outcome)
			fmt.Fprintf(out, "Meaning:     %s\n", res.Message)
			fmt.Fprintf(out, "Description: %s\n", res.Description)
			if res.Fix != "" {
				fmt.Fprintf(out, "Likely fix:  %s\n", res.Fix)
			}
			return nil
		},
	}
	return cmd
}
