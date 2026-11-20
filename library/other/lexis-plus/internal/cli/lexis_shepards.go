// Hand-written: Shepard's red-flag report for a case (Warning-level negative treatment only).
// pp:data-source live
// pp:client-call  (requests go through the hand-written lexisSession in lexis_session.go)

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	shepWarningValue = "6" // Shepard's "Warning" (red) treatment group
	shepPageSize     = 50
	shepMaxItems     = 300
)

type redTreatment struct {
	Treatment string   `json:"treatment"`       // e.g. "Overruled by"
	Where     []string `json:"where,omitempty"` // reporter pages where the citing decision applies it
}

type redFlag struct {
	Source      string         `json:"source"` // "citing decision" or "appellate history"
	Case        string         `json:"case"`
	Citation    string         `json:"citation,omitempty"`
	Court       string         `json:"court,omitempty"`
	Date        string         `json:"date,omitempty"`
	DocID       string         `json:"doc_id,omitempty"`
	Red         []redTreatment `json:"red_treatments"`
	OtherLabels []string       `json:"other_treatments,omitempty"`
	Headnotes   []string       `json:"headnotes_affected,omitempty"`
	Context     string         `json:"lexis_summary,omitempty"`  // Lexis's own excerpt of the citing decision
	Language    []string       `json:"court_language,omitempty"` // passages from the citing decision that treat the flagged case
	SavedTo     string         `json:"saved_to,omitempty"`
	Reason      string         `json:"reason,omitempty"` // plain-language reason, added with `case redflag-memo`
}

type redFlagReport struct {
	Cite        string            `json:"cite"`
	Case        string            `json:"case"`
	Signal      string            `json:"signal"`   // Shepard's signal for the case, in words
	RedFlag     bool              `json:"red_flag"` // true when the case's own signal is Warning
	Counts      map[string]int    `json:"citing_decision_counts,omitempty"`
	HistoryFlag bool              `json:"negative_appellate_history,omitempty"`
	Flags       []redFlag         `json:"red_flags"`
	HeadnoteKey map[string]string `json:"headnote_topics,omitempty"`
	Note        string            `json:"note,omitempty"`
	Overall     string            `json:"bottom_line,omitempty"`
	CaseFile    string            `json:"case_file,omitempty"`
	Memo        string            `json:"memo,omitempty"`
}

var shepSignalWords = map[string]string{
	"CaselawWarning":    "Warning (red): negative treatment indicated",
	"CaselawCaution":    "Caution (yellow): possible negative treatment",
	"CaselawQuestioned": "Questioned (orange): validity questioned",
	"CaselawPositive":   "Positive (green)",
	"CaselawNeutral":    "Neutral",
	"CaselawCitedBy":    "Cited by (blue)",
}

func (s *lexisSession) postJSON(ctx context.Context, path string, body any) ([]byte, error) {
	raw, _ := json.Marshal(body)
	b, _, err := s.do(ctx, "POST", path, nil, raw, "application/json")
	return b, err
}

type shepItem struct {
	Title       string `json:"title"`
	DecidedDate string `json:"decidedDate"`
	Court       string `json:"court"`
	Category    struct {
		Label string `json:"label"`
	} `json:"category"`
	Link struct {
		Href string `json:"href"`
	} `json:"link"`
	DocSummary struct {
		FirstParallel string `json:"firstParallel"`
		Parallels     string `json:"parallels"`
		HeadNotes     []struct {
			ID string `json:"id"`
		} `json:"headNotes"`
		PinPoints []struct {
			Category string `json:"category"`
			Name     string `json:"name"`
			Chicklet struct {
				Title string `json:"title"`
			} `json:"chicklet"`
			PinPoints []struct {
				Cite    string `json:"cite"`
				PinPage string `json:"pinPage"`
			} `json:"pinPoints"`
		} `json:"pinPoints"`
	} `json:"docSummary"`
}

type shepReport struct {
	Signal     string     `json:"signal"`
	TotalItems int        `json:"totalResults"`
	Items      []shepItem `json:"items"`
	Topline    struct {
		CaseName string `json:"caseName"`
	} `json:"topline"`
	Request struct {
		CitingID string `json:"citingDecisionsShepardsId"`
	} `json:"shepardsRequest"`
}

