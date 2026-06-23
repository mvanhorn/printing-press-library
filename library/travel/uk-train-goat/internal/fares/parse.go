package fares

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// maxLOCRecordLen sizes the scanner buffer for LOC files. Real RJFAF LOC
// records reach 289 bytes (dtd2mysql LOC.ts defines fields out to col 288),
// which exceeds the standard buffer used for the other ≤139-byte feed files.
// Without this, bufio.Scanner returns ErrTooLong on real L records.
const maxLOCRecordLen = 1024

// maxRecordLen sizes the scanner buffer for the non-LOC feed files, whose
// records are all ≤139 bytes.
const maxRecordLen = 256

// scanLines runs fn for every line of r using a scanner whose buffer is capped
// at maxLen bytes, and returns the scanner's terminal error.
func scanLines(r io.Reader, maxLen int, fn func(line string)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, maxLen), maxLen)
	for sc.Scan() {
		fn(sc.Text())
	}
	return sc.Err()
}

// parsePence converts an integer-pence string to int. Empty input is an error.
func parsePence(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty fare")
	}
	return strconv.Atoi(s)
}

// parseDate converts a feed ddmmyyyy date to sortable YYYYMMDD.
// Returns "" for empty or incorrectly sized input.
func parseDate(d string) string {
	if len(d) != 8 {
		return ""
	}
	return d[4:8] + d[2:4] + d[0:2]
}

// ParseLOC parses an RJFAF LOC file (station location records) from r.
// Only insert records (col 1 == "R", col 2 == "L") with a non-blank CRS are
// returned.
func ParseLOC(r io.Reader) ([]Location, error) {
	var locs []Location
	err := scanLines(r, maxLOCRecordLen, func(line string) {
		if len(line) < 2 || line[:1] != recMarkerInsert || line[1:2] != "L" {
			return
		}
		crs := field(line, locCRS[0], locCRS[1])
		if crs == "" {
			return
		}
		locs = append(locs, Location{
			NLC:       field(line, locNLC[0], locNLC[1]),
			CRS:       crs,
			Name:      field(line, locDescription[0], locDescription[1]),
			StartDate: parseDate(field(line, locStart[0], locStart[1])),
			EndDate:   parseDate(field(line, locEnd[0], locEnd[1])),
		})
	})
	return locs, err
}

// ParseTTY parses an RJFAF TTY file (ticket type records) from r.
// Only insert records (col 1 == "R") are returned.
// TicketClass is not populated: spec.go pins no offset for it.
func ParseTTY(r io.Reader) ([]TicketType, error) {
	var tickets []TicketType
	err := scanLines(r, maxRecordLen, func(line string) {
		if len(line) < 1 || line[:1] != recMarkerInsert {
			return
		}
		tickets = append(tickets, TicketType{
			Code:        field(line, ttyCode[0], ttyCode[1]),
			Description: field(line, ttyDescription[0], ttyDescription[1]),
			TicketType:  field(line, ttyType[0], ttyType[1]),
		})
	})
	return tickets, err
}

// ParseNFO parses an RJFAF NFO file (non-derivable fare overrides) from r.
// Only insert records (col 1 == "R") with a blank railcard code (adult fares)
// are returned. Rows whose adult-fare pence field cannot be parsed are skipped.
func ParseNFO(r io.Reader) ([]NonDerivableFare, error) {
	var fares []NonDerivableFare
	err := scanLines(r, maxRecordLen, func(line string) {
		if len(line) < 1 || line[:1] != recMarkerInsert {
			return
		}
		if field(line, nfoRailcard[0], nfoRailcard[1]) != "" {
			return
		}
		p, perr := parsePence(field(line, nfoAdultFare[0], nfoAdultFare[1]))
		if perr != nil {
			return
		}
		fares = append(fares, NonDerivableFare{
			OriginNLC:       field(line, nfoOrigin[0], nfoOrigin[1]),
			DestNLC:         field(line, nfoDest[0], nfoDest[1]),
			Route:           field(line, nfoRoute[0], nfoRoute[1]),
			TicketCode:      field(line, nfoTicket[0], nfoTicket[1]),
			Pence:           p,
			RestrictionCode: field(line, nfoRestriction[0], nfoRestriction[1]),
			StartDate:       parseDate(field(line, nfoStart[0], nfoStart[1])),
			EndDate:         parseDate(field(line, nfoEnd[0], nfoEnd[1])),
		})
	})
	return fares, err
}

