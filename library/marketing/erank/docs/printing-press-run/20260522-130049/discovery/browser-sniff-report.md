# eRank Browser-Sniff Report

## Capture
- Target: `https://members.erank.com/keyword-tool/top-listings?keyword=dad%20mug&source=etsy&country=USA`
- Auth: logged-in browser session, transferred into an agent-browser HAR capture.
- HAR: `/Users/smacdonald/printing-press/.runstate/grumpykinco-7e542738/runs/20260522-130049/discovery/browser-sniff-capture.har`
- Generated spec: `/Users/smacdonald/printing-press/.runstate/grumpykinco-7e542738/runs/20260522-130049/research/erank-browser-sniff-spec.yaml`
- Traffic analysis: `/Users/smacdonald/printing-press/.runstate/grumpykinco-7e542738/runs/20260522-130049/discovery/traffic-analysis.json`

## Replayable Surface
- `GET /api/keyword-tool/stats`
- `GET /api/keyword-tool/top-listings`
- `GET /api/keyword-tool/related-searches`
- `GET /api/keyword-tool/near-matches`
- `GET /api/keyword-tool/etsy-tags`
- `POST /api/keyword-tool/competition`
- `POST /api/keyword-tool/google-data`
- `POST /api/keyword-tool/keyword-difficulty`
- `POST /api/keyword-tool/save-history`
- `GET /api/keywordlist/names`
- `GET /api/keywordlist/terms`
- `GET /api/quota/daily`

## Reachability
- `traffic-analysis.json` classified runtime as `browser_http`.
- Generation hints: `browser_http_transport`, `requires_protected_client`, `weak_schema_confidence`.
- Implication: generated CLI should use browser-compatible HTTP transport and reusable eRank session auth. Plain unauthenticated HTTP is not enough for member endpoints.

## Risks
- Capture is from one authenticated keyword workflow only (`dad mug`, Etsy, USA), so body schemas for POST endpoints are weak.
- Account/member endpoints appeared in the HAR and should be kept out of the headline surface unless needed for auth/setup.
- Live dogfood will require a valid eRank session or a hold verdict.
