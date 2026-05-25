# Blender Absorb / Transcend Manifest

## Absorbed Features

| Source | Feature | Best Source | Decision |
| --- | --- | --- | --- |
| Blender CLI | Background rendering and Python execution | Blender Manual command-line arguments | Absorbed into `script run`, `render still`, and `render turntable` |
| Blender Python API | Import/render operators via `bpy.ops` | Blender API operator docs | Absorbed into generated Python job scripts |
| blender-mcp | Live stdio/http MCP server | Local installed `blmcp` source | Absorbed into `mcp serve` and `mcp status` |
| Onshape handoff context | CAD export should flow into Blender scene planning | Prior Onshape handoff plus local research | Absorbed into `handoff plan` and `asset import` |

## Transcendence Features

| Feature | Command | Score | Buildability | Evidence |
| --- | --- | ---: | --- | --- |
| Runtime mode resolver | `resolve blender` | 8 | hand-code | Local setup may be WSL Windows Blender, Linux PATH Blender, or MCP-only. |
| Safe headless script launcher | `script run` | 7 | hand-code | Blender argument order matters and raw output is not agent-shaped. |
| CAD-to-Blender handoff manifest | `handoff plan` | 8 | hand-code | Onshape export targets need a receiving-side Blender job contract. |
| Tandem MCP session controls | `mcp serve` / `mcp status` | 7 | hand-code | User explicitly asked for both tandem and headless modes. |
| Deterministic render job builders | `render still` / `render turntable` | 7 | hand-code | Render jobs are expensive; agents need previewable commands and stable defaults. |

## Killed Candidates

| Candidate | Kill Reason |
| --- | --- |
| Full material authoring DSL | Too broad for first printable CLI; should come after scene import/render reliability. |
| CFD/OnScale workflow | User mentioned possible future workflows, but current evidence is insufficient and it would depend on a separate simulation API. |
| Natural-language scene generation | Would depend on an LLM rather than local Blender/API leverage, so it fails the verifiability check. |
