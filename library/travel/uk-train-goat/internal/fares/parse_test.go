// uk-train-goat hand-authored: tests for ParseFFL using inline real record
// literals captured from live feed RJFAF798 (2026-06-16).
package fares

import (
	"strings"
	"testing"
)

// Real records from the live feed, verified by spec_test.go offset tests.
const fflSample = "RF0027003201000000AS3112299902032025ATO01Y0000020\n" +
	"RT0000001FDS00021450  \n" +
	"RT0000001SSR00011400YW"

func TestParseFFL(t *testing.T) {
	flows, fares, err := ParseFFL(strings.NewReader(fflSample))
	if err != nil {
		t.Fatalf("ParseFFL returned error: %v", err)
	}

	// Flow assertions.
	if len(flows) != 1 {
		t.Fatalf("want 1 flow, got %d", len(flows))
	}
	fl := flows[0]
	if fl.OriginNLC != "0027" {
		t.Errorf("OriginNLC = %q, want %q", fl.OriginNLC, "0027")
	}
	if fl.DestNLC != "0032" {
		t.Errorf("DestNLC = %q, want %q", fl.DestNLC, "0032")
	}
	if fl.TOC != "ATO" {
		t.Errorf("TOC = %q, want %q", fl.TOC, "ATO")
	}
	if fl.Direction != "S" {
		t.Errorf("Direction = %q, want %q", fl.Direction, "S")
	}
	if fl.StartDate != "20250302" {
		t.Errorf("StartDate = %q, want %q", fl.StartDate, "20250302")
	}
	if fl.EndDate != "29991231" {
		t.Errorf("EndDate = %q, want %q", fl.EndDate, "29991231")
	}

	// Fare assertions.
	if len(fares) != 2 {
		t.Fatalf("want 2 fares, got %d", len(fares))
	}
	var fds, ssr *Fare
	for i := range fares {
		switch fares[i].TicketCode {
		case "FDS":
			fds = &fares[i]
		case "SSR":
			ssr = &fares[i]
		}
	}
	if fds == nil {
		t.Fatal("fare with TicketCode FDS not found")
	}
	if fds.FlowID != "0000001" {
		t.Errorf("FDS FlowID = %q, want %q", fds.FlowID, "0000001")
	}
	if fds.Pence != 21450 {
		t.Errorf("FDS Pence = %d, want 21450", fds.Pence)
	}
	if fds.RestrictionCode != "" {
		t.Errorf("FDS RestrictionCode = %q, want empty", fds.RestrictionCode)
	}

	if ssr == nil {
		t.Fatal("fare with TicketCode SSR not found")
	}
	if ssr.Pence != 11400 {
		t.Errorf("SSR Pence = %d, want 11400", ssr.Pence)
	}
	if ssr.RestrictionCode != "YW" {
		t.Errorf("SSR RestrictionCode = %q, want %q", ssr.RestrictionCode, "YW")
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"02032025", "20250302"},
		{"31122999", "29991231"},
		{"", ""},
		{"0203", ""},      // short
		{"020320251", ""}, // too long (len 9)
	}
	for _, c := range cases {
		if got := parseDate(c.in); got != c.want {
			t.Errorf("parseDate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParsePence(t *testing.T) {
	p, err := parsePence("00021450")
	if err != nil {
		t.Fatalf("parsePence(%q) returned error: %v", "00021450", err)
	}
	if p != 21450 {
		t.Errorf("parsePence(%q) = %d, want 21450", "00021450", p)
	}

	_, err = parsePence("")
	if err == nil {
		t.Error("parsePence(\"\") should return error")
	}
}

// TTY parser tests.

const ttySample = "R0AA311229991707201916072019SMART TKS      2SS31122999001001000000001000NNY87\n" +
	"R0AB311229991707201916072019SMART TKR      2RS31122999001001000000001000NNY84"

func TestParseTTY(t *testing.T) {
	tickets, err := ParseTTY(strings.NewReader(ttySample))
	if err != nil {
		t.Fatalf("ParseTTY returned error: %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("want 2 ticket types, got %d", len(tickets))
	}
	var aa, ab *TicketType
	for i := range tickets {
		switch tickets[i].Code {
		case "0AA":
			aa = &tickets[i]
		case "0AB":
			ab = &tickets[i]
		}
	}
	if aa == nil {
		t.Fatal("ticket 0AA not found")
	}
	if aa.Description != "SMART TKS" {
		t.Errorf("0AA Description = %q, want %q", aa.Description, "SMART TKS")
	}
	if aa.TicketType != "S" {
		t.Errorf("0AA TicketType = %q, want %q", aa.TicketType, "S")
	}
	if ab == nil {
		t.Fatal("ticket 0AB not found")
	}
	if ab.TicketType != "R" {
		t.Errorf("0AB TicketType = %q, want %q", ab.TicketType, "R")
	}
}

// NFO parser tests.

const nfoSample = "R0027003401000   ADTO311229992903202607012025N0000166000000830  YNN\n" +
	"R0027003401000SRN ADTO311229992903202607012025N0000166000000830  YNN" // non-blank railcard, must be skipped

func TestParseNFO(t *testing.T) {
	fares, err := ParseNFO(strings.NewReader(nfoSample))
	if err != nil {
		t.Fatalf("ParseNFO returned error: %v", err)
	}
	if len(fares) != 1 {
		t.Fatalf("want 1 fare (railcard row filtered), got %d", len(fares))
	}
	f := fares[0]
	if f.OriginNLC != "0027" {
		t.Errorf("OriginNLC = %q, want %q", f.OriginNLC, "0027")
	}
	if f.DestNLC != "0034" {
		t.Errorf("DestNLC = %q, want %q", f.DestNLC, "0034")
	}
	if f.TicketCode != "ADT" {
		t.Errorf("TicketCode = %q, want %q", f.TicketCode, "ADT")
	}
	if f.Pence != 1660 {
		t.Errorf("Pence = %d, want 1660", f.Pence)
	}
}

// FSC parser tests.

const fscSample = "RAC5503753112299918102018"

func TestParseFSC(t *testing.T) {
	members, err := ParseFSC(strings.NewReader(fscSample))
	if err != nil {
		t.Fatalf("ParseFSC returned error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("want 1 cluster member, got %d", len(members))
	}
	m := members[0]
	if m.ClusterID != "AC55" {
		t.Errorf("ClusterID = %q, want %q", m.ClusterID, "AC55")
	}
	if m.MemberNLC != "0375" {
		t.Errorf("MemberNLC = %q, want %q", m.MemberNLC, "0375")
	}
	if m.StartDate != "20181018" {
		t.Errorf("StartDate = %q, want %q", m.StartDate, "20181018")
	}
	if m.EndDate != "29991231" {
		t.Errorf("EndDate = %q, want %q", m.EndDate, "29991231")
	}
}

// RST restriction header tests.

const rstSample = "RRHC1COFF-PEAK                      NOT VALID ON C\n" +
	"RRD1CSOME OTHER RECORD TYPE\n" + // non-RRH line, must be skipped
	"RRHC2CANYTIME                       VALID ALL TIMES"

func TestParseRSTHeaders(t *testing.T) {
	restrictions, err := ParseRSTHeaders(strings.NewReader(rstSample))
	if err != nil {
		t.Fatalf("ParseRSTHeaders returned error: %v", err)
	}
	if len(restrictions) != 2 {
		t.Fatalf("want 2 restrictions, got %d", len(restrictions))
	}
	var offPeak *Restriction
	for i := range restrictions {
		if restrictions[i].Code == "1C" {
			offPeak = &restrictions[i]
		}
	}
	if offPeak == nil {
		t.Fatal("restriction with code 1C not found")
	}
	if offPeak.Description != "OFF-PEAK" {
		t.Errorf("1C Description = %q, want %q", offPeak.Description, "OFF-PEAK")
	}
}

// RLC railcard tests.

const rlcSample = "00D311229990604202306042023ADI PERKBOX          NNNN00DN001001000000000000001000\n" +
	"   311229993006201030062010APUBLIC              NNNN   Y008001008001008000008000" // blank code, must be skipped

func TestParseRLC(t *testing.T) {
	cards, err := ParseRLC(strings.NewReader(rlcSample))
	if err != nil {
		t.Fatalf("ParseRLC returned error: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("want 1 railcard (blank-code PUBLIC skipped), got %d", len(cards))
	}
	c := cards[0]
	if c.Code != "00D" {
		t.Errorf("Code = %q, want %q", c.Code, "00D")
	}
	if c.Description != "DI PERKBOX" {
		t.Errorf("Description = %q, want %q", c.Description, "DI PERKBOX")
	}
}

// LOC group-member parser tests.
//
// Full-length (289-char) real RJFAF798 L records, verbatim from the live feed.
// These exceed the 256-byte default scanner buffer; the LOC parsers raise it to
// maxLOCRecordLen so real records parse without bufio.ErrTooLong.
const locLineLondonTerminals = "RL701072001092025090820250704201770 1072LONDON TERMINALS   00000     1072  01GL00271 02LONDON TERMINALS                         LONDON TERMINALSLONDON TERMINALS                                            LONDON TERMINALS                                        0 NNNNNN1006009S0169177072185"
const locLinePaddington = "RL703087001092025070820251408202470 3087LONDON PADDINGTNPAD00123     1072  01GL00271 09LONDON PADDINGTN                         LONDON PADDINGTNLONDON PADDINGTON                                           LONDON PADDINGTON                                       3 NNNNNN1006191S0000000000185"

// locGroupMemberSample: group L (UIC 7010720→NLC 1072), member L (PAD, UIC
// 7030870→NLC 3087), then M records. PAD resolves; VIC (member UIC 7054260 has
// no L record) is skipped; the amend-marker line is ignored.
const locGroupMemberSample = locLineLondonTerminals + "\n" +
	locLinePaddington + "\n" +
	"RM7010720311229997030870PAD\n" + // PAD member: both UICs resolvable → kept
	"RM7010720311229997054260VIC\n" + // VIC member: member UIC 7054260 has no L record → skipped
	"AM7010720311229997030870PAD" // amend marker: must be ignored

func TestParseLOCGroupMembers(t *testing.T) {
	members, err := ParseLOCGroupMembers(strings.NewReader(locGroupMemberSample))
	if err != nil {
		t.Fatalf("ParseLOCGroupMembers returned error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("want 1 GroupMember (VIC skipped, amend ignored), got %d", len(members))
	}
	m := members[0]
	if m.MemberNLC != "3087" {
		t.Errorf("MemberNLC = %q, want %q", m.MemberNLC, "3087")
	}
	if m.GroupNLC != "1072" {
		t.Errorf("GroupNLC = %q, want %q", m.GroupNLC, "1072")
	}
	if m.EndDate != "29991231" {
		t.Errorf("EndDate = %q, want %q", m.EndDate, "29991231")
	}
}

// LOC parser tests.

const locSample = "RL703087001092025070820251408202470 3087LONDON PADDINGTNPAD0\n" +
	"RL000000001092025070820251408202470 0000              \n" + // blank CRS, must be skipped
	"AL703087001092025070820251408202470 3087LONDON PADDINGTNPAD0" // amend marker, must be skipped

func TestParseLOC(t *testing.T) {
	locs, err := ParseLOC(strings.NewReader(locSample))
	if err != nil {
		t.Fatalf("ParseLOC returned error: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("want 1 location, got %d", len(locs))
	}
	l := locs[0]
	if l.NLC != "3087" {
		t.Errorf("NLC = %q, want %q", l.NLC, "3087")
	}
	if l.CRS != "PAD" {
		t.Errorf("CRS = %q, want %q", l.CRS, "PAD")
	}
	if l.Name != "LONDON PADDINGTN" {
		t.Errorf("Name = %q, want %q", l.Name, "LONDON PADDINGTN")
	}
}

// TestParseLOCFullLengthRecord locks in that a full 289-char real LOC record
// parses without bufio.ErrTooLong. The 256-byte default scanner buffer truncates
// these; maxLOCRecordLen is the fix.
func TestParseLOCFullLengthRecord(t *testing.T) {
	if len(locLinePaddington) != 289 {
		t.Fatalf("fixture length = %d, want 289 (real-feed structural max)", len(locLinePaddington))
	}
	locs, err := ParseLOC(strings.NewReader(locLinePaddington))
	if err != nil {
		t.Fatalf("ParseLOC on full-length record returned error: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("want 1 location, got %d", len(locs))
	}
	l := locs[0]
	if l.NLC != "3087" {
		t.Errorf("NLC = %q, want %q", l.NLC, "3087")
	}
	if l.CRS != "PAD" {
		t.Errorf("CRS = %q, want %q", l.CRS, "PAD")
	}
}
