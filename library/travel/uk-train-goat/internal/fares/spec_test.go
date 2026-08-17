// uk-train-goat hand-authored: verifies RJFAF field offsets against real
// records captured from live feed RJFAF798 (2026-06-16). If an offset is
// wrong, the matching case fails — this test IS the offset verification.
package fares

import "testing"

func TestField(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		start int
		width int
		want  string
	}{
		// FFL flow record (prefix RF).
		{"ffl flow origin", "RF0027003201000000AS3112299902032025ATO01Y0000020", fflFlowOrigin[0], fflFlowOrigin[1], "0027"},
		{"ffl flow dest", "RF0027003201000000AS3112299902032025ATO01Y0000020", fflFlowDest[0], fflFlowDest[1], "0032"},
		{"ffl flow route", "RF0027003201000000AS3112299902032025ATO01Y0000020", fflFlowRoute[0], fflFlowRoute[1], "01000"},
		{"ffl flow direction", "RF0027003201000000AS3112299902032025ATO01Y0000020", fflFlowDir[0], fflFlowDir[1], "S"},
		{"ffl flow end date", "RF0027003201000000AS3112299902032025ATO01Y0000020", fflFlowEnd[0], fflFlowEnd[1], "31122999"},
		{"ffl flow start date", "RF0027003201000000AS3112299902032025ATO01Y0000020", fflFlowStart[0], fflFlowStart[1], "02032025"},
		{"ffl flow toc", "RF0027003201000000AS3112299902032025ATO01Y0000020", fflFlowTOC[0], fflFlowTOC[1], "ATO"},
		{"ffl flow id", "RF0027003201000000AS3112299902032025ATO01Y0000020", fflFlowID[0], fflFlowID[1], "0000020"},

		// FFL fare record (prefix RT).
		{"ffl fare flow id", "RT0000001FDS00021450  ", fflFareFlowID[0], fflFareFlowID[1], "0000001"},
		{"ffl fare ticket", "RT0000001FDS00021450  ", fflFareTicketCode[0], fflFareTicketCode[1], "FDS"},
		{"ffl fare pence", "RT0000001FDS00021450  ", fflFarePence[0], fflFarePence[1], "00021450"},
		{"ffl fare no restriction", "RT0000001FDS00021450  ", fflFareRestriction[0], fflFareRestriction[1], ""},
		{"ffl fare with restriction", "RT0000001SSR00011400YW", fflFareRestriction[0], fflFareRestriction[1], "YW"},

		// LOC location record (prefix RL) — London Paddington.
		{"loc nlc", "RL703087001092025070820251408202470 3087LONDON PADDINGTNPAD0", locNLC[0], locNLC[1], "3087"},
		{"loc description", "RL703087001092025070820251408202470 3087LONDON PADDINGTNPAD0", locDescription[0], locDescription[1], "LONDON PADDINGTN"},
		{"loc crs", "RL703087001092025070820251408202470 3087LONDON PADDINGTNPAD0", locCRS[0], locCRS[1], "PAD"},

		// FSC cluster record (marker R, no record-type char).
		{"fsc cluster id", "RAC5503753112299918102018", fscClusterID[0], fscClusterID[1], "AC55"},
		{"fsc cluster nlc", "RAC5503753112299918102018", fscClusterNLC[0], fscClusterNLC[1], "0375"},

		// TTY ticket-type record.
		{"tty code", "R0AA311229991707201916072019SMART TKS      2SS31122999001001000000001000NNY87", ttyCode[0], ttyCode[1], "0AA"},
		{"tty description", "R0AA311229991707201916072019SMART TKS      2SS31122999001001000000001000NNY87", ttyDescription[0], ttyDescription[1], "SMART TKS"},
		{"tty type single", "R0AA311229991707201916072019SMART TKS      2SS31122999001001000000001000NNY87", ttyType[0], ttyType[1], "S"},
		{"tty type return", "R0AB311229991707201916072019SMART TKR      2RS31122999001001000000001000NNY84", ttyType[0], ttyType[1], "R"},

		// RLC railcard record (no marker).
		{"rlc code", "00D311229990604202306042023ADI PERKBOX          NNNN00DN001001000000000000001000", rlcCode[0], rlcCode[1], "00D"},
		{"rlc public description", "   311229993006201030062010APUBLIC              NNNN   Y008001008001008000008000", rlcDescription[0], rlcDescription[1], "PUBLIC"},

		// NFO non-derivable fare override (marker R, adult fare in pence).
		{"nfo origin", "R0027003401000   ADTO311229992903202607012025N0000166000000830  YNN", nfoOrigin[0], nfoOrigin[1], "0027"},
		{"nfo dest", "R0027003401000   ADTO311229992903202607012025N0000166000000830  YNN", nfoDest[0], nfoDest[1], "0034"},
		{"nfo ticket", "R0027003401000   ADTO311229992903202607012025N0000166000000830  YNN", nfoTicket[0], nfoTicket[1], "ADT"},
		{"nfo adult fare", "R0027003401000   ADTO311229992903202607012025N0000166000000830  YNN", nfoAdultFare[0], nfoAdultFare[1], "00001660"},

		// RST header row (prefix RRH): code -> description.
		{"rst header code", "RRHC1COFF-PEAK                      NOT VALID ON C", rstHeaderCode[0], rstHeaderCode[1], "1C"},
		{"rst header description", "RRHC1COFF-PEAK                      NOT VALID ON C", rstHeaderDesc[0], rstHeaderDesc[1], "OFF-PEAK"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := field(c.line, c.start, c.width); got != c.want {
				t.Errorf("field(%q, %d, %d) = %q, want %q", c.line, c.start, c.width, got, c.want)
			}
		})
	}
}

// TestFieldBounds covers the clamp/empty paths so a short or truncated
// record never panics the parser.
func TestFieldBounds(t *testing.T) {
	if got := field("RT0001", 50, 4); got != "" {
		t.Errorf("start past end: got %q, want empty", got)
	}
	if got := field("RT0000001FDS00021450", 13, 8); got != "00021450" {
		t.Errorf("end exactly at line length: got %q", got)
	}
	if got := field("RT0000001FDS000214", 13, 8); got != "000214" {
		t.Errorf("end past line should clamp: got %q, want %q", got, "000214")
	}
}
