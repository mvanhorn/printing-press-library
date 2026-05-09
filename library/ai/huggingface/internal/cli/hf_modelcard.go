package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfx"
)

type modelCardResponse struct {
	hfx.Envelope
	ID                string             `json:"id"`
	Author            string             `json:"author"`
	LibraryName       string             `json:"library_name"`
	PipelineTag       string             `json:"pipeline_tag,omitempty"`
	License           string             `json:"license"`
	Downloads         int                `json:"downloads"`
	Likes             int                `json:"likes"`
	LastModified      string             `json:"last_modified"`
	Gated             string             `json:"gated,omitempty"`
	Private           bool               `json:"private,omitempty"`
	Tags              []string           `json:"tags"`
	BaseModel         []string           `json:"base_model,omitempty"`
	ContextLength     int                `json:"context_length,omitempty"`
	ModelType         string             `json:"model_type,omitempty"`
	Architectures     []string           `json:"architectures,omitempty"`
	TotalParams       int                `json:"total_params,omitempty"`
	MoETotalExperts   int                `json:"moe_total_experts,omitempty"`
	MoEActivePerTok   int                `json:"moe_active_per_tok,omitempty"`
	EffectiveGGUFSize string             `json:"effective_gguf_size,omitempty"`
	Siblings          []siblingSummary   `json:"siblings,omitempty"`
	Explain           string             `json:"explain,omitempty"`
}

type siblingSummary struct {
	Path  string `json:"path"`
	Size  int64  `json:"size_bytes"`
	Quant string `json:"quant,omitempty"`
}

func newHFModelCardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model-card <id>",
		Short: "Fetch a stack-relevant model card (MoE active params, effective GGUF size).",
		Long: `model-card fetches the HF model card and surfaces stack-relevant fields:
MoE total/active experts (config.json num_experts, num_experts_per_tok),
effective GGUF size from siblings, base model, license, last modified.`,
		Example: `  huggingface-pp-cli model-card Qwen/Qwen2.5-7B-Instruct
  huggingface-pp-cli model-card bartowski/Qwen2.5-7B-Instruct-GGUF --json
  huggingface-pp-cli model-card Qwen/Qwen3-MoE-A14B-Instruct --explain`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			ctx := cmd.Context()

			model, status, err := hfFetchModel(ctx, id, hfTokenForRequests())
			if err != nil {
				if status == 429 {
					return hfRateLimited("%s", err.Error())
				}
				if status == 404 {
					return hfNotFound("%s", err.Error())
				}
				return err
			}

			resp := modelCardResponse{Envelope: hfx.NewEnvelope("model-card")}
			resp.ID = model.ID
			resp.Author = model.Author
			resp.LibraryName = model.LibraryName
			resp.PipelineTag = model.PipelineTag
			resp.License = model.CardData.License
			resp.Downloads = model.Downloads
			resp.Likes = model.Likes
			resp.LastModified = model.LastModified
			resp.Private = model.Private
			resp.Tags = hfMaxStrings(model.Tags, 30)
			resp.BaseModel = hfBaseModelStrings(model.CardData.BaseModel)
			resp.ContextLength = model.CardData.ContextLength

			// gated: can be string ("auto"|"manual") or bool
			if len(model.Gated) > 0 {
				resp.Gated = strings.Trim(string(model.Gated), `"`)
			}

			// Decode config.json (already nested in card response when present)
			if len(model.Config) > 0 {
				if mt, ok := model.Config["model_type"].(string); ok {
					resp.ModelType = mt
				}
				if archs, ok := model.Config["architectures"].([]any); ok {
					for _, a := range archs {
						if s, ok := a.(string); ok {
							resp.Architectures = append(resp.Architectures, s)
						}
					}
				}
				// MoE
				cfg, _ := decodeConfigFromMap(model.Config)
				total, active := hfx.MoEActiveParams(cfg)
				resp.MoETotalExperts = total
				resp.MoEActivePerTok = active
				if resp.ContextLength == 0 && cfg.MaxPositionEmbeddings > 0 {
					resp.ContextLength = cfg.MaxPositionEmbeddings
				}
			}

			// Sibling summary + effective GGUF size
			var totalGGUF int64
			for _, s := range model.Siblings {
				size := s.Size
				if size == 0 && s.LFS != nil {
					size = s.LFS.Size
				}
				summ := siblingSummary{Path: s.Path, Size: size}
				if hfx.IsGGUF(s.Path) {
					if q, ok := hfx.DetectQuant(s.Path); ok {
						summ.Quant = q.Code
					}
					totalGGUF += size
				}
				resp.Siblings = append(resp.Siblings, summ)
			}
			// Prefer gguf.total when present (seed: cheapest size source)
			if g := model.GGUF; g != nil {
				if total, ok := g["total"].(float64); ok && total > 0 {
					resp.EffectiveGGUFSize = hfHumanGB(int64(total))
				}
			}
			if resp.EffectiveGGUFSize == "" && totalGGUF > 0 {
				resp.EffectiveGGUFSize = hfHumanGB(totalGGUF)
			}

			if flags.explain {
				resp.Explain = explainModelCard(resp)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n", resp.ID, resp.LibraryName)
			fmt.Fprintf(cmd.OutOrStdout(), "  License:    %s\n", resp.License)
			fmt.Fprintf(cmd.OutOrStdout(), "  Modified:   %s\n", resp.LastModified)
			fmt.Fprintf(cmd.OutOrStdout(), "  Downloads:  %d   Likes: %d\n", resp.Downloads, resp.Likes)
			if resp.Gated != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Gated:      %s\n", resp.Gated)
			}
			if len(resp.BaseModel) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Base:       %s\n", strings.Join(resp.BaseModel, ", "))
			}
			if resp.ModelType != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Arch:       %s   Type: %s\n", strings.Join(resp.Architectures, ","), resp.ModelType)
			}
			if resp.MoETotalExperts > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  MoE:        %d experts, %d active/tok\n", resp.MoETotalExperts, resp.MoEActivePerTok)
			}
			if resp.ContextLength > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Context:    %d tokens\n", resp.ContextLength)
			}
			if resp.EffectiveGGUFSize != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  GGUF size:  %s (%d siblings)\n", resp.EffectiveGGUFSize, ggufSiblingCount(resp.Siblings))
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	return cmd
}

