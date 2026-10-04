# Same-reviewer Greptile triage — PR2267

Current published head: `a60db57af56ae93ebd0300b741506ed2b355fe05`. The isolated publish clone is clean. All three findings are valid and need a focused per-CLI fix; publication readiness remains held. No implementation, GitHub, acceptance-marker, global harness, ledger or phase receipts were changed. No new agent, updater or full suite was run.

## Reproductions

The exact approved installed CLI/MCP peers were exercised in fresh temporary storage with learning disabled. Their SHA256 values remain CLI `5d93ef94a811e9dfdb74f91be8b4f73f566dd65740a9b5bb99c8dc70531f6ed0`, MCP `ed7a7df1004541111263090aa4b290fbf007720b10ffc8a95067ed9a371689dc`. The controlled-response fixture passed 21 checks, including actual default sync and actual MCP SQL/inspect/save/saved calls. All four fixture source requests were anonymous GETs; no source/provider write occurred. Controlled product/area bodies demonstrate program behavior, not a new claim about real catalog facts.

A separate real public-source reproduction at 2026-10-04T15:29:05Z confirmed the P1. Root `search zz_tabiwa_greptile_no_match_2267_a60db57a --data-source live --no-cache --limit 3 --json --no-learn` returned three unrelated products: N1005300, N1005900 and N4002000. Native `catalog search` with the same query and `--region 20` returned zero after scanning all 214 source records. See the public proof for bounded original Japanese names and observation time. Both calls used a fresh temporary home.

## Minimal fix batch

1. **P1 — root search falsely labels unrelated catalog rows as query matches.** `internal/cli/search.go:119–126` sends unsupported `q` to `/ticketList/search`, supplies no selected regional preference and applies no local text filter. `outputSearchResults` only removes empty rows and limits them. The native catalog callback already handles regional preference, bounded scans and Japanese name/overview matching. Keep generic root `search` as local FTS over supported synced geography: auto/local resolve locally; explicit live should return a usage error directing the user to `catalog search`. Preserve `--type`, `--db`, limit and local provenance. A small handwritten root-command override can retain the generated local FTS callback while preventing its live branch; adjust its help/examples and per-CLI metadata. Do not copy the native search flag binder onto the root command: root resource-type/DB flags have different meanings and duplicate flag names.

2. **P2 — SQL description uses an unpopulated collection.** `internal/mcp/tools.go:62` advertises a `resources WHERE resource_type='catalog'` query, but the default and supported sync resource lists contain only geography. Actual default fixture sync produced exactly two geography rows; actual MCP catalog SQL returned zero, and the geography equivalent returned both names. This is valid SQL against an existing generic resources table, not a missing-table/schema failure. Change the example to geography and, where helpful, name the separate `catalog_saved` surface for selected evidence. Do not add bulk product sync or move the selected cache to satisfy a description.

