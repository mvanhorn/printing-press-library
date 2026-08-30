# kie-cli

An unofficial command-line client for [Kie.ai](https://kie.ai), the AI generation
aggregator API covering image, video, music, and chat models from Google, Runway,
Veo, Suno, Flux, Kling, Grok, Claude, and 100+ other models under one unified
task-based API.

Generated with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
from Kie.ai's published documentation at [docs.kie.ai](https://docs.kie.ai), then
hand-patched and validated (`go build`, `go test ./...`, `cli-printing-press
scorecard` — grade A / 88%).

## What's covered

- **70 endpoints** across the dedicated product APIs: Common (credits, download
  links), File Upload, 4o Image, Flux Kontext, Runway (+ Aleph), Veo3.1, Suno
  (music/lyrics/vocals/MIDI/WAV/voice-cloning — its largest family), Gemini Omni,
  and per-model chat completions (GPT, Claude, Gemini, Grok, Codex).
- **The full Market catalog** (120+ image/video/audio models) via Kie.ai's own
  unified `createTask` / `recordInfo` pattern — one pair of CLI commands
  (`kie-ai-jobs market-create-task` / `market-query-task`) that takes any
  model's identifier and JSON input. See [`docs/MODELS.md`](docs/MODELS.md) for
  the full catalog with `--model` values.

Every generation endpoint follows the same async shape: submit a task, get a
`taskId` back, then either poll a `record-info`/`get-details` endpoint or supply
a `--call-back-url` to receive a webhook when it's done.

## Install

Requires Go 1.26.5+.

```bash
git clone https://github.com/kelvincushman/kie-cli.git
cd kie-cli
go build -o kie ./cmd/kie-pp-cli
sudo mv kie /usr/local/bin/   # or anywhere on your PATH
```

## Auth

Get an API key at [kie.ai/api-key](https://kie.ai/api-key), then either:

```bash
kie auth set-token YOUR_API_KEY
```

or

```bash
export KIE_BEARER_AUTH="YOUR_API_KEY"
```

Verify everything's wired up:

```bash
kie doctor
```

## Usage

```bash
# Check remaining credits
kie chat

# Generate an image with Google's Nano Banana via the unified Market endpoint
kie kie-ai-jobs market-create-task \
  --model google/nano-banana \
  --input '{"prompt": "a corgi wearing a tiny wizard hat", "aspect_ratio": "1:1"}'

# Poll the resulting task
kie kie-ai-jobs market-query-task --task-id task_google_xxxxx

# Generate a Veo 3.1 video (dedicated endpoint)
kie veo generate-veo3-1-video --prompt "drone shot over a foggy mountain range"

# Generate music with Suno
kie generate music --prompt "upbeat synthwave with female vocals" --model V5 --instrumental false --call-back-url https://example.com/callback

# Edit an image with Flux Kontext
kie flux generate-or-edit-image --prompt "make the sky purple" --input-image https://example.com/photo.png
```

Every command supports `--help`, `--json`/`--agent` for machine-readable output,
and `--dry-run` to preview a request without sending it. Run `kie --help` for
the full command tree, or `kie api` to browse endpoints by name.

This CLI also ships an MCP server (`kie-pp-mcp`) and a local SQLite-backed
teach/recall loop (`kie recall`, `kie teach`) for agent use — see `kie
agent-context --pretty` for the machine-readable capability description.

## The Market model catalog

The 120+ marketplace models (Seedream, Ideogram, Kling, Wan, Hailuo, PixVerse,
ElevenLabs, and more) all share the same two endpoints, so they aren't each a
separate CLI command — instead, [`docs/MODELS.md`](docs/MODELS.md) lists every
known `--model` value by category. That catalog is a point-in-time snapshot;
see below for how it's kept current.

## Keeping this up to date

Kie.ai adds new models to the Market frequently and occasionally adds new
dedicated endpoints. Two things keep this repo from going stale:

1. **`scripts/weekly-refresh.sh`** re-fetches the Market catalog pages from
   docs.kie.ai, regenerates `docs/MODELS.md`, and commits+pushes if anything
   changed. It runs weekly via cron (Monday 03:07 local time — see `crontab -l`
   on the maintainer's machine). It does **not** touch the OpenAPI spec or
   regenerate CLI code — new dedicated endpoints (a new product family, not
   just a new Market model) still need a human to fetch the new doc page(s)
   and re-run the generation pipeline below.
2. For a full spec/CLI regen when Kie.ai ships new dedicated endpoints, first
   add the new page slug(s) to `DEDICATED_PAGES` in `research/build_spec.py`,
   then:
   ```bash
   cd research/
   python3 build_spec.py   # refetches everything, rewrites the spec + docs/MODELS.md
   cli-printing-press generate --spec kie-final-openapi.yaml --name kie \
     --spec-source docs --output .. --force
   ```
   then re-apply the patch in `.printing-press-patches/` if it still applies,
   and re-run `go build ./... && go test ./...`.

## Known limitations

- The Market catalog's per-model `input` shape isn't validated client-side —
  it's passed through as JSON (`--input` / `--stdin`), matching how loosely
  Kie.ai itself documents per-model parameters. Check `docs/MODELS.md` or the
  model's docs.kie.ai page for its exact input fields.
- Two Market pages (`wan/2-7-videoedit`, `z-image/z-image`) didn't serve a
  markdown mirror at fetch time and are missing from the catalog.
- Live API verification wasn't run (no API key was available during
  generation) — endpoints are structurally correct against the documented
  spec but haven't been smoke-tested against the real API. `path_validity`,
  `auth_protocol`, and `live_api_verification` show as unscored in the
  scorecard for this reason.

## Credits

- API and docs: [Kie.ai](https://kie.ai) — this is an unofficial, community
  client, not affiliated with or endorsed by Kie.ai.
- Generated with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
  by Matt Van Horn and Trevin Chow.

## License

Apache-2.0 — see [LICENSE](LICENSE).
