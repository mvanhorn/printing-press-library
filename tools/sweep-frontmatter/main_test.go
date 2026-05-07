package main

import (
	"strings"
	"testing"
)

// Each test case is a real-shaped fragment of legacy frontmatter from
// the live library, paired with the expected post-sweep output. The
// fragments are intentionally minimal — full SKILL.md round-trips are
// covered by the manual dry-run against the live library before commit.

func TestStripFrontmatterLegacyEnvBlocks_FourShapes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			// Mercury shape: single inline env list + envVars block
			name: "single-inline-env-and-envVars",
			in: `name: pp-mercury
metadata:
  openclaw:
    requires:
      env: ["MERCURY_BEARER_AUTH"]
      bins:
        - mercury-pp-cli
    envVars:
      - name: MERCURY_BEARER_AUTH
        required: true
        description: "MERCURY_BEARER_AUTH credential."
    install:
      - kind: go`,
			want: `name: pp-mercury
metadata:
  openclaw:
    requires:
      bins:
        - mercury-pp-cli
    install:
      - kind: go`,
		},
		{
			// Linear shape: bins then block-style env, plus primaryEnv
			name: "block-style-env-and-primaryEnv",
			in: `metadata:
  openclaw:
    requires:
      bins:
        - linear-pp-cli
      env:
        - LINEAR_API_KEY
    primaryEnv: LINEAR_API_KEY
    install:`,
			want: `metadata:
  openclaw:
    requires:
      bins:
        - linear-pp-cli
    install:`,
		},
		{
			// Dominos shape: empty inline env list + multi-entry envVars
			name: "empty-env-and-multi-entry-envVars",
			in: `metadata:
  openclaw:
    requires:
      env: []
      bins:
        - dominos-pp-cli
    envVars:
      - name: DOMINOS_USERNAME
        required: false
        description: "x"
      - name: DOMINOS_PASSWORD
        required: false
        description: "y"
    install:`,
			want: `metadata:
  openclaw:
    requires:
      bins:
        - dominos-pp-cli
    install:`,
		},
		{
			// Already-canonical shape (no legacy declarations) is a no-op
			name: "no-op-on-canonical-shape",
			in: `metadata:
  openclaw:
    requires:
      bins:
        - shopify-pp-cli
    install:`,
			want: `metadata:
  openclaw:
    requires:
      bins:
        - shopify-pp-cli
    install:`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripFrontmatterLegacyEnvBlocks(tc.in)
			if got != tc.want {
				t.Errorf("stripFrontmatterLegacyEnvBlocks(%s) mismatch.\n--- want ---\n%s\n--- got ---\n%s", tc.name, tc.want, got)
			}
		})
	}
}

func TestEnsureFrontmatterTopLevelFields(t *testing.T) {
	ctx := patchSkillCtx{PressVersion: "3.10.0", AuthorName: "Trevin Chow"}

	t.Run("inserts after description when fields absent", func(t *testing.T) {
		in := `name: pp-test
description: "a CLI"
argument-hint: "..."
`
		want := `name: pp-test
description: "a CLI"
version: "3.10.0"
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "..."
`
		if got := ensureFrontmatterTopLevelFields(in, ctx); got != want {
			t.Errorf("\nwant: %q\ngot:  %q", want, got)
		}
	})

	t.Run("idempotent when fields match canonical values", func(t *testing.T) {
		in := `name: pp-test
description: "a CLI"
version: "3.10.0"
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "..."
`
		if got := ensureFrontmatterTopLevelFields(in, ctx); got != in {
			t.Errorf("expected no-op when ctx matches existing values; got: %q", got)
		}
	})

	t.Run("rewrites author when ctx differs (per-CLI map correction)", func(t *testing.T) {
		in := `description: "a CLI"
version: "3.10.0"
author: "Wrong Operator"
license: "Apache-2.0"
`
		want := `description: "a CLI"
version: "3.10.0"
author: "Trevin Chow"
license: "Apache-2.0"
`
		if got := ensureFrontmatterTopLevelFields(in, ctx); got != want {
			t.Errorf("expected author rewritten to ctx value;\nwant: %q\ngot:  %q", want, got)
		}
	})

	t.Run("escapes special characters via fmt %q", func(t *testing.T) {
		ctxQuoted := patchSkillCtx{PressVersion: "3.10.0", AuthorName: `Trevin "Quoted" Chow`}
		in := `description: "a CLI"
`
		got := ensureFrontmatterTopLevelFields(in, ctxQuoted)
		// %q produces a Go-quoted string which is also valid YAML
		// double-quoted scalar — embedded quotes are escaped.
		if !strings.Contains(got, `author: "Trevin \"Quoted\" Chow"`) {
			t.Errorf("special-character escape missing; got: %q", got)
		}
	})
}

