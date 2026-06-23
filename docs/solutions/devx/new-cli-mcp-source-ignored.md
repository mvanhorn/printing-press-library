---
name: new-cli-mcp-source-ignored
date: 2026-06-22
problem_type: bug
category: devx
component: .gitignore, library/<category>/<slug>/cmd/<slug>-pp-mcp/main.go
root_cause: the root ignore rule for generated MCP binaries also ignored new MCP source directories unless the files were force-added
resolution_type: fix
tags: [printing-press, github-actions, manifest-verifier, mcp, gitignore]
---

# New CLI PR failed manifest verification because the MCP command source was ignored

## Symptoms

GitHub Actions failed the `Verify manifest.json / Validate MCPB manifest contract` workflow for a new CLI PR.

The failure snippet was:

```text
##[group]library/media-and-entertainment/paul-graham
##[error]cmd/paul-graham-pp-mcp directory is missing
##[endgroup]
Checked 254 manifest.json file(s); 1 failed.
```

Local package checks were misleadingly green because the missing source existed on disk:

```text
go test ./...
? github.com/.../cmd/paul-graham-pp-mcp [no test files]
```

## What didn't work

Running only local Go checks did not catch the issue because `go test ./...` includes ignored files that exist in the working tree.

Running the publish-package verifier did not catch the exact CI failure either; it validates the publish package shape but the failing Actions job runs `.github/scripts/verify-manifest/verify_manifest.py` across all `manifest.json` files.

Checking `git status --short` without `--ignored` was also insufficient because the MCP source directory was ignored and therefore hidden.

## Solution

Add an explicit library-source exception for MCP command directories and make sure the MCP `main.go` is tracked.

```gitignore
# Compiled Go binaries (generated CLIs)
*-pp-cli
*-pp-mcp
*-pp-cli-[0-9]*
# But allow CLI source directories in the library
!library/**/*-pp-cli/
!library/**/*-pp-mcp/
```

Then rerun the exact workflow verifier locally:

```bash
python3 .github/scripts/verify-manifest/verify_manifest.py
```

For the Paul Graham PR, this changed the result from one failed manifest to:

```text
Checked 254 manifest.json file(s); 0 failed.
```

## Why this works

The root `.gitignore` ignored `*-pp-mcp` to keep generated MCP binaries out of commits. A new generated package also has a source directory named `cmd/<slug>-pp-mcp/`, so Git treated the entire command directory as ignored until it was force-added.

The negation rule keeps the binary ignore behavior while allowing source directories under `library/**/cmd/` to be tracked normally.

## Prevention

For new generated CLI packages, run both:

```bash
git status --ignored --short library/<category>/<slug>/cmd/<slug>-pp-mcp/
python3 .github/scripts/verify-manifest/verify_manifest.py
```

Review the committed diff for both command entrypoints:

```text
library/<category>/<slug>/cmd/<slug>-pp-cli/main.go
library/<category>/<slug>/cmd/<slug>-pp-mcp/main.go
```
