// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: value a multi-program balance portfolio from stdin or a file.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelPortfolioCmd(flags *rootFlags) *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "portfolio [balance...]",
		Short: "Value many loyalty balances at once and total them in USD",
		Long: strings.TrimSpace(`
Value a portfolio of loyalty balances against The Points Guy's valuations and
total them. Provide balances as "Program = points" entries (also accepts ":" or
tab separators) as positional arguments, via stdin, or via --file. Unmatched
programs are reported separately and excluded from the total.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli portfolio "Amex Membership Rewards=120000" "United MileagePlus=50000"
  printf 'Amex MR=120000\nUnited=50000\n' | thepointsguy-pp-cli portfolio --agent
  thepointsguy-pp-cli portfolio --file balances.txt
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would value a balance portfolio")
				return nil
			}
			// Read balances (after dryRunOK, per verify contract). Sources, in
			// order: --file, positional args (each an entry; also split on \n),
			// then stdin.
			var lines []string
			switch {
			case file != "":
				// #nosec G304 -- `file` is the user's own --file argument; reading a caller-chosen path is the intended behavior of this flag.
				b, err := os.ReadFile(file)
				if err != nil {
					return usageErr(fmt.Errorf("reading --file: %w", err))
				}
				lines = strings.Split(string(b), "\n")
			case len(args) > 0:
				for _, a := range args {
					// Accept literal or escaped newlines within a single arg.
					a = strings.ReplaceAll(a, "\\n", "\n")
					lines = append(lines, strings.Split(a, "\n")...)
				}
			case stdinIsTerminal():
				// Nothing piped, no file, no args: show help rather than block.
				return cmd.Help()
			default:
				sc := bufio.NewScanner(cmd.InOrStdin())
				sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				for sc.Scan() {
					lines = append(lines, sc.Text())
				}
			}

			type balance struct {
				program string
				points  float64
			}
			var balances []balance
			var parseErrs []string
			for _, ln := range lines {
				ln = strings.TrimSpace(ln)
				if ln == "" || strings.HasPrefix(ln, "#") {
					continue
				}
				prog, ptsStr, ok := splitBalanceLine(ln)
				if !ok {
					parseErrs = append(parseErrs, ln)
					continue
				}
				pts, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(ptsStr), ",", ""), 64)
				if err != nil {
					parseErrs = append(parseErrs, ln)
					continue
				}
				balances = append(balances, balance{program: strings.TrimSpace(prog), points: pts})
			}
			if len(balances) == 0 {
				if flags.asJSON || flags.agent {
					return emitJSON(cmd, flags, map[string]any{"error": "no balances provided", "unparsed_lines": parseErrs})
				}
				return usageErr(fmt.Errorf("no balances provided; supply \"Program = points\" lines via stdin or --file"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := &tpgClientCtx{client: newTPGClient(flags), ctx: ctx}
			byProg, month, err := currentValuations(cmd, flags, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			type entry struct {
				Program       string  `json:"program"`
				Points        float64 `json:"points"`
				CentsPerPoint float64 `json:"cents_per_point"`
				ValueUSD      float64 `json:"value_usd"`
			}
			entries := make([]entry, 0, len(balances))
			unmatched := make([]string, 0)
			var total float64
			for _, b := range balances {
				v, _, ok := resolveValuation(byProg, b.program)
				if !ok {
					unmatched = append(unmatched, b.program)
					continue
				}
				val := dollarsFromPoints(b.points, v.CentsPerPoint)
				total += val
				entries = append(entries, entry{v.Program, b.points, v.CentsPerPoint, round2(val)})
			}
			if len(unmatched) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d program(s) not matched and excluded from the total: %s\n",
					len(unmatched), strings.Join(unmatched, ", "))
			}

			view := struct {
				Month     string   `json:"month"`
				Holdings  []entry  `json:"holdings"`
				TotalUSD  float64  `json:"total_usd"`
				Unmatched []string `json:"unmatched_programs"`
				Unparsed  []string `json:"unparsed_lines,omitempty"`
			}{month, entries, round2(total), unmatched, parseErrs}

			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, view)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "PROGRAM\tPOINTS\tCENTS/PT\tVALUE\n")
			for _, e := range entries {
				fmt.Fprintf(w, "%s\t%.0f\t%.2f\t$%.2f\n", e.Program, e.Points, e.CentsPerPoint, e.ValueUSD)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal estimated value: $%.2f (TPG %s)\n", total, month)
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Read balances from a file instead of stdin")
	return cmd
}

// stdinIsTerminal reports whether stdin is an interactive terminal (not a pipe).
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// splitBalanceLine parses "Program <sep> points" where sep is '=', ':', or a tab.
func splitBalanceLine(ln string) (program, points string, ok bool) {
	for _, sep := range []string{"=", "\t", ":"} {
		if i := strings.LastIndex(ln, sep); i > 0 {
			return ln[:i], ln[i+len(sep):], true
		}
	}
	// Fallback: "Program <spaces> 12345" — split on last whitespace run.
	fields := strings.Fields(ln)
	if len(fields) >= 2 {
		last := fields[len(fields)-1]
		if _, err := strconv.ParseFloat(strings.ReplaceAll(last, ",", ""), 64); err == nil {
			return strings.Join(fields[:len(fields)-1], " "), last, true
		}
	}
	return "", "", false
}
