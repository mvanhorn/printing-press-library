# Blender PP CLI

Agent-native Blender automation for CAD handoff, headless rendering, turntables, and live MCP sessions.

## Quickstart

```bash
blender-pp-cli resolve blender --json
blender-pp-cli handoff plan --asset exports/assembly.glb --mode turntable --frames 120 --json
blender-pp-cli render still --output startup.png --engine BLENDER_EEVEE --resolution 1280x720 --dry-run --json
blender-pp-cli scene inspect --json --timeout 60s
blender-pp-cli mcp serve --transport http --host 127.0.0.1 --port 8000 --dry-run --json
```

No API credentials are required. Set `BLENDER_BIN` or `BLENDER_MCP_BIN` when the auto-resolver should use a specific executable.

## Runtime Modes

This CLI has two separate jobs:

- **Headless Blender** runs deterministic background work through `blender --background --python ...`. Use this for imports, inspections, still renders, turntable setup, CI smoke tests, and repeatable CAD-to-render jobs.
- **Tandem `blender-mcp`** starts or probes the separately installed `blender-mcp` server for live agent/Blender sessions. Use this when an agent needs to inspect and steer an open Blender scene interactively.

Runtime resolution checks `BLENDER_BIN`, then `PATH`, then known installed locations. On WSL, Windows Blender is supported at paths such as `/mnt/c/Program Files/Blender Foundation/Blender 5.1/blender.exe`; absolute Linux paths passed to that executable are translated with `wslpath -w`, with a `\\wsl.localhost\Ubuntu\...` fallback. Set `BLENDER_MCP_BIN` when `blender-mcp` is not on `PATH`.

## CAD to Blender Handoff

Prefer Onshape exports as `.glb`/`.gltf` for Blender rendering. GLB preserves scene hierarchy, basic colors, and instancing better than legacy geometry-only paths. Use `.stl` only when you need raw geometry; it does not carry material, normal, or hierarchy data. OBJ/FBX can work, but expect more manual cleanup.

A practical handoff looks like this:

```bash
blender-pp-cli handoff plan --asset exports/assembly.glb --mode turntable --frames 120 --json
blender-pp-cli asset import --asset exports/assembly.glb --output assembly.blend --json
blender-pp-cli scene inspect --file assembly.blend --json
blender-pp-cli render still --blend assembly.blend --output assembly.png --resolution 1280x720 --json
```

CAD meshes often arrive with dense flat faces, split normals, placeholder colors, and assembly-origin pivots. Treat the CLI as the deterministic import/render rail; use Blender Python or tandem MCP follow-up for Weighted Normal cleanup, material replacement, camera framing from bounding boxes, and live scene correction.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Runtime readiness
- **`resolve blender`** — Locate Blender and blender-mcp across WSL, PATH, and explicit env vars, then classify the available mode as headless, tandem, or both.

  _Prevents brittle assumptions about whether Blender is a Linux binary, Windows binary under WSL, or MCP-only install._

  ```bash
  blender-pp-cli resolve blender --json
  ```

### Headless automation
- **`script run`** — Build and optionally execute a correct `blender --background ... --python ... -- ...` invocation with dry-run preview and JSON result capture.

  _Turns arbitrary Blender Python automation into a predictable CLI primitive._

  ```bash
  blender-pp-cli script run --script job.py --dry-run
  ```

### CAD handoff
- **`handoff plan`** — Turn an Onshape/export asset path into a reproducible Blender job manifest with blend output, collection, render, and turntable defaults.

  _Bridges CAD export reasoning into repeatable rendering and animation work._

  ```bash
  blender-pp-cli handoff plan --asset exports/assembly.glb --mode turntable --frames 120 --json
  ```

### Tandem workflow
- **`mcp serve`** — Start blender-mcp over stdio or HTTP, with dry-run previews and status probing for live agent/Blender collaboration.

  _Lets an agent explicitly switch from deterministic headless jobs to live inspection and steering._

  ```bash
  blender-pp-cli mcp serve --transport http --host 127.0.0.1 --port 8000 --dry-run --json
  ```

### Rendering
- **`render still`** — Create still and turntable render invocations with explicit engine, output, resolution, frame count, and safe dry-run support.

  _Makes visual production reproducible instead of a one-off sequence of UI actions._

  ```bash
  blender-pp-cli render still --output startup.png --engine BLENDER_EEVEE --resolution 1280x720 --dry-run --json
  ```

## Verification

This run completed full live dogfood against the local setup:

```text
30 passed, 0 failed, 10 skipped
```

Phase 5 evidence is in `.manuscripts/20260520-035524/proofs/phase5-acceptance.json`.
