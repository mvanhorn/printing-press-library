# contacts list --email: spec.yaml carries the new `email` query param (2026-10-15)

- `spec.yaml` `/contacts` GET gains the public `email` param exactly as the app's `public/openapi.yaml` documents it (exploded
  form-style array, max 20, exact match on the primary and secondary email slots; app main 318adf38d). It sits next to `SearchParam`.
- `internal/cli/contacts_list.go`, `internal/cli/helpers.go` (`appendArrayQueryParam`), `internal/mcp/tools.go` and `tools-manifest.json`
  are the Printing Press 4.32.1 output for that one spec change (generated from the old and the new spec.yaml into scratch directories and
  the difference applied here, so the 4.28.0 print and its hand-fixes are untouched). `--email` takes a comma list or a JSON array and
  sends one `email` param per address. A repeated `--email a --email b` keeps only the last value (the generator emits a string flag).
- `internal/mcp/code_orch.go` was NOT updated: the 4.32.1 template differs from this tree's 4.28.0 file in the hunks that carry array
  query params, and it holds hand-fixes. The code-orchestration surface therefore does not bind `email` yet; the endpoint-mirror tools do.
- `upstream/openapi.yaml` was left stale on purpose (the public document has grown by ~13,000 lines since the 2026-09-16 reprint, and
  `make sync-spec` would fail TestSpecMatchesUpstreamPublicAPI until a full reprint); that refresh is a separate change.
- `internal/cli/contacts_list_email_test.go` pins the flag, the repeated params (comma and JSON forms) and the no-flag case.

## Round 2/3 hand-fixes (Codex round 1 and 2 findings, 2026-10-15)

Printing Press is a third-party generator (@mvanhorn/printing-press-library); these gaps cannot be fixed upstream by us, so they are hand-fixes
here, and a reprint from a newer generator must keep them (the tests below fail if it does not).

- `--email` is repeatable (`internal/cli/contacts_list.go`: `StringArrayVar`, not the generated string flag). `appendRepeatableArrayQueryParam`
  (`internal/cli/helpers.go`) flattens every occurrence (plain value, comma list, JSON array) and adds one `email` key per value with
  `url.Values.Add` (so `a+b@example.com` goes out as `a%2Bb%40example.com`). A supplied-but-empty value (empty, whitespace, empty comma item,
  `[]`, blank JSON element) is rejected with a usage error BEFORE any request, never turned into absence. More than 20 distinct values are
  sent as given: the server owns that 400 (it collapses duplicates, so 21 duplicates are legal).
- `internal/mcp/code_orch.go`: the `contacts.list` entry now binds `email`, and the execute handler adds GET/DELETE array params as repeated
  keys with `url.Values.Add` onto the path (`codeOrchAddArrayQuery`; an empty array adds `name=` so the API answers its 400). Scalars still
  ride the `map[string]string`. This replaces gap (b) above.
- `spec.yaml` and `tools-manifest.json`: the `email` parameter text now comes from the corrected `public/openapi.yaml` (20 distinct addresses,
  254 characters, 400 on empty or invalid, 422 = the unready-index fallback exceeding its stored-spelling bind budget, not a contact count).
  The API-source fix is on branch `fable/openapi-email-wording` of conduyt-svicons.
- The four accidental `*.orig` copies (contacts_list.go, helpers.go, tools.go, tools-manifest.json) were deleted.
- Tests: `internal/cli/contacts_list_email_test.go` (repeated flags, blank inputs send nothing, 21 distinct values, `+` encoding) and
  `internal/mcp/code_orch_array_query_test.go` (execute handler sends two `email` occurrences, empty array is not absence, binding present).
- Round 3 (Codex round 2 P2): an explicit empty scalar GET/DELETE param (`params: {"email": ""}`) used to take the scalar map path, where
  `client.do` drops empty values, so the request went out unfiltered. The execute handler now routes any empty scalar through the path
  `url.Values` as `name=`, the same as an empty array, so the API answers its 400. Pinned by
  `TestCodeOrchExecuteContactsListEmptyEmailScalarIsNotAbsence`. A reprint must keep this.

## Reprint 637 (2026-10-30)

`--email` stays the repeatable StringArrayVar with `appendRepeatableArrayQueryParam` (the print emits a plain string flag again). The
other public array params of GET /contacts (`tag`, `tagId`) are printed as plain string flags sent once; not made repeatable here.
