// Hand-written: locate a brief's quotation in a Lexis document, report the reporter page and
// speaker, and list every un-signaled difference from the source text. No network.

package cli

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type qItem struct {
	n    string
	raw  string
	soft bool // word carrying a bracketed (signaled) change, e.g. "[T]he"
	wild bool // standalone bracketed insertion, e.g. "[the employer]"
}

var (
	qNoteRe     = regexp.MustCompile(`(?i)[\[(]\s*(sic|emphasis added|emphasis in original|emphasis omitted|italics added|italics omitted|italics in original|internal citations? omitted|citations? omitted|fn\.? omitted|footnotes? omitted|internal quotation marks omitted|internal quotations omitted|cleaned up)\s*\.?[\])]`)
	qEllipsisRe = regexp.MustCompile(`\s*(?:\.\s?){3,4}\s*|\s*\x{2026}\.?\s*|\s*\*\s\*\s\*\s*`)
	qAttachRe   = regexp.MustCompile(`([\p{L}\p{N}]*)\[([^\]\s]*)\]([\p{L}\p{N}]*)`)
	qScanRe     = regexp.MustCompile("\\[[^\\]]*\\]|\x01?[\\p{L}\\p{N}]+(?:['’][\\p{L}]+)*")
	qPinRe      = regexp.MustCompile(`(\d{1,5})(?:\s?[-\x{2013}]\s?(\d{1,5}))?`)
	qPinFnRe    = regexp.MustCompile(`(?i)(?:fn\.?|n\.|note)\s*(\d+)`)
)

// parseQuote splits a brief's quotation at its ellipses and marks bracketed (signaled) changes.
func parseQuote(q string) (frags [][]qItem, raws []string, notes []string) {
	q = strings.NewReplacer("“", `"`, "”", `"`, "‘", "'", "’", "'", " ", " ").Replace(q)
	for _, m := range qNoteRe.FindAllStringSubmatch(q, -1) {
		notes = append(notes, "brief signals: "+strings.ToLower(m[1]))
	}
	q = qNoteRe.ReplaceAllString(q, " ")
	parts := qEllipsisRe.Split(q, -1)
	if len(parts) > 1 {
		notes = append(notes, "brief signals an omission (ellipsis) "+strconv.Itoa(len(parts)-1)+" time(s)")
	}
	for _, part := range parts {
		raw := strings.TrimSpace(part)
		work := qAttachRe.ReplaceAllStringFunc(part, func(m string) string {
			g := qAttachRe.FindStringSubmatch(m)
			if g[1] == "" && g[3] == "" {
				return m // standalone bracket: handled as an insertion below
			}
			return " \x01" + g[1] + g[2] + g[3] + " "
		})
		var items []qItem
		for _, t := range qScanRe.FindAllString(work, -1) {
			switch {
			case strings.HasPrefix(t, "["):
				items = append(items, qItem{wild: true, raw: t})
			case strings.HasPrefix(t, "\x01"):
				items = append(items, qItem{n: ccNorm(t[1:]), raw: t[1:], soft: true})
			default:
				items = append(items, qItem{n: ccNorm(t), raw: t})
			}
		}
		for len(items) > 0 && items[0].wild {
			items = items[1:]
		}
		for len(items) > 0 && items[len(items)-1].wild {
			items = items[:len(items)-1]
		}
		if len(items) == 0 {
			continue
		}
		frags = append(frags, items)
		raws = append(raws, raw)
	}
	for _, f := range frags {
		for _, it := range f {
			if it.soft || it.wild {
				notes = append(notes, "brief signals a bracketed change: "+it.raw)
			}
		}
	}
	return frags, raws, notes
}

const ccWildMax = 12

func matchFrom(items []qItem, k int, toks []ccTok, i int) (int, bool) {
	for k < len(items) {
		it := items[k]
		if it.wild {
			for skip := 0; skip <= ccWildMax; skip++ {
				if e, ok := matchFrom(items, k+1, toks, i+skip); ok {
					return e, true
				}
				if i+skip >= len(toks) || toks[i+skip].n == ccSentinel {
					break
				}
			}
			return 0, false
		}
		if i >= len(toks) || toks[i].n == ccSentinel {
			return 0, false
		}
		if !it.soft && toks[i].n != it.n {
			return 0, false
		}
		i++
		k++
	}
	return i, true
}

