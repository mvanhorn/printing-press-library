# Blender Printing Press CLI Brief

## Thesis

Blender is a local application and Python automation surface rather than a remote HTTP API. A useful printed CLI should therefore make the local runtime explicit: resolve Blender, run safe headless jobs, support tandem blender-mcp sessions, and preserve CAD-to-render handoff intent in structured manifests.

## Users

- CAD-to-rendering agents that receive Onshape exports and need to create a Blender scene without opening the UI.
- Human/agent pairs working in a live Blender session who need MCP-assisted inspection and steering.
- Automation agents that need repeatable stills or turntables for design review, documentation, or animation drafts.

## Top Workflows

- Resolve whether Blender can run headlessly, whether blender-mcp is installed, and which execution path to use.
- Turn an exported CAD asset into a Blender job manifest with output blend path, collection, render engine, and turntable settings.
- Preview render/import/script commands with dry-run JSON before starting expensive Blender work.
- Start or probe blender-mcp when the user is present for tandem work.

## Evidence

- Blender Manual documents `--background`, render options, argument ordering, and passing script args after `--`.
- Blender Python API documents operator invocation through `bpy.ops`, including import and render operator families.
- Local blender-mcp source exposes stdio/http transports, live code execution, blendfile summaries, screenshots, and render helpers.
- Local verification found Blender at `/mnt/c/Program Files/Blender Foundation/Blender 5.1/blender.exe` and blender-mcp at `/home/markimus/.local/bin/blender-mcp`.

## NotebookLM Limitation

The handoff asked future sessions to query the user's NotebookLM research first. `nlm` is installed, but authentication is expired; `nlm list notebooks` returned an authentication error and instructed re-login. This run proceeded with official docs and local installed package inspection.
