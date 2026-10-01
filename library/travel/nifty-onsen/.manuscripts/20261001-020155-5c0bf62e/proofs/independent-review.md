# Independent Nifty Onsen review

**PASS — no outstanding findings in the reviewed source and refreshed output samples.** Five initial findings were corrected by the builder and rechecked by the same independent reviewer. This is a correctness/security/domain/documentation review, not the final Press shipcheck verdict.

Run: `20261001-020155-5c0bf62e`
Provider: `https://onsen.nifty.com`
Reviewed at: 2026-10-01T01:51:55.798678+00:00

The reviewer read the local AGENTS.md, Press phases 14–17 and output-review contract, approved absorb manifest, research.json planned/built features, provider code and tests, generated CLI/MCP boundary, README and SKILL. No additional agents, source edits, publication, account operations, reservation operations or coupon issuance/redemption were performed. The only repository write by the reviewer is this report. Reviewer CLI/MCP runtime files were isolated under `/tmp`.

## Findings and disposition

All line references below identify the initial reviewed code unless described as the corrected call site.

| Severity | Finding, reproduction and evidence | Correction and recheck |
|---|---|---|
| P2 / medium | `internal/onsen/parse.go:143,583`: `absolute("")` returned the provider root, preventing `onsen_url` fallback when map responses lacked `onsen_detail_url`. Initial passing scorecard and nearby samples returned `https://onsen.nifty.com` for every facility despite real canonical URLs in the source response. | Empty links now stay empty. Parser regression checks URL and boolean source fields. Refreshed `evidence/e2e/nearby.json` returns `/shibuya-onsen/onsen018453/`, `/shibuya-onsen/onsen019649/` and `/nakano-onsen/onsen018307/`, ordered by 0.729, 0.777 and 0.793 km. |
| P2 / medium | `internal/onsen/parse.go:504` and `internal/cli/onsen_commands.go:148`: keyword-selected default coupon terms dropped the visible one-person-per-coupon and 24-hour reuse conditions for coupon `260820230023`; modern-card details were null. Help promised complete default term sets. | Cards without a recognized description now retain their complete visible text in `details`. Refreshed **default**, non-full-text `evidence/measured-coupons-uncached.json` includes both conditions, subscription requirement, weekday/holiday prices and expiry. Parser regression checks default details preservation. |
| P2 / medium | `internal/mcp/tools.go:44–61`: generated typed `bath_search`/`bath_show` handlers returned raw HTML through the generic client and prevented parsed command mirrors from using those names. Search lacked region/filter/page/limit/projection and day-use semantics. Generic context advertised unsupported sync/SQL/cursor workflows. | `RegisterTools` now calls preserved `registerOnsenTools` at `internal/mcp/tools.go:43`. The provider extension mirrors all seven actual workflows and provides provider context. Actual stdio schema/calls and refreshed three-call live MCP proof return parsed envelopes; SQL and raw HTML endpoint tools are absent. Reserved cobratree code remains unchanged. |
| P2 / medium | `internal/cli/onsen_commands.go:260–280`: `bath compare … --data-source=local --max-age=1ns --select=facility.id --agent` removed row freshness and left top metadata with `stale:false`, zero age and blank fetch time. Reproduced using isolated cached records derived from earlier live detail evidence. | Per-ID freshness/source URLs now survive projection in coverage metadata; top freshness reflects oldest successful facts and any stale row. `TestProjectedComparisonRetainsStaleProvenance` passes. Refreshed live comparison includes both IDs in `item_provenance`. |
| P3 / low | README Unique Features and SKILL Unique Capabilities claimed exclusivity over every other tool without comparative evidence. | Both now say “Verified provider workflows in this build.” |

Cache parser version and directory were advanced to v2, so older incorrectly parsed URLs/terms cannot silently survive these parser corrections.

## Final focused follow-up

