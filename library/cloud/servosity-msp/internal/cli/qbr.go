// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/qbrreport"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/store"
)

// newQBRCmd builds the QBR (Quarterly Business Review) generator. It
// assembles a single client's backup-section deck and renders it as
// Markdown, HTML, or PDF (via headless Chrome). The command is read-only
// against the local SQLite sync store; pass --json for the machine
// envelope.
//
// Verify-friendly RunE: validation of <company> happens inside RunE
// (not via cobra.Args) so --dry-run can short-circuit before any IO.
func newQBRCmd(flags *rootFlags) *cobra.Command {
	var quarter string
	var format string
	var outPath string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "qbr <company>",
		Short: "Generate a Quarterly Business Review backup report for one client",
		Long: `Assemble the backup section of a client's QBR — coverage map, job success
rate, restore tests, open issues, and storage trend — and render it as
Markdown, HTML, or PDF.

The <company> argument may be a numeric company ID or a name substring.
If a name substring matches more than one company the command fails with
the list of matches so you can disambiguate.

Reads from the local sync store. Run 'servosity-msp-cli sync' first.`,
		Example: `  # Current quarter, PDF to a file
  servosity-msp-cli qbr "Acme Co" --out acme-q1.pdf

  # Explicit quarter as Markdown to stdout
  servosity-msp-cli qbr 12345 --quarter 2026-Q1 --format md

  # Self-contained HTML (no charts, embeddable)
  servosity-msp-cli qbr "Acme" --format html --out acme.html`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// --dry-run short-circuits BEFORE arg or flag validation so the
			// verify probe can run `qbr --dry-run` (no positional, no flags)
			// and get a clean exit-0 instead of either a help dump or an
			// "--out is required" usage error. Verify probes rely on this
			// path returning nil with no side effects.
			if dryRunOK(flags) {
				return nil
			}

			// Verify-friendly: print help instead of failing when no args
			// arrive (the press's verify probe runs every command with no
			// args; failing on MinimumNArgs masks real bugs).
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("expected exactly one <company> argument, got %d", len(args)))
			}

			// Validate flags before any IO so usage errors surface cleanly.
			switch format {
			case "md", "html", "pdf":
				// ok
			default:
				return usageErr(fmt.Errorf("invalid --format %q: must be md, html, or pdf", format))
			}
			if quarter == "" {
				quarter = qbrreport.CurrentQuarter(time.Now())
			}
			if _, _, err := qbrreport.ParseQuarter(quarter); err != nil {
				return usageErr(err)
			}
			if (format == "html" || format == "pdf") && outPath == "" {
				return usageErr(fmt.Errorf("--out is required for --format %s", format))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("servosity-msp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'servosity-msp-cli sync' first.", err)
			}
			defer db.Close()

			company, err := qbrreport.LookupCompany(cmd.Context(), db.DB(), strings.TrimSpace(args[0]))
			if err != nil {
				var amb *qbrreport.AmbiguousError
				if errors.As(err, &amb) {
					var b strings.Builder
					fmt.Fprintf(&b, "Multiple companies matched %q:\n", amb.Arg)
					for _, m := range amb.Matches {
						fmt.Fprintf(&b, "  %d  %s\n", m.ID, m.Name)
					}
					fmt.Fprint(&b, "Pass a numeric company ID or a more specific name substring.")
					return usageErr(errors.New(b.String()))
				}
				return err
			}

			report, err := qbrreport.Assemble(cmd.Context(), db.DB(), company.ID, quarter)
			if err != nil {
				return fmt.Errorf("assembling QBR: %w", err)
			}

			// --json wins over --format: machine consumers get the struct.
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			switch format {
			case "md":
				w := cmd.OutOrStdout()
				if outPath != "" {
					f, ferr := os.Create(outPath)
					if ferr != nil {
						return fmt.Errorf("create --out: %w", ferr)
					}
					defer f.Close()
					w = f
				}
				return qbrreport.RenderMarkdown(report, w)
			case "html":
				f, ferr := os.Create(outPath)
				if ferr != nil {
					return fmt.Errorf("create --out: %w", ferr)
				}
				defer f.Close()
				return qbrreport.RenderHTML(report, f)
			case "pdf":
				return qbrreport.RenderPDF(report, outPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&quarter, "quarter", "", "Quarter window as YYYY-QN (default: current quarter)")
	cmd.Flags().StringVar(&format, "format", "pdf", "Output format: md, html, or pdf")
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path (required for html/pdf; optional for md)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servosity-msp-cli/data.db)")

	return cmd
}
