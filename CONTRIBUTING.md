# Contributing to Printing Press Library

## Submitting a CLI

The easiest way to submit a CLI is with the `/printing-press publish` skill:

```bash
/printing-press publish notion-pp-cli
```

The skill handles everything: validation, packaging, and PR creation. You don't need to clone this repo manually.

### What the Skill Does

1. **Finds your CLI** in `~/printing-press/library/` by name (exact, suffix, or fuzzy match)
2. **Determines the category** from the manifest, catalog, or asks you
3. **Validates** the CLI builds, passes `go vet`, and has a manifest
4. **Packages** the CLI source + manuscripts into the library structure
5. **Opens a PR** with a structured description, `--help` output, and manuscript links

### Prerequisites

- [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) installed (`go install github.com/mvanhorn/cli-printing-press/v4/cmd/cli-printing-press@latest`)
- `gh` CLI installed and authenticated (`gh auth login`)
- A generated CLI in your local library (`~/printing-press/library/`)

### Manual Submission

If you prefer to submit manually:

1. Generate your CLI with `/printing-press <API>`
2. Verify it passes quality gates: `go build ./...`, `go vet ./...`
3. Fork this repo and create a branch: `feat/<slug>`
4. Add your CLI under `library/<category>/<slug>/` with:
   - The full CLI source code
   - `.printing-press.json` manifest
   - `SKILL.md` and `.goreleaser.yaml`
   - `manifest.json` (only if the CLI ships an MCP server)
   - `.manuscripts/` directory with research and proof artifacts
5. **Do not edit `registry.json` or `cli-skills/pp-*/SKILL.md` in the PR.** They are generated artifacts, regenerated after merge by `generate-registry.yml` and `generate-skills.yml`, and the generated-artifact guard in CI fails any PR that modifies them. Instead, make sure the source files under `library/<category>/<slug>/` are present: `registry.json` is generated from `.printing-press.json` + `.goreleaser.yaml` (plus `manifest.json` if the CLI ships an MCP server), and `cli-skills/pp-<slug>/SKILL.md` is mirrored from `library/<category>/<slug>/SKILL.md`.
6. Open a PR

### Quality Expectations

- `.printing-press.json` manifest must be present with `api_name` and `cli_name`
- `go build ./...` must succeed
- `go vet ./...` must pass
- The CLI must respond to `--help` and `--version`
- Manuscripts (research briefs, shipcheck results) are strongly encouraged

### Commit Style

Conventional commits: `feat(<api-name>): add <cli-name>`
