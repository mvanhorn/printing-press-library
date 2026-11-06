# Tella browser boundary

Based on the official public docs and the 157-operation OpenAPI inventory researched on 2026-11-05. Absence from this inventory is not exhaustive proof about every undocumented editor interaction.

| Feature | Boundary / alternative |
| --- | --- |
| Recording a person's screen, camera, microphone; speaker notes and device selection | Use the web/desktop recorder. Recording links hand the person to that recorder; website Auto Capture is a separate API capability. |
| One-shot automatic layout selection | No public auto-layout endpoint. Use manual layout operations with frame inspection and a reviewed storyboard plan, or an explicit Agent request. The CLI's --layouts fails before writes. |
| Saved/Favorite layout management | No public route; clip-level layouts remain fully available through the API. |
| Viewer comment contents, replies and moderation | No public endpoints. commentsEnabled/commentEmailsEnabled settings are available. |
| Workspace brand/logo/custom domains, recorder settings, account/team/billing administration | No public mutation routes. GET account reports plan, credits and billingURL. |
| Full editor undo/redo history | No endpoint. CLI cut snapshots restore recorded cuts with divergence checks; crop, zoom, styling and inserted media require manual recovery from reference state. |
| Semantic mistake removal or a one-shot AI title | No dedicated endpoint. Official agent-chats is a paid fallback with credit checks and explicit consent. Deterministic find-retakes is narrower and requires review. |

Captions, translations, Studio Sound, eye contact, LUTs/filters, transitions, manual layouts/crop/zooms, overlays/text/music, chapters/thumbnails, motion, imports/exports, CTAs, link/password access and share-page settings have API endpoints; they are not browser-only.

Sources: [Tella features](https://www.tella.com/features), [official OpenAPI](https://api.tella.com/openapi.json), [official Tella skill](https://github.com/tellahq/skills/blob/main/skills/tella/SKILL.md), [official automatic-layout guidance](https://github.com/tellahq/skills/blob/main/skills/tella-auto-layouts/SKILL.md). The public API boundary was researched; the editor UI was not exhaustively checked. Live validation exercised preview, editing, export and storyboard workflows.
