# Ledger proof before the first press live leg (2026-10-05T15:17Z)

Owner rule: before the first press live leg, prove the >= 3 s gap and no-retry on discovery/uber-request-ledger.tsv.

## Controlled live run (real HOME, honest UA, stdlib transport, no base-URL override)
- uber-jobs-pp-cli postings --country GBR --limit 5 --sort recent --json --data-source live -> rc 0, 2 requests (probe, then one page sized 73), hits 23, returned 5, complete, newest first, contract fields present, descriptions HTML-free.
- uber-jobs-pp-cli facets --data-source live --json -> rc 0, 1 request; 38 countries (0 unmapped), 18 teams, 53 sub-teams, total 583.
- UBER_JOBS_REQUEST_LOG -> discovery/cli-requests-phase4-ledger-proof.tsv, merged into the ledger (tool "uber-jobs-pp-cli (phase 4 ledger proof, real HOME)"); ledger backup discovery/uber-request-ledger.tsv.bak-pre-phase4.

## Checks on the whole ledger for 2026-10-05
- 46 requests to uber.com/oraclecloud.com hosts today (uber.com 43, oraclecloud 3). Minimum gap between consecutive requests: 3.0 s. Pairs under 3 s: 0.
- Refusals today: two, both DIAGNOSIS before the runtime transport was chosen at 10:00:34Z, which the owner rule allows: 04:04:59Z (recon curl, 403 managed challenge) and 09:52:18Z (Phase 1.9 curl, 403 challenge). Zero refusals since the transport was chosen, so no request has followed a refusal on the chosen transport.
- No-retry on refusal is also proven offline (proofs/2026-10-05-140047-pacing-proof-offline.md: after a 403, later processes sent 0 site requests) and by tests (TestRefusalStopsAfterOneRequest, TestActiveLatchFromDiskBlocks, TestUJCGuardLatchesRefusedHost, TestUJCGuardRefusalSurvivesGeneratedRetry).

## Daily counters after this run
uber.com 43/300, Oracle 3/300.
