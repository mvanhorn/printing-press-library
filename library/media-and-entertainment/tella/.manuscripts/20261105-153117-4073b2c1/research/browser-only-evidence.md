# Browser-only boundary evidence (2026-11-05)

Official feature catalog fetched HTTP 200 from https://www.tella.com/features. Parsed the official OpenAPI (157 operations); do not confuse absent generated wrappers with absent APIs. The editor has APIs for captions, translations, Studio Sound, eye contact, filters/LUTs, clip transitions, manual layouts/crop/zooms, overlays, text, music, chapters, thumbnails, motion, imports, export, CTAs, password/link scope, comments-enabled, downloads-enabled and searchEngineIndexingEnabled. Those are NOT browser-only.

Document actual gaps conservatively:
- Capturing a person's screen, camera and microphone and displaying speaker notes uses the web/desktop recorder. POST recording-link hands the person to that recorder; website Auto Capture is a different API capability.
- One-shot auto-layout selection has no endpoint. Official tellahq/skills tella-auto-layouts composes list/update/add/remove layouts with frame inspection; the CLI must not claim a dedicated auto-layout API. Existing manual API operations can automate a supplied plan.
- Saved/Favorite layout management has no public route in the current spec. Clip-level layouts are fully exposed and should remain available.
- Viewer comment content/replies/moderation lack endpoints; toggling commentsEnabled/commentEmailsEnabled IS exposed.
- Workspace-wide brand/logo/custom-domain management, device recorder settings and account/team/billing administration have no public mutation route. GET account reports plan/credits and billingURL, not settings management.
- Full editor undo/redo history has no endpoint. CLI snapshots can restore the specific recorded state they contain; describe exact limits.
- No direct semantic remove-mistakes endpoint or one-shot AI title endpoint. Official agent-chats can be requested to handle natural-language work at credit cost, so label these as Agent fallback, not unconditionally browser-only. Deterministic retake detection is narrower.

Sources: https://www.tella.com/features; https://api.tella.com/openapi.json; https://github.com/tellahq/skills/blob/main/skills/tella/SKILL.md; https://github.com/tellahq/skills/blob/main/skills/tella-auto-layouts/SKILL.md. Scope statement: public docs/spec inventory, not exhaustive proof of every undocumented editor interaction.
