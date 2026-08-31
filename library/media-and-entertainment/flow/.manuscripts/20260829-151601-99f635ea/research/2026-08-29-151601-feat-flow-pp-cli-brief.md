# Flow CLI Brief

## API Identity
- Domain: Google Flow (`labs.google/fx/tools/flow`) — Google's AI filmmaking studio built on Veo 3.1, Nano Banana Pro (image), and Gemini Omni Flash. Internal codename "Pinhole" (seen in tRPC `toolName=PINHOLE`).
- Users: Google AI Pro/Ultra subscribers producing short AI-generated video/image content from text, image references ("Ingredients"), frame pairs ("Frames to Video"), and Scenebuilder-arranged multi-clip sequences.
- Data profile: user-owned projects, each containing images/videos/characters/scenes/collections; generation is credit-metered (10-100 credits/video depending on Veo tier); no official public API for Flow itself (separate from the documented Gemini API/Veo model access, which this CLI does NOT target — it targets the Flow product surface specifically, per the user's request).

## Reachability Risk
- **Split.** Read/status/list surface: **Low risk**, plain HTTPS with a harvested Google OAuth2 Bearer token (`ya29.*`) or session cookies, no bot-protection observed on any read call.
- Write surface (triggering new generations): **High risk / not headlessly replayable.** Every call to the single generation-agent endpoint (`flowCreationAgent:streamChat`) triggers a fresh reCAPTCHA Enterprise challenge client-side. Confirmed independently by community prior art `ffroliva/gflow-cli` (reverse-engineered, alpha, keeps a persistent headed Playwright Chromium as its actual transport because "Google's auth and reCAPTCHA gates require it").
- Evidence: see `discovery/traffic-analysis.json` and `discovery/browser-sniff-report.md` (live authenticated capture against the user's real project, including one real approved 15-credit test generation that completed successfully end-to-end).

## Top Workflows
1. Sync/list a project's assets (images, videos, characters, scenes) locally for fast offline search — reduces trips to the slow web asset grid.
2. **Pull seed images (and eventually voice/transcript material) directly from Google Drive** and stage them into a Flow project — this is the CLI's headline differentiator; Flow's own "Upload media" is a native OS file picker with no Drive integration at all.
3. Watch/poll an in-flight generation's status and credit cost without babysitting the browser tab.
4. Check remaining Flow/Veo credit balance before deciding whether to submit a batch of generations.
5. Auto-draft per-scene prompts from a source script/transcript (the user's actual ask: reduce hand-prompting tedium for an audio-drama-driven storyboard), then hand the user a ready-to-approve queue.

## Table Stakes (from community prior art — see absorb manifest)
- Text/image/frame-to-video generation triggers, batching, character consistency ("mint reusable subjects"), multi-scene manifests, credit-free local stitching, MCP server exposure for agent integration (all from `gflow-cli`).

## Data Layer
- Primary entities: projects, media assets (images/videos), characters, scenes/collections, generation jobs (workflow id + status), credit ledger.
- Sync cursor: tRPC `project.searchUserProjects` uses a `cursor`-based pagination; asset lists likely follow the same pattern (needs confirmation against a `media.list`-style endpoint in Phase 3 discovery-continuation).
- FTS/search: local SQLite mirror of project/asset metadata, searchable offline — this alone beats the web UI, which requires live round-trips for every filter/sort.

## Codebase Intelligence (from community reverse-engineering, not official docs)
- Source: `ffroliva/gflow-cli` (unofficial, alpha, MIT-ish, Python/Playwright)
- Auth: headed-Chrome-only Playwright profile; private endpoint host `aisandbox-pa.googleapis.com` confirmed independently by our own capture.
- Architecture: production transport is full UI automation; experimental pure-HTTP strategies (`evaluate_fetch`, `bearer`, `sapisidhash`) exist but are "off the main path" because the team could not reproduce the reCAPTCHA-gated video-submission path headlessly — matches our own finding exactly.
- Rate limiting: `flowkit` (another community project) mentions 10-second cooldowns between calls; apply adaptive backoff regardless.

## User Vision
- User is trying to animate a **dramatic audio play (existing mp3) with images as seed material**, and is frustrated by (a) hand-prompting every shot and (b) how slow/manual Flow's Google Drive asset picking is (there isn't one — see Reachability/Audio findings below).
- **Critical scoping finding, surfaced now rather than discovered mid-build:** Flow has no mechanism, web or API, to import or lip-sync to a pre-existing external audio file. "Voices" in the asset picker are TTS character voices (typed dialogue + named voice presets), not an audio upload point; Veo's native audio is model-generated, not externally synced. The user's actual workflow must therefore be: transcribe/segment their real mp3 into scene-timed beats → auto-draft a Flow prompt per beat against the matching seed image → drive/queue those generations → **locally mux the user's real mp3 back onto the resulting silent/Veo-audio clips with ffmpeg**. This is explicitly NOT "upload mp3 to Flow and get synced video back" — that feature does not exist anywhere in the product.

## Product Thesis
- Name: `flow-pp-cli` (working name; will refine display name in narrative research)
- Why it should exist: Flow's web UI has no Drive integration, no batch/scriptable prompting, and no offline search over a user's own project library. This CLI absorbs every community-proven workflow from `gflow-cli`/`flowkit` (character consistency, batching, multi-scene manifests, MCP exposure) and adds what none of them have: direct Google Drive ingestion and an audio-drama-to-storyboard pipeline that turns a transcript into a ready-to-approve generation queue — while being honest that the final "spend credits" click stays a transparent, user-driven action because of Google's reCAPTCHA gate on that specific step.

## Build Priorities
1. Read/sync/search data layer: projects, assets, characters, scenes, credits, generation status (all replayable, no browser needed at runtime).
2. Google Drive ingestion: pull files by folder/query directly into local staging using the official Drive API (separate, well-documented, standard OAuth — not reverse-engineered).
3. Audio-drama transcript → scene-prompt pipeline + local ffmpeg mux-back (the actual differentiator for the user's stated goal).
4. Semi-automated "prepare and hand off" flow for the generation-submission step (opens the browser at the ready-to-approve state; does not silently automate the reCAPTCHA-gated action).
