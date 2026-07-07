# OpenArt CLI Brief

## API Identity

- Domain: AI generative-media SaaS (text-to-video, text-to-image, edit, character/world consistency, lip-sync, motion-sync, audio).
- Aggregator: wraps 100+ underlying models (Seedance 2.0/BytePlus, Veo 3, Kling, Hedra, OmniHuman, InfiniteTalk, FLUX, SDXL, Stable Diffusion, etc.) behind one OpenArt credit wallet.
- Users: solo creators, indie filmmakers, marketers running paid OpenArt plans (Free/Starter/Hobbyist/Pro/Infinite). Target user has 43,356 credits and is on Infinite plan.
- Data profile: per-user generation history (videos, images, audio), per-tool credit costs, custom characters, custom worlds, trained LoRAs, projects, folders, labels.

## Reachability Risk

- **HIGH**. OpenArt has zero public API documentation. The Help Center explicitly states "no public API is currently available." Pro-plan API access is rumored in third-party reviews but unconfirmed by OpenArt. The CLI must fully reverse-engineer the web app.
- Cloudflare presence is likely; will be classified by `printing-press probe-reachability` and the Phase 1.7 browser-sniff capture.
- Brittleness: any frontend redeploy could rename hashed GraphQL operations or rotate auth headers. Mitigation: cookie-based auth (long-lived browser session), user-driven `auth login --chrome` re-import on breakage, version-pinned operation hashes captured at sniff time.
- Legal posture: this is the user's own paid account, executed on their behalf, against their workspace. Same posture as any browser automation of one's own paid SaaS. The CLI is a personal-account assistant, not a scraper.

## Top Workflows (verified from user briefing + product surface)

1. **Generate video with Seedance 2.0** — text prompt + optional reference images, choose duration/aspect/resolution, count, audio on/off, then poll a long-running job (~5-10 min) and produce a hosted URL or local download. THE headline.
2. **Generate image** — text-to-image with model selection (FLUX, SDXL, etc.), bulk count, references.
3. **Edit existing image** — inpainting, background remove/change, hand/face fix, upscale, object replace.
4. **Lip-sync video** — pick a model (OmniHuman, InfiniteTalk, Hedra, Kling), provide source video + audio.
5. **Motion-sync video** — apply movement from reference clip onto a target subject.
6. **Sync media library + search** — list/search/download my generation history offline.

## Table Stakes (what every OpenArt power user expects)

- Video generation: text-to-video, image-to-video, frame-to-frame (start/end), elements-to-video (visual references), restyle, extend, upscale.
- Image generation across model catalog with consistent prompt + reference + bulk-count interface.
- Edit image: inpaint, bg-remove, bg-change, hand fix, face fix, upscale, object replace.
- Edit video: lip-sync, motion-sync, replace character, restyle, extend.
- Async job polling with progress + final-URL/download.
- Credit balance + per-job cost preview.
- Media library: list, filter by tool/model/date, download, delete.
- Character + World assets: create, list, reuse in generations.
- Auth: login via Chrome cookie import (Google OAuth → cookies); no API key path exists.

## Data Layer (SQLite, the engine of transcendence)

- Primary entities:
  - `media` — every generated artifact (id, type=video/image/audio, model, prompt, references, params, output_url, local_path, credits_spent, status, created_at, project_id, labels)
  - `jobs` — long-running generation jobs (id, type, model, params, status, progress, started_at, completed_at, media_id, error)
  - `models` — catalog (slug, family=video/image/audio, vendor=byteplus/runway/kling/etc., url-route, credits_per_unit_default, supports={text,image,video,audio refs}, max_duration, resolutions, last_seen_at)
  - `characters` — saved characters (id, name, ref_images, created_at)
  - `worlds` — saved worlds (id, name, ref_images, created_at)
  - `projects` — workspace projects (id, name, created_at)
  - `folders` / `labels` — organizational
  - `credits_ledger` — every spend event derived from job costs (date, model, cost, balance_after)
- Sync cursor: per-resource `updated_at` for incremental media/job/character pulls.
- FTS: `media_fts` over (prompt, label, model, project_name).

## Codebase Intelligence

- No SDK, no MCP, no community wrappers exist for OpenArt as of 2026-05-13. This is a green-field reverse-engineer.
- Underlying inference vendors are well-documented:
  - Seedance 2.0 → BytePlus ModelArk: https://docs.byteplus.com/en/docs/ModelArk/1520757 (gives us schema clues for prompt/duration/resolution/aspect inputs)
  - Lip-Sync: Hedra, Kling, OmniHuman, InfiniteTalk are all separately documented vendors that OpenArt is wrapping.