func warningFilter() map[string]any {
	return map[string]any{
		"fromCache": false, "isPrefilter": false,
		"items": []any{map[string]any{
			"fieldName": "treatgroup", "info": "Warning", "name": "Analysis",
			"filters": []any{map[string]any{"count": 0, "fieldName": "treatmentno", "isApplied": true, "isEdited": false,
				"isExpand": false, "isNonDisplay": false, "text": "Warning", "value": shepWarningValue}},
		}},
	}
}

func shepBody(shepID string, category, start int, filters any) map[string]any {
	b := map[string]any{
		"masterFeatureId": lexisProductID, "reportCategory": category, "start": start, "num": fmt.Sprint(shepPageSize),
		"viewType": 1, "crb": true, "shepardsIdentifier": shepID, "sourceType": "", "isNewRequest": true, "isPrefilter": true,
		"keywords": "", "overRulingRisk": true, "clientDetails": map[string]string{"client": "-None-"}, "zoneInfo": nil,
	}
	if filters != nil {
		b["filters"] = filters
	}
	return b
}

var teaserTagRe = regexp.MustCompile(`<[^>]+>`)

func cleanTeaser(h string) string {
	t := html.UnescapeString(teaserTagRe.ReplaceAllString(h, " "))
	return strings.Join(strings.Fields(t), " ")
}

func redTreatments(it shepItem) (red []redTreatment, other []string) {
	lastRed := false // which list the previous treatment went to, for "in part"-style modifiers
	for _, p := range it.DocSummary.PinPoints {
		name := strings.TrimSpace(p.Name)
		// Shepard's sends qualifiers such as "in part" as their own entry right after the
		// treatment they modify ("Reversed by", "in part"); fold them back together.
		if lower := strings.ToLower(name); name != "" && name == lower && !strings.HasSuffix(lower, " by") {
			if lastRed && len(red) > 0 {
				red[len(red)-1].Treatment += " (" + name + ")"
			} else if !lastRed && len(other) > 0 {
				other[len(other)-1] += " (" + name + ")"
			}
			continue
		}
		isRed := p.Category == "WARNING_ANALYSIS" || strings.HasPrefix(strings.ToLower(p.Chicklet.Title), "red")
		if isRed {
			rt := redTreatment{Treatment: name}
			for _, pp := range p.PinPoints {
				w := pp.Cite
				if pp.PinPage != "" {
					w += " p." + pp.PinPage
				}
				rt.Where = append(rt.Where, w)
			}
			red = append(red, rt)
			lastRed = true
		} else if name != "" && name != "Cited by" {
			other = append(other, name)
			lastRed = false
		}
	}
	return
}

func courtName(it shepItem) string {
	if it.Category.Label != "" {
		return it.Category.Label
	}
	return it.Court
}

func itemToFlag(it shepItem, source string) (redFlag, bool) {
	red, other := redTreatments(it)
	if len(red) == 0 {
		return redFlag{}, false
	}
	f := redFlag{Source: source, Case: it.Title, Citation: it.DocSummary.FirstParallel, Court: courtName(it), Date: it.DecidedDate, Red: red, OtherLabels: other}
	if i := strings.Index(it.Link.Href, "urn:contentItem:"); i >= 0 {
		f.DocID = strings.TrimPrefix(it.Link.Href[i:], "urn:contentItem:")
	}
	for _, h := range it.DocSummary.HeadNotes {
		f.Headnotes = append(f.Headnotes, h.ID)
	}
	return f, true
}

