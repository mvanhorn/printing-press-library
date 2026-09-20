package nepraper

import (
	"strings"
	"testing"
)

// TestMetricFromCaption feeds the matcher the actual caption strings extracted
// from the seven reports, spelling mangling included. NEPRA's text layer splits
// and re-orders words, so these are the real inputs, not tidied ones.
func TestMetricFromCaption(t *testing.T) {
	cases := []struct {
		caption string
		want    Metric
		fy      string
	}{
		{"Table 1: Transmission and Distribution (T&D) Losses", MetricTDLosses, "FY2020-21"},
		{"Table 01: Transmission and Distribution Losses", MetricTDLosses, "FY2021-22"},
		{"Table01:TrissionandansmDistributionLosses", MetricTDLosses, "FY2024-25"},
		{"Table 15 :TransmissionandDistribution(T&D)Losses", MetricTDLosses, "FY2024-25"},

		{"Table 2: Financial Loss due to breach of T & D Loss", MetricTDLossFinancialImpact, "FY2020-21"},
		{"Table02:FinancialLossduetobreachofT&Dlosstarget", MetricTDLossFinancialImpact, "FY2024-25"},
		{"Table 13: Financial Loss Due to breach of Recovery Targets", MetricRecoveryFinancialImpct, "FY2021-22"},
		{"Table 0 4 : FinancialLossDuetoBreachofRecoveryTargets", MetricRecoveryFinancialImpct, "FY2024-25"},

		{"Table 3: Recovery", MetricRecovery, "FY2020-21"},
		{"Table03:Recovery(%)", MetricRecovery, "FY2024-25"},
		{"Table 1 6:Billing&Collection(%)", MetricRecovery, "FY2024-25"},

		{"Table 5: System Average Interruption frequency Index", MetricSAIFI, "FY2020-21"},
		{"Table 14 : System Average Interruption Frequency Index(SAIFI)", MetricSAIFI, "FY2021-22"},
		{"Table 05: System Average Interruption Frequency Index (SAIFI) without LT interruptions", MetricSAIFI, "FY2022-23"},
		{"Table 06: System Average Interruption Frequency Index (SAIFI) with LT interruptions", MetricSAIFI, "FY2022-23"},
		{"Table 05 :SystemAverageInterruptionFrequencyIndex(SAIFI)", MetricSAIFI, "FY2024-25"},
		{"Table 17 : SystemAverageInterruptionFrequencyIndex(SAIFI)", MetricSAIFI, "FY2024-25"},

		{"Table 6: System Average Interruption duration index", MetricSAIDI, "FY2020-21"},
		{"Table 15: System Average Interruption Duration Index (SAIDI)", MetricSAIDI, "FY2021-22"},
		{"Table 0 6:SystemAverageInterruptionDur ationIndex(SAIDI )", MetricSAIDI, "FY2024-25"},
		{"Table 18 :SystemAverageDurationFrequencyIndex(SAIDI)", MetricSAIDI, "FY2024-25"},

		{"Table 16: % Eligible consumer who were not provided new connection within prescribed", MetricNewConnections, "FY2021-22"},
		{"Table 19 :TimeFrameforNewConnection", MetricNewConnections, "FY2024-25"},
		{"Table 26: % of Pending Ripe Connections", MetricNewConnections, "FY2021-22"},
		{"Table 10: DISCO/Category wise progressive total no. of pending connections as on June,", MetricPendingConnections, "FY2022-23"},
		{"Table 11: DISCO wise aging of no. of pending ripe connections as on June, 2023", MetricPendingConnections, "FY2022-23"},

		{"Table 17: Average Load Shedding (Hours) daily", MetricLoadShedding, "FY2021-22"},
		{"Table2 0 :LoadShedding(Hours)", MetricLoadShedding, "FY2024-25"},

		{"Table 9: Nominal Voltages", MetricNominalVoltage, "FY2020-21"},
		{"Table 18: No. of Consumers Complaints made about Nominal Voltages", MetricNominalVoltage, "FY2021-22"},
		{"Table 28: No. of Consumers complaints who made about Voltages", MetricNominalVoltage, "FY2021-22"},

		{"Table 19: Consumer Complaints", MetricConsumerComplaints, "FY2021-22"},
		{"Table2 2 :ConsumerServiceComplaints", MetricConsumerComplaints, "FY2024-25"},
		{"Table 12 er:ConsumComplaints", MetricConsumerComplaints, "FY2024-25"},
		{"Table 2: Analysis of Data Regarding Complaints", MetricConsumerComplaints, "FY2014-15"},

		{"Table 11: SAFETY", MetricSafety, "FY2020-21"},
		{"Table 13 :SafetyAccidents", MetricSafety, "FY2024-25"},
		{"Table 2 3 : FatalAccidents", MetricSafety, "FY2024-25"},

		{"Table 12: Fault Rate (No. of faults/km)", MetricFaultRate, "FY2020-21"},
		{"Table 14 :FaultRate(No.offaults/km)", MetricFaultRate, "FY2024-25"},
		{"Table 31: No. of Faults/KM", MetricFaultRate, "FY2021-22"},
	}
	for _, tc := range cases {
		t.Run(tc.fy+" "+tc.caption, func(t *testing.T) {
			got, ok := MetricFromCaption(tc.caption)
			if !ok {
				t.Fatalf("MetricFromCaption(%q) did not match anything, want %s", tc.caption, tc.want)
			}
			if got != tc.want {
				t.Errorf("MetricFromCaption(%q) = %s, want %s", tc.caption, got, tc.want)
			}
		})
	}
}

