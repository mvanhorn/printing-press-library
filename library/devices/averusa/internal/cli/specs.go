// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: specs — one model's datasheet spec fields as text or JSON.
// pp:data-source local
// Local-store command: fields exist only in the harvested corpus
// (extracted from the datasheet PDF by `harvest --with-specs`).

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/store"
)

func newNovelSpecsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "specs <model>",
		Short: "One model's full spec fields as clean text or JSON for spec-compliance tables and agent pipelines.",
		Long: strings.Trim(`
Dump one model's structured spec fields for RFP compliance tables or agent
pipelines. Fields are extracted from the model's datasheet PDF during
`+"`harvest --with-specs`"+` and stored locally — there is no AVer endpoint
that returns specs.

Use this command to dump one model's structured spec fields.
Do NOT use it to compare models; use 'compare' instead.
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli specs CAM570
  averusa-pp-cli specs CAM570 --json --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<model>=CAM570",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "specs")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("--data-source live has no live equivalent for specs; spec fields only exist in the corpus after `harvest --with-specs`"))
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("specs requires exactly one model, e.g. `specs CAM570`"))
			}
			model := normalizeModel(args[0])

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				return notFoundErr(fmt.Errorf("no corpus for model %s", model))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			fields, err := st.SpecFields(model)
			if err != nil {
				return err
			}
			if len(fields) == 0 {
				return notFoundErr(fmt.Errorf("no spec fields for model %q; run `averusa-pp-cli harvest --with-specs` first", model))
			}
			if flags.asJSON {
				return flags.printJSON(cmd, struct {
					Model  string                   `json:"model"`
					Fields []store.AVerUSASpecField `json:"fields"`
				}{Model: model, Fields: fields})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", strings.ToUpper(model))
			sorted := append([]store.AVerUSASpecField(nil), fields...)
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].Field < sorted[j].Field })
			for _, f := range sorted {
				fmt.Fprintf(w, "  %s: %s\n", f.Field, f.Value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	return cmd
}
