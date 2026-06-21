# Gamma CLI Absorb Manifest
Run: 20260619-153208-b1328533

## Absorbed From Ecosystem

### From Official MCP (`statechangelabs/gamma-app-mcp`)
- [x] All 7 REST API operations (generate, from-template, poll, themes, folders, archive, delete)
- [x] Async generation → poll loop with generationId
- [x] Credits tracking from poll response

### From `gamma_ai_python_sdk_ppt_generator`
- [x] Typed request/response structs with all enum values validated
- [x] Poll loop with configurable max-attempts and interval

## Novel Features (Invented)

### Core Workflow
1. **`--watch` flag on `generate` and `from-template`** — auto-polls every 5s and prints live status updates until completed/failed. Shows progress spinner. Returns final URL + export URL.

2. **`--download <dir>` flag** — after generation completes (with `--watch`), automatically downloads the `exportUrl` to a local file. Saves as `<gammaId>.pdf/pptx/zip`.

3. **`gamma poll <generationId>`** — standalone poll command. Re-polls a running or past generation by ID (useful if terminal died mid-watch). Supports `--watch` to continue polling.

### Batch & Config
4. **`gamma generate --input-file <file>`** — read a JSON/YAML file with a list of generation configs and batch-generate all of them. Prints a table of generationIds on kick-off, then polls all concurrently.

5. **`gamma config set/get`** — persist default `themeId`, `format`, `imageOptions.source`, `numCards`, etc. to `~/.config/gamma/config.yaml`. Flags override config, config overrides hardcoded defaults.

### Ergonomic Output
6. **`--json` flag on all commands** — emit raw JSON for piping into `jq` or scripts.

7. **`gamma themes`** — list themes with color/tone keywords. Supports `--query`, `--type` filter, `--all` (auto-paginate all pages).

8. **`gamma folders`** — list folders with `--query`, `--all` auto-paginate.

### Management
9. **`gamma archive <gammaId>`** — archive a gamma. Accepts gammaId (`g_...`) or gammaUrl (extracts the ID).

10. **`gamma delete <gammaId>`** — delete a gamma. Prompts for `--confirm` unless `--yes` / `-y` flag set (destructive).

### Image & Sharing Ergonomics
11. **`--image-source <source>`** shorthand — `aiGenerated`, `noImages`, `webFreeToUse`, etc. as a top-level flag without nesting into `imageOptions.source`.

12. **`--sharing <level>`** — quick sharing preset: `private` (noAccess/noAccess), `view-link` (noAccess/view), `edit-link` (noAccess/edit), `workspace-view` (view/noAccess).

13. **`--header-footer logo`** — convenience flag: puts themeLogo in bottom-left, cardNumber in bottom-right.

## Commands Summary

```
gamma generate [flags]          # POST /generations + optional --watch --download
gamma from-template [flags]     # POST /generations/from-template + optional --watch
gamma poll <id> [--watch]       # GET /generations/{id} + optional loop
gamma themes [--query] [--all]  # GET /themes
gamma folders [--query] [--all] # GET /folders
gamma archive <gammaId|url>     # POST /gammas/{id}/archive
gamma delete <gammaId|url> [-y] # DELETE /gammas/{id}
gamma config [set|get] [key] [value]
gamma completion <shell>        # shell completion
gamma version
```

## Out of Scope
- `get_gammas` / `read_gamma` from MCP — REST API has no equivalent; requires OAuth
- Editing existing gammas — not supported by API ("generate a new one")
