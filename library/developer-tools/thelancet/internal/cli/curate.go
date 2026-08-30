// Hand-authored Lancet analytics command. Not generated.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/store"
)

func newNovelCurateCmd(flags *rootFlags) *cobra.Command {
	var topic string
	var journal string
	var sortBy string
	var output string
	var openAccess bool
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "curate",
		Short: "Build a ranked Lancet reading list for a topic (Markdown/BibTeX/JSON)",
		Long: "Select Lancet works matching a topic or keyword and rank them by citations or\n" +
			"date, exportable as a Markdown list, BibTeX, or JSON. Reads the local mirror;\n" +
			"run 'thelancet-pp-cli refresh' first.",
		Example:     "  thelancet-pp-cli curate --topic 'gene therapy' --sort citations --output bibtex\n  thelancet-pp-cli curate --topic immunotherapy --journal lancet-oncology --output markdown",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--topic=cancer"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if topic == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--topic is required (the subject to curate)"))
			}
			switch sortBy {
			case "", "citations", "date":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sort must be 'citations' or 'date'"))
			}
			switch output {
			case "", "json", "markdown", "bibtex":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--output must be 'json', 'markdown', or 'bibtex'"))
			}
			issn, err := resolveJournalISSN(journal)
			if err != nil {
				_ = cmd.Usage()
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Prefer the local store when present; fall back to a live OpenAlex
			// query when the mirror is missing or yields nothing, so curate
			// works out-of-the-box before any refresh.
			var rows []lancet.WorkRow
			resolvedPath := dbPath
			if resolvedPath == "" {
				resolvedPath = defaultDBPath("thelancet-pp-cli")
			}
			if _, statErr := os.Stat(resolvedPath); statErr == nil {
				st, oerr := store.OpenWithContext(ctx, resolvedPath)
				if oerr != nil {
					return fmt.Errorf("opening database: %w", oerr)
				}
				st.DB().SetMaxOpenConns(1)
				rows, err = lancet.Curate(ctx, st.DB(), topic, issn, sortBy, openAccess, limit)
				st.Close()
				if err != nil {
					return fmt.Errorf("curating: %w", err)
				}
			}
			if len(rows) == 0 {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				rows, err = lancet.CurateLive(ctx, c, topic, issn, sortBy, openAccess, limit)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			if rows == nil {
				rows = []lancet.WorkRow{}
			}
			// Explicit doc formats take priority over --json envelope.
			switch output {
			case "markdown":
				return renderCurateMarkdown(cmd, topic, rows)
			case "bibtex":
				return renderCurateBibtex(cmd, rows)
			}
			if done, err := emitLancet(cmd, flags, rows); done || err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no works matched %q (try a broader topic or run refresh)\n", topic)
				return nil
			}
			for _, w := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "[%d cites] %s (%d)\n", w.Cited, w.Title, w.Year)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&topic, "topic", "", "Topic or keyword to curate (matches title or topic)")
	cmd.Flags().StringVar(&journal, "journal", "", "Scope to a Lancet journal slug, or omit for all")
	cmd.Flags().StringVar(&sortBy, "sort", "citations", "Sort order: citations or date")
	cmd.Flags().StringVar(&output, "output", "", "Output format: json (default), markdown, or bibtex")
	cmd.Flags().BoolVar(&openAccess, "open-access", false, "Only include open-access works")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum works to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/thelancet-pp-cli/data.db)")
	return cmd
}

func renderCurateMarkdown(cmd *cobra.Command, topic string, rows []lancet.WorkRow) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "# Lancet reading list: %s\n\n", topic)
	if len(rows) == 0 {
		fmt.Fprintln(w, "_No matching works found._")
		return nil
	}
	for _, r := range rows {
		doi := ""
		if r.DOI != "" {
			doi = fmt.Sprintf(" — [%s](https://doi.org/%s)", r.DOI, r.DOI)
		}
		fmt.Fprintf(w, "- **%s** (%s, %d) — %d citations%s\n", r.Title, r.Journal, r.Year, r.Cited, doi)
	}
	return nil
}

func renderCurateBibtex(cmd *cobra.Command, rows []lancet.WorkRow) error {
	w := cmd.OutOrStdout()
	for i, r := range rows {
		key := bibKey(r, i)
		fmt.Fprintf(w, "@article{%s,\n", key)
		fmt.Fprintf(w, "  title   = {%s},\n", r.Title)
		fmt.Fprintf(w, "  journal = {%s},\n", r.Journal)
		fmt.Fprintf(w, "  year    = {%d},\n", r.Year)
		if r.DOI != "" {
			fmt.Fprintf(w, "  doi     = {%s},\n", r.DOI)
		}
		fmt.Fprintf(w, "}\n\n")
	}
	return nil
}

func bibKey(r lancet.WorkRow, i int) string {
	base := r.DOI
	if base == "" {
		base = fmt.Sprintf("lancet%d", i)
	}
	repl := strings.NewReplacer("/", "_", ".", "_", "(", "", ")", "", ":", "_", " ", "")
	return "lancet_" + repl.Replace(base)
}
