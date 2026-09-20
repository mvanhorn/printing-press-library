# NEPRA novel-features brainstorm — audit trail

Subagent: 202,413 tokens, 17 tool calls, 491 s. First print (no prior research).
16 candidates generated -> 10 survivors (>= 5/10) -> 6 killed.

## Customer model (Pass 1)

**Zoya Rehman — IPP equity analyst, Karachi brokerage.** Covers HUBC, KAPCO, NPL, NCPL,
LUCK, ALTN, PKGP. For an IPP, P&L is capacity payment on *availability* plus energy payment
as fuel pass-through, so load factor, availability/delicensing status and receivable days
move her target price. NEPRA's generation sheet is the only public monthly plant-level
source of the first two and she cannot open it — a naive UTF-8 read of the 493,187-byte
FY2023-24 file reports `<table>=0, <tr>=0, <td>=0` and the file looks empty. So she waits
for quarterly accounts or reads a think-tank PDF footered "Data Source: NEPRA SIR".
*Frustration:* no stable key across the seven sheets. KAPCO's S.No moved 9->33, Kohinoor
12->35, Narowal 11->58, and Narowal lost its `(HUBCO)` tag in FY2023-24 so a name join
silently drops the subsidiary she is modelling. HUBCO's 1,292 MW flagship went 49.24% load
factor -> 1.93% -> `DELICENSED`; KAPCO booked 1,601 MW installed / 1,345 MW dependable
against `0.00` GWh in all twelve months of FY2023-24 — the capacity-payment-without-
generation thesis in one row — and none of it is queryable anywhere.

**Faraz Iqbal — energy correspondent, business daily.** When the SIR drops, every outlet
prints the same six numbers within 48 hours (T&D 17.55% vs allowed 11.43%, Rs 265 bn,
recovery 96.62%, circular debt Rs 1.614 tn). He prints the national average because the
per-DISCO panel is on p.63 of a 266-page PDF, and the corpus is 610.6 MB across ten SIRs at
68,768–163,650 B/s — SIR 2025 alone is 316 MB, 34–80 minutes of download.
*Frustration:* cannot answer "which DISCO's loss gap widened most over five years and what
did it cost" on deadline — QESCO +7.43 pp costing Rs 69,534 M, PESCO Rs 208,191 M,
Rs 442,944 M system-wide — because that answer exists on no NEPRA page. Nor can he see what
was quietly rewritten: FY2022-23 agricultural sales moved -96.54 GWh between SIR 2023 and
SIR 2024 with a category renamed, and SIR 2023's growth column contradicts its own printed
units on 8 of 8 categories.

**Nadia Saleem — research associate, Islamabad energy think tank.** Her flagship annual
review carries 40 `Data source:` captions — 38 of 40 cite NEPRA, 40 of 40 say "our
calculations". Two people spend a publication cycle re-deriving what NEPRA already
published. Her own foreword says the transition is missed "largely because of the incomplete
and imprecise datasets available to them".
*Frustration:* cannot separate a restatement from a correction from a definitional break.
MEPCO's FY2024-25 SAIDI prints 3547.00 in Table 06 and 1182.56 in Table 18, each triple-
attested by its own chart, so one document supports both "Near to Limit" and "the only DISCO
ever to comply with SAIFI". MEPCO's FY2020-21 SAIDI is 39733 in one report and 39.733 in the
next three, exactly 1000x, never corrected. FY2023-24's own PER is a 404 on NEPRA's index.

**Imran Dar — finance manager, Faisalabad textile mill (B2 industrial).** His August bill
carries an FCA line and he cannot say which month it belongs to or what hits the next two
bills. The consolidated FCA table stops at Jun-2022. The schedule that legally applies to
him is one of 13 near-identical SROs dated 13.01.2026 — a 100-page, 3,078,192-byte file
yielding 149 characters per page, every one a repeated registrar signature stamp.
*Frustration:* the billing lag is systematic ("shall reflect the FCA in respect of June 2026
in the billing month of August 2026") but tabulated nowhere. Worse, frozen surfaces look
live: Quarterly Data ends at "April 2022- June 2022" with `Not Yet Issued` against every
DISCO; Hydel capacities pinned to "July 2018 - June 2022"; the consumer schedule frozen at
"Notified Tariff 01-01-2019". Nothing on any page says it is stale.

## Survivors, kills and buildability
See the transcendence and killed-candidate tables in the absorb manifest beside this file.
Buildability split: 6 hand-code, 4 spec-emits.