// redFlags pulls the Shepard's red-flag (Warning-level) report for one case.
func (s *lexisSession) redFlags(ctx context.Context, a *authority, noExcerpts bool) (*redFlagReport, error) {
	if a.Kind != "case" {
		return nil, fmt.Errorf("%q is a statute; red-flag reports are for case opinions", a.Cite)
	}
	// 1. Shepard's report id and summary counts for this case.
	info, err := s.putJSON(ctx, "/f/RHPod/shepinfo", map[string]any{
		"persistentId": "urn:contentItem:" + a.DocID, "masterFeatureId": lexisProductID,
		"clientDetails": map[string]string{"client": "-None-"}, "isPeerRequest": true,
	})
	if err != nil {
		return nil, err
	}
	var si struct {
		ShepID string `json:"shepIdentifier"`
		Citing []struct {
			Count int    `json:"count"`
			Type  string `json:"type"`
		} `json:"citingdecisions"`
		History struct {
			Negative bool `json:"negativesubsequenthistory"`
			CaseNeg  bool `json:"negativecasehistory"`
		} `json:"historyinfo"`
	}
	if json.Unmarshal(info, &si) != nil || si.ShepID == "" {
		return nil, fmt.Errorf("Lexis+ has no Shepard's report for %q", a.Cite)
	}
	rep := &redFlagReport{Cite: a.Cite, Case: a.Title, Counts: map[string]int{}, Flags: []redFlag{}}
	warnCount := 0
	for _, c := range si.Citing {
		rep.Counts[c.Type] = c.Count
		if c.Type == "Warning" {
			warnCount = c.Count
		}
	}
	rep.HistoryFlag = si.History.Negative || si.History.CaseNeg

	headnotes := map[string]bool{}
	var teaserID string

	// 2. Warning-level citing decisions.
	if warnCount > 0 {
		for start := 1; start <= shepMaxItems; start += shepPageSize {
			b, err := s.postJSON(ctx, "/f/ShepardsRep/initial", shepBody(si.ShepID, 1, start, warningFilter()))
			if err != nil {
				return nil, err
			}
			var r shepReport
			if err := json.Unmarshal(b, &r); err != nil {
				return nil, fmt.Errorf("unexpected Shepard's response: %v", err)
			}
			if start == 1 {
				rep.Signal = r.Signal
				if r.Topline.CaseName != "" {
					rep.Case = r.Topline.CaseName
				}
				teaserID = r.Request.CitingID
			}
			for _, it := range r.Items {
				if f, ok := itemToFlag(it, "citing decision"); ok {
					for _, h := range f.Headnotes {
						headnotes[h] = true
					}
					rep.Flags = append(rep.Flags, f)
				}
			}
			if len(r.Items) < shepPageSize || start+shepPageSize > r.TotalItems {
				break
			}
			time.Sleep(700 * time.Millisecond)
		}
	}

	// 3. Negative appellate history of the case itself (reversed, vacated, etc.).
	if rep.HistoryFlag {
		b, err := s.postJSON(ctx, "/f/ShepardsRep/shephistory", shepBody(si.ShepID, 3, 1, nil))
		if err == nil {
			var r shepReport
			if json.Unmarshal(b, &r) == nil {
				if rep.Signal == "" {
					rep.Signal = r.Signal
				}
				if r.Topline.CaseName != "" && (rep.Case == "" || rep.Case == a.Cite) {
					rep.Case = r.Topline.CaseName
				}
				for _, it := range r.Items {
					if f, ok := itemToFlag(it, "appellate history"); ok {
						rep.Flags = append(rep.Flags, f)
					}
				}
			}
		}
	}

	// 4. Lexis's own summary excerpt of each citing decision.
	if teaserID != "" && len(rep.Flags) > 0 {
		if tb, _, err := s.do(ctx, "GET", "/f/ShepardsRepTeasers/citingdecisions/"+teaserID+"/teasers", nil, nil, ""); err == nil {
			var tj struct {
				Teasers []struct {
					PID   string `json:"persistenceId"`
					Value string `json:"teaserValue"`
				} `json:"teasers"`
			}
			if json.Unmarshal(tb, &tj) == nil {
				byID := map[string]string{}
				for _, t := range tj.Teasers {
					byID[strings.TrimPrefix(t.PID, "urn:contentItem:")] = cleanTeaser(t.Value)
				}
				for i := range rep.Flags {
					rep.Flags[i].Context = byID[rep.Flags[i].DocID]
				}
			}
		}
	}

	// 4b. The citing decision's own words about the flagged case.
	if !noExcerpts {
		s.addCourtLanguage(ctx, a, rep)
	}

	// 5. Topic labels for the headnotes of the flagged case that the red treatment touches.
	if len(headnotes) > 0 {
		var ids []string
		for h := range headnotes {
			ids = append(ids, h)
		}
		if hb, err := s.postJSON(ctx, "/f/ShepardsRepRetriever/headnotes", map[string]any{
			"displayName": strings.Join(ids, ","), "docFullPath": a.DocPath}); err == nil {
			var hj struct {
				List []struct {
					ID   string `json:"id"`
					Text string `json:"text"`
				} `json:"headnoteList"`
			}
			if json.Unmarshal(hb, &hj) == nil {
				rep.HeadnoteKey = map[string]string{}
				for _, h := range hj.List {
					rep.HeadnoteKey[h.ID] = h.Text
				}
			}
		}
	}

	if rep.Signal == "" {
		rep.Signal = signalFromType(si.Citing)
	}
	rep.RedFlag = strings.Contains(rep.Signal, "Warning")
	switch {
	case len(rep.Flags) == 0 && rep.HistoryFlag:
		rep.Note = "Lexis reports negative appellate history but returned no red-flag entries; check the Shepard's report on Lexis+."
	case len(rep.Flags) == 0:
		rep.Note = "No red flags: no Warning-level treatment from any citing decision or appellate history."
	}
	return rep, nil
}

