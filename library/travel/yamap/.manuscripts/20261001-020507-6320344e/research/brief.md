# YAMAP CLI build brief

## API Identity / product thesis
YAMAP is a Japan hiking platform. The focused CLI discovers named mountains, model courses (planned reference routes), recorded activity logs, and map coverage. Agents need compact source facts and recent contributor reports with evidence boundaries. Only first-party YAMAP reads are in scope. No API subscription, credentials, purchases, GPX download, third-party transit/weather, account backup, publishing, or mutations.

## Reachability and economics (2026-10-01 Asia/Singapore)
Direct yamap.com website GET returns AWS WAF HTTP 202 empty; Press probe confirms both stdlib and Chrome TLS transport challenged. Anonymous fresh Chrome renders useful public search results. Browser raw CDP permission was dismissed; no CDP workaround used. Supported page asset inventory exposed api.yamap.com/v6/activities/search and mountains/search. Anonymous direct API replay returns HTTP 200 JSON (captures in discovery). Default Japanese Accept-Language required; no auth headers or cookies. This is an undocumented first-party web API, not an official public developer API; contracts may change.
YAMAP Premium website currently lists 5,700 JPY/year or 780 JPY/month: https://yamap.com/premium . CLI costs no provider fee; no paid account silently required. Multiple-landmark filtering is premium per https://help.yamap.com/hc/ja/articles/30703212737561 . Downloading others' GPX is membership restricted and excluded. Map viewing links do not include offline navigation/tile coverage guarantees.

## Top workflows / build priorities
1. Search mountains by Japanese name; preserve duplicate-name candidates with prefectures and source IDs.
2. Find model courses; inspect reference distance, ascent/descent, standard course time and source closure/dashed-route flags without interpreting false as open/safe.
3. Search source activity logs and inspect one report lazily. Show activity date versus publication/update/fetch dates. Identify broad text match versus confirmed mountain traversal.
4. Find recent recorded reports from a bounded candidate set; filter by activity date, not publication time. Older seasons remain historical evidence.
5. Inspect named map area, bounds, source version, stale/deprecated status and explicit absence of navigation/track data.

## Table stakes / landscape / pain points
First-party website exposes mountain, map, model-course and diary discovery with metrics. akiyama709/yamap-export (https://github.com/akiyama709/yamap-export) provides account archival, photos, GPX, individual activity export and pacing. Its public api.py confirms Japanese language header, anonymous JSON, and a Mozilla-leading User-Agent to avoid mobile version HTTP 490. GitHub all-issues request returned an empty array on this date; no wrapper breakage reports observed. Search did not establish a maintained focused YAMAP discovery SDK/MCP/npm/PyPI alternative. Account migration/photo bulk download fall outside user scope.
Pain points: repeated full JSON consumes agent context; a text hit can mention distant mountains; report date, seasons, recorded time and model-course estimates are easily confused.

## Data layer / coverage
Bounded read-through disk JSON cache keyed by exact endpoint/query/language, explicit refresh; TTL 15 minutes and cached fetch timestamps. No full inventory crawl or surprise bulk sync. API total_count often caps activity search at 10,000: expose this as a source result window, not total coverage. One page default; hard page/time/response limits. Summary/detail retrieval remains lazy; raw images, tracks, user profiles and AI-generated descriptions omitted from compact summaries. Missing fields are null, zero is valid, source units retained.

## Domain correctness
Whole activity section total_time/active_time/rest_time differs from legacy duration/distance aggregates. Prefer regularized whole section in detail and retain definition; list metrics explicitly identify their source and missing active/rest time. Publisher mountain text, source flags, contributor report text, and official closures are separate classes. No command claims safe/open from user observations or false source flags. Full official closure coverage is unknown. A model course is a planned reference, not a trip log.

## Authorized decisions
Sole builder performs research/plan/build/fixes. User preapproved ordinary briefing and scope gates and explicitly limited delegation to exactly one fresh independent reviewer. Therefore feature brainstorm and documentation/output audits run locally; only code-review subagent is spawned. Shared global config and other projects remain untouched; .version-check was isolated to this workspace scope during preflight. No publication/PR.
