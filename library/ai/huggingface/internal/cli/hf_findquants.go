package cli

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfx"
)

// findQuantsOpts is the programmatic interface used by both find-quants RunE
// and eval-candidates. Lifted out so neither command duplicates filter parsing.
type findQuantsOpts struct {
	Preferred    []string        // canonical UPPER quant codes (e.g. ["IQ4_NL", "Q4_K_M"])
	Uploaders    map[string]bool // optional uploader allowlist (lower-case keys)
	MaxSizeBytes int64           // 0 = no size cap
	Limit        int             // result-set ceiling
}

// findQuantsCoreResult is the post-filter shape: the ranked rows + a report
// describing what got excluded.
type findQuantsCoreResult struct {
	Results []quantResult
	Filter  filterReport
}

// parseFindQuantsOpts parses CLI strings (comma lists, human size) into the
// structured opts used by findQuantsCore. Pure; no I/O.
func parseFindQuantsOpts(preferStr, uploadersStr, maxSize string, limit int) findQuantsOpts {
	opts := findQuantsOpts{Limit: limit}
	if preferStr != "" {
		for _, p := range strings.Split(preferStr, ",") {
			if t := strings.TrimSpace(strings.ToUpper(p)); t != "" {
				opts.Preferred = append(opts.Preferred, t)
			}
		}
	}
	if uploadersStr != "" {
		opts.Uploaders = map[string]bool{}
		for _, u := range strings.Split(uploadersStr, ",") {
			if t := strings.TrimSpace(strings.ToLower(u)); t != "" {
				opts.Uploaders[t] = true
			}
		}
	}
	if maxSize != "" {
		opts.MaxSizeBytes = parseHumanSize(maxSize)
	}
	return opts
}

// findQuantsCore is the programmatic path: list candidates from HF, filter +
// rank by trusted-uploader allowlist + size + preferred quant codes, return
// the truncated result + a filter-counts report. Returns an empty result
// (not an error) when no rows survive — the caller decides whether that's
// "not found" or fall-through.
func findQuantsCore(ctx context.Context, base string, opts findQuantsOpts) (findQuantsCoreResult, error) {
	q := url.Values{}
	q.Set("search", base)
	q.Set("filter", "gguf")
	pull := opts.Limit * 4
	if pull < 100 {
		pull = 100
	}
	q.Set("limit", strconv.Itoa(pull))
	q.Set("full", "true")

	ms, status, err := hfListModels(ctx, q, hfTokenForRequests())
	if err != nil {
		if status == 429 {
			return findQuantsCoreResult{}, hfRateLimited("rate limited (HTTP 429)")
		}
		return findQuantsCoreResult{}, err
	}

	report := filterReport{
		MaxSizeBytes:    opts.MaxSizeBytes,
		PreferredQuants: opts.Preferred,
	}
	if opts.Uploaders != nil {
		report.OnlyUploaders = sortKeys(opts.Uploaders)
	}

	results := []quantResult{}
	baseLower := strings.ToLower(base)
	baseLeaf := strings.ToLower(strings.SplitN(base, "/", 2)[len(strings.SplitN(base, "/", 2))-1])

	for _, m := range ms {
		if !strings.Contains(strings.ToLower(m.ID), baseLeaf) && !hasBaseModelTag(m, baseLower) {
			continue
		}
		uploader := m.Author
		if uploader == "" && strings.Contains(m.ID, "/") {
			uploader = strings.SplitN(m.ID, "/", 2)[0]
		}
		if opts.Uploaders != nil && !opts.Uploaders[strings.ToLower(uploader)] {
			report.UntrustedExcluded++
			continue
		}
		for _, s := range m.Siblings {
			if !hfx.IsGGUF(s.Path) {
				continue
			}
			qp, ok := hfx.DetectQuant(s.Path)
			if !ok {
				continue
			}
			size := s.Size
			if size == 0 && s.LFS != nil {
				size = s.LFS.Size
			}
			if len(opts.Preferred) > 0 {
				match := false
				for _, p := range opts.Preferred {
					if strings.Contains(strings.ToUpper(qp.Code), p) {
						match = true
						break
					}
				}
				if !match {
					report.NotPreferred++
					continue
				}
			}
			if opts.MaxSizeBytes > 0 && size > 0 && size > opts.MaxSizeBytes {
				report.OversizeExcluded++
				continue
			}
			results = append(results, quantResult{
				ID:           m.ID,
				Uploader:     uploader,
				UploaderRep:  hfx.IsTrustedUploader(uploader),
				Quant:        qp.Code,
				QuantFamily:  qp.Family,
				SizeBytes:    size,
				SizeGB:       hfHumanGB(size),
				Downloads:    m.Downloads,
				LastModified: m.LastModified,
			})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.UploaderRep != b.UploaderRep {
			return a.UploaderRep
		}
		if a.SizeBytes != b.SizeBytes && a.SizeBytes > 0 && b.SizeBytes > 0 {
			return a.SizeBytes < b.SizeBytes
		}
		return a.Downloads > b.Downloads
	})
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return findQuantsCoreResult{Results: results, Filter: report}, nil
}

