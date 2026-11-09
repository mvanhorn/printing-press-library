// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestMCPOutputRecognizesCompactionMetadata(t *testing.T) {
	session := mcpOutputTestSession(t)
	observeMCPOutput(session, nil, map[string]any{
		"records":            []any{map[string]any{"id": "one"}},
		"_pp_truncated":      true,
		"_pp_original_bytes": float64(4096),
		"_pp_max_bytes":      float64(1024),
	})
	metadata := session.OutputMetadata()
	if !metadata.Truncated || len(metadata.TruncationReasons) != 1 {
		t.Fatalf("metadata = %+v", metadata)
	}
	reason := metadata.TruncationReasons[0]
	if reason.Kind != "mcp_compaction" || reason.Configured != 1024 || reason.Observed != 4096 || !reason.MoreAvailable {
		t.Fatalf("compaction reason = %+v", reason)
	}
}

func mcpOutputTestSession(t *testing.T) *platform.Session {
	t.Helper()
	profile := &platform.Profile{
		SchemaVersion: platform.ProfileSchemaVersion,
		Name:          "tenant-a",
		Sources: map[string]platform.SourceProfile{
			"test-source": {
				CredentialRef:     "op://Test Vault/Test Item/token",
				ExpectedAccountID: "acct-a",
			},
		},
	}
	session, err := platform.PrepareProfileSourceFromConfig(profile, "fixture-pp-cli", "test-source")
	if err != nil {
		t.Fatal(err)
	}
	return session
}
