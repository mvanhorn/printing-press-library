// Hand-written: user-facing Lexis+ commands (case, statute, cite, batch).

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:client-call  (requests go through the hand-written lexisSession in lexis_session.go)

type fetchFlags struct {
	out, format, matter string
	noAnnot, noHead     bool
	noSummary, force    bool
	noRedflags          bool
}

func addFetchFlags(cmd *cobra.Command, f *fetchFlags, statute bool) {
	cmd.Flags().StringVar(&f.out, "out", defaultDownloadDir(), "Folder to save the file in (default: ~/Downloads/Lexis Downloads)")
	cmd.Flags().StringVar(&f.format, "format", "docx", "File format: docx (Word), pdf, or rtf")
	cmd.Flags().StringVar(&f.matter, "matter", "", "Tag this pull with a matter name (for `matter list`/`matter binder`)")
	if statute {
		cmd.Flags().BoolVar(&f.noAnnot, "no-annotations", false, "Leave out the Deering's annotations (included by default)")
	} else {
		cmd.Flags().BoolVar(&f.noHead, "no-headnotes", false, "Leave out headnotes")
		cmd.Flags().BoolVar(&f.noSummary, "no-summary", false, "Leave out the case summary")
	}
	cmd.Flags().BoolVar(&f.noRedflags, "no-redflags", false, "Skip the automatic Shepard's red-flag check and memo for cases")
}

type fetchResult struct {
	Cite        string `json:"cite"`
	Title       string `json:"title,omitempty"`
	Kind        string `json:"kind"`
	DocID       string `json:"doc_id"`
	Path        string `json:"path,omitempty"`
	Annotations bool   `json:"annotations,omitempty"`
	Error       string `json:"error,omitempty"`
	Note        string `json:"note,omitempty"` // e.g. which version of a multi-version code section was picked
	// Shepard's red-flag check, run automatically for every case downloaded.
	Signal      string         `json:"shepards_signal,omitempty"`
	RedFlags    *redFlagReport `json:"red_flag_report,omitempty"`
	Memo        string         `json:"red_flag_memo,omitempty"`
	RedFlagNote string         `json:"red_flag_note,omitempty"`
}

// applyRedFlags runs the Shepard's red-flag check for a downloaded case and, when the case is
// red-flagged, writes the explanation memo next to the case file. Failures never fail the download.
func applyRedFlags(cmd *cobra.Command, s *lexisSession, a *authority, casePath string, r *fetchResult) {
	if a.Kind != "case" {
		return
	}
	rep, err := s.redFlags(cmd.Context(), a, false)
	if err != nil {
		r.RedFlagNote = "Shepard's check failed: " + err.Error()
		return
	}
	r.Signal = rep.Signal
	if len(rep.Flags) == 0 {
		if rep.Note != "" {
			r.RedFlagNote = rep.Note
		}
		return
	}
	rep.CaseFile = casePath
	r.RedFlags = rep
	if mpath, err := writeRedFlagMemo(rep, filepath.Dir(casePath)); err == nil {
		r.Memo = mpath
	} else {
		r.RedFlagNote = "red flag found but the memo could not be written: " + err.Error()
	}
}

