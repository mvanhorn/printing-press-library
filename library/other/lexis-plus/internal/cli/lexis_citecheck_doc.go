// Hand-written: read a Lexis+ Word download into structured text for cite-checking
// (who is speaking, which reporter page, footnotes). No network.

package cli

import (
	"archive/zip"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	ccParaRe  = regexp.MustCompile(`(?s)<w:p[ >].*?</w:p>`)
	ccRunRe   = regexp.MustCompile(`(?s)<w:r[ >].*?</w:r>`)
	ccTextRe  = regexp.MustCompile(`<w:t(?: [^>]*)?>([^<]*)</w:t>|<w:tab ?/>|<w:br ?/>|<w:cr ?/>`)
	ccFnRefRe = regexp.MustCompile(`<w:footnoteReference [^>]*w:id="(-?\d+)"`)
	ccFnRe    = regexp.MustCompile(`(?s)<w:footnote [^>]*w:id="(-?\d+)"[^>]*>(.*?)</w:footnote>`)
	ccStarRe  = regexp.MustCompile(`\[(\*{1,4})(\d+)\]`)
	ccFnMark  = regexp.MustCompile("\x00fn:(-?\\d+)\x00")
	// Lexis editorial markers woven into opinion text: "HN7", "CA(1) (1)".
	ccEditRe  = regexp.MustCompile(`\bHN\d+\b|\bCA\(\d+[a-z]?\)(?:\s*\(\d+[a-z]?\))?`)
	ccScanRe  = regexp.MustCompile("\\[(\\*{1,4})(\\d+)\\]|\x00fn:(-?\\d+)\x00|[\\p{L}\\p{N}]+(?:['’][\\p{L}]+)*")
	ccFnNumRe = regexp.MustCompile(`^\s*(\d+[a-z]?)\s`)
	ccByRe    = regexp.MustCompile(`^(Opinion|Concur|Dissent) by:\s*(.*)$`)
	ccSigRe   = regexp.MustCompile(`^\s*(Warning|Questioned|Caution|Positive|Neutral|Cited)?\s*As of:\s*(.*)$`)
)

const ccSentinel = "\x00"

// ccSeg is one stretch of a Lexis document with a single speaker.
type ccSeg struct {
	Kind   string `json:"kind"`             // opinion | concurrence | dissent | footnote | caption | lexis-summary | lexis-headnotes | official-summary | statute | history | annotations
	Author string `json:"author,omitempty"` // judge named by Lexis ("Opinion by:", "Dissent by:")
	Fn     string `json:"footnote,omitempty"`
	Parent string `json:"footnote_of,omitempty"` // for footnotes: the kind of the passage that carries the footnote call
	First  string `json:"first_page,omitempty"`
	Last   string `json:"last_page,omitempty"`
	Text   string `json:"-"`
	fnID   string
}

// courtWords reports whether the segment is a judge's own writing (as opposed to Lexis editorial material).
func (s ccSeg) courtWords() bool {
	k := s.Kind
	if k == "footnote" {
		k = s.Parent
	}
	return k == "opinion" || k == "concurrence" || k == "dissent" || k == "statute"
}

func (s ccSeg) label() string {
	switch s.Kind {
	case "opinion":
		return "MAJORITY OPINION" + byAuthor(s.Author)
	case "concurrence":
		return "CONCURRENCE" + byAuthor(s.Author) + " (not the majority)"
	case "dissent":
		return "DISSENT" + byAuthor(s.Author) + " (not the holding)"
	case "footnote":
		return "FOOTNOTE " + s.Fn
	case "caption":
		return "CAPTION, HISTORY, COUNSEL (not opinion text)"
	case "lexis-summary":
		return "LEXIS CASE SUMMARY (Lexis editorial; not the court's words; never quote or cite)"
	case "lexis-headnotes":
		return "LEXIS HEADNOTES (Lexis editorial; not the court's words; never quote or cite)"
	case "official-summary":
		return "REPORTER SUMMARY AND HEADNOTES (reporter of decisions; not part of the opinion)"
	case "statute":
		return "STATUTE TEXT"
	case "history":
		return "STATUTORY HISTORY"
	case "annotations":
		return "DEERING'S ANNOTATIONS (editorial; not statute text)"
	}
	return strings.ToUpper(s.Kind)
}

func byAuthor(a string) string {
	if a == "" {
		return ""
	}
	return " by " + a
}

type ccTok struct {
	n    string // normalized word
	seg  int
	s, e int       // byte offsets in the segment text
	pg   [4]string // reporter page in effect, per star level (* = index 0)
}

