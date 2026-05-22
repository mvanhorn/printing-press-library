// Hand-authored: novel feature `score`.
//
// Score every row in an enriched CSV against a YAML ICP spec. Rules are
// declarative (regex / include / range) so an SDR can iterate the spec
// without code changes; no LLM involved. Reasons are emitted alongside
// the score so agents can explain a ranking decision.

package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ScoreResult is the top-level shape returned by `score`.
type ScoreResult struct {
	Rows          []ScoredRow `json:"rows"`
	TotalRows     int         `json:"total_rows"`
	AverageScore  float64     `json:"average_score"`
	QualifiedRows int         `json:"qualified_rows"`
	OutputPath    string      `json:"output_path,omitempty"`
}

// ScoredRow describes a single CSV row's score.
type ScoredRow struct {
	RowIndex     int      `json:"row_index"`
	Email        string   `json:"email,omitempty"`
	FullName     string   `json:"full_name,omitempty"`
	Score        int      `json:"score"`
	ScoreReasons []string `json:"score_reasons,omitempty"`
}

// icpSpec is the on-disk YAML shape consumed by `score`.
type icpSpec struct {
	ICP struct {
		Name  string    `yaml:"name"`
		Rules []icpRule `yaml:"rules"`
	} `yaml:"icp"`
}

// icpRule is one entry under icp.rules. Match type drives which fields are read.
type icpRule struct {
	Field   string   `yaml:"field"`
	Match   string   `yaml:"match"` // "regex" | "include" | "range"
	Pattern string   `yaml:"pattern,omitempty"`
	Values  []string `yaml:"values,omitempty"`
	Min     *float64 `yaml:"min,omitempty"`
	Max     *float64 `yaml:"max,omitempty"`
	Weight  int      `yaml:"weight"`
	Label   string   `yaml:"label,omitempty"`

	// compiled artifacts (populated by prepare)
	regex     *regexp.Regexp
	valuesLow []string
}

func newScoreCmd(flags *rootFlags) *cobra.Command {
	var icpPath, inputPath, outputPath string
	var threshold int

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Score every enriched row against a YAML ICP spec (titles, sizes, geos) and emit a score column with reasons.",
		Long: `Reads a YAML ICP spec and an enriched CSV. For each row, walks the rule
list and adds the rule's weight to the row's score when the rule matches.
Match types:
  - regex:   field value matches Go regexp (e.g. "(?i)\\b(cto|vp engineering)\\b")
  - include: field value (case-insensitive) is in the values list
  - range:   numeric field falls within [min, max]

Scores are capped at 100. Use --threshold to control which rows count as
"qualified" in the JSON summary (default 60). Use --output to write a CSV
with original columns + score + score_reasons (semicolon-separated).

Nested fields like "company.industry" are matched against "company_industry"
or the bare "industry" column when the input CSV is flat.`,
		Example: "  prospeo-pp-cli score --icp icp.yaml --input enriched.csv\n" +
			"  prospeo-pp-cli score --icp icp.yaml --input enriched.csv --output scored.csv --threshold 70 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if icpPath == "" && inputPath == "" && !dryRunOK(flags) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would score rows from %s against ICP %s (threshold=%d)\n", inputPath, icpPath, threshold)
				return nil
			}
			if icpPath == "" {
				return usageErr(fmt.Errorf("--icp is required"))
			}
			if inputPath == "" {
				return usageErr(fmt.Errorf("--input is required"))
			}
			spec, err := loadICPSpec(icpPath)
			if err != nil {
				return err
			}
			result, err := runScore(spec, inputPath, outputPath, threshold)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&icpPath, "icp", "", "Path to a YAML ICP spec (required).")
	cmd.Flags().StringVar(&inputPath, "input", "", "Path to the enriched CSV to score (required).")
	cmd.Flags().StringVar(&outputPath, "output", "", "Optional output CSV path. Adds score + score_reasons columns.")
	cmd.Flags().IntVar(&threshold, "threshold", 60, "Score >= threshold is counted as 'qualified' in the summary.")
	return cmd
}

func loadICPSpec(path string) (*icpSpec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ICP file: %w", err)
	}
	var spec icpSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse ICP YAML: %w", err)
	}
	if len(spec.ICP.Rules) == 0 {
		return nil, fmt.Errorf("ICP spec at %s has no rules under icp.rules", path)
	}
	for i := range spec.ICP.Rules {
		r := &spec.ICP.Rules[i]
		switch r.Match {
		case "regex":
			if r.Pattern == "" {
				return nil, fmt.Errorf("rule %d (field=%s): regex match needs 'pattern'", i, r.Field)
			}
			re, err := regexp.Compile(r.Pattern)
			if err != nil {
				return nil, fmt.Errorf("rule %d (field=%s): compile pattern: %w", i, r.Field, err)
			}
			r.regex = re
		case "include":
			if len(r.Values) == 0 {
				return nil, fmt.Errorf("rule %d (field=%s): include match needs 'values'", i, r.Field)
			}
			r.valuesLow = make([]string, len(r.Values))
			for j, v := range r.Values {
				r.valuesLow[j] = strings.ToLower(strings.TrimSpace(v))
			}
		case "range":
			// min/max optional but at least one
			if r.Min == nil && r.Max == nil {
				return nil, fmt.Errorf("rule %d (field=%s): range match needs at least one of min/max", i, r.Field)
			}
		default:
			return nil, fmt.Errorf("rule %d (field=%s): unknown match type %q (want regex|include|range)", i, r.Field, r.Match)
		}
	}
	return &spec, nil
}