**PASS — no new consequential findings.** The same sole reviewer checked the final command boundary and Example changes. The compare help Example now uses `--ids=onsen012278,onsen001483`, matching its happy-path fixture rather than mixing positionals and flags. Both public input forms still succeed with two facilities, and stale per-ID provenance remains intact after projection.

The unsupported generated `tail` command has no active registration. A fresh current-source reviewer build and the rebuilt evidence binary omit it from `agent-context`; invoking `tail` exits 2 with an unknown-command error. Its source is retained as `internal/cli/tail.go.disabled`, byte-identical to the original run scaffold (SHA-256 `df31d1316f2d09aceea5c4fc129f5684ee37c9b276eff3260b539d0b850bc771`). The preserved patch record explains why this unapproved, nonfunctional utility is excluded. All seven approved provider workflows and the parsed MCP boundary remain present. README/SKILL/AGENTS do not promise streaming.

The reviewer rebuilt current source and reran targeted CLI-tree/provider/MCP/parser/cache checks plus the stale comparison regression; all passed. Core/parser/MCP fingerprints were unchanged from the completed review; only root registration and the compare Example changed. Final native dogfood/shipcheck/full-suite acceptance remains the builder’s separate gate.

## Final live-harness follow-up

**PASS — no new consequential findings.** The same sole reviewer checked the narrow final harness changes. `show [id]` and `coupons [id]` expose recognizable optional ID positionals without changing `--id`, canonical facility URL acceptance, or one-identity validation. Runtime MCP schemas correctly disambiguate the positional as `positional-id`; actual stdio calls using `id`, a canonical URL in `positional-id`, and the coupon positional all returned parsed offline envelopes with zero requests.

`onsenClient` changes automatic source mode to live only when `PRINTING_PRESS_DOGFOOD=1`; ordinary automatic caching is unchanged, and explicit local mode remains local. This prevents warm parsed results from standing in for the required live acceptance reads. `TestOnsenLiveHarnessRefreshesWarmCache` passed: an in-memory transport received a request despite a fresh cached result, its distinct fixture replaced the cached facts, and explicit local mode still reported offline cache. This is deterministic harness-boundary evidence, separate from real provider live acceptance.

Targeted current CLI input/projection/tree, parsed MCP registration, detail/coupon parsing and cache-source tests passed. Provider parser/model/catalog/client and MCP implementation fingerprints remained unchanged. Final native acceptance/full-suite verification remains the builder’s separate gate.

## Phase 14: seven SKILL semantic checks

| Check | Result |
|---|---|
| Trigger phrases match capabilities | PASS. Day-use discovery, source inspection, public coupon terms, shortlist comparison and coordinate candidates map to implemented commands. |
| Verified-set alignment | PASS. All five research `novel_features_built` entries appear with matching commands/descriptions; the two distinct `bath show` entries are intentional. No planned-only feature is claimed. |
| Novel descriptions match commands | PASS. Runtime help for show, coupons, nearby, search and compare matches parser/workflow behavior and bounds. |
| Stub/gated disclosure | PASS. Provider workflows are implemented. Bounded coverage, incomplete inventory, uncertain eligibility and source-layout failures are disclosed. |
| Auth narrative | PASS. Public provider workflows need no account or API key. The map page key/cookie are transient query context. No nonexistent login commands are prescribed. |
| Recipe output claims | PASS. Examples produce projected listing facts, parsed detail/coupon evidence, bounded nearby rows and comparison rows. Local-build instructions precede explicitly post-publication installer references. |
| Marketing-copy smell | PASS after removal of the unsupported exclusivity sentence. Remaining capability descriptions are concrete. |

## Phases 15–17: factual, output and code checks

README/SKILL/AGENTS examples and command limits match runtime help and source. They correctly describe source-page pagination, default day-use filtering, bitmask caveats, canonical information handoffs, parsed cache modes, 30-minute default age, the six-hour fallback for zero max-age, computed catalog rejection of live mode and no remote mutations. Final evidence packaging/Press receipts remain the builder’s responsibility.

