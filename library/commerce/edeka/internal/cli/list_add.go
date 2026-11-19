// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

func newNovelListAddCmd(flags *rootFlags) *cobra.Command {
	var file, market string
	cmd := &cobra.Command{
		Use:   "add [lines...]",
		Short: "Add many free-text lines with quantities at once and see what matched.",
		Long: "Use this command to add several free-text lines at once. Do NOT use it for one item; use 'basket add'.\n\n" +
			"Each argument (or each line of --file, '-' for stdin) is one shopping-list line such as \"milch\", \"2x butter\"\n" +
			"or \"kaffee x2\". Every line is matched against the market's search and the best match is added. Lines\n" +
			"without a match are reported and skipped. --dry-run shows the matches without changing the basket.",
		Example:     "  edeka-pp-cli list add milch butter '2x kaffee' --dry-run\n  edeka-pp-cli list add --file einkaufsliste.txt --json",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "lines=milch", "pp:live-happy-path": "true", "pp:verified-by-proof": "list-add-lifecycle.md", "mcp:read-flags": "file"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if done, err := verifyDryRun(cmd, flags, "list add"); done {
				return err
			}
			lines := append([]string{}, args...)
			if file != "" {
				if cliutil.IsMCPChildEnv() && file != "-" {
					return usageErr(errors.New("--file is not available through MCP; pass the lines as arguments"))
				}
				var r io.Reader
				if file == "-" {
					r = cmd.InOrStdin()
				} else {
					f, err := os.Open(filepath.Clean(file))
					if err != nil {
						return usageErr(err)
					}
					defer f.Close()
					r = f
				}
				sc := bufio.NewScanner(r)
				for sc.Scan() {
					if l := strings.TrimSpace(strings.TrimLeft(sc.Text(), "-*• ")); l != "" {
						lines = append(lines, l)
					}
				}
				if err := sc.Err(); err != nil {
					return usageErr(fmt.Errorf("reading the list: %w", err))
				}
			}
			if len(lines) == 0 {
				return cmd.Help()
			}
			if len(lines) > 60 {
				return usageErr(fmt.Errorf("at most 60 lines per call (got %d)", len(lines)))
			}
			sh, err := openShop(cmd, flags, market)
			if err != nil {
				return err
			}
			var results []*addResult
			added, missed := 0, 0
			for _, line := range lines {
				qty, q := edeka.ParseListLine(line)
				res, err := addToBasket(cmd.Context(), sh, q, qty, 0, flags.dryRun)
				if err != nil {
					var ce *cliError
					if errors.As(err, &ce) && (ce.code == 3 || ce.code == 2) {
						results = append(results, &addResult{Query: q, Quantity: qty, Error: ce.err.Error()})
						missed++
						continue
					}
					_ = sh.Persist()
					// report what was already added before the failure, so a retry
					// does not add those lines twice
					partial := map[string]any{"lines": len(lines), "matched": added, "not_matched": missed, "dry_run": flags.dryRun, "results": results, "stopped_at": line, "error": err.Error()}
					return emitThen(cmd, flags, partial, err)
				}
				results = append(results, res)
				added++
			}
			_ = sh.Persist()
			out := map[string]any{"lines": len(lines), "matched": added, "not_matched": missed, "dry_run": flags.dryRun, "results": results}
			return emit(cmd, flags, out, func(w io.Writer) {
				for _, r := range results {
					printAddResult(w, r)
				}
				fmt.Fprintf(w, "%d of %d lines matched.\n", added, len(lines))
			})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Read one line per item from this file ('-' for stdin)")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}