type ccDoc struct {
	Path      string   `json:"docx"`
	Kind      string   `json:"kind"` // case | statute
	Title     string   `json:"title"`
	Court     string   `json:"court,omitempty"`
	Date      string   `json:"date,omitempty"`
	Signal    string   `json:"lexis_signal_when_pulled,omitempty"`
	AsOf      string   `json:"pulled_as_of,omitempty"`
	Reporters []string `json:"reporters,omitempty"` // index 0 is the "*" pagination, 1 is "**", ...
	FirstPage string   `json:"first_page,omitempty"`
	Segs      []ccSeg  `json:"sections"`
	toks      []ccTok
	grams     map[string][]int32
}

func readZipEntry(path, name string) ([]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, 256<<20))
		}
	}
	return nil, os.ErrNotExist
}

// ccParas turns WordprocessingML into paragraph strings. Footnote calls become "\x00fn:ID\x00".
func ccParas(x string) []string {
	var out []string
	for _, p := range ccParaRe.FindAllString(x, -1) {
		var b strings.Builder
		for _, r := range ccRunRe.FindAllString(p, -1) {
			if m := ccFnRefRe.FindStringSubmatch(r); m != nil {
				b.WriteString("\x00fn:" + m[1] + "\x00")
				continue
			}
			for _, t := range ccTextRe.FindAllStringSubmatch(r, -1) {
				if strings.HasPrefix(t[0], "<w:t") && !strings.HasPrefix(t[0], "<w:tab") {
					b.WriteString(t[1])
				} else {
					b.WriteString(" ")
				}
			}
		}
		s := html.UnescapeString(b.String())
		s = strings.ReplaceAll(s, " ", " ")
		out = append(out, s)
	}
	return out
}

func ccClean(p string) string {
	p = ccStarRe.ReplaceAllString(p, "")
	p = ccFnMark.ReplaceAllString(p, "")
	return strings.Join(strings.Fields(p), " ")
}

// parseLexisDocx reads a Lexis+ Word download (case or annotated statute).
func parseLexisDocx(path string) (*ccDoc, error) {
	body, err := readZipEntry(path, "word/document.xml")
	if err != nil {
		return nil, fmt.Errorf("%s is not a readable Word file: %w", path, err)
	}
	paras := ccParas(string(body))
	d := &ccDoc{Path: path, Kind: "statute"}
	for _, p := range paras {
		if ccClean(p) == "Reporter" {
			d.Kind = "case"
			break
		}
	}
	fn := map[string]string{}
	if fb, err := readZipEntry(path, "word/footnotes.xml"); err == nil {
		for _, m := range ccFnRe.FindAllStringSubmatch(string(fb), -1) {
			t := strings.TrimSpace(strings.Join(ccParas(m[2]), "\n"))
			if t != "" {
				fn[m[1]] = t
			}
		}
	}
	if d.Kind == "case" {
		d.parseCase(paras)
	} else {
		d.parseStatute(paras)
	}
	d.index(fn)
	return d, nil
}

