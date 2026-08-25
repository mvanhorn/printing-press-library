# SculptOK CLI Brief

## API Identity
- Domain: AI relief / depth-map / image-to-3D generation for 3D printing, CNC, laser engraving, bas-relief. Operator: MODERNFOX LTD.
- Users: 3D-printing hobbyists, CNC/laser makers, jewelers/bas-relief artists, ZBrush/Blender users (official plugins exist).
- Data profile: async "draw" jobs (depth-map, 3D, STL, bg/HD), each producing image/model URLs; a credit wallet shared with the web app; drawing history.

## Official API — YES (documented, key-authed REST)
- Doc: https://www.sculptok.com/api/apidoc (JS SPA; rendered & fully verified this run).
- Base URL: `https://api.sculptok.com/api-open/`  (separate host from the Cloudflare-protected www SPA).
- Auth: header `apikey: <key>` + `Content-Type: application/json`. Keys created in the logged-in dashboard, shown once.
- Envelope: `{code, msg, data}`, HTTP **always 200**. `code 0` = success; `401` Unauthorized; `10020` apikey empty; `10021` invalid key. (CLI must key errors off `code`, not HTTP status.)
- No OpenAPI/Swagger/Postman file exists -> internal YAML spec hand-authored from the verified contract (see discovery/api-contract-verified.md).

### Endpoint surface (9)
| Method | Path | Purpose | Cost (credits) |
|---|---|---|---|
| GET | /point/info | credit balance (data.point) | free |
| GET | /point/page | credit history (limit,page) | free |
| POST | /image/upload | multipart `file` -> data.src URL | free |
| POST | /draw/prompt | depth-map draw (imageUrl,style,hd_fix,optimal_size,extInfo,version,draw_hd) -> promptId | 10 / 15 pro2k / 30 pro4k |
| POST | /draw/hd/prompt | bg removal + HD restore (imageUrl,hdFix,removeBack) -> promptId | 2 |
| POST | /draw/3d/prompt | 3D draw (imageUrl,hd_fix=basic|standard|high) -> promptId | 10 |
| POST | /draw/stl/prompt | image->STL (image_url,width_mm,min_thickness,max_thickness,invert,scale_image) -> promptId | 3 |
| GET | /draw/prompt?uuid= | draw status (status,currentStep,position,imgRecords[3]) | free |
| GET | /image/page | drawing history (limit,page) | free |

Casing nuances (preserved verbatim): depth-map `imageUrl`+`hd_fix`; bg/hd `imageUrl`+`hdFix`; 3d `imageUrl`+`hd_fix`; stl `image_url`. Status uses query `uuid`. `/draw/prompt` is shared: POST=submit, GET=status.

## Reachability Risk
- None for the API. `api.sculptok.com` is plain **nginx, no Cloudflare, no challenge**. Verified this run: no key -> `{"code":10020}`; bad key -> `{"code":10021}`; clean JSON, HTTP 200, server: nginx.
- www.sculptok.com (marketing/web app) IS behind Cloudflare + SPA — not targeted. The web app's own backend (`/odyssey-api/`) is session-auth and NOT the public API; do not target it.
- Phase 1.9 reachability gate: PASS (401-style auth-required envelope with no key, which is expected).

## Top Workflows
1. **Image -> depth map** (headline): upload local image -> submit draw -> poll -> get 3 depth-map candidates. Core use for CNC/laser/bas-relief.
2. **Image -> printable STL**: upload -> submit STL with thickness/width/invert/scale -> poll -> STL URL. Direct 3D-print path.
3. **Image -> 3D model**: upload -> submit 3D draw -> poll -> model.
4. **Pre-process**: background removal + HD restoration before depth-mapping (2 credits, cheap, improves results).
5. **Account ops**: check credit balance / history before burning credits on draws.

## Table Stakes (vs the API + competitors)
- Every documented endpoint reachable from CLI (credits, upload, depth-map, bg/HD, 3D, STL, status, history).
- Submit->poll handled automatically (the API is async; raw endpoints alone force manual polling).
- All draw/STL params exposed as flags with the documented enums, ranges, defaults.
- Credit-cost transparency before each paid call (10/15/30/2/10/3).

## Data Layer
- Primary entities: `jobs` (draws: id/promptId, kind, status, params, input image, result URLs, credit cost, created), `credit_events` (history), `images` (uploaded src URLs).
- Sync cursor: drawing history (`/image/page`) + credit history (`/point/page`), paged.
- FTS/search: over jobs (by kind, status, remarks) and credit events.

## Competitors / positioning
- Relief/depth-map niche is almost entirely web-only (Reliefmod, ImageToStl, DepthR, VOXELASE DepthGen Pro). SculptOK is nearly unique among relief tools in having a real public API.
- Image-to-3D API leaders (Meshy, Tripo, Rodin) are the dev-experience bar: SDKs, OpenAPI, skill files. SculptOK has none of that — a polished CLI + MCP + local job store + offline search is the exact gap.

## User Pain Points
1. Credits burn fast (HD/4k/pro). A CLI that shows cost before each call and tracks spend locally directly addresses this.
2. No fine manual control of depth layers in the UI — CLI exposing all documented params (thickness, invert, scale, bit depth, version) gives power users more leverage.
3. Async UX friction (submit then wait/refresh) — a CLI that submits + auto-polls + downloads results is a real workflow win.

## Product Thesis
- Name: SculptOK CLI (`sculptok-pp-cli`).
- Why it should exist: First and only CLI/MCP for SculptOK. Turns the async, credit-metered web tool into a scriptable, agent-native pipeline: local image in -> depth map / STL / 3D out, with credit-cost preflight, automatic polling, a local SQLite job+credit store for offline search and spend tracking, and `--json`/`--select`/typed exit codes throughout.

## Build Priorities
1. Data layer: jobs + credit_events + images; sync from history endpoints.
2. Absorb: all 9 endpoints as typed commands (response_path: data), all params as flags.
3. Transcend: hand-built submit->poll "generate" workflows (depthmap/stl/3d/restore) that accept a LOCAL image path, upload, preflight credit cost, poll to completion, persist the job, and download/print result URLs; plus local job search, spend analytics, and batch.

## Generation Notes / risks
- Envelope: generator has no `code != 0` error detection; use `response_path: data` for reads, and hand-build the workflow commands with explicit `code` checks + typed exit codes.
- Multipart upload (`/image/upload` field `file`) is outside the scalar param set; the hand-built workflow commands own upload from a local path (sibling client in internal/sculptok/).
- Credits: live generation testing costs real credits; Phase 5 live smoke should use FREE reads (point/info, point/page, image/page, status) only, unless the user explicitly approves spending.
