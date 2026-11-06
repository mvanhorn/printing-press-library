# Tella browser-free lesson editing reprint

## API Identity
- Domain: Tella recorded video editing for course lessons.
- Users: Course creator Lavery, who records lessons in Tella and wants to review live previews on her phone; a course production assistant who edits recordings, manages retakes, and delivers MP4 plus captions; an automation agent that needs safe repeatable operations and rollback.
- Data profile: videos, clips, source media, transcript words/sentences, cuts, exports, playlists, webhook events; locally cached transcripts support offline search.
- Homepage: https://www.tella.com

## Top Workflows
1. Find a new recording, duplicate it for testing, inspect its timeline/transcript.
2. Run a safe lesson edit pass: snapshot, automatic zooms/crop/layouts/fillers/silences, propose retakes, apply house recipe captions/chapters/thumbnail; finish with a live preview link and stop for review.
3. After review export, poll status, download MP4 and SRT/VTT to a local delivery folder.
4. Inspect storyboard images and compact timeline; undo cuts from snapshots when a pass needs correction.
5. Batch transcript search and playlist workflows retain all prior capabilities.

## Table Stakes
The official Tella API and hosted MCP expose the current typed endpoint surface. Official tellahq/skills provides agent guidance. Descript and Loom offer transcript-based video editing and review workflows. Match the official API surface without cookie transport.

## Data Layer
- Preserve the existing SQLite store and sync/search interfaces and all 16 prior novel features.
- Keep transcript snapshots for reversible cuts; avoid silently replacing working state with auto-refresh.
- Lists are envelopes, empty lists store no synthetic rows, webhook filter types use comma lists.

## User Vision
Most wanted: live preview links and automatic edits. The seven command groups in the user brief are required, not optional brainstorming. No publishing or social-account mutations. Write BROWSER-ONLY.md from docs/spec evidence.

## Reachability Risk

The official public HTTPS API uses bearer authentication through TELLA_API_KEY. Validation writes were limited to owned test copies, with explicit paid consent, bounded credit spend and cleanup evidence.

## Build Priorities
1. Preserve prior novel commands and all patch contracts, including cherry-pick 9113e8e6de.
3. videos preview; videos auto; lessons pass/ship; clips find-retakes; agent-chats start/list; videos storyboard-image/timeline; Idempotency-Key on every write.
4. Live matrix for every leaf command and MCP tool, explicit skip reasons for unsafe operations, cleanup proof, export/caption artifact validation.

## Current Evidence
- Official OpenAPI fetched HTTP 200; 157 operations, 40 generated resource groups.
- GET /v1/account with vault key returned HTTP 200; initial credit balance 9000.
- Official tellahq/skills has tella, auto-cut, remove-mistakes, auto-layouts, shorts and motion guidance. Auto-layouts uses manual layout operations plus visual judgment, no public automatic-layouts endpoint. Keep --layouts fail-closed with that explanation.
- Melvynx/tella-cli README claims auto-layout endpoints, but they are absent from the current canonical API. Do not regenerate dead calls based on those examples.
- Repository issues fully enumerated in one page: tellahq/skills 27 (<100) and Melvynx/tella-cli 0; no current inaccessible-API signals found.
- Docs: https://www.tella.com/developers, https://www.tella.com/features, https://www.tella.com/mcp. Official MCP is closed server implementation; public skill source read directly.
