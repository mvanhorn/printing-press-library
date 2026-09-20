// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package nepraxwalk resolves the plant names NEPRA publishes in its annual
// generation workbooks onto stable canonical plants, and maps those plants to
// their listed operator on the Pakistan Stock Exchange.
//
// # Why this package exists
//
// NEPRA's seven annual generation workbooks share no stable key.
//
// S.No is worthless across years. Of the 107 plant names common to the
// FY2017-18 and FY2023-24 workbooks (whitespace-normalised), ZERO keep the
// same S.No, because FY2023-24 was re-sorted by technology: AES Lalpir moved
// 1->36, Allai Khwar 56->8, Kot Addu (KAPCO) 9->33, Kohinoor (KEL) 12->35,
// Narowal 11->58. Joining fiscal years on S.No swaps plants' entire histories.
//
// Names are the only usable key, but they drift. "Narowal Energy Ltd.
// (HUBCO)" in FY2017-18 becomes "Narowal Energy Ltd." in FY2023-24, so an
// exact string join silently DROPS a wholly owned HUBCO subsidiary that an
// analyst modelling HUBC is specifically trying to see. Hence: names plus a
// curated alias table, stored as reviewable data in crosswalk.json.
//
// # The KEL trap
//
// NEPRA writes "Kohinoor Energy Limited. (KEL)" for a 131 MW RFO plant near
// Lahore. On the PSX, KEL is K-Electric Limited, a KSE-100 listed utility.
// The listed company behind the NEPRA row is Kohinoor Energy Limited, symbol
// KOHE. [ResolveTicker] therefore REFUSES the bare token "KEL" with an
// [AmbiguousTokenError] rather than picking a side. Five more verified
// collisions get the same treatment: AGL, APL, SPL, AEL and HEPL. See
// [AmbiguousTokens].
//
// # Absent, not zero
//
// K-Electric's own generation fleet is not in this dataset at all. Searching
// both fetched workbooks' name columns for BQPS, Korangi, SITE, Bin Qasim,
// KESC, "K-Electric" and "Karachi Electric" returns zero rows out of 241
// published names. [PlantsForParent]("KEL") therefore reports
// [StatusAbsentFromDataset] with an explanatory note, never an empty slice
// that a caller could read as "it generated nothing".
//
// # Conservatism
//
// Resolution is exact-or-refuse. There is no edit distance, no token overlap
// and no prefix matching anywhere in this package. The only tolerance is a
// single trailing-parenthetical strip, and even that is refused when the
// stripped form hits more than one plant — which is a live case, not a
// hypothetical: NEPRA publishes both of its Foundation Wind farms with the
// base string "Foundation Wind Energy-I Ltd.", distinguished only by
// "(FWEL-I)" versus "(FWEL-II)". See [PossibleDuplicates].
//
// Absence is always representable. An empty PSXTicker means "no ticker
// asserted", and [ListedStatus] tells "not listed on PSX" apart from "listing
// unknown". An empty InstalledCapacityMWRaw means the workbook cell was blank
// or NBSP-only — not reported — and never zero; use
// [Observation.InstalledCapacityMW], whose second return reports whether a
// number was published at all.
package nepraxwalk
