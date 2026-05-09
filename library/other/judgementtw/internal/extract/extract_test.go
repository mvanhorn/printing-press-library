// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package extract

import (
	"testing"
	"time"
)

func TestParseJID(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantErr  bool
		wantPart JID
	}{
		{
			name: "supreme criminal narcotics",
			raw:  "TPSM,115,台抗,703,20260430,1",
			wantPart: JID{
				Court: "TPS", CaseType: "M", Year: 115, CaseChar: "台抗",
				No: 703, JDate: "20260430", Check: 1,
			},
		},
		{
			name: "high court drug appeal",
			raw:  "TPHM,110,毒抗,1212,20210831,1",
			wantPart: JID{
				Court: "TPH", CaseType: "M", Year: 110, CaseChar: "毒抗",
				No: 1212, JDate: "20210831", Check: 1,
			},
		},
		{
			name:    "missing parts",
			raw:     "TPSM,115,台抗,703",
			wantErr: true,
		},
		{
			name:    "empty",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "non-int year",
			raw:     "TPSM,abc,台抗,703,20260430,1",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Court != tc.wantPart.Court || got.CaseType != tc.wantPart.CaseType ||
				got.Year != tc.wantPart.Year || got.CaseChar != tc.wantPart.CaseChar ||
				got.No != tc.wantPart.No || got.JDate != tc.wantPart.JDate ||
				got.Check != tc.wantPart.Check {
				t.Errorf("Parse(%q) = %+v, want match for %+v", tc.raw, got, tc.wantPart)
			}
		})
	}
}

