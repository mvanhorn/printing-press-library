// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newValidateCmd(flags *rootFlags) *cobra.Command {
	var comune, foglio, particella, sezione string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Parse-only syntax check for an Italian cadastral reference. No upstream call.",
		Long: "Validates the shape of a cadastral reference without hitting any API:\n" +
			"  - comune: 4-character codice belfiore (1 uppercase letter + 3 alphanumeric, e.g. H501)\n" +
			"  - foglio: numeric (zero-padding allowed)\n" +
			"  - particella: numeric or alphanumeric (1+ chars)\n" +
			"  - sezione: optional single uppercase letter\n\n" +
			"Useful as a pre-flight guard in form-style flows or batch imports — invalid input never burns an API call.",
		Example:     "  catasto-pp-cli validate --comune H501 --foglio 508 --particella B --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if comune == "" && foglio == "" && particella == "" && sezione == "" {
				return cmd.Help()
			}
			report := validateReport(comune, foglio, particella, sezione)
			data, _ := json.Marshal(report)
			if !report.Valid {
				if err := printOutputWithFlags(cmd.OutOrStdout(), data, flags); err != nil {
					return err
				}
				return usageErr(fmt.Errorf("invalid cadastral reference: %s", strings.Join(report.Errors, "; ")))
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&comune, "comune", "", "Codice belfiore of the comune (4 chars, e.g. H501).")
	cmd.Flags().StringVar(&foglio, "foglio", "", "Map sheet number (numeric).")
	cmd.Flags().StringVar(&particella, "particella", "", "Parcel number (numeric or alphanumeric).")
	cmd.Flags().StringVar(&sezione, "sezione", "", "Cadastral section (optional, single uppercase letter).")
	return cmd
}

type validateReportT struct {
	Valid      bool     `json:"valid"`
	Comune     string   `json:"comune"`
	Foglio     string   `json:"foglio"`
	Particella string   `json:"particella"`
	Sezione    string   `json:"sezione"`
	Errors     []string `json:"errors,omitempty"`
}

func validateReport(comune, foglio, particella, sezione string) validateReportT {
	out := validateReportT{
		Comune:     strings.ToUpper(strings.TrimSpace(comune)),
		Foglio:     strings.TrimSpace(foglio),
		Particella: strings.TrimSpace(particella),
		Sezione:    strings.ToUpper(strings.TrimSpace(sezione)),
	}
	var errs []string
	if out.Comune == "" {
		errs = append(errs, "comune is required")
	} else if !isCodiceBelfiore(out.Comune) {
		errs = append(errs, fmt.Sprintf("comune %q is not a valid codice belfiore (expected 4 chars: 1 letter + 3 alphanumeric)", out.Comune))
	}
	if out.Foglio == "" {
		errs = append(errs, "foglio is required")
	} else if !isNumeric(out.Foglio) {
		errs = append(errs, fmt.Sprintf("foglio %q is not numeric", out.Foglio))
	}
	if out.Particella == "" {
		errs = append(errs, "particella is required")
	} else if !isAlnum(out.Particella) {
		errs = append(errs, fmt.Sprintf("particella %q must be alphanumeric", out.Particella))
	}
	if out.Sezione != "" {
		if len(out.Sezione) != 1 || !(out.Sezione[0] >= 'A' && out.Sezione[0] <= 'Z') {
			errs = append(errs, fmt.Sprintf("sezione %q must be a single uppercase letter", out.Sezione))
		}
	}
	out.Errors = errs
	out.Valid = len(errs) == 0
	return out
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isAlnum(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return false
		}
	}
	return true
}
