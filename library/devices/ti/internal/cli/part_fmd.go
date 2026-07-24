// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/ti/internal/ticompliance"
)

func newNovelPartFMDCmd(flags *rootFlags) *cobra.Command {
	var flagCookies string
	var flagOutDir string

	cmd := &cobra.Command{
		Use:   "fmd <opn>",
		Short: "Fetch the IPC-1752A Class-D FMD XML for a part via the deposited myTI cookie snapshot (login-gated).",
		Long: strings.TrimSpace(`
Resolves the part's Material Content report page in a cookie-injected
Browserbase session and drives the page's own standards-download form with
IPC-1752A Class D selected (standardsId=3). The XML is validated (HTML and
error pages rejected) and written to --out-dir as TI_<OPN>_FMD_1752A_ClassD.xml.`),
		Example: strings.Trim(`
  ti-pp-cli part fmd TUSB320RWBR --cookies ./ti-cookies.json --out-dir ./evidence --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:write-positionals": ""},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would inject cookies and fetch the IPC-1752A Class-D XML")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an orderable part number is required"))
			}
			path := resolveCookiesPath(flagCookies)
			if path == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--cookies <file> (or TI_COOKIES_FILE) is required"))
			}
			cookies, err := ticompliance.LoadCookies(path)
			if err != nil {
				return err
			}
			outDir := flagOutDir
			if outDir == "" {
				outDir = "."
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			opn := strings.ToUpper(strings.TrimSpace(args[0]))
			if err := ticompliance.ValidateOPN(opn); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			b, err := ticompliance.NewBrowser(false)
			if err != nil {
				return err
			}
			defer b.Close(ctx)
			if err := b.InjectCookies(ctx, cookies); err != nil {
				return err
			}
			xmlPath := filepath.Join(outDir, "TI_"+opn+"_FMD_1752A_ClassD.xml")
			res, err := b.AcquireFMD(ctx, opn, xmlPath)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagCookies, "cookies", "", "path to the deposited myTI cookie snapshot JSON (fallback: TI_COOKIES_FILE)")
	cmd.Flags().StringVar(&flagOutDir, "out-dir", "", "directory to write the FMD XML into (default: current directory)")
	return cmd
}
