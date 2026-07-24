// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/ti/internal/ticompliance"
)

// complianceView is the lane-shaped envelope: verdicts + optional evidence
// artifact paths in one JSON object.
type complianceView struct {
	ticompliance.OrderableCompliance
	CoC []ticompliance.DownloadedStatement `json:"coc_statements,omitempty"`
}

func newNovelPartComplianceCmd(flags *rootFlags) *cobra.Command {
	var flagCocDir string

	cmd := &cobra.Command{
		Use:   "compliance <opn>",
		Short: "Anonymous RoHS/REACH verdicts for one orderable part number, optionally bundled with TI's signed statement PDFs.",
		Long: strings.TrimSpace(`
Resolves an orderable part number (e.g. TUSB320RWBR) to its generic's product
page, reads the server-rendered JSON-LD, and emits TI's raw compliance
vocabulary verbatim (rohs_compliance, reach_compliance, MSL, packaging).
No login, no browser — plain HTTP. With --coc-dir it also downloads the
applicable signed blanket statement PDFs as evidence artifacts.

Fails closed: exit non-zero when the part cannot be resolved or when neither
RoHS nor REACH is present. For per-part exemptions, SVHC detail, or FMD XML
use 'part ratings' / 'part fmd' (login-gated, cookie snapshot required).`),
		Example: strings.Trim(`
  ti-pp-cli part compliance TUSB320RWBR --json
  ti-pp-cli part compliance TUSB320RWBR --json --coc-dir ./evidence
  ti-pp-cli part compliance TUSB320RWBR --agent --select rohs_compliance,reach_compliance`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the part's product page and read RoHS/REACH")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an orderable part number is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			res, err := ticompliance.ResolveCompliance(ctx, args[0])
			if err != nil {
				return err
			}
			view := complianceView{OrderableCompliance: *res}
			if flagCocDir != "" {
				coc, err := ticompliance.DownloadStatements(ctx, flagCocDir, nil)
				if err != nil {
					return fmt.Errorf("downloading CoC statements: %w", err)
				}
				view.CoC = coc
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCocDir, "coc-dir", "", "directory to download TI's signed blanket statement PDFs into (bundled as coc_statements)")
	return cmd
}
