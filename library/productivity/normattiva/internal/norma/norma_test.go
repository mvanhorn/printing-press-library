// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package norma

import (
	"strings"
	"testing"
	"time"
)

func TestParseURNRoundTrip(t *testing.T) {
	cases := []string{
		"urn:nir:stato:legge:1990-08-07;241",
		"urn:nir:stato:legge:1990-08-07;241~art3",
		"urn:nir:stato:regio.decreto:1942-03-16;262:2~art2043",
		"urn:nir:stato:decreto.legislativo:2003-06-30;196~art13!vig=2010-01-01",
		"urn:nir:stato:decreto.legislativo:2003;196~art13@originale",
		"urn:nir:stato:costituzione:1947-12-27~art3",
		"urn:nir:stato:legge:1990-08-07;241~art21octies",
	}
	for _, c := range cases {
		u, err := ParseURN(c)
		if err != nil {
			t.Fatalf("ParseURN(%q): %v", c, err)
		}
		if got := u.String(); got != c {
			t.Errorf("round trip %q -> %q", c, got)
		}
	}
	if _, err := ParseURN("urn:nir:regione.lazio:legge:2000;1"); err == nil {
		t.Error("expected error for non-State URN")
	}
}

func TestURNBuilders(t *testing.T) {
	u, _ := ParseURN("urn:nir:stato:legge:1990-08-07;241")
	if got := u.WithArticle("21-octies").WithVigenza("2010-01-01").String(); got != "urn:nir:stato:legge:1990-08-07;241~art21octies!vig=2010-01-01" {
		t.Errorf("builder = %q", got)
	}
	if got := u.WithArticle("3").WithOriginale().String(); got != "urn:nir:stato:legge:1990-08-07;241~art3@originale" {
		t.Errorf("originale = %q", got)
	}
	// Annex is only rendered when an article is present.
	cc, _ := LookupFonte("codice civile")
	if got := cc.Base().String(); got != "urn:nir:stato:regio.decreto:1942-03-16;262" {
		t.Errorf("annex leaked into act URN: %q", got)
	}
}

func TestNormalizeArticle(t *testing.T) {
	cases := map[string]string{"3": "3", "Art. 3": "3", "21-octies": "21octies", "21 octies": "21octies", "3-BIS": "3bis", "articolo 2043": "2043"}
	for in, want := range cases {
		if got := NormalizeArticle(in); got != want {
			t.Errorf("NormalizeArticle(%q) = %q, want %q", in, got, want)
		}
	}
	if got := DisplayArticle("21octies"); got != "21-octies" {
		t.Errorf("DisplayArticle = %q", got)
	}
}

func TestLookupFonte(t *testing.T) {
	cases := map[string]string{"codice civile": "codice-civile", "c.c.": "codice-civile", "C.P.C.": "codice-procedura-civile", "gdpr": "codice-privacy", "Costituzione": "costituzione", "tuel": "tuel"}
	for in, want := range cases {
		f, ok := LookupFonte(in)
		if !ok || f.Key != want {
			t.Errorf("LookupFonte(%q) = %q,%v want %q", in, f.Key, ok, want)
		}
	}
	if _, ok := LookupFonte("legge inesistente sui gatti"); ok {
		t.Error("expected miss")
	}
	if got := SearchFonti("codice procedura"); len(got) != 2 {
		t.Errorf("SearchFonti(codice procedura) = %d results, want 2", len(got))
	}
}

func TestExtractCitations(t *testing.T) {
	text := "Visto l'art. 3, comma 1, della L. 7 agosto 1990, n. 241; richiamati l'art. 2043 c.c. e l'articolo 13 del d.lgs. 196/2003. " +
		"Considerato il D.P.R. n. 445 del 2000, art. 38 e l'art. 21-octies L. 241/1990; vista la Costituzione."
	got := ExtractCitations(text)
	want := []string{
		"urn:nir:stato:legge:1990-08-07;241~art3",
		"urn:nir:stato:regio.decreto:1942-03-16;262:2~art2043",
		"urn:nir:stato:decreto.legislativo:2003-06-30;196~art13",
		"urn:nir:stato:decreto.del.presidente.della.repubblica:2000;445~art38",
		"urn:nir:stato:legge:1990-08-07;241~art21octies",
		"urn:nir:stato:costituzione:1947-12-27",
	}
	if len(got) != len(want) {
		for _, c := range got {
			t.Logf("got %q -> %s", c.Raw, c.URN())
		}
		t.Fatalf("ExtractCitations found %d citations, want %d", len(got), len(want))
	}
	for i, c := range got {
		if s := c.URN().String(); s != want[i] {
			t.Errorf("citation %d (%q) = %s, want %s", i, c.Raw, s, want[i])
		}
	}
	if got[0].Comma != "1" {
		t.Errorf("comma = %q, want 1", got[0].Comma)
	}
}

