package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

type compareQuantsResponse struct {
	hfx.Envelope
	Compared []compareQuantsRow `json:"compared"`
	Errors   []compareQuantsErr `json:"errors,omitempty"`
	Explain  string             `json:"explain,omitempty"`
}

type compareQuantsRow struct {
	ID               string               `json:"id"`
	Author           string               `json:"author"`
	UploaderRep      bool                 `json:"uploader_rep"`
	License          string               `json:"license"`
	Downloads        int                  `json:"downloads"`
	LastModified     string               `json:"last_modified"`
	GGUFTotalSize    int64                `json:"gguf_total_size_bytes,omitempty"`
	GGUFTotalSizeGB  string               `json:"gguf_total_size_gb,omitempty"`
	Quants           []compareQuantsQuant `json:"quants,omitempty"`
	MoEActiveExperts int                  `json:"moe_active_per_tok,omitempty"`
	MoETotalExperts  int                  `json:"moe_total_experts,omitempty"`
	BaseModel        []string             `json:"base_model,omitempty"`
}

type compareQuantsQuant struct {
	Code   string `json:"code"`
	Family string `json:"family"`
	Path   string `json:"path"`
	Size   int64  `json:"size_bytes,omitempty"`
}

type compareQuantsErr struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

func newHFCompareQuantsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare-quants <id1> <id2> [<idN>...]",
		Short: "Side-by-side comparison of multiple quant variants.",
		Long: `compare-quants fetches model cards for N HF ids in parallel and renders
side-by-side: total GGUF size, per-quant breakdown, license, downloads,
MoE active-params (when applicable), trusted-uploader badge, base model.`,
		Example: `  huggingface-pp-cli compare-quants \
    bartowski/Qwen2.5-7B-Instruct-GGUF \
    mradermacher/Qwen2.5-7B-Instruct-GGUF \
    unsloth/Qwen2.5-7B-Instruct-GGUF
  huggingface-pp-cli compare-quants <id1> <id2> --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			rows := make([]compareQuantsRow, len(args))
			errs := make([]compareQuantsErr, 0)
			var mu sync.Mutex
			var wg sync.WaitGroup
			sem := make(chan struct{}, 6)

			for i, id := range args {
				wg.Add(1)
				sem <- struct{}{}
				go func(i int, id string) {
					defer wg.Done()
					defer func() { <-sem }()
					row, err := fetchCompareRow(ctx, id)
					if err != nil {
						mu.Lock()
						errs = append(errs, compareQuantsErr{ID: id, Error: err.Error()})
						mu.Unlock()
						return
					}
					rows[i] = row
				}(i, id)
			}
			wg.Wait()

			// Filter zero rows (errors); preserve user order
			compacted := make([]compareQuantsRow, 0, len(rows))
			for _, r := range rows {
				if r.ID != "" {
					compacted = append(compacted, r)
				}
			}
			if len(compacted) == 0 {
				return hfNotFound("none of the requested ids could be fetched (see errors[] in JSON for details)")
			}

			resp := compareQuantsResponse{
				Envelope: hfx.NewEnvelope("compare-quants"),
				Compared: compacted,
				Errors:   errs,
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: parallel-fetched %d ids, %d succeeded. Sorted by user-provided order; quants per row sorted by size ascending.",
					len(args), len(compacted))
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "compare-quants (%d ids)\n\n", len(compacted))
			for _, r := range compacted {
				star := " "
				if r.UploaderRep {
					star = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s  (%s)  %s\n", star, r.ID, r.Author, r.License)
				if r.GGUFTotalSizeGB != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    total GGUF: %s   downloads: %d\n", r.GGUFTotalSizeGB, r.Downloads)
				}
				if r.MoETotalExperts > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "    MoE: %d experts, %d active/tok\n", r.MoETotalExperts, r.MoEActiveExperts)
				}
				for _, q := range r.Quants {
					fmt.Fprintf(cmd.OutOrStdout(), "    %-12s  %s\n", q.Code, hfHumanGB(q.Size))
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if len(errs) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Errors:")
				for _, e := range errs {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", e.ID, e.Error)
				}
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	return cmd
}

func fetchCompareRow(ctx context.Context, id string) (compareQuantsRow, error) {
	m, status, err := hfFetchModel(ctx, id, hfTokenForRequests())
	if err != nil {
		return compareQuantsRow{}, fmt.Errorf("fetch %s (HTTP %d): %w", id, status, err)
	}
	uploader := m.Author
	if uploader == "" && strings.Contains(m.ID, "/") {
		uploader = strings.SplitN(m.ID, "/", 2)[0]
	}

	// Quants from siblings
	var quants []compareQuantsQuant
	var totalGGUF int64
	for _, s := range m.Siblings {
		if !hfx.IsGGUF(s.Path) {
			continue
		}
		q, ok := hfx.DetectQuant(s.Path)
		if !ok {
			continue
		}
		size := s.Size
		if size == 0 && s.LFS != nil {
			size = s.LFS.Size
		}
		quants = append(quants, compareQuantsQuant{
			Code:   q.Code,
			Family: q.Family,
			Path:   s.Path,
			Size:   size,
		})
		totalGGUF += size
	}
	sort.Slice(quants, func(i, j int) bool { return quants[i].Size < quants[j].Size })

	// Prefer gguf.total when available
	if g := m.GGUF; g != nil {
		if total, ok := g["total"].(float64); ok && total > 0 && totalGGUF == 0 {
			totalGGUF = int64(total)
		}
	}

	// MoE
	cfg, _ := decodeConfigFromMap(m.Config)
	moeTotal, moeActive := hfx.MoEActiveParams(cfg)

	return compareQuantsRow{
		ID:               m.ID,
		Author:           uploader,
		UploaderRep:      hfx.IsTrustedUploader(uploader),
		License:          m.CardData.License,
		Downloads:        m.Downloads,
		LastModified:     m.LastModified,
		GGUFTotalSize:    totalGGUF,
		GGUFTotalSizeGB:  hfHumanGB(totalGGUF),
		Quants:           quants,
		MoETotalExperts:  moeTotal,
		MoEActiveExperts: moeActive,
		BaseModel:        hfBaseModelStrings(m.CardData.BaseModel),
	}, nil
}