func signalFromType(c []struct {
	Count int    `json:"count"`
	Type  string `json:"type"`
}) string {
	for _, t := range []string{"Warning", "Questioned", "Caution", "Positive", "Neutral"} {
		for _, x := range c {
			if x.Type == t && x.Count > 0 {
				return "Caselaw" + t
			}
		}
	}
	return ""
}

func (s *lexisSession) putJSON(ctx context.Context, path string, body any) ([]byte, error) {
	raw, _ := json.Marshal(body)
	b, _, err := s.do(ctx, "PUT", path, nil, raw, "application/json")
	return b, err
}

func newCaseRedflagsCmd(flags *rootFlags) *cobra.Command {
	var out string
	var download, noExcerpts, noMemo bool
	cmd := &cobra.Command{
		Use:   "redflags <citation>...",
		Short: "List only the Shepard's red flags (Warning-level negative treatment) for California cases, with the reason for each",
		Long: "For each case, asks Lexis+ for its Shepard's report and keeps only red (Warning) treatment: decisions that overrule, " +
			"abrogate, disapprove, or otherwise negatively treat it, and any negative appellate history. Each red flag shows the treatment " +
			"(e.g. \"Overruled by\"), the decision that applied it, where, the headnotes affected, and Lexis's own summary of that decision.\n" +
			"Yellow Caution and orange Questioned flags are left out on purpose. Use --download to also save each red-flag decision as Word.",
		Example:     "  lexis-plus-pp-cli case redflags \"23 Cal.3d 880\"\n  lexis-plus-pp-cli case redflags \"27 Cal.3d 167\" \"17 Cal.5th 46\" --json",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "cite=23 Cal.3d 880"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "at least one case citation"); err != nil {
				return err
			}
			s, err := newLexisSession(flags)
			if err != nil {
				return err
			}
			var reports []*redFlagReport
			failed := false
			for i, c := range args {
				if i > 0 {
					time.Sleep(1500 * time.Millisecond)
				}
				a, err := s.resolveCite(cmd.Context(), c)
				if err == nil {
					a.Title = a.Cite
					var rep *redFlagReport
					if rep, err = s.redFlags(cmd.Context(), a, noExcerpts); err == nil {
						if download {
							for j := range rep.Flags {
								fl := &rep.Flags[j]
								if fl.DocID == "" {
									continue
								}
								ca := &authority{Cite: fl.Citation, Title: fl.Case, Kind: "case", Collection: "cases", DocID: fl.DocID,
									DocPath: "/shared/document/cases/urn:contentItem:" + fl.DocID}
								time.Sleep(1500 * time.Millisecond)
								if p, derr := s.deliver(cmd.Context(), ca, deliverOpts{Format: "docx", Headnotes: true, Summary: true, OutDir: out}); derr == nil {
									fl.SavedTo = p
								}
							}
						}
						if !noMemo && len(rep.Flags) > 0 {
							if mpath, merr := writeRedFlagMemo(rep, out); merr == nil {
								rep.Memo = mpath
							}
						}
						reports = append(reports, rep)
						continue
					}
				}
				failed = true
				reports = append(reports, &redFlagReport{Cite: c, Flags: []redFlag{}, Note: "FAILED: " + err.Error()})
			}
			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), reports, flags); err != nil {
					return err
				}
			} else {
				printRedFlagText(cmd, reports)
			}
			if failed {
				return fmt.Errorf("one or more citations failed")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noExcerpts, "no-excerpts", false, "Skip reading the red-flag decisions for the court's own language (faster, fewer Lexis requests)")
	cmd.Flags().BoolVar(&download, "download", false, "Also save each red-flag decision as a Word file")
	cmd.Flags().StringVar(&out, "out", defaultDownloadDir(), "Folder for the red-flag memo and --download files")
	cmd.Flags().BoolVar(&noMemo, "no-memo", false, "Don't write the Word red-flag memo")
	return cmd
}

