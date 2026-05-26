package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newExportCmd() *cobra.Command {
	var formatF, query, output string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a SQL query's result to a file (CSV or JSON)",
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			if err := guardReadOnlySQL(query); err != nil {
				return err
			}
			ext := strings.ToLower(formatF)
			if ext != "csv" && ext != "json" {
				return fmt.Errorf("--format must be csv or json (got %q)", formatF)
			}
			if output == "" {
				output = fmt.Sprintf("defillama-export-%s.%s", time.Now().Format("20060102-150405"), ext)
			}
			abs, err := filepath.Abs(output)
			if err != nil {
				return err
			}
			f, err := os.Create(abs)
			if err != nil {
				return err
			}
			defer f.Close()

			origStdout := os.Stdout
			// runReadOnlySQL writes to os.Stdout. Swap it so the file gets the bytes.
			os.Stdout = f
			defer func() { os.Stdout = origStdout }()

			m := format.ModeCSV
			if ext == "json" {
				m = format.ModeJSON
			}
			if err := runReadOnlySQL(cx, query, m); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", abs)
			fmt.Fprintln(origStdout, abs)
			return nil
		}),
	}
	cmd.Flags().StringVar(&formatF, "format", "csv", "csv or json")
	cmd.Flags().StringVar(&query, "query", "", "SELECT statement")
	cmd.Flags().StringVar(&output, "output", "", "output path (default: defillama-export-<ts>.<ext>)")
	return cmd
}