The initial scorecard had five eligible passing samples; they were actually reviewed rather than treating empty/failing samples as a clean pass. The refreshed 24-case matrix at **2026-10-01 09:33:23 +08:00** covers three prefectures, full pages 1/2, explicit zero results, natural/private/combined source filters, spa/sento/stay detail, public coupons, nearby/empty radius, comparison, projection, offline cache and usage errors. Canonical Japanese names/URLs and source-ranked/straight-line ordering are plausible; no encoding/entity errors or silent source drops were observed in reviewed samples. Refreshed measured defaults return 5,291 coupon bytes and 4,064 nearby bytes; cached calls use zero network requests. Measurements are single samples with bytes/4 token estimates, not statistical performance claims.

The active combined private bath/room badge is normalized to the combined label and cannot alone create a rentable-private-bath or private-room claim. Narrower explicit source labels are required. Natural hot spring is distinct from ordinary bath categories; name text is not certification. Detail admission retains weekday/holiday, age, fee-basis and extras verbatim. Policies retain explicit facility-local wording without universal permission claims. Reservable inventory remains unknown. Nearby uses the source’s documented zoom-minus-one request and a bounded candidate window, then straight-line sorting and local radius filtering; empty output does not imply a complete radius inventory.

One HTTP client per invocation, sequential requests, context deadlines, finite coordinate/query/page/limit checks, response and parsed-cache caps, restricted provider redirects, redacted transport errors and typed 429 handling were checked. Map query key/cookies stay in memory and are absent from parsed caches/output metadata. Auto/local/live/no-cache modes and stale fallback are coherent; throttles are never converted into cached success. Partial comparison was additionally exercised with one existing cache entry and one missing ID: the success remains visible, stderr warns, and metadata identifies the failed ID. All-failure paths return nonzero.

Actual reviewer stdio MCP checks verified context plus exactly seven provider tools, full parsed search/show schemas, a zero-request local projected show retaining holiday/towel admission evidence, and a live-mode rejection for the computed filters tool. Builder live `evidence/mcp-smoke.json` separately proves parsed search/show/nearby network calls. No arbitrary shell command or user-chosen filesystem destination is introduced by the provider extension.

Reviewer targeted tests passed for parser/catalog/source/cache/bounds, CLI input/projection, current MCP registration, and the new stale-comparison regression. Existing full-suite evidence was inspected; final full-suite/shipcheck/scorecard reruns are owned by the builder. `evidence/gosec-final.json` contains zero findings in the authored provider files and 22 inherited generated/reserved candidates, documented separately; this review does not claim those candidates are 22 confirmed vulnerabilities or that the entire scaffold has been security-certified.

Limit: live rate-limit events, source layout changes and actual venue admission/coupon acceptance cannot be guaranteed by these snapshots. Structural errors and conservative unknown states are the intended behavior.

## Reviewed source fingerprint

SHA-256, relative to the project root. No Git repository is available. Later source/doc changes invalidate the corresponding reviewed fingerprint.

