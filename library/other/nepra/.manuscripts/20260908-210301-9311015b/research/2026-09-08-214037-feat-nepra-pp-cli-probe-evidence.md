

########## PROBE: nepra_generation_schema_map ##########

# NEPRA plant-level generation HTML: actual schema, cross-year stability

Working files (absolute): `/private/tmp/claude-501/-Users-hamzaqazi-Desktop-Personal/5916c20a-9668-41e6-8ce7-5af354a69c76/scratchpad/nepra/`
(`raw_<FY>.htm` = as fetched, `utf8_<FY>.htm` = transcoded, `grid.py` = rowspan/colspan-expanding parser, plus `hdr.py`/`rows.py`/`diff.py`/`fmt.py`/`sent.py`/`validate.py`/`cat.py`)

## 1. Fetch + encoding (all real numbers)

All three via `curl -sS --max-time 30` with the browser UA, no retries needed:

| FY | HTTP | bytes | content-type | md5 | re-fetch md5 identical |
|---|---|---|---|---|---|
| 2017-18 | 200 | 426,282 | text/html | f6a63e4bc87b61cb2eabe989682b56dc | YES |
| 2020-21 | 200 | 455,640 | text/html | 722f48935ab931a32f352dc29d7863ca | YES |
| 2023-24 | 200 | 493,187 | text/html | 044cc6caaa71d0c17502892d6f275f75 | YES |

Byte counts match your already-verified figures exactly. Every file re-fetched byte-identical (reproduced before asserting).

**Encoding trap, characterised precisely.** `file` reports `HTML document text, ISO-8859 text`; the declared charset is `charset=windows-1252`. `iconv -f UTF-8 -t UTF-8` FAILS on all three → a strict UTF-8 read traps. The sole offending byte is **`0xA0`** (windows-1252 NBSP): 128 occurrences in FY17-18 and FY20-21, 132 in FY23-24. Nothing else is non-ASCII. Proof: after `iconv -f WINDOWS-1252 -t UTF-8`, byte delta is exactly +128/+128/+132 (each 0xA0 → 2-byte C2 A0), and the output validates as UTF-8.

Note two macOS gotchas I hit: BSD `grep` has no `-P` (my first non-ASCII count returned a bogus 0), and macOS `iconv` has no `-o` flag (use `>` redirect).

## 2. Structure

| FY | `<table>` | `<tr>` | `<td>` | `<th>` | colspan attrs | rowspan attrs |
|---|---|---|---|---|---|---|
| 2017-18 | 1 | 114 | 4,479 | **0** | 15 | 6 |
| 2020-21 | 1 | 114 | 4,479 | **0** | 15 | 6 |
| 2023-24 | 1 | 139 | 4,965 | **0** | 30 | 6 |

FY2023-24's 1 table / 139 `<tr>` / 4,965 `<td>` reproduces your prior figure. **Zero `<th>` in any year** — there are no semantic header cells; headers are ordinary `<td>`s identified only by position.

## 3. EXACT column headers, verbatim, in order

The header is a **3-row band** (not one row), built from six `rowspan=3` cells plus a hierarchical month/metric split. Physical rows: tr#0 = all-empty spacer, tr#1 = `colspan=32` title, tr#2/3/4 = header band, then data.

Title cell (verbatim, identical all three years):
`Details of Installed & Dependable Capacity (MW), Monthly Energy Generation (GWh) and Plant Utilization (%)`

The 32 real columns, verbatim (`top | mid | bottom` of the band):

```
col 00  S.No                      (rowspan=3)
col 01  Name of Companies         (rowspan=3)
col 02  Technology                (rowspan=3)
col 03  Fuel                      (rowspan=3)
col 04  Installed Capacity (MW)   (rowspan=3)
col 05  Dependable Capacity (MW)  (rowspan=3)
col 06  FY 2023-24 | Jul | % age
col 07  FY 2023-24 | Jul | GWh
col 08  FY 2023-24 | Aug | % age
col 09  FY 2023-24 | Aug | GWh
col 10  FY 2023-24 | Sep | % age
col 11  FY 2023-24 | Sep | GWh
col 12  FY 2023-24 | Oct | % age
col 13  FY 2023-24 | Oct | GWh
col 14  FY 2023-24 | Nov | % age
col 15  FY 2023-24 | Nov | GWh
col 16  FY 2023-24 | Dec | % age
col 17  FY 2023-24 | Dec | GWh
col 18  FY 2023-24 | Jan | % age
col 19  FY 2023-24 | Jan | GWh
col 20  FY 2023-24 | Feb | % age
col 21  FY 2023-24 | Feb | GWh
col 22  FY 2023-24 | Mar | % age
col 23  FY 2023-24 | Mar | GWh
col 24  FY 2023-24 | Apr | % age
col 25  FY 2023-24 | Apr | GWh
col 26  FY 2023-24 | May | % age
col 27  FY 2023-24 | May | GWh
col 28  FY 2023-24 | Jun | % age
col 29  FY 2023-24 | Jun | GWh
col 30  FY 2023-24 | Sum | % age
col 31  FY 2023-24 | Sum | GWh
```

Verbatim details that matter: the metric label is **`% age`** with a literal space (confirmed: `grep -c '>% age<'` = 13 in every year — 12 months + Sum), the total column is **`Sum`** (not "Total"), `S.No` (not "S. No"), and months run **fiscal order Jul→Jun**, not calendar order.

**`% age` precedes `GWh` in every month pair.** Confirmed against data: Tarbela Jul = col6 `'94.94'` (percent), col7 `'2,456.58'` (GWh). A parser assuming value-then-percent silently swaps all 26 monthly fields.

## 4. Row counts — real, and reconciled

| FY | total `<tr>` | data | header | blank | title/spacer | distinct names | S.No contiguous |
|---|---|---|---|---|---|---|---|
| 2017-18 | 114 | **108** | 3 | 2 | 1 | **108** | 1..108, no gaps/dupes |
| 2020-21 | 114 | **108** | 3 | 2 | 1 | **108** | 1..108, no gaps/dupes |
| 2023-24 | 139 | **133** | 3 | 2 | 1 | **133** | 1..133, no gaps/dupes |

Reconciles exactly: 114 − 3 − 2 − 1 = 108; 139 − 3 − 2 − 1 = 133. The 2 blank rows are tr#0 (leading spacer) and the final tr (trailing). **There are no subtotal/grand-total rows** — no "Total" row exists; every numbered row is a plant.

## 5. Distinct plants/companies

108 / 108 / 133 — **`Name of Companies` is unique within every year** (0 duplicates in all three). It is a plant-level row, not company-level: e.g. FY2017-18 carries three separate `Tricon Boston Consulting Corporation (Pr…)` rows (distinct suffixes), and `JDW Sugar Mills Limited. (Unit-II) Rahim Yar Khan` names a unit.

Cross-year name overlap (exact string match): FY17-18∩FY20-21 = 107 (1 in/1 out); FY20-21∩FY23-24 = 106 (2 dropped, 27 added); FY17-18∩FY23-24 = 107.

## 6. Fuel / Technology categories, verbatim

`Technology` — 9 distinct, **identical set in all three years**:
`'THERMAL'`, `'THERMAL- COAL'`, `'Coal'`, `'HYDEL-(WAPDA)'`, `'HYDEL-(IPP)'`, `'NUCLEAR'`, `'WIND'`, `'SOLAR'`, `'BIOGAS'`

FY2023-24 counts: THERMAL 46, WIND 36, HYDEL-(WAPDA) 12, BIOGAS 9, THERMAL- COAL 8, SOLAR 8, HYDEL-(IPP) 7, NUCLEAR 6, Coal 1.

`Fuel` — 16 distinct, **identical set in all three years**:
`'WIND'`, `'HYDEL'`, `'RFO'`, `'COAL'`, `'BAGASSE'`, `'SOLAR'`, `'GAS'`, `'NUCLEAR'`, `'RLNG'`, `'RLNG/HSD'`, `'GAS/HSD'`, `'RFO/RLNG/HSD'`, `'BAGASSE/COAL'`, `'Natural Gas'`, `'Natural Gas/ Furnace Oil'`, `'Lignite Coal'`

Dirty-vocabulary traps inside these verbatim values: mixed case (`'COAL'` vs `'Coal'` vs `'Lignite Coal'`; `'GAS'` vs `'Natural Gas'`), a stray space after the hyphen in `'THERMAL- COAL'`, a stray space inside `'Natural Gas/ Furnace Oil'`, and multi-fuel values encoded as `/`-joined compounds. `Technology='Coal'` (n=1, Lakhra) is a category-of-one that duplicates `'THERMAL- COAL'` semantically.

## 7. Three verbatim data rows per year

**FY2017-18, S.No 1** — `Name='AES Lalpir power limited.'` `Technology='THERMAL'` `Fuel='RFO'` `Installed='362'` `Dependable='350'`
Jul %age=`'56.07'` GWh=`'151.01'` · Aug `'55.48'`/`'149.43'` · Sep `'43.08'`/`'112.29'` · Oct `'0.00'`/`'0.01'` · Nov `'7.61'`/`'19.83'` · Dec `'48.92'`/`'131.76'` · Jan `'34.27'`/`'92.30'` · Feb `'8.80'`/`'21.40'` · Mar `'27.11'`/`'73.01'` · Apr `'51.54'`/`'129.89'` · May `'56.23'`/`'146.43'` · Jun `'24.12'`/`'60.77'` · **Sum `'35.49'`/`'1,088.12'`**

**FY2017-18, S.No 42** — `Name='Karachi Nuclear Power Plant-II (K-2)'` `Technology='NUCLEAR'` `Fuel='NUCLEAR'` `Installed=''` `Dependable=''`
All 26 monthly cells `''` (NBSP). Note capacity is blank too.

**FY2020-21, S.No 84** — `Name='Tenaga Generasi Ltd.'` `Technology='WIND'` `Fuel='WIND'` `Installed='Export to K.Electric'` `Dependable=''`
All 26 monthly cells `''`. **A prose string sits in the `Installed Capacity (MW)` numeric column.**

**FY2020-21, S.No 38** — `Name='Chashma Nuclear Power Plant 1 (CHASHNUPP-I)'` `Technology='NUCLEAR'` `Fuel='NUCLEAR'` `Installed='325'` `Dependable='301'`
Jul `'99.55'`/`'222.93'` · Aug `'91.74'`/`'205.45'` · Sep `'98.58'`/`'213.63'` · Oct **`'101.00'`**/`'226.18'` · Nov `'100.47'`/`'217.73'` · Dec `'100.83'`/`'225.80'` · Jan `'91.27'`/`'204.39'` · Feb `'101.00'`/`'204.29'` · Mar `'100.97'`/`'226.11'` · Apr `'49.59'`/`'107.48'` · May **`'0.00'`/`'0.00'`** · Jun `'96.96'`/`'210.13'` · Sum `'85.87'`/`'2,264.12'`

**FY2023-24, S.No 1** — `Name='Tarbela Hydropower Project (WAPDA)'` `Technology='HYDEL-(WAPDA)'` `Fuel='HYDEL'` `Installed='3,478'` `Dependable='3,478'`
Jul `'94.94'`/`'2,456.58'` · Aug `'98.06'`/`'2,537.49'` · Sep `'75.86'`/`'1,899.70'` · Oct `'33.13'`/`'857.20'` · Nov `'29.59'`/`'740.93'` · Dec `'19.91'`/`'515.21'` · Jan `'14.87'`/`'384.89'` · Feb `'24.15'`/`'584.65'` · Mar `'19.27'`/`'498.64'` · Apr `'8.93'`/`'223.74'` · May `'39.64'`/`'1,025.76'` · Jun `'65.54'`/`'1,641.15'` · **Sum `'43.87'`/`'13,365.93'`**

**FY2023-24, S.No 14** — `Name='Kotri Power Station'` `Technology='THERMAL'` `Fuel='Natural Gas/ Furnace Oil'` `Installed='174'` `Dependable='120'`
All 26 monthly cells = `'DELICENSED'` (one `<td colspan=26>`).

**FY2023-24, S.No 39** — `Name='Habibullah Coastal Power Company (PVT) Limited. (HCPC)'` `Technology='THERMAL'` `Fuel='GAS'` `Installed='140'` `Dependable='129'`
All 26 monthly cells = `'DECOMMISSIONED'`.

## 8. THE CRITICAL QUESTION: is the schema stable? — YES, logically stable

Strict column-by-column diff of the 3-row header band across all three years, after stripping trailing all-empty padding columns and masking only the self-referential FY label:

```
FY2017-18: physical width=40, trailing all-empty cols=8  -> 32 real columns
FY2020-21: physical width=40, trailing all-empty cols=8  -> 32 real columns
FY2023-24: physical width=39, trailing all-empty cols=7  -> 32 real columns

STRICT diff (FY label masked): -> NO DIFFERENCES in cols 0..31
```

**Columns added: none. Removed: none. Renamed: none. Re-ordered: none.** All three years are exactly 32 columns with byte-identical header text in identical order. The only varying header cell is the legitimately data-bearing band label: `'FY 2017-18'` / `'FY 2020-21'` / `'FY 2023-24'`.

The 40-vs-39 physical width is **pure trailing empty padding** (Excel export artifact), not a schema change. It never shifts a real column — but it does mean you must not hard-code physical width or index from the right-hand end.

### What DOES drift (all row-level and representational, not column-level)

**(a) Row order was re-sorted between FY2020-21 and FY2023-24 — S.No is NOT a stable key.**
- FY17-18 → FY20-21: of 107 common names, **107 keep the same S.No** (0 changed).
- FY20-21 → FY23-24: of 106 common names, **0 keep the same S.No** (all 106 changed).
- FY17-18/FY20-21 order starts `AES Lalpir power limited.` (THERMAL first). FY23-24 is regrouped by technology, starting `Tarbela Hydropower Project (WAPDA)` (HYDEL first) and ending with WIND.
- Examples: `AES Lalpir power limited.` S.No 1→36; `Allai Khwar Hydropower Project (WAPDA)` 56→8; `(NPPCL) - Haveli Bahadar Shah` 29→63.
- No year is alphabetically sorted.

**(b) "Not reported" changed HTML representation.** FY17-18/FY20-21 encode a fully-unreported year as 26 individual `&nbsp;` cells (full-width rows, 40 raw `<td>`). FY23-24 encodes it as a single **empty `<td colspan=26>`** (14 raw `<td>`) — structurally the same construct as DELICENSED, just unlabelled.

**(c) New sentinel vocabulary appears in FY23-24 only:** `DELICENSED` (12 rows) and `DECOMMISSIONED` (1 row). Absent in FY17-18/FY20-21. FY20-21 has its own one-off: `Export to K.Electric` (3 rows).

**(d) Linked-PDF path convention drifts hard** (see §12).

## 9. Number formatting

- **Thousands separators: YES, comma.** Present in 61 / 59 / 68 cells in cols 4..31 (e.g. `'3,478'`, `'2,456.58'`, `'13,365.93'`, `'1,601'`). Any `float()` must strip commas first.
- **Parentheses for negatives: NO.** Zero cells contain `(` or `)` in the numeric area, all three years.
- **Minus signs: NO.** Zero cells contain `-` in cols 4..31. There are no negative values.
- **Units embedded in cells: NO.** Zero `%` characters and no `MW`/`GWh` text in data cells — units live only in the header (`(MW)`, `GWh`, `% age`).
- Monthly/Sum values are 2-decimal (`'56.07'`); capacity columns are bare integers (`'362'`, `'3,478'`).
- **`% age` legitimately exceeds 100:** 8 cells in FY17-18 (max **112.2**), 11 in FY20-21 (max 101.0), 16 in FY23-24 (max **121.09**). A `0 ≤ pct ≤ 100` validator would wrongly reject real published data.
- **The `Sum` column is not always arithmetically consistent.** External check (Sum GWh vs sum of the 12 monthly GWh, 2% tolerance): FY17-18 97/97 match, FY23-24 118/118 match, FY20-21 104/105 — one genuine upstream error at `(NPPCL) - Balloki`: twelve clean months summing to **5945.21** against a stated Sum of **`'5,905.65'`** (Δ 39.56 GWh). Verified against raw HTML; not a parse artifact. Store `Sum` as reported; never recompute silently over it, and never assume it reconciles.

## 10. Merged / spanning header cells — YES, and they break naive parsing

Confirmed spanning cells in FY2023-24 (identical pattern in the other years):
- tr#1: `<td colspan=32>` title.
- tr#2: six `rowspan=3` cells (`S.No`…`Dependable Capacity (MW)`) + `<td colspan=26>FY 2023-24`.
- tr#3: thirteen `colspan=2` cells (`Jul`…`Jun`, `Sum`).
- tr#4: 33 plain cells (the `% age`/`GWh` leaves).
- 13 data rows: `<td colspan=26>DELICENSED|DECOMMISSIONED`.
- 2 data rows: **empty** `<td colspan=26>`.

Quantified naive-parser damage — raw `<td>`-per-`<tr>` histogram vs expanded:
```
FY2017-18 raw: {9:1, 15:1, 21:1, 34:1, 40:110}  -> expanded uniformly to 40
FY2020-21 raw: {9:1, 15:1, 21:1, 34:1, 40:110}  -> expanded uniformly to 40
FY2023-24 raw: {8:1, 14:16, 20:1, 33:1, 39:120} -> expanded uniformly to 39
```
FY2023-24 has **16 rows with only 14 raw `<td>`** (1 header + 13 sentinel + 2 empty-merged). A positional parser that ignores `colspan` reads `DELICENSED` as Jul `% age` and then misaligns the seven trailing padding cells into Jul GWh…Oct `% age`. Because `<th>` count is 0 and there is no header row to anchor on, this fails silently.

Two more parse traps found in the raw bytes:
- **Cell text wraps across source lines**: `<td …>Details of\n  Installed &amp; …`, `>Natural Gas/\n  Furnace Oil<`, `>Name of\n  Companies<`, `>Export\n  to K.Electric<`. Any line-oriented `grep`/regex extraction splits these. Whitespace must be collapsed after tag stripping.
- **Values nested two levels deep**: many `% age` cells are `<td><a href="…pdf"><span style='font-size:16.0pt'>69.07</span></a></td>`. A parser reading only a `<td>`'s direct text node returns empty for these. Must take all descendant text.

## 11. Footnote markers / asterisks — NONE, but there ARE meaning-changing sentinels

- **Zero `*` characters** anywhere in the data area, the `Name of Companies` column, or `Technology`/`Fuel`, in all three years. No footnote superscripts, no daggers, no footnote block.
- **But four in-band sentinel strings change cell meaning entirely**, occupying numeric columns:
  - `DELICENSED` — 312 expanded cells / 12 rows (FY23-24 only)
  - `DECOMMISSIONED` — 26 expanded cells / 1 row (FY23-24 only)
  - `Export to K.Electric` — 3 cells, in the **`Installed Capacity (MW)`** column (FY20-21 only)
  - empty `colspan=26` — 2 rows (FY23-24)
- `<u style='visibility:hidden;mso-ignore:visibility'>&nbsp;</u>` wrappers appear 117× (FY17-18), 26× (FY20-21), 0× (FY23-24). I opened them: they contain only `&nbsp;` — **no hidden data**, pure Excel formatting. But they mean "blank" has two distinct HTML encodings within the same file.

## 12. Blank vs zero — DEMONSTRABLY DIFFERENT. Do not conflate.

**The file distinguishes them, and the distinction is clean and machine-detectable.**

The blank pattern is strictly **all-or-nothing per row**. Every row that has any blank in the monthly block has **exactly 26 blanks and exactly 0 `'0.00'` cells**. No row anywhere in any of the three years mixes blanks with zeros in the monthly block. Meanwhile `'0.00'` appears **437 / 540 / 471** times, always in rows that are otherwise fully populated.

Rows with a fully blank monthly block: 11 (FY17-18), 3 (FY20-21), 2 (FY23-24).

Longitudinal proof that blank ≠ zero — same plant, tracked across years:

| Plant | FY2017-18 | FY2020-21 | FY2023-24 |
|---|---|---|---|
| `Karachi Nuclear Power Plant-II (K-2)` | 26 blanks, 0 zeros, capacity blank | 0 blanks, **16 explicit `0.00`**, Sum GWh `'1,705.91'` | 0 blanks, 4 zeros, Sum `'5,896.71'` |
| `Tarbela Ext. 04 Hydropower Project (WAPDA)` | 26 blanks, 0 zeros | 0 blanks, 12 zeros, Sum `'3,427.64'` | 0 blanks, 6 zeros, Sum `'4,653.15'` |
| `Golen Gol Hydropower Project (WAPDA)` | 26 blanks, 0 zeros | 0 blanks, **0 zeros**, Sum `'84.89'` | 0 blanks, 0 zeros, Sum `'177.30'` |

K-2 is the decisive case: in FY2017-18 it is blank across the board *including its capacity columns* (plant licensed but not yet built — it reached criticality in 2021). In FY2020-21 the same plant reports **16 explicit `0.00` month-cells alongside a non-zero annual Sum of 1,705.91 GWh** — i.e. `0.00` is a genuine measurement of "ran zero this month", coexisting in the same row with real output. Blank is the absence of a measurement; `0.00` is a measured zero. Conflating them would, in FY2017-18 alone, invent 11 plants × 12 months of false zero generation.

Five distinguishable states, all of which the store must keep separate:

