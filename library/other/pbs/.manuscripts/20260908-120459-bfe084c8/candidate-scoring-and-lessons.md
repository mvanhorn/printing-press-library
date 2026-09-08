# Pakistani Printing Press candidates — scored table, 2026-09-08
Registry screened live: 504 entries / 22 categories. 3 PK CLIs exist, all mine (psx, nccpl, daraz).
Axes 1-5. Uniqueness weighted highest per brief. Legal is treated as a GATE, not just a score.

| # | Candidate | Uniq | Reach | Legal | Durab | Verdict |
|---|---|---|---|---|---|---|
| 1 | PBS (weekly SPI city panel)   | 5 | 5 | 5 | 3 | BUILD |
| 2 | SBP (auction yield curve)     | 5 | 5 | 3 | 4 | BUILD |
| 3 | NEPRA (DISCO perf + gen HTML)  | 5 | 5 | 4 | 3 | BUILD |
| 4 | PPRA (tender + award join)     | 5 | 4 | 5 | 3 | BUILD (one flag) |
| 5 | OGRA (geocoded pump prices)    | 5 | 4 | 4 | 3 | BUILD |
| 6 | PakWheels (used-car hedonic)   | 5 | 5 | 2 | 4 | BUILD / ToS conflict |
| 7 | PTA (per-operator monthly)     | 5 | 5 | 2 | 3 | BUILD / ToS conflict |
| 8 | DRAP (MRP forward capture)     | 4 | 5 | 3 | 2 | BUILD scoped |
| 9 | FBR (aggregate revenue)        | 4 | 4 | 2 | 3 | WEAK |
|10 | PMEX (futures OI/volume)       | 4 | 1 | 1 | 1 | DEAD - managed CF challenge 26/26 |
|11 | Bookme (bus fares)             | 4*| 2 | 1 | 2 | DEAD-BY-POLICY |
|12 | PriceOye                       | 1 | 4 | 2 | 3 | DEAD - premise refuted |
|13 | OLX PK                         | 2 | 3 | 1 | 2 | DEAD-BY-POLICY - PII inline |
|14 | SECP                           | 3 | 1 | 1 | - | DEAD-BY-POLICY + CF-blocked |
|15 | Sastaticket                    | 1 | 3 | 1 | 2 | DEAD-BY-POLICY |
|16 | Graana                         | 1 | 2 | 2 | 2 | DEAD - fails Zameen bar |
* Bookme uniqueness is POTENTIAL only; unreachable within policy.

## Unique output, per surviving candidate
PBS   128,316 city-item-week price obs (17 cities x 51 items x min/avg/max x 148 weeks)
      + item weight vector republished weekly. LIVE SITE DEEPER THAN WAYBACK.
SBP   4,936 auction rows; MTB WAY from 1998-06-24, PIB from 2000-12-14; per-tenor cut-off
      from 2014. Plus BID-LEVEL ladders (523/423/338 bid lines measured) = full demand curve.
NEPRA 10 DISCOs x FY x ~14 metrics incl T&D loss vs TARGET, SAIFI, SAIDI, recovery %.
      + 134 plants x 12 months generation as CLEAN HTML (12,864 plant-months, no PDF).
PPRA  ~1,860 live + ~1,824 archived notices + 568 AWARDS joinable on Tender No =
      estimate vs awarded vs winning firm. Exists nowhere.
OGRA  40 OMCs x date x INDIVIDUAL PUMP (lat/long, dealer, tehsil, district), 6,762 docs
      back to 2017-02-16. + daily Platts->pump formula build-up (~6 months only).
PKWhl 81,221 listings, 30 JSON-LD Products per fetch, full hedonic vector; expired ads keep
      URL at 200 but LOSE the price -> local store is the only record.
PTA   5 operators x 6 monthly vars = 44 series; PTA keeps only a ROLLING 12 MONTHS.
      37 contiguous months recoverable now from 15 Wayback editions.
DRAP  21,366 MRP rows keyed (RegNo,PackSize) + 76,230-row registry in ONE request
      + SRO/recall archive to 2002-02-14. Historical MRP = DEAD.

## Hard negatives ESTABLISHED (do not re-ask)
- NEPRA per-DISCO FCA table: DEAD. OCR garbage ("IEICO Lts$) MEPCd flSCO"), find_tables()=0,
  AND the CPPA-G petition carries no DISCO breakdown at all. Only national FCA survives.
- OGRA notified petroleum prices: Konica photocopier scans, 0 text operators, 2015..2026-08-29.
- PTA MNP monthly volumes: DO NOT EXIST. 0 hits in the 118-pg 2024 report; 2025 report
  mentions portability twice, both as regulations under review.