func TestPatchSkillPrerequisites_RewritesExistingSection(t *testing.T) {
	// A prior sweep inserted Prerequisites with stale content (e.g., the
	// pre-npx install line). The next sweep must replace it with the
	// canonical content rather than skip — otherwise install-command
	// updates can't propagate across re-sweeps.
	body := `---
name: pp-x
---

# X — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the ` + "`x-pp-cli`" + ` binary. STALE INSTALL CONTENT FROM PREVIOUS SWEEP — should be replaced.

## When to Use

stuff.
`
	ctx := patchSkillCtx{CLIName: "x-pp-cli", APIName: "x", Category: "other"}
	got := patchSkillPrerequisites(body, ctx)

	// Stale content gone, canonical content present.
	if strings.Contains(got, "STALE INSTALL CONTENT") {
		t.Errorf("stale Prerequisites content not removed:\n%s", got)
	}
	if !strings.Contains(got, "npx -y @mvanhorn/printing-press install x --cli-only") {
		t.Errorf("canonical npx install line not present:\n%s", got)
	}
	if strings.Count(got, "## Prerequisites: Install the CLI") != 1 {
		t.Errorf("Prerequisites heading should appear exactly once; got %d", strings.Count(got, "## Prerequisites: Install the CLI"))
	}

	// Idempotency: running a second time with same ctx should produce
	// identical output.
	gotAgain := patchSkillPrerequisites(got, ctx)
	if gotAgain != got {
		t.Errorf("second run should produce zero diff;\ngot diff:\n%s", gotAgain)
	}
}

func TestPatchSkillPrerequisites_MovesExistingCLIInstallation(t *testing.T) {
	body := `---
name: pp-x
---

# X — Printing Press CLI

Stuff.

## Argument Parsing

1. Foo
2. otherwise → CLI installation

## CLI Installation

1. Check Go is installed: ` + "`go version`" + `
2. Install:
   ` + "```bash" + `
   go install github.com/mvanhorn/printing-press-library/library/other/x/cmd/x-pp-cli@latest
   ` + "```" + `

## MCP Server Installation

stuff.

## Direct Use

1. Check if installed.
   If not found, offer to install (see CLI Installation above).
`
	ctx := patchSkillCtx{CLIName: "x-pp-cli", APIName: "x", Category: "other"}
	got := patchSkillPrerequisites(body, ctx)

	// Prerequisites must be present near the top.
	prereqIdx := strings.Index(got, "## Prerequisites: Install the CLI")
	mcpIdx := strings.Index(got, "## MCP Server Installation")
	if prereqIdx < 0 || mcpIdx < 0 || prereqIdx >= mcpIdx {
		t.Errorf("Prerequisites must appear before MCP Server Installation; prereq=%d mcp=%d", prereqIdx, mcpIdx)
	}

	// Old `## CLI Installation` heading must be gone.
	if strings.Contains(got, "## CLI Installation") {
		t.Errorf("legacy ## CLI Installation heading still present:\n%s", got)
	}

	// References to the old heading must be updated.
	if strings.Contains(got, "see CLI Installation above") {
		t.Errorf("stale 'see CLI Installation above' reference still present")
	}
	if !strings.Contains(got, "see Prerequisites at the top of this skill") {
		t.Errorf("expected 'see Prerequisites at the top of this skill' reference")
	}

	// Argument Parsing routing rule must be updated.
	if strings.Contains(got, "otherwise → CLI installation") {
		t.Errorf("stale 'otherwise → CLI installation' routing rule still present")
	}
	if !strings.Contains(got, "otherwise → see Prerequisites above") {
		t.Errorf("expected 'otherwise → see Prerequisites above' routing rule")
	}
}

func TestPatchReadme_AnchorBased(t *testing.T) {
	// When the anchor is present (post-U3 fresh print), insert right after it.
	body := `Some content.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for...

<!-- pp-hermes-install-anchor -->
<details>
<summary>Manual JSON config (advanced)</summary>
`
	ctx := patchReadmeCtx{CLIName: "x-pp-cli", APIName: "x"}
	got, err := patchReadme(body, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "## Install via Hermes") {
		t.Errorf("Install via Hermes section must be inserted")
	}
	if !strings.Contains(got, "## Install via OpenClaw") {
		t.Errorf("Install via OpenClaw section must be inserted")
	}
	// Anchor must not be duplicated.
	if strings.Count(got, "<!-- pp-hermes-install-anchor -->") != 1 {
		t.Errorf("anchor should appear exactly once; got %d", strings.Count(got, "<!-- pp-hermes-install-anchor -->"))
	}
}

func TestPatchReadme_FallbackToClaudeDesktop(t *testing.T) {
	body := `Some content.

## Use with Claude Code

stuff.

## Use with Claude Desktop

other stuff.
`
	ctx := patchReadmeCtx{CLIName: "x-pp-cli", APIName: "x"}
	got, err := patchReadme(body, ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Insertion must be BEFORE Use with Claude Desktop.
	hermesIdx := strings.Index(got, "## Install via Hermes")
	desktopIdx := strings.Index(got, "## Use with Claude Desktop")
	if hermesIdx < 0 || desktopIdx < 0 || hermesIdx >= desktopIdx {
		t.Errorf("Install via Hermes must appear before Use with Claude Desktop; hermes=%d desktop=%d", hermesIdx, desktopIdx)
	}
}

func TestPatchReadme_Idempotent(t *testing.T) {
	body := `Some content.

<!-- pp-hermes-install-anchor -->
## Install via Hermes

stuff.

## Use with Claude Desktop
`
	ctx := patchReadmeCtx{CLIName: "x-pp-cli", APIName: "x"}
	got, err := patchReadme(body, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != body {
		t.Errorf("expected idempotent no-op when Install via Hermes already present;\ngot diff:\n%s", got)
	}
}
