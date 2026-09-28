# Lightweight generation constraints

Read-only findings from installed `cli-printing-press` **v4.32.5** (`generate --help`, phase 10 instructions, and its existing Go module cache). No phase entry, generation, generator edits, or code changes performed by this research agent.

Source root: `/Users/zjsng/go/pkg/mod/github.com/mvanhorn/cli-printing-press/v4@v4.32.5/`.

## Supported pre-generation decisions

```yaml
http_transport: standard
auth:
  type: none
learn:
  disabled: true
cache:
  enabled: false
mcp:
  transport: [stdio]
```

Root explicitly chose learning disabled because travel discovery does not need hidden learning writes or the extra command surface. `learn.disabled: true` is the real switch; `learn.enabled: false` is not the sanctioned opt-out. Root must record this phase-10 decision. Leave freshness auto-refresh disabled for per-user saved lists; use explicit fetch times and bounded refresh instead.

`generate` accepts `--transport standard` and `--mcp-transport stdio`, plus `--mcp-endpoint-tools visible|hidden`, `--mcp-orchestration endpoint-mirror|code`, and `--mcp-intents <file>`. There are **no** exposed flags for no-MCP, disabling all built-ins, or deferring store initialization. Resource/endpoint count flags cap large specs; they are not framework-trimming controls.

## Runtime and dependency constraints

- MCP source is generated unconditionally (`internal/generator/vision_templates.go:150`, `generator.go:4310`). It has a separate executable entry point. Building/invoking the CLI does not require starting an MCP sidecar. Small specs otherwise default to compiled stdio+HTTP; explicit `[stdio]` prevents unnecessary HTTP listener support.
- SQLite is conditional on profiler-selected `VisionSet.Store` (`templates/go.mod.tmpl`). Learning defaults on and forces Store; disabling learning removes that forced promotion but **does not guarantee no SQLite**. Profiling can still select Store based on persistence/search/data volume. There is no documented spec switch to force Store off. Examine the generated profile before choosing saved-list storage.
- Store handles open explicitly; `store.OpenWithContext` creates/migrates, while `OpenReadOnlyContext` skips creation/migration. Root help bypasses freshness hooks, and learning disabled omits learning hooks/journaling. There is no deferred-store flag to set. Implement saved-list initialization in the commands that need it.
- Standard transport avoids browser/TLS/HTTP3 dependencies. Auth:none avoids cookie/WebSocket/auth browser dependencies. HTML extraction adds `golang.org/x/net/html`; base framework uses Cobra/pflag/TOML. MCP-go remains in generated go.mod; modernc SQLite appears only when Store is selected. Do not confuse module requirements with packages linked into the ordinary CLI binary.

## Small help and agent surface

The base generated framework reserves eight verbs: agent-context, completion, doctor, feedback, help, profile, version, which (`generator.go:3558`). Learning adds seven root families; profiling adds optional storage/sync/workflow/import/export/analytics surfaces. There is no generator-wide built-in-count setting.

Use a separate preserved `internal/cli/<name>.go` file with `registerNovelCommand` to add the primary find/show/areas/cuisines/lists surface and shape help after all ordinary registrations (`templates/root.go.tmpl:701`). Advanced commands can remain explicitly callable while `Hidden=true` removes them from ordinary help. Cobra help/usage functions may be customized through that hook without replacing generated root.go or modifying the generator.

The MCP walker skips hidden commands and framework-only commands; `mcp:hidden=true` prunes an entire subtree, whereas a Cobra-hidden parent alone may still expose visible children (`templates/cobratree/classify.go.tmpl`, `walker.go.tmpl`). Hidden flags are omitted from mirrored tool schemas. Apply read-only annotations to novel reads, and honest local-write semantics to saved-list mutations. Hiding Cobra commands does not by itself remove separately generated typed endpoint mirrors; use `mcp.endpoint_tools: hidden` only if the intended command surface provides the complete useful interface.

Code orchestration only compresses typed endpoint mirrors; it does not compress Cobra-mirrored novel commands. For this small CLI it adds little value. Keep tool count and token footprint under inspection instead of assuming orchestration makes all tooling compact.

## HTML parser boundary

Installed spec/runtime supports `response_format: html` and `html_extract.mode` values `page`, `links`, `table`, `embedded-json`; fields `limit`, `link_prefixes`, `script_selector`, `json_path` (`internal/spec/spec.go:2825`, `:5551`). Some installed skill examples omit table mode and show older selector comments; the current template also supports attribute selectors.

There is no custom extractor registry or declarative card-field selector mapping. Generic extraction cannot assemble Tabelog list cards, meal budgets, station distance, multiple JSON-LD nodes and detail tables into the required restaurant model. Put source-specific parsers in separate hand-authored files, reusing the generated bounded HTTP client and meaningful fixture/live E2E checks. Avoid editing the generated extractor. Novel calls must inherit command context/timeout and clean HTML text as phase 11 requires.
