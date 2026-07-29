// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored (was a generator stub). Credit-cost preflight: joins the
// documented per-kind/version cost table with the live /point/info balance so
// users see would-spend vs remaining before any credit is spent.
//
// pp:data-source live

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelCostCmd(flags *rootFlags) *cobra.Command {
	var style, drawHD, batchDir string
	var restoreFirst bool

	cmd := &cobra.Command{
		Use:   "cost [depthmap|stl|threed|restore]",
		Short: "Estimate the credit cost of a draw or batch and compare it to your balance",
		Long: strings.Trim(`
Estimate how many credits a draw (or a whole --batch directory) would cost and
compare it against your live credit balance — before spending anything.

Costs: depthmap 10 (style=pro 2k 15; style=pro + draw-hd=4k 30); restore 2;
threed 10; stl 3. Reads are free. For where credits already went, use 'analytics'.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli cost depthmap
  sculptok-pp-cli cost depthmap --style pro --draw-hd 4k
  sculptok-pp-cli cost stl --batch ./photos`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would estimate credit cost")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide a draw kind: depthmap, stl, threed, or restore"))
			}
			kind := args[0]
			if _, ok := drawSpecs[kind]; !ok {
				return usageErr(fmt.Errorf("unknown kind %q (use depthmap, stl, threed, or restore)", kind))
			}

			count := 1
			if batchDir != "" {
				entries, err := os.ReadDir(batchDir)
				if err != nil {
					return fmt.Errorf("reading batch dir: %w", err)
				}
				count = 0
				for _, e := range entries {
					if !e.IsDir() && imageExts[strings.ToLower(filepath.Ext(e.Name()))] {
						count++
					}
				}
				if count == 0 {
					return fmt.Errorf("no images found in %s", batchDir)
				}
			}

			per := drawCost(kind, style, drawHD)
			if restoreFirst && kind != "restore" {
				per += drawCost("restore", "", "")
			}
			total := per * count

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := map[string]any{
				"kind":            kind,
				"images":          count,
				"creditsPerImage": per,
				"creditsTotal":    total,
			}
			c, err := newSculptokClient(flags)
			if err == nil && c.HasKey() {
				if bal, berr := c.Balance(ctx); berr == nil {
					view["balance"] = bal
					view["balanceAfter"] = bal - total
					view["affordable"] = bal >= total
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s: %d credits/image x %d = %d credits\n", kind, per, count, total)
			if bal, ok := view["balance"].(int); ok {
				status := "OK"
				if bal < total {
					status = "INSUFFICIENT"
				}
				fmt.Fprintf(w, "balance: %d  ->  after: %d  (%s)\n", bal, bal-total, status)
			} else {
				fmt.Fprintln(w, "balance: unavailable (set SCULPTOK_API_KEY to compare)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&style, "style", "", "Depth-map style for cost (pro raises depthmap cost)")
	cmd.Flags().StringVar(&drawHD, "draw-hd", "", "Resolution for cost (4k with --style pro = 30 credits)")
	cmd.Flags().BoolVar(&restoreFirst, "restore-first", false, "Include a restore pre-pass (2 credits) in the estimate")
	cmd.Flags().StringVar(&batchDir, "batch", "", "Estimate for every image in this directory")
	return cmd
}
