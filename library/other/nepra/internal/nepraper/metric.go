package nepraper

import (
	"regexp"
	"strings"
)

// Metric is a NEPRA performance parameter.
//
// The metric is keyed on what the table caption says, never on the table
// number. Table numbering drifts across years -- SAIFI is Table 5, 5, 5, 14,
// 05 *and* 06, 5 and 17 in the seven reports this package was built against --
// so a parser keyed on numbers silently reads the wrong column.
type Metric string

const (
	MetricTDLosses               Metric = "td_losses"
	MetricTDLossFinancialImpact  Metric = "td_loss_financial_impact"
	MetricRecovery               Metric = "recovery"
	MetricRecoveryFinancialImpct Metric = "recovery_financial_impact"
	MetricSAIFI                  Metric = "saifi"
	MetricSAIDI                  Metric = "saidi"
	MetricNewConnections         Metric = "new_connections_time_frame"
	MetricPendingConnections     Metric = "pending_connections"
	MetricLoadShedding           Metric = "load_shedding"
	MetricNominalVoltage         Metric = "nominal_voltage"
	MetricConsumerComplaints     Metric = "consumer_complaints"
	MetricSafety                 Metric = "safety_fatalities"
	MetricFaultRate              Metric = "fault_rate"

	// MetricUnknown is used when a table has DISCO rows but its caption does
	// not match any known parameter. The table is still recorded, with its
	// caption, so that nothing is silently dropped.
	MetricUnknown Metric = "unknown"
)

// MetricTitles gives the canonical NEPRA wording for each metric, taken
// verbatim from the FY2018-19 table of contents where available.
var MetricTitles = map[Metric]string{
	MetricTDLosses:               "Transmission & Distribution Losses (%)",
	MetricTDLossFinancialImpact:  "Financial impact of breach of T&D loss target",
	MetricRecovery:               "Recovery (%)",
	MetricRecoveryFinancialImpct: "Financial impact of breach of recovery target",
	MetricSAIFI:                  "System Average Interruption Frequency Index (SAIFI - No.)",
	MetricSAIDI:                  "System Average Interruption Duration Index (SAIDI - Min)",
	MetricNewConnections:         "Time Frame for New Connections (%)",
	MetricPendingConnections:     "Pending ripe connections",
	MetricLoadShedding:           "Load Shedding (Hrs)",
	MetricNominalVoltage:         "Nominal Voltage",
	MetricConsumerComplaints:     "Consumer Service Complaints",
	MetricSafety:                 "Safety (No. of Fatalities)",
	MetricFaultRate:              "Fault Rate",
	MetricUnknown:                "unrecognised caption",
}

// ReliabilityMetrics are the two system-reliability indices, the metrics the
// FY2024-25 double-publication conflict lives in.
func ReliabilityMetrics() []Metric { return []Metric{MetricSAIFI, MetricSAIDI} }

// Metrics returns every metric this package recognises, in report order.
func Metrics() []Metric {
	return []Metric{
		MetricTDLosses, MetricTDLossFinancialImpact, MetricRecovery,
		MetricRecoveryFinancialImpct, MetricSAIFI, MetricSAIDI,
		MetricNewConnections, MetricPendingConnections, MetricLoadShedding,
		MetricNominalVoltage, MetricConsumerComplaints, MetricSafety,
		MetricFaultRate,
	}
}

// Variant labels distinguish two tables that report the same metric in the same
// report. NEPRA publishes at least three flavours of duplicate.
const (
	// VariantHeadline is the per-metric table in the analysis section, the one
	// that carries the NEPRA target and the breach.
	VariantHeadline = "headline"
	// VariantComparison is the five-year comparison table in the trend
	// section. Its column for the report's own fiscal year is the one that
	// disagrees with the headline table in FY2024-25.
	VariantComparison = "five_year_comparison"
	// VariantWithoutLT / VariantWithLT are FY2022-23's explicit split of SAIFI
	// and SAIDI by whether low-tension interruptions are counted.
	VariantWithoutLT = "without_lt_interruptions"
	VariantWithLT    = "with_lt_interruptions"
)

