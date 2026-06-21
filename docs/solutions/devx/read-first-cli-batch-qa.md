---
name: read-first-cli-batch-qa
date: 2026-06-22
problem_type: knowledge
category: devx
component: cli-printing-press generated library packages
applies_when: generating or reviewing multi-package read-first CLI/MCP batches for printing-press-library
tags: [cli-printing-press, qa, mcp, generated-clis]
---

# Multi-package read-first CLI batches need generated-surface QA beyond build and smoke checks.

## Applies when

When adding several generated `library/<category>/<slug>` CLI/MCP packages in one PR, especially from compact local OpenAPI specs and without live vendor credentials.

## The pattern

Run the normal Go checks, then explicitly verify the generated user surfaces and publish-only invariants:

```bash
go test ./...
go vet ./...
go build ./...
python3 .github/scripts/verify-publish-package/verify_publish_package.py --base-ref upstream/main
python3 .github/scripts/verify-supply-chain/scan.py --base-ref upstream/main
```

For each package, also run:

```bash
<slug>-pp-cli --help
<slug>-pp-cli version
<slug>-pp-cli doctor --json
<slug>-pp-mcp --help
python3 .github/scripts/verify-skill/verify_skill.py --dir library/<category>/<slug>
```

Use dogfood and shipcheck to catch behavior that `go test` will not:

```bash
cli-printing-press shipcheck --no-live-check --json library/<category>/<slug>
```

Check these generated files before committing:

- `.printing-press.json` includes `novel_features`, but do not list generated baseline helpers such as `which` as novel features.
- `.printing-press-patches/default-sync-resources.json` exists when docs-derived specs need non-empty default sync resources.
- `internal/cli/sync.go` has a non-empty `defaultSyncResources()` list, otherwise `sync` can build and smoke-test while doing no useful default work.
- `SKILL.md` recipe commands include required positional args. For example, use `marketo-engage-pp-cli search record --help`, not `marketo-engage-pp-cli search --help`.
- `.manuscripts/<run-id>/proofs/` contains local verification, Phase 5 skip, and shipcheck proof artifacts.
- The diff does not include `registry.json`, `cli-skills/`, `.mcpb`, build outputs, root binaries, or generated CLI executables.

Vendor and generator quirks from this batch:

- Amplitude's `/export` surface collided with the generated/template `export` name. Model it with a non-reserved resource name such as `event_exports`.
- Gainsight's public `https://api.gainsight.com` resolved to private/internal AWS addresses in local validation. Use a tenant-placeholder default such as `https://example.gainsightcloud.com` and document that users configure their real tenant URL.
- Marketo Engage also needs a placeholder instance URL because customer REST hosts are tenant-specific.
- `shipcheck` can fail in the local sandbox with `httptest: failed to listen ... bind: operation not permitted`; rerun with normal host permissions before treating that as a package failure.

## Why

Generated CLI packages can pass compile, vet, and basic `--help` smoke checks while still having broken agent-facing workflows. The failures in this batch were mostly metadata and generated-surface issues: empty default sync resources, SKILL recipes that omitted required positionals, proof artifacts missing from manuscripts, reserved resource names, and environment-specific doctor/shipcheck behavior.

## Example

Before fixing the SKILL recipe, the verifier rejected a command that looked plausible but was missing the generated positional argument:

```bash
marketo-engage-pp-cli search --help
```

Use a resource-qualified command that matches the generated Cobra tree:

```bash
marketo-engage-pp-cli search record --help
```

Before fixing sync defaults, a generated package could include an empty default list:

```go
func defaultSyncResources() []string {
	return nil
}
```

Patch it to the package's core read resources and keep the proof in `.printing-press-patches/default-sync-resources.json`:

```go
func defaultSyncResources() []string {
	return []string{"prospects", "accounts"}
}
```

## Counter-cases

This checklist is not a replacement for live vendor Phase 5 validation when credentials exist. It is the local QA path for read-first packages that intentionally ship credential skip markers, and it should be paired with live checks before claiming authenticated vendor behavior.
