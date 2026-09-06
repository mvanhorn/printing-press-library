# Itinerary workflow token measurements

Measured September 6, 2026 with tiktoken 0.14.0 and `o200k_base`. Static synthetic workloads plus actual CLI discovery stdout; not a model-quality evaluation or a billing guarantee.

| Task | Previous local approach | New approach | Change |
|---|---:|---:|---:|
| Three complete days, command + stdout | 4,938 tokens / 3 calls | 4,267 tokens / 1 call | 13.6% fewer tokens |
| Mixed block creation, preview + apply, including input file | 1,002 tokens / 6 calls | 614 tokens / 2 calls | 38.7% fewer tokens |
| Unchanged day delta, command + stdout | 275 tokens | 219 tokens | 20.4% fewer tokens |
| One-note day delta, command + stdout | 347 tokens | 296 tokens | 14.7% fewer tokens |
| Review discovery stdout | 4,597 tokens, entire previous inventory | 1,140 tokens, task workflow | 75.2% fewer tokens; narrower scope |

The complete current inventory costs 4,702 tokens (up from 4,597), and the current skill costs 2,681 tokens (up from the prior local candidate's 2,493). Existing `--for-edit` costs 3,809 tokens (previously 3,686). Improvements come from choosing task-specific entry points and sharing context, not making every surface smaller.

The three-day fixture repeats four places, day notes and checklists while retaining a global no-car constraint and flight booking. The combined read includes trip context and all non-day sections; ordinary single-day reads have a narrower context policy. Both preserve all selected-day constraints used in this fixture. Retention tests include admission cutoffs, closure status, missing travel estimates, checklist items and bookings.

The overview costs 3,844 command/output tokens for this fixture. It adds a trip-wide orientation step; it is not a replacement for full stop details. Its ordinary stop-note omissions are explicit, while complete note/checklist blocks, global context and booking constraints remain. Selected full-day reads follow it. Between-day links explicitly identify last/first routable places, do not infer a hotel return or overnight schedule slack, and become unknown when transport bookings intervene.

An illustrative first review costs 9,263 command/output tokens across three calls: task discovery, overview, then all three detailed days. Including the skill once makes 11,944 tokens. Expanding all three days is intentionally a coverage workload; an agent can narrow to the days relevant to its decision. This is not asserted as a reduction against an equivalent previous whole-trip review, and it excludes model reasoning and tool framing.

Creation measurement uses real report writers and builders with simulated successful receipts, deterministic synthetic IDs and the same compact JSON rendering on both sides. It charges the 73-token batch input file once at preview and reuses it at apply. The actual private-fixture test separately confirmed a real place/note/checklist transaction, readback of returned IDs, and explicit journal undo restoring the full day snapshot. The published benchmark contains no private fixture content.

Delta schema 2 inherits unchanged components, order and warnings from the matching baseline. Explicit empty fields clear values; reconstruction tests verify the complete resulting digest. The consumer must possess the complete baseline: CLI access to a saved file does not restore model context. Old, malformed or incompatible state returns a full snapshot.

Reproduce with `WANDERLOG_TOKEN_BENCH_DIR` and `TestPlanFlowTokenWorkload`, `TestPlanDayTokenWorkload`, `TestCreateBatchTokenWorkload`, then `scripts/token-benchmark.py`. Capture actual `agent-context --task review|create|edit --agent` stdout as flow-task-TASK.json. Compare skill text separately; no silent dependency installation. Historical day workflow numbers from the first pass remain separately identified in token-efficiency.md.

Limitations: tokenizer proxy; synthetic rather than production distribution; no reasoning, tool framing, provider pricing/cache discount, latency, or model-planning success measurement. CLI calls are not HTTP requests. State deltas still fetch live API data.

## Final verification

All Go tests passed. Final live acceptance: 272 passed, zero failed, 183 skipped/unverified. All 12 publication checks passed, including build, vet, govulncheck and source-matched acceptance. Mixed block creation, stable-ID readback and explicit journal undo restored the private synthetic fixture day exactly. The skill verifier and whitespace check passed. No global installation or publication.