type ccDeviation struct {
	Brief  string `json:"brief"`  // words as the brief has them ("" = the brief left words out)
	Source string `json:"source"` // words as the source has them ("" = the brief added words)
}

type ccHit struct {
	start, end int // token span [start,end)
	exact      bool
	sim        float64
	devs       []ccDeviation
}

func hardCount(items []qItem) int {
	n := 0
	for _, it := range items {
		if !it.wild {
			n++
		}
	}
	return n
}

func (d *ccDoc) exactHits(items []qItem) []ccHit {
	var out []ccHit
	for i := range d.toks {
		if d.toks[i].n == ccSentinel {
			continue
		}
		if !items[0].soft && d.toks[i].n != items[0].n {
			continue
		}
		if e, ok := matchFrom(items, 0, d.toks, i); ok {
			out = append(out, ccHit{start: i, end: e, exact: true, sim: 1})
			if len(out) >= 25 {
				break
			}
		}
	}
	return out
}

func (d *ccDoc) gramIndex() map[string][]int32 {
	if d.grams != nil {
		return d.grams
	}
	d.grams = map[string][]int32{}
	for i := 0; i+2 < len(d.toks); i++ {
		a, b, c := d.toks[i].n, d.toks[i+1].n, d.toks[i+2].n
		if a == ccSentinel || b == ccSentinel || c == ccSentinel {
			continue
		}
		k := a + " " + b + " " + c
		d.grams[k] = append(d.grams[k], int32(i))
	}
	return d.grams
}

// nearHits finds the closest passages when the quotation does not appear word for word.
func (d *ccDoc) nearHits(items []qItem) []ccHit {
	if hardCount(items) < 5 {
		return nil
	}
	idx := d.gramIndex()
	votes := map[int]int{}
	const bucket = 16
	for k := 0; k+2 < len(items); k++ {
		a, b, c := items[k], items[k+1], items[k+2]
		if a.wild || b.wild || c.wild || a.soft || b.soft || c.soft {
			continue
		}
		for _, p := range idx[a.n+" "+b.n+" "+c.n] {
			off := int(p) - k
			if off < 0 {
				off = 0
			}
			votes[off/bucket]++
		}
	}
	type bv struct{ b, v int }
	var bs []bv
	for b, v := range votes {
		bs = append(bs, bv{b, v + votes[b+1]})
	}
	sort.Slice(bs, func(i, j int) bool {
		if bs[i].v != bs[j].v {
			return bs[i].v > bs[j].v
		}
		return bs[i].b < bs[j].b
	})
	var out []ccHit
	seen := map[int]bool{}
	for n, x := range bs {
		if n >= 4 {
			break
		}
		lo := x.b*bucket - 20
		hi := (x.b+2)*bucket + len(items) + 20 + 3*ccWildMax
		if lo < 0 {
			lo = 0
		}
		if hi > len(d.toks) {
			hi = len(d.toks)
		}
		h, ok := d.alignWithinSections(items, lo, hi)
		if !ok || seen[h.start] {
			continue
		}
		seen[h.start] = true
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].sim > out[j].sim })
	return out
}

// alignWithinSections aligns the quotation separately inside each run of tokens between section
// sentinels in toks[lo:hi] and returns the best hit. A near match therefore never spans two
// sections (an opinion and a dissent, or a footnote and the body), so its source text and speaker
// always come from one section.
func (d *ccDoc) alignWithinSections(items []qItem, lo, hi int) (ccHit, bool) {
	var best ccHit
	found := false
	start := lo
	for i := lo; i <= hi; i++ {
		if i < hi && d.toks[i].n != ccSentinel {
			continue
		}
		if i > start {
			if h, ok := d.align(items, start, i); ok && (!found || h.sim > best.sim) {
				best, found = h, true
			}
		}
		start = i + 1
	}
	return best, found
}