// fetchOne resolves one citation and downloads it. kind "" = auto-detect from the lookup result.
func fetchOne(cmd *cobra.Command, flags *rootFlags, s *lexisSession, cite, wantKind string, f fetchFlags) (*fetchResult, error) {
	ctx := cmd.Context()
	a, err := s.resolveCite(ctx, cite)
	if err != nil {
		return nil, err
	}
	if wantKind != "" && a.Kind != wantKind {
		return nil, fmt.Errorf("%q resolved to a %s, not a %s", cite, a.Kind, wantKind)
	}
	a.Title = s.docTitle(ctx, a)
	if a.Title == "" {
		a.Title = titleCase(s.suggestTitle(ctx, cite))
	}
	if a.Title == "" {
		a.Title = a.Cite
	}
	if flags.dryRun {
		return &fetchResult{Cite: a.Cite, Title: a.Title, Kind: a.Kind, DocID: a.DocID}, nil
	}
	path, err := s.deliver(ctx, a, deliverOpts{Format: f.format, Annotations: !f.noAnnot, Headnotes: !f.noHead, Summary: !f.noSummary, OutDir: f.out, Matter: f.matter})
	if err != nil {
		return nil, err
	}
	res := &fetchResult{Cite: a.Cite, Title: a.Title, Kind: a.Kind, DocID: a.DocID, Path: path, Annotations: a.Kind == "statute" && !f.noAnnot, Note: a.Status}
	if !f.noRedflags {
		applyRedFlags(cmd, s, a, path, res)
	}
	return res, nil
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 && w[0] >= 'a' && w[0] <= 'z' && w != "v." && w != "of" && w != "the" && w != "and" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func printFetch(cmd *cobra.Command, flags *rootFlags, r []fetchResult) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), r, flags)
	}
	w := cmd.OutOrStdout()
	for _, x := range r {
		switch {
		case x.Error != "":
			fmt.Fprintf(w, "FAILED  %s: %s\n", x.Cite, x.Error)
		case x.Path == "":
			fmt.Fprintf(w, "FOUND   %s  %s  (%s %s) [dry run, not downloaded]\n", x.Cite, x.Title, x.Kind, x.DocID)
		default:
			extra := ""
			if x.Annotations {
				extra = " (with annotations)"
			}
			fmt.Fprintf(w, "SAVED   %s  %s%s\n        -> %s\n", x.Cite, x.Title, extra, x.Path)
		}
		if x.RedFlags != nil {
			for _, fl := range x.RedFlags.Flags {
				var ts []string
				for _, t := range fl.Red {
					ts = append(ts, t.Treatment)
				}
				fmt.Fprintf(w, "RED FLAG %s: %s %s, %s (%s %s)\n", x.Cite, strings.Join(ts, "; "), fl.Case, fl.Citation, fl.Court, fl.Date)
			}
			if x.Memo != "" {
				fmt.Fprintf(w, "        memo -> %s\n", x.Memo)
			}
		} else if x.Signal != "" && x.Kind == "case" {
			fmt.Fprintf(w, "        Shepard's: no red flag (%s)\n", strings.TrimPrefix(x.Signal, "Caselaw"))
		}
		if x.RedFlagNote != "" && x.RedFlags == nil && !strings.HasPrefix(x.RedFlagNote, "No red flags") {
			fmt.Fprintf(w, "        %s\n", x.RedFlagNote)
		}
	}
	return nil
}

func newCaseCmd(flags *rootFlags) *cobra.Command {
	root := &cobra.Command{Use: "case", Short: "California reported cases by citation"}
	var f fetchFlags
	get := &cobra.Command{
		Use:   "get <citation>...",
		Short: "Find a California case by citation (e.g. \"27 Cal.3d 167\") and download it as Word",
		Example: "  lexis-plus-pp-cli case get \"27 Cal.3d 167\" --out ./authorities\n" +
			"  lexis-plus-pp-cli case get \"172 Cal.App.4th 1\" --format pdf",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:happy-args": "cite=27 Cal.3d 167"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "at least one citation"); err != nil {
				return err
			}
			s, err := newLexisSession(flags)
			if err != nil {
				return err
			}
			var res []fetchResult
			failed := false
			for i, c := range args {
				if i > 0 {
					time.Sleep(1500 * time.Millisecond)
				}
				r, err := fetchOne(cmd, flags, s, c, "case", f)
				if err != nil {
					failed = true
					res = append(res, fetchResult{Cite: c, Error: err.Error()})
					continue
				}
				res = append(res, *r)
			}
			if err := printFetch(cmd, flags, res); err != nil {
				return err
			}
			if failed {
				return fmt.Errorf("one or more citations failed")
			}
			return nil
		},
	}
	addFetchFlags(get, &f, false)
	root.AddCommand(get, newCaseRedflagsCmd(flags), newCaseRedflagMemoCmd(flags))
	return root
}

