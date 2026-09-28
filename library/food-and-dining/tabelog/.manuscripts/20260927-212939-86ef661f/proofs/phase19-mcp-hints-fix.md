# Late polish MCP hint correction

Fixed the manual catalog finding with narrow metadata changes. Native lists_refresh still declares local-write for its notebook updates, while a custom post-registration hook now advertises openWorldHint=true for its remote detail reads. The unchanged reserved Cobra walker, CLI classification, input schema, existing shell-out handler and child-CLI tenant ownership are preserved.

The offline comparison recipe explicitly declares readOnly=true, destructive=false and openWorld=false. The selected refresh recipe declares readOnly=false, destructive=false and openWorld=true. No feature, CLI behavior or runner guard changed; no reserved/generator source was edited.

Before/after focused checks show the metadata regressions fail first, then pass 19 events with 0 failures across CLI/MCP/E2E. The real executable MCP stdio tools/list catalog asserts all three hint sets and preserved refresh schema/tenant metadata. Tenant-gate conformance verifies 2 in-process and 12 child-CLI tools. Four local mutation classifications and isolated happy arguments remain intact. No source-origin requests or broad suite ran.

Production diff: phase19-mcp-hints-product.patch. Hashes, restoration notes and exact test names: phase19-mcp-hints-proof.json. Raw results: phase19-mcp-hints-before.jsonl and phase19-mcp-hints-after.jsonl. Durable patch/provenance is mirrored under the working .printing-press-patches directory. Source_research and root were signaled as soon as the edited tree passed and froze, so rebuilding/fresh acceptance can proceed while this prose is recorded.
