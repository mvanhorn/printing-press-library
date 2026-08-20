package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/ucp"
	"github.com/spf13/cobra"
)

func newCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "check <domain>",
		Short:   "Fetch a domain's /.well-known/ucp and grade its manifest (A-F)",
		Example: `  ucp-pp-cli check checkout.coffeecircle.com --json`,
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			domain := args[0]
			ctx := cmd.Context()

			m, err := ucp.FetchManifest(ctx, domain)
			if err != nil {
				return fmt.Errorf("fetch manifest: %w", err)
			}
			report := ucp.Validate(m)

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			// Human table output
			w := cmd.OutOrStdout()
			tw := newTabWriter(w)
			fmt.Fprintf(tw, "Domain\t%s\n", domain)
			fmt.Fprintf(tw, "Version\t%s\n", report.Version)
			fmt.Fprintf(tw, "Grade\t%s\n", report.Grade)
			fmt.Fprintf(tw, "Score\t%d/100\n", report.Score)
			fmt.Fprintf(tw, "Transports\t%s\n", strings.Join(report.Transports, ", "))

			capDisplay := capSummary(report.Capabilities)
			fmt.Fprintf(tw, "Capabilities\t%s\n", capDisplay)

			phDisplay := capSummary(report.PaymentHandlers)
			fmt.Fprintf(tw, "PaymentHandlers\t%s\n", phDisplay)

			if len(report.Errors) > 0 {
				fmt.Fprintf(tw, "Errors\t%s\n", strings.Join(report.Errors, "; "))
			}
			if len(report.Warnings) > 0 {
				fmt.Fprintf(tw, "Warnings\t%s\n", strings.Join(report.Warnings, "; "))
			}
			tw.Flush()

			// Exit codes based on grade
			switch report.Grade {
			case "D", "E":
				return &cliError{code: 3, err: fmt.Errorf("grade %s (score %d/100)", report.Grade, report.Score)}
			case "F":
				return &cliError{code: 4, err: fmt.Errorf("grade F — manifest has critical errors")}
			}
			return nil
		},
	}
	return cmd
}

func capSummary(caps []string) string {
	if len(caps) == 0 {
		return "(none)"
	}
	preview := caps
	suffix := ""
	if len(caps) > 3 {
		preview = caps[:3]
		suffix = fmt.Sprintf(" (+%d more)", len(caps)-3)
	}
	return fmt.Sprintf("%d — %s%s", len(caps), strings.Join(preview, ", "), suffix)
}
