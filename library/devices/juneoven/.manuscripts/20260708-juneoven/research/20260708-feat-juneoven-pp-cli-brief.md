# Juneoven CLI Brief

## API Identity
- Domain: June smart oven, controlled over June's (Weber-owned) cloud. Reverse-engineered protocol: REST for token/status, Ed25519-signed WebSocket for commands, SRP-6a self-pairing. No official API.
- Users: home cooks and homelab/automation owners of a June oven who want control outside the June phone app.
- Data profile: live cavity-temperature telemetry (streamed ~1/s during a cook), interior camera frames (signed image URLs), cook plans/presets with steps, device state (idle/active), connection state. Cloud exposes only LIVE state — there is no history endpoint.

## Reachability Risk
- Low for a paired owner (verified live end-to-end against a real oven). Strategic risk: Weber acquired June and the cloud's future is uncertain; owners fear the app/cloud being sunset. This raises the value of owning your own client and your own local record of cook data.

## Users (personas grounded in research)
- **The homelab automator.** Requested a June integration on the Home Assistant forum (2021) and never got one; runs homebridge-june-oven or nothing. Wants the oven in scripts and agents, JSON-native, and a hedge against Weber killing the cloud. Explicit community demand: "expose oven setpoint and current temperature" to automation.
- **The weekly baker.** Runs the same few bakes repeatedly (sourdough, roasts, reheats). Taps the same preheat on the touchscreen or app every time; wants repeatable, named cooks and to be told the moment it's actually up to temperature instead of guessing.
- **The agent operator.** Drives the oven from a terminal / Claude Code / Hermes. Wants every action as JSON with an ack, and wants to ask "what is the oven doing" and "how long until ready" programmatically.

## Top Workflows (named rituals)
1. **Preheat-and-wait**: start a preheat, then stand around guessing when it's ready. June's own "ready" signal is app-side; from a terminal there is none today.
2. **Repeatable cook**: run the identical mode+temp (+timer) they run every week, then cancel or let it finish.
3. **Remote watch**: monitor an active cook (temperature climbing, camera) without standing at the oven.
4. **Glance**: "is the oven on / what's it set to" from wherever they are.
5. **After-the-fact review**: "how long did that roast take / how fast does my oven preheat" — impossible today because the cloud keeps no history.

## Reachability / hardware context (evidence)
- June hardware breaks often; owners report 2nd/3rd replacement units (Trustpilot). A local cook log survives oven swaps and cloud changes.
- No official or community Home Assistant integration exists despite a standing request; homebridge-june-oven is the only third-party control path, and it is HomeKit-only.

## Data Layer
- Primary entities: cook sessions (mode, target, start, end, outcome), telemetry samples (timestamp, cavity temp, progress) tied to a session, presets/cook-plans, camera frames.
- Sync cursor: telemetry is push-only over WS during a cook — captured by a watch/record path, not a REST sync.
- FTS/search: over logged cook sessions.

## Product Thesis
- Name: juneoven-pp-cli
- Why it should exist: the only agent-native, no-HomeKit, no-app June control, and the only tool that gives owners a durable LOCAL record of their oven's behavior (cook history, preheat performance, temperature curves) that June's live-only cloud never keeps — a hedge against unreliable hardware and an uncertain post-acquisition cloud.

## Build Priorities
1. Direct control (pair/status/preheat/temp/timer/cancel) — done, live-verified.
2. Remote observability (watch telemetry + camera) — done.
3. Local record + derived insight nobody else has (cook log, preheat stats, ready-wait, temperature-curve export).
