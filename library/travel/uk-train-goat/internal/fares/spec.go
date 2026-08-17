// uk-train-goat hand-authored: RJFAF fixed-width record field offsets.
//
// Offsets verified 2026-06-20 against live feed RJFAF798 (published
// 2026-06-16) and cross-checked against the authoritative open-source
// parser planarnetwork/dtd2mysql (config/fares/file/*.ts). Those configs
// use 0-based offsets; the values here are 1-based [start, length], i.e.
// dtd2mysql_offset + 1.
//
// Record framing differs per file:
//   - MultiRecordFile (FFL, LOC): col 1 = RJIS update marker (R insert /
//     A amend / D delete), col 2 = record type ('F'/'T', 'L'/'A'/'G'/...).
//   - SingleRecordFile with marker (FSC, TTY, NFO): col 1 = update marker,
//     data fields follow from col 2.
//   - RLC has NO marker: the railcard code occupies cols 1-3 directly.
//   - RST is a MultiRecordFile whose record type is the 2 chars at cols
//     2-3 (e.g. "RH" header rows arrive as lines beginning "RRH").
package fares

import "strings"

// field returns the trimmed value at a 1-based column range. RJFAF records
// are fixed-width ASCII, so byte slicing is safe. An out-of-range start
// returns ""; an end past the line is clamped to the line length.
func field(line string, start, length int) string {
	if start < 1 || start-1 >= len(line) {
		return ""
	}
	end := start - 1 + length
	if end > len(line) {
		end = len(line)
	}
	return strings.TrimRight(line[start-1:end], " ")
}

// recMarkerInsert is the update marker (col 1) on full-refresh records.
const recMarkerInsert = "R"

// FFL flow record (line prefix "RF").
var (
	fflFlowOrigin = [2]int{3, 4}
	fflFlowDest   = [2]int{7, 4}
	fflFlowRoute  = [2]int{11, 5}
	fflFlowStatus = [2]int{16, 3}
	fflFlowUsage  = [2]int{19, 1}
	fflFlowDir    = [2]int{20, 1} // 'R' = reversible (valid both ways); 'S' = single direction
	fflFlowEnd    = [2]int{21, 8} // ddmmyyyy
	fflFlowStart  = [2]int{29, 8}
	fflFlowTOC    = [2]int{37, 3}
	fflFlowID     = [2]int{43, 7}
)

// FFL fare record (line prefix "RT").
var (
	fflFareFlowID      = [2]int{3, 7}
	fflFareTicketCode  = [2]int{10, 3}
	fflFarePence       = [2]int{13, 8} // integer pence
	fflFareRestriction = [2]int{21, 2}
)

// LOC location record (line prefix "RL").
var (
	locUIC         = [2]int{3, 7}
	locEnd         = [2]int{10, 8}
	locStart       = [2]int{18, 8}
	locNLC         = [2]int{37, 4}
	locDescription = [2]int{41, 16}
	locCRS         = [2]int{57, 3}
)

// LOC group-member record (line prefix "RM"): maps a station to a station group.
// M records carry no start_date. Group and member are identified by UIC; resolve to
// NLC via the L records (locUIC -> locNLC) that precede them in the file.
var (
	locgmGroupUIC  = [2]int{3, 7}
	locgmEnd       = [2]int{10, 8} // ddmmyyyy
	locgmMemberUIC = [2]int{18, 7}
)

// FSC fare-station-cluster record (marker at col 1, single record type).
var (
	fscClusterID  = [2]int{2, 4}
	fscClusterNLC = [2]int{6, 4}
	fscEnd        = [2]int{10, 8}
	fscStart      = [2]int{18, 8}
)

// TTY ticket-type record (marker at col 1, single record type).
var (
	ttyCode        = [2]int{2, 3}
	ttyEnd         = [2]int{5, 8}
	ttyStart       = [2]int{13, 8}
	ttyDescription = [2]int{29, 15}
	ttyType        = [2]int{45, 1} // 'S' single, 'R' return
)

// RLC railcard record (NO marker; code occupies cols 1-3).
var (
	rlcCode        = [2]int{1, 3}
	rlcEnd         = [2]int{4, 8}
	rlcStart       = [2]int{12, 8}
	rlcDescription = [2]int{29, 20} // col 28 holds a 1-char holder_type that precedes the name
)

// NFO non-derivable-fare-override record (marker at col 1, single type).
var (
	nfoOrigin      = [2]int{2, 4}
	nfoDest        = [2]int{6, 4}
	nfoRoute       = [2]int{10, 5}
	nfoRailcard    = [2]int{15, 3}
	nfoTicket      = [2]int{18, 3}
	nfoEnd         = [2]int{22, 8}
	nfoStart       = [2]int{30, 8}
	nfoAdultFare   = [2]int{47, 8} // integer pence
	nfoRestriction = [2]int{63, 2}
)

// RST restriction header rows (line prefix "RRH"): code -> description.
var (
	rstHeaderCode = [2]int{5, 2}
	rstHeaderDesc = [2]int{7, 30}
)
