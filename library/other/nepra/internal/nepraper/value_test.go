package nepraper

import "testing"

func TestValueZeroIsAbsentNotZero(t *testing.T) {
	var v Value
	if v.Kind != KindAbsent {
		t.Errorf("Value{}.Kind = %s, want %s: the zero value must mean "+
			"\"nothing known\", never \"zero\"", v.Kind, KindAbsent)
	}
	if _, ok := v.Float(); ok {
		t.Error("Value{}.Float() returned a number; an unset value has no number")
	}
	if got := v.String(); got != "<absent>" {
		t.Errorf("Value{}.String() = %q, want %q", got, "<absent>")
	}
}

func TestParseFigure(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantKind   ValueKind
		wantNum    float64
		wantRaw    string
		wantLabel  string
		wantReason bool
		why        string
	}{
		{
			name: "plain percentage", in: "36.6",
			wantKind: KindNumeric, wantNum: 36.6,
			why: "FY2018-19 TABLE 1 PESCO actual reported loss",
		},
		{
			name: "parenthesised negative", in: "(0.16)",
			wantKind: KindNumeric, wantNum: -0.16,
			why: "NEPRA prints a met target as a bracketed negative breach (GEPCO, FY2018-19)",
		},
		{
			name: "thousands separator", in: "21,715.3",
			wantKind: KindNumeric, wantNum: 21715.3,
			why: "FY2018-19 TABLE 6 MEPCO breach of SAIDI target",
		},
		{
			name: "a real zero", in: "0.00",
			wantKind: KindNumeric, wantNum: 0,
			why: "PESCO met its SAIDI target; the published breach really is zero",
		},
		{name: "bare zero", in: "0", wantKind: KindNumeric, wantNum: 0},
		{name: "percent sign", in: "9.5%", wantKind: KindNumeric, wantNum: 9.5},
		{name: "leading minus", in: "-0.44", wantKind: KindNumeric, wantNum: -0.44},
		{
			name: "truncated excel label", in: "19,535.",
			wantKind: KindUnverified, wantRaw: "19,535.", wantReason: true,
			why: "the trailing digits are absent from the text layer; 19535 and 19.535 " +
				"are both fabrications",
		},
		{
			name: "another truncated label", in: "28,189.",
			wantKind: KindUnverified, wantRaw: "28,189.", wantReason: true,
		},
		{
			name: "two decimal points", in: "17.9.2",
			wantKind: KindUnverified, wantRaw: "17.9.2", wantReason: true,
			why: "cell reconstruction is ambiguous; refuse rather than pick a reading",
		},
		{
			// Raw keeps what the glyphs actually spelled; Label carries the
			// canonical reading. Overwriting Raw with the label used to
			// destroy the evidence that the PDF said "FarAway".
			name: "far away breach label", in: "FarAway",
			wantKind: KindQualitative, wantRaw: "FarAway", wantLabel: "Far Away",
			why: "FY2020-21 onwards NEPRA publishes a label, not a magnitude; it is not zero",
		},
		{
			name: "near to limit breach label", in: "NeartoLimit",
			wantKind: KindQualitative, wantRaw: "NeartoLimit", wantLabel: "Near to Limit",
		},
		{
			name: "spaced breach label", in: "Far Away",
			wantKind: KindQualitative, wantRaw: "Far Away", wantLabel: "Far Away",
		},
		{
			name: "dash means not published", in: "-",
			wantKind: KindAbsent,
			why:      "FY2024-25 prints - for K-Electric's loss and gives it in a footnote",
		},
		{name: "empty", in: "", wantKind: KindAbsent},
		{name: "whitespace", in: "   ", wantKind: KindAbsent},
		{name: "n/a", in: "N/A", wantKind: KindAbsent},
		{
			name: "free text", in: "Not Reported",
			wantKind: KindQualitative, wantRaw: "Not Reported",
			why: "an unrecognised string is kept verbatim, never coerced to a number",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseFigure(tc.in)
			if got.Kind != tc.wantKind {
				t.Fatalf("ParseFigure(%q).Kind = %s, want %s (%s)",
					tc.in, got.Kind, tc.wantKind, tc.why)
			}
			if tc.wantKind == KindNumeric {
				n, ok := got.Float()
				if !ok {
					t.Fatalf("ParseFigure(%q).Float() not ok", tc.in)
				}
				if n != tc.wantNum {
					t.Errorf("ParseFigure(%q) = %v, want %v", tc.in, n, tc.wantNum)
				}
				return
			}
			if _, ok := got.Float(); ok {
				t.Errorf("ParseFigure(%q).Float() returned a number for a %s value",
					tc.in, got.Kind)
			}
			if tc.wantRaw != "" && got.Raw != tc.wantRaw {
				t.Errorf("ParseFigure(%q).Raw = %q, want %q", tc.in, got.Raw, tc.wantRaw)
			}
			if tc.wantLabel != "" && got.Label != tc.wantLabel {
				t.Errorf("ParseFigure(%q).Label = %q, want %q", tc.in, got.Label, tc.wantLabel)
			}
			if tc.wantLabel == "" && got.Label != "" {
				t.Errorf("ParseFigure(%q).Label = %q, want no canonical label", tc.in, got.Label)
			}
			if tc.wantReason && got.Reason == "" {
				t.Errorf("ParseFigure(%q) gave no reason for being %s", tc.in, got.Kind)
			}
		})
	}
}

func TestValueConstructors(t *testing.T) {
	if v := Unavailable("404 on NEPRA's index"); v.Kind != KindUnavailable || v.Reason == "" {
		t.Errorf("Unavailable() = %+v, want KindUnavailable with a reason", v)
	}
	if v := Excluded("excluded on the record"); v.Kind != KindExcluded {
		t.Errorf("Excluded().Kind = %s, want %s", v.Kind, KindExcluded)
	}
	if v := Numeric(2.5, "2.5"); !v.IsNumeric() || v.Raw != "2.5" {
		t.Errorf("Numeric() = %+v, want numeric with raw kept", v)
	}
	// No non-numeric kind may leak a number.
	for _, v := range []Value{
		Unavailable("x"), Excluded("x"), Unverified("19,535.", "x"), Qualitative("Far Away"), Absent(),
	} {
		if _, ok := v.Float(); ok {
			t.Errorf("%s value leaked a number", v.Kind)
		}
	}
}
