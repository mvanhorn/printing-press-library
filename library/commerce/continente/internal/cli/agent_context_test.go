package cli

import "testing"

func TestCompactAgentContextPayload(t *testing.T) {
	t.Parallel()

	payload := compactAgentContextPayload(agentContext{
		SchemaVersion: "4",
		CLI: agentContextCLI{
			Name:        "continente-pp-cli",
			Version:     "1.0.0",
			Description: "Storefront product and cart workflows",
		},
		Auth: agentContextAuth{
			Mode: "direct",
		},
		Commands: []agentContextCommand{
			{
				Name:  "checkout",
				Short: "Inspect checkout",
				Subcommands: []agentContextCommand{{
					Name:        "slots",
					Short:       "List slots",
					Annotations: map[string]string{"mcp:read-only": "true"},
				}},
			},
		},
		AvailableProfiles: []string{"briefing"},
		PreferredStore:    &agentContextStore{ID: "439", Name: "Continente Mafra"},
	})

	if payload.Auth.Mode != "direct" {
		t.Fatalf("payload.Auth.Mode = %q, want direct", payload.Auth.Mode)
	}
	if payload.OutputProfile.MetaMode != "minimal" {
		t.Fatalf("payload.OutputProfile.MetaMode = %q, want minimal", payload.OutputProfile.MetaMode)
	}
	if len(payload.Commands) != 1 || payload.Commands[0].Name != "checkout" {
		t.Fatalf("unexpected compact commands: %#v", payload.Commands)
	}
	if len(payload.Commands[0].Subcommands) != 1 || !payload.Commands[0].Subcommands[0].ReadOnly {
		t.Fatalf("unexpected compact subcommands: %#v", payload.Commands[0].Subcommands)
	}
	if len(payload.Auth.SessionModes) == 0 {
		t.Fatalf("expected session modes in compact auth payload")
	}
}
