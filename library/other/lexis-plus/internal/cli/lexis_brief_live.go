// Hand-written: commands that call Lexis+ with your own session.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// pp:client-call  (requests go through the hand-written lexisSession in lexis_session.go)

func newBriefCheckCmd(flags *rootFlags) *cobra.Command {
	var max int
	check := &cobra.Command{
		Use:         "check <file>",
		Short:       "Check every citation in a draft against Lexis+ (recognized or not)",
		Long:        "Use this command to validate every cite in a document. Do NOT use it for a single cite; use 'cite check' instead.",
		Example:     "  lexis-plus-pp-cli brief check motion.docx",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": "file=example-value", "pp:verified-by-proof": "brief-citecheck-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
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
			if len(cs) > max {
				return usageErr(fmt.Errorf("%d citations found; --max is %d. Raise --max deliberately if you really need more", len(cs), max))
			}
			s, err := newLexisSession(flags)
			if err != nil {
				return err
			}
			type row struct {
				Cite   string `json:"cite"`
				Status string `json:"status"` // recognized | NOT RECOGNIZED | ERROR
				Title  string `json:"title,omitempty"`
				Error  string `json:"error,omitempty"` // why the lookup failed (ERROR rows only)
			}
			var rows []row
			unknown, failedN := 0, 0
			for i, c := range cs {
				if i > 0 {
					time.Sleep(1 * time.Second)
				}
				_, err := s.resolveCite(cmd.Context(), c.Cite)
				switch {
				case err == nil:
					rows = append(rows, row{Cite: c.Cite, Status: "recognized", Title: titleCase(s.suggestTitle(cmd.Context(), c.Cite))})
				case isCiteNotFound(err):
					unknown++
					rows = append(rows, row{Cite: c.Cite, Status: "NOT RECOGNIZED"})
				case ExitCode(err) == 4:
					// An expired session would fail every remaining cite the same way.
					return err
				default:
					// A timeout or server error says nothing about the citation itself.
					failedN++
					rows = append(rows, row{Cite: c.Cite, Status: "ERROR", Error: err.Error()})
				}
			}
			result := func() error {
				switch {
				case failedN > 0:
					return apiErr(fmt.Errorf("%d of %d lookups failed before Lexis+ could answer (see ERROR rows); those citations were not checked", failedN, len(rows)))
				case unknown > 0:
					return notFoundErr(fmt.Errorf("%d of %d citations were not recognized by Lexis+", unknown, len(rows)))
				}
				return nil
			}
			if flags.asJSON {
				if err := printJSONFilteredKeep(cmd.OutOrStdout(), rows, flags, "cite", "status", "title", "error"); err != nil {
					return err
				}
				return result()
			}
			for _, r := range rows {
				detail := r.Title
				if r.Error != "" {
					detail = r.Error
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-15s %-28s %s\n", r.Status, r.Cite, detail)
			}
			return result()
		},
	}
	check.Flags().IntVar(&max, "max", 40, "Refuse to check more than this many citations in one run")

	return check
}

// uniquePullName returns base, or base with " (2)", " (3)", ... appended, so that dir/name+ext
// does not already exist.
func uniquePullName(dir, base, ext string) string {
	name := base
	for n := 2; ; n++ {
		if _, err := os.Stat(filepath.Join(dir, name+ext)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s (%d)", base, n)
	}
}

// ---- statute annotation diff ----

func newStatuteWhatsNewCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "whatsnew <citation>",
		Short:       "Re-pull an annotated statute and show what changed in the annotations since your last pull",
		Example:     "  lexis-plus-pp-cli statute whatsnew \"Cal Lab Code 1102.5\"",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "cite=Cal Lab Code 1102.5", "pp:verified-by-proof": "statute-matter-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "a citation"); err != nil {
				return err
			}
			var prev *logEntry
			for _, e := range readLog() {
				if e.Kind == "statute" && e.Annotations && e.Format == "docx" && citeKey(e.Cite) == citeKey(args[0]) {
					if _, err := os.Stat(e.Path); err == nil {
						e := e
						prev = &e
					}
				}
			}
			if prev == nil {
				return fmt.Errorf("no earlier annotated pull of %q in your log; run `statute get` first", args[0])
			}
			s, err := newLexisSession(flags)
			if err != nil {
				return err
			}
			a, err := s.resolveCite(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			a.Title = prev.Title
			// Read the previous copy before downloading, and give the new pull a name no earlier
			// pull can have, so a second run on the same day compares against the earlier copy
			// instead of overwriting it.
			oldT, err := readDocText(prev.Path)
			if err != nil {
				return err
			}
			dir := filepath.Dir(prev.Path)
			name := uniquePullName(dir, safeFileName(a.Cite)+" "+time.Now().Format("2006-01-02 150405"), ".docx")
			newPath, err := s.deliver(cmd.Context(), a, deliverOpts{Format: "docx", Annotations: true, OutDir: dir, Name: name, Matter: prev.Matter})
			if err != nil {
				return err
			}
			newT, err := readDocText(newPath)
			if err != nil {
				return err
			}
			oldL, newL := lineSet(oldT), lineSet(newT)
			var added, removed []string
			for l := range newL {
				if !oldL[l] {
					added = append(added, l)
				}
			}
			for l := range oldL {
				if !newL[l] {
					removed = append(removed, l)
				}
			}
			sort.Strings(added)
			sort.Strings(removed)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"previous": prev.Path, "current": newPath, "added": added, "removed": removed}, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Compared with your pull of %s\nNew copy: %s\n", prev.At[:10], newPath)
			fmt.Fprintf(w, "\nAdded (%d):\n", len(added))
			for _, l := range added {
				fmt.Fprintf(w, "  + %s\n", trunc(l, 300))
			}
			fmt.Fprintf(w, "\nRemoved (%d):\n", len(removed))
			for _, l := range removed {
				fmt.Fprintf(w, "  - %s\n", trunc(l, 300))
			}
			return nil
		},
	}
}
