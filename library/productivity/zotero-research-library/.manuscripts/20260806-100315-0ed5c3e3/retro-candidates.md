## Sync pagination silent no-op (2026-08-06)
- Spec omitted `cursor_param` on offset pagination; generator defaulted to `after`.
  Zotero ignores unknown query params, so sync refetched page 1 forever while the
  cursor climbed (18,650+ on a 3,308-item library) and stored the same 25 rows.
- Fix applied: explicit `cursor_param: start` + `limit_param: limit` on all 8
  paginated endpoints in the spec; regen; verified rows now advance.
- Generator retro candidates:
  1. Offset-type pagination defaulting to param name "after" is a trap — "offset"
     or "start" are the ecosystem norms; better, require cursor_param when type=offset.
  2. Sync could detect first-page-repeat (identical id set on consecutive pages)
     and abort with a pagination-config error instead of looping to the cap.
- Also noted: Zotero `Backoff` header on 2xx has no hand-authorable seam in the
  generated client transport (429 Retry-After and 5xx backoff are already
  handled); needs a header-observation hook to be implementable per-print.
