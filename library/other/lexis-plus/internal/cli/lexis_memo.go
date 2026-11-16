// Hand-written: red-flag explanation memo (.docx) and the local cache it is rebuilt from.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/lexis-plus/internal/docx"
)

type memoRun = docx.Run
type memoPara = docx.Para

func mp(style string, runs ...memoRun) memoPara { return memoPara{Style: style, Runs: runs} }
func mPlain(t string) memoRun                   { return memoRun{Text: t} }
func mBold(t string) memoRun                    { return memoRun{Text: t, Bold: true} }
func mItal(t string) memoRun                    { return memoRun{Text: t, Italic: true} }

// ---- the red-flag memo ----

func memoFileName(rep *redFlagReport) string {
	base := rep.Cite
	if rep.Case != "" && rep.Case != rep.Cite {
		base = rep.Case + " - " + rep.Cite
	}
	return safeFileName(base+" - RED FLAG memo") + ".docx"
}

func buildMemo(rep *redFlagReport) []memoPara {
	sig := shepSignalWords[rep.Signal]
	if sig == "" {
		sig = rep.Signal
	}
	name := rep.Case
	if name == "" {
		name = rep.Cite
	}
	n := len(rep.Flags)
	p := []memoPara{
		mp("Title", mPlain("Shepard's Red Flag Memo")),
		mp("Normal", mBold("Case: "), mItal(name), mPlain(", "+rep.Cite)),
		mp("Normal", mBold("Shepard's signal: "), mPlain(sig)),
		mp("Normal", mBold("Red flags: "), mPlain(fmt.Sprintf("%d", n))),
	}
	if rep.CaseFile != "" {
		p = append(p, mp("Normal", mBold("Case file: "), mPlain(filepath.Base(rep.CaseFile))))
	}
	p = append(p, mp("Small", mItal(fmt.Sprintf("Prepared %s from the Lexis+ Shepard's report. Red (Warning-level) treatment only.", time.Now().Format("January 2, 2006")))))

	p = append(p, mp("Heading1", mPlain("Bottom line")))
	if strings.TrimSpace(rep.Overall) != "" {
		p = append(p, mp("Normal", mPlain(rep.Overall)))
	} else {
		var parts []string
		for _, f := range rep.Flags {
			var ts []string
			for _, t := range f.Red {
				ts = append(ts, t.Treatment)
			}
			parts = append(parts, fmt.Sprintf("%s %s, %s (%s %s)", strings.Join(ts, "; "), f.Case, f.Citation, f.Court, f.Date))
		}
		p = append(p, mp("Normal", mPlain("Shepard's shows negative treatment: "+strings.Join(parts, "; ")+". See the court's language below.")))
	}

	for i, f := range rep.Flags {
		var ts []string
		for _, t := range f.Red {
			ts = append(ts, t.Treatment)
		}
		p = append(p, mp("Heading1", mPlain(fmt.Sprintf("Red flag %d: %s", i+1, strings.Join(ts, "; ")))))
		p = append(p, mp("Normal", mBold("Decision: "), mItal(f.Case), mPlain(fmt.Sprintf(", %s (%s, %s)", f.Citation, f.Court, f.Date))))
		src := "A later decision citing this case"
		if f.Source == "appellate history" {
			src = "This case's own appellate history (a higher court reviewed it)"
		}
		p = append(p, mp("Normal", mBold("Source: "), mPlain(src)))
		p = append(p, mp("Normal", mBold("Shepard's treatment: "), mPlain(strings.Join(ts, "; "))))
		if len(f.OtherLabels) > 0 {
			p = append(p, mp("Normal", mBold("Other treatment in the same decision: "), mPlain(strings.Join(f.OtherLabels, ", "))))
		}
		for _, t := range f.Red {
			if len(t.Where) > 0 {
				p = append(p, mp("Normal", mBold("Where: "), mPlain(strings.Join(t.Where, "; "))))
			}
		}
		if len(f.Headnotes) > 0 {
			hn := strings.Join(f.Headnotes, ", ")
			if len(rep.HeadnoteKey) > 0 {
				hn += " (" + uniqueTopics(rep.HeadnoteKey) + ")"
			}
			p = append(p, mp("Normal", mBold("Headnotes of this case affected: "), mPlain(hn)))
		}
		p = append(p, mp("Heading2", mPlain("Reason")))
		if strings.TrimSpace(f.Reason) != "" {
			p = append(p, mp("Normal", mPlain(f.Reason)))
		} else {
			p = append(p, mp("Normal", mItal("Not yet summarized. The court's own language is below.")))
		}
		if len(f.Language) > 0 {
			p = append(p, mp("Heading2", mPlain("The court's language")))
			for _, l := range f.Language {
				p = append(p, mp("Quote", mPlain("“"+l+"”")))
			}
		} else if f.Context != "" {
			p = append(p, mp("Heading2", mPlain("Lexis excerpt")))
			p = append(p, mp("Quote", mPlain(trunc(f.Context, 900))))
		}
		if f.SavedTo != "" {
			p = append(p, mp("Normal", mBold("Flagging decision saved as: "), mPlain(filepath.Base(f.SavedTo))))
		}
	}

	if len(rep.Counts) > 0 {
		var parts []string
		for _, k := range []string{"Warning", "Questioned", "Caution", "Positive", "Neutral", "Cited"} {
			if c, ok := rep.Counts[k]; ok {
				parts = append(parts, fmt.Sprintf("%s %d", k, c))
			}
		}
		p = append(p, mp("Heading1", mPlain("Other Shepard's activity")))
		p = append(p, mp("Normal", mPlain("Citing decisions by Shepard's level: "+strings.Join(parts, ", ")+". Only red (Warning) treatment is analyzed in this memo.")))
	}
	p = append(p, mp("Heading1", mPlain("Before relying on this")))
	p = append(p, mp("Small", mPlain("This memo is generated from Lexis+ Shepard's data and the flagging decisions' text. The reason is a summary of the court's language, not legal advice. Read the flagging decision and run a full Shepard's check in Lexis+ before citing this case.")))
	return p
}