- DRAP historical MRPs: 1 Wayback capture; /cp/RDI_Pricing.svc 500 on 4/4 (permanently broken);
  rd Products frozen 2023-01-31.
- PMEX: managed CF challenge 26/26 incl static PDFs; robots.txt UNREADABLE so no crawl
  permission establishable; upload-folder year != report year on 7/33 recovered files.
- SECP: register is OAuth-gated (personal data); aggregate stats behind CF challenge Go can't
  pass; robots has ClaudeBot Disallow + ai-train=no.
- PriceOye "prices across sellers": REFUTED. 36/36 rows priced by PriceOye itself,
  30/36 zero dispersion, 20/36 stamps predate 2026.
- Graana price index: does not exist. 7 candidate URLs all 404. robots disallows Go-http-client.
- FBR: Excel discontinued after FY2021-22; ToS licence scoped to "residents and citizens of
  Pakistan" and grants copying "without any deletion, addition or modification".

## Governance summary
PBS   robots `Disallow:` empty + WRITTEN OPEN-GOVT LICENCE permitting COMMERCIAL reuse w/ attribution
PPRA  ppra.gov.pk robots = "User-agent: *" 14 bytes, no Disallow; others 404. Statutorily public.
NEPRA/OGRA  no ToS at all; Cloudflare STOCK block: ClaudeBot/GPTBot/CCBot/Google-Extended
      Disallow + ai-train=no. Structurally identical to CDC, where owner elected to proceed.
SBP   no robots retrievable (403 Signature ID 1030010095); "All Rights Reserved"; NO prohibition.
      CARVE OUT KIBOR: PDFs state "Data Source: Refinitiv" (third-party licensed).
DRAP  robots permissive, no ToS, BUT in-app disclaimer says data "cannot ... be used for
      ... research, citation, statistical analysis".
PTA   robots permissive BUT ToS: "internal, personal, noncommercial purposes" +
      "Copying, redistribution ... strictly prohibited" citing PECA 2016 (criminal statute).
PKWhl robots friendly, NO AI directive at all, BUT ToS bans "automated means including robot,
      deep link, page scrape".

## PPRA FLAG — needs an explicit decision
EPADS JSON API (apiprd.eprocure.gov.pk) authenticates with a Basic key hardcoded in the
public Angular bundle: `Authorization: Basic <REDACTED>` -> decodes to a hardcoded admin-named credential (value withheld)
(decoded locally, confirmed). Every browser visitor sends it; it is not a personal account.
BUT publishing a CLI that hardcodes an admin-named credential is a different act from a
browser sending it. MITIGATION AVAILABLE: federal EPMS (epms.ppra.gov.pk) is clean
server-rendered HTML with NO VIEWSTATE and NO credential, and it carries the awards +
the Tender No join key. So the unique output is obtainable WITHOUT the credential.
Recommendation: build on EPMS HTML; leave EPADS out, or behind an explicit opt-in flag.

## Generalizable engineering findings (for the handoff)
1. DASH CONVENTION DIFFERS ON EVERY SOURCE, AND WITHIN SOURCES. CDC=zero. MUFAP=missing.
   PBS: weekly 0=missing / monthly blank=missing / AppxB "N.A.". SBP: TWO dashes, opposite
   meanings, IDENTICAL ON SCREEN (53 numeric 0 under accounting format `_(* "-"??_)` + 1
   literal "-"; a CSV export collapses both). FBR=zero (proven by summation). NEPRA PDFs
   =zero (proven arithmetically) but NEPRA's own SOI HTML uses DELICENSED/DECOMMISSIONED.
   OGRA dash means THREE things. There is no safe default; measure per SURFACE, not per site.
2. NEPRA `(0)` is a ROUNDED SMALL NEGATIVE, not zero (55,794,935 -> 55,794,934, delta -1).
3. IDENTITY MUST COME FROM THE PAYLOAD, NOT THE REQUEST. PakWheels expired ad ...-Karachi-62
   returns HTTP 200 serving four JSON-LD Products for OTHER live Daihatsu Charades in Karachi
   (1987/1985/1986/1986 @ 450k/270k/220k/300k). Same make, model AND city -> no sanity check
   can catch it. Only asserting offers.url ends with the requested ad ID disambiguates.
4. BEYOND-END PAGINATION REPEATS SILENTLY. PakWheels ?page=4000 returned 31/31 identical ad
   IDs to ?page=3249 at HTTP 200 while still claiming ~81,220 total. DRAP ?page=abc and
   ?page=-1 silently serve page 1. Stopping rule must be page-content identity.
5. STATUS FIELDS LIE. PakWheels offers.availability = InStock on 100% of blocks, live AND
   expired. Gate on the "no longer active"/ViewSold marker instead.
