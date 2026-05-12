package blsdata

// pp:novel-static-reference
//
// Footnote codes are documented at https://download.bls.gov/pub/time.series/<survey>/<abbr>.footnote
// and propagate the same single-letter conventions across most surveys. We
// embed the canonical letters here; survey-specific codes can be authored
// later via a flat-file ingest path.

// FootnoteCode is one footnote entry.
type FootnoteCode struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

// Footnotes returns the BLS footnote-code lookup table. Codes are
// case-sensitive; BLS publishes them as single uppercase letters.
func Footnotes() []FootnoteCode {
	return []FootnoteCode{
		{Code: "A", Text: "Annual averages do not include benchmark values; some categories are not seasonally adjusted."},
		{Code: "B", Text: "Reflects a discontinuity from prior series. Use caution comparing across the break."},
		{Code: "C", Text: "Corrected. Replaces a previously published value."},
		{Code: "D", Text: "Data are not available because category contains few firms or workers."},
		{Code: "E", Text: "Annual average computed from data published in the news release. Differs slightly from annual averages calculated from monthly values."},
		{Code: "F", Text: "Forecast or projected value."},
		{Code: "G", Text: "Adjusted to a new geographic or industry classification (e.g., NAICS revision)."},
		{Code: "H", Text: "Estimate has high relative standard error; interpret with caution."},
		{Code: "I", Text: "Imputed value used when the actual observation was unavailable."},
		{Code: "J", Text: "Reference period changed."},
		{Code: "K", Text: "Confidential; data withheld to avoid disclosing data of an individual reporting entity."},
		{Code: "L", Text: "Less than 0.05 (or 0.5 depending on series precision)."},
		{Code: "M", Text: "Methodology change applied this period."},
		{Code: "N", Text: "Not available."},
		{Code: "O", Text: "Outlier; observation may be affected by a one-time event (e.g., natural disaster, strike)."},
		{Code: "P", Text: "Preliminary; will be revised in subsequent releases."},
		{Code: "Q", Text: "Quartile or quantile boundary; not a standard observation."},
		{Code: "R", Text: "Revised; replaces a value published earlier."},
		{Code: "S", Text: "Special; refer to release notes for interpretation."},
		{Code: "T", Text: "Truncated for confidentiality."},
		{Code: "U", Text: "Unavailable for this period."},
		{Code: "V", Text: "Volatile; treat short-term movements with caution."},
		{Code: "W", Text: "Withdrawn; series no longer published."},
		{Code: "X", Text: "Suppressed for disclosure reasons."},
		{Code: "Y", Text: "Year-over-year change basis (rather than month-over-month)."},
		{Code: "Z", Text: "Zero; value rounds to zero at the published precision."},
		// Numeric codes from the API (footnotes object also uses these)
		{Code: "1", Text: "Includes other industries not shown separately."},
		{Code: "2", Text: "Discontinued series."},
		{Code: "3", Text: "Suppressed to avoid disclosure of confidential information."},
		{Code: "4", Text: "Reflects expanded geographic coverage starting this period."},
		{Code: "5", Text: "Insufficient response to meet publication standards."},
		{Code: "6", Text: "Updated population controls applied."},
		{Code: "7", Text: "Annual updates applied; revisions reflect benchmark."},
		{Code: "8", Text: "Includes data benchmarked or revised in this period."},
		{Code: "9", Text: "Data unavailable for this period."},
	}
}

// DecodeFootnote returns the plain-English text for a single footnote code,
// or empty string if the code is unknown.
func DecodeFootnote(code string) string {
	for _, f := range Footnotes() {
		if f.Code == code {
			return f.Text
		}
	}
	return ""
}
