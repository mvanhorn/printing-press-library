# Blender Live Smoke

## Commands

```bash
./bin/blender-pp-cli script run --script /tmp/blender-pp-smoke.py --json --timeout 60s
./bin/blender-pp-cli scene inspect --timeout 60s
./bin/blender-pp-cli render still --output /tmp/blender-pp-smoke.png --resolution 64x64 --timeout 120s
```

## Result

- Blender launched through `/mnt/c/Program Files/Blender Foundation/Blender 5.1/blender.exe`.
- `script run` executed a Python smoke script and reported 3 startup-scene objects.
- `scene inspect` returned the startup cube, light, camera, materials, and render engine.
- `render still` wrote `/tmp/blender-pp-smoke.png` with `BLENDER_EEVEE` at 64x64.

## Fixes Proven

- Windows Blender under WSL receives UNC-translated script/output paths instead of raw `/tmp/...` paths.
- Python tracebacks and file-open failures are treated as command failures even when Blender exits with code 0.
- Default render engine is `BLENDER_EEVEE`, matching the installed Blender 5.1.1 runtime.
