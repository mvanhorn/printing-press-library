package hfx

import (
	"encoding/json"
	"strings"
)

// FeatureClassification represents the result of architecture-feature detection.
type FeatureClassification struct {
	Feature    string `json:"feature"`
	Detected   bool   `json:"detected"`
	Variant    string `json:"variant,omitempty"`    // e.g. "training-only", "speculative", "inference-ready"
	Evidence   string `json:"evidence,omitempty"`   // config.json field/value, or card-text excerpt
	Confidence string `json:"confidence,omitempty"` // "high" (config.json), "medium" (heuristic), "low" (card-text only)
}

// ConfigJSON is a partial decode of a HuggingFace config.json — only the
// fields we use for arch-feature detection. Unknown fields are kept in Extra
// for ad-hoc inspection.
type ConfigJSON struct {
	ModelType                  string         `json:"model_type"`
	Architectures              []string       `json:"architectures"`
	HiddenSize                 int            `json:"hidden_size"`
	NumHiddenLayers            int            `json:"num_hidden_layers"`
	NumAttentionHeads          int            `json:"num_attention_heads"`
	NumKeyValueHeads           int            `json:"num_key_value_heads"`
	IntermediateSize           int            `json:"intermediate_size"`
	MaxPositionEmbeddings      int            `json:"max_position_embeddings"`
	NumExperts                 int            `json:"num_experts"`
	NumExpertsPerTok           int            `json:"num_experts_per_tok"`
	NumLocalExperts            int            `json:"num_local_experts"`
	SlidingWindow              int            `json:"sliding_window"`
	UseSlidingWindow           *bool          `json:"use_sliding_window"`
	RopeScaling                map[string]any `json:"rope_scaling"`
	NumNextnPredictLayers      int            `json:"num_nextn_predict_layers"`      // Qwen MTP
	MultiTokenPredictionLayers int            `json:"multi_token_prediction_layers"` // DeepSeek MTP
	MLAQHeadDim                int            `json:"qk_nope_head_dim"`              // DeepSeek MLA signature
	MLAVHeadDim                int            `json:"v_head_dim"`                    // DeepSeek MLA signature
	Extra                      map[string]any `json:"-"`
}

// ParseConfigJSON decodes a config.json blob. Returns a partial classification
// on parse error so callers always get something to work with.
func ParseConfigJSON(raw []byte) (ConfigJSON, error) {
	var c ConfigJSON
	if err := json.Unmarshal(raw, &c); err != nil {
		return ConfigJSON{}, err
	}
	return c, nil
}