func TestCaseTypeName(t *testing.T) {
	cases := map[string]string{
		"M": "刑事", "V": "民事", "A": "行政",
		"P": "懲戒", "C": "憲法", "X": "X",
	}
	for in, want := range cases {
		if got := CaseTypeName(in); got != want {
			t.Errorf("CaseTypeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCaseTypeFromEnglish(t *testing.T) {
	cases := map[string]string{
		"criminal": "M", "civil": "V", "administrative": "A",
		"disciplinary": "P", "constitutional": "C",
		"M": "M", "v": "V", "刑事": "M",
		"unknown": "unknown",
	}
	for in, want := range cases {
		if got := CaseTypeFromEnglish(in); got != want {
			t.Errorf("CaseTypeFromEnglish(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCourtHierarchyRank(t *testing.T) {
	cases := map[string]int{
		"TPS": 3, "JCC": 3,
		"TPH": 2, "TCH": 2, "IPC": 2,
		"TPT": 1,
		"TPD": 0, "PCD": 0, "KSY": 0,
	}
	for code, want := range cases {
		if got := CourtHierarchyRank(code); got != want {
			t.Errorf("CourtHierarchyRank(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestParseDate(t *testing.T) {
	want := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	cases := []string{
		"115/4/30",
		"115-04-30",
		"115年4月30日",
		"民國115年4月30日",
		"2026-04-30",
		"2026/04/30",
		"20260430",
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			got, err := ParseDate(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(want) {
				t.Errorf("ParseDate(%q) = %v, want %v", s, got, want)
			}
		})
	}
	if _, err := ParseDate("not-a-date"); err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestROCRoundTrip(t *testing.T) {
	g := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	r := ROCFromGregorian(g)
	if r.Year != 115 || r.Month != 4 || r.Day != 30 {
		t.Errorf("got %+v, want {115 4 30}", r)
	}
	if back := r.Gregorian(); !back.Equal(g) {
		t.Errorf("round-trip lost data: got %v, want %v", back, g)
	}
}

func TestIsAPIServiceWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Taipei")
	cases := []struct {
		hour int
		want bool
	}{
		{0, true}, {1, true}, {3, true}, {5, true}, {6, false},
		{12, false}, {18, false}, {23, false},
	}
	for _, tc := range cases {
		ts := time.Date(2026, 5, 9, tc.hour, 30, 0, 0, loc)
		if got := IsAPIServiceWindow(ts); got != tc.want {
			t.Errorf("IsAPIServiceWindow(hour=%d) = %v, want %v", tc.hour, got, tc.want)
		}
	}
}

func TestExtractCitations(t *testing.T) {
	body := `按裁判確定前犯數罪者，併合處罰之；數罪併罰有二裁判以上宣告多數有期徒刑者，
於各刑中之最長期以上，各刑合併之刑期以下，定其應執行之刑期，但不得逾30年，刑法第50條第1項前段、
第51條第5款及第53條分別規定甚明。被告違反毒品危害防制條例第17條第2項規定。
行政程序法第92條`
	cs := ExtractCitations(body)
	have := map[string]int{}
	for _, c := range cs {
		have[c.Statute] = c.Article
	}
	if have["刑法"] == 0 {
		t.Errorf("expected 刑法 with article, got %+v", cs)
	}
	if have["毒品危害防制條例"] != 17 {
		t.Errorf("expected 毒品危害防制條例 §17, got %+v", cs)
	}
	if have["行政程序法"] != 92 {
		t.Errorf("expected 行政程序法 §92, got %+v", cs)
	}
}

func TestExtractJIDReferences(t *testing.T) {
	body := `不服臺灣高等法院臺中分院中華民國115年3月10日定其應執行刑之裁定（115年度聲字第264號），
參考 TPHM,110,毒抗,1212,20210831,1 與 TPSM,115,台抗,703,20260430,1 之見解。`
	jids := ExtractJIDReferences(body)
	want := map[string]bool{
		"TPHM,110,毒抗,1212,20210831,1": true,
		"TPSM,115,台抗,703,20260430,1":  true,
	}
	if len(jids) != len(want) {
		t.Errorf("got %d JIDs, want %d (%+v)", len(jids), len(want), jids)
	}
	for _, j := range jids {
		if !want[j] {
			t.Errorf("unexpected JID %q", j)
		}
	}
}

func TestExtractSentences(t *testing.T) {
	cases := []struct {
		name    string
		verdict string
		check   func(t *testing.T, ss []Sentence)
	}{
		{
			name:    "year+month prison",
			verdict: "處有期徒刑11年2月。",
			check: func(t *testing.T, ss []Sentence) {
				if len(ss) != 1 || ss[0].Kind != SentencePrison || ss[0].PrisonMonths != 11*12+2 {
					t.Errorf("got %+v", ss)
				}
			},
		},
		{
			name:    "fine",
			verdict: "併科罰金新台幣30,000元。",
			check: func(t *testing.T, ss []Sentence) {
				if len(ss) != 1 || ss[0].Kind != SentenceFine || ss[0].FineNTD != 30000 {
					t.Errorf("got %+v", ss)
				}
			},
		},
		{
			name:    "life prison",
			verdict: "處無期徒刑。",
			check: func(t *testing.T, ss []Sentence) {
				if len(ss) < 1 || ss[0].Kind != SentenceLifePrison {
					t.Errorf("got %+v", ss)
				}
			},
		},
		{
			name:    "detention",
			verdict: "處拘役50日。",
			check: func(t *testing.T, ss []Sentence) {
				if len(ss) != 1 || ss[0].Kind != SentenceDetention {
					t.Errorf("got %+v", ss)
				}
			},
		},
		{
			name:    "compound",
			verdict: "處有期徒刑8個月，併科罰金新臺幣10,000元，緩刑2年，並予易服社會勞動。",
			check: func(t *testing.T, ss []Sentence) {
				kinds := map[SentenceKind]bool{}
				for _, s := range ss {
					kinds[s.Kind] = true
				}
				if !kinds[SentencePrison] || !kinds[SentenceFine] ||
					!kinds[SentenceProbation] || !kinds[SentenceLabor] {
					t.Errorf("missing some kind: %+v", ss)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, ExtractSentences(tc.verdict))
		})
	}
}

func TestPrimarySentence(t *testing.T) {
	ss := []Sentence{
		{Kind: SentenceFine, FineNTD: 1000},
		{Kind: SentencePrison, PrisonMonths: 12},
		{Kind: SentenceProbation, Probation: 2},
	}
	p := PrimarySentence(ss)
	if p.Kind != SentencePrison {
		t.Errorf("expected prison primary, got %v", p.Kind)
	}
	if PrimarySentence(nil).Kind != SentenceUnknown {
		t.Error("empty input should be unknown")
	}
}

func TestCleanHTML(t *testing.T) {
	in := `<div>hello <script>alert(1)</script>&nbsp;world &amp; friends</div>`
	got := CleanHTML(in)
	want := "hello world & friends"
	if got != want {
		t.Errorf("CleanHTML = %q, want %q", got, want)
	}
}

func TestExtractByID(t *testing.T) {
	in := `<html><body><div id="jud"><span>裁判字號：最高法院</span></div></body></html>`
	got := CleanHTML(ExtractByID(in, "jud"))
	if got != "裁判字號：最高法院" {
		t.Errorf("got %q", got)
	}
}

func TestSecondsUntilNextWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Taipei")
	// Inside window: should return 0
	inside := time.Date(2026, 5, 9, 3, 0, 0, 0, loc)
	if got := SecondsUntilNextWindow(inside); got != 0 {
		t.Errorf("inside window: got %d, want 0", got)
	}
	// Just after window closes (06:00)
	outside := time.Date(2026, 5, 9, 6, 0, 0, 0, loc)
	got := SecondsUntilNextWindow(outside)
	if got < 18*3600-10 || got > 18*3600+10 {
		t.Errorf("at 06:00 should be ~18h: got %d", got)
	}
}