// TestTableNumberingDrifts is the reason the parser keys on captions. The same
// metric carries a different table number in every year, and in FY2022-23 it
// carries two in the same report.
func TestTableNumberingDrifts(t *testing.T) {
	saifiCaptions := map[string]string{
		"FY2018-19":            "TABLE 5",
		"FY2019-20":            "TABLE 5",
		"FY2020-21":            "Table 5: System Average Interruption frequency Index",
		"FY2021-22":            "Table 14 : System Average Interruption Frequency Index(SAIFI)",
		"FY2022-23 without LT": "Table 05: System Average Interruption Frequency Index (SAIFI) without LT interruptions",
		"FY2022-23 with LT":    "Table 06: System Average Interruption Frequency Index (SAIFI) with LT interruptions",
		"FY2024-25 headline":   "Table 05 :SystemAverageInterruptionFrequencyIndex(SAIFI)",
		"FY2024-25 comparison": "Table 17 : SystemAverageInterruptionFrequencyIndex(SAIFI)",
	}
	numbers := map[string]bool{}
	for where, caption := range saifiCaptions {
		label, _, ok := parseCaption(caption)
		if !ok {
			t.Fatalf("%s: %q is not recognised as a caption", where, caption)
		}
		numbers[label] = true
		// A bare "TABLE 5" carries no metric; the section heading supplies it.
		// Anything with a caption body must resolve to SAIFI.
		if len(caption) > 10 {
			if m, matched := MetricFromCaption(caption); !matched || m != MetricSAIFI {
				t.Errorf("%s: MetricFromCaption(%q) = %s/%v, want saifi", where, caption, m, matched)
			}
		}
	}
	if len(numbers) < 4 {
		t.Errorf("SAIFI appears under %d distinct table numbers (%v); the corpus has at "+
			"least four, which is why table numbers must never be used as a key",
			len(numbers), numbers)
	}
}

func TestParseCaptionRejectsCrossReferences(t *testing.T) {
	accept := []struct{ line, label string }{
		{"TABLE 5", "Table 5"},
		{"TABLE 20", "Table 20"},
		{"Table 1: Transmission and Distribution (T&D) Losses", "Table 1"},
		{"Table 05 :SystemAverageInterruptionFrequencyIndex(SAIFI)", "Table 5"},
		{"Table 0 4 : FinancialLossDuetoBreachofRecoveryTargets", "Table 4"},
		{"Table2 1 :NominalVoltages", "Table 21"},
	}
	for _, tc := range accept {
		t.Run("accept "+tc.line, func(t *testing.T) {
			label, body, ok := parseCaption(tc.line)
			if !ok {
				t.Fatalf("parseCaption(%q) rejected a real caption", tc.line)
			}
			if label != tc.label {
				t.Errorf("label = %q, want %q", label, tc.label)
			}
			if body == "" {
				t.Error("caption body is empty; provenance needs the caption text")
			}
		})
	}
	reject := []string{
		"Table of Contents",
		"TABLE OF CONTENTS",
		"Table 1 indicates the",
		"Table 2 contains analysis of the complaints data based on two parameters; percentage of",
		"Table 20 and Figure 20 indicate the data related to average daily load shedding hours for the",
		"Table 16 and its graphical representation illustrate that only GEPCO, MEPCO, SEPCO and",
		"Table 04 illustrates the revenue losses incurred by distribution companies due to poor",
		"From the data shown in Table 6, it is noted that except PESCO and IESCO, none of the DISCO has",
		"Above table and figures show the recovery positions and each DISCO along with breach of",
		"F IGURE 5",
	}
	for _, line := range reject {
		t.Run("reject "+line, func(t *testing.T) {
			if label, _, ok := parseCaption(line); ok {
				t.Errorf("parseCaption(%q) accepted a narrative cross-reference as caption %q",
					line, label)
			}
		})
	}
}

