package advisor

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type overlayEntry struct {
	IDPatterns     []string `json:"id_patterns"`
	Family         string   `json:"family"`
	CtxWindow      int      `json:"ctx_window"`
	PriceInPer1M   float64  `json:"price_in_per_1m"`
	PriceOutPer1M  float64  `json:"price_out_per_1m"`
	LatencyP50Ms   int      `json:"latency_p50_ms"`
	SupportsTools  bool     `json:"supports_tools"`
	SupportsVision bool     `json:"supports_vision"`
	Strengths      []string `json:"strengths"`
}

type overlayMeta struct {
	SchemaVersion int            `json:"schema_version"`
	Models        []overlayEntry `json:"models"`
	Default       overlayEntry   `json:"default"`
}

func parseOverlay(modelsJSON []byte) (overlayMeta, error) {
	var meta overlayMeta
	if len(modelsJSON) == 0 {
		return meta, nil
	}
	if err := json.Unmarshal(modelsJSON, &meta); err != nil {
		return meta, fmt.Errorf("advisor: parsing models.json: %w", err)
	}
	if meta.SchemaVersion != 0 && meta.SchemaVersion != SchemaVersion {
		return meta, fmt.Errorf("advisor: models.json schema_version=%d, expected %d", meta.SchemaVersion, SchemaVersion)
	}
	return meta, nil
}

func applyOverlay(m *Model, meta overlayMeta) {
	for _, mm := range meta.Models {
		for _, pat := range mm.IDPatterns {
			if globMatch(pat, m.ID) {
				m.Family = orStr(mm.Family, m.Family)
				m.CtxWindow = mm.CtxWindow
				m.PriceInPer1M = mm.PriceInPer1M
				m.PriceOutPer1M = mm.PriceOutPer1M
				m.LatencyP50Ms = mm.LatencyP50Ms
				m.SupportsTools = mm.SupportsTools
				m.SupportsVision = mm.SupportsVision
				m.Strengths = append([]string{}, mm.Strengths...)
				return
			}
		}
	}
	m.CtxWindow = meta.Default.CtxWindow
	m.PriceInPer1M = meta.Default.PriceInPer1M
	m.PriceOutPer1M = meta.Default.PriceOutPer1M
	m.LatencyP50Ms = meta.Default.LatencyP50Ms
	m.SupportsTools = meta.Default.SupportsTools
	m.SupportsVision = meta.Default.SupportsVision
	m.Strengths = append([]string{}, meta.Default.Strengths...)
}

func globMatch(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if ok, err := filepath.Match(pattern, s); err == nil && ok {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(s, pattern[1:])
	}
	return pattern == s
}

func orStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
