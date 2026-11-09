# Milanote crowd-sniff report (2026-11-08)

Approval: user said on 2026-11-08 09:54Z: "if there already is an open source CLI available we can definitely just use that or build upon it".

1. npm Packages Analyzed — none relevant (no Milanote SDK on npm).
2. GitHub Repos Searched — `cli-printing-press crowd-sniff --api milanote` (authenticated gh token): 35 code-search hits, all false positives (Homebrew cask JSON, unrelated `/api/*` apps). Manually mined instead: cotletdev/milanote-api-mcp-server (Python, Elastic-2.0, last commit 2026-11-05, 30 tools, recorded 2026-10-18), serhanceetin/Milanote-Unofficial-API (Python, 2023, stale), setobox/milanote-api (TS worker, public share links only).
3. Endpoints Discovered (manual, cotlet source; community-sdk tier):
| Method | Path | Source |
|---|---|---|
| GET | /api/users/me | cotlet + sniff |
| GET | /api/boards?ids= | cotlet + sniff |
| GET | /api/boards/search | cotlet + sniff |
| GET | /api/users/me/history | cotlet + sniff |
| GET | /api/labels | cotlet + sniff |
| GET | /api/users/me/trash?other-users=false | cotlet + sniff |
| GET | /api/users/me/counts | cotlet + sniff |
| GET | /api/permissions/board/{id} | cotlet + sniff |
| GET | /api/users?ids= | cotlet |
| POST | /api/elements/duplicate | cotlet |
| PUT/DELETE | /api/elements/{id}/publish | cotlet |
| POST | /api/elements/{id}/permissions, /permissions-invite | cotlet |
| POST | /media-service/api/export/{id} | cotlet |
| socket.io | action types: ELEMENT_CREATE, ELEMENT_UPDATE, ELEMENT_MOVE_MULTI, ELEMENT_SET_TYPE, COMMENTS_ADD/UPDATE/DELETE, LABEL_CREATE | cotlet + sniff |
4. Base URL Resolution — https://app.milanote.com (sniff + cotlet agree). Crowd spec's base-URL candidates were noise.
5. Auth Patterns Detected — cotlet: Chrome cookie import over CDP, `curl_cffi` Chrome TLS (our probe shows stdlib HTTP is enough).
6. Parameter Name Evidence — search uses `query` (not `q`), `sort` (lastViewed etc.), `offset`, `locale`; cotlet names board_id/card_id/column.
7. Coverage Summary — crowd-sniff spec discarded (0 real endpoints); cotlet's recorded surface folded into the absorb manifest instead.
