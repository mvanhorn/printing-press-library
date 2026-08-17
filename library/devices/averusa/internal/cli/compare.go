// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: compare — side-by-side datasheet spec fields across models.
// pp:data-source local
// Local-store command: spec fields exist only in the harvested corpus
// (extracted from each model's datasheet PDF by `harvest --with-specs`).

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "compare <modelA> <modelB> [model...]",
		Short: "Side-by-side spec fields for two or more AVer models from their datasheets, ready for bid comparisons and RFP tables.",
		Long: strings.Trim(`
Compare datasheet spec fields side-by-side across two or more AVer models.
Fields are extracted from each model's datasheet PDF during `+"`harvest --with-specs`"+`
and stored locally; there is no AVer endpoint that returns specs at all.

Use this command to compare spec fields across models.
Do NOT use it for a single model's full spec dump; use 'specs <model>' instead.
Do NOT use it to check which documents exist for a model; use 'coverage <category>' instead.
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli compare CAM570 CAM550
  averusa-pp-cli compare CAM570 CAM550 --json --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":     "true",
			"pp:happy-args":     "<modelA>=CAM570;<modelB>=CAM550",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("--data-source live has no live equivalent for compare; spec fields only exist in the corpus after `harvest --with-specs`"))
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare requires at least two models, e.g. `compare CAM570 CAM550`"))
			}

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				return notFoundErr(fmt.Errorf("no corpus to compare against"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			var models []compareModelSpecs
			totalFields := 0
			for _, raw := range args {
				model := normalizeModel(raw)
				fields, err := st.SpecFields(model)
				if err != nil {
					return err
				}
				fm := map[string]string{}
				for _, f := range fields {
					fm[f.Field] = f.Value
				}
				models = append(models, compareModelSpecs{Model: model, Fields: fm})
				totalFields += len(fm)
			}
			if totalFields == 0 {
				return notFoundErr(fmt.Errorf("no spec fields found for the requested models; run `averusa-pp-cli harvest --with-specs` first"))
			}
			if flags.asJSON {
				return flags.printJSON(cmd, models)
			}
			return renderCompareTable(cmd, models)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	return cmd
}

type compareModelSpecs struct {
	Model  string            `json:"model"`
	Fields map[string]string `json:"fields"`
}

func renderCompareTable(cmd *cobra.Command, models []compareModelSpecs) error {
	// Union of field names, sorted.
	union := map[string]bool{}
	for _, m := range models {
		for f := range m.Fields {
			union[f] = true
		}
	}
	fields := make([]string, 0, len(union))
	for f := range union {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	w := cmd.OutOrStdout()
	// Column widths: field column + one per model.
	widths := []int{len("Field")}
	for _, m := range models {
		wid := len(m.Model)
		for f := range m.Fields {
			if len(f) > wid {
				wid = len(f)
			}
		}
		widths = append(widths, wid)
	}
	fmt.Fprintf(w, "%s", pad("Field", widths[0]))
	for i, m := range models {
		fmt.Fprintf(w, "  %s", pad(m.Model, widths[i+1]))
	}
	fmt.Fprintln(w)
	for _, f := range fields {
		fmt.Fprintf(w, "%s", pad(f, widths[0]))
		for i, m := range models {
			fmt.Fprintf(w, "  %s", pad(m.Fields[f], widths[i+1]))
		}
		fmt.Fprintln(w)
	}
	return nil
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// normalizeModel maps user input like "CAM570", "VC520 Pro 3" or "cam570"
// onto the lowercase slug keys the corpus uses.
func normalizeModel(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