// normalizeCaption lowercases and strips everything but letters, digits and
// '%' so that "Table 0 6:System Average Interruption Dur ationIndex(SAIDI )"
// and "Table 6: System Average Interruption duration index" compare equal. The
// PER text layer splits words at arbitrary points, so any matcher that depends
// on word boundaries fails on at least one year.
func normalizeCaption(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '%':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// captionRule maps a normalised-caption substring to a metric. Order matters:
// the financial-impact rules must be tested before the plain loss and recovery
// rules, because their captions contain those words too.
type captionRule struct {
	needle string
	metric Metric
}

var captionRules = []captionRule{
	{"financiallossduetobreachoftd", MetricTDLossFinancialImpact},
	{"financiallossduetobreachoftdloss", MetricTDLossFinancialImpact},
	{"financiallossduetobreachoftdlosstarget", MetricTDLossFinancialImpact},
	{"financiallossduetobreachoflosstarget", MetricTDLossFinancialImpact},
	{"financiallossduetobreachofrecovery", MetricRecoveryFinancialImpct},
	{"financiallossduetobreachoftargets", MetricRecoveryFinancialImpct},
	{"financiallossdueto", MetricTDLossFinancialImpact},
	{"financialimpact", MetricTDLossFinancialImpact},

	{"systemaverageinterruptionfrequencyindex", MetricSAIFI},
	{"systemaveragefrequency", MetricSAIFI},
	{"saifi", MetricSAIFI},
	{"systemaverageinterruptiondurationindex", MetricSAIDI},
	{"systemaverageinterruptionduration", MetricSAIDI},
	{"systemaveragedurationfrequencyindex", MetricSAIDI},
	{"systemaveragedurationindex", MetricSAIDI},
	{"saidi", MetricSAIDI},

	{"transmissionanddistribution", MetricTDLosses},
	{"transmissionandistribution", MetricTDLosses},
	{"trissionandansmdistribution", MetricTDLosses},
	{"t%ddistributionlosses", MetricTDLosses},
	{"tdlosses", MetricTDLosses},

	{"billingcollection", MetricRecovery},
	{"recovery", MetricRecovery},

	{"timeframefornewconnection", MetricNewConnections},
	{"eligibleconsumer", MetricNewConnections},
	{"eligibleconsumers", MetricNewConnections},
	{"notprovidednewconnection", MetricNewConnections},
	// A caption that pairs "%" with "pending" is the percentage-of-ripe-
	// connections parameter, which is the same quantity as "% Eligible
	// consumer who were not provided new connection". A caption that says
	// "pending" without a "%" is the raw pending-connection count. They are
	// different measurements and must not share a metric key, or FY2022-23's
	// count table (Table 10) and percentage table (Table 21) manufacture ten
	// conflicts between counts and percentages.
	{"%ofpendingripeconnections", MetricNewConnections},
	{"%pendingripeconnections", MetricNewConnections},
	{"%ofpendingconnections", MetricNewConnections},
	{"pendingripeconnections", MetricPendingConnections},
	{"pendingconnections", MetricPendingConnections},
	{"pendingcoecnntions", MetricPendingConnections},
	{"agingof", MetricPendingConnections},

	{"loadshedding", MetricLoadShedding},
	// Any caption mentioning voltage is the nominal-voltage parameter, even
	// when it is phrased as a complaints count ("No. of Consumers complaints
	// who made about Voltages"). Without this the voltage tables land in
	// consumer_complaints and fabricate conflicts against the real complaints
	// tables.
	{"nominalvoltage", MetricNominalVoltage},
	{"voltage", MetricNominalVoltage},
	{"consumercomplaints", MetricConsumerComplaints},
	{"consumerservicecomplaints", MetricConsumerComplaints},
	{"consumercompts", MetricConsumerComplaints},
	{"complaints", MetricConsumerComplaints},
	{"safety", MetricSafety},
	{"fatalaccidents", MetricSafety},
	{"faultrate", MetricFaultRate},
	{"faultskm", MetricFaultRate},
	{"faults", MetricFaultRate},
}

// MetricFromCaption resolves a table caption (or a section heading) to a
// Metric. ok is false when nothing matches, in which case the caller records
// MetricUnknown together with the caption text rather than discarding the table.
func MetricFromCaption(caption string) (Metric, bool) {
	n := normalizeCaption(caption)
	if n == "" {
		return MetricUnknown, false
	}
	for _, r := range captionRules {
		if strings.Contains(n, r.needle) {
			return r.metric, true
		}
	}
	return MetricUnknown, false
}

// VariantFromCaption reads FY2022-23's explicit LT-interruption split out of a
// caption. ok is false when the caption carries no variant marker, leaving the
// variant to be decided from the table's own column headers.
func VariantFromCaption(caption string) (string, bool) {
	n := normalizeCaption(caption)
	switch {
	case strings.Contains(n, "withoutltinterruption"):
		return VariantWithoutLT, true
	case strings.Contains(n, "withltinterruption"):
		return VariantWithLT, true
	}
	return "", false
}

// captionPattern matches a real table caption: "TABLE 5", "Table 1:",
// "Table 0 6:...", "Table 14 : ...".
//
// It deliberately does NOT match the narrative cross-references that litter
// these reports ("Table 1 indicates the", "Table 16 mentions the %age of
// consumers"), which is why a separator or end-of-line is required right after
// the number.
var captionPattern = regexp.MustCompile(`^table0*([0-9]{1,2})(?:$|[:.\-])`)

// normalizeCaptionKey lowercases and removes only whitespace, keeping the
// punctuation captionPattern depends on. It is separate from normalizeCaption,
// which strips punctuation for needle matching.
func normalizeCaptionKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), ""))
}

