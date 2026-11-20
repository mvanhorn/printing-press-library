// Hand-written: commands that read only your local retrieval log.

package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newBriefGapsCmd(flags *rootFlags) *cobra.Command {
	var matter string
	gaps := &cobra.Command{
		Use:         "gaps <file>",
		Short:       "Which authorities cited in a draft you have not pulled yet (and which pulled ones it no longer cites)",
		Long:        "Use this command to see which of a document's authorities are missing from your pulled set. Do NOT use it to fetch them; run 'batch --file' on the output.",
		Example:     "  lexis-plus-pp-cli brief gaps motion.docx --matter Acme",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "file=example-value", "pp:verified-by-proof": "brief-citecheck-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source local
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "a file"); err != nil {
				return err
			}
			cs, err := briefCites(args[0])
			if err != nil {
				return err
			}
			pulled := map[string]logEntry{}
			for _, e := range readLog() {
				if matter != "" && !strings.EqualFold(e.Matter, matter) {
					continue
				}
				pulled[citeKey(e.Cite)] = e
			}
			cited := map[string]bool{}
			var missing []string
			for _, c := range cs {
				cited[citeKey(c.Cite)] = true
				if _, ok := pulled[citeKey(c.Cite)]; !ok {
					missing = append(missing, c.Cite)
				}
			}
			var unused []string
			for k, e := range pulled {
				if !cited[k] {
					unused = append(unused, e.Cite)
				}
			}
			sort.Strings(unused)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"cited_not_pulled": missing, "pulled_not_cited": unused}, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Cited but not pulled (%d):\n", len(missing))
			for _, m := range missing {
				fmt.Fprintf(w, "  %s\n", m)
			}
			fmt.Fprintf(w, "Pulled but not cited in this draft (%d):\n", len(unused))
			for _, m := range unused {
				fmt.Fprintf(w, "  %s\n", m)
			}
			return nil
		},
	}
	gaps.Flags().StringVar(&matter, "matter", "", "Only compare against pulls tagged with this matter")

	return gaps
}

// ---- matter ledger and binder ----

func newMatterCmd(flags *rootFlags) *cobra.Command {
	root := &cobra.Command{Use: "matter", Short: "Your pulls grouped by matter (from `--matter` on case/statute/batch)"}

	list := &cobra.Command{
		Use:         "list",
		Short:       "List matters and how many authorities were pulled for each",
		Long:        "Use this command to browse your retrieval log by matter. Do NOT use it for full-text lookup in the log; use 'search'.",
		Example:     "  lexis-plus-pp-cli matter list --json",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source local
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			counts := map[string]int{}
			for _, e := range readLog() {
				m := e.Matter
				if m == "" {
					m = "(untagged)"
				}
				counts[m]++
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), counts, flags)
			}
			var names []string
			for n := range counts {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %d\n", n, counts[n])
			}
			return nil
		},
	}

	var olderThan int
	show := &cobra.Command{
		Use:         "show <matter>",
		Short:       "Show everything pulled for a matter: cite, title, format, annotations, date, path",
		Example:     "  lexis-plus-pp-cli matter show Acme --older-than 30",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "matter=example"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source local
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "a matter name"); err != nil {
				return err
			}
			var rows []logEntry
			for _, e := range readLog() {
				if !strings.EqualFold(e.Matter, args[0]) {
					continue
				}
				if olderThan > 0 {
					t, err := time.Parse(time.RFC3339, e.At)
					if err != nil || time.Since(t) < time.Duration(olderThan)*24*time.Hour {
						continue
					}
				}
				rows = append(rows, e)
			}
			if len(rows) == 0 && olderThan == 0 {
				return notFoundErr(fmt.Errorf("nothing in the log for matter %q", args[0]))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			for _, e := range rows {
				a := ""
				if e.Annotations {
					a = " +annotations"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-24s %s [%s%s]\n    %s\n", e.At[:10], e.Cite, e.Title, e.Format, a, e.Path)
			}
			return nil
		},
	}
	show.Flags().IntVar(&olderThan, "older-than", 0, "Only show pulls at least this many days old")

	var out string
	binder := &cobra.Command{
		Use:         "binder <matter>",
		Short:       "Copy a matter's files into one folder (cases, then statutes) with an index.csv",
		Example:     "  lexis-plus-pp-cli matter binder Acme --out ./binder",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:happy-args": "matter=example;--out=example-value", "pp:verified-by-proof": "statute-matter-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source local
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "a matter name"); err != nil {
				return err
			}
			if out == "" {
				return usageErr(fmt.Errorf("--out is required"))
			}
			var rows []logEntry
			for _, e := range readLog() {
				if strings.EqualFold(e.Matter, args[0]) {
					rows = append(rows, e)
				}
			}
			if len(rows) == 0 {
				return fmt.Errorf("nothing in the log for matter %q", args[0])
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Kind != rows[j].Kind {
					return rows[i].Kind == "case"
				}
				return rows[i].Cite < rows[j].Cite
			})
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			idx, err := os.Create(filepath.Join(out, "index.csv"))
			if err != nil {
				return err
			}
			defer idx.Close()
			cw := csv.NewWriter(idx)
			_ = cw.Write([]string{"order", "kind", "cite", "title", "file", "format", "annotations", "pulled", "sha256"})
			n := 0
			for i, e := range rows {
				data, err := os.ReadFile(e.Path)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "skipping %s: file missing (%s)\n", e.Cite, e.Path)
					continue
				}
				n++
				dst := fmt.Sprintf("%02d - %s%s", i+1, safeFileName(e.Cite), filepath.Ext(e.Path))
				if err := os.WriteFile(filepath.Join(out, dst), data, 0o644); err != nil {
					return err
				}
				_ = cw.Write([]string{fmt.Sprint(i + 1), e.Kind, e.Cite, e.Title, dst, e.Format, fmt.Sprint(e.Annotations), e.At, e.SHA256})
			}
			cw.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "Binder written to %s (%d files + index.csv)\n", out, n)
			return cw.Error()
		},
	}
	binder.Flags().StringVar(&out, "out", "", "Folder to build the binder in")

	root.AddCommand(list, show, binder)
	return root
}