// writeRedFlagMemo writes the memo next to the case file (dir) and caches the report so a
// plain-language reason can be added later without asking Lexis+ again.
func writeRedFlagMemo(rep *redFlagReport, dir string) (string, error) {
	if dir == "" {
		dir = defaultDownloadDir()
	}
	path := filepath.Join(dir, memoFileName(rep))
	if err := docx.Write(path, buildMemo(rep)); err != nil {
		return "", err
	}
	rep.Memo = path
	_ = saveRedFlagCache(rep)
	return path, nil
}

func redFlagCachePath(cite string) string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, "lexis-plus-pp-cli", "redflags", citeKey(cite)+".json")
}

func saveRedFlagCache(rep *redFlagReport) error {
	p := redFlagCachePath(rep.Cite)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

func loadRedFlagCache(cite string) (*redFlagReport, error) {
	b, err := os.ReadFile(redFlagCachePath(cite))
	if err != nil {
		return nil, fmt.Errorf("no red-flag report saved for %q; run `case get` or `case redflags` on it first", cite)
	}
	var rep redFlagReport
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, err
	}
	return &rep, nil
}

func newCaseRedflagMemoCmd(flags *rootFlags) *cobra.Command {
	var reasons []string
	var overall, out string
	cmd := &cobra.Command{
		Use:   "redflag-memo <citation>",
		Short: "Rewrite a case's red-flag memo with a plain-language bottom line and reason for each red flag",
		Long: "Rebuilds the Word memo from the red-flag report saved when the case was downloaded or checked; no Lexis+ request is made.\n" +
			"Give --reason once per red flag, in the order the flags appear, and optionally --overall for the bottom line.",
		Example: "  lexis-plus-pp-cli case redflag-memo \"23 Cal.3d 880\" --overall \"Overruled; do not cite for a private right of action under Ins. Code 790.03(h).\" " +
			"--reason \"Moradi-Shalal held the Royal Globe court misread the Legislature's intent and overruled it.\"",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:happy-args": "cite=23 Cal.3d 880"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "a case citation"); err != nil {
				return err
			}
			rep, err := loadRedFlagCache(args[0])
			if err != nil {
				return err
			}
			if len(reasons) > len(rep.Flags) {
				return usageErr(fmt.Errorf("%d --reason values given but the case has %d red flag(s)", len(reasons), len(rep.Flags)))
			}
			for i, r := range reasons {
				rep.Flags[i].Reason = strings.TrimSpace(r)
			}
			if cmd.Flags().Changed("overall") {
				rep.Overall = strings.TrimSpace(overall)
			}
			dir := out
			if dir == "" && rep.Memo != "" {
				dir = filepath.Dir(rep.Memo)
			}
			path, err := writeRedFlagMemo(rep, dir)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]string{"cite": rep.Cite, "memo": path}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "MEMO    %s\n        -> %s\n", rep.Cite, path)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&reasons, "reason", nil, "Plain-language reason for a red flag (repeat once per flag, in order)")
	cmd.Flags().StringVar(&overall, "overall", "", "Bottom line for the whole case")
	cmd.Flags().StringVar(&out, "out", "", "Folder for the memo (default: where it was first written)")
	return cmd
}
