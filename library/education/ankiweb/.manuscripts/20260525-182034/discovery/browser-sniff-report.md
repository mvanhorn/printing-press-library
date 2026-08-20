# AnkiWeb Browser-Sniff Discovery Report

## 1. User Goal Flow
- **Goal:** Search the shared-deck catalog, open a deck's detail, download it, and list my synced decks/stats.
- **Steps completed:**
  1. Loaded `/shared/decks` → observed `GET /svc/shared/list-decks?search=` on load.
  2. Searched "spanish" → `GET /svc/shared/list-decks?search=spanish` (200, protobuf, 1039 decks).
  3. Opened deck detail `/shared/info/241428882` → `GET /svc/shared/item-info?sharedId=241428882` (200, protobuf, 60KB).
  4. Clicked Download → `GET /svc/shared/download-deck/241428882?t=<signed-jwt>` (503; token op=sdd).
  5. Loaded `/decks` (authenticated) → `POST /svc/decks/deck-list-info` (200 with session, 403 without).
- **Coverage:** 5 of 5 planned steps.

## 2. Backend / Configuration
- **Backend:** Claude-in-Chrome MCP (drove the user's logged-in Chrome; fresh capture tab, closed after).
- **Stack:** SvelteKit SPA (`_app/immutable/*`) over a `/svc/` protobuf service layer. Server HTML shells are empty (client-rendered) — confirms why plain WebFetch returned nothing.
- **No anti-bot challenge** observed; standard HTTP transport is viable.

## 3. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|--------|------|--------|--------------|------|
| GET | /svc/shared/list-decks?search= | 200 | application/octet-stream (protobuf) | public |
| GET | /svc/shared/item-info?sharedId= | 200 | application/octet-stream (protobuf) | public |
| GET | /svc/shared/download-deck/{id}?t= | 503/400 | application/octet-stream | public + signed token |
| POST | /svc/decks/deck-list-info | 200 / 403 | application/octet-stream (protobuf) | cookie-required |

## 4. Decoded Protobuf Schemas (reverse-engineered, validated)
**list-decks** → repeated field 1, each a deck message:
| Field | Meaning | Evidence |
|-------|---------|----------|
| 1 | id (varint) | matches /shared/info/{id} |
| 2 | title (string) | rendered title |
| 3 | upvotes (varint) | f3+f4 == rendered "Ratings" column (96+3=99, 153+8=161, 133+7=140) |
| 4 | downvotes (varint) | "" |
| 5 | modified (unix ts) | 1730569176 == 2024-11-02 rendered "Modified" |
| 6 | notes (varint) | == rendered "Notes" |
| 7 | audio (varint) | == rendered "Audio" |
| 8 | images (varint, omitted if 0) | == rendered "Images" |

**item-info** → field 1 wraps the full detail incl. description and a repeated reviews list (each review: f1 ts, f2 rating, f4 text). Core deck fields mirror list-decks.

**deck-list-info** → small protobuf (307 bytes) listing the user's synced decks.

## 5. Authentication Context
- Authenticated session used: **yes** (user's logged-in Chrome via chrome-MCP, no cookie transfer needed).
- Auth type: **cookie** (HttpOnly session cookie; `has_auth` is the JS-visible flag cookie).
- Validation (Step 2d): `POST /svc/decks/deck-list-info` → **200 with cookies, 403 without** ⇒ cookie auth works. Generated CLI will support `auth login --chrome` + `ANKIWEB_COOKIES` env var.
- Session state NOT archived (no session-state.json written; chrome-MCP used live session).

## 6. Known Constraints
1. **Protobuf responses** — every `/svc/` response is binary protobuf. Generated JSON decoding won't work; Phase 3 must hand-write wire-format decoders for the 3 read endpoints (field maps above make this tractable).
2. **Download token** — `download-deck/{id}` requires a signed `?t=` JWT (`{"op":"sdd","iat":...,"jv":1}`) minted by AnkiWeb's client JS. Not reproducible from captured traffic; the `download` command is a documented gap pending token reverse-engineering.

## 7. Replayability
- search / info / my-decks: replayable via standard HTTP + cookie header (no resident browser needed). ✓
- download: blocked on token minting. ✗ (documented gap)
