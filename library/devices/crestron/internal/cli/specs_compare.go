// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Fetches both product pages and aligns their specification tables.

package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	specFootnoteAfterSemicolon = regexp.MustCompile(`;\s*\d+(?:\s*,\s*\d+)*(\s|$)`)
	specFootnoteBeforeParen    = regexp.MustCompile(`\s+\d+(?:\s*,\s*\d+)*\s*\)`)
	specFootnoteTrailing       = regexp.MustCompile(`\s+\d+(?:\s*,\s*\d+)*\s*$`)
	specWhitespaceRun          = regexp.MustCompile(`\s+`)
)

// stripSpecFootnotes removes Crestron's superscript footnote reference markers
// from a spec value. Each product page numbers its own footnotes, so two models
// with identical spec text still carry different marker digits ("...compatible
// 14 )" vs "...compatible 15 )"). Comparing raw values reported roughly half of
// all rows as differences when nothing about the spec itself differed.
//
// Only the equality test uses this; both models' verbatim values are still what
// gets displayed, so an over-eager strip can hide a row but can never show the
// user altered text. Markers are bare integer tokens in three positions:
// directly after a semicolon, immediately before a closing paren, and at the end
// of the value. Real spec numbers carry a unit ("500 mA", "20 W typical",
// "8 channels") and so never match. A marker sitting mid-sentence
// ("... Series 11 Decoder: ...") is left alone as too risky to detect.
func stripSpecFootnotes(s string) string {
	s = specFootnoteAfterSemicolon.ReplaceAllString(s, ";$1")
	s = specFootnoteBeforeParen.ReplaceAllString(s, ")")
	s = specFootnoteTrailing.ReplaceAllString(s, "")
	return strings.TrimSpace(specWhitespaceRun.ReplaceAllString(s, " "))
}

type specDiffRow struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Left    string `json:"left,omitempty"`
	Right   string `json:"right,omitempty"`
	Same    bool   `json:"same"`
}

type specsCompareView struct {
	Left        string        `json:"left_model"`
	Right       string        `json:"right_model"`
	Rows        []specDiffRow `json:"rows"`
	Differences int           `json:"differences"`
	Shared      int           `json:"shared"`
	OnlyLeft    int           `json:"only_left"`
	OnlyRight   int           `json:"only_right"`
	Note        string        `json:"note,omitempty"`
}

func newNovelSpecsCompareCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "compare <model-a> <model-b>",
		Short: "Compare two models field by field across the full specification table.",
		Long: strings.Trim(`
Align two products' specification tables section by section and field by field.

Crestron renders one product at a time, so comparing siblings means opening two
tabs and scrolling. Models in the same series often share a firmware release and
differ in only a handful of specification rows; this surfaces exactly those.

By default only differing rows are shown. Use --all to include matching rows.

Use this command to compare two models field by field.
Do NOT use it to display one model's specs; use 'specs'.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli specs compare DM-NVX-360 DM-NVX-363 --agent
  crestron-pp-cli specs compare DM-NVX-360 DM-NVX-363 --all
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two models are required"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "specs compare")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, haveMirror := openMirror(ctx, flagDB)
			if haveMirror {
				defer func() { _ = st.Close() }()
			}

			type side struct {
				model string
				rows  map[string]specDiffRow
			}
			load := func(model string) (side, error) {
				s := side{model: model, rows: map[string]specDiffRow{}}
				r, err := resolveModel(ctx, c, st, model)
				if err != nil {
					return s, err
				}
				if r.URL == "" {
					return s, fmt.Errorf("no product page known for %q; run 'crestron-pp-cli sync' to build the local catalog", model)
				}
				_, specs, err := fetchProductPage(ctx, c, r.URL)
				if err != nil {
					return s, fmt.Errorf("fetching specs for %s: %w", model, err)
				}
				for _, sec := range specs {
					for _, row := range sec.Rows {
						s.rows[sec.Name+"\x00"+row.Key] = specDiffRow{
							Section: sec.Name, Key: row.Key, Left: row.Value,
						}
					}
				}
				return s, nil
			}

			left, lerr := load(args[0])
			right, rerr := load(args[1])
			if (lerr != nil && !needsSync(lerr)) || (rerr != nil && !needsSync(rerr)) {
				if lerr != nil {
					return lerr
				}
				return rerr
			}
			if lerr != nil || rerr != nil {
				// Crestron has no live product search, so a catalog path is only
				// available once 'sync' has run. Report that as an actionable
				// empty result rather than an error, so agents still get JSON.
				view := specsCompareView{Left: args[0], Right: args[1], Rows: make([]specDiffRow, 0)}
				cause := lerr
				if cause == nil {
					cause = rerr
				}
				view.Note = cause.Error()
				fmt.Fprintln(cmd.ErrOrStderr(), "run: crestron-pp-cli sync --resources categories,products")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}

			view := specsCompareView{Left: left.model, Right: right.model, Rows: make([]specDiffRow, 0)}
			keys := map[string]bool{}
			for k := range left.rows {
				keys[k] = true
			}
			for k := range right.rows {
				keys[k] = true
			}
			ordered := make([]string, 0, len(keys))
			for k := range keys {
				ordered = append(ordered, k)
			}
			sort.Strings(ordered)

			for _, k := range ordered {
				l, hasL := left.rows[k]
				r, hasR := right.rows[k]
				row := specDiffRow{Section: l.Section, Key: l.Key}
				if !hasL {
					row.Section, row.Key = r.Section, r.Key
				}
				if hasL {
					row.Left = l.Left
				}
				if hasR {
					row.Right = r.Left
				}
				row.Same = hasL && hasR &&
					stripSpecFootnotes(row.Left) == stripSpecFootnotes(row.Right)
				switch {
				case row.Same:
					view.Shared++
				case hasL && hasR:
					view.Differences++
				case hasL:
					view.OnlyLeft++
				default:
					view.OnlyRight++
				}
				if row.Same && !flagAll {
					continue
				}
				view.Rows = append(view.Rows, row)
			}

			if len(view.Rows) == 0 {
				view.Note = fmt.Sprintf("%s and %s have identical specification tables across %d fields",
					left.model, right.model, view.Shared)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "SECTION\tFIELD\t%s\t%s\n", left.model, right.model)
			for _, r := range view.Rows {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					truncateText(r.Section, 22), truncateText(r.Key, 26),
					truncateText(dashIfEmpty(r.Left), 34), truncateText(dashIfEmpty(r.Right), 34))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d differing, %d identical, %d only on %s, %d only on %s\n",
				view.Differences, view.Shared, view.OnlyLeft, left.model, view.OnlyRight, right.model)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Include fields that are identical between the two models")
	return cmd
}