func (d *ccDoc) parseCase(paras []string) {
	cur := ccSeg{Kind: "caption"}
	pendingAuthor := map[string]string{}
	carry := ""
	header := 0 // paragraphs of header consumed
	flush := func(next ccSeg) {
		if strings.TrimSpace(cur.Text) != "" {
			d.Segs = append(d.Segs, cur)
		}
		cur = next
		cur.Text = carry
		carry = ""
	}
	wantReporter := false
	for _, p := range paras {
		c := ccClean(p)
		if c == "" {
			// Lexis often puts a page marker in a paragraph of its own; keep it or the page count slips.
			if marks := ccStarRe.FindAllString(p, -1); len(marks) > 0 {
				cur.Text += strings.Join(marks, " ") + "\n"
			}
			continue
		}
		if c == "End of Document" {
			break
		}
		// header lines before the Reporter block
		if header >= 0 {
			if m := ccSigRe.FindStringSubmatch(c); m != nil && d.AsOf == "" && d.Title == "" {
				d.Signal, d.AsOf = m[1], m[2]
				continue
			}
			if c == "Reporter" {
				wantReporter = true
				header = -1
				continue
			}
			switch header {
			case 0:
				d.Title = c
			case 1:
				d.Court = c
			case 2:
				parts := strings.Split(c, ";")
				d.Date = strings.TrimSpace(parts[0])
				for _, x := range parts {
					if strings.Contains(x, "Decided") {
						d.Date = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(x), ", Decided"))
					}
				}
			}
			header++
			cur.Text += p + "\n"
			continue
		}
		if wantReporter {
			wantReporter = false
			for _, part := range strings.Split(c, ";") {
				part = strings.TrimSpace(part)
				stars := len(part) - len(strings.TrimRight(part, "*"))
				if stars == 0 || stars > 4 {
					continue
				}
				for len(d.Reporters) < stars {
					d.Reporters = append(d.Reporters, "")
				}
				d.Reporters[stars-1] = strings.TrimSpace(strings.TrimRight(part, "* "))
			}
			if len(d.Reporters) > 0 {
				f := strings.Fields(d.Reporters[0])
				if len(f) > 0 {
					d.FirstPage = f[len(f)-1]
				}
			}
			cur.Text += p + "\n"
			continue
		}
		marks := strings.Join(ccStarRe.FindAllString(p, -1), " ")
		if m := ccByRe.FindStringSubmatch(c); m != nil {
			pendingAuthor[m[1]] = strings.TrimSpace(m[2])
			carry += marks
			continue
		}
		switch c {
		case "Case Summary":
			carry += marks
			flush(ccSeg{Kind: "lexis-summary"})
			continue
		case "LexisNexis® Headnotes", "LexisNexis Headnotes":
			carry += marks
			flush(ccSeg{Kind: "lexis-headnotes"})
			continue
		case "Headnotes/Summary", "Headnotes/Syllabus", "Syllabus":
			carry += marks
			flush(ccSeg{Kind: "official-summary"})
			continue
		case "Opinion":
			carry += marks
			flush(ccSeg{Kind: "opinion", Author: pendingAuthor["Opinion"]})
			continue
		case "Concur":
			carry += marks
			flush(ccSeg{Kind: "concurrence", Author: pendingAuthor["Concur"]})
			continue
		case "Dissent":
			carry += marks
			flush(ccSeg{Kind: "dissent", Author: pendingAuthor["Dissent"]})
			continue
		}
		if (strings.HasPrefix(c, "Counsel:") || strings.HasPrefix(c, "Judges:")) && cur.Kind != "opinion" && cur.Kind != "concurrence" && cur.Kind != "dissent" && cur.Kind != "caption" {
			flush(ccSeg{Kind: "caption"})
		}
		cur.Text += p + "\n"
	}
	flush(ccSeg{})
}

var ccSecHeadRe = regexp.MustCompile(`^§+\s*[\d.]+[a-z]?\.`)

func (d *ccDoc) parseStatute(paras []string) {
	cur := ccSeg{Kind: "caption"}
	flush := func(next ccSeg) {
		if strings.TrimSpace(cur.Text) != "" {
			d.Segs = append(d.Segs, cur)
		}
		cur = next
	}
	for _, p := range paras {
		c := ccClean(p)
		if c == "" {
			continue
		}
		if c == "End of Document" {
			break
		}
		if d.Title == "" {
			d.Title = c
		}
		switch {
		case cur.Kind == "caption" && ccSecHeadRe.MatchString(c):
			flush(ccSeg{Kind: "statute"})
		case cur.Kind == "statute" && c == "History":
			flush(ccSeg{Kind: "history"})
			continue
		case (cur.Kind == "statute" || cur.Kind == "history") && (c == "Annotations" || c == "Notes to Decisions" || c == "Notes"):
			flush(ccSeg{Kind: "annotations"})
		}
		cur.Text += p + "\n"
	}
	flush(ccSeg{})
}