func newStatuteCmd(flags *rootFlags) *cobra.Command {
	root := &cobra.Command{Use: "statute", Short: "California statutes (Deering's annotated codes) by citation"}
	var f fetchFlags
	get := &cobra.Command{
		Use:   "get <citation>...",
		Short: "Find a California statute section (e.g. \"Cal Lab Code 1102.5\") and download it as Word with annotations",
		Example: "  lexis-plus-pp-cli statute get \"Cal Lab Code 1102.5\" --out ./authorities\n" +
			"  lexis-plus-pp-cli statute get \"Cal Gov Code 12940\" --no-annotations",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:happy-args": "cite=Cal Lab Code 1102.5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "at least one citation"); err != nil {
				return err
			}
			s, err := newLexisSession(flags)
			if err != nil {
				return err
			}
			var res []fetchResult
			failed := false
			for i, c := range args {
				if i > 0 {
					time.Sleep(1500 * time.Millisecond)
				}
				r, err := fetchOne(cmd, flags, s, c, "statute", f)
				if err != nil {
					failed = true
					res = append(res, fetchResult{Cite: c, Error: err.Error()})
					continue
				}
				res = append(res, *r)
			}
			if err := printFetch(cmd, flags, res); err != nil {
				return err
			}
			if failed {
				return fmt.Errorf("one or more citations failed")
			}
			return nil
		},
	}
	addFetchFlags(get, &f, true)
	root.AddCommand(get, newStatuteWhatsNewCmd(flags))
	return root
}

func newCiteCmd(flags *rootFlags) *cobra.Command {
	root := &cobra.Command{Use: "cite", Short: "Check citations against Lexis+ without downloading"}
	check := &cobra.Command{
		Use:         "check <citation>...",
		Short:       "Say whether Lexis+ recognizes each citation, and what document it points to",
		Example:     "  lexis-plus-pp-cli cite check \"27 Cal.3d 167\" \"Cal Lab Code 1102.5\"",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "cite=27 Cal.3d 167"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "at least one citation"); err != nil {
				return err
			}
			s, err := newLexisSession(flags)
			if err != nil {
				return err
			}
			var out []authority
			for i, c := range args {
				if i > 0 {
					time.Sleep(1 * time.Second)
				}
				a, err := s.resolveCite(cmd.Context(), c)
				if err != nil {
					out = append(out, authority{Cite: c, Status: "not recognized: " + err.Error()})
					continue
				}
				a.Title = titleCase(s.suggestTitle(cmd.Context(), c))
				if a.Status == "" {
					a.Status = "recognized"
				}
				out = append(out, *a)
			}
			unknown := 0
			for _, a := range out {
				if a.DocID == "" {
					unknown++
				}
			}
			unknownErr := func() error {
				if unknown == 0 {
					return nil
				}
				return notFoundErr(fmt.Errorf("%d of %d citations were not recognized by Lexis+", unknown, len(out)))
			}
			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
				return unknownErr()
			}
			for _, a := range out {
				if a.DocID != "" {
					note := ""
					if a.Status != "recognized" {
						note = "  (" + a.Status + ")"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "OK   %s  %s  [%s %s]%s\n", a.Cite, a.Title, a.Kind, a.DocID, note)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "??   %s  %s\n", a.Cite, a.Status)
				}
			}
			return unknownErr()
		},
	}
	root.AddCommand(check)
	return root
}

