// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"math"
	"strings"
)

// MatchRadiusM is the largest distance at which live data from a Multi
// Ekicube bank is attached to a coinlocker-navi record. It is a proximity
// association, not an identity: the two sources share no id, so the bank may
// be a neighbouring bank.
const MatchRadiusM = 30

// GateRadiusM is the stricter radius for copying the bank's gate side onto a
// record: exactly one bank must lie within it.
const GateRadiusM = 15

// GateSourceBank labels a gate value copied from a Multi Ekicube bank.
const GateSourceBank = "multiecube_single_bank_within_15m"

// LiveMatch is live data attached to a coinlocker-navi record.
type LiveMatch struct {
	Source          string           `json:"source"`
	ObservedAt      string           `json:"observed_at"`
	SourceAsOf      *string          `json:"source_as_of"`
	MatchBasis      string           `json:"match_basis"`
	MatchDistanceM  *int             `json:"match_distance_m,omitempty"`
	CandidatesInRad int              `json:"candidates_in_radius,omitempty"`
	Ekicube         *EkicubeLocation `json:"ekicube,omitempty"`
	Maihama         *MaihamaBlock    `json:"maihama,omitempty"`
	Caveat          string           `json:"caveat"`
}

// DistanceM is the haversine distance in metres.
func DistanceM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// NearestEkicube returns the closest bank within radius and how many banks
// were inside the radius.
func NearestEkicube(lat, lon float64, banks []EkicubeLocation, radius float64) (*EkicubeLocation, int, int) {
	var best *EkicubeLocation
	bestD := math.MaxFloat64
	n := 0
	for i := range banks {
		d := DistanceM(lat, lon, banks[i].Lat, banks[i].Lon)
		if d <= radius {
			n++
			if d < bestD {
				bestD = d
				best = &banks[i]
			}
		}
	}
	if best == nil {
		return nil, 0, 0
	}
	return best, int(math.Round(bestD)), n
}

// AttachEkicube adds live data and gate side to records near a bank.
func AttachEkicube(rows []Locker, banks []EkicubeLocation, observedAt string, withLive bool) {
	for i := range rows {
		if rows[i].Lat == nil || rows[i].Lon == nil {
			continue
		}
		if g := GateFor(*rows[i].Lat, *rows[i].Lon, banks); g != nil {
			if c := TextGateConflict(rows[i], *g); c != "" {
				rows[i].GateConflict = &c
			} else {
				src := GateSourceBank
				rows[i].Gate, rows[i].GateSource = g, &src
			}
		}
		if !withLive {
			continue
		}
		b, d, n := NearestLiveBank(rows[i], banks)
		if b == nil {
			continue
		}
		bank := *b
		bank.Caveat = ""
		rows[i].Live = &LiveMatch{
			Source:          "multiecube",
			ObservedAt:      observedAt,
			MatchBasis:      "nearest_gate_consistent_bank_within_30m_may_be_neighbouring_bank",
			MatchDistanceM:  &d,
			CandidatesInRad: n,
			Ekicube:         &bank,
			Caveat:          ReservableCaveat,
		}
	}
}

// NearestLiveBank returns the closest bank within MatchRadiusM whose gate
// side does not contradict the record's own 改札内/改札外 text, its distance,
// and how many banks lie within the radius. A bank on the other side of the
// gates is a different bank, so its counts are never shown for the record.
func NearestLiveBank(r Locker, banks []EkicubeLocation) (*EkicubeLocation, int, int) {
	if r.Lat == nil || r.Lon == nil {
		return nil, 0, 0
	}
	var best *EkicubeLocation
	bestD := math.MaxFloat64
	n := 0
	for i := range banks {
		d := DistanceM(*r.Lat, *r.Lon, banks[i].Lat, banks[i].Lon)
		if d > MatchRadiusM {
			continue
		}
		n++
		if banks[i].Gate != nil && TextGateConflict(r, *banks[i].Gate) != "" {
			continue
		}
		if d < bestD {
			bestD = d
			best = &banks[i]
		}
	}
	if best == nil {
		return nil, 0, n
	}
	return best, int(math.Round(bestD)), n
}

// AttachMaihama adds Maihama block data by exact locker id. It replaces any
// proximity-based Multi Ekicube match on the same record.
func AttachMaihama(rows []Locker, board MaihamaBoard, observedAt string) {
	byID := map[string]MaihamaBlock{}
	for _, b := range board.Blocks {
		if b.LockerID != nil {
			byID[*b.LockerID] = b
		}
	}
	for i := range rows {
		b, ok := byID[rows[i].ID]
		if !ok {
			continue
		}
		// An exact locker-id match with physical counts beats a proximity match.
		blk := b
		rows[i].Live = &LiveMatch{
			Source:     "coinlocker-navi-maihama",
			ObservedAt: observedAt,
			SourceAsOf: board.SourceAsOf,
			MatchBasis: "locker_id",
			Maihama:    &blk,
			Caveat:     "physical empty counts from the coinlocker-navi Maihama map at source_as_of; they can change before you arrive",
		}
	}
}

// GateFor returns the gate side for a point when exactly one bank with a
// structured gate value lies within GateRadiusM, else nil.
func GateFor(lat, lon float64, banks []EkicubeLocation) *string {
	gb, _, gn := NearestEkicube(lat, lon, banks, GateRadiusM)
	if gb == nil || gn != 1 || gb.Gate == nil {
		return nil
	}
	g := *gb.Gate
	return &g
}

// TextGateConflict reports when the record's own name or note names the other
// gate side (改札内 = inside, 改札外 = outside). The text never sets a gate;
// it only blocks a Multi Ekicube value that disagrees with it.
func TextGateConflict(r Locker, bankGate string) string {
	text := r.Name
	if r.Note != nil {
		text += " " + *r.Note
	}
	saysIn, saysOut := strings.Contains(text, "改札内"), strings.Contains(text, "改札外")
	switch {
	case bankGate == "outside" && saysIn:
		return "record text says 改札内 (inside); nearest Multi Ekicube bank says outside; gate left unknown"
	case bankGate == "inside" && saysOut:
		return "record text says 改札外 (outside); nearest Multi Ekicube bank says inside; gate left unknown"
	case bankGate == "both" && saysIn != saysOut:
		return "record text names one side only; nearest Multi Ekicube bank says both; gate left unknown"
	}
	return ""
}
