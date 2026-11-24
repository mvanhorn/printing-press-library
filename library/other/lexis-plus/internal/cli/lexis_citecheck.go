// Hand-written: `citecheck` commands. They turn the user's own Lexis+ pulls into the source of
// truth for a citation audit: page-marked text, a quote/pin locator, and a brief-wide packet.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:client-call  (requests go through the hand-written lexisSession in lexis_session.go)

func newCitecheckCmd(flags *rootFlags) *cobra.Command {
	root := &cobra.Command{Use: "citecheck", Short: "Cite-check a brief against Lexis+: page-marked source text, quote and pin checks, Shepard's"}
	root.AddCommand(newCitecheckTextCmd(flags), newCitecheckQuoteCmd(flags), newCitecheckPacketCmd(flags))
	return root
}

// lazySession opens the Lexis+ session only when a command actually needs the network.
type lazySession struct {
	flags *rootFlags
	s     *lexisSession
	used  bool
}

func (l *lazySession) get() (*lexisSession, error) {
	if l.s != nil {
		return l.s, nil
	}
	s, err := newLexisSession(l.flags)
	if err != nil {
		return nil, err
	}
	l.s = s
	return s, nil
}

// pace keeps Lexis requests about 1.5 seconds apart.
func (l *lazySession) pace() {
	if l.used {
		time.Sleep(1500 * time.Millisecond)
	}
	l.used = true
}

func pulledDocx(cite string) (logEntry, bool) {
	k := citeKey(cite)
	var found logEntry
	ok := false
	for _, e := range readLog() {
		if e.Format == "docx" && citeKey(e.Cite) == k {
			if _, err := os.Stat(e.Path); err == nil {
				found, ok = e, true // latest pull wins
			}
		}
	}
	return found, ok
}

type ccSource struct {
	Cite     string   `json:"cite"`
	Kind     string   `json:"kind,omitempty"`
	Title    string   `json:"title,omitempty"`
	Court    string   `json:"court,omitempty"`
	Date     string   `json:"date,omitempty"`
	Docx     string   `json:"docx,omitempty"`
	Text     string   `json:"text_file,omitempty"`
	Pages    string   `json:"pages,omitempty"`
	Report   []string `json:"reporters,omitempty"`
	Sections []ccSeg  `json:"sections,omitempty"`
	Pulled   string   `json:"pulled,omitempty"` // "now" or "earlier (reused)"
	AsOf     string   `json:"lexis_copy_as_of,omitempty"`
	Signal   string   `json:"lexis_signal_when_pulled,omitempty"`
	Error    string   `json:"error,omitempty"`
	doc      *ccDoc
	auth     *authority
}

// ensureSource returns the parsed Lexis document for a citation, downloading it only if it is not already on disk.
// textDir: "" writes the text file next to the Word copy, "-" writes nothing, anything else is a folder that
// gets the text file and (when the Word copy lives elsewhere) a copy of the Word file.
func ensureSource(cmd *cobra.Command, flags *rootFlags, ls *lazySession, cite, out, matter string, refresh bool, textDir string) (*ccSource, error) {
	if st := extractCites(cite); len(st) == 1 && st[0].Kind == "statute" {
		cite = st[0].Cite // "Lab. Code, § 1102.5" -> "Cal Lab Code 1102.5"
	}
	src := &ccSource{Cite: cite}
	path := ""
	if !refresh {
		if e, ok := pulledDocx(cite); ok {
			path, src.Kind, src.Pulled = e.Path, e.Kind, "earlier (reused)"
		}
	}
	if path == "" {
		s, err := ls.get()
		if err != nil {
			return nil, err
		}
		ls.pace()
		r, err := fetchOne(cmd, flags, s, cite, "", fetchFlags{out: out, format: "docx", matter: matter, noRedflags: true})
		if err != nil {
			return nil, err
		}
		path, src.Kind, src.Pulled = r.Path, r.Kind, "now"
		src.auth = &authority{Cite: r.Cite, Title: r.Title, Kind: r.Kind, DocID: r.DocID}
	}
	d, err := parseLexisDocx(path)
	if err != nil {
		return nil, err
	}
	txt := ""
	if textDir != "-" {
		if textDir != "" && mustAbs(filepath.Dir(path)) != mustAbs(textDir) {
			if b, err := os.ReadFile(path); err == nil {
				cp := filepath.Join(textDir, filepath.Base(path))
				if err := os.WriteFile(cp, b, 0o644); err == nil {
					path = cp
				}
			}
		}
		txt = strings.TrimSuffix(path, filepath.Ext(path)) + ".citecheck.txt"
		if err := os.WriteFile(txt, []byte(d.renderText()), 0o644); err != nil {
			return nil, err
		}
	}
	d.Path = path
	src.doc, src.Docx, src.Text, src.AsOf = d, path, txt, d.AsOf
	src.Kind, src.Title, src.Court, src.Date, src.Report, src.Signal = d.Kind, d.Title, d.Court, d.Date, d.Reporters, d.Signal
	lo, hi := 0, 0
	for _, t := range d.toks {
		if n, err := strconv.Atoi(t.pg[0]); err == nil {
			if lo == 0 || n < lo {
				lo = n
			}
			if n > hi {
				hi = n
			}
		}
	}
	if lo > 0 {
		src.Pages = strconv.Itoa(lo) + "-" + strconv.Itoa(hi)
	}
	for _, s := range d.Segs {
		if s.Kind != "footnote" {
			src.Sections = append(src.Sections, s)
		}
	}
	return src, nil
}