type findQuantsResponse struct {
	hfx.Envelope
	BaseModel string         `json:"base_model"`
	Results   []quantResult  `json:"results"`
	Filtered  filterReport   `json:"filtered,omitempty"`
	Explain   string         `json:"explain,omitempty"`
}

type quantResult struct {
	ID           string `json:"id"`
	Uploader     string `json:"uploader"`
	UploaderRep  bool   `json:"uploader_rep"`
	Quant        string `json:"quant,omitempty"`
	QuantFamily  string `json:"quant_family,omitempty"`
	SizeBytes    int64  `json:"size_bytes,omitempty"`
	SizeGB       string `json:"size_gb,omitempty"`
	Downloads    int    `json:"downloads"`
	LastModified string `json:"last_modified"`
}

type filterReport struct {
	UntrustedExcluded int      `json:"untrusted_excluded,omitempty"`
	OversizeExcluded  int      `json:"oversize_excluded,omitempty"`
	NotPreferred      int      `json:"not_preferred_excluded,omitempty"`
	OnlyUploaders     []string `json:"only_uploaders,omitempty"`
	MaxSizeBytes      int64    `json:"max_size_bytes,omitempty"`
	PreferredQuants   []string `json:"preferred_quants,omitempty"`
}

func newHFFindQuantsCmd(flags *rootFlags) *cobra.Command {
	var preferStr, uploadersStr, maxSize string
	cmd := &cobra.Command{
		Use:   "find-quants <base-model>",
		Short: "Find GGUF quant variants of a base model, sorted by uploader rep + size.",
		Long: `find-quants searches HF for repos tagged base_model:quantized:<id>
or base_model:<id>, filters to GGUF artifacts, parses quant codes from
filenames, and sorts by trusted-uploader allowlist + size.

Default trusted-uploader set: unsloth, bartowski, mradermacher.`,
		Example: `  huggingface-pp-cli find-quants Qwen/Qwen2.5-7B-Instruct
  huggingface-pp-cli find-quants Qwen/Qwen3-MoE-A14B --max-size 25g --prefer iq4_nl,q4_k_m
  huggingface-pp-cli find-quants Qwen/Qwen2.5-7B --uploaders bartowski,mradermacher --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base := args[0]
			opts := parseFindQuantsOpts(preferStr, uploadersStr, maxSize, flags.limit)
			core, err := findQuantsCore(cmd.Context(), base, opts)
			if err != nil {
				return err
			}
			if len(core.Results) == 0 {
				return hfNotFound("no quants found for %q (try widening --uploaders or removing --prefer)", base)
			}

			resp := findQuantsResponse{
				Envelope:  hfx.NewEnvelope("find-quants"),
				BaseModel: base,
				Results:   core.Results,
				Filtered:  core.Filter,
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: %d quants surface after filters; trusted=%d. Sort: trusted-first, then ascending size, then ascending downloads.",
					len(core.Results), countTrusted(core.Results))
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "find-quants for %s (%d results)\n\n", base, len(core.Results))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %-15s  %-10s  %-8s  %s\n", "ID", "UPLOADER", "QUANT", "SIZE", "DOWNLOADS")
			for _, r := range core.Results {
				rep := " "
				if r.UploaderRep {
					rep = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s%-49s  %-15s  %-10s  %-8s  %d\n", rep, r.ID, r.Uploader, r.Quant, r.SizeGB, r.Downloads)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\n  (* = trusted uploader)")
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&preferStr, "prefer", "", "Preferred quants (comma-separated, e.g. iq4_nl,q4_k_m)")
	cmd.Flags().StringVar(&uploadersStr, "uploaders", "", "Restrict to specific uploaders (comma-separated; default: trusted set)")
	cmd.Flags().StringVar(&maxSize, "max-size", "", "Max GGUF size (e.g. 25g, 8gb)")
	return cmd
}

func hasBaseModelTag(m hfModel, baseLower string) bool {
	for _, t := range m.Tags {
		tl := strings.ToLower(t)
		if strings.HasPrefix(tl, "base_model:") && strings.Contains(tl, baseLower) {
			return true
		}
	}
	return false
}

func parseHumanSize(s string) int64 {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "tb"):
		mult = 1 << 40
		s = strings.TrimSuffix(s, "tb")
	case strings.HasSuffix(s, "gb"):
		mult = 1 << 30
		s = strings.TrimSuffix(s, "gb")
	case strings.HasSuffix(s, "mb"):
		mult = 1 << 20
		s = strings.TrimSuffix(s, "mb")
	case strings.HasSuffix(s, "t"):
		mult = 1 << 40
		s = strings.TrimSuffix(s, "t")
	case strings.HasSuffix(s, "g"):
		mult = 1 << 30
		s = strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "m"):
		mult = 1 << 20
		s = strings.TrimSuffix(s, "m")
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int64(f * float64(mult))
}

func sortKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func maxIntInline(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func countTrusted(rs []quantResult) int {
	n := 0
	for _, r := range rs {
		if r.UploaderRep {
			n++
		}
	}
	return n
}