6. IN-DOCUMENT INVARIANTS ARE FREE VALIDATORS AND BOTH ENERGY REGULATORS SHIP THEM.
   NEPRA states D=B+C, G=E+F, I=B+E, J=C+F, K=I+J. OGRA states C={A+B}. Every extractor on
   both sites produced confident, plausible, WRONG numbers until validated against these.
   Treat a broken invariant as a PARSE FAILURE, never a data point.
7. FONT-MAP CORRUPTION WITH NO ERROR SIGNAL (OGRA): merged ToUnicode maps turned 385.28 into
   38D.28 and 360.53 into 360.D3. Per-page font resolution is mandatory.
8. OCR CORRUPTS DIGITS AS LETTERS (DRAP): `Rs.l036` (letter l) -> naive Rs\.?([\d,]+) yields
   036, silently wrong by 10x. Also Indian lakh grouping: Rs.1,26,000 = 126,000.
9. PURE-GO PDF EXTRACTION WORKS ON NEPRA: ledongthuc vs MuPDF = 93-97% parity over 6 docs,
   0 panics, across Foxit/Adobe/OmniPage. No PDFKit/swift bridge needed. Contrast CDC.
10. SOFT-404s AT HTTP 200, AND HASHING CANNOT DETECT THEM. SBP missing PDFs return 200 +
    text/html whose body ECHOES THE REQUEST in <link rel=canonical>, so every soft-404 has a
    DIFFERENT hash. Only Content-Type works. OGRA soft-404s redirect to /no-found/404 at 200.
11. HTTP 200 + `[]` (2 bytes) is indistinguishable from a bogus request on SBP
    slug=foreign-exchange-reserves (byte-identical sha 4f53cda18c2b to a junk slug).
12. CHANGE DETECTION VIA HASH IS BROKEN BY CLOUDFLARE EMAIL OBFUSCATION: NEPRA returns a
    different sha every request for a byte-identical 306,343 B page (rotating data-cfemail
    XOR key). Normalise cfemail + beacon token first. PTA rotates a CSRF token identically.
13. SBP SENTINEL ALPHABET: 2,040 non-numeric cells in numeric columns (BR 601, NBR 530,
    Bids Rejected 361, No Bid Received 338), self-documented in the file's own footnotes, and
    THE ENCODING DRIFTS WITHIN ONE FILE BY VINTAGE (PIB accepted-face: numeric 0 in 2024,
    "-" early 2025, "BR" Mar 2025 for the same state). Coercing non-numeric to NULL destroys
    3 distinct states and silently drops 123 of 997 MTB rows.
14. SBP PDF TEXT LAYER INSERTS A SPACE AFTER THE LEADING DIGIT of any >=5-digit number
    ("1 4,709.700"). Reproduced on 40+ rows. Shifts every downstream column.
15. PTA operator column order is NOT stable within one document (Table 2.1 = Jazz ZonG
    Telenor Ufone; Table 4.1 = Jazz Telenor Ufone ZonG). Positional parsing swaps operators.
16. APPEND-ONLY CAN BE PROVEN, NOT ASSUMED: SBP archive vintage (977 rows) vs live (997)
    = 20 added, 0 dropped, 0 of 977 common values restated. Do this before trusting any
    overwritten-in-place cumulative file.

## ============ PBS CORRECTIONS — my own re-measurement, 2026-09-08 ~11:5x ============
I re-verified the PBS agent's three decisive claims directly against
scratchpad/pbs/Annex_03.09.2026.xlsx and SPI-Monthly-Prices-Annex-8.xlsx.
HOLDS: robots.txt is `User-agent: *` / `Disallow:` (empty = allow all), verbatim.
HOLDS: the release index IS a JS literal in the served HTML (`const data = [...]`),
       211 'annexure' occurrences, 141 .xlsx, 379 .pdf. NEW: there is a SECOND array,
       `cpidata1`, which the agent did not mention — the CPI index is separate from the
       weekly index. Do not assume one array covers both surfaces.
HOLDS: 17 cities, and Appendix-B uses the string sentinel `N.A.` (seen directly in rows
       2, 3, 4: 'Other Urea' N.A., 'Calcium Ammonium Nitrate' N.A., 'S.S Phosphate' N.A.).

**CORRECTED — the clean dash dichotomy is FALSE.** The agent reported "Weekly file: 3 zeros,
0 blanks. Monthly file: 3 blanks, 0 zeros." Measured by GRID POSITION (a blank cell is
OMITTED from the xlsx XML entirely, so counting existing <c> elements cannot see blanks —
that was my first error too, and it reproduced the agent's answer):
    WEEKLY  Appendix-A : numeric-0 = 43   blank/absent = 51   real numerics = 3272
    MONTHLY annex      : numeric-0 = 11   blank/absent =  3   real numerics = 1108
