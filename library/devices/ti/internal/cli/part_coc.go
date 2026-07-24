// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/ti/internal/ticompliance"
)

func newNovelPartCoCCmd(flags *rootFlags) *cobra.Command {
	var flagOutDir string
	var flagDocs string

	cmd := &cobra.Command{
		Use:   "coc",
		Short: "Download TI's signed blanket compliance statement PDFs (RoHS, RoHS exemptions, REACH, low-halogen, IEC 62474).",
		Long: strings.TrimSpace(`
TI is a manufacturer that publishes signed company-wide compliance statements;
these PDFs are the evidence artifacts for the extraction lane (manufacturer-CoC
pattern — no page render needed). Anonymous download, no login.`),
		Example: strings.Trim(`
  ti-pp-cli part coc --out-dir ./evidence --json
  ti-pp-cli part coc --out-dir ./evidence --docs szzq088,szzq087`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would download TI's signed statement PDFs")
				return nil
			}
			if flagOutDir == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--out-dir is required"))
			}
			var docs []string
			if flagDocs != "" {
				docs = strings.Split(flagDocs, ",")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			res, err := ticompliance.DownloadStatements(ctx, flagOutDir, docs)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagOutDir, "out-dir", "", "directory to download the statement PDFs into")
	cmd.Flags().StringVar(&flagDocs, "docs", "", "comma-separated TI literature numbers to fetch (default: all five)")
	return cmd
}
