# Absorb Manifest

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | API-key authenticated REST access | Onshape API Keys docs | Signed Onshape request headers in the Go client | Works from environment variables and supports non-interactive agents |
| 2 | Endpoint exploration | Glassworks API Explorer | `api`, `which`, `agent-context`, and generated command help | Terminal-native discovery without opening the browser |
| 3 | Document search | Glassworks documents endpoints | `documents search` | Compact JSON, `--select`, local provenance envelope, MCP mirroring |
| 4 | Workspace/version discovery | Onshape document model docs | `workspaces list`, `versions list` | Makes DWVME routing explicit for agents |
| 5 | Element listing | Glassworks elements endpoints | `elements` | Typed routing table for Part Studios, assemblies, BOMs, blobs, and app tabs |
| 6 | Part inspection | Glassworks parts endpoints | `parts list`, `parts get` | Extracts part IDs needed for export and downstream CAD workflows |
| 7 | Assembly inspection | Glassworks assemblies endpoints | `assemblies get` | Selectable nested assembly instance data for context-efficient review |
| 8 | Translation/export jobs | Glassworks translation endpoints | `translations create`, `translations get`, `exports part`, `exports element` | Ready base for Blender, manufacturing, and simulation handoff |
| 9 | Local archive/search | Printing Press local store | `sync`, `search`, `analytics`, `workflow archive` | Lets agents reuse CAD context across sessions and reduce repeated API calls |
| 10 | Agent/MCP surface | Existing Onshape MCP interest | `onshape-pp-mcp` and runtime Cobra-tree tool mirror | Same CLI behavior is available to MCP-capable agents |

## Transcendence

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|--------------------------|
| 1 | CAD document triage for agents | `documents search` | spec-emits | Combines Onshape search with `--agent`, `--select`, compact JSON, and provenance metadata |
| 2 | Workspace-aware element routing | `elements` | spec-emits | Turns Onshape DWVME URLs into an agent-usable routing table |
| 3 | Assembly instance graph capture | `assemblies get` | spec-emits | Makes nested assembly data selectable and context-safe |
| 4 | Part studio inventory for export decisions | `parts list` | spec-emits | Bridges Part Studio IDs to export/render/simulation handoff |
| 5 | Offline CAD archive and search | `sync` | hand-code | Local SQLite state gives agents durable memory across CAD review sessions |

## Follow-Up Candidates

- Add a first-class `workflow blender-prep` command that searches documents, resolves workspace + assembly/part IDs, starts a translation job, polls it, and writes a manifest for Blender import.
- Add mass-properties and bounding-box commands for simulation/rendering decisions.
- Add BOM/table support to connect assemblies to purchased/manufactured components.
- Add FeatureScript evaluation helpers for parameterized studies and Onscale/CFD preflight.