BOTH files contain BOTH conventions. The rule is NOT "weekly=0, monthly=blank". Zero and
blank must be carried as DISTINCT nullable states in BOTH surfaces, and the semantics of
each must be established per (surface, column-block), not per file.

**MISSED BY THE AGENT, AND IT IS THE MOST DANGEROUS FEATURE: the weekly Appendix-A is not
one wide table. It is THREE STACKED ROW-BLOCKS, each with its own repeated city header.**
Merged-cell map of sheet1 (30 merges total):
    rows   1-3  : D:X title, D:X title, then D:F G:I J:L M:O P:R S:U V:X  = 7 cities x 3 cols
    rows  59-61 : D:X, D:X, then D:F G:I J:L M:O P:R S:U V:X              = 7 cities x 3 cols
    rows 117-119: D:X, D:X, then D:F G:I J:L M:O + P:Q R:S T:W            = IRREGULAR
    7 + 7 + 3 = 17 cities.
Consequences, all binding on the build:
  (a) A parser that reads ONE header row and applies it to all 153 data rows will assign the
      WRONG CITY to roughly two-thirds of the panel — silently, with plausible values.
      City must be resolved from the NEAREST PRECEDING header block, per row.
  (b) Block 3's merges are NOT uniform triplets: P:Q and R:S span 2 columns and T:W spans 4,
      against 3 everywhere else. Never hardcode a 3-column stride.
  (c) The data column span is A..Y = 22 usable columns, i.e. ~7 cities per block — NOT
      17 x 3 = 51 columns. Anyone sizing the parser at 51 columns is reading the wrong shape.
  (d) These are exactly the spanning colspan/rowspan headers that press `mode: table`
      mis-keys (cli-printing-press#4279). Confirms a HAND-WRITTEN parser is mandatory.
  (e) Header cells appear to use INLINE strings (<is><t>) not sharedStrings: a reader that
      only looks at <v> sees None for every city name. City names ARE in sharedStrings too
      ('Islamabad (01)', 'Gujranwala (03)', 'Lahore (05)', 'Faisalabad (06)') — with the
      (NN) city codes present in the WEEKLY file, so the agent's "monthly drops the codes"
      note is a monthly-only observation, not a general one. Soft-hyphens ('Gujran-wala',
      'Faisal-abad') appear in APPENDIX-B, not only in the monthly.
LESSON THAT GENERALISES: the extraction-quality check must be run from OUTSIDE the parser
(count cities x items x blocks recovered vs the 17 x 51 the source claims), exactly as the
CDC ISIN-recovery measurement was. An inside-out check would have passed all of the above.

## ==== PBS ROUND 2: index-array ground truth, measured by me 2026-09-08 ~12:1x ====
Parsed both JS literals out of the served HTML directly. CONFIRMS the agent on depth and on
the Feb-2024 loss; CORRECTS it on eight points that change the build.

CONFIRMED: 148 weekly releases, MIN 2023-07-13, MAX 2026-09-03. Coverage 148/165 = 89.7%.
CONFIRMED: the Sep-Oct 2024 hole is real and is 2024-09-12 -> 2024-11-07, a 56-day gap =
  7 missing weeks (agent said "6-week block"; it is 7). 17 missing weeks in total.
CONFIRMED FROM THE INDEX ITSELF: CPI_Monthly_Prices_Annex_0.pdf is listed under BOTH
  "March 2024" and "February 2024" -- the only duplicated annex URL in the 50-month array.
  February 2024 is lost. This is now verified from two independent directions.

CORRECTIONS THAT CHANGE THE BUILD:
1. **THE ARRAY IS NOT SORTED.** array[0]=2026-09-03 but array[-1]=2024-08-08, while the true
   MIN is 2023-07-13. `strictly descending? False`. Reading array[-1] as "oldest" understates
   depth by two years. Sort by the PARSED date, never trust array order.
2. **ONLY 44 OF 148 ROWS HAVE XLSX.** annexureExcel/reportExcel are present on 44/148; the
   other 104 are PDF-only (newest row without xlsx = 2025-10-16). So the text-layer PDF parser
   covers 70% of the panel and is NOT optional. The agent framed xlsx as the main path.
3. **35 DISTINCT ANNEXURE FILENAME SHAPES**, not "5 date encodings". Includes `Annex.pdf`
   with NO DATE AT ALL, `SPI##Annexture_##.##.##########USCP-merged.pdf`, and typos
   SumarySPI / SummaryReport / USCCP / Annexture / Annexture-USCP. Top shape covers only 53/148.
4. **THE `date` FIELD IS AUTHORITATIVE, NOT THE FILENAME.** 2 rows disagree:
   date=21-08-2025 -> Annex_13.08.2025.pdf (8 days off) and date=04-06-2025 -> Annex.pdf.
   The agent's "classify by FILENAME, never by JSON key" is right about the annexure/report
   KEY SWAP but wrong about the DATE. Both rules are needed and they point opposite ways:
   use the `date` field for the as-of date; use the filename to tell annexure from report.
5. **URL CONVENTIONS ARE MIXED INSIDE ONE ARRAY.** weekly: 115 relative-without-leading-slash
   ("wp-content/uploads/...") vs 33 absolute ("https://www.pbs.gov.pk/..."). cpi: 50/50
   absolute. A naive urljoin on the relative form silently produces a wrong path.
6. **ALL 202 FILES SIT IN ONE CONSTANT DIRECTORY `/wp-content/uploads/2020/07/`.** The upload
   path carries ZERO date information -- there is no dated directory tree to walk. Contrast CDC,
   where /assets/uploads/YYYY/MM/ was informative.
7. **RELEASES ARE NOT ALWAYS THURSDAY.** 130 Thu, 9 Wed, 5 Sat, 2 Sun, 1 Tue, 1 Fri. Any
   "expected Thursday" gap detector produces 18 false gaps.
8. **cpidata1 carries Urban/Rural CONSTRUCTION xlsx on only 13 of 50 months**, and the key is
   literally `Urban ` WITH A TRAILING SPACE while its sibling is `Rural`. Key normalisation is
   required or 13 months of a second dataset are silently dropped.

ALSO NEW, from the xlsx sharedStrings (159 strings, read directly):
9. The weekly Appendix-A carries MORE than 17x51x{MIN,AVG,MAX}: also `National Ave.`,
   `% Change over` {`Prv. Wk`, `Cor. Wk`}, and `Yearly Average Prices` {`25-26`, `24-25`,
   `Diff`, `% Chng`}. The panel is wider than the agent described.
10. **Appendix-B is FIVE sub-tables**, not one: A Fertilizers (9 products, 50kg/bag),
    B Cement, C CNG (per litre in Punjab, per kg otherwise -- unit varies BY REGION),
    D **WAGE RATES** (Painter, Mason (Raj), Labourer, Plumber, Electrician; Daily / P-Point),
    E Wheat Rates (Wheat 10kg, Wheat Flour Fine 1kg). A weekly city-level WAGE series is a
    second unique panel the agent placed only in the monthly CPI annex.
11. The file DOCUMENTS ITS OWN SENTINEL: string 112 is literally
    "N.A. stands for Not Available." So N.A. = Not Available is sourced, not inferred.
12. Soft-hyphen city spellings (`Islam-abad`, `Gujran-wala`, `Faisal-abad`, `Sar-godha`,
    `Baha-walpur`, `Hyder-abad`, `Pesha-war`, `Khuz-dar`, `Rawal-pindi`) appear in the WEEKLY
    file's Appendix-B, not only in the monthly annex as the agent said. Both spellings ship in
    ONE file: Appendix-A uses "Islamabad (01)" with codes, Appendix-B uses "Islam-abad" without.
13. An `&amp;` HTML entity survives into a sharedString ("Nitro. Phosph. &amp; Pot. (Npk)").
    Needs entity unescape -- the cliutil.CleanText / `&#39;` bug class.
14. A footnote marker lives INSIDE a label: `Electricity Charges for Q1*`. Same class as CDC's
    FREEZE/`*`/`***` markers inside name strings.
15. Header cells concatenate MULTIPLE dates with runs of whitespace:
    "Average Price for      03-09-26 27-08-26 04-09-25" and "% Change over   27-08-26 04-09-25".
    A header parser must split these, and note the 4 renderings in ONE file:
    03-09-2026 / 03.09.2026 / 03-09-26 / 27.08.2026.
VERIFIED CITY LIST (17, with codes): Islamabad 01, Rawalpindi 02, Gujranwala 03, Sialkot 04,
Lahore 05, Faisalabad 06, Sargodha 07, Multan 08, Bahawalpur 09, Karachi 10, Hyderabad 11,
Sukkur 12, Larkana 13, Peshawar 14, Bannu 15, Quetta 16, Khuzdar 17.
VERIFIED ITEM COUNT: 51 item descriptions in Appendix-A. Matches the claimed 51 exactly.

## ==== PBS ROUND 3: PDF viability + the two-file architecture, measured 2026-09-08 ~12:2x ====

**PURE-GO PDF EXTRACTION WORKS ON EVERY PBS VINTAGE. No PDFKit/swift bridge needed.**
github.com/ledongthuc/pdf against 7 real files spanning 2023-07-13..2026-09-03:
  Annex_03.09.2026.pdf            4 pages, 4 ok, 0 err, 32,641 chars, 3,243 2dp-nums, 15ms
  m_Annex-29-05-2025.pdf          4/4 ok, 32,627 chars, 3,240 nums, 13ms
  m_SPI-Annex-USCP_17042025.pdf   5/5 ok, 33,550 chars, 3,257 nums, 19ms
  m_SPI-AnnexUSCP_12122024.pdf    5/5 ok, 33,671 chars, 3,254 nums, 19ms
  m_SPI20Annex26USCP_21032024.pdf 5/5 ok, 33,752 chars, 3,294 nums, 22ms
  old_annex_13072023.pdf          5/5 ok, 33,840 chars, 3,299 nums, 23ms
  cpi_dup0.pdf                    1/1 ok,  9,842 chars, 1,120 nums, 4ms
ZERO panics, ZERO page errors, 4-23ms. This is the NEPRA outcome, not the CDC outcome, and it
makes the build pure Go and cross-platform. Critical, because 104 of 148 releases are PDF-only.
CORRECTION to the agent: the PDFs are NOT image-free. The 4MB annexures carry 11-15
/Subtype /Image objects. They extract perfectly anyway, so the images are decorative
(logos/charts), NOT scanned pages. "0 images" is true only of the ~430KB recent files.

**EXTRACTION QUALITY MEASURED FROM OUTSIDE THE EXTRACTOR** (vocab ground truth taken from the
xlsx sharedStrings, then searched for in the PDF-derived text — the CDC ISIN-recovery method):
  newest 2026-09-03: cities 17/17 (100.0%)  items 51/51 (100.0%)
  oldest 2023-07-13: cities 17/17 (100.0%)  items 50/51 ( 98.0%)
The single miss is `Gas Charges for Q1`, and it is NOT an extraction failure —
**THE ITEM BASKET IS NOT CONSTANT ACROSS THE 148 RELEASES.** That item is absent from the 2023
release and present in 2026. Consequence: never assume 51 items per release; carry the observed
item set per release and treat basket membership as itself a tracked series.
`N.A.` appears 30x in the 2023 text and 27x in the 2026 text, so Appendix-B ships in the PDF too.

**ARCHITECTURE CORRECTION: EACH RELEASE HAS TWO DATA FILES, NOT ONE.**
`TOTAL` occurs ZERO times in the annexure text. The weight vector and the TOTAL invariant live
in the SPI **REPORT** file (`report` / `reportExcel`), not the annexure. Read of
3.-SPI-Report-03.09.2026.xlsx (157 sharedStrings, 3 sheets) shows the report carries:
  * QUINTILE INCOME BANDS with real thresholds: Q1 "Upto Rs. 17,732", Q2 "17,733-22,888",
    Q3 "22,889-29,517", Q4 "29,518-44,175", Q5 "Above 44,175". These get REVISED, so tracking
    the bands across 148 releases is itself a unique income-threshold series nobody holds.
  * "Weight of expenditure group in % / Lowest Combined" AND
    "Impact of expenditure group in % points / Lowest Combined" -> weights AND impacts,
    each in a Lowest and a Combined variant (four columns, not two).
  * The three ranked sections WITH COUNTS: "the following 17 items registered INCREASE",
    "7 items registered DECREASE", "27 items remained UNCHANGED". 17+7+27 = 51.
    **FREE INVARIANT: the three section counts must sum to the release's item count.**
  * A rolling 10-week SPI trend table.
  * MONTHLY, QUARTERLY and HALF-YEARLY SPI aggregations (strings 89/105/110) - three further
    surfaces the agent never mentioned.
  * Only ONE `TOTAL` shared string, not three. A shared string is stored once and referenced
    N times, so one string legitimately renders as three rows. Counting sharedStrings is NOT
    counting rows - do not use string frequency as a row count anywhere.
CONSEQUENCE FOR THE BUILD: sync must fetch BOTH files per release (annexure -> city x item
panel; report -> quintile indices, weights, impacts, section counts, income bands) and the
store needs both. A one-file-per-release design silently drops the entire weight/index side.

## ==== PBS ROUND 4: the header-group bug (generalizable) — 2026-09-08 ~12:4x ====

**BUG I SHIPPED AND THE TEST CAUGHT: adjacent header groups with near-identical titles.**
Appendix-A's third band is not only cities. Resolved from the merge ranges with inline strings
read properly (my earlier Python could not read inline strings and mis-reported the columns):
    row 119: D=Bannu (15)  G=Quetta (16)  J=Khuzdar (17)
             M:O = "National Average"      subs MIN / AVG / MAX      <- the CURRENT week
             P:Q = "National Ave."         subs Prv. Wk / Cor. Wk    <- LAST week, LAST year
             R:S = "% Change over"         subs Prv. Wk / Cor. Wk
             T:W = "Yearly Average Prices" subs 25-26 / 24-25 / Diff / % Chng
For item 3 the row reads M=110  N=154  O=200  P=153.98  Q=155.51  R=0.01  S=-0.97
                          T=155.74  U=159.01  V=-3.27  W=-2.06
My first implementation collected every column whose HEADER matched /national/ into one flat
list and kept the last present value. It therefore returned **153.98 — LAST WEEK's national
average — while the correct current figure is 154.00 at column N.**
A 0.02 numerical difference and a COMPLETELY DIFFERENT VARIABLE. No range check, no sanity
check, and no eyeball on the number would ever catch that: 153.98 is a perfectly plausible
price for rice. Only resolving each merge span to its own group with its own sub-labels fixes it.
GENERALIZES TO: any source with sibling header groups whose titles differ only by abbreviation
("National Average" vs "National Ave."). Key on (group title + sub-label), never on a substring
of the title alone. This is the same family as the PakWheels similar-ad substitution — a wrong
value that is correctly typed, plausible, and adjacent to the right one.
The fix also RECOVERED FOUR SERIES the first pass was silently discarding: previous-week and
corresponding-week national averages, the percent change over each, and two fiscal-year
averages with their difference and percent change. They are now captured as named derived
series per item, read through and never recomputed.

**MY OWN EARLIER ERRORS, corrected here so they are not repeated:**
- I reported "44 of 148 releases have xlsx". The real number is **48**: 44 rows carry the
  `annexureExcel` key, and 4 more (2025-10-16, 2025-07-31, 2025-07-24, 2025-07-17) point at an
  .xlsx from the plain `annexure` key. Counting the KEY understates the xlsx era by 4 releases;
  classifying by FILENAME finds all 48.
- I reported the two filename/date mismatches as "one 8-day, one Annex.pdf with no date".
  Wrong: `Annex.pdf` agrees vacuously. The real second one is on the REPORT key —
  2026-02-19 -> Executive-Summary-SPI-Report_20.02.2026.pdf, a 1-day disagreement. My earlier
  check only inspected the annexure key, so it saw one of the two.
- The CPI array uses TWO month spellings: 47 full ("August 2026") and 3 abbreviated
  ("Feb 2026", "Jan 2026", "Aug 2025"). Parsing only the full form silently dropped 3 months
  AND their construction files, which is why a construction count read 10 instead of 13.
- The PDFs are NOT image-free: the 4MB annexures carry 11-15 image objects. They extract
  perfectly, so the images are decorative, but "0 images" was wrong as stated.

**PARSER STATE: 20 tests green**, including the three that matter most —
  * all three stacked bands contribute rows AND partition the cities disjointly
    (a single-header parser still yields 17 city names and plausible numbers while
     misattributing two thirds of the panel; only the partition test catches it)
  * the -16.6% null-coercion case: 14 present + 3 zero cells for Rice IRRI-6/9, mean 155.86
    excluding zeros vs 128.36 including them
  * min <= avg <= max on every complete triplet: 0 violations across >500 triplets

## ==== PBS ROUND 5: what only the LIVE run found — 2026-09-08 ~14:xx ====

**FOUR upstream file collisions, not one.** I found the Feb-2024 one by hand; the live
`releases` run found three more, and each means one release's data is unrecoverable:
  CPI_Monthly_Prices_Annex_0.pdf -> cpi-monthly:2024-02-01 + cpi-monthly:2024-03-01
  Annex_13.08.2025.pdf           -> spi-weekly:2025-08-13 + spi-weekly:2025-08-21
  4.-Annex-03-07-2025.pdf        -> cpi-monthly:2025-06-01 + spi-weekly:2025-07-03
  Monthly-Review-June-2025.pdf   -> cpi-monthly:2025-06-01 + spi-weekly:2025-07-10
The second one EXPLAINS the 2025-08-21 filename/date mismatch: PBS pointed the 2025-08-21
row at the 2025-08-13 file, so the 2025-08-21 week was never actually published. Two
collisions cross SERIES (a CPI month sharing a file with an SPI week), which no
single-series analysis would ever surface.

**THREE filename/date mismatches, not two.** 2026-02-19, 2025-08-21 and 2025-06-01. The
third is a CPI month, so a weekly-only check sees two.

**The weekly Appendix-A has 864 complete min/avg/max triplets per release**, not 51x17=867.
Measured identically on all four synced releases. The 3-triplet shortfall is the item whose
cells are uncollected in some cities — consistent, not an error.

**`verify` national-average median deviation: 0.419% to 0.453% across four releases.**
This independently reproduces the 0.45% figure measured during discovery by a completely
different route (a Python rebuild off the raw xlsx). Two methods, same answer.

**`movers` reproduces PBS's own published percent change.** Recomputed +26.30% WoW for
Onions from stored levels; the Bureau's own report sheet states 26.3 for the same item and
week. So deriving percentages from levels is not just a workaround for the zeroed percent
columns — it is verifiably equivalent where the source does publish a figure.

**Only 1-3 of 51 impact values per release are non-zero.** Measured on four releases
(2, 3, 2, 1). This is the quantified form of the "percent/impact columns are published as
zero" trap: it is not that they are ALL zero, it is that essentially all are, so any
analysis keyed on them silently gets nothing.

## Generalizable engineering lessons added this session
17. ADJACENT HEADER GROUPS WHOSE TITLES DIFFER ONLY BY ABBREVIATION. "National Average"
    (current, min/avg/max) sits beside "National Ave." (previous week / last year) in the
    same header row. Substring-matching the title returned last week's figure as this
    week's: 153.98 where 154.00 is right. Key on (group title + SUB-LABEL), never on a
    substring of the title. Same family as the PakWheels similar-ad substitution.
18. A SECTION THAT RESTARTS ITS OWN NUMBERING WILL BE ABSORBED BY THE PRECEDING SECTION'S
    COLUMN MODEL. Appendix-B's fertilizer and wage tables restart at item 1 and were
    attributed to Appendix-A's cities, emitting an electrician's wage as the price of beef.
    Two independent guards are cheap: an explicit section-marker reset AND a monotonic
    item-number check. Either alone stops it; both together survive a retitled heading.
19. THE COLUMN-INDEX ROW IS A DATA ROW TO A NAIVE PARSER. PBS prints "1 2 3 ... 24" beneath
    each band; it parses as item 1 with description "2 3". Require a description to contain
    a letter. This one is nastier than it looks: once ingested, it made the monotonic guard
    from #18 close the band on the FIRST real data row and silently drop the entire panel.
20. TUKEY FENCES DEGENERATE ON ZERO-INFLATED DISTRIBUTIONS. With 27 of 51 items unchanged,
    Q1 is exactly 0, the IQR collapses, and every mover is flagged — a flag that is always
    true carries no information. Fence over the SUBSET THAT MOVED and say so in the output.
21. PDF TEXT RUNS CAN BE PER-GLYPH. PBS emits one text run per character, so a naive read
    yields ["1","W","h","e","a","t"]. Rebuild rows by Y-banding then split cells at
    font-relative X GAPS. Column boundaries from header midpoints are NOT enough on their
    own: a left-aligned description under a centred header pulls its first letters into the
    previous column, so the label region needs gap tokenization while the right-aligned
    numeric region needs column assignment. Hybrid, not one or the other.
22. WHEN A SOURCE PHYSICALLY OVERLAPS TWO FIELDS, RECORD THE LOSS RATHER THAN GUESS.
    PBS descriptions long enough to overflow their column overlap the unit column in the
    file itself, making clean separation geometrically impossible. Result: 82.4% exact
    labels, 0 silent corruptions, and every non-exact label flagged. A flagged loss a
    caller can exclude beats a repaired guess they cannot detect.
23. RECORD WHICH URL A FETCH ACTUALLY USED. A release listing both a .pdf and an .xlsx in
    one role cannot be re-checked against upstream if coverage stores only (as_of, role):
    a join picks one arbitrarily and every run reports a spurious "re-pointed". Store the
    url on the coverage row and compare against the live URL SET.
24. A WEIGHTED MEAN OF LEVELS IS NOT AN INDEX. Emit an explicit `measure` field naming what
    the number is ("weighted_mean_price_pkr" vs "index_<date>_eq_100") rather than trusting
    a field name to carry the distinction.
25. CROSS-ENCODING VALIDATION IS THE STRONGEST TEST AVAILABLE. Where a source publishes the
    SAME release in two formats, parse both and require cell-for-cell agreement. It found
    every one of the defects above; no single-format test suite would have.
26. THE STORE FILE IS PRE-CREATED BY THE FRAMEWORK'S LEARN LOOP, so a missing-store check
    via os.Stat is almost always FALSE and is the wrong question. The real signal is whether
    the panel holds rows. Getting this wrong made a fresh install answer a legitimate query
    with "no item matches Onions" — plus a hint naming a command that did not exist. Adding
    an explicit EMPTY-panel state (distinct from missing-store and from unmatched-item) took
    the scorecard's live sample probe from 3/8 to 7/8. Treat missing / empty / unmatched as
    three states in any printed CLI that ships the learn loop.