func TestExtractCitationsNegative(t *testing.T) {
	for _, s := range []string{"Il delegato ha firmato il verbale.", "Nel 2020 ci sono stati 241 casi.", "dell'articolo precedente"} {
		if got := ExtractCitations(s); len(got) != 0 {
			t.Errorf("ExtractCitations(%q) = %+v, want none", s, got)
		}
	}
}

func TestParseCitation(t *testing.T) {
	cases := []struct{ in, art, want string }{
		{"codice civile", "2043", "urn:nir:stato:regio.decreto:1942-03-16;262:2~art2043"},
		{"d.lgs. 196/2003", "13", "urn:nir:stato:decreto.legislativo:2003-06-30;196~art13"},
		{"art. 3 L. 241/1990", "", "urn:nir:stato:legge:1990-08-07;241~art3"},
		{"R.D. 262/1942 art. 2043", "", "urn:nir:stato:regio.decreto:1942-03-16;262:2~art2043"},
		{"d.l. 18/2020", "1", "urn:nir:stato:decreto.legge:2020;18~art1"},
		{"2043 c.c.", "", "urn:nir:stato:regio.decreto:1942-03-16;262:2~art2043"},
		{"urn:nir:stato:legge:1990-08-07;241", "5", "urn:nir:stato:legge:1990-08-07;241~art5"},
		{"legge 7 agosto 1990, n. 241", "", "urn:nir:stato:legge:1990-08-07;241"},
	}
	for _, c := range cases {
		got, err := ParseCitation(c.in, c.art)
		if err != nil {
			t.Fatalf("ParseCitation(%q): %v", c.in, err)
		}
		if s := got.URN().String(); s != c.want {
			t.Errorf("ParseCitation(%q,%q) = %s, want %s", c.in, c.art, s, c.want)
		}
	}
	if _, err := ParseCitation("la mia legge preferita", ""); err == nil {
		t.Error("expected error for unrecognized reference")
	}
}

func TestRenderArticle(t *testing.T) {
	src := `<div class="bodyTesto"><h2 class="preamble-title-akn"></h2><h2 class="article-num-akn" id="art_3">Art. 3</h2>
<div class="article-heading-akn">(Motivazione del provvedimento)</div><div class="art-commi-div-akn">
<div class="art-comma-div-akn"><span class="comma-num-akn">1. </span><span class="art_text_in_comma">Ogni provvedimento deve essere motivato, salvo quanto previsto dalla <a href="http://www.normattiva.it/uri-res/N2Ls?urn:nir:stato:legge:1990-08-07;241~art2">l&#39;art. 2</a>.</span></div>
<div class="art-comma-div-akn"><span class="comma-num-akn">2. </span><span>La motivazione non &egrave; richiesta.</span></div></div></div>`
	txt := RenderArticle(src, RenderOptions{})
	for _, want := range []string{"Art. 3", "(Motivazione del provvedimento)", "1. Ogni provvedimento deve essere motivato", "l'art. 2", "2. La motivazione non è richiesta."} {
		if !strings.Contains(txt, want) {
			t.Errorf("plain render missing %q:\n%s", want, txt)
		}
	}
	if strings.Contains(txt, "<") {
		t.Errorf("tags leaked: %s", txt)
	}
	md := RenderArticle(src, RenderOptions{Markdown: true, Links: true})
	for _, want := range []string{"## Art. 3", "_(Motivazione del provvedimento)_", "**1.**", "[urn:nir:stato:legge:1990-08-07;241~art2]"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown render missing %q:\n%s", want, md)
		}
	}
	if h := ArticleHeading(src); h != "Art. 3" {
		t.Errorf("ArticleHeading = %q", h)
	}
	if refs := ReferencedURNs(src); len(refs) != 1 {
		t.Errorf("ReferencedURNs = %v", refs)
	}
	if !IsAbrogated(`<div class="ins-akn art_abrogato-akn">((ARTICOLO ABROGATO DAL D.LGS. 101/2018))</div>`) || IsAbrogated(src) {
		t.Error("IsAbrogated misclassified")
	}
}