// TestParseCaptionInLine covers FY2018-19, which sets its tables beside a
// running commentary so that a caption shares a baseline with prose.
func TestParseCaptionInLine(t *testing.T) {
	cases := []struct {
		name  string
		cells []string
		want  string
		ok    bool
	}{
		{
			name:  "caption at the end of a prose line",
			cells: []string{"picture", "is", "also", "given", "which", "TABLE", "1"},
			want:  "Table 1", ok: true,
		},
		{
			name:  "caption alone",
			cells: []string{"TABLE", "6"},
			want:  "Table 6", ok: true,
		},
		{
			name:  "bracketed cross-reference is not a caption",
			cells: []string{"individual", "targets", "set", "by", "NEPRA", "(Table", "1).", "SEPCO"},
			ok:    false,
		},
		{
			name:  "mid-sentence reference is not a caption",
			cells: []string{"From", "the", "data", "shown", "in", "Table", "6,", "it", "is", "noted"},
			ok:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			label, _, ok := parseCaptionInLine(tc.cells)
			if ok != tc.ok {
				t.Fatalf("parseCaptionInLine(%v) ok = %v, want %v (label %q)",
					tc.cells, ok, tc.ok, label)
			}
			if ok && label != tc.want {
				t.Errorf("label = %q, want %q", label, tc.want)
			}
		})
	}
}

func TestFiscalYearFromHeader(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"2020-21", "FY2020-21", true},
		{"2020-2021", "FY2020-21", true},
		{"2016-17", "FY2016-17", true},
		{"FY 2018-19", "FY2018-19", true},
		{"2018 - 19", "FY2018-19", true},
		{"2024-25", "FY2024-25", true},
		{"Reported Figures", "", false},
		{"Target by NEPRA", "", false},
		{"(No.)", "", false},
		{"13", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := FiscalYearFromHeader(tc.in)
			if ok != tc.ok {
				t.Fatalf("FiscalYearFromHeader(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("FiscalYearFromHeader(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestVariantFromCaption(t *testing.T) {
	cases := []struct {
		caption string
		want    string
		ok      bool
	}{
		{"Table 05: System Average Interruption Frequency Index (SAIFI) without LT interruptions", VariantWithoutLT, true},
		{"Table 06: System Average Interruption Frequency Index (SAIFI) with LT interruptions", VariantWithLT, true},
		{"Table 07: System Average Interruption Duration Index (SAIDI) without LT interruptions", VariantWithoutLT, true},
		{"Table 05 :SystemAverageInterruptionFrequencyIndex(SAIFI)", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.caption, func(t *testing.T) {
			got, ok := VariantFromCaption(tc.caption)
			if ok != tc.ok || got != tc.want {
				t.Errorf("VariantFromCaption(%q) = %q/%v, want %q/%v",
					tc.caption, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestMetricTitlesMatchNEPRAWording pins the canonical wording to the FY2018-19
// table of contents, which is the closest thing this corpus has to a
// specification.
func TestMetricTitlesMatchNEPRAWording(t *testing.T) {
	want := map[Metric]string{
		MetricTDLosses:           "Transmission & Distribution Losses (%)",
		MetricRecovery:           "Recovery (%)",
		MetricSAIFI:              "System Average Interruption Frequency Index (SAIFI - No.)",
		MetricSAIDI:              "System Average Interruption Duration Index (SAIDI - Min)",
		MetricNewConnections:     "Time Frame for New Connections (%)",
		MetricLoadShedding:       "Load Shedding (Hrs)",
		MetricNominalVoltage:     "Nominal Voltage",
		MetricConsumerComplaints: "Consumer Service Complaints",
	}
	for m, title := range want {
		if got := MetricTitles[m]; got != title {
			t.Errorf("MetricTitles[%s] = %q, want %q (verbatim from the FY2018-19 contents page)",
				m, got, title)
		}
	}
	for _, m := range Metrics() {
		if strings.TrimSpace(MetricTitles[m]) == "" {
			t.Errorf("metric %s has no title", m)
		}
	}
}
