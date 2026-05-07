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
	ctx := patchSkillCtx{AuthorName: "Trevin Chow"}

	t.Run("inserts after description when fields absent", func(t *testing.T) {
		in := `name: pp-test
description: "a CLI"
argument-hint: "..."
`
		want := `name: pp-test
description: "a CLI"
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
author: "Wrong Operator"
license: "Apache-2.0"
`
		want := `description: "a CLI"
author: "Trevin Chow"
license: "Apache-2.0"
`
		if got := ensureFrontmatterTopLevelFields(in, ctx); got != want {
			t.Errorf("expected author rewritten to ctx value;\nwant: %q\ngot:  %q", want, got)
		}
	})

	t.Run("strips legacy version: line without re-emitting", func(t *testing.T) {
		// Earlier sweep emitted `version:` tracking the Press version.
		// That decision was reverted (see top-of-file comment); a re-sweep
		// must drop the line and not re-add it.
		in := `description: "a CLI"
version: "3.10.0"
author: "Trevin Chow"
license: "Apache-2.0"
`
		want := `description: "a CLI"
author: "Trevin Chow"
license: "Apache-2.0"
`
		if got := ensureFrontmatterTopLevelFields(in, ctx); got != want {
			t.Errorf("expected version: line stripped;\nwant: %q\ngot:  %q", want, got)
		}
	})

	t.Run("escapes special characters via fmt %q", func(t *testing.T) {
		ctxQuoted := patchSkillCtx{AuthorName: `Trevin "Quoted" Chow`}
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

func TestPatchReadmeHermesOpenClaw_OrderAfterInstall(t *testing.T) {
	// Canonical layout: ## Install → ## Install for Hermes → ## Install for OpenClaw → next section.
	body := `# X CLI

## Install

[install body]

## Authentication

stuff.
`
	ctx := patchReadmeCtx{CLIName: "x-pp-cli", APIName: "x", Category: "other"}
	got := patchReadmeHermesOpenClaw(body, ctx)

	installIdx := strings.Index(got, "## Install\n")
	hermesIdx := strings.Index(got, "## Install for Hermes")
	openclawIdx := strings.Index(got, "## Install for OpenClaw")
	authIdx := strings.Index(got, "## Authentication")

	if installIdx < 0 || hermesIdx < 0 || openclawIdx < 0 || authIdx < 0 {
		t.Fatalf("missing expected section: install=%d hermes=%d openclaw=%d auth=%d\n%s",
			installIdx, hermesIdx, openclawIdx, authIdx, got)
	}
	if !(installIdx < hermesIdx && hermesIdx < openclawIdx && openclawIdx < authIdx) {
		t.Errorf("expected order Install → Install for Hermes → Install for OpenClaw → Authentication; got positions %d/%d/%d/%d:\n%s",
			installIdx, hermesIdx, openclawIdx, authIdx, got)
	}
}

func TestPatchReadmeHermesOpenClaw_MovesFromBottomToAfterInstall(t *testing.T) {
	// Fedex-shape: Install at top, Hermes/OpenClaw deep in the file
	// near Use with Claude Desktop. Sweep moves them up.
	body := `# Fedex CLI

## Install

cli body.

## Authentication

auth body.

## Use with Claude Code

claude code body.

<!-- pp-hermes-install-anchor -->
## Install via Hermes

hermes body.

## Install via OpenClaw

openclaw body.

## Use with Claude Desktop

desktop body.
`
	ctx := patchReadmeCtx{CLIName: "fedex-pp-cli", APIName: "fedex", Category: "commerce"}
	got := patchReadmeHermesOpenClaw(body, ctx)

	hermesIdx := strings.Index(got, "## Install for Hermes")
	authIdx := strings.Index(got, "## Authentication")
	codeIdx := strings.Index(got, "## Use with Claude Code")

	if hermesIdx < 0 || authIdx < 0 || codeIdx < 0 {
		t.Fatalf("missing expected section: hermes=%d auth=%d code=%d\n%s", hermesIdx, authIdx, codeIdx, got)
	}
	// Hermes must now be BEFORE Authentication (i.e. moved to top), not BEFORE Use with Claude Code (its old neighbor).
	if !(hermesIdx < authIdx) {
		t.Errorf("Hermes must appear before Authentication after sweep; hermes=%d auth=%d:\n%s", hermesIdx, authIdx, got)
	}
	// Old "via" naming gone.
	if strings.Contains(got, "## Install via Hermes") || strings.Contains(got, "## Install via OpenClaw") {
		t.Errorf("legacy 'via' headings still present:\n%s", got)
	}
	// Anchor still present, exactly once.
	if strings.Count(got, "<!-- pp-hermes-install-anchor -->") != 1 {
		t.Errorf("anchor should appear exactly once; got %d", strings.Count(got, "<!-- pp-hermes-install-anchor -->"))
	}
}

func TestPatchReadmeHermesOpenClaw_MovesFromTopToAfterInstall(t *testing.T) {
	// ESPN-shape: Hermes/OpenClaw are FIRST (above Install), need to move down.
	body := `# ESPN CLI

A summary.

<!-- pp-hermes-install-anchor -->
## Install via Hermes

hermes body.

## Install via OpenClaw

openclaw body.

## Install

cli body.

## Authentication

auth body.
`
	ctx := patchReadmeCtx{CLIName: "espn-pp-cli", APIName: "espn", Category: "media-and-entertainment"}
	got := patchReadmeHermesOpenClaw(body, ctx)

	installIdx := strings.Index(got, "## Install\n")
	hermesIdx := strings.Index(got, "## Install for Hermes")
	openclawIdx := strings.Index(got, "## Install for OpenClaw")
	authIdx := strings.Index(got, "## Authentication")

	if installIdx < 0 || hermesIdx < 0 || openclawIdx < 0 || authIdx < 0 {
		t.Fatalf("missing expected section: install=%d hermes=%d openclaw=%d auth=%d\n%s",
			installIdx, hermesIdx, openclawIdx, authIdx, got)
	}
	if !(installIdx < hermesIdx && hermesIdx < openclawIdx && openclawIdx < authIdx) {
		t.Errorf("expected order Install → Install for Hermes → Install for OpenClaw → Authentication; got positions %d/%d/%d/%d:\n%s",
			installIdx, hermesIdx, openclawIdx, authIdx, got)
	}
	// Old "via" naming gone.
	if strings.Contains(got, "## Install via Hermes") || strings.Contains(got, "## Install via OpenClaw") {
		t.Errorf("legacy 'via' headings still present:\n%s", got)
	}
}

func TestPatchReadmeHermesOpenClaw_Idempotent(t *testing.T) {
	body := `# X CLI

## Install

cli body.

## Authentication

auth body.
`
	ctx := patchReadmeCtx{CLIName: "x-pp-cli", APIName: "x", Category: "other"}
	first := patchReadmeHermesOpenClaw(body, ctx)
	second := patchReadmeHermesOpenClaw(first, ctx)
	if second != first {
		t.Errorf("second run should produce zero diff;\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestPatchReadmeHermesOpenClaw_NoOpWhenInstallSectionAbsent(t *testing.T) {
	// agent-capture has no ## Install heading. Tool should leave it
	// alone rather than insert at an arbitrary position.
	body := `# agent-capture

## Quick Start

stuff.
`
	ctx := patchReadmeCtx{CLIName: "agent-capture-pp-cli", APIName: "agent-capture", Category: "developer-tools"}
	got := patchReadmeHermesOpenClaw(body, ctx)
	if got != body {
		t.Errorf("expected no-op when ## Install absent;\ngot:\n%s", got)
	}
}

func TestPatchReadmeInstall_RewritesLegacyBinaryGoSection(t *testing.T) {
	// Legacy shape: ## Install with ### Binary and ### Go subsections
	// from the pre-npx readme.md.tmpl.
	body := `# X CLI

Some prose.

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://example/releases). On macOS, clear the Gatekeeper quarantine.

### Go

` + "```" + `
go install github.com/mvanhorn/printing-press-library/library/other/x/cmd/x-pp-cli@latest
` + "```" + `

## Authentication

stuff.
`
	ctx := patchReadmeCtx{CLIName: "x-pp-cli", APIName: "x", Category: "other"}
	got := patchReadmeInstall(body, ctx)

	// Legacy headings gone.
	if strings.Contains(got, "### Binary\n") {
		t.Errorf("legacy ### Binary subsection still present:\n%s", got)
	}
	// Canonical npx install line present.
	if !strings.Contains(got, "npx -y @mvanhorn/printing-press install x\n") {
		t.Errorf("canonical npx install line not present:\n%s", got)
	}
	if !strings.Contains(got, "npx -y @mvanhorn/printing-press install x --cli-only") {
		t.Errorf("--cli-only variant not present:\n%s", got)
	}
	// Go fallback retained, with module path derived from category.
	if !strings.Contains(got, "go install github.com/mvanhorn/printing-press-library/library/other/x/cmd/x-pp-cli@latest") {
		t.Errorf("Go fallback module path missing:\n%s", got)
	}
	// Pre-built binary block retained as last subsection.
	if !strings.Contains(got, "### Pre-built binary") {
		t.Errorf("Pre-built binary subsection missing:\n%s", got)
	}
	// Surrounding sections preserved.
	if !strings.Contains(got, "## Authentication") {
		t.Errorf("trailing ## Authentication section was lost:\n%s", got)
	}
	if !strings.Contains(got, "Some prose.") {
		t.Errorf("leading prose was lost:\n%s", got)
	}
	// ## Install heading appears exactly once.
	if strings.Count(got, "## Install\n") != 1 {
		t.Errorf("## Install heading should appear exactly once; got %d", strings.Count(got, "## Install\n"))
	}
}

func TestPatchReadmeInstall_Idempotent(t *testing.T) {
	body := `# X CLI

## Install

### Binary

old binary text.

### Go

old go text.

## Authentication
`
	ctx := patchReadmeCtx{CLIName: "x-pp-cli", APIName: "x", Category: "other"}
	first := patchReadmeInstall(body, ctx)
	second := patchReadmeInstall(first, ctx)
	if second != first {
		t.Errorf("second run should produce zero diff;\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestPatchReadmeInstall_NoOpWhenInstallSectionAbsent(t *testing.T) {
	// agent-capture's README has Quick Start but no ## Install heading.
	// Tool must leave it alone.
	body := `# agent-capture

Some prose.

## Quick Start

stuff.
`
	ctx := patchReadmeCtx{CLIName: "agent-capture-pp-cli", APIName: "agent-capture", Category: "developer-tools"}
	got := patchReadmeInstall(body, ctx)
	if got != body {
		t.Errorf("expected no-op when ## Install absent;\ngot:\n%s", got)
	}
}

func TestPatchReadmeInstall_DoesNotMatchInstallViaHermes(t *testing.T) {
	// `## Install via Hermes` must not be confused with `## Install`.
	body := `# X CLI

## Install via Hermes

stuff.

## Install via OpenClaw

other stuff.
`
	ctx := patchReadmeCtx{CLIName: "x-pp-cli", APIName: "x", Category: "other"}
	got := patchReadmeInstall(body, ctx)
	if got != body {
		t.Errorf("expected no-op when only ## Install via X headings present (no bare ## Install);\ngot:\n%s", got)
	}
}

func TestPatchReadmeInstall_CategoryPathFromContext(t *testing.T) {
	// The Go module path must reflect the category passed in ctx, not
	// hardcode "other". This catches a regression where category got
	// dropped during a refactor.
	body := `# Y CLI

## Install

### Go

` + "```" + `
go install github.com/mvanhorn/printing-press-library/library/other/y/cmd/y-pp-cli@latest
` + "```" + `

## Next
`
	ctx := patchReadmeCtx{CLIName: "y-pp-cli", APIName: "y", Category: "commerce"}
	got := patchReadmeInstall(body, ctx)
	if !strings.Contains(got, "library/commerce/y/cmd/y-pp-cli@latest") {
		t.Errorf("expected module path under library/commerce/...; got:\n%s", got)
	}
	if strings.Contains(got, "library/other/y/cmd/y-pp-cli@latest") {
		t.Errorf("legacy library/other/... path leaked through:\n%s", got)
	}
}
