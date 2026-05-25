---
name: pp-blender
description: Use this skill when an agent needs to automate Blender locally, prepare CAD/export assets for Blender, render stills or turntables, or start a blender-mcp tandem session.
---

# Blender PP CLI

## Prerequisites: Install the CLI

This skill drives the `blender-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install blender --cli-only
   ```
2. Verify: `blender-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use `blender-pp-cli` to choose between local headless Blender automation and live blender-mcp tandem work.

## Runtime

Run this first:

```bash
blender-pp-cli resolve blender --json
```

If auto-resolution is wrong, set `BLENDER_BIN` or `BLENDER_MCP_BIN`.

Use headless Blender commands for deterministic jobs: import, inspect, render, turntable setup, and CI/live dogfood smoke tests. Use tandem `blender-mcp` only when an agent needs an interactive live Blender scene it can inspect and steer.

On WSL, Windows Blender paths under `/mnt/c/Program Files/Blender Foundation/.../blender.exe` are valid. The CLI translates absolute Linux paths for Windows Blender with `wslpath -w`; if `wslpath` is unavailable, it falls back to `\\wsl.localhost\Ubuntu\...` paths. Set:

```bash
export BLENDER_BIN="/mnt/c/Program Files/Blender Foundation/Blender 5.1/blender.exe"
export BLENDER_MCP_BIN="$HOME/.local/bin/blender-mcp"
```

## Common Workflows

Create a CAD-to-Blender job plan:

```bash
blender-pp-cli handoff plan --asset exports/assembly.glb --mode turntable --frames 120 --json
```

Import an Onshape-exported GLB and inspect the saved scene:

```bash
blender-pp-cli asset import --asset exports/assembly.glb --output assembly.blend --json
blender-pp-cli scene inspect --file assembly.blend --json
```

Preview a headless render:

```bash
blender-pp-cli render still --output startup.png --engine BLENDER_EEVEE --resolution 1280x720 --dry-run --json
```

Run a Blender Python script:

```bash
blender-pp-cli script run --script job.py --json
```

Start tandem MCP mode:

```bash
blender-pp-cli mcp serve --transport http --host 127.0.0.1 --port 8000 --json
```

## Notes

- Use `--dry-run` before imports, renders, and server starts when you are planning.
- Headless commands use Blender's background Python path.
- Tandem commands rely on the separately installed `blender-mcp` executable.
- Prefer `.glb` or `.gltf` for Onshape-to-Blender rendering handoff. GLB keeps hierarchy, basic colors, and instancing better than STL/OBJ.
- Treat `.stl` as geometry-only fallback. It will not preserve materials, normals, or hierarchy.
- CAD imports usually need follow-up cleanup: Weighted Normal modifiers for split-normal artifacts, material replacement for placeholder CAD colors, scale checks, and camera framing from object bounds.
- Use `scene inspect --json --timeout 60s` as the live startup-scene smoke path. Avoid committed binary `.blend` fixtures unless a workflow explicitly needs one.

## Unique Capabilities

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