// align runs a word-level alignment of the quotation against toks[lo:hi] (free ends on the source side).
// Callers pass a window with no section sentinel inside it (see alignWithinSections).
func (d *ccDoc) align(items []qItem, lo, hi int) (ccHit, bool) {
	w := d.toks[lo:hi]
	m, n := len(items), len(w)
	if m == 0 || n == 0 {
		return ccHit{}, false
	}
	const (
		opDiag = iota + 1
		opUp
		opLeft
	)
	D := make([][]int32, m+1)
	B := make([][]uint8, m+1)
	for i := range D {
		D[i] = make([]int32, n+1)
		B[i] = make([]uint8, n+1)
	}
	for i := 1; i <= m; i++ {
		c := int32(1)
		if items[i-1].wild {
			c = 0
		}
		D[i][0] = D[i-1][0] + c
		B[i][0] = opUp
	}
	for i := 1; i <= m; i++ {
		it := items[i-1]
		for j := 1; j <= n; j++ {
			sub := int32(1)
			if it.wild || it.soft || (w[j-1].n == it.n && w[j-1].n != ccSentinel) {
				sub = 0
			}
			best, op := D[i-1][j-1]+sub, uint8(opDiag)
			up := int32(1)
			if it.wild {
				up = 0
			}
			if v := D[i-1][j] + up; v < best {
				best, op = v, opUp
			}
			left := int32(1)
			if it.wild || (i < m && items[i].wild) {
				left = 0
			}
			if v := D[i][j-1] + left; v < best {
				best, op = v, opLeft
			}
			D[i][j], B[i][j] = best, op
		}
	}
	bj := 1
	for j := 1; j <= n; j++ {
		if D[m][j] < D[m][bj] {
			bj = j
		}
	}
	hard := hardCount(items)
	sim := 1 - float64(D[m][bj])/float64(hard)
	if sim < 0.6 {
		return ccHit{}, false
	}
	// backtrace
	type step struct {
		b, s string
		same bool
	}
	var steps []step
	i, j := m, bj
	first, last := -1, -1
	for i > 0 {
		switch B[i][j] {
		case opDiag:
			it := items[i-1]
			t := w[j-1]
			same := it.wild || it.soft || t.n == it.n
			steps = append(steps, step{it.raw, d.Segs[t.seg].Text[t.s:t.e], same})
			if last < 0 {
				last = j - 1
			}
			first = j - 1
			i--
			j--
		case opUp:
			if !items[i-1].wild {
				steps = append(steps, step{items[i-1].raw, "", false})
			}
			i--
		default: // opLeft
			t := w[j-1]
			free := items[i-1].wild || (i < m && items[i].wild)
			if t.n != ccSentinel {
				steps = append(steps, step{"", d.Segs[t.seg].Text[t.s:t.e], free})
			}
			j--
		}
	}
	if first < 0 {
		return ccHit{}, false
	}
	h := ccHit{start: lo + first, end: lo + last + 1, sim: sim}
	var cur *ccDeviation
	for k := len(steps) - 1; k >= 0; k-- {
		st := steps[k]
		if st.same {
			cur = nil
			continue
		}
		if cur == nil {
			h.devs = append(h.devs, ccDeviation{})
			cur = &h.devs[len(h.devs)-1]
		}
		if st.b != "" {
			cur.Brief = strings.TrimSpace(cur.Brief + " " + st.b)
		}
		if st.s != "" {
			cur.Source = strings.TrimSpace(cur.Source + " " + st.s)
		}
	}
	h.exact = len(h.devs) == 0
	return h, true
}

// ---- results ----

type ccLocation struct {
	Section  string            `json:"section"` // opinion | concurrence | dissent | lexis-summary | ...
	Speaker  string            `json:"speaker"` // plain-language label
	Author   string            `json:"author,omitempty"`
	Footnote string            `json:"footnote,omitempty"`
	Pages    string            `json:"pages,omitempty"` // official reporter pages the passage sits on
	Parallel map[string]string `json:"parallel_pages,omitempty"`
}

