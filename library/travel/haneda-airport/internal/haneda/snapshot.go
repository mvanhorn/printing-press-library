package haneda

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"
)

const maxSnapshotBytes = int64(8 << 20)

// SaveSnapshot atomically writes complete scoped observations; existing files require explicit overwrite.
func SaveSnapshot(path string, s Snapshot, overwrite bool) error {
	if err := validateSnapshot(s); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if int64(len(data)) > maxSnapshotBytes {
		return fmt.Errorf("snapshot exceeds 8 MiB")
	}
	parent := filepath.Dir(path)
	if err = os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	if info, e := os.Lstat(path); e == nil {
		if !overwrite {
			return fmt.Errorf("snapshot destination already exists; choose another path or pass --overwrite")
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot destination must be a regular file")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	f, err := os.CreateTemp(parent, ".haneda-snapshot-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(append(data, '\n')); err != nil {
		return errors.Join(err, f.Close())
	}
	if err = f.Sync(); err != nil {
		return errors.Join(err, f.Close())
	}
	if err = f.Close(); err != nil {
		return err
	}
	if overwrite {
		return os.Rename(tmp, path)
	}
	// Linking an already-complete temp file gives an atomic, exclusive destination.
	if err = os.Link(tmp, path); err != nil {
		return fmt.Errorf("create snapshot destination exclusively: %w", err)
	}
	return nil
}

// LoadSnapshot refuses malformed/incomplete snapshots before presenting offline facts.
func LoadSnapshot(path string) (Snapshot, error) {
	var s Snapshot
	// #nosec G304 -- The operator selects a local snapshot; regular-file, byte-budget and schema checks immediately follow.
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return s, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxSnapshotBytes {
		return s, fmt.Errorf("snapshot must be a regular file under 8 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSnapshotBytes+1))
	if err != nil {
		return s, err
	}
	if int64(len(data)) > maxSnapshotBytes {
		return s, fmt.Errorf("snapshot exceeds 8 MiB")
	}
	if err = json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse snapshot: %w", err)
	}
	return s, validateSnapshot(s)
}
func validateSnapshot(s Snapshot) error {
	if s.Schema != SnapshotSchema {
		return fmt.Errorf("unsupported snapshot schema; expected %s", SnapshotSchema)
	}
	if !s.Board.Complete || s.Board.ScanCapHit {
		return fmt.Errorf("snapshot must contain a complete, unfiltered source scope")
	}
	if s.Board.Coverage.FlightLookup != "" || (s.Board.Coverage.QueryMode != "" && s.Board.Coverage.QueryMode != "board") {
		return fmt.Errorf("a flight lookup cannot be saved as an unfiltered board snapshot")
	}
	if err := validSnapshotTime(&s.SavedAt); err != nil {
		return fmt.Errorf("snapshot has invalid saved_at")
	}
	if err := validSnapshotTime(&s.Board.ObservedAt); err != nil {
		return fmt.Errorf("snapshot has invalid observed_at")
	}
	if len(s.Board.Sources) == 0 || s.Board.Flights == nil || len(s.Board.Flights) > 10000 {
		return fmt.Errorf("snapshot is missing source coverage or exceeds 10000 flight groups")
	}
	q := Query{Kind: s.Board.Coverage.Kind, Direction: s.Board.Coverage.Direction, Date: s.Board.Coverage.RequestedDate, Limit: 20, MaxScan: 10000}
	if err := ValidateQuery(q, time.Now(), false); err != nil {
		return fmt.Errorf("snapshot coverage: %w", err)
	}
	if q.Date == "" || s.Board.Coverage.Origin == "" {
		return fmt.Errorf("snapshot must identify its service date and source origin")
	}
	seen := map[string]bool{}
	count := 0
	expected := map[string]bool{}
	for _, kind := range kinds(s.Board.Coverage.Kind) {
		for _, direction := range directions(s.Board.Coverage.Direction) {
			expected[kind+"|"+direction] = true
		}
	}
	sourceCounts := map[string]int{}
	actualCounts := map[string]int{}
	for _, source := range s.Board.Sources {
		pair := source.Kind + "|" + source.Direction
		if !expected[pair] {
			return fmt.Errorf("snapshot contains a source outside its declared scope")
		}
		if _, duplicate := sourceCounts[pair]; duplicate {
			return fmt.Errorf("snapshot repeats a source scope")
		}
		sourceCounts[pair] = source.SourceTotal
		for _, stamp := range []*string{source.ReportedAt, source.UpdatedAt} {
			if err := validSnapshotTime(stamp); err != nil {
				return err
			}
		}
		if source.SourceTotal < 0 {
			return fmt.Errorf("snapshot has invalid source total")
		}
		count += source.SourceTotal
	}
	if len(sourceCounts) != len(expected) {
		return fmt.Errorf("snapshot is missing a required source kind/direction")
	}
	if count != len(s.Board.Flights) || count != s.Board.ScannedRecords {
		return fmt.Errorf("snapshot flight count does not cover every reported source record")
	}
	for _, f := range s.Board.Flights {
		if f.ID == "" || seen[f.ID] {
			return fmt.Errorf("snapshot has missing/duplicate group identity")
		}
		seen[f.ID] = true
		kind, direction, date, number, err := ParseIdentity(f.ID)
		if err != nil || kind != f.Kind || direction != f.Direction || date != f.ServiceDate || number != f.SourcePrimaryFlight {
			return fmt.Errorf("snapshot identity does not match its flight fields")
		}
		if (s.Board.Coverage.Kind != "all" && f.Kind != s.Board.Coverage.Kind) || (s.Board.Coverage.Direction != "both" && f.Direction != s.Board.Coverage.Direction) {
			return fmt.Errorf("snapshot flight lies outside its declared kind/direction")
		}
		actualCounts[f.Kind+"|"+f.Direction]++
		if len(f.ListedFlights) == 0 || f.ListedFlights[0].Number != f.SourcePrimaryFlight {
			return fmt.Errorf("snapshot must retain a nonempty ordered flight list aligned with its source primary")
		}
		for _, listed := range f.ListedFlights {
			n, err := NormalizeNumber(listed.Number)
			if err != nil || n != listed.Number {
				return fmt.Errorf("snapshot has invalid listed flight number")
			}
		}
		for _, stamp := range []*string{f.ScheduledAt, f.RevisedAt, f.ActualAt, f.SourceReportedAt} {
			if err := validSnapshotTime(stamp); err != nil {
				return err
			}
		}
		if f.ActualAt != nil || f.OperatingFlight != nil {
			return fmt.Errorf("this snapshot schema cannot establish actual times or operating identity from the supported source")
		}
		if f.Status.Known != (ptr(f.Status.Category) != nil || ptr(f.Status.Text) != nil) {
			return fmt.Errorf("snapshot status known flag contradicts its source fields")
		}
	}
	for pair, total := range sourceCounts {
		if actualCounts[pair] != total {
			return fmt.Errorf("snapshot per-source record count is inconsistent")
		}
	}
	return nil
}

func validSnapshotTime(stamp *string) error {
	if stamp == nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *stamp)
	if err != nil {
		return fmt.Errorf("snapshot contains an invalid nullable RFC3339 time")
	}
	_, offset := t.Zone()
	if offset != 9*60*60 {
		return fmt.Errorf("snapshot normalized Haneda timestamps must use JST +09:00")
	}
	return nil
}

type FieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}
type Change struct {
	ID     string        `json:"id"`
	Type   string        `json:"type"`
	Fields []FieldChange `json:"fields"`
}
type Diff struct {
	Coverage         Coverage `json:"coverage"`
	BeforeObservedAt string   `json:"before_observed_at"`
	AfterObservedAt  string   `json:"after_observed_at"`
	Changes          []Change `json:"changes"`
	TotalChanges     int      `json:"total_changes"`
	Notes            []string `json:"notes"`
}

// ValidateSnapshotQuery refuses to treat an unobserved scope as an empty match.
// Adjacent service-day rows remain available unless an explicit date narrows
// to the snapshot's source request day; that observation is not a full board
// for some other request date merely because it contains a rollover row.
func ValidateSnapshotQuery(s Snapshot, q Query) error {
	c := s.Board.Coverage
	if q.Date != "" && q.Date != c.RequestedDate {
		return fmt.Errorf("requested date %s is not covered by snapshot request date %s; choose a matching snapshot or query live data", q.Date, c.RequestedDate)
	}
	covered := map[string]bool{}
	for _, kind := range kinds(c.Kind) {
		for _, direction := range directions(c.Direction) {
			covered[kind+"|"+direction] = true
		}
	}
	for _, kind := range kinds(q.Kind) {
		for _, direction := range directions(q.Direction) {
			if !covered[kind+"|"+direction] {
				return fmt.Errorf("requested %s/%s is not covered by snapshot %s/%s; choose a matching snapshot or query live data", kind, direction, c.Kind, c.Direction)
			}
		}
	}
	return nil
}

// DiffSnapshots compares compatible complete observations and never labels disappearance as cancellation.
func DiffSnapshots(before, after Snapshot) (Diff, error) {
	r := Diff{Coverage: before.Board.Coverage, BeforeObservedAt: before.Board.ObservedAt, AfterObservedAt: after.Board.ObservedAt, Changes: []Change{}, Notes: []string{"A service absent from the later source is no_longer_reported, not proof of cancellation. Only source-declared status changes establish a reported cancellation."}}
	if err := validateSnapshot(before); err != nil {
		return r, err
	}
	if err := validateSnapshot(after); err != nil {
		return r, err
	}
	bc, ac := before.Board.Coverage, after.Board.Coverage
	if bc.Origin != ac.Origin || bc.RequestedDate != ac.RequestedDate || bc.Kind != ac.Kind || bc.Direction != ac.Direction {
		return r, fmt.Errorf("snapshot coverage mismatch; use the same origin, date, kind and direction")
	}
	// Valid v1 observations predating the explicit mode field still represent boards.
	if r.Coverage.QueryMode == "" {
		r.Coverage.QueryMode = "board"
	}
	a, _ := time.Parse(time.RFC3339, before.Board.ObservedAt)
	b, _ := time.Parse(time.RFC3339, after.Board.ObservedAt)
	if b.Before(a) {
		return r, fmt.Errorf("--after must be observed at or after --before")
	}
	old := map[string]Flight{}
	fresh := map[string]Flight{}
	for _, f := range before.Board.Flights {
		old[f.ID] = f
	}
	for _, f := range after.Board.Flights {
		fresh[f.ID] = f
	}
	for id, f := range fresh {
		p, ok := old[id]
		if !ok {
			r.Changes = append(r.Changes, Change{ID: id, Type: "appeared", Fields: []FieldChange{}})
			continue
		}
		changes := []FieldChange{}
		fields := []struct {
			name          string
			before, after any
		}{{"status", p.Status, f.Status}, {"scheduled_at", p.ScheduledAt, f.ScheduledAt}, {"revised_at", p.RevisedAt, f.RevisedAt}, {"actual_at", p.ActualAt, f.ActualAt}, {"terminal", p.Terminal, f.Terminal}, {"boarding_gates", p.BoardingGates, f.BoardingGates}, {"checkin_counters", p.CheckinCounters, f.CheckinCounters}, {"security_checks", p.SecurityChecks, f.SecurityChecks}, {"arrival_exits", p.ArrivalExits, f.ArrivalExits}, {"facilities", p.Facilities, f.Facilities}, {"listed_flights", p.ListedFlights, f.ListedFlights}, {"other_airport", p.Airport, f.Airport}}
		for _, v := range fields {
			if !reflect.DeepEqual(v.before, v.after) {
				changes = append(changes, FieldChange{Field: v.name, Before: v.before, After: v.after})
			}
		}
		if len(changes) > 0 {
			r.Changes = append(r.Changes, Change{ID: id, Type: "changed", Fields: changes})
		}
	}
	for id := range old {
		if _, ok := fresh[id]; !ok {
			r.Changes = append(r.Changes, Change{ID: id, Type: "no_longer_reported", Fields: []FieldChange{}})
		}
	}
	sort.Slice(r.Changes, func(i, j int) bool { return r.Changes[i].ID < r.Changes[j].ID })
	r.TotalChanges = len(r.Changes)
	return r, nil
}
