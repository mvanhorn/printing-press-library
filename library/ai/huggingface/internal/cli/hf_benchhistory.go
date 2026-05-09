package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

type benchHistoryResponse struct {
	hfx.Envelope
	Candidate    string     `json:"candidate"`
	HarnessRoot  string     `json:"harness_root"`
	MemoryDir    string     `json:"memory_dir"`
	MatchKeys    []string   `json:"match_keys"`
	Runs         []benchRun `json:"runs"`
	BaselineRuns []benchRun `json:"baseline_runs,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	Explain      string     `json:"explain,omitempty"`
}

type benchRun struct {
	Date       string            `json:"date"`
	HarnessKey string            `json:"harness_key"`
	Label      string            `json:"label"`
	Passed     int               `json:"passed"`
	Total      int               `json:"total"`
	FirstPass  int               `json:"first_pass,omitempty"`
	AvgTok     float64           `json:"avg_tok,omitempty"`
	AvgTimeSec float64           `json:"avg_time_sec,omitempty"`
	ByType     map[string]string `json:"by_type,omitempty"`
	ByCat      map[string]string `json:"by_cat,omitempty"`
	SourceFile string            `json:"source_file"`
}

func newHFBenchHistoryCmd(flags *rootFlags) *cobra.Command {
	var harnessFlag, baselineKey string
	cmd := &cobra.Command{
		Use:   "bench-history <id>",
		Short: "Join HF id with local model-eval-harness results.",
		Long: `bench-history walks workspace/memory/model-eval-*.json (or the dir under
--harness) and matches harness keys against an HF id via substring heuristic.
Returns historical runs (date, passed/total, byType breakdown) and optionally
a vs-baseline delta when --baseline is provided.

Exits 6 cleanly when no harness data dir is reachable.

Heuristic: harness keys are hand-curated short labels (e.g. "qwen3-30b-deepseek-distill-q5"),
not slug-derived. Match is case-insensitive substring on the model leaf name.`,
		Example: `  huggingface-pp-cli bench-history Qwen/Qwen3-30B-A3B
  huggingface-pp-cli bench-history Qwen/Qwen3-30B-A3B --baseline gemma4-26b-iq4 --json
  huggingface-pp-cli bench-history bartowski/Qwen2.5-7B-Instruct-GGUF --harness /tmp/harness`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			candidate := args[0]

			// Resolve harness root + the workspace/memory dir that holds model-eval-*.json
			harnessRoot := harnessFlag
			if harnessRoot == "" {
				cands := []string{"workspace/scripts/model-eval-harness"}
				if root := hfOpenclawRoot(); root != "" {
					cands = append(cands, filepath.Join(root, "workspace", "scripts", "model-eval-harness"))
				}
				harnessRoot = firstExistingPath(cands)
			}
			if harnessRoot == "" {
				return hfConfigMissing("no harness dir found (pass --harness or run from the OpenClaw repo)")
			}
			// memory dir is the sibling: ../../memory relative to harness root
			memoryDir := filepath.Join(filepath.Dir(filepath.Dir(harnessRoot)), "memory")
			if _, err := os.Stat(memoryDir); err != nil {
				// Fall back: harnessRoot/../memory
				memoryDir = filepath.Join(filepath.Dir(harnessRoot), "..", "memory")
				if _, err := os.Stat(memoryDir); err != nil {
					return hfConfigMissing("no memory dir found at %s (harness output expected at workspace/memory/model-eval-*.json)", memoryDir)
				}
			}
			memoryAbs, _ := filepath.Abs(memoryDir)

			// Find all model-eval-*.json files
			files, err := filepath.Glob(filepath.Join(memoryDir, "model-eval-*.json"))
			if err != nil {
				return hfConfigMissing("listing harness output: %v", err)
			}
			if len(files) == 0 {
				return hfNotFound("no model-eval-*.json files in %s — run the harness first", memoryAbs)
			}

			leaf := strings.ToLower(strings.SplitN(candidate, "/", 2)[len(strings.SplitN(candidate, "/", 2))-1])
			// Strip common quant/format suffixes from leaf to widen match
			for _, sfx := range []string{"-gguf", "-instruct-gguf", "-mlx"} {
				leaf = strings.TrimSuffix(leaf, sfx)
			}

			matchedKeys := map[string]bool{}
			runs := []benchRun{}
			var baselineRuns []benchRun

			for _, f := range files {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				var doc struct {
					Date    string                 `json:"date"`
					Results map[string]benchRawRow `json:"results"`
				}
				if err := json.Unmarshal(data, &doc); err != nil {
					continue
				}
				date := doc.Date
				if date == "" {
					// Fall back to filename date
					base := filepath.Base(f)
					date = strings.TrimPrefix(strings.TrimSuffix(base, ".json"), "model-eval-")
				}
				for key, row := range doc.Results {
					keyLower := strings.ToLower(key)
					labelLower := strings.ToLower(row.Label)
					match := strings.Contains(keyLower, leaf) || strings.Contains(labelLower, leaf)
					if match {
						matchedKeys[key] = true
						runs = append(runs, row.toRun(date, key, f))
						continue
					}
					if baselineKey != "" && (strings.EqualFold(key, baselineKey) || strings.Contains(labelLower, strings.ToLower(baselineKey))) {
						baselineRuns = append(baselineRuns, row.toRun(date, key, f))
					}
				}
			}

			if len(runs) == 0 {
				return hfNotFound("no harness runs match HF id %q (matched against leaf=%q across %d eval files)", candidate, leaf, len(files))
			}

			// Sort by date desc
			sort.SliceStable(runs, func(i, j int) bool { return runs[i].Date > runs[j].Date })
			sort.SliceStable(baselineRuns, func(i, j int) bool { return baselineRuns[i].Date > baselineRuns[j].Date })

			matchKeysSlice := make([]string, 0, len(matchedKeys))
			for k := range matchedKeys {
				matchKeysSlice = append(matchKeysSlice, k)
			}
			sort.Strings(matchKeysSlice)

			resp := benchHistoryResponse{
				Envelope:     hfx.NewEnvelope("bench-history"),
				Candidate:    candidate,
				HarnessRoot:  harnessRoot,
				MemoryDir:    memoryAbs,
				MatchKeys:    matchKeysSlice,
				Runs:         runs,
				BaselineRuns: baselineRuns,
				Notes:        "Match heuristic: case-insensitive substring on harness key + label vs candidate's leaf name. Suffixes -gguf/-instruct-gguf/-mlx are stripped.",
			}
			if flags.explain {
				latest := runs[0]
				summary := fmt.Sprintf("explain: %d runs found across %d harness eval files; latest %s passed=%d/%d",
					len(runs), len(files), latest.Date, latest.Passed, latest.Total)
				if len(baselineRuns) > 0 {
					b := baselineRuns[0]
					delta := latest.Passed - b.Passed
					summary += fmt.Sprintf("; baseline %s passed=%d/%d (delta=%+d)", b.HarnessKey, b.Passed, b.Total, delta)
				}
				resp.Explain = summary
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "bench-history for %s\n", candidate)
			fmt.Fprintf(cmd.OutOrStdout(), "  matched keys: %s\n\n", strings.Join(matchKeysSlice, ", "))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %-40s  %-9s  %s\n", "DATE", "HARNESS_KEY", "PASSED", "AVG_TOK")
			for _, r := range runs {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %-40s  %-9s  %.0f\n", r.Date, r.HarnessKey, fmt.Sprintf("%d/%d", r.Passed, r.Total), r.AvgTok)
			}
			if len(baselineRuns) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\n  Baseline:")
				for _, r := range baselineRuns {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %-40s  %-9s  %.0f\n", r.Date, r.HarnessKey, fmt.Sprintf("%d/%d", r.Passed, r.Total), r.AvgTok)
				}
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&harnessFlag, "harness", "", "Path to model-eval-harness dir (default: auto-discover)")
	cmd.Flags().StringVar(&baselineKey, "baseline", "", "Harness key OR label substring of the baseline to diff against")
	return cmd
}

// benchRawRow mirrors the shape inside results.<key> in model-eval-<date>.json.
type benchRawRow struct {
	Label     string            `json:"label"`
	Passed    int               `json:"passed"`
	Total     int               `json:"total"`
	FirstPass int               `json:"firstPass"`
	AvgTok    float64           `json:"avgTok"`
	AvgTime   float64           `json:"avgTime"`
	ByType    map[string]string `json:"byType"`
	ByCat     map[string]string `json:"byCat"`
}

func (r benchRawRow) toRun(date, key, source string) benchRun {
	return benchRun{
		Date:       date,
		HarnessKey: key,
		Label:      r.Label,
		Passed:     r.Passed,
		Total:      r.Total,
		FirstPass:  r.FirstPass,
		AvgTok:     r.AvgTok,
		AvgTimeSec: r.AvgTime,
		ByType:     r.ByType,
		ByCat:      r.ByCat,
		SourceFile: source,
	}
}
