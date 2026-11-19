# mcp-canonical-bounding: verification record (2026-11-17)

## Mutation proofs
Every mutant was run with `mutation-verify.sh` in a disposable `git archive HEAD | tar -x` copy (never cp -R, never git inside a copy), with `--expect-fail-output` (the knocked-out row's own FAIL line), a `-skip` knock-out whose scope proof is `--knockout-delta-line '=== RUN '`, `--syntax-cmd 'go build ./internal/...'`, and a control mutant that breaks a different behavior in the same file. All verdicts: CAUGHT (reachability was not separately cover-checked). Copies were deleted afterwards.

| Mutant | File | Mutation | Knock-out row (expected FAIL line vanishes) | Control (breaks a different behavior) | Control's own failure |
|---|---|---|---|---|---|
| M01_fixed_50_cap | etsy_canonical.go | binary search ceiling replaced by a fixed 50 | TestMCPOutputNeverExceedsBudget/binary_search_maximality | M07_tool_check_removed | TestMirrorCursorErrors/wrong_tool_before_args |
| M02a_cursor_offset_plus1 | etsy_canonical.go | next_cursor offset +1 | TestMCPOutputNeverExceedsBudget/cursor_union_pages_concatenate_deep_equal_in_order | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M02b_cursor_offset_minus1 | etsy_canonical.go | next_cursor offset -1 | TestMCPOutputNeverExceedsBudget/cursor_union_pages_concatenate_deep_equal_in_order | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M03_wrapper_accounting_deleted | etsy_canonical.go | wrapper/metadata byte accounting deleted (fit check ignores 400 envelope bytes) | TestMCPOutputNeverExceedsBudget/past_budget | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M04_oversized_empty_page | etsy_canonical.go | oversized item yields an empty page | TestMCPOutputNeverExceedsBudget/single_oversized_item_full_field_set | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M05_digest_check_removed | etsy_canonical.go | cursor digest check removed | TestMirrorCursorErrors/stale_digest_with_valid_offset | M01 | TestMCPOutputNeverExceedsBudget/binary_search_maximality |
| M06_args_hash_check_removed | etsy_canonical.go | cursor args-hash check removed | TestMirrorCursorErrors/wrong_args | M01 | TestMCPOutputNeverExceedsBudget/binary_search_maximality |
| M07_tool_check_removed | etsy_canonical.go | cursor tool check removed | TestMirrorCursorErrors/wrong_tool_before_args | M01 | TestMCPOutputNeverExceedsBudget/binary_search_maximality |
| M08a_supports_cursor_gate_removed_core | etsy_canonical.go | SupportsCursor gate removed in the envelope builder | TestMCPOutputNeverExceedsBudget/supports_cursor_false_next_cursor_key_absent_via_raw_map_with_tool_note | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M08b_supports_cursor_gate_removed_wiring | shellout.go | SupportsCursor forced true at the handler (ignores registration) | TestEtsyMCPBoundingSeam/market_snapshot_overbudget_no_cursor_and_do_not_rerun_note | M17 | TestEtsyMCPBoundingSeam/inline_market_inputs |
| M09_computed_readonly_instead_of_annotation | typemap.go | cursorEligible uses computed read-only (read-only-unless-flags under MCP read-only) instead of the literal annotation | TestEtsyMCPBoundingSeam/schema_assertions | M20 | TestEtsyMCPBoundingSeam/pages_to_completion_union_equals_cli_results |
| M10_objects_over_scalars_picker_removed | etsy_canonical.go | objects-over-scalars array picker removed | TestMCPOutputNeverExceedsBudget/scalar_array_only_to_preview | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M11_agent_descent_removed | etsy_canonical.go | agent {meta,results} descent removed | TestMCPOutputNeverExceedsBudget/agent_descent_pages_inner_array_with_canonical_fields_top_level | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M12_budget_ignores_env | etsy_canonical.go | Budget() ignores PP_MCP_MAX_BYTES | TestMCPOutputNeverExceedsBudget/past_budget | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M13_floor_clamp_removed | etsy_canonical.go | budget floor clamp removed | TestMCPOutputNeverExceedsBudget/budget_floor_clamp | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| M14a_per_response_sum_secondary_always_full | shellout.go | per-response sum removed: secondary block always emitted in full | TestEtsyPerResponseBudget/failure_huge_both | M17 | TestMirrorArgGuardBeforeExec |
| M14b_per_response_reserve_removed | shellout.go | per-response sum removed: marker reserve dropped before sizing the primary | TestEtsyPerResponseBudget/success_drops_hints | M17 | TestMirrorArgGuardBeforeExec |
| M15_mirror_wiring_reverted_to_bound_text | shellout.go | success path back to bound.Text (Mirror bypassed) | TestEtsyPerResponseBudget/success_drops_hints | M17 | TestMirrorArgGuardBeforeExec |
| M16_cursor_forwarded_to_cli | shellout.go | cursor forwarded to the child CLI | TestEtsyMCPBoundingSeam/cursor_stale_after_new_snapshot | M17 | TestEtsyMCPBoundingSeam/inline_market_inputs |
| M17_pre_exec_arg_guard_removed | shellout.go | pre-exec argument guard disabled | TestEtsyMCPBoundingSeam/inline_market_inputs | M08b | TestEtsyMCPBoundingSeam/market_snapshot_overbudget_no_cursor_and_do_not_rerun_note |
| M18_typed_nextupstream_case_removed | bound.go | typed nextPageCursor nextUpstream case removed | TestEndpointPageResponseWrapsUpstreamCursorOpaquely (top-level) | C18 (typed local offset +1) | TestEndpointPageResponseContinuesFromOpaqueCursor (top-level) |
| M19_cursor_dropped_from_schema | typemap.go | cursor dropped from the listed schema (piece a) | TestEtsyMCPBoundingSeam/schema_assertions | M20 | TestEtsyMCPBoundingSeam/pages_to_completion_union_equals_cli_results |
| M20_cursor_dropped_from_allowed_args | typemap.go | cursor dropped from allowed structured args (piece b) | TestEtsyMCPBoundingSeam/pages_to_completion_union_equals_cli_results | M19 | TestEtsyMCPBoundingSeam/schema_assertions |

Refused configuration: the first run of the bound.Text-revert mutant against the seam row `compact_true_projects_rows_over_mcp` exited 2 (knock-out scope unproven: the seam helper aborts the parent test on failure, so the mutant run had one fewer `=== RUN ` line than the knock-out run). It was re-run against `TestEtsyPerResponseBudget/success_drops_hints`, which does not abort siblings, and was CAUGHT.
Control note: `M18` uses a bound.go control (typed local offset +1) because the typed guard lives in bound.go; its expected failure text is the top-level test name, so no row-level knock-out applies and the text was verified absent from the control run by the script.

## Live smoke (step 2, behaviour and counts only)
Isolated store (HOME/ETSY_HOME/ETSY_DATA_DIR under scratch, ETSY_SHOP_ID unset), 5 Etsy calls total: one `market snapshot` of three generic terms (1 page of 40 each) which stored 120 listings and 120 shop rows. Over the real stdio `etsy-pp-mcp` with `PP_MCP_MAX_BYTES=4000`: `market_demand` paged in 3 pages (3630, 3849, 3404 bytes, each at most 4000), `count`=3 on every page, `truncated` true, true, false, no `next_cursor` on the last page; the union of `results` equals the direct CLI output (12454 bytes). `cursor` is present in the `market_demand` schema; a garbage cursor returns `cursor_invalid`; a cursor resent without its original arguments returns `cursor_args_mismatch`.

## Regen-merge dry-run (plan section 0b item 8)
Fresh tree generated offline from `spec.json` with the same press version (4.35.6, `--mcp-orchestration code`, the worktree research dir); the fresh `bound.go`, `shellout.go`, `typemap.go` and `walker.go` are byte-identical to the pre-change generated files. `cli-printing-press regen-merge <worktree> --fresh <tree> --json` (dry-run, no `--apply`): 491 files; 283 TEMPLATED-CLEAN, 139 NOVEL, 42 TEMPLATED-WITH-ADDITIONS, 13 TEMPLATED-VALUE-DRIFT, 10 TEMPLATED-BODY-DRIFT, 4 PUBLISHED-ONLY-TEMPLATED; 2 lost AddCommand registrations (auth login, pre-existing); go.mod not merged. Per-file verdicts for this patch are in the patch record's `reprint_status`. Key result: `bound.go` is TEMPLATED-CLEAN and would be overwritten (MaxBytes edit lost); the simulated overwrite fails `go vet` in the bound package because the Etsy test assigns `MaxBytes`.

## Round 2 (build-gate fixes, base edb80ab)
Same method as round 1 (mutation-verify.sh in a `git archive HEAD | tar -x` copy, `--syntax-cmd 'go build ./internal/...'`, `--expect-fail-output '--- FAIL: <Test>/<row> ('`, `-skip` knock-out with `--knockout-delta-line '=== RUN '`, a control in the same file). All 14 verdicts CAUGHT; copies deleted. Top-level rows (`-`) have a single `=== RUN ` line, so the knock-out deselects the whole test. Round-1 mutants M01 (control for R03) and M07, M17, M20 (controls) were reused unchanged.

| Mutant | File | Mutation | Knock-out row | Control | Control's own failure |
|---|---|---|---|---|---|
| R01_preview_note_promises_cursor | etsy_canonical.go | Preview note asks for next_cursor when SupportsCursor | TestMCPOutputNeverExceedsBudget/array_free_to_preview | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| R02_oversized_note_promises_cursor | etsy_canonical.go | oversized-last note asks for next_cursor | TestMCPOutputNeverExceedsBudget/oversized_last_item_omits_next_cursor | M07 | same |
| R03_cursor_length_cap_removed | etsy_canonical.go | `len(raw) > 1024` -> `> 1<<30` | TestMirrorCursorErrors/oversize_valid_cursor | M01 | TestMCPOutputNeverExceedsBudget/binary_search_maximality |
| R05_oversized_next_cursor_wrong_offset | etsy_canonical.go | `cursor(o + 1)` -> `cursor(1)` | TestMCPOutputNeverExceedsBudget/oversized_middle_item_cursor_offset_one_next_is_two | M07 | same |
| R08_budget_ceiling_removed | etsy_canonical.go | 16 MiB Budget ceiling removed | TestHardeningBudget/MaxInt64 | M07 | same |
| R12_name_first_in_id_keys | etsy_canonical.go | `name` first in oversized_item_id keys | TestHardeningReceiptOversizedID (top-level) | M07 | same |
| R04_empty_cursor_is_invalid | shellout.go | `cursor:""` treated as invalid | TestEtsyMCPBoundingSeam/empty_cursor_equals_page_one | M17 | TestEtsyMCPBoundingSeam/inline_market_inputs |
| R14_limit_dropped_from_narrow_list | shellout.go | `limit` dropped from the narrowing list | TestEtsyMCPBoundingSeam/market_tags_overbudget_preview_note_names_limit | M17 | same |
| R07_cli_args_unsorted | shellout.go | `sort.Strings(keys)` removed from the generated cliArgsFromMCP (copy-only mutant; never committed) | TestEtsyMCPBoundingSeam/multi_arg_paging_window_compact_top | M17 | same |
| R06_true_stdout_size_uses_retained_len | shellout.go | StdoutBytes/StdoutOverflow from retained length | TestEtsyRunCLICommandTrueStdoutSize (top-level) | M17 | TestMirrorArgGuardBeforeExec |
| R09_stderr_preallocated | shellout.go | stderr capture preallocates MaxBytes+1 | TestHardeningStderrAllocationAndFailure (top-level) | M17 | TestMirrorArgGuardBeforeExec |
| R13_money_summary_cursor_eligible | typemap.go | money summary exclusion removed | TestEtsyMCPBoundingSeam/schema_assertions | M20 | TestEtsyMCPBoundingSeam/pages_to_completion_union_equals_cli_results |
| R10_pattern_count_cap_removed | mkt_rules.go | 256-pattern cap removed | TestRulesCompilationCaps/too_many | R11 | TestRulesCompilationCaps/too_long |
| R11_pattern_bytes_cap_removed | mkt_rules.go | 512-byte cap removed | TestRulesCompilationCaps/too_long | R10 | TestRulesCompilationCaps/too_many |

Round-2 live smoke (behaviour and counts only, existing isolated scratch store, 0 new Etsy calls): `market_tags` over `PP_MCP_MAX_BYTES=4000` returns a Preview (4000 bytes, truncated true, no `next_cursor` key, note names `limit`, note has no next_cursor); `market_demand` with `{window:"6mo",compact:true,top:2}` returned one page whose results equal the CLI output (the three-term store is too small to page at that width; the hermetic seam row pages it); `cursor:""` equals page 1; `money_summary` has no `cursor` property and rejects the parameter.
Gate: build, vet, gofmt, and `go test -count=1 ./...` pass both with the ETSY_* variables unset and with `PP_MCP_MAX_BYTES=4000` exported (TestMain in the three mcp packages isolates the budget).


## Round 3 — Group F

F1 and F6 were CAUGHT using `mutation-verify.sh` in a `git archive HEAD` scratch copy with only this group's working changes overlaid. Both used a bare one-package `go test -v -count=1` command (no `-run` filter), `--syntax-cmd 'go test -run "^$" <package>'`, their own FAIL-line expectation, a single-row `-skip` knock-out and `--knockout-delta-line '=== RUN '`. Each knock-out passed on the mutant with exactly one fewer RUN line; each control went RED for its own failure without the hazard's expected failure. Etsy credentials and the byte-budget override were unset for test runs. No reachability claim beyond those knock-out/control results is made.

| Item | File | Mutation | Knock-out row | Control mutation | Control's own failure |
|---|---|---|---|---|---|
| F1 | internal/mcp/bound/etsy_canonical.go | Remove the snapshot recovery advice appended to a stale message | TestStaleGuidanceScope/with_snapshot | DefaultMaxBytes 32000 to 32001 | TestBudgetValuesAndClampWarning |
| F6 | internal/mcp/cobratree/shellout.go | Bound the two-block failure with budget minus the marker and reserve 0, restoring the reduced max_bytes report | TestFailureFullBudget/two_block_failure_reports_full_budget_max_bytes | Change the NoRerun note opening so its exact pin fails | TestEtsyMirrorRegisteredNarrowing/live |

The F1 seam additionally makes `orders_unshipped` exceed 4000 bytes with synthetic receipts, writes another receipt between pages, and asserts a stale error with no snapshot wording. The market seam checks snapshot advice, its agent-wrapper location, and select recovery. F7's actual caller prefixes differ: file rules use `rules:`, while inline rules use `--rules-json:`. Both now appear once; the existing exit-2 test covers both paths. Unpinned duplicate inner prefixes on missing-name and regexp errors were also removed.

## Round 3 — Group G and operator proofs (base 750c140, tests at 49fb4d7)
Same method; every run used the bare one-package test command (no `-run` filter): bound, cobratree, mcp, mcpconformance or cli as listed. All 6 verdicts CAUGHT. S01's knock-out run exits 1 because four conformance arms (default and opt-in, readOnly and readWrite) catch the mutant, not only the knocked-out row; only the named row's FAIL line is required to vanish. G1's default-arm half holds trivially (`full:true` is refused before paging, and `{}` never emits `name`); the opt-in arm, which must find the last-page sentinel, is the part that proves the walk goes past page 1. Controls reused from earlier rounds (M07, M17, R11) or a new floors control (`{"readOnly", 20, false}` -> `true`) in the conformance file.

| Mutant | File | Mutation | Package | Knock-out row | Control | Control's own failure |
|---|---|---|---|---|---|---|
| S01_walk_stops_after_page_one | etsy_buyer_conformance_test.go | `for page := 1; cursor != ""` -> `false && ...` (walk stops after page 1) | mcpconformance | TestEtsyMCPBuyerConformanceOptInSeesSentinels/readOnly | floors control | TestEtsyConformanceFloors (top-level) |
| S02_oversized_middle_note_without_cursor_sentence | etsy_canonical.go | `mirrorNote(req, false, opening)` on the oversized note | bound | TestMCPOutputNeverExceedsBudget/oversized_middle_item_cursor_offset_one_next_is_two | M07 | TestMirrorCursorErrors/wrong_tool_before_args |
| S03_transaction_id_dropped_from_keys | etsy_canonical.go | `transaction_id` removed from the id key list | bound | TestHardeningReceiptOversizedID/transaction_id | M07 | same |
| S04_cap_does_not_accumulate | mkt_rules.go | `count +=` -> `count =` | cli | TestRulesCompilationCaps/cap_accumulates_across_targets | R11 | TestRulesCompilationCaps/too_long |
| S05_supports_cursor_override_noop | shellout.go | SupportsCursor override from cursorEligible made a no-op | cobratree | TestHardeningNativeCursor (top-level) | M17 | TestMirrorArgGuardBeforeExec |
| S06_snapshot_advice_for_every_tool | shellout.go | `SnapshotParam: true` for every tool | mcp | TestEtsyMCPBoundingSeam/orders_unshipped_stale_message_has_no_snapshot | M17 | TestEtsyMCPBoundingSeam/inline_market_inputs |

F1 and F6 proofs are in the Group F section above.

### Truth conditions verified (round 3)
- Tools registering a `snapshot` param (listed by the stdio server's tools/list): market_demand, market_gaps, market_price, market_shops, market_tags. The other 10 cursor-eligible tools have none (market_trend uses from/to).
- Page-1 `snapshot_id`: probed with the built CLI: top-level in plain output, `results.snapshot_id` with `--agent` (not in meta); `--select` can drop it (it did for demand and tags).
- Cursor tools that always return a Preview (results is a single object; probed plain and `--agent`): market_tags, market_trend, money_reconcile, reviews_digest, shop_attention. The other 10 return `results` as a list of objects.
- Non-mirror truncation signals (read from tools.go/bound.go): typed endpoint and `etsy_*` tools use EndpointPageResponse/EndpointResponse, `search` and `context` use toolResultJSON -> bound.JSON (all `_pp_truncated`); `sql` builds its own envelope with `truncated: true` and no `_pp_truncated`.

### Round-3 live smoke (behaviour only, existing isolated store, 0 new Etsy calls)
A page-1 `market_demand` cursor went stale after a one-row synthetic edit to the scratch store; the error began `cursor_stale`, contained "without `cursor`" and the snapshot advice (market tool). `market_tags` over budget returned a Preview whose note says it cannot be paged and has no next_cursor text. The NoRerun note's tool names are all registered. The `orders_unshipped` stale case and the `learnings_list` note are covered by the hermetic seam/unit rows (the scratch store has no receipts).

## Final polish (base f8d4c19)
Items 6 and 7 proofs, round-3 method (bare `./internal/mcp/` package command, no `-run`; both CAUGHT):

| Mutant | File | Mutation | Knock-out row | Control | Control's own failure |
|---|---|---|---|---|---|
| P01_preview_opening_drops_cannot_be_paged | etsy_canonical.go | Preview opening loses "cannot be paged" | TestEtsyMCPBoundingSeam/reviews_digest_overbudget_preview_cannot_be_paged | M07 (tool check removed) | TestEtsyMCPBoundingSeam/cursor_wrong_tool |
| P02_snapshot_id_key_renamed | market_demand.go | result key `snapshot_id` -> `snap_id` | TestEtsyMCPBoundingSeam/market_demand_stale_message_has_snapshot_advice | P06 (`top` flag renamed) | TestEtsyMCPBoundingSeam/multi_arg_paging_window_compact_top |

A first P01 attempt with M05 (digest check removed) as control was refused (exit 2): that control does not fail any seam row, so it was inert there; M07 was used instead. The seam rows added are `reviews_digest_overbudget_preview_cannot_be_paged` and `money_reconcile_overbudget_preview_cannot_be_paged` (each asserts truncated, a preview, no `next_cursor` key and a note saying "cannot be paged"); item 7 extends `market_demand_stale_message_has_snapshot_advice` with an `agent:true` page 1 asserting `results.snapshot_id` and the plain page 1 asserting top-level `snapshot_id`.
Item 8 (help-path `max_bytes`) was skipped by design: the help path's primary block is the fixed message "command printed help instead of running", which cannot exceed the minimum budget; oversized help output is the secondary block, which is replaced by the drop marker (covered by the existing `help_huge` row of TestEtsyPerResponseBudget).
