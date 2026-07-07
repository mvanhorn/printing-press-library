// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/store"
	"github.com/spf13/cobra"
)

func TestPrepareShieldAskPayloadRedactsQuestionPII(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	prepared, err := prepareShieldAskPayload(
		ctx,
		s,
		"Alice Johnson bought from alice@example.com.",
		"What is the risk for John Smith with SSN 123-45-6789?",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"Alice Johnson", "alice@example.com", "John Smith", "123-45-6789"} {
		if strings.Contains(prepared.Payload, raw) {
			t.Fatalf("payload still contains raw PII %q: %s", raw, prepared.Payload)
		}
	}
	if len(prepared.Leaks) != 0 {
		t.Fatalf("payload leaks = %#v, want none", prepared.Leaks)
	}
	if prepared.MaskedEntities < 4 {
		t.Fatalf("masked entities = %d, want at least 4", prepared.MaskedEntities)
	}
}

func TestWriteShieldIngestResultPrintsCorpusWithoutOutput(t *testing.T) {
	var out bytes.Buffer
	cmd := newTestOutputCmd(&out)
	if err := writeShieldIngestResult(cmd, &rootFlags{}, "", "masked corpus", map[string]any{"tranches": 1, "output": ""}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "masked corpus" {
		t.Fatalf("stdout = %q, want masked corpus", out.String())
	}
}

func TestWriteShieldIngestResultIncludesCorpusInJSONMode(t *testing.T) {
	var out bytes.Buffer
	cmd := newTestOutputCmd(&out)
	if err := writeShieldIngestResult(cmd, &rootFlags{asJSON: true}, "", "masked corpus", map[string]any{"tranches": 1, "output": ""}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["masked_corpus"] != "masked corpus" {
		t.Fatalf("masked_corpus = %#v, want masked corpus", got["masked_corpus"])
	}
}

func TestShieldAskRunOutputRehydratesAnswerAndReasoning(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SaveVaultEntry(ctx, store.VaultEntry{Token: "EMAIL_3f9a4abc", Value: "alice@example.com", Kind: "EMAIL"}); err != nil {
		t.Fatal(err)
	}
	run := store.RunRecord{Answer: "Email EMAIL_3f9a4abc", Reasoning: "Saw EMAIL_3f9a4abc"}
	got, err := rehydrateRunForOutput(ctx, s, run, "Email alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Answer != "Email alice@example.com" || got.Reasoning != "Saw alice@example.com" {
		t.Fatalf("rehydrated run = %#v", got)
	}
}

func TestShieldScanMaxRiskHelpClarifiesPerEntityMax(t *testing.T) {
	cmd := newShieldScanCmd(&rootFlags{})
	flag := cmd.Flags().Lookup("max-risk")
	if flag == nil || !strings.Contains(flag.Usage, "per-entity max risk") || !strings.Contains(flag.Usage, "not volume-weighted") {
		t.Fatalf("max-risk usage = %q", flag.Usage)
	}
}

func newTestOutputCmd(out *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	return cmd
}