// index tokenizes every segment, tracks reporter pages, and appends footnotes as their own segments.
func (d *ccDoc) index(fn map[string]string) {
	var pg [4]string
	pg[0] = d.FirstPage
	type fnAt struct {
		pg     [4]string
		parent string
		author string
	}
	at := map[string]fnAt{}
	var order []string
	scan := func(si int, startPg [4]string) [4]string {
		seg := &d.Segs[si]
		p := startPg
		seg.First = p[0]
		txt := seg.Text
		if d.Kind == "case" {
			txt = ccEditRe.ReplaceAllStringFunc(txt, func(m string) string { return strings.Repeat(" ", len(m)) })
		}
		first := true
		for _, m := range ccScanRe.FindAllStringSubmatchIndex(txt, -1) {
			switch {
			case m[2] >= 0: // star page marker
				lvl := m[3] - m[2] - 1
				p[lvl] = txt[m[4]:m[5]]
				if first && lvl == 0 {
					seg.First = p[0]
				}
			case m[6] >= 0: // footnote call
				id := txt[m[6]:m[7]]
				if _, ok := at[id]; !ok {
					parent := seg.Kind
					if seg.Kind == "footnote" {
						parent = seg.Parent
					}
					at[id] = fnAt{pg: p, parent: parent, author: seg.Author}
					order = append(order, id)
				}
			default:
				first = false
				d.toks = append(d.toks, ccTok{n: ccNorm(txt[m[0]:m[1]]), seg: si, s: m[0], e: m[1], pg: p})
			}
		}
		seg.Last = p[0]
		d.toks = append(d.toks, ccTok{n: ccSentinel, seg: si})
		return p
	}
	body := len(d.Segs)
	for i := 0; i < body; i++ {
		pg = scan(i, pg)
	}
	// Footnotes that were never called from the body still get listed (page unknown).
	var rest []string
	for id := range fn {
		if _, ok := at[id]; !ok {
			rest = append(rest, id)
		}
	}
	sort.Slice(rest, func(i, j int) bool { a, _ := strconv.Atoi(rest[i]); b, _ := strconv.Atoi(rest[j]); return a < b })
	order = append(order, rest...)
	for _, id := range order {
		t, ok := fn[id]
		if !ok {
			continue
		}
		a := at[id]
		num := id
		if m := ccFnNumRe.FindStringSubmatch(t); m != nil {
			num = m[1]
			t = strings.TrimSpace(t[len(m[0]):])
		}
		d.Segs = append(d.Segs, ccSeg{Kind: "footnote", Fn: num, Parent: a.parent, Author: a.author, Text: t, fnID: id})
		scan(len(d.Segs)-1, a.pg)
	}
}

func ccNorm(w string) string {
	w = strings.ToLower(w)
	w = strings.NewReplacer("'", "", "’", "").Replace(w)
	return w
}

// renderText writes the document as plain text with speaker banners and reporter page markers kept.
func (d *ccDoc) renderText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.Title)
	if d.Court != "" {
		fmt.Fprintf(&b, "%s, %s\n", d.Court, d.Date)
	}
	if len(d.Reporters) > 0 {
		b.WriteString("Page markers: ")
		for i, r := range d.Reporters {
			if r != "" {
				fmt.Fprintf(&b, "[%s123] = page of %s; ", strings.Repeat("*", i+1), r)
			}
		}
		b.WriteString("\nA marker shows where that page BEGINS. Text before the first marker is on the first page.\n")
	}
	if d.AsOf != "" {
		fmt.Fprintf(&b, "Lexis signal when this copy was pulled (%s): %s\n", d.AsOf, d.Signal)
	}
	wroteFn := false
	for _, s := range d.Segs {
		s.Text = d.fnLabels(s.Text)
		if s.Kind == "footnote" {
			if !wroteFn {
				b.WriteString("\n===== FOOTNOTES =====\n")
				wroteFn = true
			}
			fmt.Fprintf(&b, "\nfn %s (footnote to the %s, called on page %s):\n%s\n", s.Fn, s.Parent, s.First, s.Text)
			continue
		}
		fmt.Fprintf(&b, "\n===== %s =====\n", s.label())
		b.WriteString(s.Text)
	}
	return b.String()
}

// pageText returns the judge-written text that falls on the given official reporter pages.
func (d *ccDoc) pageText(lo, hi int) string {
	var b strings.Builder
	lastSeg := -1
	start, end := -1, -1
	emit := func() {
		if lastSeg >= 0 && start >= 0 {
			s := d.Segs[lastSeg]
			for end < len(s.Text) && !strings.ContainsRune(" \n\t[\x00", rune(s.Text[end])) {
				end++ // keep the closing punctuation of the last word
			}
			fmt.Fprintf(&b, "\n--- %s ---\n%s\n", s.label(), strings.TrimSpace(d.fnLabels(s.Text[start:end])))
		}
		start, end = -1, -1
	}
	for _, t := range d.toks {
		if t.n == ccSentinel {
			emit()
			lastSeg = -1
			continue
		}
		p, err := strconv.Atoi(t.pg[0])
		in := err == nil && p >= lo && p <= hi && d.Segs[t.seg].courtWords()
		if !in {
			emit()
			continue
		}
		if start < 0 {
			start = t.s
			lastSeg = t.seg
		}
		end = t.e
	}
	emit()
	return strings.TrimSpace(b.String())
}

// fnLabels replaces footnote-call markers with the footnote's printed number, e.g. " [fn 3] ".
func (d *ccDoc) fnLabels(t string) string {
	return ccFnMark.ReplaceAllStringFunc(t, func(m string) string {
		id := ccFnMark.FindStringSubmatch(m)[1]
		for _, s := range d.Segs {
			if s.Kind == "footnote" && s.fnID == id {
				return " [fn " + s.Fn + "] "
			}
		}
		return " [fn] "
	})
}