func newCitecheckTextCmd(flags *rootFlags) *cobra.Command {
	var out, pages string
	var refresh bool
	cmd := &cobra.Command{
		Use:   "text <citation>...",
		Short: "Get a case or statute from Lexis+ as plain text with reporter page markers and speaker labels",
		Long: "Writes \"<name>.citecheck.txt\" next to the Word copy: the opinion with its [*123] reporter page markers kept, and a banner at the start of the\n" +
			"majority opinion, each concurrence, each dissent, and each block of Lexis editorial material. Reuses a copy already pulled unless --refresh.\n" +
			"With --pages, prints only the judge-written text on those official reporter pages.",
		Example: "  lexis-plus-pp-cli citecheck text \"27 Cal.3d 167\" --json\n" +
			"  lexis-plus-pp-cli citecheck text \"27 Cal.3d 167\" --pages 175-176",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:data-source": "auto", "pp:happy-args": "cite=27 Cal.3d 167", "pp:live-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source auto (reuses a copy already on disk; fetches from Lexis+ only when none is)
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "at least one citation"); err != nil {
				return err
			}
			ls := &lazySession{flags: flags}
			var res []ccSource
			failed := false
			for _, c := range args {
				src, err := ensureSource(cmd, flags, ls, c, out, "", refresh, "")
				if err != nil {
					failed = true
					res = append(res, ccSource{Cite: c, Error: err.Error()})
					continue
				}
				if pages != "" {
					lo, hi, _, ok := parsePin(pages)
					if !ok {
						return usageErr(fmt.Errorf("--pages wants a page or a range, e.g. 175 or 175-176"))
					}
					t := src.doc.pageText(lo, hi)
					if t == "" {
						t = "(no opinion text carries page " + pages + " in this document; it runs " + src.Pages + ")"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s, %s, page(s) %s\n%s\n", src.Title, c, pages, t)
					continue
				}
				res = append(res, *src)
			}
			if pages == "" {
				if flags.asJSON {
					if err := printJSONFiltered(cmd.OutOrStdout(), res, flags); err != nil {
						return err
					}
				} else {
					for _, r := range res {
						if r.Error != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "FAILED  %s: %s\n", r.Cite, r.Error)
							continue
						}
						fmt.Fprintf(cmd.OutOrStdout(), "TEXT    %s  %s  (pages %s; pulled %s)\n        -> %s\n", r.Cite, r.Title, r.Pages, r.Pulled, r.Text)
					}
				}
			}
			if failed {
				return fmt.Errorf("one or more citations failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", defaultDownloadDir(), "Folder for a newly pulled Word copy (default: ~/Downloads/Lexis Downloads)")
	cmd.Flags().StringVar(&pages, "pages", "", "Print only the opinion text on these official reporter pages (e.g. 175 or 175-176)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Pull a fresh copy from Lexis+ even if one is already on disk")
	return cmd
}

