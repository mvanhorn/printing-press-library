# TenderNed Novel Features Brainstorm

## Customer model

**Sanne — Bid manager at a mid-size Dutch civil engineering firm (~150 employees, Utrecht).**

*Today (without this CLI):* Sanne keeps tenderned.nl pinned in one tab and a Google Sheet of "live opportunities" in another. Each Monday morning she runs the same four searches by hand — CPV 45000000 (works), CPV 71000000 (engineering services), keyword "spoor", keyword "kademuur" — clicks through 3-5 result pages each, opens promising notices in new tabs, and copy-pastes title/closing-date/buyer into the sheet. She cannot answer "which of last week's notices are sub-€200k and would actually fit our team capacity?" without re-clicking each notice to read the value field.

*Weekly ritual:* Monday morning sweep of new publications across her firm's CPV stripes; Wednesday closing-deadline check ("what's closing in the next 10 days that we haven't responded to?"); Friday afternoon scan of one or two "watched" gemeenten that historically award her work.

*Frustration:* The sub-threshold long tail. Mercell shows above-threshold (EU TED) notices fine, but the €40k-€200k municipal jobs — her firm's bread and butter — require manually walking TenderNed page by page, because no off-the-shelf alert tool reliably filters on "national-only AND value-band AND CPV stripe."

---

**Joris — Procurement-market analyst at a public-sector consultancy (Den Haag).**

*Today (without this CLI):* Joris writes quarterly reports for ministries on "how is gemeente X buying ICT services compared to peers?" He scrapes TenderNed with a fragile Python script via the JSON endpoint, dumps results to CSV, and pivots in Excel. He has no persistent local store, so every analysis re-fetches the same publications. He cannot easily answer "show me the CPV-code drift for Rotterdam over the last 3 years" without re-scraping each time.

*Weekly ritual:* Pulls fresh publication data into his ad-hoc cache; rebuilds per-buyer dashboards in Excel; checks which awards (gegunde opdrachten) closed in the prior week to update his win/loss tracker per buyer.

*Frustration:* No way to do cross-entity queries. "Top 10 buyers in NUTS NL33 by tender count for CPV 72000000 in 2025" requires either three nested loops in Python or a paid Mercell subscription. The TenderNed API has no aggregation endpoint.

---

**Eva — Compliance & oversight officer at a mid-size gemeente (Eindhoven).**

*Today (without this CLI):* Eva has to verify that her own gemeente's tender publications are coherent across the lifecycle: every PIN (vooraankondiging) should have a CN (aankondiging van opdracht) within the announced window, every CN should have either a CAN (gunning) or a cancellation, and any modifications post-award should be linked. Today she checks this by clicking through her own buyer profile on tenderned.nl, eye-balling each publication's eForms code, and maintaining a spreadsheet of orphans.

*Weekly ritual:* Friday lifecycle reconciliation — pull all her gemeente's publications from the last 18 months, group by tender-thread (PIN→CN→CAN→Modification), flag orphans, and email the procurement officer responsible for each gap.

*Frustration:* The thread linkage exists in eForms XML (each notice carries pointers to predecessors) but is not exposed in the JSON list response. She has to download XML per-notice and parse it by hand.

---

**Pieter — Investigative data journalist at a national newspaper (Amsterdam).**

*Today (without this CLI):* Pieter chases stories like "which municipalities are over-relying on a single ICT supplier?" or "which awards went out via negotiated-without-prior-publication procedure?" He uses a mix of WOO-requests and ad-hoc tenderned.nl searches. He has no local store, no FTS, and no way to do "all awards to vendor X across all buyers" because TenderNed indexes by buyer, not by awardee.

*Weekly ritual:* One or two FOIA-adjacent investigations a week. Pulls a candidate buyer's last 200 notices, downloads the documents, greps the PDFs for vendor names, and cross-references against company-register data.

*Frustration:* No bulk grep across attached document bodies. The TenderNed UI lets him download docs one at a time, but a search like "every notice where the bestek mentions vendor X" is a multi-hour manual job.

## Candidates (pre-cut)

(16 candidates generated; see Survivors and Killed sections below for the final cut.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | Persona | Why only us |
|---|---------|---------|-------|--------------|---------|-------------|
| 1 | Buyer dossier | `tenderned buyers dossier <id-or-name>` | 9/10 | hand-code | Sanne, Joris | Cross-entity SQLite aggregation; no API aggregation endpoint |
| 2 | Sub-threshold lead finder | `tenderned leads --national --max-value <eur> --cpv <code...>` | 9/10 | hand-code | Sanne | TenderNed-unique: sub-EU-threshold tenders never reach TED |
| 3 | Closing-deadline view | `tenderned closings --within <N>d [--cpv ...] [--buyer ...]` | 8/10 | hand-code | Sanne | Local index on sluitingsDatum + buyer/CPV filter |
| 4 | Watch saved query | `tenderned watch add/list/run <name>` | 8/10 | hand-code | Sanne, Pieter | SQLite-stored cursor; "what's new since last run?" pattern |
| 5 | CPV drift over time | `tenderned cpv drift [--buyer <id>] [--nuts <code>] --years <N>` | 7/10 | hand-code | Joris | Historical CPV-mix shifts impossible without local snapshot |
| 6 | TED cross-reference | `tenderned ted-link <pubId>` | 8/10 | hand-code | Sanne, Joris | Extracts canonical TED publication number from eForms XML |
| 7 | Tender-thread reconcile | `tenderned thread reconcile --buyer <id>` | 7/10 | hand-code | Eva | PIN→CN→CAN→Modification chain only visible via XML predecessor refs |
| 8 | Buyer top-N aggregation | `tenderned buyers top --cpv <code> [--nuts <code>] [--since <date>]` | 8/10 | hand-code | Joris | Pure SQLite aggregation impossible via the API |
| 9 | Document grep | `tenderned docs grep <pattern> [--buyer ...] [--cpv ...] [--limit N]` | 7/10 | hand-code | Pieter | Bulk regex over downloaded bestek/PvE documents |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Awards-to-vendor reverse lookup | Requires bulk XML crawl of every CAN; awardee field not confirmed in JSON | Document grep (#9) partially covers it |
| RSS-to-SQLite live ingest | Thin wrapper over absorbed `feed latest` + `sync` | Subsumed by the foundation `sync` command |
| Procedure-mix breakdown | Subsumed by buyer dossier output | Buyer dossier (#1) |
| Buyer value-bands histogram | Subsumed by buyer dossier + leads | Buyer dossier (#1) + Leads (#2) |
| XML field extract (generic xpath) | Thin wrapper over `xml fetch` + `xmllint`; named use cases covered | TED cross-reference (#6) + Thread reconcile (#7) |
| Closing-soon watch alert | Composition of closings + watch — `watch run` over a closing query | Closings (#3) + Watch (#4) |
| Top buyers leaderboard | No named persona; Joris's real need is sliced top-N | Buyer top-N (#8) |
