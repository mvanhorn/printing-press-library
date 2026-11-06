// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelFoundingCmd(flags *rootFlags) *cobra.Command {
	var target int
	var paywall, dbPath string
	cmd := &cobra.Command{
		Use:   "founding",
		Short: "Answer whether at least N people paid with GO or NO-GO and the count.",
		Long:  "Use this command for a GO/NO-GO verdict on whether at least N people have paid. Do NOT use this command to list payers or remaining capacity; use 'seats' instead.\n\nExit codes: 0 GO, 8 NO-GO, 2 usage error",
		Example: strings.Trim(`
  circle-pp-cli founding --go-no-go 10 --json
  circle-pp-cli founding --go-no-go 10 --paywall 1000969 --data-source live
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,8", "pp:happy-args": "--go-no-go=1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "founding")
			}
			if target <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--go-no-go is required and must be greater than zero"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("circle-pp-cli")
			}
			payers, source, err := loadCirclePayers(ctx, cmd, flags, dbPath, paywall)
			if err != nil {
				return err
			}
			result := foundingVerdict(len(payers), target, source)
			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				err = printJSONFiltered(out, result, flags)
			} else {
				_, err = fmt.Fprintf(out, "%s: %d paid / %d target; %d short (%s)\n", result.Verdict, result.Paid, result.Target, result.ShortBy, source)
			}
			if err != nil {
				return err
			}
			if result.Verdict == "NO-GO" {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &cliError{code: 8, err: fmt.Errorf("NO-GO")}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&target, "go-no-go", 0, "Required number of distinct payers for a GO verdict; must be greater than zero.")
	cmd.Flags().StringVar(&paywall, "paywall", "", "Limit payers to this paywall ID.")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local synced store (default: the CLI's store)")
	return cmd
}

type foundingResult struct {
	Verdict string `json:"verdict"`
	Paid    int    `json:"paid"`
	Target  int    `json:"target"`
	ShortBy int    `json:"short_by"`
	Source  string `json:"source"`
}

func foundingVerdict(paid, target int, source string) foundingResult {
	verdict := "NO-GO"
	if paid >= target {
		verdict = "GO"
	}
	return foundingResult{verdict, paid, target, max(target-paid, 0), source}
}