3. **P2 — optional save invalidates the static read-only tool hint.** The emitted `catalog_search`, `catalog_inspect` and `catalog_compare` tools expose `save`; their current annotations say `readOnlyHint=true`. Actual MCP `catalog_inspect` with `{"product-id":"J0001900","region":"20","save":true}` created the previously absent `cache/catalog/saved.db`, returned `saved=true`, and `catalog_saved` returned the persisted product. The [official MCP schema](https://modelcontextprotocol.io/specification/2025-11-25/schema#toolannotations) defines readOnlyHint across the tool's environment, including explicit persistent local evidence. GET-only provider behavior and a default save=false call do not satisfy this whole-tool contract. The [maintainers' annotation guidance](https://blog.modelcontextprotocol.io/posts/2026-03-16-tool-annotations/) provides no persistent selected-cache exception. My earlier round3 interpretation checked provider/default-read semantics and was insufficient for the accepted save=true input. Correct that interpretation without reopening the closed cache chronology or artifact findings.

   Recommended: preserve optional CLI and MCP saving, set `mcp:read-only=false`, `mcp:local-write=true`, and `pp:live-happy-path=true` only for commands with the save capability. This includes the promoted executable `catalog` command as well as native search/inspect/compare; geography and saved remain reads. Supply real nonempty `--save` happy fixtures and run the normal acceptance with `--allow-destructive`. Disclose in tool descriptions that reads can persist selected bounded local evidence. Refresh the per-CLI context/tools manifest/skill and rebuilt native peers/bundle from actual emitted contracts; do not change the runner or hand-edit acceptance markers.

## Supported runner contract, independently executed

Exact cached SDK: `<go-module-cache>/github.com/mvanhorn/cli-printing-press/v4@v4.33.0`.

- `internal/pipeline/live_dogfood.go:714–726` classifies false read-only plus true local-write as mutating even for GET endpoints.
- Lines1644–1654 require both command `pp:live-happy-path=true` and operator `AllowDestructive` to disable automatic dry-run; the operator flag alone is insufficient when the command advertises dry-run.
- Lines1805–1849 preserve the selected fixture args, remove dry-run for the approved real run, request JSON, and use a disposable working directory.
- Lines194–199 and319–375 install the scoped home. `internal/pipeline/subprocess_env.go:38–107,218–266` rewrites HOME/XDG and strips the selected CLI's relocation variables; none-auth tabiwa has no credential sync-back.

One isolated Go contract test imported the unmodified cached runner. Annotation alone and operator flag alone kept `--dry-run`; both keys performed exactly one real local save from one HTTP GET, reused its JSON result, wrote outside the fixture source and operator storage, stripped an inherited CLI home override, and removed the runner storage afterward. The operator sentinel remained unchanged. This proves the supported runner mechanism with a tiny selected-save fixture; the fixed tabiwa commands still need their own normal acceptance and positive save evidence.

Reproduce from `<temporary>/tabiwa-local-write-runner-recheck` (the exact helper sources and go.mod/go.sum are copied into `proofs/reviewer-greptile-a60db57a-runner-helper`):

```sh
GOPROXY=off GOSUMDB=off GOWORK=off go test -mod=mod -run '^TestTabiwaReviewerSupportedLocalWriteOptIn$' -count=1 -v
```

The alternative read-only MCP subset is also feasible: hide save on the separate Cobra tree used by RegisterAll, and rely on the existing hidden-flag schema/argument allowlist plus flag-like positional rejection to reject save rather than merely hiding it from documentation. It needs matching CLI/MCP descriptions and denial proofs and removes MCP selected-save value. The supported local-write opt-in is smaller and preserves the existing useful interface. No new tools or framework changes are needed.

## Focused proofs required after fixes

- Generic root search with auto/local and explicit geography returns matching FTS rows and local provenance after supported sync; an impossible query returns empty. Explicit live fails with the native command pointer and performs zero HTTP requests. Custom DB/type/limit behavior remains usable. Native positive/negative regional substring queries remain genuine.
- Actual emitted SQL query description names geography; running its exact example after default sync returns nonempty names. Selected catalog observations remain in the independent cache.
- Actual emitted save-capable tools have readOnlyHint=false, schema save=true remains accepted, read tools retain their contracts, and descriptions/context/manifest/skill agree. Default no-save leaves selected cache absent. Positive save calls for search, inspect and compare return nonempty selected evidence and saved=true; saved returns the corresponding persisted observations with original clocks. Provider requests stay GET-only and no Queue-it/details/auth path is added.
- Normal runner-generated current-source acceptance uses `--allow-destructive` with the explicit command annotation and genuine nonempty `--save` fixtures. Inspect actual happy argv, outputs and saved evidence: no dry-run substitution, empty/help-only coverage or hand-authored marker. Keep explicit framework skips candid. Carry existing newest-observation tests if the save code changes.
- Refresh/rebuild staged and bundled peers and metadata, verify extracted byte/dependency/platform parity and actual domain MCP contracts. Use this same reviewer for the frozen final delta; root owns promotion and GitHub responses.

## Proof files

`reviewer-greptile-a60db57a-triage.json` records finding disposition, exact inspected project/publication file hashes and proof hashes. `reviewer-greptile-a60db57a-runtime.json` contains the 21-check actual-peer fixture proof. `reviewer-greptile-a60db57a-public.json` contains the real public false-query reproduction. `reviewer-greptile-a60db57a-runner-contract.json` records the three operator/annotation cases and actual isolated execution. `reviewer-greptile-a60db57a-fixture.py` reproduces runtime behavior with the same installed peer hashes; `reviewer-greptile-a60db57a-runner-helper/` preserves the offline runner reproduction.
