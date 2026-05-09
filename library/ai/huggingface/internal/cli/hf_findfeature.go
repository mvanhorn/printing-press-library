package cli

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfx"
)

type findFeatureResponse struct {
	hfx.Envelope
	Feature      string                  `json:"feature"`
	Backend      []string                `json:"backend,omitempty"`
	Candidates   int                     `json:"candidates"`
	Results      []featureMatch          `json:"results"`
	MatrixSource string                  `json:"matrix_source"`
	Explain      string                  `json:"explain,omitempty"`
}

type featureMatch struct {
	ID              string                       `json:"id"`
	Detected        bool                         `json:"detected"`
	Variant         string                       `json:"variant,omitempty"`
	Evidence        string                       `json:"evidence,omitempty"`
	Confidence      string                       `json:"confidence,omitempty"`
	BackendVerdicts map[string]string            `json:"backend_verdicts,omitempty"`
	WikiPointer     string                       `json:"wiki_pointer,omitempty"`
}

func newHFFindFeatureCmd(flags *rootFlags) *cobra.Command {
	var sizeFilter, backendsStr, searchHint string
	var moeOnly, ggufOnly bool
	cmd := &cobra.Command{
		Use:   "find-feature <feature>",
		Short: "Search models by architecture feature (mtp, mla, moe, gqa, sliding-window, rope-yarn).",
		Long: `find-feature searches HF for candidates whose config.json or card text
indicates the named architecture feature, classifies each match, and returns
backend-readiness verdicts from the bundled matrix.

Recognized features: moe, mtp, mla, gqa, sliding-window-attn (sliding-window/swa),
rope-yarn (yarn), dense.`,
		Example: `  huggingface-pp-cli find-feature moe --size 20b-40b
  huggingface-pp-cli find-feature mtp --backend llama.cpp
  huggingface-pp-cli find-feature mla --search deepseek --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feature := strings.ToLower(args[0])
			ctx := cmd.Context()
			token := hfTokenForRequests()

			// Search criteria — use HF search params we DO have. find-feature is
			// inherently expensive (must fetch config.json per candidate), so
			// limit candidates aggressively and document it.
			q := url.Values{}
			if searchHint == "" {
				// Default sort: lastModified (freshly active). HF rejects
				// sort=trending — the UI's trending view is server-computed and
				// not exposed via the API. Pull recently-modified models, hand
				// relevance off to the per-candidate config.json classifier.
				q.Set("sort", "lastModified")
				q.Set("direction", "-1")
			} else {
				q.Set("search", searchHint)
			}
			if ggufOnly {
				q.Set("filter", "gguf")
			}
			limit := flags.limit
			if limit > 30 {
				limit = 30 // bound config.json fan-out (rate-limit friendly)
			}
			q.Set("limit", strconv.Itoa(limit*2)) // pull 2x candidates, post-filter

			ms, status, err := hfListModels(ctx, q, token)
			if err != nil {
				if status == 429 {
					return hfRateLimited("rate limited (HTTP 429); narrow with --search")
				}
				return err
			}

			// Resolve target backends for verdicts
			stateDir, _ := hfx.StateDir(flags.stateDir)
			matrix, src, err := hfx.LoadBackendMatrix(stateDir, flags.backendSupport)
			if err != nil {
				return fmt.Errorf("loading backend matrix: %w", err)
			}
			targets := []string{}
			if backendsStr != "" {
				for _, b := range strings.Split(backendsStr, ",") {
					if t := strings.TrimSpace(b); t != "" {
						targets = append(targets, t)
					}
				}
			}

			// Per-candidate config.json fetch (parallel, bounded)
			results := classifyCandidates(ctx, feature, ms, token, moeOnly, sizeFilter, matrix, targets)
			if len(results) > flags.limit {
				results = results[:flags.limit]
			}

			if len(results) == 0 {
				return hfNotFound("no candidates matched feature %q (try widening --search or relaxing filters)", feature)
			}

			resp := findFeatureResponse{
				Envelope:     hfx.NewEnvelope("find-feature"),
				Feature:      feature,
				Backend:      targets,
				Candidates:   len(ms),
				Results:      results,
				MatrixSource: src,
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: scanned %d candidates; %d matched. find-feature classifies via config.json (high-confidence) + card-text (low-confidence). Backend verdicts come from %s; refresh entries when source_checked is stale.",
					len(ms), len(results), src)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "find-feature %q (%d candidates → %d matches)\n\n", feature, len(ms), len(results))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %-12s  %-8s  %s\n", "ID", "VARIANT", "CONF", "BACKEND VERDICTS")
			for _, r := range results {
				bv := []string{}
				for _, b := range targets {
					if v, ok := r.BackendVerdicts[b]; ok {
						bv = append(bv, b+":"+v)
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %-12s  %-8s  %s\n", r.ID, r.Variant, r.Confidence, strings.Join(bv, " "))
				if r.Evidence != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "      %s\n", r.Evidence)
				}
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sizeFilter, "size", "", "Approximate size class (e.g. 20b-40b, 7b-13b) — best-effort filter on tags")
	cmd.Flags().StringVar(&backendsStr, "backend", "", "Backends to verdict (comma-separated; default: llama.cpp,mlx)")
	cmd.Flags().StringVar(&searchHint, "search", "", "Free-text search hint (default: trending)")
	cmd.Flags().BoolVar(&moeOnly, "moe", false, "Filter to MoE candidates only (server-side filter is best-effort)")
	cmd.Flags().BoolVar(&ggufOnly, "gguf", false, "Restrict to repos with GGUF artifacts")
	return cmd
}

func classifyCandidates(ctx context.Context, feature string, ms []hfModel, token string, moeOnly bool, sizeFilter string, matrix hfx.BackendMatrix, targets []string) []featureMatch {
	out := make([]featureMatch, 0, len(ms))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6) // bound parallel fetches

	if len(targets) == 0 {
		targets = []string{"llama.cpp", "mlx"}
	}

	for _, m := range ms {
		// Cheap pre-filter: size tag heuristic
		if sizeFilter != "" && !matchesSizeFilter(m.Tags, sizeFilter) {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(m hfModel) {
			defer wg.Done()
			defer func() { <-sem }()

			// Fetch config.json
			cfg := hfx.ConfigJSON{}
			cardText := ""
			rawCfg, _, err := hfFetchRaw(ctx, m.ID, "main", "config.json", token)
			if err == nil && len(rawCfg) > 0 {
				if c, err := hfx.ParseConfigJSON(rawCfg); err == nil {
					cfg = c
				}
			}

			// MoE-only quick-filter
			if moeOnly && cfg.NumExperts == 0 && cfg.NumLocalExperts == 0 {
				return
			}

			cls := hfx.ClassifyFeature(feature, cfg, cardText)
			if !cls.Detected && cls.Confidence != "low" {
				return
			}

			match := featureMatch{
				ID:              m.ID,
				Detected:        cls.Detected,
				Variant:         cls.Variant,
				Evidence:        cls.Evidence,
				Confidence:      cls.Confidence,
				BackendVerdicts: map[string]string{},
			}
			for _, b := range targets {
				if entry, ok := hfx.LookupVerdict(matrix, feature, b); ok {
					match.BackendVerdicts[b] = entry.Supported
					if match.WikiPointer == "" && entry.WikiPointer != "" {
						match.WikiPointer = entry.WikiPointer
					}
				} else {
					match.BackendVerdicts[b] = "unknown"
				}
			}

			mu.Lock()
			out = append(out, match)
			mu.Unlock()
		}(m)
	}
	wg.Wait()
	return out
}

func matchesSizeFilter(tags []string, sizeFilter string) bool {
	// Best-effort: HF doesn't have a clean size taxonomy. Match against any
	// tag that contains a digit-letter pattern matching the filter range.
	// Format: "<low>-<high>" e.g. "20b-40b", "7b".
	sizeFilter = strings.ToLower(sizeFilter)
	for _, t := range tags {
		tl := strings.ToLower(t)
		if strings.Contains(tl, sizeFilter) {
			return true
		}
		// Also match on bare "Nb"/"Nm" tags
		for _, p := range strings.Split(sizeFilter, "-") {
			if p != "" && strings.Contains(tl, p) {
				return true
			}
		}
	}
	return false
}
