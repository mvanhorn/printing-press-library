# Phase 17 correctness review — round 1

Scope: gov_catalog.go, gov_edgar.go, gov_local.go, internal/govdata/*, internal/govcache/* and generated root execution/hooks. No edits made.

## Findings

1. **P1 — California filter phrases are escaped as literal quotes — internal/govdata/catalog.go:234,237.** The Go raw literals include backslashes (`res_format:\"CSV\"` and `organization:\"slug\"`) in the actual Solr fq parameter. The captured portal contract uses ordinary phrase delimiters. These flags can return incorrect/empty results. Construct normal quoted Solr literals with proper quote/backslash escaping or constrain accepted filter values; test decoded fq exactly and exercise one real filtered request.

2. **P1 — Financial numbers lose precision before output — internal/cli/gov_edgar.go:99.** `json.Unmarshal` into map[string]any converts every fact value to float64, so a valid `val:9007199254740993` round-trips as 9007199254740992. This violates raw financial-fact preservation. Use Decoder.UseNumber (and adapt ticker CIK conversion to json.Number), retaining number tokens through output. The same ingress issue at internal/govdata/catalog.go:117 happens before Raw metadata is marshaled, so later canonical fingerprint UseNumber cannot recover lost metadata digits. Add exact-number roundtrip tests at ingress.

3. **P2 — Custom errors bypass declared exit categories — internal/cli/gov_catalog.go:331 and internal/cli/gov_edgar.go:186.** Custom RunE paths return plain govdata/secJSON errors; generated Execute only categorizes Cobra usage errors and ExitCode returns 1 for any other untyped error. Confirmed `catalog find water --source california --limit 0 --json --no-learn` exits 1, although it is a usage error (2). HTTP404/429 and JSON failures similarly bypass 3/7/5. Validate catalog bounds with usageErr and classify custom API failures at the command boundary, preserving already typed usage errors. Add error exit tests for usage, API and throttle.

## Checks without findings

- Data-source annotations reach root strategy validation before command IO: live custom catalog/SEC rejects local; local snapshot tools reject live.
- Catalog and SEC live commands establish boundCtx before first HTTP request. Local snapshot DB calls use caller context; request-timeout flag is documented as network request timeout.
- Catalog scoped snapshot Complete derives next cursor/failure evidence; query scope and failures persist. Snapshot comparisons label absence as snapshot membership rather than live deletion.
- Store queries are parameterized and Load drains its rows without nested pooled query deadlocks. Snapshot save transaction commits before pooled Upsert helpers.
- SEC rejects errorType/error envelopes and missing search hits envelopes. Positive CIK, date range, accession, concept and archive filename checks are present. Source-derived URLs and bounded history/result caps are present.
- Resource/SEC downloads use bounded bytes, explicit selected output and no-overwrite linking; no hidden browser runtime.
- Full-text search preserves document-hit semantics and total relation rather than claiming distinct filing counts.

Convergence: three in-scope findings remain for parent autofix; re-review all personas after fixes.

# Round 2

Re-reviewed all same custom source scope, root hook strategy enforcement, regenerated California WithTier calls, and new gov_zz_errors command wrappers.

- Numeric ingress precision fixed with Decoder.UseNumber in SEC/catalog; CIK and count consumers updated.
- Catalog bounds/source validation is typed usage error before IO; command wrappers now classify API errors. The observed intermediate SEC_USER_AGENT-first HTTP mapping and aggregated429 omission were reported; latest source shows HTTP404/401/403/429 before SEC_USER_AGENT and is corrected.
- Intake manifest uses atomic temporary-file/no-overwrite linking. Scope header copies are removed. URL userinfo is rejected. Catalog --save no-local-data guard is present.

**PASS — no remaining correctness findings in round 2.** Final byte-level verification confirms the California fq delimiters are ordinary quote byte34, with no backslash byte92 around values. solrQuote mapping matches one backslash to two and quote to backslash+quote. The intermediate visually escaped tool output was ambiguous; final actual source bytes are correct. All three round-1 findings are resolved in the inspected source.

# Round 3 — final

PASS — no correctness findings. Re-reviewed the same custom catalog/SEC/local snapshot scope and source-routed client/root seams after the auth regeneration. California generated endpoints explicitly select the no-auth tier, while raw federal commands use default authHeader and saved-config credentials. The credential-isolation regression test passed. Confirmed source-level rate-limit preservation through errors.Join, HTTP status classification before missing User-Agent configuration, Decoder.UseNumber and numeric consumers, actual quote byte delimiters, bounded scan and download paths, live context boundaries, data-source enforcement, and immutable snapshot provenance. No source edits were made by this reviewer.
