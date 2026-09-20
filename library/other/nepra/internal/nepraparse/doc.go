// Package nepraparse parses NEPRA's plant-level generation workbooks —
// the "List of Companies Generation wise <FY>" sheets published under
// State of Industry Reports / Detail of Generation. Each fiscal year is one
// Excel-exported HTML file (sheet001.htm). Reachable years: 2017-18 .. 2023-24.
//
// Everything below was measured against the live files, not assumed. The whole
// point of this package is that a naive HTML table reader corrupts the panel
// silently rather than erroring, so each trap is handled explicitly and the
// handling is observable through [Census], [HeaderFingerprint] and [SumReport].
//
// # Encoding
//
// The files are windows-1252. The HTTP response declares no charset; only the
// in-document <meta http-equiv=Content-Type content="text/html; charset=windows-1252">
// does. The only non-ASCII byte is 0xA0 (NBSP) — 128 occurrences in FY2017-18
// and FY2020-21, 132 in FY2023-24. Because of those bytes the files are
// "binary" to line-oriented tools: `grep -c '<td' sheet001.htm` reports 0 on a
// 493,187-byte file. [Decode] handles this.
//
// # Structure
//
// Exactly one <table> and ZERO <th> in every year, so header detection must be
// text-matched, not semantic: any library that looks for <th> reports
// "no header" and treats row 0 as data.
//
// The header is a three-row band, not one row. tr#0 is an all-empty spacer,
// tr#1 is a <td colspan=32> title, tr#2/#3/#4 are the band (six rowspan=3
// stub cells plus <td colspan=26>FY 2023-24; thirteen colspan=2 month cells;
// thirty-three leaf cells), and data begins at tr#5.
//
// There are 32 logical columns and the schema is stable across all three
// sampled years — byte-identical header text, identical order. See
// [CanonicalColumns]. Two properties of that layout are load-bearing:
//
//   - Months run in FISCAL order Jul->Jun. Indexing them as calendar months
//     mis-dates every observation by up to six months. [Month] is a fiscal
//     ordinal and [FiscalYear.Period] does the calendar mapping.
//   - Within each month "% age" comes FIRST, then "GWh". Reading them the
//     other way round swaps all 26 monthly fields and still yields
//     plausible-looking numbers. [Workbook.ColumnOrder] proves the ordering
//     from the data itself.
//
// Physical row width is 40 columns in FY2017-18/FY2020-21 and 39 in FY2023-24.
// The surplus is trailing empty padding from the Excel export; it never shifts
// a real column. It is not reliably *empty*, though: one row in every sampled
// year carries a stray ";" in the last padding cell, so "strip trailing
// all-empty columns" is not sufficient on its own. The header band fixes the
// logical width at 32 and any non-empty cell past it is recorded as residue
// (see [Plant.Residue] and [Census.ResidueCells]).
//
// # Cell states
//
// Five states are distinguishable and none may be collapsed into another:
// a real measured numeric (including a genuine 0.00), an NBSP blank meaning
// NOT REPORTED, DELICENSED, DECOMMISSIONED, and "Export to K.Electric".
// See [CellState] and [Value].
//
// Blank is not zero, and the files prove it. The blank pattern is strictly
// all-or-nothing per row: every row with any blank in the monthly block has
// exactly 26 blanks and exactly zero "0.00" cells, and no row in any year
// mixes the two ([Census.MixedBlankZeroRows] is 0). Meanwhile "0.00" occurs
// 437/540/471 times (FY2017-18/FY2020-21/FY2023-24), always in otherwise
// fully-populated rows. Longitudinally, Karachi Nuclear Power Plant-II (K-2)
// is blank for all 26 monthly cells in FY2017-18 (licensed, not yet built) and
// then in FY2020-21 reports 16 explicit "0.00" month cells alongside a
// non-zero annual Sum of 1,705.91 GWh. Conflating blank with zero invents
// 11 plants x 12 months of false zero generation in FY2017-18 alone.
//
// States DELICENSED / DECOMMISSIONED / "Export to K.Electric" retain valid
// capacity values while the generation year is non-numeric — Kotri Power
// Station in FY2023-24 has Installed 174 MW, Dependable 120 MW and all 26
// monthly cells DELICENSED — so a status sentinel must never null the
// capacity. "Export to K.Electric" is the odd one out: it appears in the
// Installed Capacity (MW) column, not the monthly block.
//
// # Other traps handled here
//
//   - colspan on DATA rows. Thirteen FY2023-24 data rows carry a single
//     <td colspan=26>DELICENSED|DECOMMISSIONED and two carry an EMPTY
//     <td colspan=26>. Raw td-per-tr in FY2023-24 is {8:1, 14:16, 20:1,
//     33:1, 39:120}. A parser that ignores colspan reads "DELICENSED" as
//     Jul %age and then misaligns the padding cells into Jul GWh..Oct %age,
//     silently, because there is no <th> to cross-check against. colspan and
//     rowspan are expanded into a real grid before any column is assigned.
//   - Thousands separators: commas in 61/59/68 cells. strconv.ParseFloat
//     rejects "3,478". Commas are stripped only after a cell has been
//     classified as numeric, so "DELICENSED" can never become a number.
//   - Text wraps across source lines and through hidden spans:
//     ">Name of\n  Companies<", ">Natural Gas/\n  Furnace Oil<",
//     ">Export\n  to K.Electric<", and
//     "Company Limit<span style='display:none'>ed.</span>". Markup is removed
//     without inserting whitespace (so "Limit"+"ed." rejoins as "Limited."),
//     <br> becomes a space, then whitespace is collapsed. Never line-based
//     regex.
//   - S.No is NOT a stable key across years. It is stable FY2017-18 ->
//     FY2020-21 (107/107 unchanged) but 0/106 unchanged in FY2023-24 because
//     the file was re-sorted by technology (AES Lalpir 1->36, Allai Khwar
//     56->8). [Plant.SNo] is documented and named as an in-year ordinal only;
//     cross-year identity is a different package's problem.
//
// # Verification
//
// Sum == sum(12 monthly GWh) holds for 97/97 (FY2017-18), 104/105 (FY2020-21)
// and 118/118 (FY2023-24) eligible rows on the GWh columns, but only 7/97,
// 12/105 and 3/118 on the "% age" columns. That asymmetry is a proof of
// column order, exposed as [Workbook.CheckSum] and [Workbook.ColumnOrder].
// The one genuine FY2020-21 mismatch ((NPPCL) - Balloki: months total
// 5,945.21 against a reported 5,905.65) is reported, not hidden.
//
// Nothing in this package fabricates, interpolates or back-dates a value.
// A missing observation is representable as missing and stays that way.
package nepraparse