```text
ed9e5474bb91f050f37073b44217eeba6b8d57680c7e4e7176db631f76bdea23  AGENTS.md
452c596b48563136cff40448e93306aa3bc760a48f78b873e8d68424154496cd  README.md
c174b3739a1ad56e99c52da84f48160bb38ac08e6c240bac5ece9aa1d767d189  SKILL.md
086d388827ac15651d8fc32c7155a1f29bc8c0e38ea47484007296578ffd1b8c  internal/onsen/model.go
7096e0e66c1fa8c218764068c4c58163d9eb7372eab4e4045e43d6cbbc501100  internal/onsen/catalog.go
d9083ca65ac4be6706eeb022cb30960fd061c1647ee9918bc3e99ef115b56045  internal/onsen/parse.go
e9a465110ce0d7c06cbfe3967961af64f9ca65a67bc1433e1cc031cb08d4659e  internal/onsen/client.go
a9ec31a8fcdcb2283fa3d8310653d61bd8c74a1835facc1e350b6770f9849328  internal/onsen/parse_test.go
71d6fc271893f1da7c381538334faeb4bceabd579d8c8966d42d085caa532a15  internal/onsen/client_test.go
88d05360ff081e4756c94bb29896ba2919cf82d0c953bd6107280be70c47f877  internal/cli/bath.go
cf2e59cff0650d5c7b964c80a2cf7bd65b26e30cd6af9635794de4feb1b8fce8  internal/cli/bath_search.go
492a2835418824998c2b855a38994ae86ae44da6fa6cfec0a50579b90db1da24  internal/cli/bath_show.go
ab4db022f6531b44278ffa90f372c2c1f3d12867527fc2c45e9c15f314ef4f66  internal/cli/onsen_commands.go
a82a5ecfea314bde5a9579b1368c3803b8aad580700a2b5cf1ce53156488619f  internal/cli/onsen_commands_test.go
d3e4998266a70c4041a6b790f316bd28a2dd82cb047254f1bf209c7722e181b4  internal/cli/root.go
05d21c930412caba93176abf628c22148e1c973b8f61fd85000a3add5f72a352  internal/cli/helpers.go
7e8e28827fb0e462b31e51aaed4ea6b28181d2d64cb44963b19d9e75f0042fe8  internal/client/client.go
63be2bce6ba29430f5fd3dee546f7e3396bb6434578491317aaf91602db58060  internal/mcp/tools.go
13b37e6d1bd9a5940ddc7bcea2254d1e56b9a32f3663eec25dae1663f3ee3fde  internal/mcp/tools_test.go
a2a25318d42cc7d25cf113819092d60bc9927e33a0f8b6701ef2223eae684037  internal/mcp/onsen_tools.go
d005ceba86f41cf9e48ae5b46fdae42e29c04c895a2d067e8a64a2d3a4f7c7e2  internal/mcp/cobratree/walker.go
3c153485c058debe648013a5961442c417ace1424d4aef816369a4540772e51c  internal/mcp/cobratree/names.go
c022b153bcc248ee0e124ff89caaff6172386e64cd23a980e5a16c2fc8a05a89  internal/mcp/cobratree/shellout.go
ad0c8e077bc07f9f34c309b3e3b4722e430369f4da8c7ba649ac45dbb71a150f  internal/mcp/cobratree/typemap.go
8cf7e1df4d6edf1611e31a310245ba7b815319c8b32e4c9dcfa7f50d55161da6  internal/mcp/bound/bound.go
730a5b3039d3894b19f2efe59528588bf031504db9ef32a8343547f635c51c1b  cmd/nifty-onsen-pp-mcp/main.go
df31d1316f2d09aceea5c4fc129f5684ee37c9b276eff3260b539d0b850bc771  internal/cli/tail.go.disabled
bd5271297682004a366298d8b06cdcc854c4f03f1d512db710403b9a8f1e4ac8  .printing-press-patches/provider-has-no-generic-resource-stream.json
```

## Refreshed reviewed proof fingerprint

```text
9d1b576cc892e998b40e56ce941d1256a63e2e2f32344faeb4019e04b25e6b54  evidence/e2e/summary.json
9cb7ccb46aa29a1653abacd7744bc4737c14e96b934530cfd14800ccb3617f09  evidence/e2e/nearby.json
3be36653fb3fe8f0521f0b36b6c6efba45d2f87d7a3b34b03e5a8503e17d61d9  evidence/e2e/coupons.json
432affbd4f83e50e5ace8b95840ee9f2c94f8ed24c21e3aafc855c6bf761c65a  evidence/e2e/compare.json
740ed212a46d421e6876158fd9c4ec58aab972c0c65d9b8387aea958798d174f  evidence/measured-coupons-uncached.json
a631b4dc42c77f4001c64b9d15d9c0138d61d6742769c194cba47368f026d383  evidence/metrics.json
e15673ce24d2ea258debc090d9d8fa2c3df3fe55d1341e38a463a2a0c95ae02e  evidence/mcp-smoke.json
```
