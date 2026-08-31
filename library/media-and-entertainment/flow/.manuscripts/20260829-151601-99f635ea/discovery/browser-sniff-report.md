# Flow Browser-Sniff Report

**Capture method:** browser-use, headed, authenticated with the user's real Google account, against the user's real Flow project (`labs.google/fx/tools/flow/project/b9a189f8-...`).
**Primary goal walked:** add existing image assets to a project and generate an animated clip from them (image-to-video).
**Cost incurred:** one real Veo generation, 15 credits, approved explicitly by the user.

## What was confirmed

1. **Reachability:** `probe-reachability` on the app shell returns `browser_clearance_http` (reCAPTCHA present). The private API host `aisandbox-pa.googleapis.com` itself answers plain HTTP with no bot-protection at the bare-host level.
2. **Auth scheme:** all `aisandbox-pa.googleapis.com` calls carry `Authorization: Bearer ya29....` — a **standard Google OAuth2 access token**, minted client-side from the logged-in session. Same-origin `labs.google/fx/api/trpc/*` reads ride on ordinary session cookies.
3. **Split surface — reads vs. writes:**
   - **Reads/status are clean, replayable REST/tRPC calls with no fresh challenge**: `GET /v1/credits`, `POST /v1/video:batchCheckAsyncVideoGenerationStatus`, `PATCH /v1/flowWorkflows/{id}`, `GET /fx/api/trpc/project.searchUserProjects`, `general.fetchUserPreferences`, `general.fetchUserAcknowledgement`.
   - **All generation-triggering actions** (submit prompt+ingredients, approve the credit-spend confirmation, even a plain-language "what's the status?" chat message) go through **one conversational endpoint**, `POST /v1/flowCreationAgent:streamChat?alt=sse`, and **every single call to it** was immediately followed by a fresh `POST https://www.google.com/recaptcha/enterprise/clr` token mint. This matches independent community prior art (`ffroliva/gflow-cli`'s README: pure-HTTP video submission "returns HTTP 401 under non-Chrome browsers plus a reCAPTCHA... cannot reproduce headlessly").
4. **No in-app Google Drive picker.** "Upload media" opens a native OS `<input type=file>` dialog (two variants: image-only, and image+video). There is no embedded Google Picker / Drive browser inside Flow's own UI — this is exactly the friction the user described.
5. **No user-audio import path, anywhere.** The upload accept-lists have zero audio MIME types. The "Voices" tab in the asset picker is a **text-to-speech voice caster** (named voices + a typed "Sample Dialogue" box), not an audio-file attach point. Veo's native audio is model-generated (ambience + lip-synced dialogue from the prompt), not a sync target for an external track.

## Endpoints discovered

| Endpoint | Method | Host | Auth | Recaptcha? | Verdict |
|---|---|---|---|---|---|
| `project.searchUserProjects` | GET | labs.google (tRPC) | cookie | No | Replayable |
| `general.fetchUserPreferences` | GET | labs.google (tRPC) | cookie | No | Replayable |
| `general.fetchUserAcknowledgement` | GET | labs.google (tRPC) | cookie | No | Replayable |
| `/v1/credits` | GET | aisandbox-pa | Bearer | No | Replayable |
| `/v1/video:batchCheckAsyncVideoGenerationStatus` | POST | aisandbox-pa | Bearer | No | Replayable |
| `/v1/flowWorkflows/{id}` | PATCH | aisandbox-pa | Bearer | No | Replayable |
| `/v1/flowCreationAgent:streamChat` | POST (SSE) | aisandbox-pa | Bearer | **Yes, every call** | Not replayable outside a live browser |
| `/v1/flow:batchLogFrontendEvents` | POST | aisandbox-pa | Bearer | No | Telemetry, skip |

## Implication for CLI scope

- **Ship fully:** local project/asset/character/scene listing and sync, credit balance, generation status polling/watch, and — the CLI's actual differentiator — pulling images (and eventually the audio transcript) directly from **Google Drive via the official Drive API** instead of the local-download-then-reupload dance the web UI forces.
- **Cannot ship as silent automation:** actually triggering a new generation. The generation-agent endpoint requires a fresh, live-page-context reCAPTCHA Enterprise solve on every turn; there is no reusable harvested credential that satisfies it headlessly. Per Printing Press's own rule, a resident hidden browser standing in for this is not an acceptable shipped transport.
- **Recommended shape for the create step:** the CLI prepares everything (stages Drive assets into the project, writes the per-scene prompt derived from the audio transcript) and then opens the user's real browser at the exact project/prompt state, ready for one manual click of "Approve" — automating the two tedious parts (asset wrangling + prompt authoring) while leaving the actual credit-spend trigger a transparent, ToS-honest human action.
- **Audio-drama workflow must be scoped honestly:** Flow cannot ingest or lip-sync to the user's existing mp3. The CLI's real value here is (a) transcribing/segmenting that mp3 into scene-timed beats, (b) turning each beat into a Flow prompt against the matching seed image, (c) driving/queuing those generations, and (d) muxing the user's real audio track back onto the resulting clips locally with ffmpeg. This must be stated plainly to the user before they approve the build.
