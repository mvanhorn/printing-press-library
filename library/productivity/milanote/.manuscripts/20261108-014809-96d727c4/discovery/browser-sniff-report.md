# Milanote browser-sniff report (2026-11-08)

## 1. User Goal Flow
- Goal: build and organise a board — create a board, add notes, headings, lists, to-dos and images at exact canvas positions, move them, read the board back.
- Steps completed (all on a throwaway board "CLI sandbox" on Home):
  1. Navigate Home (SPA) → USER_NAVIGATE + `GET /api/boards?ids=` + `GET /api/boards/modified?ids=`
  2. Drag Board tool → ELEMENT_CREATE BOARD; title → ELEMENT_UPDATE {title, icon, color}; `GET /api//icons?search=`
  3. Open board → `GET /api/users/me/custom-templates-init`
  4. Note → ELEMENT_CREATE CARD {textContent:null} then ELEMENT_UPDATE textContent (tiptap/ProseMirror doc)
  5. Heading → ELEMENT_CREATE CARD {textContent: heading level 1 textAlign center, transparent.enabled:true, background:null}
  6. Drag note → ELEMENT_MOVE_MULTI {moves:[{id, location{parentId,section:CANVAS,position{x,y,score}}, from}], moveOperation:DROP}
  7. Drop image file → ELEMENT_CREATE IMAGE {image:null}; `POST /api/elements/attachments/presign` → S3 PUT (presigned) → ELEMENT_UPDATE {image{width,height,original,transparent}, file{type,ext,mime,filename,size,lastModified}}
  8. To-do → ELEMENT_CREATE TASK_LIST {version:V2, textContent: taskList/taskItem}
  9. Rich text toolbar → bulletList/listItem, orderedList, marks bold, callout {icon, iconCode}; text styles: Large heading (h1), Normal heading (h2), Small text, Code block, Callout, Quote, colour, highlight.
- Secondary: search, history, labels, trash, counts, permissions (read-only GETs).
- Coverage: 9/9 planned steps.

## 2. Pages & Interactions
- /BOARD_ID/ellis--morrow → clicked "Home" breadcrumb (SPA)
- /BOARD_ID/home → dragged Board tool, typed "CLI sandbox"
- /BOARD_ID/cli-sandbox → note, heading (… → Heading), note drag, image drop, list note, to-do, text-style menu

## 3. Browser-Sniff Configuration
- Backend: CDP into the user's already-running Chrome through the long-lived browser.mjs relay (chrome-MCP-equivalent; existing Milanote tab only). Page-context interceptors wrapped fetch, XMLHttpRequest and WebSocket.send/message; no navigation of other tabs.
- Pacing: ~1 req/s for the read probes; no 429s.
- Proxy-envelope: not detected. Writes are socket.io `action` events, not REST.

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| GET | /api/users/me | 200 | json | auth-required (401 without) |
| GET | /api/boards?ids=&excludeSelf=&loadAncestors= | 200 | json | auth-required |
| GET | /api/boards/modified?ids= | 200 | json | auth-required |
| GET | /api/boards/search?query=&offset=&sort=&locale= | 200 | json | auth-required |
| GET | /api/users/me/history | 200 | json | auth-required |
| GET | /api/labels | 200 | json | auth-required |
| GET | /api/users/me/trash?other-users=false | 200 | json | auth-required |
| GET | /api/users/me/counts | 200 | json | auth-required |
| GET | /api/permissions/board/{id} | 200 | json | auth-required |
| GET | /api//icons?search=&queryType=&count= | 200 | json | auth-required |
| GET | /api/users/me/custom-templates-init | 200 | json | auth-required |
| POST | /api/elements/attachments/presign | 200 | json | auth-required |
| GET/POST | /socket.io/?userId=&EIO=4&transport=polling[&sid=] | 200 | text (Engine.IO 4) | auth-required |
Known from cotletdev/milanote-api-mcp-server (not re-captured): POST /api/elements/duplicate, PUT/DELETE /api/elements/{id}/publish, /api/elements/{id}/permissions(-invite), GET /api/users?ids=, POST /media-service/api/export/{id}.

## 5. Traffic Analysis
- Protocols: rest_json (0.75) for reads; socket.io (Engine.IO 4) `42<ack>["action",{…}]` → `43<ack>[{"status":200}]` for every write. Browser uses the websocket transport; the server also accepts long-polling, which replays from plain HTTP.
- Auth signals: cookie `__Secure-milanote.session_token` (HttpOnly, ~1 month). AWSALB/AWSALBCORS are load-balancer stickiness cookies: a polling sid only works when the follow-up requests carry the AWSALB value set by the handshake response (400 otherwise). `aws-waf-token` present in browser but NOT required for API replay.
- Action envelope fields added to every write: sync:true, user{_id,clientId,clientTick}, tokens:[], deviceId, sessionId, channels:[boardId]. Element ids are client-generated: base62(unix seconds) + clientId (from /api/users/me) + base62(tick).
- Canvas units: position {x,y} in grid units (~10 css px at 100% zoom); `score` orders elements; section CANVAS|INBOX|TRASH.
- Generation hints: requires_browser_auth=false at runtime (cookie import once); has_rpc_envelope for writes (hand-built client).
- Warnings: sniffed spec merged `q`/`query` on search (only `query` is real); `/api//icons` has a double slash upstream.

## 6. Coverage Analysis
- Exercised: boards, cards (note/heading/list/callout), images, to-dos, moves, search, history, labels, trash list, counts, permissions.
- Not exercised live: link cards (url unfurl), columns, comments, sharing/publish, export, duplicate, card width resize, lines/arrows, sketch, colour swatch, document cards. Shapes for links/columns/comments/sharing/export/duplicate come from the cotlet project's recorded calls.

## 7. Response Samples
See `research/milanote-browser-sniff-spec-samples/` (personal fields redacted: email, names, tokens, icon svg).

## 8. Rate Limiting Events
- None. ~1 req/s effective for probes.

## 9. Authentication Context
- Authenticated browser-sniff using the user's existing logged-in Chrome tab (Google SSO account).
- Cookie replay (Step 2d): `/api/users/me` with only the session cookie → 200; without → 401. PASS. Socket.io polling write with session cookie + AWSALB jar → ack status 200 and the change persisted. PASS.
- Session cookie values were held only in $SESSION_DIR (0600, outside the run dir) and never written to discovery/research artifacts.
