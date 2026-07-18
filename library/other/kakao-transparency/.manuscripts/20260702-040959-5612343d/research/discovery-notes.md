# kakao-transparency — API discovery notes

Source: the public Kakao Privacy site (https://privacy.kakao.com/transparency).

The page's own loader (`/js/renew/sub/transparency.js`) drives a single
unauthenticated JSON endpoint:

    $.get("/api/transparency/" + year + "/" + halfYearId, ...)

Live evidence gathered 2026-07-02 (curl):

- `GET /api/transparency/2025/2` → `{"success":true,"data":{...,"reports":[8 rows]}}`
- Coverage probed back to `2012/1` (published) — 1H 2012 is the archive start.
- Out-of-range periods (`2011/2`, `2026/1`, `2027/2`) return the site's HTML
  error page — observed as both HTTP 200 and HTTP 500 — never a JSON error.
- `Accept-Language: ko|en` switches the localized fields (title, content,
  category, halfYear, fileUrl). The row-level count fields are strings in the
  Korean payload and bare numbers in the English payload.
- Each payload carries `fileUrl`/`enFileUrl` XLSX workbook links
  (t1.kakaocdn.net) and `prevYn`/`nextYn` (1/0) archive-walk flags.

No existing CLI/SDK wraps this surface (searched GitHub, npm, PyPI for
kakao transparency / 카카오 투명성 clients — only ad-report SDKs exist).