// parseCaption reports whether a line is a table caption and, if so, returns
// the table label as printed ("Table 5") and the caption body.
func parseCaption(line string) (label, body string, ok bool) {
	m := captionPattern.FindStringSubmatch(normalizeCaptionKey(line))
	if m == nil {
		return "", "", false
	}
	label = "Table " + m[1]
	body = strings.TrimSpace(line)
	return label, body, true
}

// parseCaptionInLine finds a caption that shares its baseline with narrative
// prose. FY2018-19 sets its loss table beside a running commentary, so the
// caption line reads "picture is also given which TABLE 1" -- a whole-line
// match misses it and the report's first table disappears.
//
// Only a suffix beginning with a bare "TABLE"/"Table" cell is considered, which
// keeps the cross-references that riddle these reports ("as shown in (Table 1).
// SEPCO ...", "From the data shown in Table 6, it is noted") out.
func parseCaptionInLine(cells []string) (label, body string, ok bool) {
	if label, body, ok = parseCaption(strings.Join(cells, " ")); ok {
		return label, body, true
	}
	for i, c := range cells {
		if c != "TABLE" && c != "Table" {
			continue
		}
		if label, body, ok = parseCaption(strings.Join(cells[i:], " ")); ok {
			return label, body, true
		}
	}
	return "", "", false
}

// fyPattern matches NEPRA's fiscal-year column headers in every spelling the
// reports use: "2020-21", "2020-2021", "FY 2018-19", "2018 - 19".
var fyPattern = regexp.MustCompile(`^(?:fy)?(20[0-9]{2})[-–]{1,2}(20)?([0-9]{2})$`)

// FiscalYearFromHeader normalises a column header to the canonical
// "FY2020-21" form. ok is false when the header is not a fiscal year, which is
// how headline tables (whose headers are "Reported Figure", "Target by NEPRA")
// are told apart from five-year comparison tables.
func FiscalYearFromHeader(header string) (string, bool) {
	n := strings.Join(strings.Fields(strings.ToLower(header)), "")
	n = strings.ReplaceAll(n, "–", "-")
	m := fyPattern.FindStringSubmatch(n)
	if m == nil {
		return "", false
	}
	return "FY" + m[1] + "-" + m[3], true
}

// NormalizeFY canonicalises a fiscal-year string to "FY2020-21".
func NormalizeFY(fy string) string {
	if n, ok := FiscalYearFromHeader(fy); ok {
		return n
	}
	return strings.TrimSpace(fy)
}