func decodeConfigFromMap(cfg map[string]any) (hfx.ConfigJSON, error) {
	// JSON round-trip via cli.encoder is the safest way to handle the typed
	// fields hfx.ConfigJSON declares without rewriting the parser.
	if cfg == nil {
		return hfx.ConfigJSON{}, nil
	}
	type bridge = hfx.ConfigJSON
	var b bridge
	if v, ok := cfg["model_type"].(string); ok {
		b.ModelType = v
	}
	if v, ok := cfg["num_experts"].(float64); ok {
		b.NumExperts = int(v)
	}
	if v, ok := cfg["num_local_experts"].(float64); ok {
		b.NumLocalExperts = int(v)
	}
	if v, ok := cfg["num_experts_per_tok"].(float64); ok {
		b.NumExpertsPerTok = int(v)
	}
	if v, ok := cfg["max_position_embeddings"].(float64); ok {
		b.MaxPositionEmbeddings = int(v)
	}
	if v, ok := cfg["num_attention_heads"].(float64); ok {
		b.NumAttentionHeads = int(v)
	}
	if v, ok := cfg["num_key_value_heads"].(float64); ok {
		b.NumKeyValueHeads = int(v)
	}
	if v, ok := cfg["sliding_window"].(float64); ok {
		b.SlidingWindow = int(v)
	}
	if v, ok := cfg["num_nextn_predict_layers"].(float64); ok {
		b.NumNextnPredictLayers = int(v)
	}
	if v, ok := cfg["multi_token_prediction_layers"].(float64); ok {
		b.MultiTokenPredictionLayers = int(v)
	}
	if v, ok := cfg["qk_nope_head_dim"].(float64); ok {
		b.MLAQHeadDim = int(v)
	}
	if v, ok := cfg["v_head_dim"].(float64); ok {
		b.MLAVHeadDim = int(v)
	}
	if v, ok := cfg["rope_scaling"].(map[string]any); ok {
		b.RopeScaling = v
	}
	return b, nil
}

func ggufSiblingCount(s []siblingSummary) int {
	n := 0
	for _, x := range s {
		if x.Quant != "" {
			n++
		}
	}
	return n
}

func explainModelCard(r modelCardResponse) string {
	var b strings.Builder
	b.WriteString("explain: ")
	if r.MoETotalExperts > 0 {
		b.WriteString(fmt.Sprintf("MoE model — %d experts, %d active per token; multiply active-params by %d for inference cost. ",
			r.MoETotalExperts, r.MoEActivePerTok, r.MoEActivePerTok))
	}
	if r.EffectiveGGUFSize != "" {
		b.WriteString(fmt.Sprintf("Repo ships GGUF artifacts totaling %s. ", r.EffectiveGGUFSize))
	}
	if r.Gated != "" {
		b.WriteString(fmt.Sprintf("Gated (%s) — set HF_TOKEN with accepted ToS or expect 401/403. ", r.Gated))
	}
	if len(r.BaseModel) > 0 {
		b.WriteString(fmt.Sprintf("Base: %s. Use `hf find-quants` on the base for quantized variants. ", strings.Join(r.BaseModel, ", ")))
	}
	return strings.TrimSpace(b.String())
}

// (cliExitf removed — replaced by hfNotFound / hfRateLimited / hfBackendUnsupported /
// hfConfigMissing / hfAlreadyCached in hf_errors.go, which return a real
// *cliError that ExitCode() unwraps to the seed code numbers.)