type ccFragResult struct {
	Text       string        `json:"brief_text"`
	Status     string        `json:"status"` // verbatim | differs | not_found
	Similarity float64       `json:"similarity,omitempty"`
	Location   *ccLocation   `json:"location,omitempty"`
	Source     string        `json:"source_text,omitempty"` // the passage exactly as the source has it
	Deviations []ccDeviation `json:"unsignaled_differences,omitempty"`
	Others     []ccLocation  `json:"also_appears_at,omitempty"`
}

type ccPinResult struct {
	Given   string `json:"given"`
	Matches *bool  `json:"matches,omitempty"` // absent when the pin could not be checked mechanically
	FoundOn string `json:"found_on,omitempty"`
	Note    string `json:"note,omitempty"`
}

type ccQuoteResult struct {
	ID        string         `json:"id,omitempty"`
	Quote     string         `json:"quote"`
	Status    string         `json:"status"` // verbatim | verbatim_with_signaled_changes | differs | partly_found | not_found
	Fragments []ccFragResult `json:"fragments"`
	Signals   []string       `json:"signals_in_brief,omitempty"`
	Pin       *ccPinResult   `json:"pin,omitempty"`
	Flags     []string       `json:"flags,omitempty"`
}

func (d *ccDoc) pagesOf(start, end, lvl int) []string {
	var out []string
	for i := start; i < end && i < len(d.toks); i++ {
		p := d.toks[i].pg[lvl]
		if p != "" && (len(out) == 0 || out[len(out)-1] != p) {
			out = append(out, p)
		}
	}
	return out
}

func pageSpan(p []string) string {
	if len(p) == 0 {
		return ""
	}
	if p[0] == p[len(p)-1] {
		return p[0]
	}
	return p[0] + "-" + p[len(p)-1]
}

func (d *ccDoc) locate(h ccHit) ccLocation {
	seg := d.Segs[d.toks[h.start].seg]
	loc := ccLocation{Section: seg.Kind, Speaker: seg.label(), Author: seg.Author, Footnote: seg.Fn, Pages: pageSpan(d.pagesOf(h.start, h.end, 0))}
	if seg.Kind == "footnote" {
		loc.Section = seg.Parent
		loc.Speaker = "FOOTNOTE " + seg.Fn + " to the " + seg.Parent
	}
	for lvl := 1; lvl < 4 && lvl < len(d.Reporters); lvl++ {
		if p := pageSpan(d.pagesOf(h.start, h.end, lvl)); p != "" && d.Reporters[lvl] != "" {
			if loc.Parallel == nil {
				loc.Parallel = map[string]string{}
			}
			loc.Parallel[d.Reporters[lvl]] = p
		}
	}
	return loc
}

// sourceText returns the matched passage widened to whole sentences, with page markers kept.
func (d *ccDoc) sourceText(h ccHit) string {
	si := d.toks[h.start].seg
	seg := d.Segs[si]
	last := h.end - 1
	for last > h.start && (d.toks[last].seg != si || d.toks[last].n == ccSentinel) {
		last-- // never take an end offset from another section's text
	}
	s, e := d.toks[h.start].s, d.toks[last].e
	t := seg.Text
	for n := 0; s > 0 && n < 400; n++ {
		if (t[s-1] == '.' || t[s-1] == '\n' || t[s-1] == '?') && n > 0 {
			break
		}
		s--
	}
	for n := 0; e < len(t) && n < 400; n++ {
		if t[e] == '\n' {
			break
		}
		if (t[e] == '.' || t[e] == '?') && (e+1 >= len(t) || t[e+1] == ' ' || t[e+1] == '\n' || t[e+1] == '"' || t[e+1] == '\'') && n > 0 {
			e++
			for e < len(t) && (t[e] == '"' || t[e] == '\'' || t[e] == ')') {
				e++
			}
			break
		}
		e++
	}
	return strings.TrimSpace(d.fnLabels(t[s:e]))
}