1. **numeric value** (incl. a real `'0.00'`) → reported measurement
2. **NBSP blank** (`''` after normalisation) → not reported / not yet operational. Two HTML encodings: bare `&nbsp;`, or `<u style='visibility:hidden'>&nbsp;</u>`. In FY23-24, an empty `<td colspan=26>`.
3. **`DELICENSED`** → licence withdrawn; capacity still stated, generation not applicable
4. **`DECOMMISSIONED`** → plant retired; capacity still stated, generation not applicable
5. **`Export to K.Electric`** → generation exists but is reported elsewhere (out of this table's scope) — the strongest possible case that blank ≠ zero, since here blank explicitly means "reported in another dataset"

Note states 3–5 retain valid `Installed`/`Dependable` capacity (`Kotri`: `'174'`/`'120'`) while the entire generation year is non-numeric. And `'0'` appears as a legitimate integer in capacity columns (`Shahdara`: Installed `'0'`, Dependable `'0'`; FY20-21 Dependable has 11 such) — distinct again from blank capacity (11 blanks in FY17-18).

## 13. "SIR Data 2024.htm" and "SIR Data 2025.htm" — NOT the missing FY2024-25 data

Correct path is under `Detail of Generation/` (both 404 at the `State of Industry Reports/` root).

```
.../Detail%20of%20Generation/SIR%20Data%202024.htm   200   374 bytes   md5 37c27f4decfbad45c80078a997f13c19
.../Detail%20of%20Generation/SIR%20Data%202025.htm   200   374 bytes   md5 37c27f4decfbad45c80078a997f13c19
```

**They are BYTE-IDENTICAL — same md5, same 374 bytes.** Neither contains any data. Both are HTML framesets. Full verbatim content of *both*:

```html
<html>
<head>
<title>SIR Data 2024</title>
</head>
<frameset rows="164,*">
	<frame name="header" scrolling="no" noresize target="main" src="home24.htm">
	<frame name="main" src="List%20of%20Companies%20Genenration%20wise%202023-24.htm">
	<noframes>
	<body>
	<p>This page uses frames, but your browser doesn't support them.</p>
	</body>
	</noframes>
</frameset>
</html>
```

So `SIR Data 2025.htm` is titled **"SIR Data 2024"** and points its main frame at **FY2023-24**. It is a stale copy-paste of the 2024 stub, not a new fiscal year.

I followed the frame chain to confirm:
- `List of Companies Genenration wise 2023-24.htm` (200, 9,838 B) — an Excel workbook frameset declaring `c_rgszSh[0] = "2023-24"` (one sheet) and linking `…_files/sheet001.htm`, i.e. exactly the FY2023-24 file already analysed above.
- `home24.htm` (200, 669 B) — header banner reading verbatim: **"Data pertaining to State of Industry Report 2023-24"**.

Corroborating negatives: `home25.htm` → **404** (nobody created a 2025 header). And `List of Companies Genenration wise 2024-25_files/sheet001.htm`, `…Generation wise 2024-25_files/sheet001.htm` (typo-corrected spelling), `List of Companies Genenration wise 2024-25.htm` → **all 404**. Control: `…2022-23_files/sheet001.htm` → 200, 490,461 B (probe method works).

**Conclusion: FY2024-25 plant-level generation data is not published at this path under any name I could find. `SIR Data 2025.htm` is a 200-OK decoy serving FY2023-24.** Enumerating by filename would silently double-count FY2023-24 as FY2024-25.

Main.htm (200, 12,210 B) href list confirms your finding that the index is incomplete — it links only:
`List of Companies Genenration wise 2017-18.htm` … `2021-22.htm` (five years), plus `FCA (2018-2022).htm`, `Hydel 1/Hydel Data.htm`, `Notification/SRO Data Webpage.htm`, `Quarterly Data (XWD & KE).htm`, `Main_files/filelist.xml`. It does **not** link FY2022-23, FY2023-24, or either SIR Data stub — all of which are nonetheless reachable.

## 14. Bonus discovery: a large per-plant-per-month PDF corpus hangs off the `% age` cells

Many `% age` cells are hyperlinks to individual monthly generation PDFs: **1,261 (FY17-18) / 1,365 (FY20-21) / 1,526 (FY23-24)** `.pdf` hrefs. Spot-checked two — both real:
- `…/Detail of Generation/WAPDA/Tarbela/Tarbela Jul 2023.pdf` → 200, 426,589 B, `application/pdf`
- `…/Detail of Generation/2017-2018/Thermal/Lal pir/Jul.pdf` → 200, 408,543 B, `application/pdf`

Path convention drifts across all three years — there is no single template:
- FY2017-18: `../2017-2018/Thermal/Lal%20pir/Jul.pdf` (year directory first; bare month filename)
- FY2020-21: `../Thermal/Lal%20Pir%202020-21/Lal%20Pir%20Jul%202020.pdf` (no year dir; year inside plant dir; verbose filename)
- FY2023-24: `../WAPDA/Tarbela/Tarbela%20Jul%202023.pdf` (**no year component in the directory at all**; year only in the filename)

Also note the top-level category directory renames: `Wapda` (FY20-21) → `WAPDA` (FY23-24). FY20-21 top dirs: Thermal 477, Wind 271, Wapda 156, Bagasse 104, Hydel 78, Nuclear 64, Solar 52, NPGCL 52, CPGCL 52, JPCL 39. This is the per-month provenance layer if you ever need to audit a monthly figure.

### VERIFIED
- ENCODING: all three files are windows-1252 (`file` says `HTML document text, ISO-8859 text`; meta says `charset=windows-1252`). `iconv -f UTF-8 -t UTF-8` FAILS on all three, proving a strict UTF-8 read traps. The ONLY non-ASCII byte is 0xA0 (NBSP): 128 occurrences in FY17-18 and FY20-21, 132 in FY23-24. Proof of exclusivity: post-transcode byte delta is exactly +128/+128/+132, matching the 0xA0 count 1:1 (each 0xA0 -> 2-byte C2 A0), and output validates as UTF-8.
- FETCH: FY2023-24 HTTP 200 / 493,187 B; FY2020-21 200 / 455,640 B; FY2017-18 200 / 426,282 B, all `text/html`. Byte counts match the pre-verified figures exactly. All three re-fetched BYTE-IDENTICAL (md5 f6a63e4bc87b61cb2eabe989682b56dc / 722f48935ab931a32f352dc29d7863ca / 044cc6caaa71d0c17502892d6f275f75 unchanged on second fetch).
- STRUCTURE: FY2023-24 = 1 <table>, 139 <tr>, 4,965 <td> (reproduces the prior figure). FY2017-18 and FY2020-21 both = 1 <table>, 114 <tr>, 4,479 <td>. **ZERO <th> elements in all three years** - there are no semantic header cells; headers are positional <td> only.
- SCHEMA IS STABLE - the headline answer. After stripping trailing all-empty padding columns and masking only the self-referential FY band label, a strict column-by-column diff of the 3-row header band across all three years reports `NO DIFFERENCES in cols 0..31`. All three reduce to exactly 32 real columns, byte-identical header text, identical order. Columns added: none. Removed: none. Renamed: none. Re-ordered: none.
- The 40-vs-39 physical width difference (FY17-18/FY20-21 = 40 cols, 8 trailing empty; FY23-24 = 39 cols, 7 trailing empty) is PURE TRAILING EMPTY PADDING, an Excel export artifact. It never shifts a real column. Verified: after stripping trailing all-empty columns, all three = 32 columns.
- EXACT HEADERS, verbatim in order (32 cols): `S.No`, `Name of Companies`, `Technology`, `Fuel`, `Installed Capacity (MW)`, `Dependable Capacity (MW)`, then 13 month/metric pairs under band label `FY <year>`: Jul, Aug, Sep, Oct, Nov, Dec, Jan, Feb, Mar, Apr, May, Jun, Sum - each splitting into `% age` then `GWh`.
- `% age` is the literal verbatim string WITH A SPACE - confirmed by `grep -c '>% age<'` returning exactly 13 in every year (12 months + Sum). Total column is literally `Sum`, not 'Total'. Months are in FISCAL order Jul->Jun, not calendar order.
- COLUMN ORDER WITHIN EACH MONTH IS `% age` FIRST, THEN `GWh`. Confirmed against data: Tarbela FY23-24 col6=`'94.94'` (a percentage), col7=`'2,456.58'` (GWh). A parser assuming value-then-percent swaps all 26 monthly fields.
- HEADER IS A 3-ROW BAND, not one row: tr#0 all-empty spacer, tr#1 `<td colspan=32>` title, tr#2/3/4 the header band, then data. Verified spanning cells in tr#2: six `rowspan=3` cells + `<td colspan=26>FY 2023-24`; tr#3: thirteen `colspan=2` month cells; tr#4: 33 plain leaf cells.
- ROW COUNTS, reconciled exactly. FY2017-18: 114 <tr> = 108 data + 3 header + 2 blank + 1 title. FY2020-21: 114 = 108+3+2+1. FY2023-24: 139 = 133+3+2+1.
- THERE ARE NO SUBTOTAL OR GRAND-TOTAL ROWS in any year. Every numbered row is a plant. The 2 blank rows per file are the leading spacer tr#0 and the trailing tr.
- DISTINCT PLANTS: 108 / 108 / 133. `Name of Companies` is UNIQUE within every year - zero duplicate names in all three files. S.No is contiguous 1..N with no gaps and no duplicates in all three.
- TECHNOLOGY vocabulary is 9 values, IDENTICAL SET in all three years: 'THERMAL', 'THERMAL- COAL', 'Coal', 'HYDEL-(WAPDA)', 'HYDEL-(IPP)', 'NUCLEAR', 'WIND', 'SOLAR', 'BIOGAS'. Zero added, zero removed across FY17-18/FY20-21/FY23-24.
- FUEL vocabulary is 16 values, IDENTICAL SET in all three years: 'WIND', 'HYDEL', 'RFO', 'COAL', 'BAGASSE', 'SOLAR', 'GAS', 'NUCLEAR', 'RLNG', 'RLNG/HSD', 'GAS/HSD', 'RFO/RLNG/HSD', 'BAGASSE/COAL', 'Natural Gas', 'Natural Gas/ Furnace Oil', 'Lignite Coal'.
- BLANK vs ZERO ARE DEMONSTRABLY DIFFERENT AND MUST NOT BE CONFLATED. The blank pattern is strictly all-or-nothing per row: every row with any blank in the monthly block has EXACTLY 26 blanks and EXACTLY 0 '0.00' cells. No row in any of the three years mixes blanks with zeros in the monthly block. Meanwhile '0.00' occurs 437/540/471 times, always in otherwise fully-populated rows.
- LONGITUDINAL PROOF that blank != zero: `Karachi Nuclear Power Plant-II (K-2)` has 26 blanks / 0 zeros / blank capacity in FY2017-18 (licensed but not yet built), then in FY2020-21 the SAME plant reports 16 explicit '0.00' month-cells ALONGSIDE a non-zero annual Sum of '1,705.91' GWh. So '0.00' is a genuine measured zero coexisting with real output in one row, while blank is the absence of measurement. Same pattern for `Tarbela Ext. 04 Hydropower Project (WAPDA)` (26 blanks FY17-18 -> Sum '3,427.64' FY20-21) and `Golen Gol Hydropower Project (WAPDA)`.
- Conflating blank with zero would, in FY2017-18 alone, invent 11 plants x 12 months of false zero generation.
- FIVE distinguishable cell states exist: (1) numeric incl. real '0.00'; (2) NBSP blank = not reported; (3) 'DELICENSED'; (4) 'DECOMMISSIONED'; (5) 'Export to K.Electric'. States 3-5 retain valid capacity values while the whole generation year is non-numeric (e.g. Kotri: Installed '174', Dependable '120', all 26 monthly cells 'DELICENSED').
- SENTINEL STRINGS occupy numeric columns: 'DELICENSED' (312 expanded cells / 12 rows, FY23-24 only), 'DECOMMISSIONED' (26 cells / 1 row, FY23-24 only), 'Export to K.Electric' (3 cells, FY20-21 only, sitting in the `Installed Capacity (MW)` column). Verified in raw HTML.
- THOUSANDS SEPARATORS: YES, commas - 61/59/68 cells in cols 4..31 (e.g. '3,478', '2,456.58', '13,365.93', '1,601'). float() must strip commas.
- NO parentheses for negatives (zero '(' in the numeric area, all years), NO minus signs (zero '-' in cols 4..31), NO units embedded in data cells (zero '%' characters, no MW/GWh text). Units live only in the header.
- NO ASTERISKS OR FOOTNOTE MARKERS ANYWHERE: zero '*' characters in the data area, in `Name of Companies`, or in Technology/Fuel, in all three years. No footnote block exists.
- MERGED CELLS BREAK NAIVE PARSING, quantified. Raw <td>-per-<tr> histograms: FY2017-18 {9:1,15:1,21:1,34:1,40:110}; FY2020-21 identical; FY2023-24 {8:1,14:16,20:1,33:1,39:120}. FY2023-24 has 16 rows with only 14 raw <td> (1 header + 13 sentinel + 2 empty-merged). After rowspan/colspan expansion every row is uniformly 40/40/39 cols.
- 'DELICENSED' is a SINGLE `<td colspan=26>` cell spanning the entire monthly block - verified in raw HTML at the Kotri row: `<td colspan=26 class=xl102 ...>DELICENSED</td>`.
- The 2 blank rows in FY2023-24 (Reshma Power Generation, Gulf Powergen) use an EMPTY `<td colspan=26>` - structurally the same merged-annotation construct as DELICENSED, just unlabelled. This is a representational drift: FY17-18/FY20-21 encode the same 'not reported' state as 26 individual `&nbsp;` cells at full row width.
- S.No IS NOT A STABLE CROSS-YEAR KEY. FY17-18->FY20-21: of 107 common names, 107 keep the same S.No (0 changed). FY20-21->FY23-24: of 106 common names, ZERO keep the same S.No (all 106 changed). The file was re-sorted by technology - FY17-18/FY20-21 start with 'AES Lalpir power limited.' (THERMAL), FY23-24 starts with 'Tarbela Hydropower Project (WAPDA)' (HYDEL) and ends with WIND. Examples: AES Lalpir 1->36, Allai Khwar 56->8, (NPPCL) - Haveli Bahadar Shah 29->63.
- No year is alphabetically sorted (verified case-insensitive sort check fails for all three).
- '% age' LEGITIMATELY EXCEEDS 100: 8 cells in FY17-18 (max 112.2), 11 in FY20-21 (max 101.0), 16 in FY23-24 (max 121.09). E.g. Chashma Nuclear FY20-21 Oct = '101.00'. A 0<=pct<=100 validator would reject real published data.
- THE `Sum` COLUMN IS NOT ALWAYS ARITHMETICALLY CONSISTENT. External validation (Sum GWh vs sum of 12 monthly GWh): FY17-18 97/97 match, FY23-24 118/118 match, FY20-21 104/105 with ONE genuine upstream error at `(NPPCL) - Balloki`: twelve clean monthly values (619.24, 694.03, 595.60, 763.73, 591.91, 413.01, 657.74, 0.00, 420.82, 401.66, 403.53, 383.94) summing to 5945.21 against a stated Sum of '5,905.65' - a 39.56 GWh discrepancy. Verified against raw HTML; not a parse artifact.
- MY COLUMN ASSIGNMENT IS VALIDATED FROM OUTSIDE THE PARSER: the Sum=sum(12 months) identity holds for 97/97, 104/105 and 118/118 numeric rows, while the same test applied to the %age columns (the off-by-one hypothesis) matches only 7/97, 12/105 and 3/118. The skipped counts (11, 3, 15) exactly equal my independently-derived non-numeric row counts (11 blank; 3 Export-to-KE; 13 sentinel + 2 blank).
- SIR Data 2024.htm and SIR Data 2025.htm ARE BYTE-IDENTICAL: both HTTP 200, both exactly 374 bytes, both md5 37c27f4decfbad45c80078a997f13c19. Correct path is under `Detail of Generation/` (both 404 at the `State of Industry Reports/` root).
- NEITHER SIR FILE CONTAINS ANY DATA. Both are 374-byte HTML framesets. Both are titled `<title>SIR Data 2024</title>` and both set `<frame name="main" src="List%20of%20Companies%20Genenration%20wise%202023-24.htm">`. So 'SIR Data 2025.htm' serves FY2023-24 - it is a stale copy-paste of the 2024 stub, NOT the missing FY2024-25 data.
- Frame chain followed and confirmed: `List of Companies Genenration wise 2023-24.htm` (200, 9,838 B) is an Excel workbook frameset declaring `c_rgszSh[0] = "2023-24"` (one sheet) linking `…_files/sheet001.htm`. `home24.htm` (200, 669 B) reads verbatim: 'Data pertaining to State of Industry Report 2023-24'.
- FY2024-25 IS NOT PUBLISHED at this path under any name tested: `List of Companies Genenration wise 2024-25_files/sheet001.htm` 404, typo-corrected `…Generation wise 2024-25_files/sheet001.htm` 404, `List of Companies Genenration wise 2024-25.htm` 404, and `home25.htm` 404 (nobody created a 2025 header). Control probe `…2022-23_files/sheet001.htm` returned 200 / 490,461 B, proving the probe method works.
- Main.htm (200, 12,210 B) confirms the index is INCOMPLETE: it links only FY2017-18..FY2021-22 plus `FCA (2018-2022).htm`, `Hydel 1/Hydel Data.htm`, `Notification/SRO Data Webpage.htm`, `Quarterly Data (XWD & KE).htm`, `Main_files/filelist.xml`. It does NOT link FY2022-23, FY2023-24, or either SIR Data stub - all of which are reachable.
- CELL TEXT WRAPS ACROSS SOURCE LINES, breaking line-based extraction: verified instances `>Name of\n  Companies<`, `>Natural Gas/\n  Furnace Oil<`, `>Export\n  to K.Electric<`, and the title `Details of\n  Installed &amp; …`.
- VALUES ARE NESTED TWO LEVELS DEEP in many cells: `<td><a href="…pdf"><span style='font-size:16.0pt'>69.07</span></a></td>`. A parser reading only a <td>'s direct text node returns empty for these; descendant text is required.
- `<u style='visibility:hidden;mso-ignore:visibility'>&nbsp;</u>` wrappers appear 117x (FY17-18), 26x (FY20-21), 0x (FY23-24). I opened them: they contain ONLY `&nbsp;` - no hidden data. But they mean 'blank' has two distinct HTML encodings in the same file.
- A LARGE PER-PLANT-PER-MONTH PDF CORPUS hangs off the `% age` cells: 1,261 (FY17-18) / 1,365 (FY20-21) / 1,526 (FY23-24) `.pdf` hrefs. Spot-checked two, both real: `WAPDA/Tarbela/Tarbela Jul 2023.pdf` -> 200, 426,589 B, application/pdf; `2017-2018/Thermal/Lal pir/Jul.pdf` -> 200, 408,543 B, application/pdf.
- PDF PATH CONVENTION DRIFTS across years with no single template: FY2017-18 `../2017-2018/Thermal/Lal%20pir/Jul.pdf` (year dir first, bare month filename); FY2020-21 `../Thermal/Lal%20Pir%202020-21/Lal%20Pir%20Jul%202020.pdf` (no year dir, year in plant dir); FY2023-24 `../WAPDA/Tarbela/Tarbela%20Jul%202023.pdf` (NO year component in the directory at all). Category dir also renamed `Wapda` (FY20-21) -> `WAPDA` (FY23-24).
- '0' IS A LEGITIMATE CAPACITY VALUE, distinct from blank capacity: Shahdara FY23-24 has Installed '0' and Dependable '0'; FY20-21 Dependable has 11 '0' cells; FY17-18 has 11 BLANK Installed and 11 blank Dependable (exactly the 11 fully-blank rows).
- DIRTY VOCABULARY within the stable category sets: mixed case ('COAL' vs 'Coal' vs 'Lignite Coal'; 'GAS' vs 'Natural Gas'), stray space after hyphen in 'THERMAL- COAL', stray space inside 'Natural Gas/ Furnace Oil', and '/'-joined multi-fuel compounds. `Technology='Coal'` is a category-of-one (Lakhra) semantically duplicating 'THERMAL- COAL'.
- Rows are plant-level, not company-level: FY2017-18 carries three separate `Tricon Boston Consulting Corporation (Pr…)` rows, and `JDW Sugar Mills Limited. (Unit-II) Rahim Yar Khan` names a specific unit.
- CROSS-YEAR NAME OVERLAP (exact match): FY17-18 n FY20-21 = 107 (1 in / 1 out); FY20-21 n FY23-24 = 106 (2 dropped, 27 added); FY17-18 n FY23-24 = 107.
- TOOLING GOTCHAS on macOS, both of which produced false results before I caught them: BSD `grep` has no `-P` (my first non-ASCII byte count wrongly returned 0), and macOS `iconv` has no `-o` flag (must use `>` redirect).
- FY2017-18 and FY2020-21 coincidentally share identical row counts (108) and identical Technology/Fuel distributions, but are genuinely DIFFERENT content - adversarially verified: name lists differ, metadata cols 1-5 differ row-for-row, and 2,548 of 2,808 numeric cells differ.

### REFUTED/UNVERIFIED
- REFUTED (my own first measurement): the initial non-ASCII byte count of 0 for all three files was WRONG - macOS BSD `grep` silently does not support `-P`. Corrected via `tr -d '\000-\177' | od`, which found 128/128/132 occurrences of 0xA0. I nearly recorded 'no non-ASCII bytes', which would have contradicted the known encoding trap.
- REFUTED: the hypothesis that a schema drift exists in the COLUMN headers. It does not. All three years are exactly 32 columns with byte-identical header text in identical order. The 40-vs-39 physical width is trailing empty padding only. A naive positional parser is safe on column identity - but is broken by colspan/rowspan (see traps), which is a different failure mode than the one anticipated.
- REFUTED: that 'SIR Data 2025.htm' is the missing FY2024-25 generation data. It is byte-identical to SIR Data 2024.htm (same md5, same 374 bytes), titled 'SIR Data 2024', and frames FY2023-24.
- REFUTED: that FY2017-18 and FY2020-21 might be duplicate files (they share identical 108 row counts, identical 40-col width, and identical Technology/Fuel value distributions). Adversarially disproved: 2,548 of 2,808 numeric cells differ, name lists differ, metadata differs row-for-row.
- REFUTED: that the `Sum` column can be trusted as the arithmetic sum of the 12 months. `(NPPCL) - Balloki` FY2020-21 states '5,905.65' where the twelve clean monthly values sum to 5945.21 (delta 39.56 GWh). Verified against raw HTML as an upstream error.
- REFUTED: that plant utilization '% age' is bounded at 100. Values up to 121.09 (FY23-24) and 112.2 (FY17-18) are real published data.
- UNVERIFIED: the SEMANTIC difference between 'DELICENSED' and 'DECOMMISSIONED'. Both wipe the generation year while retaining capacity. The file provides no legend, glossary or footnote defining either term - I found zero asterisks or footnote blocks. My reading (licence withdrawn vs plant physically retired) is inference from ordinary English, NOT from anything the file states. Do not encode it as authoritative.
- UNVERIFIED: WHY specific plants are blank. The K-2 / Tarbela Ext.04 / Golen Gol commissioning-date explanation is consistent with the blank->data transition observed across the three files, but the files themselves state no reason. The file records only 'no value present'.
- UNVERIFIED: whether 'Export to K.Electric' generation is actually retrievable from the 'Quarterly Data (XWD & KE).htm' page that Main.htm links. I did not fetch it (out of scope). The 3 affected plants' output is simply absent from this table.
- UNVERIFIED: FY2018-19, FY2019-20, FY2021-22 and FY2022-23. I fetched only the three years requested plus a 200-check on FY2022-23 (490,461 B, as a probe control). I did NOT parse them, so I cannot claim the 32-column schema holds for them - only that it holds across the FY17-18 / FY20-21 / FY23-24 span, which brackets them. The re-sort that broke S.No happened somewhere between FY2020-21 and FY2023-24; I did NOT narrow down which year.
- UNVERIFIED: whether FY2024-25 data exists anywhere on nepra.org.pk under a wholly different directory or naming scheme. I proved it 404s at four plausible paths under `Detail of Generation/` plus `home25.htm`. I did not crawl the wider site, so 'not published anywhere' is NOT established - only 'not at this path pattern'.
- UNVERIFIED: the contents of any of the 1,261-1,526 linked monthly PDFs. I confirmed two return 200 with `application/pdf` and ~400KB, establishing the corpus is real and fetchable. I did not open or parse a single one, so I make no claim about what data they carry or whether they reconcile with the HTML cells.
- UNVERIFIED / NOT ATTEMPTED: whether the trailing 7-8 padding columns ever carry content in the unparsed years. In these three they are all-empty, but I would not assume that holds universally.
- NOT DONE, per the hard rules: no database writes of any kind; ~/psx-research/data/research.db untouched. No browser was used - plain curl in Bash only. All work is in the scratchpad directory.

### BUILD IMPLICATIONS
- PARSE VIA A ROWSPAN/COLSPAN-EXPANDING GRID, NEVER POSITIONALLY OVER RAW <td>. Build a full grid that replicates a spanning cell into every position it covers, then slice columns 0..31. My `grid.py` in the scratchpad does this and yields a uniform 40/40/39-col grid from ragged raw rows ({8:1,14:16,20:1,33:1,39:120} in FY23-24). Without this, 16 FY23-24 rows mis-assign silently.
- TRANSCODE AT INGEST, ONCE: `iconv -f WINDOWS-1252 -t UTF-8` (no `-o` on macOS - use `>`). Store the UTF-8 form. Then normalise U+00A0 to a real space explicitly and treat the result as blank - but record blankness BEFORE collapsing, because blank is a distinct semantic state, not whitespace.
- STORE FIVE CELL STATES, NOT A NULLABLE FLOAT. Recommended: `value REAL NULL` plus a `status` enum ('reported', 'not_reported', 'delicensed', 'decommissioned', 'exported_to_ke'). A single nullable float collapses 'measured 0.00', 'not reported', 'DELICENSED' and 'DECOMMISSIONED' into one indistinguishable NULL - the exact failure the task warns about. 0.00 must land as value=0.0/status=reported; blank as value=NULL/status=not_reported.
- KEY PLANTS ON `Name of Companies`, NEVER ON S.No. S.No is a per-file display ordinal that was fully reshuffled between FY2020-21 and FY2023-24 (0/106 preserved). Persist S.No only as a provenance/display field alongside the source row index. Build an explicit plant-alias table keyed on the verbatim name string, since the name is the only stable identifier and is unique within each year.
- COLUMN IDENTITY IS SAFE TO PIN, WIDTH IS NOT. The 32-column schema is byte-identical across FY17-18/FY20-21/FY23-24, so you may hard-code the logical schema (S.No, Name, Technology, Fuel, Installed MW, Dependable MW, then 13 x (%age, GWh) in fiscal Jul->Jun order with Sum last). But derive the header band by TEXT MATCH each time and assert it, rather than trusting physical width (40 vs 39) or trailing padding (8 vs 7).
- ADD A HARD SCHEMA-ASSERTION GATE AT INGEST. Locate the 3-row header band, flatten each column to its (top, mid, bottom) triple, mask the `FY \d{4}-\d{2}` label, and compare against the frozen 32-column fingerprint. Refuse to ingest - loudly - on any mismatch. This is cheap and it is what converts 'the schema happens to be stable' into 'drift cannot enter the store silently'. My `diff.py` is the working version of this check.
- USE `Sum GWh == sum(12 monthly GWh)` AS AN INDEPENDENT INGEST VALIDATOR OF COLUMN ALIGNMENT. It matched 97/97, 104/105 and 118/118 on the correct columns versus 7/97, 12/105 and 3/118 on the shifted hypothesis - a decisive signal that catches an off-by-one before it reaches the store. This is extraction quality measured from OUTSIDE the extractor, per the standing lesson.
- STORE `Sum` AS REPORTED AND FLAG DISAGREEMENT - do not recompute over it. `(NPPCL) - Balloki` FY20-21 genuinely disagrees by 39.56 GWh. Persist both the published Sum and the computed sum plus a boolean discrepancy flag, so an analyst sees the upstream error instead of inheriting it or silently overwriting NEPRA's number.
- PARSE NUMBERS DEFENSIVELY BUT NON-DESTRUCTIVELY: strip commas; accept `% age` > 100 (real values reach 121.09); expect no negatives, no parentheses, no embedded units. Any cell failing numeric parse must be routed to the status enum by exact string match against the known sentinel vocabulary - and an UNRECOGNISED non-numeric string must raise, not default to NULL. That is how a future year's new sentinel gets noticed instead of silently becoming missing data.
- TAKE ALL DESCENDANT TEXT FROM EACH CELL, AND COLLAPSE WHITESPACE AFTER TAG STRIPPING. Values hide inside `<a><span>`, and cell text wraps across source lines. A direct-child-text-only reader loses a whole column; a line-based regex splits 'Natural Gas/ Furnace Oil' and 'Name of Companies'.
- ENUMERATE FISCAL YEARS BY PROBING THE URL PATTERN, NOT BY READING Main.htm OR TRUSTING FILENAMES. Main.htm omits reachable years (FY2022-23, FY2023-24). Worse, 'SIR Data 2025.htm' returns 200 with FY2023-24 content. The CLI must resolve a year by following the frameset to `…_files/sheet001.htm` and then CONFIRMING the year from the in-document band label (`FY 2023-24` at col 6) and/or the workbook's `c_rgszSh[0]`, treating the filename as an untrusted hint. Content-hash every fetch too: byte-identical md5s are exactly how the 2024/2025 duplicate was caught.
- RECORD FY2024-25 AS ABSENT/UNPUBLISHED, NOT AS ZERO OR MISSING-BY-ERROR. Four plausible paths plus home25.htm all 404 while the FY2022-23 control returns 200. The store should carry an explicit 'not published at source as of 2026-09-08' marker so downstream code cannot mistake absence for a data-collection failure or backfill it.
- NORMALISE Technology/Fuel THROUGH AN EXPLICIT LOOKUP TABLE THAT RETAINS THE RAW STRING. The vocabularies are closed and stable (9 and 16 values, identical across all three years), so seed the table from the verified lists and make an unseen value an ingest error. Keep raw + canonical columns; do not auto-upper-case, since 'Coal' vs 'THERMAL- COAL' is a real editorial distinction in the source.
- TREAT ROWS AS PLANT/UNIT GRAIN despite the 'Name of Companies' header. Model plant as the entity and company as a derived attribute, or multi-unit operators (three Tricon Boston rows, JDW Unit-II) collapse incorrectly.
- CAPTURE THE MONTHLY PDF HREFS AS A PROVENANCE TABLE (plant, fiscal year, month, URL). 1,261-1,526 real PDFs per year, ~400KB each, confirmed 200/application/pdf. Store the href verbatim from the cell - never construct it - because the path convention differs in all three years (year-dir-first / year-in-plant-dir / no-year-in-dir) and the category dir was renamed Wapda->WAPDA. This is the audit trail for any disputed monthly figure and the natural next data source.
- PERSIST PROVENANCE PER OBSERVATION: source URL, fetch timestamp, file md5, byte count, source `<tr>` index and S.No. All three files were byte-stable across re-fetch, so md5 is a reliable change-detector for cheap incremental re-ingest and for catching silent upstream edits.
- KEEP THE UPSTREAM TYPO 'Genenration' IN THE URL BUILDER, with a comment. The corrected spelling 404s. Anyone 'fixing' it breaks every fetch.

### TRAPS
- ENCODING (load-bearing, cost me a false reading): files are windows-1252. A strict UTF-8 read raises; a naive UTF-8 grep finds nothing. The sole culprit is 0xA0 (NBSP), 128-132 per file. Always `LC_ALL=C grep -a` or transcode first.
- macOS TOOLING: BSD `grep` has NO `-P` flag and fails silently, returning a wrong count of 0 rather than an error. macOS `iconv` has NO `-o` flag. Both bit me; both produce plausible-looking wrong answers.
- NBSP IS THE BLANK. An 'empty' cell is `&nbsp;` (0xA0), NOT an empty string. `cell.strip() == ''` returns FALSE for these unless you strip U+00A0 explicitly. This is the single easiest way to silently mislabel 'not reported' as a present value.
- ZERO <th> IN THE ENTIRE FILE. There is nothing to anchor a header on semantically. Any header detection must be positional or text-matched, and any library that looks for <th> to find headers will report 'no header'.
- COLSPAN/ROWSPAN. The header is a 3-ROW band (six rowspan=3 cells + colspan=26 band label + thirteen colspan=2 month cells). Thirteen FY23-24 data rows carry a single `<td colspan=26>DELICENSED|DECOMMISSIONED`, and two carry an EMPTY `<td colspan=26>`. Raw td-per-tr in FY23-24 is {8:1, 14:16, 20:1, 33:1, 39:120}. A parser ignoring colspan reads 'DELICENSED' as Jul %age then misaligns the seven trailing padding cells into Jul GWh..Oct %age - silently, because there is no <th> to cross-check against.
- COLUMN ORDER IS `% age` THEN `GWh`, not the intuitive value-then-percent. Getting this backwards swaps all 26 monthly fields and still yields plausible-looking numbers. Guard it with the Sum=sum(12 months) identity, which matches 97/97, 104/105, 118/118 on the GWh columns but only 7/97, 12/105, 3/118 on the %age columns.
- MONTHS ARE IN FISCAL ORDER Jul->Jun. Sorting or indexing them as calendar months mis-dates every observation by up to six months.
- S.No IS NOT A STABLE KEY ACROSS YEARS. Stable FY17-18->FY20-21 (107/107 unchanged) but 0/106 unchanged in FY23-24 - the file was re-sorted by technology. Joining years on S.No would silently swap plants' entire histories (AES Lalpir 1->36, Allai Khwar 56->8). Key on `Name of Companies`.
- BLANK vs '0.00' - the semantic trap the task flags, and it is REAL. '0.00' is a measured zero (437/540/471 occurrences, coexisting with real output in the same row); NBSP-blank is absence of measurement (all-or-nothing, exactly 26 blanks per affected row, never mixed with zeros). Treating them alike invents 11 plants x 12 months of false zero generation in FY2017-18 alone.
- NON-NUMERIC SENTINELS SIT IN NUMERIC COLUMNS. 'DELICENSED', 'DECOMMISSIONED' in the monthly block; 'Export to K.Electric' in the `Installed Capacity (MW)` column. A bare float() either throws or, worse, a try/except that nulls silently converts 'DELICENSED' into the same NULL as 'not reported' - destroying the distinction. Five states must be preserved, not three.
- THOUSANDS SEPARATORS: commas in 61-68 cells per year ('3,478', '13,365.93'). `float('3,478')` throws; a naive int() on '1,601' truncates or fails. Strip commas before parsing - but only after you have decided the cell is numeric at all.
- TEXT WRAPS ACROSS SOURCE LINES: `>Name of\n  Companies<`, `>Natural Gas/\n  Furnace Oil<`, `>Export\n  to K.Electric<`. Any line-based grep/sed/regex extraction splits these into fragments or misses them. Whitespace must be collapsed AFTER tag stripping.
- VALUES NESTED TWO LEVELS DEEP: many %age cells are `<td><a href='...pdf'><span>69.07</span></a></td>`. Reading only a <td>'s direct text node yields empty for these - a whole column of silent nulls. Take all descendant text.
- 'BLANK' HAS TWO HTML ENCODINGS IN THE SAME FILE: bare `&nbsp;` and `<u style='visibility:hidden;mso-ignore:visibility'>&nbsp;</u>` (117x in FY17-18, 26x in FY20-21, 0x in FY23-24). And a third across years: the empty `<td colspan=26>` in FY23-24. Same meaning, three shapes.
- '% age' EXCEEDS 100 (up to 121.09). A `0<=pct<=100` sanity check silently drops 8-16 real cells per year.
- THE `Sum` COLUMN DOES NOT ALWAYS RECONCILE: `(NPPCL) - Balloki` FY20-21 states '5,905.65' vs 5945.21 actual (delta 39.56 GWh). If you recompute Sum you overwrite NEPRA's published figure; if you assume it reconciles, your validator fires a false positive. Store as-reported AND flag the mismatch.
- TRAILING PADDING COLUMNS (7-8 all-empty) mean the real width is 32, not 39/40. Never hard-code physical width and never index from the right-hand end - the padding count itself differs between years (8 vs 7).
- THE FILENAME TYPO IS UPSTREAM AND LOAD-BEARING: 'Genenration', not 'Generation'. The typo-corrected URL 404s. ('Genenration' also appears 2x inside the FY23-24 content itself.)
- 'SIR Data 2025.htm' IS A 200-OK DECOY. Plausible name, HTTP 200, but byte-identical to the 2024 stub and serving FY2023-24. Enumerating fiscal years by filename would double-count FY2023-24 as FY2024-25 - a fabricated year of data with a real HTTP 200 behind it.
- THE INDEX IS INCOMPLETE (confirms prior finding): Main.htm links only FY2017-18..FY2021-22. FY2022-23 and FY2023-24 are reachable but unlinked. Never use Main.htm as the sole enumerator - probe the URL pattern directly.
- PDF PATH CONVENTION HAS NO STABLE TEMPLATE across years (year-dir-first vs year-in-plant-dir vs no-year-in-dir), plus a `Wapda`->`WAPDA` case rename. Any hard-coded PDF path builder works for exactly one fiscal year. Note FY23-24 paths carry no year at all, so different years' PDFs can collide in one directory.
- MIXED-CASE / STRAY-SPACE CATEGORY VALUES: 'COAL' vs 'Coal', 'GAS' vs 'Natural Gas', 'THERMAL- COAL' (space after hyphen), 'Natural Gas/ Furnace Oil' (space after slash). Naive GROUP BY splits the same category into several. But do NOT blindly upper-case-and-collapse either: `Technology='Coal'` (n=1, Lakhra) and 'THERMAL- COAL' are distinct strings in the source and should be normalised deliberately, with the raw value retained.
- ROWS ARE PLANT/UNIT-LEVEL DESPITE THE HEADER SAYING 'Name of Companies'. Three `Tricon Boston Consulting Corporation` rows in FY17-18; `JDW Sugar Mills Limited. (Unit-II) Rahim Yar Khan`. Treating the column as a company identifier collapses distinct units.
- COINCIDENTAL METADATA IDENTITY: FY17-18 and FY20-21 share identical row counts (108), identical widths, and identical Technology/Fuel value sets. A fingerprint based on those alone would wrongly conclude the files are duplicates. Only the numeric block distinguishes them (2,548/2,808 cells differ).


########## PROBE: nepra-per-disco-reliability ##########

## 0. WHAT I FETCHED (plain curl only, no browser)

Base path is `https://nepra.org.pk/` + the `Standards/…` or `M&E/PER/Distribution/…` fragment. `https://nepra.org.pk/publications/Standards/…` and lowercase `standards/` both 404 — case and the missing `publications/` prefix matter.

`--max-time 30` was insufficient for a cold Cloudflare MISS (first full GET of the 3.02 MB FY2018-19 file died at 791,592/3,024,893 bytes after 30.003 s). The server sends `Accept-Ranges: bytes`, so I fetched every PDF as 1 MB `curl -r` chunks, each call still `-sS --max-time 30`. Byte counts below are `Content-Length` and were reconciled against `stat -f%z` on the assembled file — all seven match exactly.

| FY | URL fragment | HTTP | Content-Length | assembled bytes | `file(1)` | pages (pymupdf) | text chars |
|---|---|---|---|---|---|---|---|
| 2014-15 | `Standards/PER%20DISCOs%20and%20KE%20for%202014-15.pdf` | **200** | 1,022,593 | 1,022,593 | `PDF document, version 1.7` | 27 | 39,416 |
| 2018-19 | `Standards/2020/PER%20DISCOs%202018-19.pdf` | **200** | 3,024,893 | 3,024,893 | `PDF document, version 1.7` | 28 | 51,131 |
| 2019-20 | `Standards/2021/PER%20DISCOs%202019-20%20updated.pdf` | **200** | 3,026,235 | 3,026,235 | `PDF document, version 1.7` | 26 | 43,249 |
| 2020-21 | `Standards/2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf` | **200** | 2,277,838 | 2,277,838 | `PDF document, version 1.7` | 29 | 53,168 |
| 2021-22 | `Standards/2023/PER-DISCO%20FY%202021-22%20final.pdf` | **200** | 2,435,038 | 2,435,038 | `PDF document, version 1.7` | 39 | 73,654 |
| 2022-23 | `M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf` | **200** | 2,483,696 | 2,483,696 | `PDF document, version 1.7` | 43 | 82,813 |
| 2024-25 | `M&E/PER/Distribution/2026/PER%202024-25%20Distribution%20Companies.pdf` | **200** | 1,563,742 | 1,563,742 | `PDF document, version 1.7` | 35 | 72,584 |

Trailing-space trap **confirmed, load-bearing**: `…Distribution%20Companies%20.pdf` → `HTTP/1.1 200 OK, Content-Length: 2277838`; the same URL without `%20` → `HTTP/1.1 404 Not Found, Content-Length: 9`. One byte of URL decides it.

`file(1)` also prints `1 pages` for six of the seven — it misreads linearized `/Count`. Real page counts came from pymupdf. Don't trust `file` for page count.

## 1. TEXT LAYER — all seven are real text, NOT scans

Every one carries an embedded text layer. `Creator: Nitro Pro 8` on the six older ones (FY2024-25 has no creator string). Character counts 39,416–82,813 (table above). Embedded raster images 40–455 per file, but those are the *chart* bitmaps and logos — every table body extracts as text.

`zero_text_pages` (pages with <20 chars) = **0** for six files; the FY2020-21 file has exactly **1** (its cover page, p.1). No OCR is required anywhere. `pdftotext`/`pdfinfo`/`qpdf` are not installed on this box; `pymupdf` and `pypdf 6.14.2` are. I used pymupdf, and verified the critical cells a second time at *span* level (text + bbox + font size) so the numbers below are not artifacts of reading-order flattening.

## 2. METRICS ACTUALLY PRESENT (verbatim section titles)

FY2018-19 table of contents, quoted exactly:

> `2.1 Transmission & Distribution Losses (%)` / `2.1.1 Financial Impact due to breach of losses target` / `2.2 Recovery (%)` / `2.2.1 Financial Impact due to breach of recovery target` / `2.3 System Average Interruption Frequency Index (SAIFI – No.)` / `2.4 System Average Interruption Duration Index (SAIDI - Min)` / `2.5 Time Frame for New Connections (%)` / `2.6 Load Shedding (Hrs)` / `2.7 Nominal Voltage` / `2.8 Consumer Service Complaints` / `2.9 Safety (No. of Fatalities for both Employees & General Public)` / `2.10 Fault Rate`

So: **T&D losses vs target — yes. SAIFI — yes. SAIDI — yes. Recovery ratio vs 100% target — yes. Consumer complaints — yes.** Plus, in every year: financial loss in Rs. million from breaching the loss and recovery targets, % pending ripe connections vs the PSDR 5% limit, load-shedding hours, nominal-voltage complaints, fatal accidents (employee/public split), and fault rate (faults/km). No CAIDI, no MAIFI, no ENS table in any year (the string "ENS" appears in FY2022-23/FY2024-25 prose only).

Keyword counts confirm presence rather than mere mention — e.g. FY2021-22: SAIFI ×37, SAIDI ×33, `T&D` ×23, Target ×44, Recovery ×30, Complaint ×60, Loss ×83.

## 3. VERBATIM TABLE ROWS (value shape on the record)

**FY2018-19 `TABLE 1` (T&D), header `Name of DISCO | Actual Reported (%) | Allowed in Tariff (%) | Breach of Target (%)` with a `(1) (2) (3) 4=(2-3)` formula row:**
```
PESCO 36.6 31.95 4.65     IESCO 8.86 8.65 0.21      GEPCO 9.87 10.03 (0.16)
FESCO 9.8 10.24 (0.44)    LESCO 13.2 11.76 1.44     MEPCO 15.8 15.00 0.8
QESCO 23.6 17.50 6.1      SEPCO 37.0 29.75 7.25     HESCO 29.5 22.59 6.91
K-Electric 19.1 18.75 0.35            W. Av: 17.923 16.181 1.742
```
Negative breaches are printed in **accounting parentheses** `(0.16)`, not with a minus sign. FY2019-20 switched to `-0.52`. FY2021-22 uses `-0.44`.

**FY2018-19 `TABLE 5` (SAIFI) — `Reported Figures (No.) | Target set by NEPRA (No.) | Breach of Target (No.)`:**
```
PESCO 189.01 253.66 0   IESCO 0.05 13 0      GEPCO 27.13 13 14.13
FESCO 36.86 37.43 0     LESCO 30.19 41.88 0  MEPCO 369.16 142.61 226.55
QESCO 97.98 86.86 11.12 SEPCO 516.37 67.99 448.38  HESCO 170.86 123.40 47.46
K-Electric 28.95 17.20 11.75
```

**FY2018-19 `TABLE 6` (SAIDI, minutes):**
```
PESCO 16696.51 17358.60 0.00   IESCO 1.27 14 0.00   GEPCO 45.19 14 31.19
FESCO 1627.99 1605.08 22.91    LESCO 3538.93 716.81 2,822.12
MEPCO 31419.30 9704.00 21,715.3  QESCO 8402.4 3735.58 4,666.82
SEPCO 4306.74 606.47 3,700.27  HESCO 10973.67 5227.11 5,746.56
K-Electric 2950.22 751.73 2,198.49
```

**FY2024-25 `Table 01` (T&D) — note the row order and the missing K-Electric:**
```
PESCO 37.15 19.26 17.89  IESCO 8.61 7.31 1.3   GEPCO 10.6 8.9 1.7
FESCO 9.02 8.38 0.64     LESCO 13.7 9.46 4.24  MEPCO 13.81 11.34 2.47
QESCO 38.38 13.81 24.57  SEPCO 39.18 16.31 22.87  HESCO 27.89 17.55 10.34
W. AVG: 17.55 11.43 6.12
K-Electric *  -  -  -
```
`W. AVG:` sits at row 10 and `K-Electric *` at row 11, with K-Electric's real numbers only in the footnote prose: *"K-Electric's reported actual T&D loss is 14.73% against the target of 14.27%… has not been incorporated in the report."*

**DISCOs that appear:** PESCO, IESCO, GEPCO, FESCO, LESCO, MEPCO, QESCO, SEPCO, HESCO, K-Electric — exactly ten, in every one of the seven reports.

**TESCO is absent from the reliability tables in all seven.** String count for `TESCO`: 0 in FY2018-19, FY2019-20, FY2020-21, FY2021-22, FY2022-23; 2 in FY2014-15; 3 in FY2024-25. In FY2014-15 it appears only in the *complaints* table (`TESCO 384,031 5,892 1.5 16`) alongside a twelfth entity, `BTPL 16,879 1,838 10.9 5` — so that one report has a **12-entity complaints table and a 10-entity SAIFI/SAIDI chart**. FY2024-25 explains the exclusion verbatim: *"although the data has been obtained from TESCO… TESCO's distribution and data recording systems are largely unreliable for most of the parameters, making data unsuitable for inclusion in this report. Consequently, TESCO's data has not been incorporated."*

## 4. THE 3.00x CLAIM — **REPRODUCED**, in the FY2024-25 report, for MEPCO, on both indices

The FY2024-25 PER prints MEPCO's SAIFI and SAIDI twice each, and the two printings differ by a factor of three.

**SAIFI.** `Table 05: System Average Interruption Frequency Index (SAIFI)`, page 12 of 34 (PDF p.13), header `FY 2024-25 / Name of DISCO | Reported Figure (NO.) | Target by NEPRA (No.) | Breach of Target`:
```
MEPCO   30.67   13   Near to Limit
```
span-verified: `'MEPCO' x=135.5 y=676.0`, `'30.67' x=229.5 y=676.1`, `'13' x=323.5`, `'Near to Limit' x=390.7`.

`Table 17: System Average Interruption Frequency Index (SAIFI)`, page 27 of 34 (PDF p.28), 2024-25 column:
```
MEPCO   471   43.94   34.26   31.57   10.23
```
span-verified: `'MEPCO' x=97.5 y=234.9`, `'10.23' x=476.7 y=234.6`.

**30.67 / 10.23 = 2.998045x**

**SAIDI.** `Table 06: System Average Interruption Duration Index (SAIDI)`, page 14 of 34 (PDF p.15):
```
MEPCO   3547.00   14   Far Away
```
span-verified `'3547.00' x=202.6 y=171.6`.

`Table 18: System Average Duration Frequency Index (SAIDI)`, page 28 of 34 (PDF p.29), 2024-25 column:
```
MEPCO   39.733   2794   4723.73   3726.61   1182.56
```
span-verified `'1182.56' x=467.7 y=237.0`.

**3547.00 / 1182.56 = 2.999425x**

Each figure is *internally* triple-attested, which rules out a transcription slip on my side: `Figure 09`'s data labels carry `3547.00` (agreeing with Table 06) and `Figure 21`'s carry `1182.56` (agreeing with Table 18); likewise `Figure 08` carries `30.67`'s neighbours and `Figure 20` carries `10.23`. The report is consistently wrong in two halves. Direction is uncertain: 1182.56 × 3 = 3547.68 and 3547.00 / 3 = 1182.33, so neither is an exact multiple — it reads as a ÷3 applied at a slightly different rounding stage (MEPCO is administered in three operating regions).

**Consequence, not cosmetic:** MEPCO's SAIFI target is 13. At 30.67 MEPCO breaches by 17.67 and the report says `Near to Limit`. At 10.23 MEPCO would be the **only DISCO in the history of the series to comply with SAIFI**. The same report supports both verdicts.

**Scope of the 3.00x finding — this is the only place it occurs.** I scanned every within-report duplicate pair in all seven PDFs (headline §2 table vs §3 five-year comparison table last column, plus chart data labels) for ratios in [2.90, 3.10]:
- FY2018-19: 40 SAIFI/SAIDI pairs, **1** mismatch, and it is trivial — MEPCO SAIFI `369.16` (Table 5) vs `369.159` (Table 17), ratio 1.0000027.
- FY2019-20, FY2020-21, FY2021-22: **0** mismatches. Recovery: 40 pairs across four reports, **0** mismatches.
- FY2022-23: the biggest source of within-report duplication, and its max ratio is **1.3000x** (see §5).
- FY2024-25: MEPCO 2.998x / 2.999x, plus five small ones (LESCO SAIFI 28.16 vs 28.61 = 1.0160 — digit transposition; K-Electric SAIFI 68.46 vs 68.64 = 1.0026 — also transposed; IESCO 15.39 vs 15.31 = 1.0052; GEPCO 49.49 vs 49.41 = 1.0016; PESCO SAIDI 13469.55 vs 13469.53 = 1.0000006).

So "3.00x within one report" is true of exactly one DISCO in one report year, not a property of the series.

## 5. THE OTHER within-report duplicate: FY2022-23 publishes SAIFI and SAIDI twice by definition

FY2022-23 is the only year with a definitional split, and it is a genuine parse hazard because both variants are captioned only "SAIFI"/"SAIDI" in the TOC and both carry target 13/14 and the same `Breach of Target` wording:

- `Table 05: System Average Interruption Frequency Index (SAIFI) without LT interruptions`
- `Table 06: System Average Interruption Frequency Index (SAIFI) with LT interruptions`
- `Table 07: … (SAIDI) without LT interruptions`
- `Table 08: … (SAIDI) with LT interruptions`

with-LT ÷ without-LT ratios:

| DISCO | SAIFI w/o → w | ratio | SAIDI w/o → w | ratio |
|---|---|---|---|---|
| PESCO | 162.08 → 184.67 | 1.1394 | 12265.48 → 14,227.82 | 1.1600 |
| IESCO | 17.98 → 17.97 | **0.9994** | 1006.34 → 1,006.34 | 1.0000 |
| GEPCO | 18.35 → 22.01 | 1.1995 | 32.16 → 38.59 | 1.1999 |
| FESCO | 31.49 → 34.95 | 1.1099 | 1031.62 → 1,219.38 | 1.1820 |
| LESCO | 29.13 → 29.13 | 1.0000 | 3550.05 → 3,550.05 | 1.0000 |
| MEPCO | 28.92 → 34.26 | 1.1846 | 3633.73 → 4,723.73 | **1.3000** |
| QESCO | 86.39 → 98.37 | 1.1387 | 7020.47 → 8,083.47 | 1.1514 |
| SEPCO | 98.55 → 117.50 | 1.1923 | 1319.17 → 1,468.03 | 1.1128 |
| HESCO | 114.37 → 133.04 | 1.1632 | 6270.83 → 7,513.75 | 1.1982 |
| K-Electric | 25.35 → 25.34 | **0.9996** | 1911.72 → 1,911.72 | 1.0000 |

Max spread **1.3000x**, not 3.00x. Two rows are arithmetically impossible: IESCO and K-Electric report *fewer* interruptions **with** LT faults included than without. FY2022-23's own §3.3/§3.4 comparison tables silently adopt the **with-LT** series (PESCO 2022-23 = 184.67 / 14,227.82), so its headline `Table 05` value 162.08 disagrees with its own `Table 19` by 1.1394x.

## 6. THE 100–700x STEP CLAIM — **REPRODUCED in substance, wrong in range.** Actual band is 109x to 1,079x

**(a) Genuine reporting-basis breaks inside a single published five-year row.** From `Table 17`/`Table 18` of FY2024-25 and `Table 24`/`Table 25` of FY2021-22 (identical values, so cross-confirmed):

| DISCO | metric | step | ratio |
|---|---|---|---|
| IESCO | SAIDI | 2020-21 `1.36` → 2021-22 `1027.01` | **755.15x** |
| IESCO | SAIFI | 2020-21 `0.05` → 2021-22 `20.56` | **411.20x** |
| GEPCO | SAIDI | 2022-23 `38.59` → 2023-24 `4216.56` | **109.27x** |
| MEPCO | SAIDI | 2020-21 `39.733` → 2021-22 `2794` | 70.32x |

NEPRA explains the IESCO break itself, FY2021-22 p.31, verbatim: *"the figure of IESCO has significantly gone upward from 0.05 to 20.5 because previously it had miscalculated the SAIFI and now whole mechanism has been clarified to IESCO after several meetings with NEPRA team."* (Note the prose says `20.5`, the table says `20.56`.) And for MEPCO: *"MEPCO has drastically reduced the number of SAIFI as it had made its understandings better for calculating SAIFI and excluded all type of planned outages."* Nine years earlier the FY2018-19 report had already written: *"thorough inspection of IESCO's data pertaining to SAIFI and SAIDI for the year 2016-17 was carried out and found that IESCO is continuously misreporting the data. Accordingly, the Authority… imposed a penalty of Rs. 04 Million."* These are definition/enforcement breaks, not signal.

**(b) A hard 1000x data-entry error, propagated across three later reports.** MEPCO SAIDI FY2020-21:
- FY2020-21 PER `Table 6` (p.11): `MEPCO 39733 14 Far Away`; its `Figure 7` data labels also print `39733`; its `Table 16` (p.22) also prints `39733`. Span-verified `'39733' x=471.9 y=227.4`.
- FY2021-22 PER `Table 25` (p.32): `MEPCO … 39.733 …`. Span-verified `'39.733' x=419.2 y=243.9`.
- FY2022-23 PER `Table 20`: `39.73`. FY2024-25 PER `Table 18`: `39.733`.

**39733 / 39.733 = exactly 1000.0000x.** A comma became a period and the corruption became canonical.

The same comma→period corruption appears at least twice more, both 1000x, in FY2018-19 `TABLE 22` (Consumer Service Complaints):
- `IESCO 62.167 …` for 2014-15, where the FY2014-15 report's own `Table 2` prints `IESCO 2,423,317 62,167 2.6 170`.
- `SEPCO 8,857 8,516 9.085 28,900 7,571` for 2016-17, where FY2020-21 `Table 20` prints `9085`.
- And `K-Electric … 2.675,268 …` for 2016-17 — **not a parseable number at all**; FY2020-21 prints `2675268`.

**(c) The target series steps 100–603x, which is probably what the original screen actually measured.** NEPRA abandoned per-DISCO SAIDI targets (10% reduction on a 5-year mean) after FY2019-20 and reverted to the flat PSDR-2005 statutory 14 minutes:

| DISCO | SAIDI target FY2019-20 | FY2020-21 | step |
|---|---|---|---|
| PESCO | 15,100.93 | 14 | **1,078.64x** |
| MEPCO | 8,439.82 | 14 | **602.84x** |
| HESCO | 4,450.88 | 14 | **317.92x** |
| QESCO | 3,188.62 | 14 | **227.76x** |
| FESCO | 1,402.03 | 14 | **100.14x** |
| K-Electric | 649.48 | 14 | 46.39x |
| LESCO | 554.98 | 14 | 39.64x |
| SEPCO | 484.33 | 14 | 34.59x |

Four of these land squarely in 100–700x. The SAIFI targets made the same jump but only 1.25x–18.42x (`PESCO 239.44 → 13`), because the SAIFI limit is 13 not 14 relative to much smaller numbers.

## 7. CROSS-REPORT RESTATEMENT — small everywhere except the 1000x error

Every report restates the prior four years. I diffed every (metric, DISCO, year) cell that appears in two or more reports:

- **SAIFI/SAIDI:** exactly **one** conflict across all overlapping cells — MEPCO SAIDI 2020-21, the 1000x one. Every other restated SAIFI/SAIDI value is byte-identical across reports. The series is *stable*; that is a real, useful property.
- **T&D losses:** 5 conflicts, all rounding — `GEPCO 2017-18 {10.01, 10.01, 10.1, 10.1}`, `HESCO 2020-21 {28.2, 28}`, `LESCO 2020-21 {11.96, 12}`, `MEPCO 2020-21 {14.97, 14.9}`, `QESCO 2020-21 {27.96, 27.9}`. Max spread 0.20 pp.
- **Recovery:** 1 conflict — `IESCO 2018-19 {FY2018-19: 88.0, FY2019-20: 90, FY2020-21: 90, FY2021-22: 90}`, a 2.00 pp upward revision (1.0227x) never annotated.
- FY2022-23 `Table 20` prints `SEPCO 1,468.02` where its own `Table 08` prints `1,468.03`, and `HESCO 7,513.70` vs `7,513.75`.

## 8. PARSE BURDEN — moderate-to-hard, and layouts are NOT consistent across years

Text extraction is easy. Getting a *correct* per-DISCO-per-year table is not. Concrete, observed obstacles:

1. **Two tables per metric per report, no stable identity.** Each report has a headline §2 table for its own FY *and* a §3 five-year comparison table whose last column repeats it. Which you pick changes the answer for MEPCO FY2024-25 by 3x.
2. **Table numbering is not stable.** SAIFI is `TABLE 5` in FY2018-19 and FY2019-20, `Table 5` in FY2020-21, `Table 14` in FY2021-22, `Table 05`+`Table 06` in FY2022-23, `Table 05` in FY2024-25. Comparison SAIFI is `TABLE 17` / `TABLE 15` / `Table 15` / `Table 24` / `Table 19` / `Table 17`.
3. **Column schema changes.** FY2018-19/FY2019-20 `Breach of Target` is numeric with a `4=(2-3)` formula row. From FY2020-21 it becomes free text: `Far Away`, `Near to Limit`, `Within Limit`. FY2024-25 adds a third value `Away`. Any numeric parser silently drops the column.
4. **Comparison-table year windows slide and the headings are stale.** FY2019-20's §3 heading reads `Comparison of Data for the year 2018-19 with last four years (2015-16, 2016-17, 2017-18 and 2018-19)` — wrong year, twice, in a FY2019-20 report. FY2020-21's reads `COMPARISION OF DATA FOR FY2019-20 WITH LAST FOUR YEARS (2016-17, 2017-18, 2018-19, & 2019-20)`. **Never key columns off the section heading** — the actual column headers (`2016-17 … 2020-21`) are correct.
5. **Captions and bodies interleave.** On FY2022-23 PDF p.17, reading order emits `Table 07: …without LT`, `Figure 10: …without LT`, `Table 08: …with LT`, then body-1, then body-2. By bbox, body-1 (`PESCO 12265.48` at y=153.6) is *above* the Figure 10 caption (y=521.7) and body-2 (`PESCO 14,227.82` at y=576.8) is below it. Caption-adjacency labelling mislabels both tables. You need coordinates.
6. **Cell merges break rows.** FY2018-19 `TABLE 22`, LESCO row renders as `227,596` then a single span `1,548,464 1,245,699` then `6,231,274` then `548,487` — two cells fused, so a per-span row parser sees 4 values for 5 years and shifts.
7. **Chart data labels leak into the text stream and disagree with the tables.** FY2018-19 `TABLE 1` picks up `K-Electric 0 10 20 30 40` — the y-axis ticks of Figure 2. In FY2024-25, `Figure 08` labels (`15.31`, `49.41`, `28.16`, `68.64`) and `Figure 20` labels (`15.31`, `49.41`, `28.61`, `68.64`) disagree with each other *and* with `Table 05` (`15.39`, `49.49`, `28.16`, `68.46`) and `Table 17` (`15.39`, `49.49`, `28.61`, `68.64`). Four sources, four different LESCO/IESCO/GEPCO/K-Electric values.
8. **FY2014-15 has no SAIFI/SAIDI table at all** — they exist only as Excel chart data labels in `Figure 3` and `Figure 4`, and the labels are **truncated by column width**: `19,535.`, `28,189.`, `15,896.`, `13,419.`, `27,946.`, `12813.9`, `17704.6`, `27934.9`, `15677.6`. FY2018-19 `TABLE 18` shows PESCO 2014-15 SAIDI is really `27934.98` and MEPCO `15677.65` — the last digit is gone in the source. FY2010-11..FY2013-14 SAIDI is therefore **unrecoverable to full precision from that PDF**.
9. **FY2014-15 chart rows have silent missing cells.** The recovery chart's `2010-11` and `2011-12` rows carry **9 values for 10 named series** (`93, 85.4, 98.8, 97.04, 98.1, 97.97, 41, 76.3, 90.17`), while `2012-13`/`2013-14`/`2014-15` carry 10. Positional parsing misassigns every DISCO after the gap.
10. **Number formatting is inconsistent inside a single document.** FY2022-23 `Table 07` uses `12265.48` (no separator) and `Table 08` uses `14,227.82` (comma) for the same metric on facing pages. Negatives appear as `(0.16)` (FY2018-19) and `-0.52` (FY2019-20+). Some cells are unparseable: `2.675,268`, `2,317947`.
11. **Roster gaps.** FY2024-25 `Table 01` has `W. AVG:` at row 10 and `K-Electric *` with `-  -  -` at row 11; K-Electric's real T&D figures (14.73% vs 14.27%) live only in a footnote paragraph. Positional row-10 = K-Electric returns the weighted average.
12. **The official index has a dead link.** `https://nepra.org.pk/publications/Performance%20Reports.php` (HTTP 200, 57,982 B) is the authoritative enumerator, but its `../M&E/PER/Distribution/PER%20DISCOs%202023-24.pdf` entry returns **404** on three retries and on six encoding/path variants (`%26` vs literal `&`, trailing `%20`, `/2024/`, `/2025/`, uppercase `DISCOS`) and with `-e` Referer set. FY2023-24's own report is unfetchable; its values are only knowable second-hand from FY2024-25's comparison columns.
13. **Filename patterns are unguessable.** `PER DISCOs 2016-17 (FFinal).pdf` (double F), `PER 2022-23 - DSICOs.pdf` (DSICOs), `NEPRA PER 2021 Distribution Companies .pdf` (trailing space), `PER DISCOs 2019-20 updated.pdf`, and a directory move from `/Standards/<pubyear>/` to `/M&E/PER/Distribution/` after FY2021-22 with `/2026/` reappearing for FY2024-25. Enumerate the index page; never construct.
14. **A half-year report shares the directory.** `Performance Evaluation Report - Distribution Companies Jul-Dec 2024.pdf` (HTTP 200, 4,893,527 B) is a Jul–Dec 2024 semiannual PER that will collide with any annual-FY primary key.

Estimated build cost for a clean per-DISCO-per-year reliability table: **coordinate-aware extraction with a hand-written per-report-year table map**, roughly 12–15 distinct layout cases across the 13 published DISCO PERs, plus a value-level reconciliation pass. Not an OCR problem; a schema-drift problem.

### VERIFIED
- All seven PER PDFs fetched return HTTP/1.1 200 with Content-Type: application/pdf, and assembled bytes equal Content-Length exactly: FY2014-15 1,022,593 B; FY2018-19 3,024,893 B; FY2019-20 3,026,235 B; FY2020-21 2,277,838 B; FY2021-22 2,435,038 B; FY2022-23 2,483,696 B; FY2024-25 1,563,742 B.
- Base path is https://nepra.org.pk/Standards/... (or /M&E/PER/Distribution/...). The variant https://nepra.org.pk/publications/Standards/2020/PER%20DISCOs%202018-19.pdf returns 404 (Content-Length 9), and lowercase /standards/ also returns 404.
- TRAILING-SPACE TRAP CONFIRMED: .../Standards/2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf -> HTTP/1.1 200 OK, Content-Length: 2277838. The same URL without the trailing %20 -> HTTP/1.1 404 Not Found.
- Every PDF has a REAL TEXT LAYER, not a scan. Extracted character counts: 39,416 / 51,131 / 43,249 / 53,168 / 73,654 / 82,813 / 72,584. Pages with <20 chars: 0 for six files, exactly 1 (the cover) for FY2020-21. Creator metadata is 'Nitro Pro 8' on six of seven. No OCR needed anywhere.
- file(1) reports 'PDF document, version 1.7' for all seven but prints '1 pages' for six of them; real page counts via pymupdf are 27/28/26/29/39/43/35. Do not trust file(1) for page count.
- Reliability metrics ACTUALLY present in every year, verbatim from the FY2018-19 TOC: '2.1 Transmission & Distribution Losses (%)', '2.2 Recovery (%)', '2.3 System Average Interruption Frequency Index (SAIFI - No.)', '2.4 System Average Interruption Duration Index (SAIDI - Min)', '2.5 Time Frame for New Connections (%)', '2.6 Load Shedding (Hrs)', '2.7 Nominal Voltage', '2.8 Consumer Service Complaints', '2.9 Safety (No. of Fatalities...)', '2.10 Fault Rate', plus 2.1.1/2.2.1 financial impact of breaching the loss and recovery targets.
- T&D loss table carries BOTH actual and target: FY2018-19 TABLE 1 header is 'Name of DISCO | Actual Reported (%) | Allowed in Tariff (%) | Breach of Target (%)' with row 'PESCO 36.6 31.95 4.65' and 'W. Av: 17.923 16.181 1.742'.
- SAIFI and SAIDI both carry NEPRA targets: FY2018-19 TABLE 5 'PESCO 189.01 253.66 0 / IESCO 0.05 13 0 / SEPCO 516.37 67.99 448.38'; TABLE 6 'PESCO 16696.51 17358.60 0.00 / MEPCO 31419.30 9704.00 21,715.3'.
- Exactly TEN entities appear in the reliability tables of all seven reports: PESCO, IESCO, GEPCO, FESCO, LESCO, MEPCO, QESCO, SEPCO, HESCO, K-Electric. TESCO string count is 0 in five of the seven reports.
- TESCO is excluded on the record. FY2024-25 verbatim: 'although the data has been obtained from TESCO, the electricity supply to the large number of consumers remains un-metered. Furthermore, TESCO's distribution and data recording systems are largely unreliable for most of the parameters, making data unsuitable for inclusion in this report. Consequently, TESCO's data has not been incorporated.'
- FY2014-15 has a 12-ENTITY complaints table (adds BTPL 16,879 1,838 10.9 5 and TESCO 384,031 5,892 1.5 16) but a 10-entity SAIFI/SAIDI chart - the roster is inconsistent WITHIN one report.
- 3.00x CLAIM REPRODUCED (SAIDI): FY2024-25 PER Table 06 (page 14 of 34) prints 'MEPCO 3547.00 14 Far Away'; the same report's Table 18 (page 28 of 34), 2024-25 column, prints 'MEPCO 39.733 2794 4723.73 3726.61 1182.56'. 3547.00 / 1182.56 = 2.999425x. Both span-verified ('3547.00' x=202.6 y=171.6 p15; '1182.56' x=467.7 y=237.0 p29).
- 3.00x CLAIM REPRODUCED (SAIFI): FY2024-25 Table 05 (page 12 of 34) prints 'MEPCO 30.67 13 Near to Limit'; Table 17 (page 27 of 34), 2024-25 column, prints 'MEPCO 471 43.94 34.26 31.57 10.23'. 30.67 / 10.23 = 2.998045x. Span-verified.
- Each of the two MEPCO figures is triple-attested inside the report - Figure 09's data labels carry 3547.00 (matching Table 06) and Figure 21's carry 1182.56 (matching Table 18) - so the discrepancy is in the document, not in my extraction.
- The 3x discrepancy flips MEPCO's compliance verdict: SAIFI target is 13, so 30.67 is a breach labelled 'Near to Limit' while 10.23 would make MEPCO the only compliant DISCO in the entire series.
- 3.00x is NOT a general property: scanning every within-report SAIFI/SAIDI/recovery duplicate pair in all seven PDFs for a ratio in [2.90,3.10] returns MEPCO FY2024-25 only. FY2019-20, FY2020-21 and FY2021-22 have ZERO within-report mismatches; FY2018-19 has one at 1.0000027x (369.16 vs 369.159).
- FY2022-23 is the only year that publishes each index twice by definition: 'Table 05: SAIFI without LT interruptions' vs 'Table 06: SAIFI with LT interruptions', and 'Table 07: SAIDI without LT' vs 'Table 08: SAIDI with LT'. Max with/without ratio is 1.3000x (MEPCO SAIDI 3633.73 -> 4,723.73), then 1.1999x (GEPCO SAIDI 32.16 -> 38.59). Nowhere near 3.00x.
- Two FY2022-23 rows are arithmetically impossible: IESCO SAIFI without-LT 17.98 vs with-LT 17.97 (ratio 0.9994) and K-Electric 25.35 vs 25.34 (0.9996) - fewer interruptions with LT faults included than without.
- FY2022-23's own comparison tables silently adopt the WITH-LT series (Table 19 PESCO 2022-23 = 184.67, Table 20 = 14,227.82), so its headline Table 05 value 162.08 disagrees with its own Table 19 by 1.1394x.
- 100-700x STEP REPRODUCED in substance: inside a single published five-year row, IESCO SAIDI steps 1.36 (2020-21) -> 1027.01 (2021-22) = 755.15x; IESCO SAIFI 0.05 -> 20.56 = 411.20x; GEPCO SAIDI 38.59 (2022-23) -> 4216.56 (2023-24) = 109.27x. The IESCO steps appear identically in FY2021-22 Table 24/25, FY2022-23 Table 19/20 and FY2024-25 Table 17/18.
- NEPRA documents the IESCO break itself, FY2021-22 p.31 verbatim: 'the figure of IESCO has significantly gone upward from 0.05 to 20.5 because previously it had miscalculated the SAIFI and now whole mechanism has been clarified to IESCO after several meetings with NEPRA team.' (Prose says 20.5; the table says 20.56.)
- FY2018-19 p.9 verbatim on the same DISCO: 'thorough inspection of IESCO's data pertaining to SAIFI and SAIDI for the year 2016-17 was carried out and found that IESCO is continuously misreporting the data. Accordingly, the Authority took serious notice and initiated legal proceedings and imposed a penalty of Rs. 04 Million.'
- EXACT 1000x DATA-ENTRY ERROR, propagated across three later reports: MEPCO SAIDI FY2020-21 is 39733 in the FY2020-21 PER (Table 6 p.11, Figure 7 labels, and Table 16 p.22 - span-verified '39733' x=471.9 y=227.4) but 39.733 in FY2021-22 Table 25 (span-verified x=419.2 y=243.9), 39.73 in FY2022-23 Table 20, and 39.733 in FY2024-25 Table 18. 39733/39.733 = exactly 1000.0000x.
- The same comma-to-period corruption occurs at least twice more, both 1000x: FY2018-19 TABLE 22 prints 'IESCO 62.167' for 2014-15 where FY2014-15's Table 2 prints 62,167; and 'SEPCO ... 9.085' for 2016-17 where FY2020-21 Table 20 prints 9085. A third cell, 'K-Electric ... 2.675,268', is not a parseable number at all (real value 2675268).
- THE TARGET SERIES is where 100-700x really lives: NEPRA dropped per-DISCO SAIDI targets after FY2019-20 and reverted to the flat PSDR-2005 14-minute limit. Steps FY2019-20 -> FY2020-21: PESCO 15,100.93 -> 14 = 1,078.64x; MEPCO 8,439.82 -> 14 = 602.84x; HESCO 4,450.88 -> 14 = 317.92x; QESCO 3,188.62 -> 14 = 227.76x; FESCO 1,402.03 -> 14 = 100.14x. SAIFI targets made the same jump but only 1.25x-18.42x.
- THE REPORTED SAIFI/SAIDI SERIES IS OTHERWISE REMARKABLY STABLE ACROSS REPORTS: diffing every (metric, DISCO, year) cell restated in two or more reports yields exactly ONE conflict - the MEPCO 1000x one. All other restated SAIFI/SAIDI values are byte-identical across reports.
- T&D loss restatements are rounding-only: 5 conflicts, max spread 0.20 pp (GEPCO 2017-18 {10.01,10.1}; HESCO 2020-21 {28.2,28}; LESCO 2020-21 {11.96,12}; MEPCO 2020-21 {14.97,14.9}; QESCO 2020-21 {27.96,27.9}).
- Recovery has exactly one silent revision: IESCO 2018-19 is 88.0 in the FY2018-19 report but 90 in FY2019-20, FY2020-21 and FY2021-22 (1.0227x), never annotated.
- SECTION HEADINGS CARRY THE WRONG FISCAL YEAR. FY2019-20's TOC reads 'Comparison of Data for the year 2018-19 with last four years (2015-16, 2016-17, 2017-18 and 2018-19)'; FY2020-21's reads 'COMPARISION OF DATA FOR FY2019-20 WITH LAST FOUR YEARS (2016-17, 2017-18, 2018-19, & 2019-20)'. The in-table column headers are correct.
- BREACH COLUMN SCHEMA CHANGES MID-SERIES: numeric with a '4=(2-3)' formula row in FY2018-19/FY2019-20; free text from FY2020-21 ('Far Away', 'Near to Limit', 'Within Limit'); FY2024-25 adds a fourth value 'Away'.
- CAPTION/BODY INTERLEAVE: on FY2022-23 PDF p.17 the reading-order stream emits both captions (Table 07 without-LT, Table 08 with-LT) before both bodies. By bbox, body-1 ('PESCO 12265.48' at y=153.6) sits ABOVE the 'Figure 10: ...without LT' caption at y=521.7, and body-2 ('PESCO 14,227.82' at y=576.8) below it. Caption-adjacency labelling mislabels both.
- FY2014-15 HAS NO SAIFI/SAIDI TABLE - only Excel chart data labels in Figure 3/Figure 4, and they are TRUNCATED by column width: '19,535.', '28,189.', '15,896.', '13,419.', '27,946.', '27934.9', '15677.6'. FY2018-19 TABLE 18 shows the true values are 27934.98 and 15677.65 - the last digit is lost in the source.
- FY2014-15's recovery chart rows for 2010-11 and 2011-12 carry NINE values for TEN named series (93, 85.4, 98.8, 97.04, 98.1, 97.97, 41, 76.3, 90.17), while 2012-13/2013-14/2014-15 carry ten. Positional parsing misassigns every DISCO after the gap.
- CHART LABELS DISAGREE WITH TABLES AND WITH EACH OTHER in FY2024-25: Table 05 says IESCO 15.39, GEPCO 49.49, LESCO 28.16, K-Electric 68.46; Table 17 says 15.39, 49.49, 28.61, 68.64; Figure 08 labels say 15.31, 49.41, 28.16, 68.64; Figure 20 labels say 15.31, 49.41, 28.61, 68.64. Four sources, four different value sets.
- CHART AXIS TICKS LEAK INTO TABLE PARSES: FY2018-19 TABLE 1 picks up 'K-Electric 0 10 20 30 40' - the y-axis ticks of Figure 2 - as an eleventh row.
- CELL MERGE BREAKS A ROW: FY2018-19 TABLE 22 LESCO renders as '227,596' then a single fused span '1,548,464 1,245,699' then '6,231,274' then '548,487' - 4 spans for 5 years.
- NUMBER FORMATTING IS INCONSISTENT WITHIN ONE DOCUMENT: FY2022-23 Table 07 uses '12265.48' (no separator) while Table 08 uses '14,227.82' (comma) for the same metric on facing pages. Negatives are '(0.16)' in FY2018-19 and '-0.52' in FY2019-20+.
- FY2024-25 Table 01 puts 'W. AVG: 17.55 11.43 6.12' at row 10 and 'K-Electric *  -  -  -' at row 11; K-Electric's real T&D figures (14.73% actual vs 14.27% target) appear only in the footnote prose, not the table.
- AUTHORITATIVE ENUMERATOR FOUND: https://nepra.org.pk/publications/Performance%20Reports.php (HTTP 200, 57,982 B) lists all 13 DISCO PERs including the four I had no path for (2015-16, 2016-17, 2017-18, 2022-23, 2024-25). All HEAD 200 except one.
- THE OFFICIAL INDEX CONTAINS A DEAD LINK: '../M&E/PER/Distribution/PER%20DISCOs%202023-24.pdf' returns HTTP 404 on three retries and on six variants (%26 vs literal &, trailing %20, /2024/ and /2025/ subdirs, uppercase DISCOS) and with -e Referer set to the index page.
- FILENAMES ARE UNGUESSABLE - upstream typos confirmed by 200 responses: 'PER DISCOs 2016-17 (FFinal).pdf' (double F, 1,765,126 B), 'PER 2022-23 - DSICOs.pdf' (DSICOs, 2,483,696 B), 'NEPRA PER 2021 Distribution Companies .pdf' (trailing space), 'PER DISCOs 2019-20 updated.pdf'. Directory moved from /Standards/<pubyear>/ to /M&E/PER/Distribution/ after FY2021-22, then to /M&E/PER/Distribution/2026/ for FY2024-25.
- A HALF-YEAR REPORT SHARES THE DIRECTORY: 'Performance Evaluation Report - Distribution Companies Jul-Dec 2024.pdf' returns HTTP 200, Content-Length 4,893,527 - a Jul-Dec 2024 semiannual PER that collides with any annual-FY primary key.
- The server sends 'Accept-Ranges: bytes' and 'Server: cloudflare'; a cold cf-cache-status MISS delivers only ~26 KB/s so a 3 MB file cannot complete inside --max-time 30, but 1 MB Range requests complete in ~0.23 s on a HIT. Ranged chunking is the way to honour the 30 s cap.

### REFUTED/UNVERIFIED
- REFUTED AS STATED - 'contradict themselves by exactly 3.00x': the ratios are 2.998045x (SAIFI) and 2.999425x (SAIDI), not exactly 3.00. 1182.56 x 3 = 3547.68 and 3547.00 / 3 = 1182.33, so neither figure is an exact multiple of the other. Consistent with a divide-by-three applied at a different rounding stage, not a clean factor.
- REFUTED AS STATED - 'within one report' as a general property: the ~3.00x discrepancy exists in exactly ONE report (FY2024-25) for exactly ONE DISCO (MEPCO), on both indices. FY2019-20, FY2020-21 and FY2021-22 have zero within-report SAIFI/SAIDI mismatches; FY2018-19 has one at 1.0000027x; FY2022-23's within-report duplication maxes at 1.3000x. A CLI must not present 3x self-contradiction as characteristic of the series.
- REFUTED AS STATED - the '100-700x' RANGE is wrong at both ends. The observed year-over-year steps are 109.27x (GEPCO SAIDI 2022-23->2023-24), 411.20x (IESCO SAIFI), 755.15x (IESCO SAIDI) and 803.45x (the MEPCO 39.733 artifact); the target-series steps run 34.59x to 1,078.64x. Nothing clusters at 100-700x - four of the eight largest exceed 700x.
- UNVERIFIED PROVENANCE of the original screen's numbers: I cannot tell whether the earlier screen measured the reported series, the target series, or the 1000x MEPCO artifact. My best reconstruction is that '100-700x' came from the SAIDI TARGET reversion to the flat 14-minute PSDR limit (MEPCO 602.84x, HESCO 317.92x, QESCO 227.76x, FESCO 100.14x all sit in-band) rather than from any reported figure. That is inference, not proof.
- FY2023-24 PER IS NOT RETRIEVABLE. The only path NEPRA publishes for it - ../M&E/PER/Distribution/PER%20DISCOs%202023-24.pdf - returns HTTP 404 on every attempt. Recorded as UNAVAILABLE, not missing-data. FY2023-24 SAIFI/SAIDI values are knowable only second-hand from the FY2024-25 comparison columns (e.g. GEPCO SAIDI 4216.56, K-Electric SAIFI 71.31) and are therefore unverified against a primary source.
- FY2010-11 through FY2013-14 SAIDI VALUES ARE UNRECOVERABLE from the FY2014-15 PDF: they exist only as truncated Excel chart data labels ('19,535.', '28,189.', '15,896.', '13,419.', '27,946.', '21241', '1321', '1250', '1137'). The trailing digits are absent from the text layer. Record as UNVERIFIED rather than reading the truncated string as a number.
- WHICH DISCO IS BLANK in the FY2014-15 recovery chart's 2010-11 and 2011-12 rows is UNDETERMINED. Nine values are printed for ten named series. K-Electric/KESC is the plausible omission (it is the only series with a shorter history and the 2012-13 row's tenth value, 88.65, is K-Electric's), but I could not prove it and did not assume it.
- FIVE PER PDFs CONFIRMED REACHABLE BUT SHAPE UNVERIFIED - I recorded HTTP 200 and Content-Length only, did not download or parse: Standards/Performance Evaluation Report of DISCOs and K-Electric.pdf (7,929,032 B, year unstated in the filename), Standards/Performance Evaluation Report of Distribution Companies & K-Electric for Year 2013-14.pdf (3,134,331 B), Standards/2017/PER 2015-16.pdf (1,911,376 B), Standards/2018/PER DISCOs 2016-17 (FFinal).pdf (1,765,126 B), Standards/2019/PER DISCOs 2017-18 (Final).pdf (1,708,069 B), and M&E/PER/Distribution/Performance Evaluation Report - Distribution Companies Jul-Dec 2024.pdf (4,893,527 B).
- DIRECTION OF THE MEPCO 3x ERROR IS UNKNOWN. I cannot establish which of 30.67/3547.00 or 10.23/1182.56 is correct. The headline table and the comparison table are each internally consistent with their own chart, so there is no tie-breaker inside the document. MEPCO's three operating regions make a divide-by-three plausible but that is speculation.
- DIRECTION OF THE 1000x MEPCO SAIDI ERROR IS INFERRED, NOT PROVEN. 39733 is more plausible on its face (MEPCO's neighbours are 31,920.87 the year before and 2,794 the year after; 39.733 min/yr would be world-class and inconsistent with 471 interruptions), and three of four reports print the corrupted form. But NEPRA never issues a correction, so 'which is right' remains unverified. Record both with a conflict flag.
- NO FY2015-16, FY2016-17 or FY2017-18 REPORT WAS PARSED, so the SAIFI/SAIDI target methodology for those years (the '5% reduction over the mean of the last five years' referenced in FY2014-15 and FY2018-19) is not verified against its own report.
- I did NOT test whether the Performance Standards (Distribution) Regulations 2026 (SRO 1430(I)-2026 dated 25-08-2026, linked from the homepage) changes the 13/14 statutory limits. If it does, the target series breaks again after FY2024-25. Unverified.
- NO ROBOTS.TXT CHECK WAS PERFORMED this session; I relied on the brief's statement about the Cloudflare named-bot block and used the supplied browser UA throughout. No self-identification as ClaudeBot. Reference/analysis use only, no training use.

### BUILD IMPLICATIONS
- DO NOT collapse a metric to one number per DISCO-year. Model it as (metric, disco, fiscal_year, source_report, source_table, source_page, variant, value) and let the CLI surface conflicts. The MEPCO FY2024-25 3x case and the MEPCO 2020-21 1000x case are unrepresentable in a flat table without silently choosing a side.
- Add an explicit `variant` dimension with at least: `headline` (section-2 table for the report's own FY), `comparison` (section-3 five-year table), `chart_label`, plus `with_lt` / `without_lt` for FY2022-23. Default the CLI to `headline` for the report's own year and `comparison` for back-years, and print which one it used.
- Emit a hard `conflict` flag whenever two sources for the same (metric, disco, year) disagree by more than a rounding tolerance, with the ratio and both citations. There are only about nine such conflicts in the whole corpus (1x 1000.0000, 2x ~3.00, 1x 1.0227, 5x T&D rounding, plus a handful of 1.00x-1.02x transpositions in FY2024-25) - small enough to ship as a curated exception table rather than a heuristic.
- Add a `break` flag distinct from `conflict`, for documented definition changes: IESCO SAIFI/SAIDI at FY2021-22, MEPCO SAIFI at FY2021-22, the FY2022-23 with/without-LT split, the SAIFI/SAIDI target reversion to flat 13/14 at FY2020-21, GEPCO and K-Electric at FY2023-24. Suppress or annotate percent-change and CAGR across a break - a naive YoY on this series prints +75,415% for IESCO SAIDI.
- Carry the TARGET as a first-class field, not a constant. SAIFI/SAIDI targets were per-DISCO through FY2019-20 (PESCO SAIDI target 15,100.93) and flat 13/14 from FY2020-21. Any compliance calculation that hardcodes 13/14 mislabels every DISCO before FY2020-21. T&D 'Allowed in Tariff' also moves every year (PESCO 31.95 -> 27.90 -> 21.33 -> 19.26).
- Never construct a PER URL. Fetch and parse https://nepra.org.pk/publications/Performance%20Reports.php as the enumerator (the /M&E/PER/Distribution/ directory returns 403), harvest hrefs byte-for-byte including trailing spaces, and treat the listing as incomplete-and-partly-broken: one entry (PER DISCOs 2023-24.pdf) 404s. Cache the resolved URL per fiscal year.
- Fetch with HTTP Range chunking. Send a HEAD, then request 1 MB ranges, verify each chunk's byte length against the requested width before concatenating, and reconcile the assembled size against Content-Length. Never append a retry onto a partial write - that produced a 3.95 MB corrupt 'PDF' that file(1) still validated.
- No OCR dependency is needed - all seven PDFs have clean text layers (39k-83k chars, zero blank pages except one cover). Build cost is a coordinate-aware table extractor plus a per-report-year layout map, roughly 12-15 distinct cases across the 13 published DISCO PERs. Budget the effort on schema drift, not on image processing.
- Bind captions to table bodies by bbox y-coordinate, not by reading-order adjacency. FY2022-23 p.17 emits both SAIDI captions before both bodies; caption-adjacency mislabels with-LT as without-LT and vice versa.
- Separate chart text from table text before parsing. Font size works in FY2024-25 (9.0pt chart labels vs 11.0pt table cells) and bbox region works generally. Otherwise axis ticks parse as a DISCO row (FY2018-19 TABLE 1 gains 'K-Electric 0 10 20 30 40') and chart labels overwrite table values with different numbers.
- Validate every parsed number against an order-of-magnitude expectation derived from the same DISCO's neighbouring years, and reject-with-flag rather than accept. This is the only defence that catches 39.733, 62.167, 9.085 and 2.675,268 - all of which parse cleanly as floats.
- Handle both negative notations - accounting parentheses '(0.16)' and minus '-0.52' - and treat '-', 'N/A' and blank as UNKNOWN, never as zero. FY2024-25 K-Electric T&D is '-' with the real value only in footnote prose.
- Match DISCO rows by name, never by position, and skip aggregate rows ('W. Av:', 'W. AVG:', 'Av:', 'Total'). FY2024-25 Table 01 places 'W. AVG:' at row 10 and 'K-Electric *' at row 11, so positional row-10 returns the weighted average as K-Electric's loss rate.
- Ship a fixed 10-entity roster (PESCO, IESCO, GEPCO, FESCO, LESCO, MEPCO, QESCO, SEPCO, HESCO, K-Electric) for reliability metrics, but derive the roster per table for complaints (FY2014-15 adds BTPL and TESCO). If a user asks for TESCO, return UNAVAILABLE with NEPRA's own stated reason, not an empty row.
- Key records by (fiscal_year, period) so the Jul-Dec 2024 semiannual PER does not collide with FY2024-25, and so FY2023-24 can be stored as report-unavailable-values-secondhand rather than as missing.
- Report FY2023-24 honestly: the primary report is a 404 on NEPRA's own index. Its values are available only as columns inside the FY2024-25 report. Tag them `provenance=secondhand` so a user can see that GEPCO's 109x SAIDI jump has never been published in a report of its own.
- Do NOT redesign the series presentation around 'figures contradict themselves by 3x'. Verified reality: the reported SAIFI/SAIDI series is byte-stable across reports in every restated cell except one, and within-report duplicates match exactly in three of six reports. Present it as a normally-consistent series with a short, named exception list - three 1000x comma errors, two ~3.00x MEPCO cells in FY2024-25, one FY2022-23 definitional split, one 2 pp recovery revision, and five sub-0.2pp T&D roundings.
- Where the source itself doubts the data, pass that through. NEPRA writes 'IESCO is continuously misreporting the data... imposed a penalty of Rs. 04 Million', 'the data submitted by DISCOs is highly objectionable as the ground situation is totally opposite', and 'there is no computerized data base mechanism in the distribution companies'. A `regulator_caveat` field quoting the report beats any confidence score the CLI could invent.
- Store source citations at page-and-table granularity (report FY, table label as printed, PDF page, document page) so every number the CLI prints can be reproduced by a human opening the PDF. This corpus has too many near-duplicate tables for a report-level citation to be checkable.

### TRAPS
- TRAILING SPACE IS REAL AND FATAL: .../NEPRA%20PER%202021%20Distribution%20Companies%20.pdf is 200; drop the %20 and it is 404. Never .strip() a filename harvested from the index page.
- --max-time 30 CANNOT COMPLETE A COLD FETCH. A cf-cache-status MISS delivers ~26 KB/s, so the 3.02 MB FY2018-19 file dies at 791,592 bytes. Use HTTP Range chunking (Accept-Ranges: bytes is set); each chunk finishes in ~0.23 s once cached.
- APPEND-ON-RETRY CORRUPTS THE FILE SILENTLY. My first chunked fetcher appended a retried range on top of the partial bytes already written, producing a 3,952,485-byte 'PDF' from a 3,024,893-byte source. file(1) still said 'PDF document, version 1.7'. Always download a chunk to a temp file, verify its length equals the requested range width, THEN concatenate - and reconcile the final size against Content-Length.
- file(1) REPORTS '1 pages' FOR SIX OF SEVEN PDFs. It misreads linearized /Count. Never use file(1) for page count; use a real PDF library.
- THE SAME METRIC IS PRINTED TWICE IN EVERY REPORT - a headline section-2 table for the report's own FY, and a section-3 five-year comparison table whose last column repeats it. For MEPCO FY2024-25 they differ by 3x. Picking the wrong one is a 300% error, not a rounding error.
- SECTION HEADINGS CARRY THE WRONG FISCAL YEAR. FY2019-20 says 'Comparison of Data for the year 2018-19...' and FY2020-21 says 'COMPARISION OF DATA FOR FY2019-20...' (also note the misspelling 'COMPARISION'). Key columns off the in-table year headers, never the heading.
- TABLE NUMBERS ARE NOT STABLE ACROSS YEARS. SAIFI is TABLE 5 / TABLE 5 / Table 5 / Table 14 / Table 05+06 / Table 05 across the six years, and its comparison twin is TABLE 17 / TABLE 15 / Table 15 / Table 24 / Table 19 / Table 17. Any rule keyed on 'Table 5' silently returns a different metric in a different year.
- FY2022-23 PUBLISHES FOUR RELIABILITY TABLES, NOT TWO: SAIFI/SAIDI each 'with LT interruptions' and 'without LT interruptions'. Both carry the same target and the same Breach wording, and the TOC calls them both just 'SAIFI'/'SAIDI'. Its own comparison tables use the WITH-LT series, so the with-LT variant is the one that joins the time series.
- CAPTIONS AND BODIES INTERLEAVE IN READING ORDER. On FY2022-23 p.17 both table captions are emitted before both table bodies, so caption-adjacency assigns 'without LT' data to the 'with LT' table. You must use bbox y-coordinates to bind a caption to a body.
- CHART DATA LABELS ARE IN THE TEXT LAYER AND DISAGREE WITH THE TABLES. FY2024-25 has four different value sets for LESCO/IESCO/GEPCO/K-Electric SAIFI across Table 05, Table 17, Figure 08 and Figure 20. Filter chart text by font size (9.0pt for chart labels vs 11.0pt for table cells in FY2024-25) or by bbox region.
- CHART Y-AXIS TICKS PARSE AS A TABLE ROW. FY2018-19 TABLE 1 acquires an eleventh row 'K-Electric 0 10 20 30 40' from Figure 2's axis. Any DISCO-name-anchored row parser will accept it.
- COMMA-TO-PERIOD CORRUPTION IS A RECURRING 1000x LANDMINE, and it PROPAGATES. MEPCO SAIDI 39733 -> 39.733 survives into three later reports. Also 'IESCO 62.167' (=62,167) and 'SEPCO 9.085' (=9,085). Detect it with an order-of-magnitude check against the DISCO's own neighbouring years, not by trusting the newest report.
- SOME CELLS ARE NOT VALID NUMBERS AT ALL: '2.675,268' (K-Electric complaints 2016-17, real value 2675268) and '2,317947' (K-Electric consumers, missing a comma). A permissive float parser will either throw or, worse, return 2.675.
- NEGATIVE VALUES CHANGE NOTATION MID-SERIES: accounting parentheses '(0.16)' and '(448.490)' in FY2018-19, minus signs '-0.52' from FY2019-20. A parser that only handles one form drops or sign-flips the good performers (GEPCO, FESCO, K-Electric).
- THE BREACH COLUMN STOPS BEING NUMERIC AFTER FY2019-20. It becomes 'Far Away' / 'Near to Limit' / 'Within Limit', with 'Away' added in FY2024-25. A numeric-only extractor silently produces a 3-column table where the source has 4.
- W. AVG / Av / Total ROWS SIT INSIDE THE DISCO BLOCK AND SOMETIMES BEFORE THE LAST DISCO. FY2024-25 Table 01 order is ...HESCO, 'W. AVG:', 'K-Electric *'. Positional row-10 = K-Electric returns the weighted average instead.
- A DISCO CAN BE PRESENT AS A ROW WITH NO DATA. FY2024-25 Table 01 has 'K-Electric *  -  -  -' and puts its real numbers (14.73% vs 14.27%) only in the footnote paragraph. Treat '-' as UNKNOWN, and know that a value may exist only in prose.
- FY2014-15 HAS NO SAIFI/SAIDI TABLE - the values live in Excel chart data labels, TRUNCATED mid-number by column width ('19,535.', '27,946.', '15,677.6'). Reading those as floats silently divides by 10 or 100.
- FY2014-15's CHART ROWS HAVE SILENT MISSING CELLS: 9 values for 10 named series in the 2010-11 and 2011-12 recovery rows. Positional zip() misassigns every DISCO after the gap and produces plausible-looking wrong numbers.
- THE ENTITY ROSTER CHANGES WITHIN A SINGLE REPORT. FY2014-15's complaints table has 12 entities (adds BTPL and TESCO); its SAIFI/SAIDI charts have 10. Do not derive the roster once per document.
- TESCO IS ABSENT FROM THE RELIABILITY SERIES ENTIRELY (0 mentions in five of seven reports) even though it is a distribution licensee. A CLI that promises TESCO must return UNAVAILABLE with NEPRA's stated reason, not a null row.
- THE OFFICIAL INDEX CONTAINS A 404. PER DISCOs 2023-24.pdf is listed on publications/Performance Reports.php but does not exist. Enumerating the index and trusting every href produces a phantom year.
- FILENAMES CARRY UPSTREAM TYPOS THAT ARE PART OF THE URL: '(FFinal)', 'DSICOs', 'updated', 'final', plus the trailing space. Never construct a PER URL; always harvest it from publications/Performance Reports.php - and remember that index is the only enumerator (the /M&E/PER/Distribution/ directory itself returns 403).
- A SEMIANNUAL REPORT SHARES THE ANNUAL DIRECTORY: 'Performance Evaluation Report - Distribution Companies Jul-Dec 2024.pdf' (200, 4,893,527 B). Keying on fiscal year alone collides it with FY2024-25.
- AN OUTLIER CAN BE A DEFINITION CHANGE THAT NEPRA ITSELF FLAGS. IESCO's 411x/755x jump and MEPCO's 10.7x drop are documented as recalculation, not performance ('previously it had miscalculated the SAIFI', 'excluded all type of planned outages'). Percent-change on this series without a break flag produces nonsense.
- CROSS-REPORT REVISIONS ARE SILENT. IESCO recovery 2018-19 is 88.0 in one report and 90 in the next three, with no footnote. The newest report is not automatically authoritative - it is also the one carrying 39.733.
- MEPCO SAIFI 10.23 vs 13 FLIPS A PASS/FAIL. Any 'compliant DISCOs' count computed from FY2024-25 Table 17 rather than Table 05 reports a first-ever SAIFI compliance that the same document denies.


########## PROBE: nepra-phase-1.5a-ecosystem-absorb-sweep ##########

# NEPRA Ecosystem Absorb Sweep — Phase 1.5a

## HEADLINE ANSWER

**No. There is no tool, package, API, MCP server, or dataset that turns NEPRA data into a queryable local dataset. Everyone hand-downloads PDFs and hand-types Excel.** Evidence below is reproducible; every number is a real observation.

The three hardest single pieces of evidence:
1. A GitHub repo literally named **`nepradataset`** contains exactly one file — a 1,357,023-byte zip holding four hand-typed 10-to-14-row spreadsheets (one misspelled `Industerial`) and a 1,334,920-byte `NEPRA 2026.PDF`.
2. **PyPI: 0 packages. npm: 0 packages.** Meanwhile `psx-data-reader` 0.0.6 ("Pakistan Stock Exchange's Data Downloader") exists — so the "Pakistani public data → installable package" pattern exists next door, and NEPRA has nothing.
3. The flagship Pakistani electricity data publication (Renewables First, *Pakistan Electricity Review 2025*, 4,207,578 B, 50 pages) footers **every page** with "Data Source: NEPRA State of Industry Report, RF Calculations" and contains **zero** occurrences of `methodolog`, `Excel`, `PDF`, `csv`, `scrap`, `manual`, or `open data`. It is an InDesign PDF with no data appendix.

---

## A. ABSORB MANIFEST — GitHub (Pakistani power / NEPRA)

Every repo below is 0–2 stars. **Not one fetches NEPRA data at runtime.** Every tariff figure in the ecosystem is a hardcoded constant.

### A1. Muhammad-Sohair/bijlicheck — the strongest consumer tool
`https://github.com/Muhammad-Sohair/bijlicheck` · TypeScript · **0 stars** · no license · 94 KB · created 2026-08-14, pushed 2026-08-16 · live at bijlicheck.vercel.app

Features (each a row to match and beat):
- Photograph-a-bill intake; field extraction via Gemini 2.5 Flash over raw REST (`lib/vision.ts`, 8,529 B)
- Recomputes **every bill head** from notified NEPRA tariff (`lib/audit.ts`, 20,878 B)
- Verdict taxonomy: verified / minor / overbilled / serious / **undercharged**
- Itemised findings: charged value, correct value, rupee delta, **the rule it violates**
- Recompute ledger: every head as-billed vs per-tariff vs Δ
- Forward-exposure projection when a bill costs Protected status (6 billing cycles)
- Auto-generates a filled-in NEPRA Consumer Affairs complaint (`components/Complaint.tsx`)
- Separates `recoverable` (provable tariff breach) from `disputed` (DISCO must justify)
- Public audit ledger (`/ledger`, `/api/ledger`, `/api/stats`) on Neon Postgres, raw SQL
- Degrades fully to seed data with no `GEMINI_API_KEY` and no `DATABASE_URL`
- **Explicit refusal paths** — K-Electric, Time-of-Use, and readings that can't account for billed units

Data layer: **hardcoded.** `lib/tariff.ts` (10,222 B) holds `TARIFF_SOURCE`, `RATES`, `PROTECTED_SLABS`, `UNPROTECTED_SLABS`. Scope is A-1 single-rate uniform only, 10 XWDISCOs. README: *"correct them in that one file."*

Its own competitive survey, verbatim — this is a market map written by a competitor:
> "Pakistan has roughly 40 million electricity connections and no way for a household to check whether its bill is correct. Every existing tool — checkbills.pk, billcalculator.com.pk, the DISCO apps — either _fetches_ your bill or _forward-estimates_ one. None of them audit a bill you have already received."

Its own stated roadmap gaps (free product spec): DISCO reference-number lookup to pull the last 12 bills; direct filing into the NEPRA CAD portal; K-Electric + ToU tariff module; aggregate enforcement identifying "which DISCO, which subdivision, which month is systematically overbilling."

### A2. atifjan2019/ebillpakistan.pk — best-in-class *provenance governance* (the one feature to steal outright)
`https://github.com/atifjan2019/ebillpakistan.pk` · JavaScript · 0 stars · 1,488 KB · pushed 2026-08-17

- `lib/tariffs.js` (17,395 B), `lib/billAnalysis.js` (11,203 B), `lib/pitc.js` (13,929 B — PITC bill fetch), `lib/discoContent.js` (102,931 B), `lib/articles.js` (107,051 B), `lib/companies.js`, `lib/discos.js`, `lib/sampleBill.js`, `lib/verify.js`
- `scripts/tariff-staleness.mjs`, `scripts/verify-report.mjs`, `scripts/dump-bill-schema.mjs`, `scripts/audit-kv-posts.mjs`
- **Staleness as a build signal**: `LAST_VERIFIED_AGAINST_SOURCE = "2026-08-16"`, `STALE_AFTER_DAYS = 90`, `tariffStaleness()`; the CI script exits 1 when stale; per-SRO `ADJUSTMENTS` carry `appliesFrom`/`appliesTo` expiry; every rate surface renders a visible staleness notice so "a missed run cannot silently ship a stale rate"
- Full provenance objects: `NEPRA_SOURCE` (name, url, htmlMirror, notifiedOn, effectiveFrom, supersedes), `SRO_BY_DISCO` (11 DISCOs → SRO numbers, 52(I)/2026 noted as K-Electric), `CATEGORY_SOURCE` with a `stillInForce` pointer

### A3. farhanshahlabs/ha_wapda_peak_hours — the only packaged integration
Python · 0 stars · **MIT** · 9 KB · pushed 2026-06-05 · HACS custom repository

- Config-flow DISCO dropdown, 10 DISCOs (IESCO, GEPCO, LESCO, MEPCO, PESCO, HESCO, QESCO, SEPCO, KE, TESCO)
- **7 entities per DISCO**: `binary_sensor.<disco>_is_peak_hour`, `_is_off_peak`, `sensor.<disco>_tariff_period`, `_time_until_peak_ends`, `_time_until_peak_starts`, `_peak_start_today`, `_peak_end_today`
- 30-second refresh; seasonal peak-window table (3 seasonal bands for IESCO/GEPCO/LESCO, 2 for MEPCO/PESCO/HESCO/QESCO/SEPCO/KE, flat for TESCO); automation/notification recipes
- Data layer: **"100% offline"**, *"all schedules are hardcoded per NEPRA/DISCO published data"* — `const.py` is 2,008 B

### A4. balochasif2021-arch/nepradataset — THE product thesis in one artifact
`https://github.com/balochasif2021-arch/nepradataset` · no language · no license · 1,326 KB · pushed 2026-05-12

Root tree: **one file**, `Dataset_NEPRA.zip`. I downloaded it (HTTP 200, 1,357,023 B) and listed it:

| Length | Name |
|---|---|
| 11,048 | `Commercial(1).xlsx` |
| 11,050 | `Commercial(2).xlsx` |
| 11,382 | `Industerial(1).xlsx` *(sic)* |
| 11,382 | `Industerial(2).xlsx` *(sic)* |
| 1,334,920 | `NEPRA 2026.PDF` |

The spreadsheets are hand-made: `dimension A1:M10` (Commercial) and `A1:M14` (Industrial), 22 and 25 shared strings, and each carries `xl/printerSettings/printerSettings1.bin` — i.e. saved from desktop Excel by a human. Actual column headers: `Variable Charges (Rs/kWh)`, `NEPRA Determined Tariff`, then `PESCO, HESCO, GEPCO, MEPCO, FESCO, LESCO, IESCO, SEPCO, TESCO, HAZECO, QESCO`, `Uniform National NEPRA Determined Tariff with PYA`. Row labels: `Regular`, `Time of Use (ToU)- On-Peak`, `Time of Use (ToU)- Off-Peak`, `Temporary Supply`, `Electric Vehicle Charging Stations (EVCS)`; industrial: `B1`, `B1 Off-Peak`, `B1 On-Peak`, `B2`, `B2/B3/B4 ToU On/Off-Peak`, `Temporary Supply`.

**A repo named "nepradataset" is 10 rows of hand-typed tariff and a PDF.**

### A5. Noor-Rehman/VoltaIQ — the only real ETL, and it isn't NEPRA
Python · 0 stars · 196,647 KB. `data/parser/iesco_parser.py` (8,265 B) parses **IESCO** load-shedding schedules. Raw inputs are hand-obtained: `islamabad.xlsx` (24,352 B), `rawalpindi.xlsx` (34,887 B). Processed: `iesco_islamabad.csv` (1,605,115 B), `iesco_rawalpindi.csv` (2,113,787 B), `iesco_schedules.csv` (3,718,781 B), `iesco_schedule_with_weather_final.csv` (2,714,429 B). FastAPI backend (`routers/feeders.py`, `predict.py`, `schedule.py`), `outage_regressor.pkl` (213,507 B). `ml_model/train_model.py` only *comments* a NEPRA SIR URL — training data is IESCO + weather.

### A6. Others (exhaustive, all NEPRA-as-citation-only)
- **AE-Usama/NEPRA-Prosumer-Bill-Calculator-2025** — TS, 0★, 20 KB. Net-billing simulation under NEPRA 2025 prosumer regs, real-time slab calc, old-net-metering vs new-net-billing comparison. `constants.ts` = 1,284 B hardcoded.
- **harisahmadkhan/Solar-NetBilling-model-** — TS, MIT, 0★. Net-billing model, bilingual en/ur glossary, cites SRO 251(I)/2026 and NAEPP "Rs 8.13/unit CY 2026"; UI says *"Verify annually at nepra.org.pk."*
- **jotilohana21/PowerCast** — HTML, 1★, pushed 2026-09-06. **Claim refuted** (see §F).
- **SadiaJaved-iqbal/csm2021** — Python, 0★. Chatbot over NEPRA Consumer Service Manual 2021.
- **HassanNawaz14/Sahulat-Web** — Python, 0★. Multi-utility dashboard; docs: *"Tariff authority: NEPRA (nepra.org.pk) — updated quarterly."*
- **zeeshuu13/checkbillsonline** — TS, 0★, 1,077 KB. 30-country bill checker; NEPRA appears only as an SEO glossary entry + `llms.txt` line.
- **musakhan123/BillAgent** — JS, 0★. Emails complaints, CCs `complaint@nepra.org.pk`.
- **fisforfaheem/Darkhwast-AI** — Dart, 0★. Gov-doc → "HAQ Score"; NEPRA portal explicitly `"complaints.nepra.org.pk (mock)"`.
- **umairBANDESHA/SolarMate** — Dart. Static links to NEPRA net-metering / feed-in-tariff pages.
- **KashishTheCoder/Sepco** — JS. Links `nepra.org.pk/tariff/Distribution SEPCO.php` as "Tariff Guide".
- **tahashk7222/solar-pakistan-rag-chatbot** — TS, 0★, 7,743 KB. RAG; `sources.md` lists NEPRA Legal.php as a manual source row dated "August 2026".
- **TheCyberThesis/salar-ai**, **Qandeel-01/Black-Sky-Sentinel** (2★), **Fahad2129/bayesian-network-load-shedding-pakistan**, **AdnanSattar/UrbanWorldModel** — NEPRA as a reference URL / bibliography entry only.

---

## B. THE GLOBAL CURATION TIER — Pakistan is represented by PDFs only

- **open-energy-transition/MapYourGrid** — 99★, 18 forks, CC-BY-4.0. `docs/global-grid-data.md` = 121,761 B. Pakistan gets **exactly 2 entries, both PDFs** (one a NEPRA-hosted NTDC investment plan).
- **ben10dynartio/gridinspector** — `crosscheck_data_sources/README.md` = 77,528 B. Pakistan gets **4 entries, all PDFs, 3 of them NEPRA**, each tagged `(report)`.

## C. THE COMPARATIVE GRADE — Pakistan scores "publications"

**rradofina/adb-research-reporting**, `data-access-audit.md` (76,725 B), line 741 verbatim:

```
| PAK | NEPRA + NTDC | `nepra.org.pk` + `ntdc.gov.pk` | A: publications | Monthly + annual |
```

Its peers on the same table:
- **IND**: `A: national + regional + state; GIS-enabled; API via data.gov.in`
- **PHL**: `A: 2003-2024 power statistics per grid (Luzon/Visayas/Mindanao), per-technology, per-sector`
- **BGD**: `A: daily reports (researcher-scraped dataset 2019-2024 with 1867 daily reports published via ScienceDirect)`

Pakistan's entire access description is the single word **"publications."** Bangladesh's answer was for a researcher to scrape 1,867 daily reports and publish the dataset through ScienceDirect. **Pakistan has no equivalent.**

## D. "API: None public" — stated outright

**AdnanSattar/UrbanWorldModel**, `docs/ENERGY_DATA_SOURCES.md` (2,964 B), verbatim:
- NEPRA — *"Data: Annual reports (generation mix, tariffs), occasional dashboards. **API: None public; may require scraping or official request.** Notes: Good for historical aggregates; not real-time"*
- NTDC — *"Data: System demand, generation, transmission; daily load curves (PDF). **API: None public**; may require scraping or data sharing agreement"*
- CPPA-G — *"Data: Market settlement stats, generation mix snapshots. **API: None public**"*
- Strategy — *"Medium-term: Scrape NTDC daily load/generation PDFs to build daily curves."*

---

## E. REGISTRIES AND DATASET HOSTS — hard zeros

| Surface | Query | Result |
|---|---|---|
| **PyPI** (full simple index, 45,845,070 B downloaded) | `nepra` | **0 packages.** Literal "nepra" occurs 2× in 45 MB, neither in a package name |
| **npm** search | `nepra` | `total: 0` |
| **npm** exact-name | nepra / nepra-cli / nepra-api / pakistan-electricity / pk-power | **404, 404, 404, 404, 404** |
| **HDX** CKAN | `nepra` | `count: 0` |
| **World Bank energydata.info** | `nepra` | `count: 0` (while `fq=PAK` → **64** datasets, none NEPRA-sourced) |
| **Open Data Pakistan** | `nepra` | `count: 0`; `tariff` → `count: 2`, both **Pew political surveys** |
| **data.gov.pk** | root | **HTTP 404**, 315 B, `Server: openresty`, body: *"HTTP Error 404. The requested resource is not found."* `www.data.gov.pk` → connection refused |
| **MCP servers** (gh repo search ×3, npm ×3) | pakistan energy / nepra mcp | **none exist** |

PyPI name-collision false positives, all unrelated: `kelectric` (Spanish-language copper-conductor sizing lib by jacometoss — *not* K-Electric Pakistan), `mepcocalc` ("mini testing calculator" — *not* MEPCO), `pypep-pepco` (Pasargad Iranian payment gateway SDK — *not* PEPCO).

**Kaggle** — 40 unique datasets across 3 queries; exactly **2** contain "nepra", both undocumented dumps:
- `saddamhussain90/nepra-gc` — title "nepra_gc", **105,309,295 B**, MIT, usability **0.25**, no description, no subtitle, no tags, 17 downloads, 3 views, v1, updated 2026-07-24
- `zainmushtaq/nepradata` — title "nepradaTA", 427,230 B, license **Unknown**, usability **0.0**, no description, no tags, 2 downloads, 8 views, updated 2025-09-24

**Adjacent precedents that prove the gap:** `psx-data-reader` 0.0.6 — "Pakistan Stock Exchange's Data Downloader" (exists). `@braynexservices/nigeria-mcp-electricity` 0.2.6 — "Nigeria Electricity Meter MCP — READ-ONLY meter validation (customer name/address + DisCo)" (exists, for **Nigeria**). `utility-bill-scraper` 0.10.3 — exists but Canada-only (7 "Canada" hits, **0** "Pakistan", **0** "NEPRA" in its 8,422-char description).

---

## F. THE SERIOUS COMPETITOR: Renewables First / PECI

**peci.renewablesfirst.org** — HTTP 200, 45,012 B, `<title>Pakistan Energy & Climate Insights</title>`. The only credible NEPRA-derived analytics product.

Routes: `/power-sector`, `/power-sector-transmission`, `/power-sector-distribution`, `/power-sector-case-studies`, `/energy-supplies`, `/power-market-watch`, `/solar-tracker`, `/climate`, plus `/final_24 sankey_RF_website 1.html`, "Solar Analytics Portal", "Financial Insights".

Headline FY24 figures it renders: Installed Capacity **46.2 GW**, Generation **137 TWh**, Transformation Capacity 500 kV **25,950 MVA** / 220 kV **38,460 MVA**, Electricity Sold **110 TWh**, Peak Demand **30.15 GW**.

Stack: Next.js + **Strapi** CMS on Bitnami with Cloudflare R2 (leaked path string `/home/bitnami/renewablesfirst-strapi/src/providers/strapi-provider-cloudflare-r2`). Charts are **Tableau Public embeds** (`public.tableau.com/javascripts/api/viz_v1.js`, `tableauViz`, `tableauPlaceholder`).

**It has no API and no export.** `/api`, `/api/power-sector`, `renewablesfirst.org/api/publications` → all **HTTP 404**. Zero occurrences of `.json`, `.csv`, `.xlsx`, `download`, or `export` in the page HTML. `renewablesfirst.org/data/`, `/tools/`, `/research/` → all HTTP 404.

Their own program claim: *"The program has compiled **20 years** of previously fragmented energy data into accessible **dashboards** through Pakistan Energy and Climate Insights, giving stakeholders their first comprehensive view of how the system actually works."*

**Their statement of the problem is our product thesis, written by the incumbent:**
> "Energy sector decisions are made with incomplete information. Critical data about power generation, grid performance, and market trends sits scattered across different agencies, often outdated or inaccessible to those who need it most. Policymakers and researchers work with fragmented information, and the public remains uninformed about the infrastructure their lives depend on. This data vacuum perpetuates poor decision-making and ensures the same mistakes keep happening."

Also `/data-toolkits`: *"Levelized Cost of Energy (LCOE) Calculator ... the first in our series of analytical"* tools.

**Pakistan Electricity Review 2025** — `uploads.renewablesfirst.org/Pakistan_Electricity_Review_2025_89f0b613d6.pdf`, HTTP 200, **4,207,578 B, 50 pages**, producer "Adobe InDesign 20.0 (Windows)", created 2025-09-04. 41 NEPRA mentions, 40 "State of Industry" mentions, and **every page footer** reads "Data Source: NEPRA State of Industry Report, RF Calculations". Hits for `methodolog`=0, `Excel`=0, `PDF`=0, `scrap`=0, `manual`=0, `csv`=0, `open data`=0. Credits *"Herald Analytics for partnering with us in the collation of data and insights."* PER 2026 also exists.

**Verdict: the best-funded NEPRA data operation in Pakistan ships a 50-page InDesign PDF and Tableau embeds, with no machine-readable release.**

Other think tanks — **blocked, not proven absent**: PIDE `pide.org.pk` HTTP 403 (4,831 B) and `/research/` 403; SDPI `sdpi.org` HTTP 200 but a 845-byte **Incapsula** block page ("Request unsuccessful. Incapsula incident ID: 526000440219857450-…"); IEEFA `ieefa.org` and `/region/pakistan` both HTTP 403.

## G. IEA — the one working machine-readable Pakistan power API

`https://api.iea.org/stats/indicator/ElecGenByFuel?countries=PAKISTAN` → **HTTP 200, 44,893 B, `application/json`**.
- **204 rows**, years **1990–2023** (34 years), units `GWh`
- Keys: `year, short, flow, value, flowLabel, flowOrder, units, product, productLabel, productOrder, seriesLabel, country`
- Row 0: `{'year':'1990','short':'PAKISTAN','flow':'EHCOAL','value':38,'flowLabel':'Coal','units':'GWh','product':'ELECTR','country':'PAK'}`

Ceiling: annual, national, generation-by-fuel only. **No plant, no DISCO, no tariff, no T&D losses, no recovery, and it stops at 2023.** (`iea.org/countries/pakistan` HTML is 403.)

---

## H. NEPRA'S OWN SURFACES — corrections to the brief, plus two new enumerators

### H1. CORRECTION — the path token is `2021-22`, NOT `FY2021-22`
The brief's `<FY>` placeholder must be expanded **without** the "FY" prefix. Reproduced both ways:
- `.../List%20of%20Companies%20Genenration%20wise%20FY2021-22_files/sheet001.htm` → **HTTP 404, 9 B**
- `.../List%20of%20Companies%20Genenration%20wise%202021-22_files/sheet001.htm` → **HTTP 200, 516,219 B** ← exactly the brief's number

With the correct token, all seven of the brief's byte counts reproduce **exactly**:

| Year | Status | Bytes |
|---|---|---|
| 2015-16 | 404 | 9 |
| 2016-17 | 404 | 9 |
| 2017-18 | 200 | **426,282** ✓ |
| 2018-19 | 200 | **418,887** ✓ |
| 2019-20 | 200 | **421,591** ✓ |
| 2020-21 | 200 | **455,640** ✓ |
| 2021-22 | 200 | **516,219** ✓ |
| 2022-23 | 200 | **490,461** ✓ |
| 2023-24 | 200 | **493,187** ✓ |
| 2024-25 | 404 | 9 |
| 2025-26 | 404 | 9 |

### H2. CORRECTION — `SIR Data 2024/2025.htm` and `Main.htm` did NOT reproduce
At base `https://nepra.org.pk/publications/State%20of%20Industry%20Reports/`:
- `SIR%20Data%202024.htm` → **HTTP 404, 9 B**
- `SIR%20Data%202025.htm` → **HTTP 404, 9 B**
- `Quarterly%20Data%20(XWD%20&%20KE).htm` → **HTTP 404, 9 B**
- `Main.htm` → **HTTP 404, 9 B**

The brief records SIR Data 2024/2025 as 200 and treats Main.htm as the site index. Neither reproduced at this base path. Either the brief used a different base, or the site changed. **Flagged, not assumed** — do not hardcode these until the true base is recovered. (Note the 404 body is 9 bytes served as `charset=iso-8859-1` — the server's default encoding is the origin of the trap.)

### H3. NEW — two reliable per-workbook enumerators (better than any index page)
Every `..._files/` directory carries the Excel manifest:

`filelist.xml` — **HTTP 200, 270 B**, verbatim:
```xml
<xml xmlns:o="urn:schemas-microsoft-com:office:office">
 <o:MainFile HRef="../List%20of%20Companies%20Genenration%20wise%202023-24.htm"/>
 <o:File HRef="stylesheet.css"/>
 <o:File HRef="tabstrip.htm"/>
 <o:File HRef="sheet001.htm"/>
 <o:File HRef="filelist.xml"/>
</xml>
```
`tabstrip.htm` — **HTTP 200, 823 B** — declares `charset=windows-1252`, `Generator content="Microsoft Excel 15"`, and names the sheets: a single tab labelled `2023-24` linking `sheet001.htm`.

Also verified: `stylesheet.css` 200 / 17,015 B; `sheet002.htm` and `sheet003.htm` → **404** (single-sheet workbooks). The parent workbook `List%20of%20Companies%20Genenration%20wise%202023-24.htm` → **HTTP 200 but only 9,838 B, and byte-identical 9,838 B for 2017-18 and 2020-21** — a boilerplate `<frameset rows="*,18">` shell with frames `frSheet`/`frScroll`/`frTabs`, **not data**. Directory listing `/Detail%20of%20Generation/` → **HTTP 403, 9 B** — no autoindex, so enumeration must be URL construction + `filelist.xml`, never a directory crawl.

### H4. ENCODING TRAP — reproduced exactly, with the offending byte located
On the real `2023-24` file (493,187 B):
- `file` → `HTML document text, ISO-8859 text`; declared `charset=windows-1252`
- **Byte-safe** (`LC_ALL=C grep -a`): `<table>=1`, `<tr>=139`, `<td>=4965` ← exactly the brief's numbers
- **Naive UTF-8** (`grep`): `<table>=0`, `<tr>=0`, `<td>=0` ← the trap, confirmed
- First invalid UTF-8 byte: **`\xa0` at offset 7076**, context `width:1520pt'><span style='mso-spacerun:yes'>\xa0</span>FY\n  2023-24</td>` — a cp1252 non-breaking space
- `cp1252` decode → 139 `<tr`, 4,965 `<td` (matches byte-safe)

### H5. NEW — NEPRA ships a Power BI dashboard (and it is not plain-HTTP reachable)
`/publications/State%20of%20Industry%20Reports/Detail%20of%20Report%202024/Detail%20Report%202024.php` → **HTTP 200, 41,623 B**, and it embeds:
`https://app.powerbi.com/view?r=eyJrIjoiZjcwMWExYWItMzNjMi00ZGJjLWFiZjYtYjM4NTYyMzI2YjkyIiwidCI6IjdjMjY4YTdhLTZiZWEtNGUwOS1iZjM1LWZjNDFlMGY5ODU3YiIsImMiOjl9`

Decoded: `{"k":"f701a1ab-33c2-4dbc-abf6-b38562326b92","t":"7c268a7a-6bea-4e09-bf35-fc41e0f9857b","c":9}`. Backend from the embed shell: `https://wabi-west-europe-f-primary-redirect.analysis.windows.net/`, base `/public/reports/`, endpoints `/conceptualschema` and `/modelsAndExploration?preferReadOnlySession=true`.

Probed with `X-PowerBI-ResourceKey` + Origin/Referer headers, 2 attempts each:
- `/public/reports/{rk}` → **curl 56 "Empty reply from server"** ×2
- `/public/reports/{rk}/modelsAndExploration` → **curl 56 "Empty reply from server"** ×2
- `/explore/reports/{rk}/modelsAndExploration` → **HTTP 403, 0 B** ×2

**Not retrievable under the no-browser constraint.** Whether it is queryable with a real session is UNVERIFIED.

That same "Detail Report 2024" page has **0 `<table>`, 0 `<tr>`, 0 `<td>`** and links only `State of Industry Report 2022.pdf` and `State of Industry Report 2023.pdf` — NEPRA's own *2024* detail index links the *2022 and 2023* PDFs. It also exposes the complaint endpoints `/CAD-Database/CMS-CAD/cregister.php` and `/CAD-Database/CMS-CAD/tcomplaint.php`.

---

## I. PAIN POINTS — verified complaints

**1. The Federal Minister for Power says the regulator's own data is wrong and late.** Awais Leghari, The Nation, 19 Jan 2026 — the State of Industry Report *"fails to present the true picture of Pakistan's power sector as it is based on **incomplete and inaccurate data**"*; *"the SOI Report should have been released in **August 2025**, but after delay in release it did not accurately reflect the improvements made by the government"*; *"regulator's data on **meter reading and billing was also insufficient**."* → a five-month publication delay and the government publicly disputing the regulator's numbers.

**2. A peer-reviewed paper admits hand-reading the SIR and eating the error.** Frontiers in Energy Research, 10.3389/fenrg.2024.1328891 (fetched, 142,658 B), verbatim:
> "As actual daily peak load data of the NPCC from 2017 to 2021 **are not available**, actual monthly peak demands are **gathered from NEPRA's State of Industry Report 2022**… **There is a chance of error** in actual peak demands collected because they are **secondary data** and are not retrieved from system-generated data of the NPCC."

and *"The number of electricity consumers **can be extracted from** NEPRA's state-of-industry report."*

**3. The single best hand-transcription evidence in the sweep** — `ebillpakistan.pk/lib/tariffs.js` comments, verbatim:
> "S.R.O. 279 and SROs 41–52 are **image-only scans**, and the XWDISCO annex of the 11-02-2026 decision **does not OCR reliably**. The rates below come from a **visual read** of Annex-B-1, cross-checked two further independent ways that agree on every one of the fifteen numbers"

> "LESCO's own tariff page was **rejected as a source**: its latest entry is 'W.E.F 26-07-2023' and is **three years stale**."

> "**WHY THIS TOOK SO LONG TO FIND**: the definitions annex only appears when a FULL Schedule of Tariff is re-notified. Quarterly and rationalisation SROs — 279(I)/2026, 1285(I)/2025, 1286(I)/2025 — are rate tables and contain **no PART-II at all**… If you are ever hunting a definition, look in a full Schedule of Tariff."

A developer visually read fifteen numbers off image-only scans and triangulated them three ways. That is the current state of the art.

**4. NTDC's website is entirely offline.** `https://ntdc.gov.pk/` → **HTTP 000, 0 bytes** on 3 attempts (initial + 2 retries, including `http://www.ntdc.gov.pk/`). The transmission company's data is unreachable, not merely unstructured.

**5. No national open data portal.** `data.gov.pk` → HTTP 404 at root.

**6. The folk workaround is re-uploading PDFs to Scribd** — multiple independent Scribd mirrors of SIR 2020 and SIR 2023 surfaced in search (e.g. `scribd.com/document/752487964`, `/706104648`, `/481802373`).

**7. Bangladesh solved this and Pakistan didn't** — per the ADB audit, BGD has a "researcher-scraped dataset 2019-2024 with 1867 daily reports published via ScienceDirect". Pakistan's row says "publications".

**Blocked, so absence is not proven:** Business Recorder's critique "Nepra: ghosts of the report past" (brecorder.com/news/40292221) → **HTTP 403** via both WebFetch and curl, body *"Just a moment... Enable JavaScript and cookies to continue"* (Cloudflare JS challenge; no browser permitted). Reddit/X threads: my searches surfaced no concrete NEPRA-data complaint threads — recorded as **not found**, not as nonexistent.

---

## J. BOTTOM LINE

Nothing in this ecosystem reads NEPRA at runtime. Every tariff number in every tool is a hardcoded constant, transcribed by a human from an image-only scan. The best-funded player ships a 50-page InDesign PDF and Tableau embeds with no API. The global grid-data community represents Pakistan with four PDF links. PyPI, npm, HDX, energydata.info, Open Data Pakistan and Kaggle contain, between them, **zero** usable NEPRA datasets — and `data.gov.pk` returns 404.

Meanwhile NEPRA itself has been quietly publishing **seven fiscal years of machine-parseable Excel-exported HTML** (426 KB–516 KB each, 139 rows × 4,965 cells for FY2023-24) that nobody has ever parsed, because a naive UTF-8 read of those files returns **zero tags and looks like an empty file**.

The moat is a one-line encoding fix that the entire ecosystem has walked past.

### VERIFIED
- NO tool exists that turns NEPRA data into a queryable local dataset. Verified across GitHub (gh repo + code search, ~15 distinct queries), PyPI (full 45,845,070-byte simple index downloaded and grepped), npm (registry search + 5 exact-name probes), HDX CKAN, World Bank energydata.info CKAN, Open Data Pakistan CKAN, Kaggle API, data.gov.pk, and 4 think tanks. Every hit is either a hardcoded-constant consumer app or a PDF citation.
- PyPI has ZERO NEPRA packages. Downloaded the complete simple index (45,845,070 B); literal 'nepra' occurs exactly 2 times in 45 MB and never as a package name. The only name-collisions are unrelated: 'kelectric' (Spanish-language copper-conductor sizing lib, jacometoss, NOT K-Electric Pakistan), 'mepcocalc' ('mini testing calculator', NOT MEPCO), 'pypep-pepco' (Pasargad Iranian payment-gateway SDK, NOT PEPCO).
- npm has ZERO NEPRA packages. Registry search text=nepra returned 'total: 0'. Exact-name probes: registry.npmjs.org/nepra -> HTTP 404, /nepra-cli -> 404, /nepra-api -> 404, /pakistan-electricity -> 404, /pk-power -> 404.
- ZERO MCP servers for Pakistani energy/power data. gh repo search for 'mcp-server pakistan', 'mcp pakistan energy', 'nepra mcp' all returned empty result sets. The only adjacent precedent on npm is @braynexservices/nigeria-mcp-electricity 0.2.6 ('Nigeria Electricity Meter MCP — READ-ONLY meter validation (customer name/address + DisCo)') — proving the category exists for Nigeria but not Pakistan.
- HDX (Humanitarian Data Exchange) has ZERO NEPRA datasets: package_search?q=nepra returned 'count: 0' (HTTP 200, 294 B).
- World Bank's own energy data portal has ZERO NEPRA datasets: energydata.info package_search?q=nepra -> 'count: 0' (HTTP 200, 216 B), while fq=vocab_country_names:PAK -> count: 64 datasets, none NEPRA-sourced (solar/wind resource maps, household panel surveys, biomass, a 2017 transmission GeoJSON, Karachi rooftop solar).
- Open Data Pakistan (opendata.com.pk) has ZERO NEPRA datasets: q=nepra -> 'count: 0'. q=tariff -> 'count: 2', both of which are Pew political surveys (march-2018-pew-political-survey, pew-research-center-july-2018).
- Pakistan's national open data portal is DOWN: https://data.gov.pk/ -> HTTP 404, 315 bytes, 'Server: openresty', body verbatim 'HTTP Error 404. The requested resource is not found.' https://www.data.gov.pk/ -> connection refused (curl exit 7, 'Failed to connect ... port 443 after 7318 ms').
- A GitHub repo literally named 'nepradataset' (balochasif2021-arch/nepradataset, no license, pushed 2026-05-12) contains exactly ONE file: Dataset_NEPRA.zip, which I downloaded (HTTP 200, 1,357,023 B) and listed. It holds 5 files: Commercial(1).xlsx 11,048 B, Commercial(2).xlsx 11,050 B, Industerial(1).xlsx 11,382 B [sic], Industerial(2).xlsx 11,382 B, and 'NEPRA 2026.PDF' 1,334,920 B. The spreadsheets are hand-made: dimensions A1:M10 and A1:M14 (10 and 14 rows), 22 and 25 shared strings, and each contains xl/printerSettings/printerSettings1.bin proving it was saved from desktop Excel by a human.
- Kaggle has only 2 NEPRA items across 40 unique datasets, both undocumented dumps: saddamhussain90/nepra-gc (title 'nepra_gc', 105,309,295 B, MIT, usability 0.25, NO description, NO subtitle, NO tags, 17 downloads, 3 views, v1, updated 2026-07-24) and zainmushtaq/nepradata (title 'nepradaTA', 427,230 B, license Unknown, usability 0.0, NO description, NO tags, 2 downloads, 8 views, updated 2025-09-24).
- Muhammad-Sohair/bijlicheck (TypeScript, 0 stars, no license, 94 KB) is the strongest consumer tool and its tariff data is HARDCODED in lib/tariff.ts (10,222 B) holding TARIFF_SOURCE, RATES, PROTECTED_SLABS, UNPROTECTED_SLABS. Its README states: 'correct them in that one file.' Features: Gemini-2.5-Flash vision bill intake, full-head recompute (lib/audit.ts 20,878 B), 5-level verdict, itemised findings with rule citation, recompute ledger, forward-exposure projection, auto-generated NEPRA complaint, recoverable-vs-disputed separation, public ledger + /api/stats, and explicit refusal paths for K-Electric/ToU.
- bijlicheck's README contains a verbatim competitor survey confirming the gap: 'Pakistan has roughly 40 million electricity connections and no way for a household to check whether its bill is correct. Every existing tool — checkbills.pk, billcalculator.com.pk, the DISCO apps — either _fetches_ your bill or _forward-estimates_ one. None of them audit a bill you have already received.'
- atifjan2019/ebillpakistan.pk ships the best-in-class provenance governance to absorb: LAST_VERIFIED_AGAINST_SOURCE = '2026-08-16', STALE_AFTER_DAYS = 90, a tariffStaleness() function, scripts/tariff-staleness.mjs that exits 1 when stale, per-SRO ADJUSTMENTS with appliesFrom/appliesTo expiry, plus NEPRA_SOURCE, SRO_BY_DISCO (11 DISCOs mapped to SRO numbers) and CATEGORY_SOURCE with a stillInForce pointer.
- ebillpakistan.pk's lib/tariffs.js comments prove humans visually transcribe image-only scans, verbatim: 'S.R.O. 279 and SROs 41-52 are image-only scans, and the XWDISCO annex of the 11-02-2026 decision does not OCR reliably. The rates below come from a visual read of Annex-B-1, cross-checked two further independent ways that agree on every one of the fifteen numbers'. Also: 'LESCO's own tariff page was rejected as a source: its latest entry is "W.E.F 26-07-2023" and is three years stale.'
- farhanshahlabs/ha_wapda_peak_hours (Python, MIT, 0 stars, 9 KB) is the only packaged integration, and it is explicitly offline: '100% offline', 'all schedules are hardcoded per NEPRA/DISCO published data' with const.py at only 2,008 B. It provides 7 entities per DISCO across 10 DISCOs with 30-second refresh.
- The global grid-data curation community represents Pakistan with PDFs only. open-energy-transition/MapYourGrid (99 stars, 18 forks, CC-BY-4.0), docs/global-grid-data.md = 121,761 B, gives Pakistan exactly 2 entries, both PDFs. ben10dynartio/gridinspector, crosscheck_data_sources/README.md = 77,528 B, gives Pakistan 4 entries, all PDFs, 3 of them NEPRA, each tagged '(report)'.
- An ADB research data-access audit grades Pakistan's power data access as the single word 'publications'. rradofina/adb-research-reporting, data-access-audit.md (76,725 B), line 741 verbatim: '| PAK | NEPRA + NTDC | `nepra.org.pk` + `ntdc.gov.pk` | A: publications | Monthly + annual |' — versus IND 'A: national + regional + state; GIS-enabled; API via data.gov.in', PHL 'A: 2003-2024 power statistics per grid (Luzon/Visayas/Mindanao), per-technology, per-sector', and BGD 'A: daily reports (researcher-scraped dataset 2019-2024 with 1867 daily reports published via ScienceDirect)'.
- 'API: None public' is stated outright in AdnanSattar/UrbanWorldModel docs/ENERGY_DATA_SOURCES.md (2,964 B): NEPRA 'API: None public; may require scraping or official request. Notes: Good for historical aggregates; not real-time'; NTDC 'API: None public; may require scraping or data sharing agreement'; CPPA-G 'API: None public'. Its strategy line: 'Medium-term: Scrape NTDC daily load/generation PDFs to build daily curves.'
- Renewables First's PECI dashboard (peci.renewablesfirst.org, HTTP 200, 45,012 B, title 'Pakistan Energy & Climate Insights') is the strongest competitor and has NO API and NO export. /api, /api/power-sector and renewablesfirst.org/api/publications all return HTTP 404. Zero occurrences of .json, .csv, .xlsx, download or export in the page HTML. It is Next.js + Strapi (leaked string '/home/bitnami/renewablesfirst-strapi/src/providers/strapi-provider-cloudflare-r2') with Tableau Public embeds (public.tableau.com/javascripts/api/viz_v1.js, tableauViz, tableauPlaceholder). Routes: /power-sector, /power-sector-transmission, /power-sector-distribution, /power-sector-case-studies, /energy-supplies, /power-market-watch, /solar-tracker, /climate.
- Renewables First states the product thesis themselves on /our-work/data-analytics, verbatim: 'Energy sector decisions are made with incomplete information. Critical data about power generation, grid performance, and market trends sits scattered across different agencies, often outdated or inaccessible to those who need it most. Policymakers and researchers work with fragmented information... This data vacuum perpetuates poor decision-making and ensures the same mistakes keep happening.' They claim to have 'compiled 20 years of previously fragmented energy data into accessible dashboards'.
- The flagship Pakistani electricity data publication is a hand-built PDF with no data release. Pakistan Electricity Review 2025 (HTTP 200, 4,207,578 B, 50 pages, producer 'Adobe InDesign 20.0 (Windows)', created 2025-09-04) footers EVERY page with 'Data Source: NEPRA State of Industry Report, RF Calculations' (41 NEPRA mentions, 40 'State of Industry' mentions) yet contains ZERO occurrences of methodolog, Excel, PDF, scrap, manual, csv, or 'open data'. It credits 'Herald Analytics for partnering with us in the collation of data and insights.'
- IEA is the ONE working machine-readable Pakistan power API: https://api.iea.org/stats/indicator/ElecGenByFuel?countries=PAKISTAN -> HTTP 200, 44,893 B, application/json, 204 rows, years 1990-2023 (34 years), units GWh, keys [year, short, flow, value, flowLabel, flowOrder, units, product, productLabel, productOrder, seriesLabel, country], row0 = {'year':'1990','flow':'EHCOAL','value':38,'flowLabel':'Coal','units':'GWh','country':'PAK'}. Ceiling: annual national generation-by-fuel only, no plant/DISCO/tariff/losses, stops at 2023.
- CORRECTION to the brief: the generation-data path token is '2021-22', NOT 'FY2021-22'. Reproduced both: '...Genenration%20wise%20FY2021-22_files/sheet001.htm' -> HTTP 404, 9 B; '...Genenration%20wise%202021-22_files/sheet001.htm' -> HTTP 200, 516,219 B, exactly matching the brief's recorded figure.
- With the corrected token all seven of the brief's byte counts reproduce EXACTLY: 2017-18=426,282; 2018-19=418,887; 2019-20=421,591; 2020-21=455,640; 2021-22=516,219; 2022-23=490,461; 2023-24=493,187. And 2015-16, 2016-17, 2024-25, 2025-26 all return HTTP 404 with a 9-byte body.
- ENCODING TRAP fully reproduced on the real 2023-24 file (493,187 B): file(1) reports 'HTML document text, ISO-8859 text', the declared charset is windows-1252; byte-safe LC_ALL=C grep -a yields <table>=1, <tr>=139, <td>=4965 (exactly the brief's numbers) while naive UTF-8 grep yields 0, 0, 0. I located the cause: the first invalid UTF-8 byte is \xa0 at offset 7076, context "width:1520pt'><span style='mso-spacerun:yes'>\xa0</span>FY\n  2023-24</td>" — a cp1252 non-breaking space. Decoding as cp1252 gives 139 <tr and 4,965 <td.
- NEW ENUMERATORS found that beat any index page: every ..._files/ directory serves filelist.xml (HTTP 200, 270 B) — the Excel manifest naming the parent workbook and all members: '<o:MainFile HRef="../List%20of%20Companies%20Genenration%20wise%202023-24.htm"/>' plus stylesheet.css, tabstrip.htm, sheet001.htm, filelist.xml — and tabstrip.htm (HTTP 200, 823 B) which declares charset=windows-1252, 'Generator content="Microsoft Excel 15"' and names the sheets (a single tab labelled '2023-24').
- The per-year parent workbook .htm is a boilerplate frameset, NOT data: '...Genenration%20wise%202023-24.htm' -> HTTP 200 but only 9,838 B, and byte-identical 9,838 B for 2017-18 and 2020-21, containing '<frameset rows="*,18">' with frames frSheet/frScroll/frTabs. sheet002.htm and sheet003.htm both 404, so these are single-sheet workbooks.
- There is NO directory autoindex: https://nepra.org.pk/publications/State%20of%20Industry%20Reports/Detail%20of%20Generation/ -> HTTP 403, 9 B. Enumeration must therefore be URL construction plus filelist.xml, never a directory crawl.
- NEW: NEPRA embeds a Power BI dashboard. /publications/State%20of%20Industry%20Reports/Detail%20of%20Report%202024/Detail%20Report%202024.php -> HTTP 200, 41,623 B, embedding app.powerbi.com/view?r=<b64> which decodes to {"k":"f701a1ab-33c2-4dbc-abf6-b38562326b92","t":"7c268a7a-6bea-4e09-bf35-fc41e0f9857b","c":9}. The embed shell reveals backend https://wabi-west-europe-f-primary-redirect.analysis.windows.net/ with base /public/reports/ and endpoints /conceptualschema and /modelsAndExploration?preferReadOnlySession=true.
- The Power BI backend is NOT reachable by plain HTTP (2 attempts each, with X-PowerBI-ResourceKey + Origin + Referer headers): /public/reports/{rk} -> curl 56 'Empty reply from server'; /public/reports/{rk}/modelsAndExploration -> curl 56 'Empty reply from server'; /explore/reports/{rk}/modelsAndExploration -> HTTP 403, 0 bytes.
- NEPRA's own 'Detail Report 2024' page contains ZERO tabular data — 0 <table>, 0 <tr>, 0 <td> byte-safe — and links only 'State of Industry Report 2022.pdf' and 'State of Industry Report 2023.pdf'. NEPRA's 2024 detail index links the 2022 and 2023 PDFs. It also exposes complaint endpoints /CAD-Database/CMS-CAD/cregister.php and /CAD-Database/CMS-CAD/tcomplaint.php.
- PAIN POINT (top-tier): Pakistan's Federal Minister for Power publicly attacked the regulator's data. Awais Leghari, The Nation, 19 Jan 2026: the SOI Report 'fails to present the true picture of Pakistan's power sector as it is based on incomplete and inaccurate data'; 'the SOI Report should have been released in August 2025, but after delay in release it did not accurately reflect the improvements made by the government'; and 'regulator's data on meter reading and billing was also insufficient.' That is a five-month publication delay plus the government disputing the regulator's numbers.
- PAIN POINT (peer-reviewed): Frontiers in Energy Research 10.3389/fenrg.2024.1328891 (fetched, 142,658 B) admits hand-reading the SIR and eating the error, verbatim: 'As actual daily peak load data of the NPCC from 2017 to 2021 are not available, actual monthly peak demands are gathered from NEPRA's State of Industry Report 2022... There is a chance of error in actual peak demands collected because they are secondary data and are not retrieved from system-generated data of the NPCC.' Also: 'The number of electricity consumers can be extracted from NEPRA's state-of-industry report.'
- PAIN POINT: NTDC's website is entirely offline, not merely unstructured. https://ntdc.gov.pk/ -> HTTP 000, 0 bytes across 3 attempts (initial plus 2 retries, including http://www.ntdc.gov.pk/).
- PAIN POINT: the folk workaround is re-uploading PDFs to Scribd. Multiple independent Scribd mirrors of SIR 2020 and SIR 2023 surfaced (scribd.com/document/752487964, /706104648, /481802373).
- Adjacent precedents prove the gap is Pakistan-power-specific, not Pakistan-wide: psx-data-reader 0.0.6 EXISTS on PyPI ('Pakistan Stock Exchange's Data Downloader'), and utility-bill-scraper 0.10.3 exists but is Canada-only (7 'Canada' hits, 0 'Pakistan', 0 'NEPRA' in its 8,422-char description).

### REFUTED/UNVERIFIED
- REFUTED — jotilohana21/PowerCast's repo description claims 'benchmarking Linear Regression vs ARIMA on 30+ years of NEPRA data'. Its own README refutes this: 'Source: World Energy Consumption dataset (Our World in Data via Kaggle)', with NEPRA used only for 'Validation: Cross-checked against NEPRA State of Industry Report 2022-23'. The repo's full tree contains ZERO data files — only PowerCast_Electricity_Forecasting.ipynb (24,378 B), README.md, index.html, script.js, style.css. Do not treat this as a NEPRA dataset.
- REFUTED (for the document I actually checked) — a search-engine summary claimed 'Coal consumption data for power sector from NEPRA is not available' in Pakistan's Economic Survey. I fetched finance.gov.pk/survey/chapter_24/14_energy.pdf (HTTP 200, 1,330,569 B, 10 pages), extracted 168,082 chars from 44 inflated streams, and found 20 'not available' occurrences — ALL of them footnotes to jet-fuel (JP-4/JP-8) price tables sourced to the Hydrocarbon Development Institute, NONE relating to NEPRA or coal. The claim is not supported by that document.
- UNVERIFIED — the widely-repeated claim that 'Data extracted from NEPRA's PDF reports are converted into Excel format using Adobe Acrobat Professional' came from a search summary attributing it to a paper. I fetched the Frontiers article it pointed at (142,658 B) and grepped byte-safely: 'acrobat' appears 0 times. The ScienceDirect papers that might contain it are paywalled. DO NOT CITE this quote — it is unsubstantiated.
- DISCREPANT vs the brief — 'SIR Data 2024.htm' and 'SIR Data 2025.htm' are recorded in the brief as both returning HTTP 200. At base https://nepra.org.pk/publications/State%20of%20Industry%20Reports/ both return HTTP 404 with a 9-byte body, as does 'Quarterly%20Data%20(XWD%20&%20KE).htm'. Either the brief used a different base path or the site changed. Do not hardcode these until the true base is recovered.
- DISCREPANT vs the brief — 'Main.htm' as a site index did not reproduce: /publications/State%20of%20Industry%20Reports/Main.htm -> HTTP 404, 9 B, and Main.htm inside the ..._files/ workbook directory also -> HTTP 404. The brief's Main.htm link inventory (FCA 2018-2022.htm, Quarterly Data, Notification/SRO Data Webpage.htm, Hydel Data.htm) could not be re-derived from any path I reached. Its role as 'the incomplete enumerator' is unconfirmed; filelist.xml is the enumerator I could actually verify.
- UNVERIFIED — the internal structure, column headers and row counts of the two Kaggle NEPRA dumps (saddamhussain90/nepra-gc at 105,309,295 B and zainmushtaq/nepradata at 427,230 B). The Kaggle file-list endpoint returned HTTP 404 (5,243 B HTML shell) because it requires authentication. Their 0.25 and 0.0 usability ratings and total absence of description/tags are verified; their contents are not.
- UNVERIFIED — whether NEPRA's Power BI dashboard is queryable via its REST backend at all. Under the no-browser constraint it refused every probe (curl 56 empty reply twice on two /public/reports paths, HTTP 403 twice on /explore/reports). It may well be queryable with a proper browser-established session; I could not establish that within the hard rules.
- BLOCKED, so absence is NOT proven — PIDE (pide.org.pk HTTP 403, 4,831 B; /research/ HTTP 403), SDPI (sdpi.org HTTP 200 but an 845-byte Incapsula block page, 'Request unsuccessful. Incapsula incident ID: 526000440219857450-350541748831652659'), IEEFA (ieefa.org HTTP 403; /region/pakistan HTTP 403), and IEA's HTML country pages (HTTP 403). I cannot rule out that these organisations publish NEPRA-derived tools or datasets behind those blocks.
- BLOCKED — Business Recorder's critical piece 'Nepra: ghosts of the report past' (brecorder.com/news/40292221) returned HTTP 403 via both WebFetch and curl, body 'Just a moment... Enable JavaScript and cookies to continue' (Cloudflare JS challenge). Its criticism of the SIR is therefore unread and uncited here.
- NOT FOUND (not proven nonexistent) — concrete Reddit/X/forum complaint threads about NEPRA data access. Multiple targeted searches surfaced no such threads. The verified pain points in this report come from a government minister, a peer-reviewed paper, developer source comments, a think tank's own program page, and an ADB data audit — not from social media. Treat 'developers complain on forums' as unsubstantiated.
- NOT ATTEMPTED — the brief's DISCO Performance Evaluation Report PDF inventory (the inconsistent /Standards/<year>/ naming, the trailing-space filename, the /M&E/PER/Distribution/ path). I did not re-fetch these; the brief's inconsistent-naming finding stands unchallenged but also unreproduced by me.
- UNVERIFIED — Renewables First's claim to have 'compiled 20 years of previously fragmented energy data'. I verified the claim text and that the dashboard renders FY24 headline figures (46.2 GW installed, 137 TWh generation, 110 TWh sold, 30.15 GW peak, 500 kV 25,950 MVA / 220 kV 38,460 MVA), but the underlying 20-year series sits inside Tableau embeds I did not extract, so its actual depth, completeness and year coverage are unconfirmed.

### BUILD IMPLICATIONS
- The product thesis is CONFIRMED and the wedge is narrow and specific: NEPRA has quietly published seven fiscal years of machine-parseable Excel-exported HTML (418 KB-517 KB each; FY2023-24 alone is 139 rows x 4,965 cells) that nobody has parsed, because a naive UTF-8 read returns zero tags and looks like an empty file. Ship 'nepra gen --year 2023-24 --format csv' as the first command; being the only tool that reads bytes correctly is the entire moat.
- Lead with the seven verified years (2017-18 through 2023-24) as a shipped, queryable local dataset. That immediately beats every competitor: PyPI 0 packages, npm 0 packages, HDX 0, energydata.info 0, Open Data Pakistan 0, Kaggle 2 undocumented dumps, and Renewables First's 50-page InDesign PDF with no data appendix.
- Absorb ebillpakistan.pk's provenance governance wholesale — it is the single best idea in the ecosystem. Every row must carry source URL, SRO/report identity, notified date, effective date, and a LAST_VERIFIED_AGAINST_SOURCE stamp; ship a `nepra verify` / staleness command that exits non-zero past a window, and surface per-adjustment expiry. Given a Federal Minister called the SIR 'incomplete and inaccurate' and five months late, provenance and vintage are correctness features, not polish.
- Use filelist.xml and tabstrip.htm as the enumerators, never an index page. filelist.xml (270 B) names the parent workbook and every member file; tabstrip.htm (823 B) names the sheets. Combine with URL construction across a candidate year range, and assert both HTTP 200 AND a minimum byte floor (~400 KB) to reject the 9-byte 404 and the 9,838-byte frameset decoy.
- Make the encoding fix a headline feature, not an implementation detail. Read bytes and decode windows-1252/cp1252 explicitly; add a regression test asserting FY2023-24 yields exactly 1 table, 139 rows and 4,965 cells. Document the \xa0-at-offset-7076 failure in the README — it is the credibility proof that we did the work nobody else did.
- Add a whitespace-normalising text layer for any Pakistani government PDF (SIR, PER DISCOs, Economic Survey). Per-character kerning makes exact-string search return false absences: 'National Electric Power Regulatory Authority' scores 0 hits naively and 3 after stripping whitespace. Normalise before matching, and never report 'not found' from a naive grep.
- Treat tariff/SRO extraction as a separate, harder track from generation tables and be honest about it. SRO 279 and 41-52 are image-only scans that do not OCR reliably; the ecosystem's best effort was a human visually reading fifteen numbers and triangulating three ways. Either ship tariffs as curated-with-citations (with a visible confidence/verification field) or defer them — do not silently OCR and present the output as data.
- Copy bijlicheck's refusal discipline into the CLI's core. It refuses K-Electric and Time-of-Use rather than mis-auditing, on the grounds that 'a wrong figure on a bill dispute is worse than no figure'. Our equivalent: never interpolate a missing year, never synthesise a DISCO row, exit with a clear UNAVAILABLE rather than a plausible number. This is also exactly the parent's hard rule 2.
- Cover the ground competitors explicitly refuse, since they have published their own gaps: K-Electric (~3.5m connections) and Time-of-Use schedules (bijlicheck refuses both), DISCO reference-number history lookup, and aggregate cross-DISCO/subdivision/month analysis. bijlicheck's 'What I'd build next' section is a free, validated roadmap.
- Ship the IEA JSON API as a cross-check source, not a substitute. 204 rows, 1990-2023, GWh by fuel gives 34 years of national context to validate our NEPRA extraction against — but it has no plant, DISCO, tariff or loss detail and stops at 2023, which is precisely the space NEPRA's own tables fill. A `--crosscheck iea` flag turns a competitor's data into our QA harness.
- Do not budget for Power BI extraction. NEPRA's dashboard (resourceKey f701a1ab-33c2-4dbc-abf6-b38562326b92, cluster wabi-west-europe-f-primary-redirect) refused every plain-HTTP probe (curl 56 twice, HTTP 403 twice). Treat it as a viewer-only surface and a documented gap; revisit only if a browser-based path is ever authorised.
- Publish to PyPI and npm early and claim the names — both `nepra` on npm and every NEPRA name on PyPI are unclaimed (404 / absent from the 45 MB index). Also ship an MCP server: zero exist for Pakistani energy, while Nigeria already has @braynexservices/nigeria-mcp-electricity. First-mover naming here is free.
- Position against Renewables First as complement-then-compete: they have the analyst credibility and 20 years of curation but no API, no export and Tableau-locked charts. Our differentiator is the machine-readable substrate under the same NEPRA source they footer on all 50 pages. A citable, versioned local dataset is the thing their own 'data vacuum' paragraph asks for.
- Target the two audiences with verified, quotable pain: researchers (a peer-reviewed paper fell back to hand-read SIR figures and documented 'a chance of error' from using secondary data) and the global grid-data community (MapYourGrid at 99 stars and gridinspector represent Pakistan with 4 PDF links total). Landing a structured Pakistan dataset into MapYourGrid is a high-leverage distribution move, and Bangladesh's 1,867-daily-report ScienceDirect dataset is the proven template for academic credibility.

### TRAPS
- ENCODING (load-bearing, confirmed): NEPRA's Excel-exported HTML is windows-1252. On the real 2023-24 file, naive UTF-8 grep returns <table>=0, <tr>=0, <td>=0 and the file looks EMPTY, while LC_ALL=C grep -a returns 1 / 139 / 4965. The first invalid UTF-8 byte is \xa0 at offset 7076 (a cp1252 non-breaking space inside <span style='mso-spacerun:yes'>). Always read bytes or decode cp1252. Note NEPRA's 404 pages are served as charset=iso-8859-1, which is why the whole site defaults this way.
- PATH TOKEN: the fiscal-year segment is '2021-22', NOT 'FY2021-22'. Prefixing 'FY' yields HTTP 404 with a 9-byte body on every year. The literal upstream typo 'Genenration' must also be preserved. Full segment: 'List of Companies Genenration wise 2021-22_files'.
- SILENT 404s: NEPRA returns HTTP 404 with a 9-byte body 'Not Found'. A pipeline that checks only for a non-empty response, or that writes whatever comes back, will happily ingest 9 bytes as a year of generation data. Assert on status AND a minimum plausible byte count (real years are 418 KB-517 KB).
- NO DIRECTORY INDEX: /Detail%20of%20Generation/ returns HTTP 403. Any crawler that expects autoindex to enumerate years will find nothing. Enumerate by constructing URLs across a candidate year range AND by reading each workbook's filelist.xml.
- FRAMESET DECOY: the per-year parent workbook '...Genenration wise 2023-24.htm' returns HTTP 200 but is only 9,838 bytes and byte-identical across 2017-18, 2020-21 and 2023-24 — a boilerplate <frameset> shell. A fetcher that grabs the 'obvious' top-level .htm per year gets three identical files containing no data and a 200 status that looks like success. The data is only in _files/sheet001.htm.
- INDEX INCOMPLETENESS IS REAL BUT WORSE THAN DOCUMENTED: NEPRA's own 'Detail Report 2024.php' page has 0 <table>/<tr>/<td> and links the State of Industry Report 2022 and 2023 PDFs — its 2024 index points at 2022/2023 documents. Never trust any NEPRA index page as an enumerator; cross-check with URL construction.
- EXCEL-EXPORT SHAPE: sheet002.htm and sheet003.htm return 404 — these are single-sheet workbooks, and tabstrip.htm confirms one tab. Do not assume multi-sheet. But DO read tabstrip.htm and filelist.xml, because they are the only machine-readable manifests NEPRA ships.
- PER-CHARACTER KERNING in Pakistani government PDFs (new trap, proven on Pakistan Economic Survey ch.14): text extracts as 'N a ti o n a l  El e c tri c   P o w e r  Re g u lato ry   A u th o rit y'. Exact-string grep for 'National Electric Power Regulatory Authority' -> 0 hits; after stripping ALL whitespace -> 3 hits. 'not available' -> 0 naive, 20 after stripping. Any keyword search over these PDFs must normalise whitespace first or it will report false absences.
- SEMANTIC TRAP - 'NEPRA data' claims are often false. jotilohana21/PowerCast advertises '30+ years of NEPRA data' but its data is Our World in Data via Kaggle, and the repo contains no data files at all. Verify every claimed NEPRA source against the actual files before treating a project as prior art or a data source.
- PACKAGE NAME COLLISIONS will produce false positives in registry sweeps: PyPI 'kelectric' is a Spanish-language copper-conductor sizing library (not K-Electric Pakistan), 'mepcocalc' is a 'mini testing calculator' (not MEPCO), 'pypep-pepco' is an Iranian Pasargad payment-gateway SDK (not PEPCO). Read the summary field, never match on name alone.
- GITHUB SEARCH TRAP - 'nepra' is a common Slavic/Balkan word stem. A bare repo search for 'nepra' returns 90 results dominated by 'nepravda', 'nepravilni glagoli', 'neprank', 'Neprazol' etc. Constrain with 'nepra pakistan' (7 results) or use code search for 'nepra.org.pk' to find real usage.
- THE SIR IS NOT AUTHORITATIVE EVEN TO ITS OWN GOVERNMENT: the Federal Minister for Power called the report 'incomplete and inaccurate' and five months late. A CLI that presents SIR figures as ground truth without carrying publication date, revision status and the dispute will mislead. Model provenance and vintage as first-class fields, the way ebillpakistan.pk does with LAST_VERIFIED_AGAINST_SOURCE and per-SRO expiry.
- TARIFF SOURCE DOCUMENTS ARE IMAGE-ONLY SCANS: SRO 279 and SROs 41-52 do not OCR reliably (per ebillpakistan.pk's source comments). Tariff extraction is a fundamentally harder problem than generation-table extraction — do not assume one pipeline covers both. Also: definitions (PART-II) appear ONLY in a full Schedule of Tariff, never in quarterly/rationalisation SROs.
- MIRROR STALENESS: DISCO websites are not a safe fallback for tariffs. LESCO's own tariff page's latest entry was 'W.E.F 26-07-2023', three years stale, and was rejected as a source by a developer who checked. IESCO's HTML tariff guide was usable. Per-DISCO mirror freshness must be verified individually, not assumed.


########## PROBE: nepra-other-surfaces-inventory ##########

## 0. Method / provenance

All fetches: `curl -sS --max-time 30` with the browser UA, no browser, no gstack. Byte-safe tag counting via `LC_ALL=C grep -a`; content decoded with Python `bytes.decode('cp1252')` (never UTF-8). Working files in `/private/tmp/claude-501/-Users-hamzaqazi-Desktop-Personal/5916c20a-9668-41e6-8ce7-5af354a69c76/scratchpad/nepra/`.

Encoding trap confirmed independently: `file main.htm` → `HTML document text, ISO-8859 text`; all four target `sheet001.htm` payloads → `ISO-8859 text`. `iconv -f WINDOWS-1252` **failed with "iconv(): Illegal byte sequence" on p19.htm** (Orders of the Authority), so `iconv` is not a safe universal pre-pass either — `cp1252` decode with `errors='replace'` in-process is the only thing that worked on every file.

`Main.htm` re-fetched: **HTTP 200, 12,210 B**. Its complete href set (byte-safe, deduped) is exactly 10 entries — the 5 generation years FY2017-18..FY2021-22, `Main_files/filelist.xml`, and the four target surfaces. **`SIR Data 2024.htm` / `SIR Data 2025.htm` are NOT linked from Main.htm** (confirming the index is incomplete in both directions).

---

## 1. The four Main.htm-linked surfaces

**All four are frameset shells, not tables.** Every one is ~9.6 KB with `<frameset>×3`, `<frame >×5`, and `<table>×2 / <tr>×3 / <td>×24` of pure chrome. Each points at a `_files/sheet001.htm` plus a `tabstrip.htm`. `filelist.xml` for each lists exactly 5 files — so **each workbook has exactly ONE sheet** (tabstrip sheet names: FCA→`FCA`, SRO→`Sheet1`, Hydel→`Hydel`, Quarterly→unnamed). No hidden second sheet to miss.

Directory-name trap: the `_files/` dir name is derived from the **shell filename**, not the link path, and the two diverge for two of the four:
- `Notification/SRO Data Webpage.htm` → `Notification/SRO Data Webpage_files/sheet001.htm`
- `Hydel 1/Hydel Data.htm` → `Hydel 1/Hydel Data_files/sheet001.htm`

### 1a. FCA (2018-2022) — **the only numeric-value HTML surface on NEPRA**

- Shell: `.../Detail%20of%20Generation/FCA%20(2018-2022).htm` → **200, 9,612 B**, ASCII, frameset.
- Payload: `.../FCA%20(2018-2022)_files/sheet001.htm` → **200, 79,843 B**, ISO-8859.
- Byte-safe tags: **1 `<table>`, 53 `<tr>`, 572 `<td>`**. Reproduced identically on a second independent fetch.
- Row anatomy: `<tr>` 0–1 spacer, 2–3 two-level header, **4–51 = 48 data rows**, 52 spacer. `cols_hist = {11: 50, 7: 1, 6: 1, 9: 1}` — every data row has 11 `<td>` (leading spacer cell + 10 real).
- **Verbatim headers**, two rows joined by `rowspan=2` / `colspan`:
  - Row 1: `Year` (rowspan=2) | `Month` (rowspan=2) | `CPPA - G` (colspan=2) | `K-Electric` (colspan=2) | `E.M.O` (colspan=4)
  - Row 2: `FCA  Requested` | `FCA  Allowed (kWh)` | `FCA  Requested` | `FCA  Allowed` | `1st Forthnight` | `Revised` | `2nd Forthnight` | `Revised`
  - Note the upstream misspelling **`Forthnight`** (for "Fortnight") and the doubled internal space in `FCA  Requested`.
- Date range **exactly Jul-2018 → Jun-2022, 48 consecutive months, zero gaps.** First data row = `2022 | Jun | 9.9095 | 9.8972 | 11.389 | 11.1023 | 01-06-22 | 06-06-22 | 16-06-22 | 22-06-22`; last = `2018 | Jul | 0.6261 | 0.3525 | 0.623 | 0.6148 | 10-07-18 | | 20-07-18 |`. Sorted newest-first.
- Numbers are Rs/kWh fuel-cost adjustments. **49 parenthesised negatives** (`(1.798)`, `(0.1930)`, `(2.5935)`…) — accounting-style, `float()` will throw or silently drop the sign.
- E.M.O columns are **dates in `dd-mm-yy`**, not numbers — and they contain their own typos: row 013 (Sep-2021) has 2nd-fortnight `16-09-22`, rows 015–019 all carry `16-0X-22` inside 2021 months. Off by a year, upstream.

### 1b. Quarterly Data (XWD & KE) — dates only, no amounts

- Shell: `.../Quarterly%20Data%20(XWD%20&%20KE).htm` → **200, 9,796 B**, ISO-8859, frameset.
- Payload: `.../Quarterly%20Data%20(XWD%20&%20KE)_files/sheet001.htm` → **200, 79,672 B**.
- Byte-safe: **1 `<table>`, 168 `<tr>`, 582 `<td>`**. Reproduced.
- Title cell verbatim: `Requested and Allowed Quarterly Adjustments of XWAPDA DISCOs &amp; K-Electric`
- **Verbatim headers** (`rowspan=2` + `colspan=2`, `<br>` inside the group labels):
  - `Quarter` (rowspan=2) | `DISCOs Name` (rowspan=2) | `Quarterly Adjustments <br> (XWAPDA DISCOs)` (colspan=2) | `Quarterly Adjustments <br> (K-Electric)` (colspan=2)
  - `Requested By Company` | `Allowed By NEPRA` | `Requested By Company` | `Allowed By NEPRA`
- **163 data rows**: `cols_hist = {7: 18, 3: 144, 5: 2, 4: 3, 2: 1}`. 16 quarter-leader rows carry 7 cells; 144 continuation rows carry only 3.
- **16 quarters, no gaps: `July 2018 - September 2018 (1st Quarter)` → `April 2022- June 2022 (4th Quarter)`.** Note the inconsistent spacing in the 2022 label (`April 2022- June 2022`, missing space before the hyphen) vs every other label.
- 10 DISCOs per quarter, fixed order: FESCO GEPCO HESCO IESCO LESCO MEPCO PESCO QESCO SEPCO TESCO. Three quarters (Oct-Dec 2020, Jul-Sep 2020, Apr-Jun 2020) insert an extra `Reconsideration Request` pseudo-DISCO row above FESCO.
- **This surface contains no tariff amounts at all** — every cell is a filing/decision date or `Not Yet Issued`. 212 `dd-mm-yyyy` cells, span `30-12-2012` … `18-08-2022`.
- **The `30-12-2012` minimum is an upstream typo**: it sits in the `Allowed By NEPRA` cell of `April 2021 - June 2021 (4th Quarter)` (row: `12-08-2021 | 30-12-2012`), i.e. NEPRA cannot have allowed a 2021 quarter in 2012. Two more: IESCO `20-05-2020` inside the Jan–Mar 2021 quarter; HESCO `05-03-2022` inside the Oct–Dec 2019 quarter. **True usable max = 18-08-2022.**

### 1c. Notification / SRO Data Webpage

- Shell: `.../Detail%20of%20Generation/Notification/SRO%20Data%20Webpage.htm` → **200, 9,636 B**, ASCII, frameset.
- Payload: `.../Notification/SRO%20Data%20Webpage_files/sheet001.htm` → **200, 40,402 B**.
- Byte-safe: **1 `<table>`, 23 `<tr>`, 245 `<td>`**. Reproduced.
- Title cell verbatim: `SRO's Data For XWDISCO'S`
- **Verbatim header** (`Year` and `SRO Date` are `rowspan=2`; a `colspan=10` row below reads `SRO Number`):
  `Year | SRO Date | FESCO | GEPCO | HESCO | IESCO | MEPCO | LESCO | PESCO | QESCO | TESCO | TESCO`
- **`TESCO` appears TWICE and `SEPCO` is missing entirely.** Verbatim markup: `<td class=xl94>TESCO<span style='mso-spacerun:yes'> </span></td>  <td class=xl99>TESCO<span style='mso-spacerun:yes'> </span></td>`. Column 11 is almost certainly SEPCO mislabelled, but that is **inference, not verified** — the page says TESCO twice.
- **19 data rows**, `cols_hist = {12: 12, 11: 9, 1: 2}` — 11 rows carry `Year`, 8 do not (`rowspan` collapse).
- Years present: **2011, 2012, 2013, 2014, 2015, 2018, 2019, 2020, 2021, 2022. 2016 and 2017 are absent.**
- 19 SRO dates in `dd-mm-yy`: `15-03-11, 06-05-11, 16-05-12, 05-08-13, 11-10-13, 01-11-14, 10-06-15, 22-03-18, 01-01-19, 28-06-19, 30-09-19, 29-11-19, 19-10-20, 23-12-20, 12-02-21, 01-10-21, 05-11-21, 05-07-22, 25-07-22`.
- Cell values are SRO numbers and **must stay strings**: the 01-01-19 row is `03 | 06 | 02 | 04 | 07 | 05 | 09 | 01 | 08 | 10` — zero-padded. Integer-casting destroys `SRO 03(I)/2019`. Some cells are blank (PESCO in 2013, TESCO in 2011).

### 1d. Hydel 1 / Hydel Data

- Shell: `.../Detail%20of%20Generation/Hydel%201/Hydel%20Data.htm` → **200, 9,587 B**, ASCII, frameset.
- Payload: `.../Hydel%201/Hydel%20Data_files/sheet001.htm` → **200, 14,783 B**.
- Byte-safe: **1 `<table>`, 33 `<tr>`, 126 `<td>`**. Reproduced.
- **Verbatim header**: `Sr # | Company Name | Dependable Capacity (MW) | Period`
- **Two sections in one table**, each announced by a single-cell row: `WAPDA Hydel Projects` (23 rows) and `Private Hydel IPPs` (6 rows) = **29 data rows**. `cols_hist = {4: 31, 1: 2}`. The `Sr #` counter **restarts at 1** in the second section — so `Sr #` is not a primary key.
- Sums I computed: WAPDA 23 projects = **9,405.49 MW** dependable; Private 6 IPPs = **483.60 MW**.
- `Period` has only 4 distinct values: `July 2018 - June 2022` (majority), `May 2019 - June 2022`, `March 2020 - June 2022`, `September 2021 - May 2022`. It is a **coverage label, not a time series** — the whole table is one frozen FY2018-22 snapshot with one MW number per plant.
- Largest entries: Tarbela 3478, Ghazi Brotha 1450, Tarbela Ext.4 1410, Mangla 1000, Neelum Jhelum 969.

---

## 2. Surfaces NOT in the given list (all statuses real)

### 2a. Generation licence + capacity register — 17 pages, 335 entities (HTML, cheap) ★ biggest find

`https://nepra.org.pk/licensing/Generation%20<X>.php`, all **HTTP 200**:

| page | bytes | entities |
|---|---|---|
| `Generation IPPs 1994` | 86,436 | 15 |
| `Generation IPPs 1995 Hydel` | 49,827 | 1 |
| `Generation IPPs 2002` | 111,708 | 29 |
| `Generation IPPs 2006 kpk` | 92,165 | 21 |
| `Generation IPPs 2006 punjab` | 88,173 | 19 |
| `Generation IPPs 2007 Balochistan` | 53,136 | 3 |
| `Generation IPPs 2015` | 70,485 | 10 |
| `Generation IPPs RE 2006` | 300,806 | 122 |
| **`Generation IPPs Sindh`** | 47,627 | **0 — page is 200 but EMPTY** |
| `Generation IPPs others` | 67,825 | 9 |
| `Generation IPPs short term` | 49,655 | 2 |
| `Generation GENCOs` | 56,104 | 4 |
| `Generation K-Electric` | 47,501 | 1 |
| `Generation WAPDA Hydel` | 44,206 | 1 |
| `Generation CPPs` | 209,681 | 65 |
| `Generation SPPs` | 91,978 | 18 |
| `Generation NPPs` | 50,684 | 5 |
| `Generation NCPPs` | 64,785 | 10 |

Structure is `<ul class="accordions toggles"> → <li class="accordion"> → <div class="accordion-header">COMPANY</div> → <div class="accordion-content"><table border="1">`, and each table is a **2-column key/value block, not a row table**. Key frequency across all 335 entities: `Gross Capacity` 332, `Licence No.` 332, `Plant Type` 331, `Licence Details` 329, `Plant Detail` 302, `Fuel Type` 216, then dated modification keys (`Modification-I` 45, `Licence Modification-I` 36, `Modification-II` 16 …).

Parsed `Gross Capacity` for **331/335 entities, sum = 47,559.97 MW**. Verbatim examples:
- `Hub Power Company Limited :: cap=1292 MW type=Thermal (Simple Cycle) fuel=RFO lic=IPGL/13/2003`
- `Kot Addu Power Company (KAPCO) IPP Privatization :: cap=1600 MW type=Thermal (Combined Cycle) fuel=Natural Gas/RFO lic=IPGL/20/2004`
- `K-Elecric :: Gross Capacity 2817.114 MW, Licence No. GL/04/2002` + Modification-I … Modification-XI (11 modifications, each a PDF)
- `WAPDA Hydel :: Gross Capacityy 17,367.96 MW, Licence No. GL(Hydel)/05/2004`
- `Nishat Power Limited :: 202.179 MW, IGSPL/15/2007`; `Nishat Chunian Power Limited :: 202.179 MW, IGSPL/14/2007`
- `Lucky Electric Power Company Limited :: 660.00 MW Coal, IGSPL/66/2015`; `Lucky Cement Limited :: 16 MW Waste Heat, SGC/104/2014`; `Lucky Cement Limited :: 29.7304MW Natural Gas, SGC/72/2011`; `Lucky Energy (Pvt.) Ltd :: 56.575 MW, SGC/030/2005`
- `China Power Hub Generation Company (Private) Limited :: 1320.00 MW Coal, IGSPL/68/2016`; `Thar Energy Limited :: 330.00 MW, IGSPL/83/2017`; `Thalnova Power Thar :: 330.00 MW, IGSPL/75/2017`; `Hub Power Generation Company (Pvt.) Limited Narowal :: 224.79 MW, IGSPL/19/2008`
- `AES Lal Pir :: 362 MW, IPGL/06/2003`; `AES PakGen :: 365 MW, IPGL/07/2003`; `Kohinoor Energy Limited :: 131.44 MW, IPGL/16/2003`; `Altern Energy Limited :: 10.5 MW Flare Gas, IPGL/21/2004`; `Attock Gen Limited :: 165.285 MW, IGSPL/08/2006`; `Saif Power Limited :: 225 MW, IGSPL/04/2006`; `Engro Powergen Qadirpur :: 226.52 MW, IGSPL/13/2007`; `Nishat Mills Limited :: 128.241 MW, SGC/40/2008`

### 2b. Tariff determination stream — the live, current surface (HTML, cheap)

Same accordion shape, but each accordion body is a **3-column, header-less table: `date | description | "view"/"View" → PDF href`**. All **HTTP 200**. Row counts and date ranges I computed:

| surface | URL | bytes (uncompressed) | rows | date range |
|---|---|---|---|---|
| IPP Thermal | `tariff/Generation%20IPPs%20Thermal.php` | 3,468,466 | 5,333 | 03-01-2005 … 04-09-2026 (+1 typo `13-10-2026`) |
| IPP Wind | `tariff/Generation%20IPPs%20Wind.php` | 1,936,670 | 2,440 | 23-08-2006 … 31-08-2026 |
| IPP Coal | `tariff/Generation%20IPPs%20Coal.php` | 982,281 | 1,318 | 23-08-2006 … 04-09-2026 |
| IPP Solar | `tariff/Generation%20IPPs%20Solar.php` | 635,855 | 751 | 28-03-2014 … 30-07-2026 |
| IPP Bio Energy | `tariff/Generation%20IPPs%20Bio%20Energy.php` | 453,623 | 566 | 28-06-2012 … 18-08-2026 |
| IPP Hydel | `tariff/Generation%20IPPs%20Hydel.php` | 437,432 | 481 | 02-04-2008 … 24-08-2026 |
| IPP Short Term | `tariff/Generation%20IPPs%20Short%20Term.php` | 74,431 | 43 | 04-02-2016 … 26-02-2020 |
| IPP Waste to Energy | `tariff/Generation%20IPPs%20Waste%20to%20Energy.php` | 47,182 | 0 | empty |
| **K-Electric distribution/supply** | `tariff/Distribution%20K-Electric.php` | 461,145 | **599** | **10-09-2002 … 07-09-2026** (24 year-accordions) |
| K-Electric generation | `tariff/Generation%20K-Electric.php` | 48,244 | 9 | 22-10-2024 … 28-08-2026 |
| 11 × XWDISCO | `tariff/Distribution%20{FESCO,GEPCO,HAZECO,HESCO,IESCO,LESCO,MEPCO,PESCO,QESCO,SEPCO,TESCO}.php` | 79,652–349,772 | **3,682** | 28-06-2004 … **07-09-2026** |
| WAPDA (pre-unbundling) | `tariff/Distribution%20WAPDA.php` | 44,236 | 6 | **27-03-1999** … 29-07-2002 |
| Upfront (16 technologies) | `tariff/Generation%20Upfront.php` | 97,111 | 79 | 28-04-2006 … 10-06-2024 |
| Petitions | `tariff/Petitions.php` | 379,368 | 698 | 13-01-2021 … 19-06-2026 |
| Orders of the Authority | `M&E/Orders%20of%20the%20Authority.php` | 225,191 | 278 | 13-01-2017 … 31-08-2026 |
| WAPDA Hydroelectric | `tariff/Generation%20WAPDA%20Hydroelectric.php` | 66,349 | 29 | 24-05-2004 … 07-07-2026 |
| Transmission NTDC + Market Operators | `tariff/Transmission%20NTDC.php`, `tariff/Market%20Operators.php` | 71,969 / 87,507 | 93 | 13-04-2004 … 17-08-2026 |

**Total: 16,405 accordion rows across those, spanning 27 Mar 1999 → 7 Sep 2026 (yesterday).**

The IPP pages carry **262 named company accordions** (Thermal 44, Hydel 43, Wind 58, Solar 30, Coal 22, Bio 29, Short-Term 2, plus shared `Transition from LIBOR to SOFR` and `Consumer Price Index (CPI) for Indexation` header blocks). Verbatim rows:
- `01-09-2026 | Decision of the Authority in the matter of Fuel Price Adjustment for the month of July 2026 for Nishat Power Limited (NPL) | View → Tariff/IPPs/Nishat%20Power/2026/TRF-71%20NPL%20FPA%20JUL-2026%2001-09-2026%2018166-70.pdf`
- `03-09-2026 | … Nishat Chunian Power Limited | View → Tariff/IPPs/Nishat%20Chunian/2026/TRF-70%20NCPL%20FPA%20JUL-2026%2003-09-2026%2018209-13.pdf`
- `05-08-2026 | Decision … Quarterly Indexations/Adjustments of tariff for July-September 2025 to April-June 2026 Quarters for Kot Addu Power Company Limited | View → Tariff/IPPs/KAPCO/2026/TRF-600%20KAPCO%20QTR%20JUL-SEP%202025%20TO%20APR-JUN%202026%2005-08-2026%2014666-70.pdf`
- `28-08-2026 | Notification S.R.O 1506(I)/2026 - NEPRA hereby notifies the Decision of the Authority dated July 28,2026 in the matter of Reimbursement of 7.5% withho… | view → Tariff/Notifications/2026/09%20Sep/SRO%201506(I)-2026%20Dated%2028-08-2026.pdf`

**Stable per-company docket numbers** are embedded in the PDF filenames and act as join keys: `TRF-71` = Nishat Power, `TRF-70` = Nishat Chunian, `TRF-600` = KAPCO, `TRF-314` = Lucky Cement, `TRF-408` = Punjab Thermal, `TRF-100` = Ex-WAPDA DISCOs collectively.

`tariff/Generation%20IPPs%20Thermal.php` alone contains **926** matches for `S.R.O nnn(I)/YYYY`; `tariff/Distribution%20LESCO.php` contains **68** SRO rows running to 2026 — so the frozen SRO Data Webpage (19 rows, ends 25-07-2022) is fully superseded here.

### 2c. Consumer-end tariff schedule — real HTML tables, but ONE stale vintage

`https://nepra.org.pk/consumer%20affairs/Electricity%20Bill.php` → **200, 152,235 B, 7 `<table>`, 112 `<tr>`, 2 pdf refs.** Genuine slab/TOU rate tables (A1 residential slabs `Up to 50 Units`, `01-100 Units`, `101-200 Units`; A2 commercial; A3 general services; B1/B2 industrial with peak/off-peak; C1(a)/(b); D agricultural/Scarp; J-1/J-2 by voltage), with `Fixed Charges` Rs./kW/M and variable Rs./kWh.

**But the verbatim column headers pin it to one date:** `Description | Fixed Charges | Notified Tariff 01-01-2019 | * Industrial Support Package w.e.f. July 01, 2019 | Qtr. Adjust. for 1st & 2nd quarter, Notified w.e.f 01-07-201… | Qtr. Adjust. for 3rd & 4th quarter … | Quarterly Uniform Tariff 1st QTR 2019-20 w.e.f. 01-12-2019 | Total Applicable Tariff | Monthly FCA for October 2019, charged in January 2020 | Total`, with a formula row `A | B | C | D | E | F | G= B+C+D+E+F`. It is a **frozen January-2020 snapshot**, ~6.5 years stale. No series, no history.

### 2d. State of Industry Report series — 22 PDFs, expensive

`publications/State%20of%20Industry%20Reports.php` → **200, 57,876 B, 0 tables, 46 pdf refs.** Links `State of Industry Reports/State of Industry Report {2004..2025}.pdf` — **22 consecutive years, consistent naming** (unlike the PER series). HEAD-verified `Content-Length` and `Last-Modified`:

| year | bytes | last-modified |
|---|---|---|
| 2019 | **106,428,020** | Thu, 14 May 2020 |
| 2020 | 42,616,315 | Tue, 20 Oct 2020 |
| 2021 | 7,595,614 | Thu, 30 Sep 2021 |
| 2022 | 6,140,818 | Fri, 30 Sep 2022 |
| 2023 | 3,425,763 | Fri, 02 Feb 2024 |
| 2024 | 9,350,361 | Tue, 31 Dec 2024 |
| 2025 | **331,853,390** | Fri, 30 Jan 2026 |

All return `accept-ranges: bytes`, `server: cloudflare`, `cache-control: max-age=2592000`. **SIR 2025 is 316 MB.**

### 2e. Performance Evaluation Reports — series extends past the already-known list

`publications/Performance%20Reports.php` → **200, 57,982 B, 1 table, 40 `<tr>`, 42 pdf refs.** New paths not in the prior list: `../M&E/PER/Distribution/2026/PER 2024-25 Distribution Companies.pdf`, `../M&E/PER/Generation/2026/PER GENERATION FY 2024-25.pdf`, `../M&E/PER/Transmission/2025/PER Transmission FY 2024-25.pdf`, `../M&E/PER/Distribution/PER 2022-23 - DSICOs.pdf` (note the typo `DSICOs`), `../M&E/PER/Generation/2024/Performance Report Evaluation Report of Operational Power Plants FY 2023-24.pdf`, plus HSE reports. Naming inconsistency is worse than documented — six different directory conventions across `Standards/`, `Standards/<year>/`, `M&E/PER/<segment>/`, `M&E/PER/<segment>/<year>/`.

### 2f. Other 200s characterized

`publications/Annual Reports.php` 56,505 B / 46 pdf; `publications/News Letters.php` 61,223 B / 54 pdf; `publications/Inquiry Reports.php` 40,424 B / 4 pdf; `licensing/Distribution XWDISCOs.php` 75,035 B / 68 tr; `licensing/Transmission.php` 55,697 B / 31 tr; `tariff/Transmission HVDC.php` 10,338 B / 62 tr; `tariff/Transmission STDCPL.php` 7,767 B / 28 tr; `tariff/Generation Benchmark.php` 40,505 B / 1 tr; `ctbcm.php` 47,848 B / 0 tr; `CSR/CSR.php` 56,410 B / 0 tr. `elicensing` and `dxp/` both **200 after 1 redirect** but only 3,068 B / 2,372 B — login shells, no data.

`licensing/Generation IPPs 1994.php` etc. also expose the **licence-modification PDF trail** (K-Electric alone has Modification-I…XI plus `Order of the Authority in the matter of 3rd Party Heat Rate Test for 248 MW KCCP`).

---

## 3. Cost classification

**Cheap (HTML, gzip-able):** every `.php` index and every `_files/sheet001.htm`. **`--compressed` is load-bearing** — without it, `tariff/Generation IPPs Wind.php` / `Solar.php` / `Coal.php` all hit `curl: (28) Operation timed out after 30s` mid-body on the first attempt; with `Accept-Encoding: gzip` the 1,936,670-byte Wind page transferred in **137,704 wire bytes** and completed instantly. Same for Solar (635,855 → 52,113) and Coal (982,281 → 75,402).

**Expensive (PDF):** every actual *number* in the tariff-determination stream (~16,405 PDFs), the 22 SIR reports (3 MB – 316 MB), ~40 PER reports, licence and modification documents.

**Zero XLSX/CSV anywhere.** I grepped `\.xls[xmb]?` across ~60 fetched pages: **0 hits on every single one.** NEPRA publishes only HTML tables and PDFs.


### VERIFIED
- All four Main.htm-linked surfaces are frameset SHELLS, not tables. Each is ~9.6 KB with identical chrome: FCA (2018-2022).htm = 200/9,612 B, Quarterly Data (XWD & KE).htm = 200/9,796 B, Notification/SRO Data Webpage.htm = 200/9,636 B, Hydel 1/Hydel Data.htm = 200/9,587 B. Byte-safe counts on all four shells: 3 <frameset>, 5 <frame >, 2 <table>, 3 <tr>, 24 <td> — pure chrome.
- Each shell's filelist.xml lists exactly 5 files (../shell.htm, filelist.xml, sheet001.htm, stylesheet.css, tabstrip.htm) and each tabstrip.htm names exactly ONE sheet (FCA -> 'FCA', SRO -> 'Sheet1', Hydel -> 'Hydel'). No hidden second sheet exists in any of the four workbooks.
- FCA payload https://nepra.org.pk/publications/State%20of%20Industry%20Reports/Detail%20of%20Generation/FCA%20(2018-2022)_files/sheet001.htm = HTTP 200, 79,843 B, ISO-8859. Byte-safe: 1 <table>, 53 <tr>, 572 <td>. Reproduced byte-for-byte on an independent second fetch.
- FCA verbatim two-level header: row 1 = 'Year'(rowspan=2) | 'Month'(rowspan=2) | 'CPPA - G'(colspan=2) | 'K-Electric'(colspan=2) | 'E.M.O'(colspan=4); row 2 = 'FCA  Requested' | 'FCA  Allowed (kWh)' | 'FCA  Requested' | 'FCA  Allowed' | '1st Forthnight' | 'Revised' | '2nd Forthnight' | 'Revised'. Upstream misspelling 'Forthnight' confirmed in raw bytes.
- FCA covers exactly Jul-2018 through Jun-2022 = 48 consecutive monthly rows, zero gaps, sorted newest-first. cols_hist = {11:50, 7:1, 6:1, 9:1}. First data row verbatim: '2022 | Jun | 9.9095 | 9.8972 | 11.389 | 11.1023 | 01-06-22 | 06-06-22 | 16-06-22 | 22-06-22'. Last: '2018 | Jul | 0.6261 | 0.3525 | 0.623 | 0.6148 | 10-07-18 | | 20-07-18 |'.
- FCA is the ONLY NEPRA HTML surface carrying actual numeric tariff values (Rs/kWh fuel cost adjustment, CPPA-G and K-Electric, requested vs allowed). It contains 49 parenthesised negative values (e.g. '(1.798)', '(0.1930)', '(2.5935)').
- Quarterly payload .../Quarterly%20Data%20(XWD%20&%20KE)_files/sheet001.htm = HTTP 200, 79,672 B. Byte-safe: 1 <table>, 168 <tr>, 582 <td>. Reproduced. Title cell verbatim: "Requested and Allowed Quarterly Adjustments of XWAPDA DISCOs &amp; K-Electric".
- Quarterly verbatim headers: 'Quarter'(rowspan=2) | 'DISCOs Name'(rowspan=2) | 'Quarterly Adjustments <br> (XWAPDA DISCOs)'(colspan=2) | 'Quarterly Adjustments <br> (K-Electric)'(colspan=2); sub-row = 'Requested By Company' | 'Allowed By NEPRA' | 'Requested By Company' | 'Allowed By NEPRA'.
- Quarterly = 163 data rows, 16 quarters with no gaps: 'July 2018 - September 2018 (1st Quarter)' through 'April 2022- June 2022 (4th Quarter)'. 10 DISCOs per quarter in fixed order FESCO GEPCO HESCO IESCO LESCO MEPCO PESCO QESCO SEPCO TESCO, plus 3 extra 'Reconsideration Request' rows in the three 2020 quarters. cols_hist = {7:18, 3:144, 5:2, 4:3, 2:1}.
- Quarterly contains NO tariff amounts whatsoever — every data cell is a filing/decision date or the literal string 'Not Yet Issued'. 212 dd-mm-yyyy cells, apparent span 30-12-2012 .. 18-08-2022.
- SRO payload .../Notification/SRO%20Data%20Webpage_files/sheet001.htm = HTTP 200, 40,402 B. Byte-safe: 1 <table>, 23 <tr>, 245 <td>. Reproduced. Title cell verbatim: "SRO's Data For XWDISCO'S".
- SRO header lists TESCO TWICE and omits SEPCO. Verbatim raw markup: <td class=xl94>TESCO<span style='mso-spacerun:yes'> </span></td>  <td class=xl99>TESCO<span style='mso-spacerun:yes'> </span></td>. Full header: Year | SRO Date | FESCO | GEPCO | HESCO | IESCO | MEPCO | LESCO | PESCO | QESCO | TESCO | TESCO, with a colspan=10 sub-row reading 'SRO Number'.
- SRO = 19 data rows covering years 2011, 2012, 2013, 2014, 2015, 2018, 2019, 2020, 2021, 2022 — 2016 and 2017 are entirely absent. 19 SRO dates in dd-mm-yy: 15-03-11 through 25-07-22. cols_hist = {12:12, 11:9, 1:2}.
- SRO cell values are zero-padded strings: the 01-01-19 row reads '03 | 06 | 02 | 04 | 07 | 05 | 09 | 01 | 08 | 10'. Integer-casting destroys SRO 03(I)/2019.
- Hydel payload .../Hydel%201/Hydel%20Data_files/sheet001.htm = HTTP 200, 14,783 B. Byte-safe: 1 <table>, 33 <tr>, 126 <td>. Reproduced. Verbatim header: 'Sr # | Company Name | Dependable Capacity (MW) | Period'.
- Hydel holds TWO sections in one table, each announced by a single-cell row: 'WAPDA Hydel Projects' (23 rows, sum 9,405.49 MW dependable) and 'Private Hydel IPPs' (6 rows, sum 483.60 MW) = 29 data rows. Sr # restarts at 1 in section 2, so it is not a primary key. cols_hist = {4:31, 1:2}.
- Hydel's Period column has only 4 distinct values ('July 2018 - June 2022', 'May 2019 - June 2022', 'March 2020 - June 2022', 'September 2021 - May 2022') — it is a coverage label, not a time series. The whole table is one frozen FY2018-22 snapshot with one MW figure per plant.
- 'SIR Data 2025.htm' is NOT a 2025 surface. It is BYTE-IDENTICAL to 'SIR Data 2024.htm' — both md5 37c27f4decfbad45c80078a997f13c19, both 374 B, both <title>SIR Data 2024</title>, both with frame src="List%20of%20Companies%20Genenration%20wise%202023-24.htm". Its header frame home24.htm (200, 669 B) reads verbatim: 'Data pertaining to State of Industry Report 2023-24'.
- The Detail-of-Generation series definitively ends at FY2023-24. I tested 4 naming variants for FY2024-25 plus SIR Data 2026.htm and home25.htm — all HTTP 404 (9-byte 'Not Found' body). home23.htm does exist (200, 672 B).
- robots.txt declares 'Sitemap:https://nepra.org.pk/sitemap_1.xml' and 'Sitemap:https://nepra.org.pk/sitemap_2.xml'. BOTH return HTTP 404, 9 bytes, body literally 'Not Found' (hex-verified: 4e6f 7420 466f 756e 64). There is no machine enumerator for the site. /sitemap.xml is also 404.
- robots.txt policy verbatim: 'User-agent: *' / 'Content-Signal: search=yes,ai-train=no,use=reference' / 'Allow: /'. Named-bot Disallow list includes Amazonbot, Applebot-Extended, Bytespider, CCBot, ClaudeBot, CloudflareBrowserRenderingCrawler, Google-Extended, GPTBot, meta-externalagent. Reference/analysis use is permitted; training is not.
- MAJOR UNLISTED SURFACE — generation licence/capacity register: 17 licensing/Generation*.php pages, all HTTP 200, 335 accordion entities total. Structure is a 2-column key/value table per entity (NOT a row table). Key frequency: Gross Capacity 332, Licence No. 332, Plant Type 331, Licence Details 329, Plant Detail 302, Fuel Type 216. Parsed Gross Capacity for 331/335 entities, sum = 47,559.97 MW.
- Licence register carries direct PSX joins with capacity and licence number: 'Hub Power Company Limited :: 1292 MW, IPGL/13/2003'; 'Kot Addu Power Company (KAPCO) IPP Privatization :: 1600 MW, IPGL/20/2004'; 'K-Elecric :: 2817.114 MW, GL/04/2002' + 11 modifications; 'Nishat Power Limited :: 202.179 MW, IGSPL/15/2007'; 'Nishat Chunian Power Limited :: 202.179 MW, IGSPL/14/2007'; 'Lucky Electric Power Company Limited :: 660.00 MW Coal, IGSPL/66/2015'; 'China Power Hub Generation Company :: 1320.00 MW, IGSPL/68/2016'; 'Thar Energy Limited :: 330.00 MW, IGSPL/83/2017'; 'Thalnova Power Thar :: 330.00 MW, IGSPL/75/2017'; 'Hub Power Generation Company Narowal :: 224.79 MW, IGSPL/19/2008'; 'WAPDA Hydel :: Gross Capacityy 17,367.96 MW, GL(Hydel)/05/2004'.
- MAJOR UNLISTED SURFACE — tariff determination stream: 16,405 accordion rows across the tariff/*.php and M&E pages, spanning 27 Mar 1999 (tariff/Distribution WAPDA.php) to 7 Sep 2026 (K-Electric + all 11 DISCOs). Schema is a header-less 3-column table: date | description | 'view'/'View' -> PDF href. Biggest single page: tariff/Generation IPPs Thermal.php = 200, 3,468,466 B, 5,333 rows, 44 company accordions.
- IPP tariff pages carry 262 named company accordions total (Thermal 44, Hydel 43, Wind 58, Solar 30, Coal 22, Bio Energy 29, Short Term 2) plus shared 'Transition from LIBOR to SOFR' and 'Consumer Price Index (CPI) for Indexation' blocks. tariff/Generation IPPs Waste to Energy.php is 200 but has 0 rows.
- Stable per-company docket numbers embedded in PDF filenames act as join keys: TRF-71 = Nishat Power, TRF-70 = Nishat Chunian, TRF-600 = KAPCO, TRF-314 = Lucky Cement, TRF-408 = Punjab Thermal, TRF-100 = Ex-WAPDA DISCOs. Verbatim: 'Tariff/IPPs/Nishat%20Power/2026/TRF-71%20NPL%20FPA%20JUL-2026%2001-09-2026%2018166-70.pdf'.
- tariff/Distribution K-Electric.php = 200, 461,145 B, 599 rows, 24 year-accordions (2002..2026 with 2004 missing), range 10-09-2002 .. 07-09-2026. Deepest single-issuer HTML timeline on the site and directly KEL-joinable.
- All 11 XWDISCO tariff pages return 200: FESCO 389 rows, GEPCO 368, HAZECO 44, HESCO 418, IESCO 371, LESCO 374, MEPCO 380, PESCO 383, QESCO 365, SEPCO 309, TESCO 281 = 3,682 rows, range 28-06-2004 .. 07-09-2026. HAZECO (new DISCO) starts only 23-06-2025.
- The frozen SRO Data Webpage is superseded by the live per-DISCO pages: tariff/Distribution LESCO.php alone contains 68 rows whose description matches S.R.O, running to 08-06-2026 ('Notification S.R.O 953(I)/2026'). tariff/Generation IPPs Thermal.php contains 926 matches for the pattern S.R.O nnn(I)/YYYY.
- The frozen FCA HTML table is superseded by PDF-only monthly FCA/MFPA determinations: tariff/Distribution LESCO.php and FESCO.php each contain ~297 'Ex-WAPDA%20DISCOS' path refs and 117 'MFPA' refs, e.g. 'tariff/Tariff/Ex-WAPDA%20DISCOS/2026/TRF-100%20MFPA%20FOR%20THE%20MONTH%20OF%20JULY%202026%20EX-WAPDA%20DISCOS%2004-09-2026%2018342-60.PDF'.
- Consumer-end tariff schedule EXISTS as real HTML: consumer affairs/Electricity Bill.php = 200, 152,235 B, 7 <table>, 112 <tr>, with slab (Up to 50 Units / 01-100 / 101-200), TOU peak/off-peak and category A1/A3/B1/B2/C1/D/J-1/J-2 rates. But its verbatim headers freeze it at one vintage: 'Notified Tariff 01-01-2019 | ... | Quarterly Uniform Tariff 1st QTR 2019-20 w.e.f. 01-12-2019 | Total Applicable Tariff | Monthly FCA for October 2019, charged in January 2020 | Total' with formula row 'G= B+C+D+E+F'. ~6.5 years stale, no series.
- State of Industry Report series = 22 consecutive PDFs 2004..2025 at 'publications/State of Industry Reports/State of Industry Report <year>.pdf' (index page 200, 57,876 B, 0 tables, 46 pdf refs). HEAD-verified Content-Length: 2019 = 106,428,020 B; 2020 = 42,616,315; 2021 = 7,595,614; 2022 = 6,140,818; 2023 = 3,425,763; 2024 = 9,350,361; 2025 = 331,853,390 (316 MB). All serve accept-ranges: bytes.
- publications/Performance Reports.php (200, 57,982 B, 42 pdf refs) extends the known PER list with three previously-unrecorded FY2024-25 reports: ../M&E/PER/Distribution/2026/PER 2024-25 Distribution Companies.pdf, ../M&E/PER/Generation/2026/PER GENERATION FY 2024-25.pdf, ../M&E/PER/Transmission/2025/PER Transmission FY 2024-25.pdf. It also confirms a typo path '../M&E/PER/Distribution/PER 2022-23 - DSICOs.pdf'.
- ZERO XLSX/CSV anywhere on NEPRA. I grepped '\.xls[xmb]?' byte-safely across ~60 fetched pages including every index and every sheet001 payload: 0 hits on all of them. The site publishes only HTML tables and PDFs.
- gzip is load-bearing for the big pages. Without --compressed, tariff/Generation IPPs Wind.php, Solar.php and Coal.php each hit 'curl: (28) Operation timed out after 30010 milliseconds' mid-body. With Accept-Encoding: gzip the 1,936,670-byte Wind page transferred in 137,704 wire bytes and completed instantly (Solar 635,855 -> 52,113; Coal 982,281 -> 75,402).
- iconv is NOT a safe universal pre-pass: 'iconv -f WINDOWS-1252 -t UTF-8 p19.htm' failed with "iconv(): Illegal byte sequence" on M&E/Orders of the Authority.php. In-process cp1252 decode with errors='replace' worked on every file.
- Main.htm re-verified: HTTP 200, 12,210 B, and its complete href set is exactly 10 entries — 5 generation years FY2017-18..FY2021-22, Main_files/filelist.xml, and the four target surfaces. It does NOT link SIR Data 2024.htm or SIR Data 2025.htm, confirming the index is incomplete in both directions.

### REFUTED/UNVERIFIED
- REFUTED: 'SIR Data 2025.htm' is a distinct FY2024-25 surface. It is byte-identical to SIR Data 2024.htm (both md5 37c27f4decfbad45c80078a997f13c19), titled 'SIR Data 2024', and frames List of Companies Genenration wise 2023-24.htm. Treating it as a 2025 surface would silently duplicate FY2023-24 data.
- REFUTED: NEPRA has a usable sitemap. robots.txt advertises sitemap_1.xml and sitemap_2.xml; both return HTTP 404 with a 9-byte 'Not Found' body. There is no machine-readable enumerator for the site at all.
- REFUTED: there is a circular-debt data surface. I grepped 'circular debt' (case-insensitive, byte-safe) across ~60 fetched pages. Exactly ONE hit, and it is not a figure — it is a row in M&E/Orders of the Authority.php: '23-10-2025 || Order of the Authority in the Matter of Show Cause Notice issued to LESCO under Regulation 4(8) & 4(9) of NEPRA (Fine) Regulations, 2021 on account of Decline in Performance with respect to Transmission and Distribution (T&D) losses and Recovery based on Circular Debt (CD) for the month of June 2024 || Decisions/2025/10 Oct/TCD-05 LESCO ORDER OF THE AUTHORITY 23-10-2025 16920.pdf'. Circular-debt NUMBERS exist only inside SIR PDFs — I did not open a SIR PDF, so no figure is verified.
- REFUTED: the Quarterly Data surface carries tariff amounts. Every one of its 163 data rows is dates-only. Anyone expecting Rs/kWh quarterly adjustment values there will get nothing.
- REFUTED: any XLSX/CSV download exists on NEPRA. Zero '.xls*' hrefs across ~60 pages.
- UNVERIFIED (inference, flagged): that the duplicate 'TESCO' column in the SRO header is really SEPCO. Column order elsewhere on the page is FESCO GEPCO HESCO IESCO MEPCO LESCO PESCO QESCO TESCO TESCO, and SEPCO is the only XWDISCO missing — but the page literally says TESCO twice and I have NOT confirmed the mapping against any SRO PDF. Do not silently rename column 11.
- UNVERIFIED: the exact interpretation of the FCA 'E.M.O' column group. The four sub-columns are '1st Forthnight | Revised | 2nd Forthnight | Revised' and contain dd-mm-yy dates, so they are notification/effective dates, but nothing on the page expands the abbreviation E.M.O and I did not confirm it.
- UNVERIFIED: the internal content of any PDF. I HEAD-ed the SIR series for size and Last-Modified but opened zero PDFs. Every tariff/FPA/quarterly-indexation NUMBER after Jun-2022 is therefore UNKNOWN from HTML alone — the post-2022 HTML rows give only date + prose description + PDF href.
- UNVERIFIED / NOT REPRODUCED: tariff/Generation IPPs Thermal.php's apparent max date 13-10-2026 is in the future (today is 2026-09-08). Its own row says 'Notification S.R.O 1395(I)/2026 ... Decision of the Authority dated August 10, 2026' and the href is 'SRO 1395(I)-2026 Dated 13-08-2026.pdf' — so the date CELL is an upstream typo for 13-08-2026. I did not open the PDF to confirm. Do not ingest 13-10-2026 as a real date.
- UNVERIFIED: which listed equity each licence entity maps to. I matched entity NAMES only (e.g. 'AES Lal Pir' -> LPL, 'AES PakGen' -> PKGP, 'Kohinoor Energy Limited' -> KOHE). No ticker mapping table was fetched or validated, and one abbreviation actively collides: the licence register lists 'Kohinoor Energy Ltd (KEL)' while KEL is K-Electric's PSX symbol. Any ticker crosswalk must be built and checked by hand.
- UNVERIFIED: whether tariff/Generation IPPs Wind.php / Solar.php / Coal.php row counts from the FIRST (timed-out, truncated) fetch were complete. I re-fetched all three with --compressed (rc=0) and used only those complete files for the counts reported. The first-pass truncated byte counts (832,161 / 576,059 / 598,463) are NOT the real page sizes and should be ignored.
- NOT ATTEMPTED: NEPRA's /elicensing and /dxp/ portals. Both return 200 after one redirect but are only 3,068 B and 2,372 B — login shells. Whether a licence DATABASE sits behind them is UNKNOWN; I did not attempt authentication.

### BUILD IMPLICATIONS
- CARRY IN V1 #1 — Generation licence & capacity register (17 licensing/Generation*.php pages, 335 entities, 47,560 MW parsed + WAPDA Hydel's 17,368 MW). Best machine-readability-to-uniqueness ratio on the site and the ONLY surface giving Gross Capacity MW + Licence No. + fuel + plant configuration per legal entity. It is also the strongest equity join: HUBC gets 5 entities (Hub Power 1292 MW IPGL/13/2003, Narowal 224.79 MW, CPHGCL 1320 MW, Thar Energy 330 MW, ThalNova 330 MW), KAPCO 2 (1600 MW IPGL/20/2004 + KAPCO Energy), LUCK 4 (Lucky Electric 660 MW, Lucky Cement 16 MW + 29.7304 MW, Lucky Energy 56.575 MW), NPL and NCPL 202.179 MW each, KEL 2817.114 MW GL/04/2002. It is a STOCK not a series, so it is a small nightly refresh, not a backfill.
- CARRY IN V1 #2 — Tariff determination stream (7 IPP fuel pages + 11 DISCO pages + 2 K-Electric pages + WAPDA + Upfront + Petitions + Orders = 16,405 rows, 27 Mar 1999 to 7 Sep 2026). This is the ONLY current surface — it was updated yesterday, while all four Main.htm surfaces froze in 2022. Ship it as a dated regulatory-EVENT feed (date, company, event type, SRO number, TRF docket, PDF href), NOT as a numeric feed: the amounts live inside the PDFs. That still makes it event-study-grade for exactly the target tickers, keyed on the stable dockets TRF-71 (NPL), TRF-70 (NCPL), TRF-600 (KAPCO), TRF-314 (Lucky Cement), TRF-100 (Ex-WAPDA DISCOs).
- CARRY IN V1 #3 — FCA (2018-2022). It is small (48 rows, 79,843 B), it is the ONLY numeric tariff surface in HTML anywhere on NEPRA, and it is a clean monthly series with zero gaps. Its K-Electric requested/allowed columns join KEL directly; its CPPA-G column is the sector-wide fuel-cost signal that drives every IPP's FPA and every DISCO's recovery. Cheapest real number on the site. Ship it with a hard-coded 2018-07..2022-06 coverage window so nobody mistakes it for live.
- DEFER — Quarterly Data (XWD & KE). 163 rows of DATES ONLY (no amounts), 66 rowspans, and at least three upstream date typos including a 9-year error (30-12-2012 in a 2021 quarter). Its entire function — 'when did each DISCO file and when did NEPRA decide' — is done better and to 2026 by the 11 per-DISCO tariff pages (3,682 rows). Low value, high parse cost.
- DEFER — SRO Data Webpage. 19 rows, ends 25-07-2022, missing 2016 and 2017, and its header names TESCO twice while omitting SEPCO. Superseded: tariff/Distribution LESCO.php alone has 68 SRO rows running to Jun-2026, and tariff/Generation IPPs Thermal.php has 926 parseable S.R.O nnn(I)/YYYY references. Do not ship a surface whose column mapping you cannot verify.
- FOLD, DO NOT SHIP SEPARATELY — Hydel Data. 29 rows, one frozen FY2018-22 period label, and one equity join (Laraib/New Bong Escape 88.0 MW = HUBC's hydel subsidiary). Add 'dependable capacity MW' as a supplementary FIELD on the licence-register records rather than a standalone command; the register already covers the same plants with gross capacity.
- DEFER ALL PDF SURFACES to v2 behind an explicit --fetch-pdf flag. SIR 2025 is 331,853,390 B (316 MB) and SIR 2019 is 106,428,020 B. All serve accept-ranges: bytes, so a v2 could range-request a table of contents rather than pull 316 MB. Circular debt exists ONLY in these PDFs — there is no HTML or numeric circular-debt surface anywhere on nepra.org.pk, so a circular-debt command cannot be built from cheap sources at all.
- MAKE --compressed NON-OPTIONAL IN THE HTTP LAYER. Three tariff pages time out at 30 s without gzip and return a silently truncated body under HTTP 200 (Wind gave 1,028 rows truncated vs 2,440 complete). Pair it with a completeness assertion — require a closing </html> or a minimum row count per page — because a truncated 200 is otherwise indistinguishable from a short page and will silently under-report.
- DO NOT USE iconv ANYWHERE. It failed with 'iconv(): Illegal byte sequence' on M&E/Orders of the Authority.php. Decode in-process as cp1252 with errors='replace' — that worked on all ~60 files including every sheet001 payload.
- HARD-CODE THE INDEX, DO NOT RELY ON DISCOVERY. robots.txt declares sitemap_1.xml and sitemap_2.xml; both are 404. Main.htm is incomplete in BOTH directions (links 5 generation years of 7 reachable; omits SIR Data 2024/2025.htm which are reachable). Ship an explicit URL manifest of the ~60 verified endpoints plus a `discover` subcommand that re-crawls the .php indexes and DIFFS against the manifest, so new years and new DISCOs (HAZECO appeared only in Jun-2025) surface as a reported delta instead of silently missing.
- BUILD TWO PARSERS, NOT ONE. licensing/Generation*.php accordions are 2-column key/value blocks; tariff/*.php accordions are 3-column date|description|href rows. Identical CSS classes, different semantics. Dispatch on cols_hist, and treat a page whose histogram does not match its expected shape as a FAILURE, not as data.
- NORMALISE KEYS FUZZILY IN THE LICENCE PARSER, AND ASSERT ON COVERAGE. Exact-match key lookup on 'Gross Capacity' silently dropped WAPDA Hydel's 17,367.96 MW because the page says 'Gross Capacityy'. Alias-map the known typos ('Gross Capacityy', 'Plant Detaill', 'Fuel'/'Fuel Type', the five modification spellings) and emit a warning whenever fewer than 100% of entities yield a capacity — that one assertion is what caught this.
- NEVER SYNTHESISE A PDF URL; ALWAYS CARRY THE VERBATIM HREF. Month folders appear as both '2026/07 July/' and '2026/07 Jul/', extensions as both .pdf and .PDF, link labels as both 'view' and 'View'. Store the href exactly as scraped.
- DEDUPE THE DETERMINATION STREAM ON THE PROSE DATE, NOT THE ROW DATE. Every decision is listed twice — once as 'Decision of the Authority' under Tariff/IPPs/<Company>/ and once as 'Notification S.R.O nnn(I)/YYYY' under Tariff/Notifications/ — with different row dates. Parse the authoritative date out of the description ('Decision of the Authority dated August 25, 2026') and treat the row date as untrusted metadata. Emit both PDFs as attachments to one event.
- FLAG, DO NOT FIX, UPSTREAM DATE TYPOS. Emit both a raw_date_cell and a parsed_date plus a date_suspect boolean set when the cell contradicts the prose or the href filename (13-10-2026 vs 'Dated 13-08-2026'), is in the future, or has a malformed shape ('-02-10-2009'). Per the never-fabricate rule, record the conflict rather than picking a value.
- TREAT A ~40-48 KB RESPONSE AS THE EMPTY-PAGE BASELINE. licensing/Generation IPPs Sindh.php is 200 at 47,627 B with 0 entities; tariff/Generation IPPs Waste to Energy.php is 200 at 47,182 B with 0 rows. Assert a minimum content yield per endpoint so a silently emptied page is reported as UNKNOWN rather than ingested as zero.
- STAMP EVERY RECORD WITH ITS COVERAGE WINDOW AND SOURCE URL. The site mixes a live 2026 stream with four surfaces frozen in 2022, a consumer tariff table frozen at Jan-2020, an Upfront table frozen at Jun-2024, and a 'SIR Data 2025' stub that actually serves FY2023-24. Without an as-of field the CLI will hand callers 2019 tariffs and 2022 FCA as current.
- RESPECT robots.txt IN THE SHIPPED CLI. Content-Signal is 'search=yes,ai-train=no,use=reference' with a named-bot Disallow list that includes ClaudeBot. Ship an operator-configurable UA that does not self-identify as a blocked named bot, document reference/analysis-only use, add polite rate limiting, and put a no-training note in the README.
- BUILD THE TICKER CROSSWALK BY HAND AND GUARD THE COLLISION. Entity names map to PSX names, not symbols, and the licence register lists 'Kohinoor Energy Ltd (KEL)' while KEL is K-Electric's PSX symbol. Ship the crosswalk as reviewable data with a confidence field, never as regex inference — a wrong HUBC/KAPCO mapping silently corrupts every downstream event study.

### TRAPS
- ENCODING: all four sheet001 payloads and Main.htm are ISO-8859/windows-1252. A UTF-8 grep finds zero tags. Worse than expected: `iconv -f WINDOWS-1252` itself FAILED with 'iconv(): Illegal byte sequence' on M&E/Orders of the Authority.php. Only in-process cp1252 decode with errors='replace' worked on every file. Do not build the pipeline on iconv.
- TIMEOUT-BY-OMISSION: three tariff pages (Wind 1.94 MB, Coal 0.98 MB, Solar 0.64 MB) time out at 30 s without Accept-Encoding: gzip and return a SILENTLY TRUNCATED body with HTTP 200 already written. Wind's first pass yielded 1,028 rows; the complete gzip fetch yielded 2,440. Always send --compressed AND verify completeness — a truncated 200 is indistinguishable from a short page.
- ROWSPAN COLLAPSE: Quarterly has 66 rowspan attributes. Only the FIRST DISCO row of each quarter block carries the quarter label and BOTH K-Electric date columns; the other 9 rows have 3 cells instead of 7. SRO has 8 rowspans, so 8 of 19 data rows have no Year. A naive row-wise parser produces ragged rows and silently mis-attributes dates. cols_hist is the detector: Quarterly {7:18, 3:144, 5:2, 4:3, 2:1}; SRO {12:12, 11:9, 1:2}.
- LEADING SPACER CELL: every data row in FCA (11 <td>) and Quarterly carries an empty leading <td> from the Excel export. Column index 0 is not 'Year' — it is nothing. Off-by-one on every field if you index positionally without dropping it.
- TWO-LEVEL HEADERS: FCA and Quarterly headers span two <tr> joined by rowspan=2 + colspan=2/4. Reading only the first <tr> gives you 'CPPA - G' and 'K-Electric' with no idea which of the two is Requested vs Allowed. You must synthesise 'CPPA-G / FCA Requested' etc. from both rows.
- PARENTHESISED NEGATIVES: FCA contains 49 accounting-style negatives, e.g. '(1.798)', '(0.1930)', '(2.5935)'. float('(1.798)') throws; a regex that strips non-numerics silently flips the sign to POSITIVE. FCA is signed by nature — this corrupts the data rather than failing loudly.
- ZERO-PADDED STRINGS: SRO numbers include '03', '06', '01', '02' (the 01-01-2019 row). int() casting destroys 'SRO 03(I)/2019'. Keep as strings.
- TWO SECTIONS IN ONE TABLE with a RESTARTING COUNTER: Hydel puts 'WAPDA Hydel Projects' (23 rows) and 'Private Hydel IPPs' (6 rows) in a single <table>, and Sr # restarts at 1. Keying on Sr # collides 6 rows. Section membership only exists as a single-cell row you must carry as state.
- UPSTREAM DATE TYPOS THAT PASS VALIDATION: Quarterly's minimum date is 30-12-2012, sitting in the 'Allowed By NEPRA' cell of the April 2021 - June 2021 quarter — a 9-year impossibility that parses fine. Also IESCO 20-05-2020 inside the Jan-Mar 2021 quarter, and HESCO 05-03-2022 inside the Oct-Dec 2019 quarter. FCA's E.M.O column has 16-09-22 / 16-07-22 / 16-06-22 / 16-05-22 / 16-04-22 / 16-03-22 / 16-02-22 / 21-01-22 sitting in 2021 MONTHS. IPP Thermal has a cell reading 13-10-2026 whose own href says 'Dated 13-08-2026'. Nishat Power has a cell literally reading '-02-10-2009' with a leading hyphen. Always cross-check the date cell against the description prose and the PDF filename; never accept the cell alone.
- HEADER TYPO KILLS YOUR KEY LOOKUP: SRO header lists TESCO twice and omits SEPCO. Licence register keys include 'Plant Detaill' (6x), 'Gross Capacityy' (WAPDA Hydel — which is why my capacity sum missed 17,368 MW and covered only 331 of 335 entities), 'Fuel' vs 'Fuel Type', and five spellings of the modification key ('Modification-I', 'Licence Modification-I', 'Licence Modification -I', 'Modification - I', 'Licence Modification'). The K-Electric accordion header itself reads 'K-Elecric'. Exact-match key lookup silently drops the largest single generator in Pakistan.
- SCHEMA IS NOT UNIFORM ACROSS ACCORDION PAGES: licensing/Generation*.php accordions are 2-column KEY/VALUE blocks (cols_hist {2:98}); tariff/*.php accordions are 3-column date|description|href rows. Same CSS classes, same markup shape, completely different semantics. A single generic accordion parser will mangle one of the two.
- _files DIRECTORY NAME DERIVES FROM THE SHELL FILENAME, NOT THE LINK PATH, and they diverge: 'Notification/SRO Data Webpage.htm' -> 'Notification/SRO Data Webpage_files/', 'Hydel 1/Hydel Data.htm' -> 'Hydel 1/Hydel Data_files/'. Naively appending '_files' to the link's basename works here but the subdirectory prefix must be preserved. Read the frame src from the shell instead of synthesising.
- MONTH-FOLDER NAMING IS INCONSISTENT WITHIN THE SAME MONTH: '2026/07 July/' AND '2026/07 Jul/' both exist; likewise '2026/06 June/' and '2026/06 Jun/'. Also mixed extension case ('.pdf' vs '.PDF') and mixed link label case ('view' vs 'View'). NEVER synthesise a PDF URL — always take the verbatim href.
- EVERY DETERMINATION IS LISTED TWICE: once as the 'Decision of the Authority' (Tariff/IPPs/<Company>/... TRF-nnn ...pdf) and again as the 'Notification S.R.O nnn(I)/YYYY' (Tariff/Notifications/YYYY/...). Same underlying event, different date, different PDF. Row counts are ~2x the event count. Dedup on the decision date parsed OUT OF THE PROSE ('Decision of the Authority dated August 25, 2026'), not the row date.
- STALE-SNAPSHOT-AS-LIVE-DATA: consumer affairs/Electricity Bill.php looks like a current consumer tariff schedule (7 real tables, 112 rows). Its headers pin it to 'Notified Tariff 01-01-2019' + 'Monthly FCA for October 2019, charged in January 2020'. Shipping it undated presents 2019 rates as today's.
- INDEX INCOMPLETENESS IS BIDIRECTIONAL: Main.htm links 5 generation years but 7 are reachable; and SIR Data 2024.htm/2025.htm are reachable but NOT linked from Main.htm. Meanwhile robots.txt's two declared sitemaps both 404. There is no reliable enumerator — the .php index pages must be hardcoded and periodically re-crawled.
- PAGE 200 != PAGE HAS DATA: licensing/Generation IPPs Sindh.php returns HTTP 200 at 47,627 B but contains ZERO entities (0 accordions, 2 boilerplate pdf refs). tariff/Generation IPPs Waste to Energy.php returns 200 at 47,182 B with 0 rows. A ~40-48 KB response on this site is the empty-page baseline (chrome only). Never treat 200 as evidence of content.


########## PROBE: nepra_users_and_unanswerable_questions ##########

# WHO uses NEPRA data, and what they cannot answer today

All numbers below came from `curl -sS --max-time 30` with a browser UA, byte-safe reads (`LC_ALL=C grep -a` / `iconv -f WINDOWS-1252`), and PyMuPDF for PDFs. Every figure is reproducible from the artifacts in `/private/tmp/claude-501/-Users-hamzaqazi-Desktop-Personal/5916c20a-9668-41e6-8ce7-5af354a69c76/scratchpad/nepra/`.

---

## GROUP 1 — Equity analysts covering PSX power stocks

**What they actually model.** For an IPP the P&L is capacity payment (earned on *availability*, not output) + energy payment (fuel pass-through) − receivable financing cost. So the three numbers that move a target price are: load factor, availability/delicensing status, and receivable days. NEPRA's `Detail of Generation` sheet is the only public source of the first two at plant level, monthly.

**Verified: every listed IPP is in the sheet, with 12 monthly load factors + GWh.** I parsed 7 fiscal years (FY2017-18 … FY2023-24) into **765 plant-years = 9,180 plant-months**. Plant counts per year: 96, 101, 106, 104, 124, 116, 118 (canonical 32-cell rows).

The single most valuable thing I built, and the thing no NEPRA page shows — HUBCO's flagship 1,292 MW Hub plant:

| FY | Load factor | GWh |
|---|---|---|
| 2017-18 | 49.24% | 5,205.94 |
| 2018-19 | 7.86% | 831.64 |
| 2019-20 | 0.33% | 35.05 |
| 2020-21 | 2.07% | 218.71 |
| 2021-22 | 11.52% | 1,219.45 |
| 2022-23 | 1.93% | 203.00 |
| 2023-24 | — | **DELICENSED** |

A −96.1% output collapse then exit. Raw FY2023-24 row, verbatim:
`[34][ ][Hub Power Company (HUBCO)][THERMAL][RFO][1,292][1,200][DELICENSED]` — n=8 cells, versus n=33 the year before.

And KAPCO, verbatim FY2023-24:
`[33][ ][Kot Addu Power Company (KAPCO)][THERMAL][RFO/RLNG/HSD][1,601][1,345][0.00][0.00]…` — **1,601 MW installed, 1,345 MW dependable, 0.00 GWh in all twelve months**, still listed as capacity. That is the capacity-payment-without-generation thesis, in one row.

Full FY2017-18→FY2023-24 annual load factors I derived: ALTN 61.34→0.00 (zero since FY2021-22); PKGP 40.64→6.92; NCPL 64.09→14.02; NPL 68.56→26.55; Kohinoor Energy 59.34→19.12; HUBCO/Narowal 63.99→10.71; CPHGC 2.38→70.21→4.35; LUCK/LEPCL 33.59→62.60→31.91.

**Questions they cannot answer today**
1. *"What is HUBC's consolidated fleet load factor by year, across Hub base + Narowal + its 26% of CPHGC?"* — impossible without a hand-maintained crosswalk, because **there is no stable key for a plant across the seven sheets.** Proven: KAPCO's S.No moved 9→33; Kohinoor 12→35; Narowal 11→58 **and its name changed from `Narowal Energy Ltd. (HUBCO)` (FY2017-18…FY2021-22) to bare `Narowal Energy Ltd.` (FY2023-24)** — the parent tag was dropped, so a name join silently loses the subsidiary.
2. *"In which month did each plant stop running, and which are formally delicensed vs merely idle?"* — the `DELICENSED`/`DECOMMISSIONED` token occupies the same column position as January's load factor, and appears **only from FY2022-23 (11 DELICENSED + 1 DECOMMISSIONED) and FY2023-24 (12 + 1); zero occurrences FY2017-18…FY2021-22.** Separately, 2 rows (Reshma, Gulf Powergen) carry capacity with **no status and no monthly data at all** — silent absence.
3. *"How much fuel cost did NEPRA disallow, and is the pass-through actually complete?"* — see Group 5; the consolidated series stops in June 2022.

**Reconciliation trap, verified.** Summing FY2023-24: active 32-cell rows = **40,614 MW** installed; the 15 status/short rows add **4,061 MW** (3,880 MW of it delicensed/decommissioned) → 44,675 MW. NEPRA's own SIR 2024 states **45,888 MW** (incl. K-Electric) and **42,512 MW** (CPPA-G system). Four different capacity numbers for 30 June 2024 depending on artifact and status handling.

**Scope trap, verified.** K-Electric's own fleet is **entirely absent** from `Detail of Generation` — grepping FY2023-24 for `bin qasim|BQPS|K-Electric|Korangi|SITE` returns **zero hits**; only "Karachi Nuclear" (2 hits, PAEC-owned) matches. NEPRA says so itself in SIR 2024 p.88: *"These figures do not include KE's own generation and purchases from IPPs."* So the sheet is CPPA-G/XWDISCO only, while the FCA table has a K-Electric column — two systems, one CLI.

---

## GROUP 2 — Credit analysts

**PACRA's February 2025 power sector study** (HTTP 200, 1,212,143 B, 50 pages) is the artifact. It carries **26 distinct source attributions**; NEPRA appears in 10 of them and **almost never alone**: `NEPRA, SBP`, `NEPRA, PBS`, `Power Division, NEPRA`, `Ember, NEPRA`, `PPIB, NEPRA, NEECA, CPPA-G, PACRA Database`, `WB, Ember, SBP, NEPRA, PBS`. `PACRA Database` recurs as its own source.

That attribution pattern *is* the finding: a credit analyst's unit of work is a **join** across NEPRA + State Bank + PBS + Power Division + PPIB, and the reason "PACRA Database" exists as a citable source is that they had to build a local store because no single one works.

**Questions they cannot answer**: *"For each IPP, capacity payment entitlement against actual dispatch, indexed to the KIBOR and USD path in the same period"* — requires NEPRA plant-months × SBP rates × tariff determination terms, three sources, none joined.

---

## GROUP 3 — Energy journalists (Business Recorder, Profit/Pakistan Today, Dawn, Geo, The News)

**The ritual is annual and identical.** SIR drops (SIR 2024 in Oct 2024; SIR 2025 on Fri 16 Jan 2026) and within 48 hours the same six numbers appear across every outlet. Verbatim, from articles I fetched:

Business Recorder on SIR 2024 (HTTP 200, 267,644 B): *"electricity consumers ended up paying for 66.12% of unutilised capacity, which includes cost of intermittency in case of RE power plants"*; average annual utilisation **33.88%**; *"The T&D losses, recorded at 18.31% for FY2023-24 compared to the allowed 11.77%, added Rs276 billion to the circular debt"*; recovery **92.44%** → Rs314.51bn; receivables Rs2.32tn which *"necessitate investigations into possible billing manipulations."*

Profit, 16 Jan 2026 (HTTP 200, 125,729 B): T&D **17.55%** vs allowed **11.43%** → Rs265bn; recovery **96.62%** vs 100% → Rs132.46bn; circular debt Rs1.614tn, down from Rs2.393tn.

**The regulator itself names the problem.** Business Recorder quoting SIR 2025 verbatim: *"it cautioned that the energy sector had deep-rooted inefficiencies, inadequate planning, **lack of digitised and reliable data**, and weak governance was among other causes of concern."*

**Questions they cannot answer on deadline**
1. *"Which DISCO's loss gap widened most over five years, and what did it cost?"* — they only ever print the national average, because the per-DISCO panel is buried on p.63 of a 266-page PDF. **I answered it** (Group 4).
2. *"Is this year's T&D number comparable to last year's?"* — see the FY2023-24 structural break below. Nobody flagged it.
3. *"What did NEPRA quietly restate?"* — I found a silent restatement nobody reported (below).

**Why they don't**: the corpus is **640,312,915 B = 610.6 MB** across ten SIRs — SIR 2016 = 97.6 MB, SIR 2019 = **106.4 MB**, SIR 2025 = **331.9 MB**. I measured NEPRA's throughput at **68,768–163,650 B/s** (a 3.4 MB PDF timed out at 30 s having received 2.83 MB). **SIR 2025 alone takes 34–80 minutes to download.** That is why the five-year table on one page is where journalism stops.

---

## GROUP 4 — Think tanks and researchers (the series-rebuilders)

**Renewables First, "Pakistan Electricity Review 2026"** (HTTP 200, 1,404,572 B, 56 pages, PDF creationDate `D:20260524`). Authors Huma Naveed and Nabiya Imran. Foreword by Sohaib Malik, verbatim:

> *"This edition of the Pakistan Electricity Review 2026 (PER26) suggests, however, that the full extent of the shift is yet to be appreciated by most stakeholders, **largely because of the incomplete and imprecise datasets available to them.**"*

> *"they underline a critical gap in the country's official energy statistics"* … *"makes a significant contribution to the country's energy policy debate by **plugging critical data gaps**."*

Quantified: the report has **40 `Data source:` captions. 38 of 40 cite NEPRA. 40 of 40 say "RF's calculations."** A two-person team spends a publication cycle re-deriving what NEPRA already published, every year, from scratch.

**CDPR, "Power, Politics, and Profits in Pakistan's Electricity Sector"**, 28 August 2025 (HTTP 200, 96,641 B). The methodology paragraph is the cleanest statement of the pain I found anywhere:

> *"Yet in Pakistan, the law requires NEPRA to disclose these contracts and their amendments over hundreds of separate documents. Each PPA can be dozens of pages long, with many having multiple amendments which tweak the tariff, risk-sharing clauses, or key contractual definitions. **Automating this process proved difficult. We thus scoured each contract line by line, recording how capacity payments, generation tariffs, return on equity, exchange rate indexation, and other key variables** evolve over the contract's typical 25- to 30-year duration."*

**I answered the exemplar question.** SIR 2023 p.63 and SIR 2024 pp.88-89 carry, per DISCO: Actual (%), Allowed (%), and "Financial impact of units lost beyond target (Rs. Million)", on a rolling five-year window. Chaining them:

*Which DISCO's T&D gap widened most, FY2018-19 → FY2022-23, and what did it cost consumers?*

| DISCO | gap FY18-19 | gap FY22-23 | widening | 5y cost (Rs M) |
|---|---|---|---|---|
| **QESCO** | +5.02 pp | +12.45 pp | **+7.43 pp** | 69,534 |
| PESCO | +14.67 | +17.24 | +2.57 | **208,191** |
| MEPCO | +0.01 | +1.88 | +1.87 | 9,933 |
| LESCO | +1.44 | +3.29 | +1.85 | 49,169 |
| HESCO | +8.76 | +8.92 | +0.16 | 53,256 |
| IESCO | +0.25 | +0.26 | +0.01 | 996 |
| SEPCO | +17.59 | +17.34 | −0.25 | 62,830 |
| GEPCO | +0.14 | −0.49 | −0.63 | **−5,244** |
| FESCO | −0.44 | −0.25 | +0.19 | **−5,575** |
| **Total** | +1.98 | +4.75 | +2.77 | **442,944** |

Answer: **QESCO widened most (+7.43 pp), costing Rs 69.5 bn; PESCO cost the most in absolute terms, Rs 208.2 bn; the system total was Rs 442.9 bn over five years. FESCO and GEPCO beat their targets and saved money.** No NEPRA page states any of this.

**The overlapping windows reconcile — verified.** For all **8 of 8** DISCOs present in both, SIR 2023's FY2019-20…FY2022-23 values match SIR 2024's for the same years, exactly, on both `Actual (%)` and financial impact. So a chained multi-year panel is constructible without contradiction. That is the empirical licence for a local store.

**But a sibling table is silently restated.** The sales table is *not* consistent. FY2022-23, SIR 2023 p.62 vs SIR 2024 p.88:

| Category | SIR 2023 | SIR 2024 | restated |
|---|---|---|---|
| Agricultural | 9,639.68 | 9,543.14 | **−96.54** |
| Public Lighting | 459.59 | 521.28 | **+61.69** |
| Others → *General Services/Others* | 4,834.72 | 4,880.32 | +45.60 (**renamed**) |
| Domestic | 53,522.91 | 53,534.20 | +11.29 |
| Bulk Supply | 4,454.68 | 4,443.93 | −10.75 |
| **Total** | 112,891.20 | 112,902.49 | +11.29 |

Some NEPRA tables reconcile across vintages and some are quietly rewritten — and only a store that keeps *both vintages* can tell you which.

**And SIR 2023's growth column does not reconcile to its own units.** Recomputing every growth % from the units printed in the same row: **8 of 8 categories mismatch.** Public Lighting is stated as **−44.37%** when NEPRA's own units give **−30.73%** — off by 13.64 pp. Domestic FY2022-23 is stated −12.87% vs −11.40% computed. The identical table in SIR 2024 reconciles to **0.00 pp on 8 of 8**. So SIR 2023's growth row is defective, and anyone who quoted "domestic consumption fell 12.87%" quoted a number NEPRA's own units contradict.

**The FY2023-24 structural break, which nobody flagged.** From SIR 2024:

| DISCO | actual FY22-23 → FY23-24 | Δ | allowed Δ | fin. impact (Rs M) |
|---|---|---|---|---|
| LESCO | 11.29 → 15.92 | **+4.63** | 8.00 → 10.00 (**+2.00**) | 21,785 → 47,635 |
| QESCO | 26.72 → 29.77 | +3.05 | −0.23 | 21,214 → 36,746 |
| GEPCO | 8.61 → 11.54 | +2.93 | −0.10 | **−1,856 → +9,217** |
| FESCO | 8.59 → 9.86 | +1.27 | −0.10 | **−1,449 → +5,040** |
| MEPCO | 14.22 → 15.28 | +1.06 | −0.51 | 7,945 → 22,657 |
| **Total** | 16.45 → 18.31 | +1.86 | +0.07 | **160,486 → 276,349** (+72%) |

Every DISCO's reported loss rose in one year; two flipped from *beating* their target to costing billions; the national bill rose 72%. NEPRA publishes no note explaining whether this is real deterioration or a definition change.

---

## GROUP 5 — Regulatory/policy watchers and ordinary consumers/businesses

**The consolidated FCA table is frozen.** `FCA (2018-2022)_files/sheet001.htm` (HTTP 200, 79,843 B, 53 `<tr>`, 572 `<td>`) holds exactly **48 months, Jul-2018 → Jun-2022**, with four rate columns: CPPA-G Requested, CPPA-G Allowed, K-Electric Requested, K-Electric Allowed, plus fortnight notification dates. Last row: `2022 | Jun | 9.9095 | 9.8972 | 11.389 | 11.1023`. It has not been extended in four years — through the largest tariff shock in Pakistan's history.

**Derived from it (nowhere on NEPRA):** CPPA-G requested **81.9748** Rs/kWh cumulative vs allowed **73.1323** → **8.8425 Rs/kWh disallowed**, with allowed below requested in **45 of 48 months**. K-Electric: 54.7450 vs 46.2369 → **8.5081** disallowed, below in 34 of 48. Worst year for disallowance: 2020 (requested 8.3106, allowed 4.4412, gap 3.8694).

**The series continues — in scattered scanned PDFs.** June 2026 decision (`.../Ex-WAPDA DISCOS/2026/TRF-100 XWDISCOS FCA JUN 2026 07-08-2026 17536-54.pdf`, HTTP 200, **793,122 B**, 13 pages), verbatim:

Request (CPPA-G letter 14 July 2026): Actual `Rs.8.9138/kWh` − Reference `Rs.7.7138/kWh` = FCA `Rs.1.2000/kWh`.
Order: *"The Authority has decided to approve the positive FCA June 2026, as follows;"* Actual `Rs.8.4641/kWh` − Reference `Rs.7.7138/kWh` = FCA **`Rs.0.7503/kWh`**.
→ **Rs.0.4497/kWh disallowed**, exactly the requested-vs-allowed structure NEPRA stopped tabulating in June 2022.

**The consumer-facing fact, verbatim:** *"XWDISCOs and KE shall reflect the FCA in respect of June 2026 in the billing month of **August 2026**."* July's decision: *"in the billing month of September 2026."* A systematic **two-month lag**, plus exclusions: *"except lifeline consumers, Electric Vehicle Charging Stations (EVCS) and pre-paid electricity consumers."*

**Questions a business owner cannot answer**
1. *"The FCA line on my August bill — which month is it, what did NEPRA cut from CPPA-G's ask, and what will hit my next two bills?"* — requires reading three scanned PDFs whose filenames follow no pattern.
2. *"What have I paid in FCA cumulatively over 24 months, versus what was requested?"* — no table exists past Jun-2022.
3. *"Which tariff schedule legally applies to me?"* — see below.

**The tariff SRO is a 100-page image with no text.** `S.R.O. 41(1)2026 dated 13.01.2026` (HTTP 200, 3,078,192 B): **100 pages, 100 embedded images, 14,900 characters of extractable text = 149 chars/page — and every character of it is a repeated signature stamp**: `"Wasim Anwar Bhinder / Registrar-1 / Wednesday, 14 January, 2026, 11:59:13 AM"`, ×100. Zero machine-readable tariff numbers. It is one of **13 near-identical SROs all dated 13.01.2026**, and you cannot even determine which DISCO it governs without OCR.

**The quarterly-adjustment table is also frozen.** `Quarterly Data (XWD & KE)_files/sheet001.htm` (HTTP 200, 79,672 B, 168 `<tr>`) stops at "April 2022- June 2022 (4th Quarter)" with `Allowed By NEPRA` = **"Not Yet Issued"** for every DISCO. `Hydel Data_files/sheet001.htm` (14,783 B, 33 `<tr>`) pins every WAPDA hydel dependable capacity to a single period, `"July 2018 - June 2022"`.

---

## TOP POWER-USER WORKFLOWS, ranked by value

### 1. `plant-panel` — the IPP load-factor and availability panel (highest value)
**Question**: "Show HUBC's consolidated fleet load factor FY18→FY24, flag the month each plant went idle, and separate delicensing from idleness."
**Local data required**: 9,180 plant-months (7 FYs × ~118 plants × 12) with columns `(fy, plant_id, name_as_published, technology, fuel, installed_mw, dependable_mw, month, load_factor_pct, gwh, status_enum)`, **plus a curated `plant_id ↔ published-name ↔ ticker` crosswalk with validity dates**, because S.No and the name string both change (KAPCO 9→33, Narowal 11→58 and `(HUBCO)` dropped). Serves equity + credit + think tanks. Needs a JOIN no NEPRA page performs: seven separate annual HTML sheets, no shared key.

### 2. `disco-gap` — the T&D-loss / recovery panel against NEPRA's own targets
**Question**: "Rank DISCOs by five-year widening of actual-vs-allowed T&D loss and the cumulative Rs cost; flag the FY2023-24 break."
**Local data required**: `(fy, disco, actual_pct, allowed_pct, financial_impact_rs_m, recovery_ratio_pct, amount_not_recovered_rs_m, source_report, source_page)` chained from overlapping five-year windows across SIR PDFs — **and the `source_report` vintage kept, so restatements are visible rather than overwritten**. This is the workflow that most repays a store: 610.6 MB of PDFs at 68–164 KB/s becomes one indexed query, and the answer (QESCO +7.43 pp / Rs 69.5 bn; PESCO Rs 208.2 bn; total Rs 442.9 bn) exists on no page.

### 3. `fca` — the fuel-cost-adjustment series, requested vs allowed, with billing month
**Question**: "Monthly FCA requested vs allowed for CPPA-G and KE since Jul-2018, the cumulative disallowance, and which consumer bill each lands on."
**Local data required**: the 48 tabulated months (Jul-2018→Jun-2022) **unioned with** rates extracted from each monthly decision PDF from Jul-2022 forward, as `(period_month, entity, actual_rs_kwh, reference_rs_kwh, fca_requested, fca_allowed, decision_date, billing_month, excluded_categories, is_review_decision, source_url)`. Two-source union + a derived `disallowance` column + the two-month `billing_month` mapping. Serves journalists, policy watchers and every consumer.

### 4. `capacity-reconcile` — capacity by status, with the four-number reconciliation
**Question**: "What was installed capacity on 30 June 2024 — and why do NEPRA's own artifacts say 40,614 / 42,512 / 44,675 / 45,888 MW?"
**Local data required**: the plant table with a `status` enum (`active | delicensed | decommissioned | listed_no_data`) and a `system` enum (`cppag | k_electric | excluded`), so each headline number is *derivable and labelled* rather than guessed. Directly serves the "surplus capacity / capacity payment" debate that dominates coverage.

### 5. `tariff-stack` — what a consumer actually pays, assembled
**Question**: "For a LESCO industrial connection in August 2026, which SRO schedule applies, plus which month's FCA, plus which quarterly adjustment, plus any court order?"
**Local data required**: `(disco, effective_from, sro_number, sro_date, source_url)` × `(billing_month → fca_period, fca_rate, excluded_categories)` × `(quarter, disco, qta_status)`. A four-way join across three document families and one frozen HTML table. Lowest analytic sophistication, largest audience.

### 6. `qa` / `diff` — the integrity layer that makes 1–5 trustworthy
Not a user question but the reason to trust the store: **78 of 9,180 plant-months report GWh > 0 with load factor 0.00**; **4 of 765 plant-years have a stated annual sum off by >1% from their own twelve monthly values** (UCH-II FY2021-22 stated 2,833.10 vs 2,799.32 computed); SIR 2023's growth column fails 8/8; the FY2022-23 sales row was restated across vintages. A `diff <vintage> <vintage>` command surfaces exactly the story journalists are missing.

### VERIFIED
- Generation panel is buildable: 7 fiscal-year sheets (FY2017-18..FY2023-24) parsed into 765 plant-years = 9,180 plant-months. Canonical rows/year: 96, 101, 106, 104, 124, 116, 118. Byte sizes 418,887 / 421,591 / 426,282 / 455,640 / 490,461 / 493,187 / 516,219 B, all HTTP 200.
- HUBCO's 1,292 MW Hub base plant collapsed and exited: annual load factor 49.24% (FY2017-18) -> 7.86 -> 0.33 -> 2.07 -> 11.52 -> 1.93 (FY2022-23) -> DELICENSED (FY2023-24). GWh 5,205.94 -> 203.00 = -96.1%. FY2023-24 row verbatim: [34][ ][Hub Power Company (HUBCO)][THERMAL][RFO][1,292][1,200][DELICENSED], n=8 cells vs n=33 the prior year.
- KAPCO generated exactly 0.00 GWh in all twelve months of FY2023-24 while still listed at 1,601 MW installed / 1,345 MW dependable. Verbatim: [33][ ][Kot Addu Power Company (KAPCO)][THERMAL][RFO/RLNG/HSD][1,601][1,345][0.00][0.00]... (24 zeros).
- There is NO stable primary key for a plant across NEPRA's annual sheets. S.No drifts (KAPCO 9->33, Kohinoor Energy 12->35, Narowal 11->58) AND the name string changes: 'Narowal Energy Ltd. (HUBCO)' in FY2017-18..FY2021-22 becomes bare 'Narowal Energy Ltd.' in FY2023-24 - the parent tag is dropped, so a name join silently loses the HUBCO subsidiary.
- DELICENSED/DECOMMISSIONED status tokens appear ONLY from FY2022-23 (11 DELICENSED + 1 DECOMMISSIONED) and FY2023-24 (12 + 1). Zero occurrences in FY2017-18 through FY2021-22 (grepped byte-safely in all seven files).
- K-Electric's own generation fleet is entirely absent from Detail of Generation. Grepping FY2023-24 for 'bin qasim|BQPS|K-Electric|Korangi|SITE' returns ZERO hits; only 'Karachi Nuclear' (2 hits) matches. NEPRA confirms in SIR 2024 p.88 verbatim: 'These figures do not include KE's own generation and purchases from IPPs.'
- Four incompatible capacity numbers for 30 June 2024: parsed active rows 40,614 MW; plus 15 status/short rows at 4,061 MW = 44,675 MW; NEPRA SIR 2024 states 42,512 MW (CPPA-G system) and 45,888 MW (including K-Electric).
- The consolidated FCA table covers exactly 48 months, Jul-2018 to Jun-2022, and has not been extended in four years. HTTP 200, 79,843 B, 53 <tr>, 572 <td>. Last row: 2022 | Jun | 9.9095 | 9.8972 | 11.389 | 11.1023.
- Derived from that table (stated nowhere on NEPRA): CPPA-G requested 81.9748 Rs/kWh cumulative vs allowed 73.1323 -> 8.8425 Rs/kWh disallowed, with allowed below requested in 45 of 48 months. K-Electric: 54.7450 requested vs 46.2369 allowed -> 8.5081 disallowed, below in 34 of 48. Worst year 2020: requested 8.3106, allowed 4.4412, gap 3.8694.
- The FCA series continues in scattered scanned PDFs. June 2026 decision (HTTP 200, 793,122 B, 13 pages): CPPA-G requested actual Rs.8.9138/kWh - reference Rs.7.7138/kWh = FCA Rs.1.2000/kWh; NEPRA's Order approved actual Rs.8.4641/kWh - reference Rs.7.7138/kWh = FCA Rs.0.7503/kWh. Disallowance Rs.0.4497/kWh.
- Systematic two-month billing lag, verbatim: 'XWDISCOs and KE shall reflect the FCA in respect of June 2026 in the billing month of August 2026' and, in the July decision, 'in the billing month of September 2026'. Exclusions verbatim: 'except lifeline consumers, Electric Vehicle Charging Stations (EVCS) and pre-paid electricity consumers'.
- NEPRA's server is CASE-SENSITIVE on the file extension. The June 2026 FCA file returns 200 (793,122 B) as .pdf and 404 as .PDF; the July 2026 file returns 200 (630,229 B) as .PDF and 404 as .pdf. Extension casing cannot be normalised or guessed. Site-wide the homepage links 154 .pdf and 81 .PDF.
- The consumer tariff SRO is a scanned image with no usable text layer. S.R.O. 41(1)2026 dated 13.01.2026 (HTTP 200, 3,078,192 B): 100 pages, 100 embedded images, 14,900 total extractable characters = 149/page, and every character is the repeated stamp 'Wasim Anwar Bhinder / Registrar-1 / Wednesday, 14 January, 2026, 11:59:13 AM'. Zero machine-readable tariff numbers. It is one of 13 near-identical SROs all dated 13.01.2026.
- SIR PDF corpus is 640,312,915 B = 610.6 MB across ten reports: 2016=97.6MB, 2017=7.2, 2018=26.5, 2019=101.5MB, 2020=40.6, 2021=7.2, 2022=5.9, 2023=3.3, 2024=8.9, 2025=316.5MB (HEAD-verified Content-Length, all HTTP 200).
- Measured NEPRA throughput 68,768-163,650 B/s: a 3.4 MB PDF timed out at 30s with 2,831,593 of 3,425,763 B received; a 2 MB ranged slice ran at 68,768 B/s. SIR 2025 alone therefore takes 34-80 minutes; the full corpus 65-155 minutes.
- The per-DISCO T&D panel exists and I extracted it: SIR 2023 p.63 and SIR 2024 pp.88-89 give Actual (%), Allowed (%) and 'Financial impact of units lost beyond target (Rs. Million)' per DISCO on rolling five-year windows.
- Exemplar question answered from real data (FY2018-19 to FY2022-23): QESCO's gap widened most, +5.02pp -> +12.45pp = +7.43pp, costing Rs 69,534 M. PESCO cost the most absolutely, Rs 208,191 M (gap +14.67 to +17.24pp). System total gap +1.98 -> +4.75pp, cumulative Rs 442,944 M. FESCO (-Rs 5,575 M) and GEPCO (-Rs 5,244 M) beat their targets.
- Overlapping SIR windows reconcile EXACTLY for the T&D table: for all 8 of 8 DISCOs present in both reports, SIR 2023's FY2019-20..FY2022-23 values equal SIR 2024's for the same years, on both actual% and financial impact. A chained multi-year panel is therefore constructible without contradiction.
- But the sales table IS silently restated. FY2022-23 as printed in SIR 2023 vs SIR 2024: Agricultural 9,639.68 -> 9,543.14 (-96.54); Public Lighting 459.59 -> 521.28 (+61.69); 'Others' renamed 'General Services/Others' 4,834.72 -> 4,880.32 (+45.60); Domestic 53,522.91 -> 53,534.20 (+11.29); Bulk Supply 4,454.68 -> 4,443.93 (-10.75); Total 112,891.20 -> 112,902.49 (+11.29).
- SIR 2023's growth column does not reconcile to its own units: 8 of 8 categories mismatch by >0.05pp. Public Lighting stated -44.37% vs -30.73% computed (13.64pp error); Domestic FY2022-23 stated -12.87% vs -11.40%. The same table in SIR 2024 reconciles to 0.00pp on 8 of 8, proving SIR 2023's row is defective rather than differently-based.
- FY2023-24 is a structural break in the reported loss series: LESCO actual 11.29->15.92 (+4.63pp) while its allowed target was RAISED 8.00->10.00; QESCO +3.05pp; GEPCO +2.93pp with financial impact flipping -1,856 -> +9,217; FESCO -1,449 -> +5,040; system total 16.45->18.31 (+1.86pp) and Rs 160,486M -> Rs 276,349M, a 72% one-year rise. NEPRA publishes no explanatory note.
- Internal-consistency defects quantified: 78 of 9,180 plant-months report GWh > 0 with load factor 0.00 (e.g. Neelum Jhelum FY2017-18 month 3: LF 0.00, 4.44 GWh); 4 of 765 plant-years have a stated annual sum off by >1% from the sum of their own twelve monthly values (UCH-II FY2021-22 stated 2,833.10 vs 2,799.32 computed, -33.78).
- Renewables First 'Pakistan Electricity Review 2026' (HTTP 200, 1,404,572 B, 56 pages, creationDate D:20260524) contains 40 'Data source:' captions: 38 of 40 cite NEPRA and 40 of 40 say "RF's calculations". Foreword verbatim: 'the full extent of the shift is yet to be appreciated by most stakeholders, largely because of the incomplete and imprecise datasets available to them' and 'plugging critical data gaps'.
- CDPR, 28 August 2025 (HTTP 200, 96,641 B), verbatim: 'the law requires NEPRA to disclose these contracts and their amendments over hundreds of separate documents. Each PPA can be dozens of pages long... Automating this process proved difficult. We thus scoured each contract line by line, recording how capacity payments, generation tariffs, return on equity, exchange rate indexation, and other key variables evolve over the contract's typical 25- to 30-year duration.'
- The regulator itself names the problem. Business Recorder quoting SIR 2025 verbatim (HTTP 200, 268,700 B): 'it cautioned that the energy sector had deep-rooted inefficiencies, inadequate planning, lack of digitised and reliable data, and weak governance was among other causes of concern.'
- Journalist headline numbers confirmed verbatim. SIR 2024 via Business Recorder: 'electricity consumers ended up paying for 66.12% of unutilised capacity'; utilisation 33.88%; 'T&D losses, recorded at 18.31% for FY2023-24 compared to the allowed 11.77%, added Rs276 billion'; recovery 92.44% -> Rs314.51bn; receivables Rs2.32tn. SIR 2025 via Profit 16 Jan 2026 (125,729 B): T&D 17.55% vs 11.43% -> Rs265bn; recovery 96.62% -> Rs132.46bn; circular debt Rs1.614tn from Rs2.393tn.
- PACRA's Feb 2025 power sector study (HTTP 200, 1,212,143 B, 50 pages) carries 26 distinct source attributions; NEPRA appears in 10 and almost never alone - 'NEPRA, SBP', 'NEPRA, PBS', 'Power Division, NEPRA', 'PPIB, NEPRA, NEECA, CPPA-G, PACRA Database'. 'PACRA Database' recurs as its own citable source: they built a local store because no single source works.
- The Quarterly and Hydel tables are frozen too. Quarterly Data (XWD & KE) sheet001.htm (HTTP 200, 79,672 B, 168 <tr>) stops at 'April 2022- June 2022 (4th Quarter)' with 'Allowed By NEPRA' = 'Not Yet Issued' for every DISCO. Hydel Data sheet001.htm (14,783 B, 33 <tr>) pins every WAPDA hydel dependable capacity to the single period 'July 2018 - June 2022'.
- The FY2017-18..FY2021-22 index (Main.htm, HTTP 200, 12,210 B) links only 5 of the 7 reachable generation years plus 4 sub-pages - confirmed incomplete, exactly as the brief stated.

### REFUTED/UNVERIFIED
- REFUTED: all six DISCO Performance Evaluation Report PDF paths listed in the brief return HTTP 404 (9-byte body), including /publications/M&E/PER/Distribution/PER DISCOs 2023-24.pdf, /publications/Standards/2020/PER DISCOs 2018-19.pdf, /publications/Standards/2021/PER DISCOs 2019-20 updated.pdf, /publications/Standards/2023/PER-DISCO FY 2021-22 final.pdf, /publications/Standards/PER DISCOs and KE for 2014-15.pdf and an /M&E/PER/ variant. I found no working PER path. The per-DISCO panel I did extract came from the SIR PDFs instead, not from PER.
- REFUTED at the stated path: 'SIR Data 2024.htm' and 'SIR Data 2025.htm' both returned HTTP 404 (9 B) under /publications/State of Industry Reports/. The brief records them as 200. Either the path differs or they moved. Their shape remains UNVERIFIED.
- CORRECTED path, not refuted: 'FCA (2018-2022).htm', 'Quarterly Data (XWD & KE).htm', 'Notification/SRO Data Webpage.htm' and 'Hydel 1/Hydel Data.htm' all 404 under the SIR root but return 200 under /publications/State of Industry Reports/Detail of Generation/. The directory itself returns 403 (no listing).
- UNVERIFIED: TESCO's five-year T&D financial impact. Only 1 of 5 values (-164) was extractable before the adjacent recovery-ratio table broke my parse, and I could not confirm which year it belongs to. TESCO's 5-year cost must be recorded as missing, not -164. My earlier draft ranking wrongly showed it.
- UNVERIFIED: IESCO's and TESCO's FY2023-24 T&D values. They sit on SIR 2024 p.88, a different page from the other seven DISCOs (p.89), and I did not parse their numbers. Their absence from my FY2023-24 break table is a parse gap, not a publication gap.
- UNVERIFIED (blocked): Dawn.com. https://www.dawn.com/news/1967418 returned HTTP 403 (5,652 B) to plain curl with a browser UA. I have no Dawn quote and did not substitute one.
- UNVERIFIED: SIR 2016 (97,607,884 B) and SIR 2019 (106,428,020 B) contents. I HEAD-verified their sizes only; both timed out on download at 30s and I did not resume them. Whether they carry the same per-DISCO table shape is unknown, so a pre-FY2018-19 panel is unproven.
- UNVERIFIED: SIR 2025 (331,853,390 B) contents. Never downloaded - at the measured 68.8-163.7 KB/s it needs 34-80 minutes. All my SIR 2025 numbers (17.55% vs 11.43%, Rs265bn, Rs132.46bn, Rs397bn, 41,121 MW, 38.82% utilisation, 'lack of digitised and reliable data') are journalists' quotations of it, NOT read from the PDF itself. They must be treated as second-hand until the PDF is parsed.
- UNVERIFIED and a live collision risk: PSX ticker assignments. NEPRA's sheet writes 'Kohinoor Energy Limited. (KEL)'. The task brief lists KEL among power stocks, where market convention treats KEL as K-Electric Limited. I did not verify any PSX symbol against an exchange source, so the NEPRA-abbreviation-to-ticker map is unproven - and NEPRA's own '(KEL)' for Kohinoor is a trap regardless.
- NOT FOUND: I did not locate any forum thread, X/Twitter complaint or Reddit post about NEPRA data access, despite it being requested. The pain evidence I have is all from published research (CDPR, Renewables First, PACRA) and the regulator's own SIR text. Recording the absence rather than inventing a quote.
- UNKNOWN, and NEPRA offers no note: whether the FY2023-24 jump in reported T&D losses (system 16.45% -> 18.31%, every DISCO up) is genuine deterioration, a change in loss definition, or a change in the reporting basis. Do not attribute a cause.
- UNVERIFIED: FY2016-17, FY2024-25 and FY2025-26 generation sheets. I did not re-probe them; the brief records 404 and I have no contrary evidence. The panel is therefore 7 years, not 10.
- PARTIAL: my FY2023-24 installed-capacity sum of 40,614 MW is a product of my own parser (98 rows at 32 cells, 16 at 33, 4 at 34). It is reproducible but is my derivation, not a NEPRA-published figure - do not quote it as NEPRA's number.
- UNVERIFIED: 'Reshma Power Generation' and 'Gulf Powergen' appear in FY2023-24 with capacity (97 MW, 84 MW) but no status token and no monthly data. Whether that means idle, delicensed or an omission is not stated anywhere I could find.

### BUILD IMPLICATIONS
- Make the plant crosswalk a first-class, versioned table in the store - not a regex. S.No drifts (KAPCO 9->33, Narowal 11->58) and the published name mutates ('Narowal Energy Ltd. (HUBCO)' -> 'Narowal Energy Ltd.'), so `plant_id` must be curated with validity dates and both the ticker map and every historical name string retained. Without it the flagship command (consolidated fleet load factor) silently drops subsidiaries.
- Store a `status` enum (active | delicensed | decommissioned | listed_no_data) as a real column, because the status token occupies the same cell position as January's load factor. Never coerce it to a number: HUBCO FY2023-24 would become 0% load factor rather than 'exited'.
- Parse by anchoring on the technology token, never on fixed column offsets. In FY2023-24 only 98 of 134 candidate rows carry the canonical 32 cells - 16 carry 33, 4 carry 34 (names split across cells by Excel line wrapping), and 16 are 6-9 cell status rows. A fixed-offset reader is wrong on ~27% of rows.
- Store a `system` enum (cppag | k_electric) and surface it in every output. K-Electric's fleet is absent from Detail of Generation but present as a column in the FCA table. A command that sums the generation sheet and calls it 'Pakistan' is wrong, and a `kel`-style ticker lookup must return an explicit 'not in this dataset' rather than empty.
- Keep document VINTAGE, not just values: `(fy, disco, metric, value, source_report, source_page)`. The T&D table reconciles 8/8 across SIR 2023 and SIR 2024, but the sales table was silently restated (Agricultural -96.54 GWh, Public Lighting +61.69, a category renamed). A store that upserts on (fy, disco) destroys the single most publishable finding. Ship a `diff <vintage> <vintage>` command.
- Recompute every published ratio and store both the stated and the derived value with a mismatch flag. SIR 2023's growth column fails to reconcile on 8 of 8 categories (Public Lighting stated -44.37% vs -30.73% computed) while SIR 2024's reconciles perfectly. 78 of 9,180 plant-months state GWh>0 with load factor 0.00. These flags ARE the product for journalists.
- Capture the byte-exact URL from the index; never construct or normalise one. Extension casing is load-bearing: the same June 2026 FCA file is 200 as .pdf and 404 as .PDF, and the reverse holds for July. Directory names are inconsistent too ('2026/06 June' amid '2026/01 Jan', '2026/03 Mar'). Crawl and record; do not template.
- Do not treat the site index as the enumerator, and do not treat a 404 as absence. Main.htm links only 5 of 7 reachable generation years; the SIR root 404s four pages that return 200 one directory down; all six PER paths in the brief are dead. The CLI needs a `refresh` that crawls month directories and stores discovered URLs, plus an `unreachable` state distinct from 'no data'.
- Budget for NEPRA's throughput as a design constraint, not an edge case. Measured 68.8-163.7 KB/s means SIR 2025 (331.9 MB) takes 34-80 minutes and the 610.6 MB corpus takes 1-2.6 hours. Fetching must be resumable (HTTP 206 works; a 3.4 MB file needed two 30s slices, SIR 2024 needed five), incremental, and cached forever - which is precisely why this is a local store rather than a bookmark.
- Plan for OCR as a data-quality layer, not a parsing step. Monthly FCA decisions are scanned: across two documents I counted five spellings of one entity (XWDISCOs 41x, XWDlSCOs 4x, XWDISCO 4x, XWDJSCOs 1x, XWDISCQs 1x) plus 'Rs.l.2000/kWh' with a letter l for the digit 1, 'consumers of FCE' for KE, and 'Kl' for KE. Cross-check every extracted rate against the arithmetic identity actual - reference = adjustment, and refuse to store a rate that fails it.
- Model the FCA as four fields plus a lag, because that is the user's actual question: (period_month, entity, actual, reference, requested, allowed, decision_date, billing_month, excluded_categories, is_review_decision). The two-month lag is systematic (June FCA -> August bill) and March 2026 has BOTH an original and a review decision - so `is_review_decision` decides whether a consumer gets the right answer.
- Union the frozen table with the PDF-derived series under one command. The 48 tabulated months (Jul-2018..Jun-2022) and everything after it are the same series from two source families; the whole value of `fca` is that the user never learns NEPRA stopped tabulating in June 2022. Expose `source_kind` for auditability, not as a choice the user must make.
- Handle two negative-number conventions in one document and validate ranges loosely. The T&D table writes negatives in parentheses ((1,032)) while the adjacent recovery table uses minus signs (-3,300.10); there are 50 parenthetical negatives in the FCA table alone. And recovery ratios legitimately exceed 100% (IESCO 116.87%, TESCO 105.66%) with negative 'amount not recovered' - a 0-100 validator would silently reject real observations.
- Segment tables before parsing, and strip page furniture. SIR 2023 p.63 carries the T&D table AND the recovery table with identical DISCO row headers - blending them is what corrupted my TESCO row. Printed page numbers ('49') and section numbers ('5.5') land inside numeric columns, and SIR 2024's loss table spans pp.88-89 with 'Total' printed on BOTH pages. Cut on table markers, not pages, and de-duplicate the Total row.
- Never rely on row order across reports. SIR 2023's DISCO order and SIR 2024's differ, and IESCO/TESCO moved to a separate page in SIR 2024. Also plan for a changing roster: the FY2023-24 table omits DISCOs the FY2022-23 one included, so a missing DISCO-year must be storable as a publication gap distinct from zero.
- Regex on report titles will break. 'STATE OF INDUSTRY REPORT 2023' became 'STATE OF THE INDUSTRY REPORT 2024', which also carries the typo 'SUPPY SECTOR'. Match loosely on the table's own column structure (a DISCO name followed by Actual/Allowed/Financial impact) rather than on headers or section titles.
- The 100-page image-only tariff SRO sets a hard boundary: with 149 chars/page and every character a signature stamp, `tariff-stack` can honestly return the SRO's identity, date, DISCO and URL plus the FCA and quarterly components, but must NOT claim to return schedule rates. Say 'schedule not machine-readable, open the PDF' rather than emitting a guessed number - and note there are 13 near-identical SROs all dated 13.01.2026, so DISCO attribution needs an external mapping.
- Output format should serve a join, not a page. Every high-value question found here (fleet load factor across renamed subsidiaries; DISCO gap widening ranked with Rs cost; requested-vs-allowed FCA with cumulative disallowance; capacity reconciliation across four definitions) is a GROUP BY or a multi-source union. Default to tidy long-form rows with explicit fy/entity/metric/value/source columns so the answers compose.

### TRAPS
- Encoding: the generation sheets are windows-1252, not UTF-8. A naive UTF-8 grep finds zero tags. Confirmed by reading all seven byte-safely (FY2023-24: 1 <table>, 139 <tr>, 4,965 <td>).
- URL case sensitivity on the extension: the identical June 2026 FCA path returns HTTP 200 (793,122 B) as .pdf and HTTP 404 as .PDF; July 2026 returns 200 (630,229 B) as .PDF and 404 as .pdf. Site-wide 154 .pdf vs 81 .PDF. Casing cannot be guessed or normalised.
- Status token in a numeric column: 'DELICENSED' / 'DECOMMISSIONED' sits where January's load factor belongs. A permissive numeric parse turns HUBCO's exit into 0% load factor; a strict one silently drops the row.
- No stable key across years: S.No drifts (KAPCO 9->33, Kohinoor 12->35, Narowal 11->58) and the name mutates ('Narowal Energy Ltd. (HUBCO)' -> 'Narowal Energy Ltd.'). Joining on either loses exactly the subsidiary an equity analyst is modelling.
- Variable cell counts: FY2023-24 has 98 rows at 32 cells, 16 at 33, 4 at 34 and 16 at 6-9 cells. Excel line-wrapping splits names into extra <td>s ('Tarbela' + 'Ext. 04 Hydropower Project (WAPDA)'; 'Jamshoro Power Generation' + 'Company Limited.'), shifting every downstream offset.
- Accounting negatives in parentheses: 50 of them in the FCA table ('(1.798)'), and the T&D financial-impact column uses the same convention ('(1,032)'). A naive float parse flips the sign on real savings.
- Two negative conventions in ONE document: the T&D table uses parentheses while the adjacent recovery table uses minus signs (-3,300.10). A single parser tuned to one convention mis-signs the other.
- Two tables share one page with identical row headers: SIR 2023 p.63 holds the T&D loss table AND the recovery-ratio table, both keyed on DISCO names. This is what corrupted my own TESCO row - a real, self-inflicted demonstration.
- Page furniture bleeds into numeric columns: the printed page number '49' and the section number '5.5' were captured as data values in the financial-impact rows.
- One logical table split across PDF pages with 'Total' printed twice: SIR 2024's loss table spans pp.88-89, IESCO/TESCO on 88 and the other seven on 89, with a Total row on each. Reading one page gives a partial roster; concatenating both double-counts Total.
- Row order and roster change between reports: SIR 2023's DISCO order differs from SIR 2024's, and DISCOs present in one year's table are absent from the next. Positional parsing and 'missing means zero' both fail.
- Report titles and headings mutate: 'STATE OF INDUSTRY REPORT 2023' -> 'STATE OF THE INDUSTRY REPORT 2024', which also carries the typo 'SUPPY SECTOR'. Header regexes break annually.
- Silent restatement across vintages: FY2022-23 sales were rewritten between SIR 2023 and SIR 2024 (Agricultural -96.54 GWh, Public Lighting +61.69, Total +11.29) and a category was renamed 'Others' -> 'General Services/Others'. An upsert-on-key store destroys the evidence.
- Published ratios that contradict their own units: SIR 2023's growth column fails to reconcile on 8 of 8 categories (Public Lighting stated -44.37% vs -30.73%). The same table in SIR 2024 reconciles exactly, so the defect is year-specific and undetectable without recomputation.
- Internally contradictory plant-months: 78 of 9,180 report GWh > 0 with load factor 0.00; 4 of 765 plant-years have a stated annual sum off by >1% from their own twelve monthly values (UCH-II FY2021-22 stated 2,833.10 vs 2,799.32).
- Wrong years in a date column: eight 2021 rows of the FCA table carry '22' in the 2nd-fortnight date ('16-09-22' for Sep-2021, '16-07-22' for Jul-2021, etc.). Date parsing succeeds and produces future dates.
- OCR corruption in the live FCA decisions: five spellings of one entity across two documents (XWDISCOs 41x, XWDlSCOs 4x, XWDISCO 4x, XWDJSCOs 1x, XWDISCQs 1x), plus 'Rs.l.2000/kWh' (letter l for digit 1), 'consumers of FCE' where KE is meant, 'Kl' for KE, 'MATrER', 'Ibr'/'far' for 'for'. A number silently becomes non-numeric.
- Two decisions for one month: March 2026 has both an original FCA decision and a separate 'REVIEW DECISION IN MATTER OF FCA FOR XWDISCOs MARCH 26'. Picking 'the' March FCA is a coin flip without an is_review flag.
- Filename chaos with no pattern for the same monthly series: 'TRF-100 MFPA XWDISCOS FCA DEC-2025...', 'TRF-100 MFPA FCA FEB-2026...' (XWDISCOS dropped), 'TRF-100 XWDISCOS FCA JUN 2026...' (MFPA dropped), 'TRF-100 MFPA FOR THE MONTH OF JULY 2026 EX-WAPDA DISCOS...' (the word FCA dropped entirely), and 'FCA - Jun 26 -Sent to NEPRA - 14-Jul-2026.pdf'. Sibling series are just as bad: 'XWDISCOS ENERGY PURCHASE DATA FOR MONTH OF JANUARY 2026' vs '...FOR THE MONTH OF MARCH 2026' vs 'REVISED XWDISCOS...' vs '(CPPA) Energy Procurement Report (Provisional) for the month of March.pdf' (no year) vs 'Quarterly Adjustment of DISCOs.pdf' (no period at all).
- Inconsistent directory naming: '2026/06 June' spelled out amid '2026/01 Jan', '2026/03 Mar', '2026/09 Sep' abbreviated.
- Image-only PDFs that pass a text-extraction smoke test: S.R.O. 41(1)2026 yields 14,900 characters across 100 pages - enough that 'did we get text?' returns yes - but every character is a repeated registrar signature stamp and zero tariff numbers are present.
- Scope trap: the generation sheet excludes K-Electric's own fleet entirely (zero hits for BQPS/Korangi/SITE) while the FCA table has a K-Electric column. Summing the sheet and calling it national capacity is wrong, and a KEL lookup finds nothing.
- Four incompatible capacity definitions for one date (40,614 / 42,512 / 44,675 / 45,888 MW for 30 June 2024) depending on artifact and status handling. Any single 'installed capacity' number is unfalsifiable without a labelled definition.
- Abbreviation collision: NEPRA writes 'Kohinoor Energy Limited. (KEL)' while market convention reads KEL as K-Electric. Trusting NEPRA's parenthetical as a ticker maps a 131 MW RFO plant onto a listed utility.
- Frozen tables that look live: the FCA table stops at Jun-2022, the Quarterly table at the Apr-Jun 2022 quarter with 'Not Yet Issued' placeholders, and Hydel dependable capacities are pinned to 'July 2018 - June 2022'. None says it is stale; a user assumes the latest row is current.
- Upstream label error: FY2023-24 rows 101 and 102 both read 'Foundation Wind Energy-I Ltd.' while their parentheticals say (FWEL-I) and (FWEL-II). The name field is wrong for one of them.
- Index incompleteness in both directions: Main.htm links only 5 of the 7 reachable generation years, while the SIR root 404s four pages that serve 200 one directory down, the directory itself 403s, and all six PER PDF paths in the brief are dead. A 404 does not mean the data is absent, and a link list does not mean the data is complete.
