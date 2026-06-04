# AnkiWeb Editor / Add-Note Discovery Report

Captured read-only from the logged-in ankiuser.net/add page. The Add submission was **intercepted and blocked** (no card written to the user's collection); request bytes captured client-side before the network call.

## Endpoints
- `POST /svc/editor/get-info-for-adding` — populate the add form. Empty request body. Response (protobuf):
  - repeated field **1** = note types, each `{ f1: id (varint), f2: name (string) }`
  - repeated field **2** = decks, each `{ f1: id (varint), f2: name (string) }`
  - field **3** (varint) = default/selected **deck_id**
  - field **4** (varint) = default/selected **notetype_id**
  - repeated field **5** = field defs of the **default** note type, each `{ f2: field name (string), ... }` (e.g. Front, Back). NOTE: passing a notetype_id in the request body does NOT change the returned fields — only the default note type's fields are exposed here.
- `POST /svc/editor/add-or-update` — the write. Request (protobuf), 63 bytes for a 2-field note:
  - repeated field **1** = note field VALUES in order (e.g. "Front text", "Back text") — names not sent; order matters
  - field **2** (string) = tags (space-separated)
  - field **3** (message) = `{ f1: notetype_id (varint), f2: deck_id (varint) }`
  - Response: 200 on success. Self-contained — no client GUID/csum/usn/sync needed (server-side add).

## Account data shape (from get-info)
- Note types: the standard Anki defaults are present (Basic, Basic (and reversed card), Basic (optional reversed card), Basic (type in the answer), Cloze, Image Occlusion) plus any custom types; each carries an account-specific numeric id and a name. One is flagged DEFAULT.
- Decks: a list of the account's decks, each with an account-specific numeric id and a name; one is flagged DEFAULT. (Specific deck names/ids redacted — they are personal account data.)
- The default note type's field names are returned in field 5 (e.g. Front, Back for a Basic type).

## Auth (per-domain session cookie)
- These editor endpoints are served from **ankiuser.net** and authenticate with **that domain's** `ankiweb` session cookie. AnkiWeb issues a **separate** session cookie per domain: the ankiweb.net cookie (used by `decks/*`, `shared/*`) is **rejected by the editor with HTTP 404** (the server uses 403 for "not logged in", so a 404 here means "authenticated, wrong session"). The CLI must send the ankiuser.net cookie (`ANKIUSER_COOKIES`) for editor commands and must NOT fall back to the ankiweb.net cookie. Verified live 2026-05-30.

## Implications for the CLI
- `add` builds the add-or-update request: ordered field values (positional `add "front" "back"` or repeatable `--field`), tags, and a target `{notetype_id, deck_id}` resolved from get-info by name.
- Default note type + deck come from get-info f4/f3. `--deck`/`--type` override by name (resolve to id).
- For the default note type, `--field name=value` can be mapped to order via f5 names; for other note types, values are sent in the order given (document this).
- Write safety: `--dry-run` prints the resolved request; confirm unless `--yes`; short-circuit under PRINTING_PRESS_VERIFY.