// rank orders hits: a judge's writing before editorial material, majority before separate opinions.
func (d *ccDoc) rank(h ccHit) int {
	seg := d.Segs[d.toks[h.start].seg]
	k := seg.Kind
	if k == "footnote" {
		k = seg.Parent
	}
	switch k {
	case "opinion", "statute":
		return 0
	case "concurrence":
		return 1
	case "dissent":
		return 2
	case "official-summary", "history":
		return 3
	}
	return 4
}

func (d *ccDoc) checkQuote(id, quote, pin string) ccQuoteResult {
	res := ccQuoteResult{ID: id, Quote: quote}
	frags, raws, notes := parseQuote(quote)
	res.Signals = notes
	if len(frags) == 0 {
		res.Status = "not_found"
		res.Flags = append(res.Flags, "the quotation has no words to look for")
		return res
	}
	signaled := false
	for _, f := range frags {
		for _, it := range f {
			if it.soft || it.wild {
				signaled = true
			}
		}
	}
	if len(frags) > 1 {
		signaled = true
	}
	prevEnd := -1
	found, verbatim := 0, 0
	var primary []ccHit
	for fi, items := range frags {
		fr := ccFragResult{Text: raws[fi]}
		hits := d.exactHits(items)
		if len(hits) == 0 {
			hits = d.nearHits(items)
		}
		if len(hits) == 0 {
			fr.Status = "not_found"
			res.Fragments = append(res.Fragments, fr)
			continue
		}
		// Prefer a judge's writing, then the first hit after the previous fragment.
		sort.SliceStable(hits, func(a, b int) bool {
			if hits[a].exact != hits[b].exact {
				return hits[a].exact
			}
			ra, rb := d.rank(hits[a]), d.rank(hits[b])
			if ra != rb {
				return ra < rb
			}
			aa, ab := hits[a].start >= prevEnd, hits[b].start >= prevEnd
			if aa != ab {
				return aa
			}
			return hits[a].start < hits[b].start
		})
		h := hits[0]
		loc := d.locate(h)
		fr.Location = &loc
		fr.Source = d.sourceText(h)
		fr.Similarity = float64(int(h.sim*1000)) / 1000
		if h.exact {
			fr.Status = "verbatim"
			verbatim++
		} else {
			fr.Status = "differs"
			fr.Deviations = h.devs
		}
		for _, o := range hits[1:] {
			if o.exact == h.exact && len(fr.Others) < 4 {
				fr.Others = append(fr.Others, d.locate(o))
			}
		}
		if prevEnd >= 0 {
			gap := h.start - prevEnd
			switch {
			case gap < 0:
				res.Flags = append(res.Flags, "the parts joined by an ellipsis appear in a different order in the source")
			case d.toks[h.start].seg != d.toks[prevEnd-1].seg:
				res.Flags = append(res.Flags, "the ellipsis joins text from different parts of the document ("+d.Segs[d.toks[prevEnd-1].seg].label()+" and "+d.Segs[d.toks[h.start].seg].label()+")")
			case gap > 120:
				res.Flags = append(res.Flags, "the ellipsis skips about "+strconv.Itoa(gap)+" words of the source")
			}
		}
		prevEnd = h.end
		found++
		primary = append(primary, h)
		res.Fragments = append(res.Fragments, fr)
	}
	switch {
	case found == 0:
		res.Status = "not_found"
	case found < len(frags):
		res.Status = "partly_found"
	case verbatim == len(frags) && signaled:
		res.Status = "verbatim_with_signaled_changes"
	case verbatim == len(frags):
		res.Status = "verbatim"
	default:
		res.Status = "differs"
	}
	// speaker flags
	seen := map[string]bool{}
	for _, h := range primary {
		seg := d.Segs[d.toks[h.start].seg]
		k := seg.Kind
		if k == "footnote" {
			k = seg.Parent
		}
		var f string
		switch k {
		case "opinion", "statute":
		case "concurrence":
			f = "these words are from a CONCURRENCE" + byAuthor(seg.Author) + ", not the majority opinion"
		case "dissent":
			f = "these words are from a DISSENT" + byAuthor(seg.Author) + ", not the court's holding"
		case "annotations":
			f = "these words are from Deering's ANNOTATIONS, not the statute's text"
		case "history":
			f = "these words are from the statutory history notes, not the statute's operative text"
		default:
			f = "these words are from " + seg.label() + ", not from the opinion itself"
		}
		if f != "" && !seen[f] {
			seen[f] = true
			res.Flags = append(res.Flags, f)
		}
	}
	if pin != "" && len(primary) > 0 {
		res.Pin = d.checkPin(pin, primary)
	} else if pin != "" {
		res.Pin = &ccPinResult{Given: pin, Note: "quotation not found, so the pin was not checked"}
	}
	return res
}

