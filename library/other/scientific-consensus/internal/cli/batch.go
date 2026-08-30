// Hand-authored novel command: batch consensus over claim files. Not generated.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// batchItem is one claim's outcome. Exactly one of Result or Error is set.
type batchItem struct {
	Claim  string           `json:"claim"`
	Result *consensusOutput `json:"result,omitempty"`
	Error  string           `json:"error,omitempty"`
}

// batchSummary carries run totals for the human-readable table; the machine
// (--json) output is the flat []batchItem array.
type batchSummary struct {
	Total  int
	OK     int
	Failed int
	Items  []batchItem
}

func newBatchCmd(flags *rootFlags) *cobra.Command {
	var limit, yearFrom int
	var enrich bool

	cmd := &cobra.Command{
		Use:   "batch <file|glob>...",
		Short: "Run consensus analysis on many claims read from file(s), one per line",
		Long: "Read claims/queries one per line from the given file(s) or glob(s), run the\n" +
			"same consensus analysis as the `consensus` command on each, and emit a\n" +
			"combined result. Blank lines and lines starting with # are skipped. A failure\n" +
			"on one line is recorded against that line and does not abort the batch. All\n" +
			"claims share one client, so the existing cache and rate limit are respected.",
		Example: "  scientific-consensus-pp-cli batch claims.txt\n" +
			"  scientific-consensus-pp-cli batch ./topics/*.txt --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run batch consensus over the given files")
				return nil
			}
			if len(args) == 0 {
				return usageErr(errors.New("provide at least one file path or glob"))
			}

			claims, err := readBatchClaims(args)
			if err != nil {
				return err
			}
			if len(claims) == 0 {
				return usageErr(errors.New("no claims found (every line was blank or a # comment)"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			sum := batchSummary{Total: len(claims)}
			prog := newProgress(flags, "analyzed", len(claims))
			for i, claim := range claims {
				out, aerr := computeConsensus(ctx, c, claim, limit, yearFrom, enrich)
				item := batchItem{Claim: claim}
				if aerr != nil {
					// Per-line failure is recorded, not fatal — one bad claim
					// must not abort the rest of the batch.
					item.Error = aerr.Error()
					sum.Failed++
				} else {
					res := out
					item.Result = &res
					sum.OK++
				}
				sum.Items = append(sum.Items, item)
				prog.update(i + 1)
			}
			prog.done()

			// Machine formats (--json etc.) get the flat per-claim array; the
			// human view is a summary table built from the same items.
			return emit(cmd, flags, sum.Items, func(w io.Writer) { renderBatch(w, sum) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 40, "works to analyze per claim (max 200)")
	cmd.Flags().IntVar(&yearFrom, "year-from", 0, "only include works published from this year onward")
	cmd.Flags().BoolVar(&enrich, "enrich", true, "enrich study-design classification with PubMed publication types")
	return cmd
}

// readBatchClaims expands each arg as a glob (falling back to a literal path
// when it matches nothing, so a missing filename errors clearly) and returns the
// trimmed claim lines across all files, de-duplicating files. Blank lines and
// lines starting with # are skipped.
func readBatchClaims(args []string) ([]string, error) {
	var files []string
	seenFile := map[string]bool{}
	for _, a := range args {
		matches, err := filepath.Glob(a)
		if err != nil {
			return nil, usageErr(fmt.Errorf("invalid path pattern %q: %w", a, err))
		}
		if len(matches) == 0 {
			matches = []string{a}
		}
		for _, f := range matches {
			if !seenFile[f] {
				seenFile[f] = true
				files = append(files, f)
			}
		}
	}

	var claims []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, configErr(fmt.Errorf("reading %q: %w", f, err))
		}
		claims = append(claims, parseClaimLines(string(data))...)
	}
	return claims, nil
}

// parseClaimLines splits file content into claim lines: trims surrounding
// whitespace, and skips blank lines and # comments.
func parseClaimLines(content string) []string {
	var claims []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		claims = append(claims, line)
	}
	return claims
}

func renderBatch(w io.Writer, s batchSummary) {
	fmt.Fprintf(w, "Batch consensus: %d claims (%d ok, %d failed)\n\n", s.Total, s.OK, s.Failed)
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "CLAIM\tVERDICT\tSCORE\tCONF\tSTUDIES\tSTATUS")
	for _, it := range s.Items {
		if it.Error != "" {
			fmt.Fprintf(tw, "%s\t-\t-\t-\t-\terror: %s\n", truncate(it.Claim, 50), truncate(it.Error, 40))
			continue
		}
		r := it.Result
		fmt.Fprintf(tw, "%s\t%s\t%+.2f\t%.0f%%\t%d\tok\n",
			truncate(it.Claim, 50), r.Verdict, r.ConsensusScore, r.Confidence*100, r.StudyCount)
	}
	_ = tw.Flush()
}
