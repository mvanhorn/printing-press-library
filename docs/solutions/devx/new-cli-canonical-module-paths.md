---
name: new-cli-canonical-module-paths
date: 2026-06-22
problem_type: bug
category: devx
component: library generated CLI packages, go.mod, cmd/*-pp-mcp
root_cause: New generated library CLIs kept local module names, and the ignored MCP main files were skipped by default rg scans during import-prefix cleanup.
resolution_type: fix
tags: [generated-cli, go-modules, supply-chain, mcp]
---

# New generated library CLIs must use canonical repository module paths.

## Symptoms

After adding the Attentive and Postscript library CLIs, the publish-package verifier passed, but the supply-chain scan failed:

```text
::error file=library/marketing/attentive/go.mod,line=1,title=supply-chain:module_path_noncanonical_on_new_cli::New library go.mod declares module attentive-pp-cli which does not start with the canonical prefix github.com/mvanhorn/printing-press-library/library/. | Fix: Use the canonical form: module github.com/mvanhorn/printing-press-library/library/<category>/<slug>.
::error file=library/marketing/postscript/go.mod,line=1,title=supply-chain:module_path_noncanonical_on_new_cli::New library go.mod declares module postscript-pp-cli which does not start with the canonical prefix github.com/mvanhorn/printing-press-library/library/. | Fix: Use the canonical form: module github.com/mvanhorn/printing-press-library/library/<category>/<slug>.
::error::supply-chain scan: 2 block-severity finding(s); see annotations above.
```

After changing the module declarations and rewriting most imports, `go test ./...` still failed for the MCP binaries:

```text
cmd/attentive-pp-mcp/main.go:10:2: package attentive-pp-cli/internal/mcp is not in std
cmd/postscript-pp-mcp/main.go:11:2: package postscript-pp-cli/internal/mcp is not in std
```

## What didn't work

- Running only the publish-package verifier was insufficient. It selected both new CLI dirs and passed, but it does not enforce canonical module paths.
- Rewriting imports with a default `rg` file list missed `cmd/*-pp-mcp/main.go`. Those MCP entrypoints are ignored by repository patterns and were only present because they had been force-added.

## Solution

Use canonical repository module paths in every new library CLI package:

```go
// Before
module attentive-pp-cli

// After
module github.com/mvanhorn/printing-press-library/library/marketing/attentive
```

```go
// Before
module postscript-pp-cli

// After
module github.com/mvanhorn/printing-press-library/library/marketing/postscript
```

Then rewrite Go import prefixes, including ignored-but-force-added MCP mains:

```go
// Before
import "attentive-pp-cli/internal/cli"
import mcptools "attentive-pp-cli/internal/mcp"

// After
import "github.com/mvanhorn/printing-press-library/library/marketing/attentive/internal/cli"
import mcptools "github.com/mvanhorn/printing-press-library/library/marketing/attentive/internal/mcp"
```

Use forced scans when checking generated MCP entrypoints:

```bash
rg -uuu '"attentive-pp-cli/internal/|"postscript-pp-cli/internal/' library/marketing/attentive library/marketing/postscript --glob '*.go'
```

The committed fix was verified with:

```bash
(cd library/marketing/attentive && go test ./...)
(cd library/marketing/postscript && go test ./...)
python3 .github/scripts/verify-publish-package/verify_publish_package.py --base-ref upstream/main
/tmp/pp-supply-chain-venv311/bin/python .github/scripts/verify-supply-chain/scan.py --base-ref upstream/main
```

## Why this works

Library packages are installed through module paths like:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/attentive/cmd/attentive-pp-cli@latest
```

If `go.mod` declares only `module attentive-pp-cli`, the package can compile locally but fails repository supply-chain policy for new library CLIs. Once the module path is canonical, all internal imports must also use that canonical prefix. The MCP mains need special attention because they are ignored by default file discovery despite being part of the PR.

## Prevention

- After generating a new library CLI, run the supply-chain scanner, not only publish-package validation.
- When changing generated package import prefixes, run `rg -uuu` or explicitly include `cmd/*-pp-mcp/main.go`.
- Treat `module <binary-name>` in a new library package as a blocker before PR.