type ccQuoteInput struct {
	ID    string `json:"id,omitempty"`
	Quote string `json:"quote"`
	Pin   string `json:"pin,omitempty"`
}

func newCitecheckQuoteCmd(flags *rootFlags) *cobra.Command {
	var quotes, pins []string
	var input, file, out string
	cmd := &cobra.Command{
		Use:   "quote <citation>",
		Short: "Find a brief's quotation in the Lexis+ source: verbatim or not, which page, and who is speaking",
		Long: "Compares each quotation word for word with the Lexis+ copy of the authority. Brackets and ellipses in the brief are treated as signaled changes;\n" +
			"every other difference is listed. Reports the official reporter page the passage sits on (and checks --pin against it), and whether the words\n" +
			"come from the majority opinion, a concurrence, a dissent, a footnote, or Lexis editorial material. Works offline once the authority is pulled.\n" +
			"Use --input for several quotations at once: a JSON array of {\"id\",\"quote\",\"pin\"} (- for stdin).",
		Example: "  lexis-plus-pp-cli citecheck quote \"27 Cal.3d 167\" --quote \"an employer's authority over its employee does not include the right to demand that the employee commit a criminal act\" --pin 178\n" +
			"  lexis-plus-pp-cli citecheck quote \"27 Cal.3d 167\" --input quotes.json --json",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:data-source": "auto", "mcp:read-only": "true", "pp:happy-args": "cite=27 Cal.3d 167"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source auto (reuses a copy already on disk; fetches from Lexis+ only when none is)
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "a citation"); err != nil {
				return err
			}
			var in []ccQuoteInput
			if input != "" {
				var b []byte
				var err error
				if input == "-" {
					b, err = io.ReadAll(cmd.InOrStdin())
				} else {
					b, err = os.ReadFile(input)
				}
				if err != nil {
					return err
				}
				if err := json.Unmarshal(b, &in); err != nil {
					return usageErr(fmt.Errorf("--input must be a JSON array of {\"id\",\"quote\",\"pin\"}: %v", err))
				}
			}
			for i, q := range quotes {
				x := ccQuoteInput{Quote: q}
				if i < len(pins) {
					x.Pin = pins[i]
				}
				in = append(in, x)
			}
			if len(in) == 0 {
				return usageErr(fmt.Errorf("give at least one --quote, or --input"))
			}
			var d *ccDoc
			if file != "" {
				var err error
				if d, err = parseLexisDocx(file); err != nil {
					return err
				}
			} else {
				src, err := ensureSource(cmd, flags, &lazySession{flags: flags}, args[0], out, "", false, "-")
				if err != nil {
					return err
				}
				d = src.doc
			}
			type outT struct {
				Cite      string          `json:"cite"`
				Title     string          `json:"title"`
				Reporters []string        `json:"reporters,omitempty"`
				Results   []ccQuoteResult `json:"results"`
			}
			o := outT{Cite: args[0], Title: d.Title, Reporters: d.Reporters}
			for _, q := range in {
				o.Results = append(o.Results, d.checkQuote(q.ID, q.Quote, q.Pin))
			}
			if flags.asJSON {
				// --agent/--compact must never drop a failed pin, a speaker warning, or an
				// un-signaled difference just because only one quotation in the batch has one.
				return printJSONFilteredKeep(cmd.OutOrStdout(), o, flags, "id", "quote", "status", "fragments", "signals_in_brief", "pin", "flags")
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s, %s\n", d.Title, args[0])
			for _, r := range o.Results {
				fmt.Fprintf(w, "\n%s  %q\n", strings.ToUpper(strings.ReplaceAll(r.Status, "_", " ")), trunc(r.Quote, 90))
				for _, f := range r.Fragments {
					if f.Location == nil {
						fmt.Fprintf(w, "  not found: %q\n", trunc(f.Text, 90))
						continue
					}
					fmt.Fprintf(w, "  page %s, %s\n  source: %s\n", f.Location.Pages, f.Location.Speaker, f.Source)
					for _, dv := range f.Deviations {
						fmt.Fprintf(w, "  DIFFERENCE  brief: %q  source: %q\n", dv.Brief, dv.Source)
					}
				}
				if r.Pin != nil {
					m := "not checked"
					if r.Pin.Matches != nil {
						m = fmt.Sprintf("matches=%v", *r.Pin.Matches)
					}
					fmt.Fprintf(w, "  pin %s: %s %s\n", r.Pin.Given, m, r.Pin.Note)
				}
				for _, f := range r.Flags {
					fmt.Fprintf(w, "  NOTE  %s\n", f)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&quotes, "quote", nil, "A quotation exactly as the brief has it, brackets and ellipses included (repeatable)")
	cmd.Flags().StringArrayVar(&pins, "pin", nil, "The pin cite the brief gives for the matching --quote, e.g. 178 or \"172-173, fn. 8\" (repeatable, same order)")
	cmd.Flags().StringVar(&input, "input", "", "JSON file with an array of {\"id\",\"quote\",\"pin\"} (- for stdin)")
	cmd.Flags().StringVar(&file, "file", "", "Check against this Lexis+ Word file instead of looking the citation up")
	cmd.Flags().StringVar(&out, "out", defaultDownloadDir(), "Folder for a newly pulled Word copy")
	return cmd
}

// ---- brief-wide packet ----

const ccRep = `Cal\.\s?(?:App\.\s?)?(?:Supp\.\s?)?(?:[2-5](?:d|th))?|Cal\.\s?Rptr\.\s?(?:[23]d)?|P\.\s?(?:[23]d)?|U\.\s?S\.|S\.\s?Ct\.|L\.\s?Ed\.\s?(?:2d)?|F\.\s?(?:[23]d|4th)|F\.\s?Supp\.\s?(?:[23]d)?|F\.\s?R\.\s?D\.|B\.\s?R\.|Fed\.\s?Appx?\.`

var (
	ccFullRe     = regexp.MustCompile(`\b(\d{1,3})\s+(` + ccRep + `)\s+(\d{1,5})\b`)
	ccShortRe    = regexp.MustCompile(`\b(\d{1,3})\s+(` + ccRep + `)\s+at\s+(?:pp?\.\s*)?(\d{1,5}(?:\s?[-\x{2013}]\s?\d{1,5})?(?:,?\s*fn\.\s*\d+)?)`)
	ccPinAfterRe = regexp.MustCompile(`^,\s*(?:at\s+)?(?:pp?\.\s*)?(\d{1,5}(?:\s?[-\x{2013}]\s?\d{1,5})?(?:,?\s*fn\.\s*\d+)?)`)
	ccRepAheadRe = regexp.MustCompile(`^\s+(?:Cal\.|P\.|U\.\s?S\.|S\.\s?Ct\.|L\.\s?Ed\.|F\.|B\.\s?R\.|Fed\.)`)
	ccBetweenRe  = regexp.MustCompile(`^(?:,\s*(?:at\s+)?(?:pp?\.\s*)?\d[\d\s,\x{2013}-]*)?,\s*$`)
	ccBracketRe  = regexp.MustCompile(`^(?:\s*\d{1,3}\s+[A-Za-z][A-Za-z. 0-9]*?\s\d{1,5}(?:,\s*\d[\d\x{2013}-]*)?,?)*\s*$`)
	ccOtherRe    = regexp.MustCompile(`\b\d{4}\s+(?:WL|U\.\s?S\.\s?(?:Dist\.|App\.)?\s?LEXIS|Cal\.\s?(?:App\.\s?)?(?:Unpub\.\s?)?LEXIS)\s+\d+\b`)
	ccParallelOK = regexp.MustCompile(`^(?:Cal\.\s?Rptr|P\.|S\.\s?Ct|L\.\s?Ed)`)
)

type ccOccur struct {
	Pin     string `json:"pin,omitempty"`
	Short   bool   `json:"short_form,omitempty"`
	Context string `json:"context"`
}

type ccAuthority struct {
	Cite      string    `json:"cite"`
	Parallels []string  `json:"parallel_cites,omitempty"`
	Seen      []ccOccur `json:"occurrences"`
	*ccSource
	Shepards     *redFlagReport `json:"shepards,omitempty"`
	ShepardsRun  string         `json:"shepards_run,omitempty"`
	ShepardsNote string         `json:"shepards_note,omitempty"`
}

func ccContext(text string, s, e int) string {
	a, b := s-320, e+320
	if a < 0 {
		a = 0
	}
	if b > len(text) {
		b = len(text)
	}
	for a > 0 && text[a]&0xC0 == 0x80 {
		a--
	}
	for b < len(text) && text[b]&0xC0 == 0x80 {
		b++
	}
	return strings.TrimSpace(text[a:b])
}

func repKey(vol, rep string) string { return citeKey(vol + " " + rep) }

// briefAuthorities lists every case citation in a brief with each place it is cited (pin and surrounding text).
func briefAuthorities(text string) (auths []*ccAuthority, other []string) {
	text = strings.Join(strings.Fields(text), " ")
	byKey := map[string]*ccAuthority{}
	byRep := map[string][]*ccAuthority{}
	var prev *ccAuthority
	prevEnd := -1
	for _, m := range ccFullRe.FindAllStringSubmatchIndex(text, -1) {
		vol, rep, page := text[m[2]:m[3]], strings.Join(strings.Fields(text[m[4]:m[5]]), " "), text[m[6]:m[7]]
		cite := vol + " " + rep + " " + page
		// a parallel cite: inside "[...]" after the official cite, or right after it with only a pin between
		parallel := false
		if prev != nil && ccParallelOK.MatchString(rep) {
			between := text[prevEnd:m[0]]
			if ccBetweenRe.MatchString(between) {
				parallel = true
			} else if i := strings.LastIndex(between, "["); i >= 0 && !strings.Contains(between[i:], "]") && len(between)-i < 120 && ccBracketRe.MatchString(between[i+1:]) {
				parallel = true
			}
		}
		if parallel {
			dup := false
			for _, p := range prev.Parallels {
				dup = dup || citeKey(p) == citeKey(cite)
			}
			if !dup {
				prev.Parallels = append(prev.Parallels, cite)
			}
			prevEnd = m[1]
			continue
		}
		pin := ""
		end := m[1]
		if pm := ccPinAfterRe.FindStringSubmatchIndex(text[m[1]:]); pm != nil && !ccRepAheadRe.MatchString(text[m[1]+pm[1]:]) {
			pin = text[m[1]+pm[2] : m[1]+pm[3]]
			end = m[1] + pm[1]
		}
		k := citeKey(cite)
		a, ok := byKey[k]
		if !ok {
			a = &ccAuthority{Cite: cite}
			byKey[k] = a
			byRep[repKey(vol, rep)] = append(byRep[repKey(vol, rep)], a)
			auths = append(auths, a)
		}
		a.Seen = append(a.Seen, ccOccur{Pin: pin, Context: ccContext(text, m[0], end)})
		prev, prevEnd = a, end
	}
	// short forms: "27 Cal.3d at p. 178" attach to the one authority with that volume and reporter
	for _, m := range ccShortRe.FindAllStringSubmatchIndex(text, -1) {
		vol, rep := text[m[2]:m[3]], strings.Join(strings.Fields(text[m[4]:m[5]]), " ")
		if c := byRep[repKey(vol, rep)]; len(c) == 1 {
			c[0].Seen = append(c[0].Seen, ccOccur{Pin: text[m[6]:m[7]], Short: true, Context: ccContext(text, m[0], m[1])})
		} else {
			for _, a := range auths {
				for _, p := range a.Parallels {
					if strings.HasPrefix(citeKey(p), repKey(vol, rep)) {
						a.Seen = append(a.Seen, ccOccur{Pin: text[m[6]:m[7]] + " (" + rep + " pagination)", Short: true, Context: ccContext(text, m[0], m[1])})
					}
				}
			}
		}
	}
	seen := map[string]bool{}
	for _, o := range ccOtherRe.FindAllString(text, -1) {
		if !seen[o] {
			seen[o] = true
			other = append(other, o)
		}
	}
	return auths, other
}

// readBriefText reads a brief (.docx with its footnotes, or plain text; - for stdin).
func readBriefText(path string, stdin io.Reader) (string, error) {
	if path == "-" {
		b, err := io.ReadAll(stdin)
		return string(b), err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".docx":
		body, err := readZipEntry(path, "word/document.xml")
		if err != nil {
			return "", err
		}
		t := strings.Join(ccParas(string(body)), "\n")
		if fb, err := readZipEntry(path, "word/footnotes.xml"); err == nil {
			t += "\n" + strings.Join(ccParas(string(fb)), "\n")
		}
		return ccFnMark.ReplaceAllString(t, " "), nil
	case ".pdf", ".doc", ".rtf", ".pages":
		return "", usageErr(fmt.Errorf("%s: this command reads .docx or plain text; convert the brief to text first", filepath.Base(path)))
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

type ccPacket struct {
	Brief       string         `json:"brief"`
	Generated   string         `json:"generated"`
	Folder      string         `json:"folder"`
	Manifest    string         `json:"manifest"`
	Authorities []*ccAuthority `json:"authorities"`
	NotChecked  []string       `json:"cites_this_tool_cannot_resolve,omitempty"`
	Incomplete  []string       `json:"incomplete,omitempty"` // authorities not fetched, or whose requested Shepard's check did not run
	Note        string         `json:"note"`
}

func newCitecheckPacketCmd(flags *rootFlags) *cobra.Command {
	var out, matter string
	var max int
	var noShep, refresh bool
	cmd := &cobra.Command{
		Use:   "packet <brief>",
		Short: "Pull the Lexis+ source text and Shepard's status for every authority a brief cites, with each place it is cited",
		Long: "Reads a brief (.docx or .txt), lists every case and California statute it cites with each pin cite and the surrounding text, then for each\n" +
			"authority saves the Lexis+ Word copy and a page-marked text file and (for cases) runs Shepard's. Writes manifest.json in --out.\n" +
			"Authorities already pulled are reused. Requests are paced about 1.5 seconds apart; a run stops at --max authorities.",
		Example:     "  lexis-plus-pp-cli citecheck packet motion.docx --out ./citecheck/motion-sources --json",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "file=example-value", "pp:verified-by-proof": "brief-citecheck-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:data-source live
			if flags.dryRun {
				return dryRunNote(cmd, flags, args)
			}
			if err := needArgs(cmd, args, 1, "a brief (.docx or .txt)"); err != nil {
				return err
			}
			text, err := readBriefText(args[0], cmd.InOrStdin())
			if err != nil {
				return err
			}
			auths, other := briefAuthorities(text)
			collapsed := strings.Join(strings.Fields(text), " ")
			spans := statuteSpans(collapsed)
			for _, st := range extractCites(text) {
				if st.Kind != "statute" {
					continue
				}
				a := &ccAuthority{Cite: st.Cite}
				for i, sp := range spans[st.Cite] {
					if i >= 6 {
						break
					}
					a.Seen = append(a.Seen, ccOccur{Context: ccContext(collapsed, sp[0], sp[1])})
				}
				auths = append(auths, a)
			}
			if len(auths) == 0 {
				return usageErr(fmt.Errorf("no case or statute citations found in %s", args[0]))
			}
			if len(auths) > max {
				return usageErr(fmt.Errorf("%d authorities cited; --max is %d (Lexis+ terms do not allow bulk downloading). Raise --max deliberately, up to 60, or split the brief", len(auths), max))
			}
			if max > 60 {
				return usageErr(fmt.Errorf("--max cannot go above 60 in one run"))
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			ls := &lazySession{flags: flags}
			byDoc := map[string]*ccAuthority{}
			var kept []*ccAuthority
			for _, a := range auths {
				tries := append([]string{a.Cite}, a.Parallels...)
				var src *ccSource
				var lastErr error
				for _, c := range tries {
					src, lastErr = ensureSource(cmd, flags, ls, c, out, matter, refresh, out)
					if lastErr == nil {
						break
					}
					if strings.Contains(lastErr.Error(), "session missing or expired") {
						return lastErr
					}
				}
				if lastErr != nil {
					a.ccSource = &ccSource{Cite: a.Cite, Error: lastErr.Error()}
					kept = append(kept, a)
					continue
				}
				src.Cite = a.Cite
				a.ccSource = src
				// two citations that are the same Lexis document (e.g. a parallel cite) are one authority
				if first, ok := byDoc[src.Docx]; ok {
					first.Parallels = append(first.Parallels, a.Cite)
					first.Seen = append(first.Seen, a.Seen...)
					continue
				}
				byDoc[src.Docx] = a
				kept = append(kept, a)
				if src.Kind == "case" && !noShep {
					s, err := ls.get()
					if err != nil {
						return err
					}
					au := src.auth
					if au == nil || au.DocPath == "" {
						ls.pace()
						r, err := s.resolveCite(cmd.Context(), a.Cite)
						if err != nil {
							a.ShepardsNote = "Shepard's not run: " + err.Error()
							continue
						}
						r.Title = src.Title
						au = r
					}
					ls.pace()
					rep, err := s.redFlags(cmd.Context(), au, false)
					if err != nil {
						a.ShepardsNote = "Shepard's check failed: " + err.Error()
						continue
					}
					a.Shepards = rep
					a.ShepardsRun = time.Now().Format(time.RFC3339)
				}
			}
			abs := mustAbs(out)
			p := ccPacket{Brief: mustAbs(args[0]), Generated: time.Now().Format(time.RFC3339), Folder: abs, Manifest: filepath.Join(abs, "manifest.json"),
				Authorities: kept, NotChecked: other,
				Note: "Shepard's entries list red (Warning) treatment in full; citing_decision_counts gives the number of citing decisions at every level, including Caution and Questioned."}
			// An authority that could not be fetched, or whose requested Shepard's check did not
			// run, leaves the packet incomplete. The manifest keeps the partial results.
			var incomplete []string
			for _, a := range kept {
				switch {
				case a.Error != "":
					incomplete = append(incomplete, a.Cite+": not fetched")
				case a.Kind == "case" && !noShep && a.Shepards == nil:
					incomplete = append(incomplete, a.Cite+": Shepard's not run")
				}
			}
			p.Incomplete = incomplete
			b, _ := json.MarshalIndent(p, "", "  ")
			if err := os.WriteFile(p.Manifest, b, 0o644); err != nil {
				return err
			}
			incompleteErr := func() error {
				if len(incomplete) == 0 || flags.allowPartialFailure {
					return nil
				}
				return partialFailureErr(fmt.Errorf("packet incomplete: %d of %d authorities failed (%s); the manifest keeps the partial results. Pass --allow-partial-failure to exit 0 anyway", len(incomplete), len(kept), strings.Join(incomplete, "; ")))
			}
			if flags.asJSON {
				if err := printJSONFilteredKeep(cmd.OutOrStdout(), p, flags, "cite", "title", "kind", "error", "docx", "text_file", "occurrences", "shepards", "shepards_note"); err != nil {
					return err
				}
				return incompleteErr()
			}
			w := cmd.OutOrStdout()
			for _, a := range kept {
				if a.Error != "" {
					fmt.Fprintf(w, "FAILED  %s: %s\n", a.Cite, a.Error)
					continue
				}
				sig := ""
				if a.Shepards != nil {
					sig = "  Shepard's: " + strings.TrimPrefix(a.Shepards.Signal, "Caselaw")
					if len(a.Shepards.Flags) > 0 {
						sig += " (" + strconv.Itoa(len(a.Shepards.Flags)) + " red flag(s))"
					}
				}
				status := "READY  "
				if a.Kind == "case" && !noShep && a.Shepards == nil {
					status = "PARTIAL"
				}
				fmt.Fprintf(w, "%s %s  %s  cited %d time(s)%s\n", status, a.Cite, a.Title, len(a.Seen), sig)
				if a.ShepardsNote != "" {
					fmt.Fprintf(w, "        %s\n", a.ShepardsNote)
				}
			}
			for _, o := range other {
				fmt.Fprintf(w, "SKIPPED %s (not a reporter citation this tool can look up)\n", o)
			}
			fmt.Fprintf(w, "manifest -> %s\n", p.Manifest)
			if len(incomplete) > 0 {
				fmt.Fprintf(w, "INCOMPLETE: %d of %d authorities failed; see FAILED/PARTIAL lines above\n", len(incomplete), len(kept))
			}
			return incompleteErr()
		},
	}
	cmd.Flags().StringVar(&out, "out", defaultDownloadDir(), "Folder for the Word copies, text files, and manifest.json")
	cmd.Flags().StringVar(&matter, "matter", "", "Tag these pulls with a matter name")
	cmd.Flags().IntVar(&max, "max", 40, "Refuse to pull more than this many authorities in one run (hard limit 60)")
	cmd.Flags().BoolVar(&noShep, "no-shepards", false, "Skip the Shepard's check")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Pull fresh copies even of authorities already on disk")
	return cmd
}
