// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/mixapi"
	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/pricing"
	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/store"
	"github.com/spf13/cobra"
)

const (
	defaultFrontierModel = "qwen/qwen3.5-397b-a17b"
	defaultGuardModel    = "qwen/qwen3.5-4b-free"
)

func openMixStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("mixlayer-pp-cli")
	}
	return store.OpenWithContext(ctx, dbPath)
}

func outputJSON(cmd *cobra.Command, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return printOutput(cmd.OutOrStdout(), raw, true)
}

func readTextArg(pathOrText string) (string, error) {
	if pathOrText == "-" {
		raw, err := os.ReadFile("/dev/stdin")
		return string(raw), err
	}
	if raw, err := os.ReadFile(pathOrText); err == nil {
		return string(raw), nil
	}
	return pathOrText, nil
}

func readRequiredFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("file path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func chatAndSave(ctx context.Context, flags *rootFlags, s *store.Store, command, prompt, model string, seed int64, reasoning bool) (store.RunRecord, error) {
	c, err := flags.newClient()
	if err != nil {
		return store.RunRecord{}, err
	}
	thinking := reasoning
	var seedPtr *int64
	if seed != 0 {
		seedPtr = &seed
	}
	res, err := mixapi.Chat(ctx, c, mixapi.ChatRequest{
		Model:    model,
		Messages: []mixapi.Message{{Role: "user", Content: prompt}},
		Thinking: &thinking,
		Seed:     seedPtr,
	})
	if err != nil {
		return store.RunRecord{}, err
	}
	params, _ := json.Marshal(map[string]any{"model": model, "seed": seed, "reasoning": reasoning})
	run := store.RunRecord{
		ID:               store.NewID("run"),
		Command:          command,
		Prompt:           prompt,
		Answer:           res.Answer,
		Reasoning:        res.Reasoning,
		Model:            model,
		Seed:             seed,
		ParamsJSON:       params,
		RawJSON:          res.Raw,
		PromptTokens:     res.PromptTokens,
		CompletionTokens: res.CompletionTokens,
		TotalTokens:      res.TotalTokens,
		CostUSD:          pricing.Estimate(model, res.PromptTokens, res.CompletionTokens),
		LatencyMS:        res.LatencyMS,
	}
	if err := s.SaveRun(ctx, run); err != nil {
		return store.RunRecord{}, err
	}
	return run, nil
}