func TestWordDiff(t *testing.T) {
	ops := WordDiff("Il termine è di trenta giorni.", "Il termine è di sessanta giorni.")
	if got := RenderDiff(ops, false); got != "Il termine è di [-trenta-]{+sessanta+} giorni." {
		t.Errorf("RenderDiff = %q", got)
	}
	ins, del := DiffStats(ops)
	if ins != 1 || del != 1 {
		t.Errorf("DiffStats = %d,%d", ins, del)
	}
	same := WordDiff("uguale testo", "uguale testo")
	if len(same) != 1 || same[0].Kind != "equal" {
		t.Errorf("identical texts produced %+v", same)
	}
	if ins, del := DiffStats(same); ins != 0 || del != 0 {
		t.Error("identical texts reported changes")
	}
}

func TestVigenzaDates(t *testing.T) {
	if got := FormatVigenzaDate("20050308"); got != "2005-03-08" {
		t.Errorf("FormatVigenzaDate = %q", got)
	}
	if _, open, ok := ParseVigenzaDate("99999999"); !open || ok {
		t.Error("99999999 should be open-ended")
	}
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	cases := map[string]string{"oggi": "2026-10-28", "2019-01-01": "2019-01-01", "01/02/2019": "2019-02-01"}
	for in, want := range cases {
		got, err := ParseDate(in, now)
		if err != nil || got != want {
			t.Errorf("ParseDate(%q) = %q,%v want %q", in, got, err, want)
		}
	}
	if _, err := ParseDate("ieri l'altro", now); err == nil {
		t.Error("expected error")
	}
}

func TestDiffHunksAndMarkers(t *testing.T) {
	ops := WordDiff("entro trenta giorni dalla relativa richiesta scritta.", "entro sessanta giorni ai sensi del regolamento scritta.")
	got := RenderDiff(ops, false)
	if want := "entro [-trenta-]{+sessanta+} giorni [-dalla relativa richiesta-]{+ai sensi del regolamento+} scritta."; got != want {
		t.Errorf("grouped diff = %q, want %q", got, want)
	}
	if s := StripAmendmentMarkers("(( (Motivazione del provvedimento) ))"); s != "(Motivazione del provvedimento)" {
		t.Errorf("StripAmendmentMarkers = %q", s)
	}
	md := RenderArticle("<div>Art. 2043. <br> (Risarcimento per fatto illecito). <br> Qualunque fatto.</div>", RenderOptions{Markdown: true})
	if !strings.HasPrefix(md, "## Art. 2043\n") {
		t.Errorf("annex heading not promoted: %q", md)
	}
}

func TestReviewRegressions(t *testing.T) {
	for in, want := range map[string]string{
		"21-bis c.p.":  "urn:nir:stato:regio.decreto:1930-10-19;1398:1~art21bis",
		"2043bis c.c.": "urn:nir:stato:regio.decreto:1942-03-16;262:2~art2043bis",
	} {
		c, err := ParseCitation(in, "")
		if err != nil || c.URN().String() != want {
			t.Errorf("ParseCitation(%q) = %s,%v want %s", in, c.URN(), err, want)
		}
	}
	got := ExtractCitations("Ai sensi degli artt. 2043 e 2059 c.c. e degli articoli 3, 21-octies ed 21-nonies L. 241/1990.")
	want := []string{"2043", "2059", "3", "21octies", "21nonies"}
	if len(got) != len(want) {
		t.Fatalf("plural citations: got %d %+v", len(got), got)
	}
	for i, c := range got {
		if c.Articolo != want[i] {
			t.Errorf("plural %d = %q want %q", i, c.Articolo, want[i])
		}
	}
	// A sentence break must not glue an article to a later act.
	cits := ExtractCitations("Si veda l'art. 5. Inoltre si applica il codice civile.")
	for _, c := range cits {
		if c.Fonte == "codice-civile" && c.Articolo != "" {
			t.Errorf("article glued across sentence: %+v", c)
		}
	}
	if NormalizeArticle("articoli 3") != "3" {
		t.Error("NormalizeArticle(articoli 3)")
	}
	big := strings.Repeat("parola ", 3000)
	ops := WordDiff(big+"fine A", "inizio "+big+"fine B")
	if ins, _ := DiffStats(ops); ins == 0 {
		t.Error("large diff lost the change")
	}
}