func printRedFlagText(cmd *cobra.Command, reports []*redFlagReport) {
	w := cmd.OutOrStdout()
	for _, r := range reports {
		sig := shepSignalWords[r.Signal]
		if sig == "" {
			sig = r.Signal
		}
		if sig == "" {
			sig = "none on file (no negative or positive signal)"
		}
		name := r.Case
		if name == "" || name == r.Cite {
			name = r.Cite
		} else {
			name += ", " + r.Cite
		}
		fmt.Fprintf(w, "%s\n  Shepard's signal: %s\n", name, sig)
		if r.Note != "" {
			fmt.Fprintf(w, "  %s\n", r.Note)
		}
		for i, f := range r.Flags {
			fmt.Fprintf(w, "  RED FLAG %d (%s): %s", i+1, f.Source, f.Case)
			if f.Citation != "" {
				fmt.Fprintf(w, ", %s", f.Citation)
			}
			fmt.Fprintf(w, " (%s %s)\n", f.Court, f.Date)
			for _, t := range f.Red {
				fmt.Fprintf(w, "    Treatment: %s", t.Treatment)
				if len(t.Where) > 0 {
					fmt.Fprintf(w, "  at %s", strings.Join(firstN(t.Where, 2), "; "))
				}
				fmt.Fprintln(w)
			}
			if len(f.Headnotes) > 0 {
				fmt.Fprintf(w, "    Headnotes affected: %s", strings.Join(f.Headnotes, ", "))
				if len(r.HeadnoteKey) > 0 {
					fmt.Fprintf(w, "  [%s]", uniqueTopics(r.HeadnoteKey))
				}
				fmt.Fprintln(w)
			}
			for _, l := range f.Language {
				fmt.Fprintf(w, "    Court's language: \"%s\"\n", l)
			}
			if f.Context != "" && len(f.Language) == 0 {
				fmt.Fprintf(w, "    Lexis summary: %s\n", trunc(f.Context, 600))
			}
			if f.SavedTo != "" {
				fmt.Fprintf(w, "    Saved: %s\n", f.SavedTo)
			}
		}
		if r.Memo != "" {
			fmt.Fprintf(w, "  MEMO -> %s\n", r.Memo)
		}
		fmt.Fprintln(w)
	}
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func uniqueTopics(m map[string]string) string {
	seen := map[string]bool{}
	var out []string
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return strings.Join(out, "; ")
}

const maxExcerptFlags = 8

var treatWordsRe = regexp.MustCompile(`(?i)overrul|disapprov|abrogat|supersed|reject|no longer|disavow|depart from|repudiat|decline to follow|not be followed|disagree|unsound|erroneous|incorrect|wrongly|mistaken|hereby`)
var starMarkRe = regexp.MustCompile(`\s*\[\*+\d*\]\s*`)
var sentenceSplitRe = regexp.MustCompile(`([.?!])\s+([A-Z\x{201C}"(])`)

// addCourtLanguage reads each red-flag decision (a temporary copy, not kept, not logged) and
// pulls the sentences where it discusses the flagged case together with its negative treatment.
func (s *lexisSession) addCourtLanguage(ctx context.Context, a *authority, rep *redFlagReport) {
	key, citeRe := flaggedCaseMatchers(rep.Case, a.Cite)
	tmp, err := os.MkdirTemp("", "lexis-redflag-")
	if err != nil {
		return
	}
	defer os.RemoveAll(tmp)
	for i := range rep.Flags {
		f := &rep.Flags[i]
		if i >= maxExcerptFlags || f.DocID == "" {
			continue
		}
		ca := &authority{Cite: f.Citation, Title: f.Case, Kind: "case", Collection: "cases", DocID: f.DocID,
			DocPath: "/shared/document/cases/urn:contentItem:" + f.DocID}
		time.Sleep(1500 * time.Millisecond)
		p, err := s.deliver(ctx, ca, deliverOpts{Format: "docx", OutDir: tmp, Name: "rf-" + f.DocID, NoLog: true})
		if err != nil {
			continue
		}
		text, err := readDocText(p)
		if err != nil {
			continue
		}
		if f.Source == "appellate history" {
			f.Language = dispositionPassages(text)
		} else {
			f.Language = treatmentPassages(text, key, citeRe)
		}
	}
}

func flaggedCaseMatchers(caseName, cite string) (string, *regexp.Regexp) {
	key := ""
	if i := strings.Index(caseName, " v. "); i > 0 {
		w := strings.Fields(caseName[:i])
		if len(w) > 2 {
			w = w[:2]
		}
		key = strings.Join(w, " ")
	}
	parts := strings.Fields(strings.NewReplacer(".", " ", ",", " ").Replace(cite))
	var re *regexp.Regexp
	if len(parts) >= 3 {
		re = regexp.MustCompile(`(?i)` + strings.Join(parts, `[\s.]*`))
	}
	return key, re
}

func treatmentPassages(text, key string, citeRe *regexp.Regexp) []string {
	var out []string
	seen := map[string]bool{}
	for _, para := range strings.Split(text, "\n") {
		para = strings.Join(strings.Fields(para), " ")
		if len(para) < 40 {
			continue
		}
		sentences := splitSentences(para)
		for i, sn := range sentences {
			mentions := (key != "" && strings.Contains(sn, key)) || (citeRe != nil && citeRe.MatchString(sn))
			if !mentions {
				continue
			}
			window := sn
			if treatWordsRe.MatchString(sn) {
				// good: the same sentence names the case and the treatment
			} else if i+1 < len(sentences) && treatWordsRe.MatchString(sentences[i+1]) {
				window = sn + " " + sentences[i+1]
			} else if i > 0 && treatWordsRe.MatchString(sentences[i-1]) {
				window = sentences[i-1] + " " + sn
			} else {
				continue
			}
			window = cleanQuote(window)
			dup := false
			for _, o := range out {
				if strings.Contains(o, window) || strings.Contains(window, o) {
					dup = true
					break
				}
			}
			if !dup && !seen[window] {
				seen[window] = true
				out = append(out, window)
			}
			if len(out) >= 3 {
				return out
			}
		}
	}
	return out
}

func splitSentences(p string) []string {
	var out []string
	last := 0
	for _, m := range sentenceSplitRe.FindAllStringSubmatchIndex(p, -1) {
		end := m[3] // end of the punctuation group
		out = append(out, strings.TrimSpace(p[last:end]))
		last = m[4]
	}
	out = append(out, strings.TrimSpace(p[last:]))
	return out
}

var dispositionRe = regexp.MustCompile(`(?i)\b(we|court)\b[^.]{0,40}\b(reverse|reversed|vacate|vacated|remand|remanded|disapprove|disapproved)\b|judgment[^.]{0,60}\bis (reversed|vacated)\b|\bis (reversed|vacated)\b|granted review (to|in order to)|granted review[^.]{0,80}(to decide|to determine|to consider)`)

// dispositionPassages pulls an appellate-history opinion's statement of the question it took up
// and its disposition ("we reverse"), which is the reason the earlier decision was reversed or vacated.
func dispositionPassages(text string) []string {
	var ours, others []string
	seen := func(list []string, sn string) bool {
		for _, o := range list {
			if strings.Contains(o, sn) || strings.Contains(sn, o) {
				return true
			}
		}
		return false
	}
	for _, para := range strings.Split(text, "\n") {
		para = strings.Join(strings.Fields(para), " ")
		if len(para) < 40 {
			continue
		}
		for _, sn := range splitSentences(para) {
			if !dispositionRe.MatchString(sn) {
				continue
			}
			sn = cleanQuote(sn)
			if seen(ours, sn) || seen(others, sn) {
				continue
			}
			// The reviewing court's own statements ("We granted review...", "we reverse")
			// explain the disposition; background recitals of earlier rulings come last.
			if weRe.MatchString(sn) {
				ours = append(ours, sn)
			} else {
				others = append(others, sn)
			}
		}
	}
	out := append(ours, others...)
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

var weRe = regexp.MustCompile(`(?i)\b(we|our)\b`)
var opinionHeadRe = regexp.MustCompile(`^(?:[A-Z][A-Za-z.' ]{0,40},\s*)?(?:C\.\s*)?J\.\s*[\x{2014}-]+\s*`)
var trailingFootnoteRe = regexp.MustCompile(`([.?!"\x{201D}])\s*\d{1,3}$`)

// cleanQuote strips star-page markers, the "J.—" opinion-author prefix, and a trailing footnote number.
func cleanQuote(s string) string {
	s = starMarkRe.ReplaceAllString(s, " ")
	s = opinionHeadRe.ReplaceAllString(strings.TrimSpace(s), "")
	s = trailingFootnoteRe.ReplaceAllString(s, "$1")
	return trunc(strings.TrimSpace(s), 650)
}
