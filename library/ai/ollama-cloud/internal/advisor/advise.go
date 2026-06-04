package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

func Advise(ctx context.Context, req Request, catalog []Model, includeExplain bool) (*Recommendation, error) {
	if len(catalog) == 0 {
		return nil, errors.New("advisor: empty catalog; run `tags --no-cache` or check OLLAMA_CLOUD_API_KEY")
	}
	feats := ExtractFeatures(req.Prompt, req.Session)
	if req.ExpectedOutputTokens <= 0 {
		req.ExpectedOutputTokens = 1024
	}

	excluded := make(map[string]bool, len(req.Exclude))
	for _, e := range req.Exclude {
		excluded[e] = true
	}

	cands := make([]Candidate, 0, len(catalog))
	for _, m := range catalog {
		c := scoreModel(m, feats, req)
		if excluded[m.ID] {
			c.Filtered = true
			c.FilterReason = "excluded by --exclude"
		}
		cands = append(cands, c)
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Filtered != cands[j].Filtered {
			return !cands[i].Filtered
		}
		if cands[i].Score != cands[j].Score {
			return cands[i].Score > cands[j].Score
		}
		return cands[i].ModelID < cands[j].ModelID
	})

	var live, filtered []Candidate
	for _, c := range cands {
		if c.Filtered {
			filtered = append(filtered, c)
		} else {
			live = append(live, c)
		}
	}
	if len(live) == 0 {
		return nil, fmt.Errorf("advisor: no candidates after filtering (%d excluded, %d filtered); relax --exclude / --require-tools / --max-latency-ms", len(req.Exclude), len(filtered))
	}

	rec := &Recommendation{SchemaVersion: SchemaVersion, AdvisedAt: time.Now().UTC()}
	picked := live[0]

	if len(live) >= 2 && req.EnableTiebreak && req.Tiebreaker != nil {
		gap := picked.Score - live[1].Score
		if picked.Score > 0 && gap/picked.Score < 0.05 {
			rec.TiebreakAttempted = true
			top := live[:2]
			id, terr := req.Tiebreaker(ctx, req.Prompt, top)
			if terr != nil {
				rec.TiebreakError = terr.Error()
			} else {
				for _, c := range top {
					if c.ModelID == id {
						picked = c
						rec.TiebreakUsed = true
						break
					}
				}
			}
		}
	}

	rec.Recommended = picked.Model.QualifiedID()
	rec.Why = picked.Why
	if len(live) > 1 {
		alts := make([]Candidate, 0, 3)
		for _, c := range live {
			if c.ModelID == picked.ModelID {
				continue
			}
			altCopy := c
			altCopy.ModelID = c.Model.QualifiedID()
			alts = append(alts, altCopy)
			if len(alts) >= 3 {
				break
			}
		}
		rec.Alternatives = alts
		// Fallback must differ from the recommendation so a routing layer that
		// retries against it has an actual escape path. live[1] is wrong when the
		// tiebreaker promotes it to the winner, so pick the first live candidate
		// that isn't the recommended model.
		for _, c := range live {
			if c.Model.QualifiedID() != rec.Recommended {
				rec.Fallback = c.Model.QualifiedID()
				break
			}
		}
	}
	rec.EstInputTokens = feats.InputTokens
	rec.EstOutputTokens = req.ExpectedOutputTokens
	rec.EstCostUSD = estimateCostUSD(picked.Model, feats.InputTokens, req.ExpectedOutputTokens)
	rec.EstLatencyMs = picked.Model.LatencyP50Ms

	if includeExplain {
		rec.Features = &feats
		rec.Filtered = filtered
	}
	return rec, nil
}