// ClassifyFeature applies heuristics to a parsed config.json and (optional)
// model-card README text and returns whether the named feature is present.
//
// Recognized features (canonicalized to lowercase):
//   - "moe" — mixture-of-experts (num_experts > 0)
//   - "mtp" — multi-token prediction (Qwen/DeepSeek variants)
//   - "mla" — multi-head latent attention (DeepSeek)
//   - "gqa" — grouped-query attention (num_kv_heads < num_attention_heads)
//   - "sliding-window-attn" — sliding-window attention
//   - "rope-yarn" — YaRN/long-context rope scaling
//   - "dense" — explicitly NOT MoE
func ClassifyFeature(feature string, cfg ConfigJSON, cardText string) FeatureClassification {
	f := strings.ToLower(strings.TrimSpace(feature))
	cls := FeatureClassification{Feature: f}
	cardTextLower := strings.ToLower(cardText)

	switch f {
	case "moe":
		if cfg.NumExperts > 0 || cfg.NumLocalExperts > 0 {
			cls.Detected = true
			cls.Confidence = "high"
			cls.Evidence = "config.json num_experts > 0"
			if cfg.NumExpertsPerTok > 0 {
				cls.Variant = "active-per-tok-" + intStr(cfg.NumExpertsPerTok) + "-of-" + intStr(maxInt(cfg.NumExperts, cfg.NumLocalExperts))
			}
		} else if strings.Contains(cardTextLower, "mixture of experts") || strings.Contains(cardTextLower, "moe") {
			cls.Detected = true
			cls.Confidence = "low"
			cls.Evidence = "card-text mention only"
		}
	case "mtp":
		if cfg.NumNextnPredictLayers > 0 {
			cls.Detected = true
			cls.Confidence = "high"
			cls.Evidence = "config.json num_nextn_predict_layers > 0 (Qwen MTP)"
			cls.Variant = "qwen-nextn"
		} else if cfg.MultiTokenPredictionLayers > 0 {
			cls.Detected = true
			cls.Confidence = "high"
			cls.Evidence = "config.json multi_token_prediction_layers > 0 (DeepSeek MTP)"
			cls.Variant = "deepseek-mtp"
		} else if strings.Contains(cardTextLower, "multi-token prediction") || strings.Contains(cardTextLower, "mtp") {
			cls.Detected = true
			cls.Confidence = "low"
			cls.Evidence = "card-text mention only"
			cls.Variant = "training-only-or-unclear"
		}
	case "mla":
		if cfg.MLAQHeadDim > 0 && cfg.MLAVHeadDim > 0 {
			cls.Detected = true
			cls.Confidence = "high"
			cls.Evidence = "config.json qk_nope_head_dim + v_head_dim (DeepSeek MLA signature)"
		} else if strings.Contains(cardTextLower, "multi-head latent attention") {
			cls.Detected = true
			cls.Confidence = "low"
			cls.Evidence = "card-text mention only"
		}
	case "gqa":
		if cfg.NumKeyValueHeads > 0 && cfg.NumAttentionHeads > 0 && cfg.NumKeyValueHeads < cfg.NumAttentionHeads {
			cls.Detected = true
			cls.Confidence = "high"
			cls.Evidence = "config.json num_key_value_heads (" + intStr(cfg.NumKeyValueHeads) + ") < num_attention_heads (" + intStr(cfg.NumAttentionHeads) + ")"
		}
	case "sliding-window-attn", "sliding-window", "swa":
		if cfg.SlidingWindow > 0 && (cfg.UseSlidingWindow == nil || *cfg.UseSlidingWindow) {
			cls.Detected = true
			cls.Confidence = "high"
			cls.Evidence = "config.json sliding_window=" + intStr(cfg.SlidingWindow)
		}
	case "rope-yarn", "yarn":
		if cfg.RopeScaling != nil {
			if t, ok := cfg.RopeScaling["type"].(string); ok && strings.EqualFold(t, "yarn") {
				cls.Detected = true
				cls.Confidence = "high"
				cls.Evidence = "config.json rope_scaling.type=yarn"
			} else if t, ok := cfg.RopeScaling["rope_type"].(string); ok && strings.EqualFold(t, "yarn") {
				cls.Detected = true
				cls.Confidence = "high"
				cls.Evidence = "config.json rope_scaling.rope_type=yarn"
			}
		}
	case "dense":
		// Dense = NOT MoE. Only assert if we can confirm MoE absent from config.
		if cfg.NumExperts == 0 && cfg.NumLocalExperts == 0 {
			cls.Detected = true
			cls.Confidence = "medium"
			cls.Evidence = "config.json num_experts == 0"
		}
	}
	return cls
}

// MoEActiveParams returns (totalParams, activeParams) estimates for an MoE
// model, given its config.json. Returns (0, 0) for non-MoE or insufficient info.
// The seed makes "active params loud for MoE" a product requirement; this
// helper is the canonical computation.
func MoEActiveParams(cfg ConfigJSON) (totalExperts, activePerTok int) {
	exp := cfg.NumExperts
	if cfg.NumLocalExperts > exp {
		exp = cfg.NumLocalExperts
	}
	return exp, cfg.NumExpertsPerTok
}

func intStr(n int) string {
	// Avoid pulling in strconv just for this; we already import elsewhere.
	// Uses the strconv-equivalent via fmt-free path.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