func parsePin(pin string) (lo, hi int, fn string, ok bool) {
	if m := qPinFnRe.FindStringSubmatch(pin); m != nil {
		fn = m[1]
		pin = qPinFnRe.ReplaceAllString(pin, "")
	}
	m := qPinRe.FindStringSubmatch(pin)
	if m == nil {
		return 0, 0, fn, false
	}
	lo, _ = strconv.Atoi(m[1])
	hi = lo
	if m[2] != "" {
		h := m[2]
		if len(h) < len(m[1]) { // 172-73
			h = m[1][:len(m[1])-len(h)] + h
		}
		hi, _ = strconv.Atoi(h)
		if hi < lo {
			hi = lo
		}
	}
	return lo, hi, fn, true
}

func (d *ccDoc) checkPin(pin string, hits []ccHit) *ccPinResult {
	r := &ccPinResult{Given: pin}
	if d.Kind == "statute" {
		r.Note = "statutes have no reporter pages: confirm the subdivision by reading source_text, which starts at the subdivision the words are in"
		return r
	}
	lo, hi, fn, ok := parsePin(pin)
	if !ok {
		r.Note = "could not read a page number from the pin"
		return r
	}
	// within reports whether every page the matched passages sit on is inside the pin's range
	// (all), whether at least one is (any), and the pages themselves. A pin must cover the whole
	// quotation: a passage joined by an ellipsis from text on another page does not pass on a
	// pin to only one of those pages.
	within := func(lvl int) (all, any bool, pages []string) {
		all = true
		for _, h := range hits {
			for _, p := range d.pagesOf(h.start, h.end, lvl) {
				if len(pages) == 0 || pages[len(pages)-1] != p {
					pages = append(pages, p)
				}
				if n, err := strconv.Atoi(p); err == nil && n >= lo && n <= hi {
					any = true
				} else {
					all = false
				}
			}
		}
		return all && len(pages) > 0, any, pages
	}
	okOff, partOff, pages := within(0)
	r.FoundOn = pageSpan(pages)
	if len(pages) == 0 {
		r.Note = "this document has no reporter page markers, so the pin cannot be checked here"
		return r
	}
	match := okOff
	r.Matches = &match
	if !okOff {
		for lvl := 1; lvl < 4 && lvl < len(d.Reporters); lvl++ {
			if all, _, _ := within(lvl); all && d.Reporters[lvl] != "" {
				r.Note = "the pin matches the pagination of " + d.Reporters[lvl] + ", not the cited reporter"
			}
		}
		if r.Note == "" && partOff {
			r.Note = "the pin covers only part of the quotation: the quoted words run across page(s) " + strings.Join(pages, ", ") + " of " + d.Reporters[0] + ", not just page " + strings.TrimSpace(pin)
		}
		if r.Note == "" {
			r.Note = "the passage is on page " + r.FoundOn + " of " + d.Reporters[0] + ", not page " + strings.TrimSpace(pin)
		}
	}
	seg := d.Segs[d.toks[hits[0].start].seg]
	switch {
	case fn != "" && seg.Fn != fn:
		match = false
		where := "the body text"
		if seg.Fn != "" {
			where = "footnote " + seg.Fn
		}
		r.Note = strings.TrimSpace(r.Note + " The pin points to footnote " + fn + ", but the passage is in " + where + ".")
	case fn == "" && seg.Fn != "":
		r.Note = strings.TrimSpace(r.Note + " The passage is in footnote " + seg.Fn + "; the pin does not say so.")
	}
	return r
}
