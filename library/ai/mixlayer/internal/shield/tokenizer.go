// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package shield

import (
	"context"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/store"
)

type RedactionResult struct {
	Text     string         `json:"text"`
	Entities []MaskedEntity `json:"entities"`
	Risk     int            `json:"risk"`
}

type MaskedEntity struct {
	Kind  string `json:"kind"`
	Value string `json:"value,omitempty"`
	Token string `json:"token"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Risk  int    `json:"risk"`
}

func Redact(ctx context.Context, s *store.Store, text string, reveal bool) (RedactionResult, error) {
	entities := Detect(text)
	masked := make([]MaskedEntity, 0, len(entities))
	for i, e := range entities {
		entry, err := s.EnsureVaultEntry(ctx, e.Kind, e.Value)
		if err != nil {
			return RedactionResult{}, err
		}
		value := ""
		if reveal {
			value = e.Value
		}
		masked = append(masked, MaskedEntity{Kind: e.Kind, Value: value, Token: entry.Token, Start: e.Start, End: e.End, Risk: e.Risk})
		entities[i].Value = entry.Token
	}
	out := text
	sort.Slice(entities, func(i, j int) bool { return entities[i].Start > entities[j].Start })
	for _, e := range entities {
		out = out[:e.Start] + e.Value + out[e.End:]
	}
	return RedactionResult{Text: out, Entities: masked, Risk: RiskScore(entities)}, nil
}

func Rehydrate(ctx context.Context, s *store.Store, text string) (string, error) {
	mapping, err := s.VaultTokenMap(ctx)
	if err != nil {
		return "", err
	}
	out := text
	tokens := make([]string, 0, len(mapping))
	for token := range mapping {
		tokens = append(tokens, token)
	}
	sort.Slice(tokens, func(i, j int) bool { return len(tokens[i]) > len(tokens[j]) })
	for _, token := range tokens {
		out = strings.ReplaceAll(out, token, mapping[token])
	}
	return out, nil
}
