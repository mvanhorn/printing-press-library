// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	// A literal rootCmd.AddCommand is what the press's static scanners
	// (dogfood, verify-skill) walk; without it they resolve "save" to the
	// framework leaf of the same name (#4567). It adds only when nothing
	// registered the command already, so the runtime tree never doubles.
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		if !hasChildCommand(rootCmd, "save") {
			rootCmd.AddCommand(newNovelSaveCmd(flags))
		}
	})
}

var searchNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// saveResult is the row save prints: the stored search plus whether saving
// changed its filters and so dropped the old baseline.
type saveResult struct {
	savedSearchRow
	Reset bool `json:"reset"`
}

func newNovelSaveCmd(flags *rootFlags) *cobra.Command {
	var pf postingFlags
	cmd := &cobra.Command{
		Use:   "save <name> [query]",
		Short: "Store a named set of filters so new can diff it on every later run",
		Long: strings.Trim(`
Store a named set of posting filters in the local store so new can diff them

The optional [query] is the keyword search; an empty query ('') means none, and
--base-query sets the same thing. Saving never touches the network: the first
new run for the name takes the baseline. Saving an existing name with different
filters drops its old baseline, so new never diffs across changed filters.
--sort is accepted for tracker compatibility and ignored, since order never
changes which postings a search contains.

Names are 1-64 letters, digits, dot, dash, or underscore. An unknown name given
to new exits not-found instead of silently starting a fresh search.`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli save uk-strategy --country GBR --base-query strategy
  uber-jobs-pp-cli save gbr-all '' --country GBR --sort recent
  uber-jobs-pp-cli save nl-ops --country NLD --team Operations --json`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write":    "true",
			"pp:data-source":     "local",
			"pp:happy-args":      "name=uk-strategy;--country=GBR",
			"pp:live-happy-path": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) > 2 {
				return usageErr(fmt.Errorf("save takes <name> and an optional [query], got %d arguments; quote a multi-word query", len(args)))
			}
			f, err := pf.filters()
			if err != nil {
				return err
			}
			name := ""
			if len(args) >= 1 {
				name = strings.TrimSpace(args[0])
			}
			if len(args) == 2 {
				q := strings.TrimSpace(args[1])
				if q != "" && f.Query != "" && q != f.Query {
					return usageErr(fmt.Errorf("the query was given twice (%q and --base-query %q); give it once", q, f.Query))
				}
				if q != "" {
					f.Query = q
				}
			}
			f.Sort = ""
			if dryRunOK(flags) {
				shown := name
				if shown == "" {
					shown = "<name>"
				}
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("save search %q with filters %s to the local store at %s", shown, filtersJSON(f), uberDBPath(pf.dbPath)))
			}
			if name == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search name is required, for example: uber-jobs-pp-cli save uk-strategy --country GBR"))
			}
			if !searchNamePattern.MatchString(name) {
				return usageErr(fmt.Errorf("search name %q is invalid: use 1-64 letters, digits, dot, dash, or underscore, starting with a letter or digit", name))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, db, err := openUberStore(ctx, pf.dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			saved, reset, err := uberjobs.SaveSearch(ctx, db, name, f, nowUTC())
			if err != nil {
				return fmt.Errorf("saving search %q: %w", name, err)
			}
			row := saveResult{savedSearchRow: savedSearchRow{ID: saved.Name, SavedSearch: *saved}, Reset: reset}
			env := uberjobs.NewEnvelope("save", uberjobs.SourceLocal, []saveResult{row}, 1)
			env.Hits = 1
			env.Meta.Complete = true
			switch {
			case reset:
				env.Meta.Note = "the filters changed, so the old baseline was dropped; the next new run takes a fresh one"
			case saved.BaselineAt == nil:
				env.Meta.Note = "no baseline yet; run: uber-jobs-pp-cli new " + name
			}
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				fmt.Fprintf(w, "saved search %s: %s\n", name, describeFilters(saved.Filters))
				if env.Meta.Note != "" {
					fmt.Fprintln(w, "note:", env.Meta.Note)
				}
				return nil
			})
		},
	}
	registerPostingFlags(cmd, &pf, false, false)
	return cmd
}

// filtersJSON renders filters compactly for dry-run text.
func filtersJSON(f uberjobs.Filters) string {
	b, err := json.Marshal(f)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// describeFilters renders filters for human output.
func describeFilters(f uberjobs.Filters) string {
	var parts []string
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			parts = append(parts, k+" "+v)
		}
	}
	add("query", f.Query)
	add("country", strings.Join(f.Countries, ","))
	add("team", f.Team)
	add("sub-team", f.SubTeam)
	add("contract", f.ContractType)
	add("work pattern", f.WorkPattern)
	add("posted within", f.PostedWithin)
	for _, p := range f.DescriptionContains {
		add("description contains", p)
	}
	for _, p := range f.DescriptionExcludes {
		add("description lacks", p)
	}
	if len(parts) == 0 {
		return "every posting"
	}
	return strings.Join(parts, ", ")
}