// LoadProviderOverlay reads a sibling-provider overlay (no live catalog needed
// for providers like local-llama where the curated list IS the catalog) and
// returns Model entries stamped with the overlay's "provider" field. The first
// id_pattern in each entry that doesn't contain a wildcard becomes the canonical
// model id; wildcard-only patterns are skipped (no concrete id to surface).
func LoadProviderOverlay(modelsJSON []byte) ([]Model, error) {
	meta, err := parseOverlay(modelsJSON)
	if err != nil {
		return nil, err
	}
	// Provider name is required for sibling overlays so QualifiedID can stamp it.
	var raw struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(modelsJSON, &raw); err != nil {
		return nil, fmt.Errorf("advisor: parsing provider overlay top-level: %w", err)
	}
	if raw.Provider == "" {
		return nil, fmt.Errorf("advisor: provider-overlay missing required `provider` field")
	}

	out := make([]Model, 0, len(meta.Models))
	for _, mm := range meta.Models {
		concrete := firstConcretePattern(mm.IDPatterns)
		if concrete == "" {
			continue
		}
		m := Model{
			ID:             concrete,
			Provider:       raw.Provider,
			Family:         mm.Family,
			CtxWindow:      mm.CtxWindow,
			PriceInPer1M:   mm.PriceInPer1M,
			PriceOutPer1M:  mm.PriceOutPer1M,
			LatencyP50Ms:   mm.LatencyP50Ms,
			SupportsTools:  mm.SupportsTools,
			SupportsVision: mm.SupportsVision,
			Strengths:      append([]string{}, mm.Strengths...),
			Source:         "overlay:" + raw.Provider,
		}
		out = append(out, m)
	}
	return out, nil
}

func firstConcretePattern(patterns []string) string {
	for _, p := range patterns {
		// Strip a single trailing "*" if present (most-common pattern shape)
		if len(p) > 0 && p[len(p)-1] == '*' {
			candidate := p[:len(p)-1]
			if candidate != "" && !containsWildcard(candidate) {
				return candidate
			}
			continue
		}
		if !containsWildcard(p) {
			return p
		}
	}
	return ""
}

func containsWildcard(s string) bool {
	for _, r := range s {
		if r == '*' || r == '?' {
			return true
		}
	}
	return false
}

func LoadCatalog(tagsJSON json.RawMessage, modelsJSON []byte) ([]Model, error) {
	var tags struct {
		Models []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Details struct {
				Family string `json:"family"`
			} `json:"details"`
		} `json:"models"`
	}
	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &tags); err != nil {
			return nil, fmt.Errorf("advisor: parsing /api/tags: %w", err)
		}
	}

	meta, err := parseOverlay(modelsJSON)
	if err != nil {
		return nil, err
	}

	out := make([]Model, 0, len(tags.Models))
	for _, t := range tags.Models {
		id := t.Name
		if id == "" {
			id = t.Model
		}
		if id == "" {
			continue
		}
		m := Model{ID: id, Family: t.Details.Family, Source: "live"}
		applyOverlay(&m, meta)
		out = append(out, m)
	}
	return out, nil
}

func ValidateCatalog(tagsJSON json.RawMessage, modelsJSON []byte) (*CatalogDrift, error) {
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(tagsJSON, &tags); err != nil {
		return nil, err
	}
	meta, err := parseOverlay(modelsJSON)
	if err != nil {
		return nil, err
	}

	out := &CatalogDrift{GeneratedAt: time.Now().UTC()}
	patternHits := make(map[string]int, len(meta.Models)*2)
	for _, m := range tags.Models {
		matched := false
		for _, mm := range meta.Models {
			for _, p := range mm.IDPatterns {
				if globMatch(p, m.Name) {
					matched = true
					patternHits[p]++
				}
			}
		}
		if !matched {
			out.UncuratedLive = append(out.UncuratedLive, m.Name)
		}
	}
	for _, mm := range meta.Models {
		for _, p := range mm.IDPatterns {
			if patternHits[p] == 0 {
				out.CuratedNotInLive = append(out.CuratedNotInLive, p)
			}
		}
	}
	sort.Strings(out.UncuratedLive)
	sort.Strings(out.CuratedNotInLive)
	return out, nil
}