// ParseLOCGroupMembers parses station-group membership from an RJFAF LOC file.
// Only insert records (col 1 == "R") are processed. 'L' records build a UIC->NLC
// map; 'M' records are resolved through it into GroupMember{MemberNLC, GroupNLC, End}.
// M records whose group or member UIC has no L-record NLC are skipped.
func ParseLOCGroupMembers(r io.Reader) ([]GroupMember, error) {
	uicToNLC := make(map[string]string)
	var members []GroupMember
	err := scanLines(r, maxLOCRecordLen, func(line string) {
		if len(line) < 2 || line[:1] != recMarkerInsert {
			return
		}
		switch line[1:2] {
		case "L":
			uic := field(line, locUIC[0], locUIC[1])
			nlc := field(line, locNLC[0], locNLC[1])
			if uic != "" && nlc != "" {
				uicToNLC[uic] = nlc
			}
		case "M":
			groupUIC := field(line, locgmGroupUIC[0], locgmGroupUIC[1])
			memberUIC := field(line, locgmMemberUIC[0], locgmMemberUIC[1])
			groupNLC, ok1 := uicToNLC[groupUIC]
			memberNLC, ok2 := uicToNLC[memberUIC]
			if !ok1 || !ok2 {
				return
			}
			members = append(members, GroupMember{
				MemberNLC: memberNLC,
				GroupNLC:  groupNLC,
				EndDate:   parseDate(field(line, locgmEnd[0], locgmEnd[1])),
			})
		}
	})
	return members, err
}

// ParseFSC parses an RJFAF FSC file (fare station cluster records) from r.
// Only insert records (col 1 == "R") are returned.
func ParseFSC(r io.Reader) ([]ClusterMember, error) {
	var members []ClusterMember
	err := scanLines(r, maxRecordLen, func(line string) {
		if len(line) < 1 || line[:1] != recMarkerInsert {
			return
		}
		members = append(members, ClusterMember{
			ClusterID: field(line, fscClusterID[0], fscClusterID[1]),
			MemberNLC: field(line, fscClusterNLC[0], fscClusterNLC[1]),
		})
	})
	return members, err
}

// ParseRSTHeaders parses restriction header rows from an RJFAF RST file.
// Only lines beginning "RRH" are returned; all other RST record types are
// ignored.
func ParseRSTHeaders(r io.Reader) ([]Restriction, error) {
	var restrictions []Restriction
	err := scanLines(r, maxRecordLen, func(line string) {
		if !strings.HasPrefix(line, "RRH") {
			return
		}
		restrictions = append(restrictions, Restriction{
			Code:        field(line, rstHeaderCode[0], rstHeaderCode[1]),
			Description: field(line, rstHeaderDesc[0], rstHeaderDesc[1]),
		})
	})
	return restrictions, err
}

// ParseRLC parses an RJFAF RLC file (railcard definitions) from r.
// RLC records have no update marker; the railcard code occupies cols 1-3
// directly. Rows with a blank code (e.g. PUBLIC/no-railcard rows) are skipped.
// MinPence and DiscountPct are not populated: spec.go pins no offsets for them;
// discount math is deferred to a later task.
func ParseRLC(r io.Reader) ([]Railcard, error) {
	var cards []Railcard
	err := scanLines(r, maxRecordLen, func(line string) {
		code := field(line, rlcCode[0], rlcCode[1])
		if code == "" {
			return
		}
		cards = append(cards, Railcard{
			Code:        code,
			Description: field(line, rlcDescription[0], rlcDescription[1]),
		})
	})
	return cards, err
}

// ParseFFL parses an RJFAF FFL file (flows + fares) from r.
// Only insert records (col 1 == "R") are processed; amend/delete are skipped.
func ParseFFL(r io.Reader) (flows []Flow, fares []Fare, err error) {
	scanErr := scanLines(r, maxRecordLen, func(line string) {
		if len(line) < 2 || line[:1] != recMarkerInsert {
			return
		}
		switch line[1:2] {
		case "F":
			flows = append(flows, Flow{
				FlowID:    field(line, fflFlowID[0], fflFlowID[1]),
				OriginNLC: field(line, fflFlowOrigin[0], fflFlowOrigin[1]),
				DestNLC:   field(line, fflFlowDest[0], fflFlowDest[1]),
				Route:     field(line, fflFlowRoute[0], fflFlowRoute[1]),
				Direction: field(line, fflFlowDir[0], fflFlowDir[1]),
				TOC:       field(line, fflFlowTOC[0], fflFlowTOC[1]),
				StartDate: parseDate(field(line, fflFlowStart[0], fflFlowStart[1])),
				EndDate:   parseDate(field(line, fflFlowEnd[0], fflFlowEnd[1])),
			})
		case "T":
			p, perr := parsePence(field(line, fflFarePence[0], fflFarePence[1]))
			if perr != nil {
				return
			}
			fares = append(fares, Fare{
				FlowID:          field(line, fflFareFlowID[0], fflFareFlowID[1]),
				TicketCode:      field(line, fflFareTicketCode[0], fflFareTicketCode[1]),
				Pence:           p,
				RestrictionCode: field(line, fflFareRestriction[0], fflFareRestriction[1]),
			})
		}
	})
	return flows, fares, scanErr
}