- Inferring spec shape from user briefing screenshot: route convention `/suite/<family>/<vendor>-<model>` (e.g. `/suite/create-video/byte-plus-seedance-2`). Model selection is reflected in the URL — useful for browser-sniff to enumerate the model catalog.

## User Vision (from briefing)

> "I have an OpenArt account, I want to be able to programmatically spend my OpenArt credits like I would if they had an API to generate videos. In particular Seedance 2 right now. I want to be able to from the CLI 'generate a video using seedance 2 that does this for 10 seconds, give me 2 videos' and it'll do that, link me to the URL or download locally. Heads up usually takes 10 min to generate. My focus is Seedance 2 but it offers LOTS of different things so we should pull in other high-value areas."

Concrete asks distilled:
- One-shot CLI command for text-to-video w/ Seedance 2 (prompt, duration, count → URL or local file).
- Async polling with progress visibility (~10 min default).
- Credit-aware: cost preview before submit, spend tracking afterward.
- Multi-modal scope: image, edit-image, edit-video, lip-sync, motion-sync, character, world, audio.
- Logged-in browser session is the auth surface; AUTH_SESSION_AVAILABLE=true.

## Source Priority

Single-source CLI. OpenArt is the only target. Browser-sniff is mandatory and was pre-approved at Phase 0.

## Product Thesis

- **Name:** `openart-pp-cli` (binary), `openart` (subcommand-friendly slug for prose).
- **Why it should exist:** OpenArt is the cheapest and broadest gateway to BytePlus Seedance + Kling + Veo + Hedra + 100+ models behind a single credit balance. There is no other way to spend an OpenArt credit balance programmatically. Every alternative (Replicate, Fal.ai, BytePlus ModelArk direct, Higgsfield) means a different vendor, a different credit balance, and a different per-second price. For a creator with thousands of credits already paid for, an OpenArt CLI converts a ~10-minute browser-tab-and-wait workflow into a fire-and-forget terminal command, with offline media-library search and credit accounting that the OpenArt UI itself doesn't expose well.

## Build Priorities

1. Browser-sniff the Seedance 2 video-gen submit + poll + download flow against the user's real session. This is the make-or-break dependency.
2. Generalize the discovered video-gen contract to all OpenArt video tools (model-routed POST + job-poll envelope is almost certainly uniform).
3. Image generation, character + world list/create, media library list/sync/download.
4. Edit-image, edit-video, lip-sync, motion-sync as model-routed siblings of (1)-(2).
5. Transcendence: credit forecasting, batch queue, watch-and-notify, local-mirror sync, prompt history search.

## Competitor Landscape (for absorb manifest)

| Tool | Surface | Coverage |
|---|---|---|
| Replicate CLI (`replicate`) | Generic model runner over Replicate's catalog | Predictions, models, hardware, training. Different credit pool. |
| `replicate` npm | TS/Node SDK | Same as above. |
| `fal-client` (PyPI), `@fal-ai/client` (npm) | Generic model runner over Fal.ai | Different credit pool. |
| Fal MCP server (`fal-image-video-mcp`) | MCP for Claude Desktop | Different credit pool. |
| Higgsfield MCP (30+ models incl. Seedream/Veo/Kling) | MCP | Different credit pool. |
| Replicate MCP | MCP | Different credit pool. |
| MiniMax MCP | MCP for MiniMax models | Different credit pool. |
| BytePlus ModelArk REST | Direct Seedance 2.0 | Single-vendor, different account. |
| OpenArt itself (web only) | Full surface | No CLI, no SDK, no MCP. |

The absorb opportunity: every competitor provides per-model invocation, async polling, history listing, account/credit display. None provides them against OpenArt. We absorb their interaction patterns and pair them with the OpenArt account the user already pays for.

## Sources

- OpenArt website: https://openart.ai/, https://openart.ai/video, https://openart.ai/help, https://openart.ai/suite/create-video/byte-plus-seedance-2, https://openart.ai/suite/lip-sync
- BytePlus Seedance 2.0 docs: https://docs.byteplus.com/en/docs/ModelArk/1520757, https://www.byteplus.com/en/topic/537367
- Replicate CLI: https://github.com/replicate/cli, https://replicate.com/docs/reference/mcp
- Fal.ai: https://fal.ai/, https://docs.fal.ai/model-apis/quickstart/
- Fal MCP (community): https://github.com/RamboRogers/fal-image-video-mcp
- Higgsfield MCP: https://higgsfield.ai/mcp
- OpenArt Seedance 2.0 blog: https://openart.ai/blog/how-to-use-seedance-2-0/, https://openart.ai/blog/seedance-2-0-handbook/