func newBatchCmd(flags *rootFlags) *cobra.Command {
	var file string
	var f fetchFlags
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Download every citation in a list (one per line; cases and statutes mixed) as Word files",
		Long: "Reads citations from --file (or stdin with --file -), one per line. Blank lines and lines starting with # are skipped.\n" +
			"Statutes are downloaded with annotations. Citations already pulled (same format, file still present) are skipped unless --force.\n" +
			"Requests are paced about 1.5 seconds apart. Keep lists to the authorities you actually need.",
		Example:     "  lexis-plus-pp-cli batch --file cites.txt --out ./authorities --matter Acme",
		Annotations: map[string]string{"pp:happy-args": "--file=example-value"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if file == "" {
				return usageErr(fmt.Errorf("--file is required"))
			}
			cites, err := readCiteLines(file, cmd)
			if err != nil {
				return err
			}
			if len(cites) == 0 {
				return usageErr(fmt.Errorf("no citations found in %s", file))
			}
			if len(cites) > 60 {
				return usageErr(fmt.Errorf("%d citations is more than this CLI will fetch in one run (limit 60); split the list", len(cites)))
			}
			have := map[string]logEntry{}
			if !f.force {
				for _, e := range readLog() {
					if e.Format == f.format || (f.format == "" && e.Format == "docx") {
						if _, err := os.Stat(e.Path); err == nil {
							have[citeKey(e.Cite)] = e
						}
					}
				}
			}
			s, err := newLexisSession(flags)
			if err != nil {
				return err
			}
			var res []fetchResult
			failed := false
			for i, c := range cites {
				if e, ok := have[citeKey(c)]; ok {
					r := fetchResult{Cite: c, Path: e.Path, Kind: e.Kind, Title: "(already pulled, skipped)"}
					if e.Kind == "case" && !f.noRedflags {
						// Shepard's status can change after a pull, so re-check skipped cases too.
						if i > 0 {
							time.Sleep(1500 * time.Millisecond)
						}
						if a, err := s.resolveCite(cmd.Context(), c); err == nil {
							a.Title = e.Title
							applyRedFlags(cmd, s, a, e.Path, &r)
						}
					}
					res = append(res, r)
					continue
				}
				if i > 0 {
					time.Sleep(1500 * time.Millisecond)
				}
				r, err := fetchOne(cmd, flags, s, c, "", f)
				if err != nil {
					failed = true
					res = append(res, fetchResult{Cite: c, Error: err.Error()})
					continue
				}
				res = append(res, *r)
			}
			if err := printFetch(cmd, flags, res); err != nil {
				return err
			}
			if failed {
				return fmt.Errorf("one or more citations failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Text file of citations, one per line (- for stdin)")
	addFetchFlags(cmd, &f, true)
	cmd.Flags().BoolVar(&f.force, "force", false, "Download again even if already pulled")
	return cmd
}

func readCiteLines(file string, cmd *cobra.Command) ([]string, error) {
	var sc *bufio.Scanner
	if file == "-" {
		sc = bufio.NewScanner(cmd.InOrStdin())
	} else {
		fh, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer fh.Close()
		sc = bufio.NewScanner(fh)
	}
	var out []string
	for sc.Scan() {
		ln := strings.TrimSpace(sc.Text())
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		out = append(out, ln)
	}
	return out, sc.Err()
}

var _ = json.Marshal

func dryRunNote(cmd *cobra.Command, flags *rootFlags, args []string) error {
	cite := ""
	if len(args) > 0 {
		cite = args[0]
	}
	msg := map[string]any{
		"meta": map[string]any{"source": "dry-run"},
		"results": map[string]any{
			"dry_run": true, "action": cmd.CommandPath() + " (not run)", "command": cmd.CommandPath(), "cite": cite,
			"note": "no request sent to Lexis+, nothing written",
		},
		"dry_run": true,
		"action":  cmd.CommandPath() + " (not run)",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), msg, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "dry run: %s would run; no request sent to Lexis+, nothing written\n", cmd.CommandPath())
	return nil
}

func needArgs(cmd *cobra.Command, args []string, n int, what string) error {
	if len(args) < n {
		return usageErr(fmt.Errorf("%s needs %s; run %q for usage", cmd.CommandPath(), what, cmd.CommandPath()+" --help"))
	}
	return nil
}

// defaultDownloadDir is where Word/PDF/RTF files land unless --out is given.
func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "Lexis Downloads"
	}
	return filepath.Join(home, "Downloads", "Lexis Downloads")
}
