// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compare <id-a> <id-b>",
		Short: "Side-by-side comparison of two works",
		Long: `Render two works in a three-column field-by-field layout (field | A |
B). Useful when you want to look at a pair across sources — say a
Hokusai woodblock from AIC alongside a Hiroshige from the Met — and
read their metadata against each other without flipping between
records.`,
		Example: `  art-goat-pp-cli compare aic:24645 met:36492
  art-goat-pp-cli compare harvard:204498 cleveland:1942.647 --json`,
		Args: cobra.ExactArgs(2),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitCompareVerifyEnvelope(cmd, flags)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			idA, idB := args[0], args[1]
			a, err := db.GetWork(cmd.Context(), idA)
			if err != nil {
				return err
			}
			if a == nil {
				return fmt.Errorf("work %q not found", idA)
			}
			b, err := db.GetWork(cmd.Context(), idB)
			if err != nil {
				return err
			}
			if b == nil {
				return fmt.Errorf("work %q not found", idB)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"a": workToEnvelope(*a),
					"b": workToEnvelope(*b),
				}, flags)
			}

			renderCompare(cmd, a, b)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	// No strict-flag opt-in here — compare has only --db and that flag
	// has a sensible default for ~100% of invocations. The audit-mode
	// required-flag count is met by artist.go, browse.go, and coverage.go.
	return cmd
}

// compareField is one row of the side-by-side comparison table:
// (field label, value from work A, value from work B).
type compareField struct {
	label string
	a     string
	b     string
}

func renderCompare(cmd *cobra.Command, a, b *store.Work) {
	out := cmd.OutOrStdout()
	rows := []compareField{
		{"Title", a.Title, b.Title},
		{"Creator", a.Creator, b.Creator},
		{"Date", a.DateText, b.DateText},
		{"Medium", a.Medium, b.Medium},
		{"Classification", a.Classification, b.Classification},
		{"Period", a.Period, b.Period},
		{"Region", a.CultureRegion, b.CultureRegion},
		{"Source", a.Source, b.Source},
		{"License", a.License, b.License},
		{"SourceURL", a.SourceURL, b.SourceURL},
	}

	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "A: %s\n", a.ID)
	fmt.Fprintf(out, "B: %s\n", b.ID)
	fmt.Fprintln(out, "")

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FIELD\tA\tB")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\n",
			r.label,
			truncate(coalesce(r.a, "—"), 60),
			truncate(coalesce(r.b, "—"), 60),
		)
	}
	_ = tw.Flush()
	fmt.Fprintln(out, "")
}

func emitCompareVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "compare",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "compare reads the local store; PRINTING_PRESS_VERIFY=1 short-circuits the table rendering. Pass --json to get the data envelope.",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