func runScore(spec *icpSpec, inputPath, outputPath string, threshold int) (*ScoreResult, error) {
	if threshold <= 0 {
		threshold = 60
	}
	f, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open input CSV: %w", err)
	}
	defer f.Close()
	rdr := csv.NewReader(f)
	rdr.FieldsPerRecord = -1
	records, err := rdr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read input CSV: %w", err)
	}
	if len(records) == 0 {
		return &ScoreResult{Rows: []ScoredRow{}, OutputPath: outputPath}, nil
	}
	headers := records[0]
	headerIdx := buildHeaderIndex(headers)

	rows := records[1:]
	scored := make([]ScoredRow, 0, len(rows))
	totalScore := 0
	qualified := 0
	for i, row := range rows {
		s, reasons := scoreRow(spec, headerIdx, row)
		sr := ScoredRow{
			RowIndex:     i,
			Email:        lookupRow(headerIdx, row, "email"),
			FullName:     firstNonEmpty(lookupRow(headerIdx, row, "full_name"), lookupRow(headerIdx, row, "name")),
			Score:        s,
			ScoreReasons: reasons,
		}
		scored = append(scored, sr)
		totalScore += s
		if s >= threshold {
			qualified++
		}
	}

	result := &ScoreResult{
		Rows:          scored,
		TotalRows:     len(rows),
		QualifiedRows: qualified,
		OutputPath:    outputPath,
	}
	if len(rows) > 0 {
		result.AverageScore = float64(totalScore) / float64(len(rows))
	}

	if outputPath != "" {
		if err := writeScoredCSV(outputPath, headers, rows, scored); err != nil {
			return result, fmt.Errorf("write scored CSV: %w", err)
		}
	}
	return result, nil
}

// scoreRow walks each rule and accumulates weight + reason on match.
func scoreRow(spec *icpSpec, headerIdx map[string]int, row []string) (int, []string) {
	score := 0
	var reasons []string
	for _, r := range spec.ICP.Rules {
		val := lookupField(headerIdx, row, r.Field)
		matched := false
		switch r.Match {
		case "regex":
			if val != "" && r.regex != nil && r.regex.MatchString(val) {
				matched = true
			}
		case "include":
			low := strings.ToLower(strings.TrimSpace(val))
			if low == "" {
				break
			}
			for _, v := range r.valuesLow {
				if low == v {
					matched = true
					break
				}
			}
		case "range":
			n, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
			if err != nil {
				break
			}
			lo := true
			hi := true
			if r.Min != nil && n < *r.Min {
				lo = false
			}
			if r.Max != nil && n > *r.Max {
				hi = false
			}
			matched = lo && hi
		}
		if matched {
			score += r.Weight
			if r.Label != "" {
				reasons = append(reasons, r.Label)
			} else {
				reasons = append(reasons, r.Field)
			}
		}
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score, reasons
}

// buildHeaderIndex lowercases all headers for case-insensitive lookups.
func buildHeaderIndex(headers []string) map[string]int {
	out := make(map[string]int, len(headers))
	for i, h := range headers {
		out[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return out
}

// lookupField resolves a (possibly dotted) ICP field name against the flat CSV.
// "company.industry" tries: company.industry, company_industry, companyindustry, industry.
func lookupField(idx map[string]int, row []string, field string) string {
	candidates := []string{strings.ToLower(field)}
	if strings.Contains(field, ".") {
		segs := strings.Split(field, ".")
		under := strings.ToLower(strings.Join(segs, "_"))
		concat := strings.ToLower(strings.Join(segs, ""))
		bare := strings.ToLower(segs[len(segs)-1])
		candidates = append(candidates, under, concat, bare)
	}
	for _, c := range candidates {
		if i, ok := idx[c]; ok && i < len(row) {
			v := strings.TrimSpace(row[i])
			if v != "" {
				return v
			}
		}
	}
	return ""
}

func lookupRow(idx map[string]int, row []string, key string) string {
	if i, ok := idx[strings.ToLower(key)]; ok && i < len(row) {
		return row[i]
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func writeScoredCSV(path string, headers []string, rows [][]string, scored []ScoredRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	// Avoid duplicating an existing "score" column.
	outHeaders := append([]string{}, headers...)
	hasScore := false
	hasReasons := false
	for _, h := range headers {
		l := strings.ToLower(strings.TrimSpace(h))
		if l == "score" {
			hasScore = true
		}
		if l == "score_reasons" {
			hasReasons = true
		}
	}
	if !hasScore {
		outHeaders = append(outHeaders, "score")
	}
	if !hasReasons {
		outHeaders = append(outHeaders, "score_reasons")
	}
	if err := w.Write(outHeaders); err != nil {
		return err
	}

	// Stable scored index by row index.
	byIdx := make(map[int]ScoredRow, len(scored))
	for _, s := range scored {
		byIdx[s.RowIndex] = s
	}
	// Order rows by original index, matching the input.
	keys := make([]int, 0, len(byIdx))
	for k := range byIdx {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	for _, k := range keys {
		s := byIdx[k]
		row := append([]string{}, rows[k]...)
		// Pad row to header width if jagged.
		for len(row) < len(headers) {
			row = append(row, "")
		}
		if !hasScore {
			row = append(row, strconv.Itoa(s.Score))
		}
		if !hasReasons {
			row = append(row, strings.Join(s.ScoreReasons, "; "))
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
