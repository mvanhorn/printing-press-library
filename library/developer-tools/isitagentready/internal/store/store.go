// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// Package store is the hand-authored local scan-history layer for
// isitagentready-pp-cli. The upstream API (POST /api/scan) is stateless and
// has no list endpoint, so there is nothing for the generator to sync. Instead
// every scan the CLI runs is appended here as one JSONL line, and the novel
// commands (history, diff, compare, gate, open-advice, batch) read this store
// to answer questions the stateless web UI cannot.
//
// The decision logic (DiffChecks, EvaluateGate, RankRecords, OpenAdvice,
// BuildHistory, BuildCompare) lives here as pure functions so it is unit
// testable without a network or a Cobra command.
package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
)

// storeMu guards the JSONL store so concurrent scans (e.g. compare's parallel
// fan-out) cannot interleave a write with another write or read within this
// process. Append takes the write lock; Load takes the read lock.
var storeMu sync.RWMutex

// ScanRecord is one persisted scan: one line in the JSONL store. Raw holds the
// full upstream report so any command can reconstruct any view offline.
type ScanRecord struct {
	URL       string          `json:"url"`
	ScannedAt string          `json:"scannedAt"`
	Level     int             `json:"level"`
	LevelName string          `json:"levelName"`
	Raw       json.RawMessage `json:"raw"`
}

// Report is the parsed shape of a POST /api/scan response.
type Report struct {
	URL             string                      `json:"url"`
	ScannedAt       string                      `json:"scannedAt"`
	Level           int                         `json:"level"`
	LevelName       string                      `json:"levelName"`
	Checks          map[string]map[string]Check `json:"checks"`
	NextLevel       NextLevel                   `json:"nextLevel"`
	IsCommerce      bool                        `json:"isCommerce"`
	CommerceSignals []json.RawMessage           `json:"commerceSignals"`
	SiteError       *SiteError                  `json:"siteError"`
}

// Check is one readiness check inside a category.
type Check struct {
	Status     string          `json:"status"`
	Message    string          `json:"message"`
	DurationMs int             `json:"durationMs"`
	Evidence   json.RawMessage `json:"evidence,omitempty"`
}

// NextLevel describes the gap to the next readiness rung. Level can be null in
// the API response, so it is a pointer.
type NextLevel struct {
	Name         string        `json:"name"`
	Level        *int          `json:"level"`
	Target       *int          `json:"target"`
	Requirements []Requirement `json:"requirements"`
}

// Requirement is one prioritized fix (the advice) to reach the next level.
type Requirement struct {
	Check       string   `json:"check"`
	Description string   `json:"description"`
	ShortPrompt string   `json:"shortPrompt"`
	Prompt      string   `json:"prompt"`
	SpecURLs    []string `json:"specUrls"`
	SkillURL    string   `json:"skillUrl"`
}

// SiteError is returned (with HTTP 200) when the scanner could not fetch the
// target site. The scan endpoint itself succeeded; the target errored.
type SiteError struct {
	HTTPStatus  int    `json:"httpStatus"`
	StatusText  string `json:"statusText"`
	BodyPreview string `json:"bodyPreview"`
	Server      string `json:"server"`
}

// ParseReport decodes a raw scan response into a Report.
func ParseReport(raw json.RawMessage) (*Report, error) {
	var r Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parsing scan report: %w", err)
	}
	return &r, nil
}

// CheckRef is a flattened check with its category and id.
type CheckRef struct {
	Category string `json:"category"`
	ID       string `json:"check"`
	Check
}

// FlatChecks returns every check across categories, sorted by category then id.
func (r *Report) FlatChecks() []CheckRef {
	var out []CheckRef
	for cat, checks := range r.Checks {
		for id, c := range checks {
			out = append(out, CheckRef{Category: cat, ID: id, Check: c})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// statusMap returns checkID -> status across all categories.
func (r *Report) statusMap() map[string]string {
	m := map[string]string{}
	for _, checks := range r.Checks {
		for id, c := range checks {
			m[id] = c.Status
		}
	}
	return m
}

// FailingChecks returns the ids of checks whose status is "fail", sorted.
func (r *Report) FailingChecks() []string {
	var out []string
	for id, s := range r.statusMap() {
		if s == "fail" {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Counts returns pass/fail/neutral/other tallies across all checks.
func (r *Report) Counts() (pass, fail, neutral, total int) {
	for _, s := range r.statusMap() {
		total++
		switch s {
		case "pass":
			pass++
		case "fail":
			fail++
		case "neutral":
			neutral++
		}
	}
	return
}

// RequirementFor returns the next-level fix requirement for a check id.
func (r *Report) RequirementFor(checkID string) (Requirement, bool) {
	for _, req := range r.NextLevel.Requirements {
		if req.Check == checkID {
			return req, true
		}
	}
	return Requirement{}, false
}

// --- Persistence (JSONL) ---

// DefaultPath returns the JSONL scan-store path under the CLI data dir.
func DefaultPath() (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "scans.jsonl"), nil
}

// Append writes one scan record as a JSONL line, creating the store if needed.
// It is safe for concurrent callers within a process.
func Append(path string, rec ScanRecord) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G304 -- path is always store.DefaultPath() (CLI data dir + the
	// literal "scans.jsonl"); it never carries untrusted user or network input.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// Load reads every scan record from the JSONL store. A missing store is not an
// error (returns an empty slice). Corrupt lines are skipped.
func Load(path string) ([]ScanRecord, error) {
	storeMu.RLock()
	defer storeMu.RUnlock()
	// #nosec G304 -- path is always store.DefaultPath() (CLI data dir + the
	// literal "scans.jsonl"); it never carries untrusted user or network input.
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var recs []ScanRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // raw reports run ~25-30KB
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec ScanRecord
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		recs = append(recs, rec)
	}
	return recs, sc.Err()
}

// --- URL matching ---

// NormalizeURL lowercases, drops the scheme, and trims a trailing slash so the
// same site matches whether the user typed https://example.com or example.com/.
func NormalizeURL(u string) string {
	u = strings.TrimSpace(strings.ToLower(u))
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimSuffix(u, "/")
	return u
}

// MatchURL reports whether two URLs refer to the same site.
func MatchURL(a, b string) bool { return NormalizeURL(a) == NormalizeURL(b) }

// HistoryFor returns every scan of a URL, oldest first.
func HistoryFor(recs []ScanRecord, url string) []ScanRecord {
	var out []ScanRecord
	for _, r := range recs {
		if MatchURL(r.URL, url) {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return cliutil.ParseStoredTime(out[i].ScannedAt).Before(cliutil.ParseStoredTime(out[j].ScannedAt))
	})
	return out
}

// Latest returns the most recent scan of a URL.
func Latest(recs []ScanRecord, url string) (ScanRecord, bool) {
	h := HistoryFor(recs, url)
	if len(h) == 0 {
		return ScanRecord{}, false
	}
	return h[len(h)-1], true
}

// LatestPerURL returns the newest scan for each distinct URL, sorted by URL.
func LatestPerURL(recs []ScanRecord) []ScanRecord {
	byURL := map[string]ScanRecord{}
	for _, r := range recs {
		key := NormalizeURL(r.URL)
		cur, ok := byURL[key]
		if !ok || cliutil.ParseStoredTime(r.ScannedAt).After(cliutil.ParseStoredTime(cur.ScannedAt)) {
			byURL[key] = r
		}
	}
	out := make([]ScanRecord, 0, len(byURL))
	for _, r := range byURL {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return NormalizeURL(out[i].URL) < NormalizeURL(out[j].URL) })
	return out
}

// --- Decision logic (pure) ---

// statusRank orders statuses so a higher rank is "more ready".
func statusRank(s string) int {
	switch s {
	case "pass":
		return 2
	case "neutral":
		return 1
	default: // fail and unknown
		return 0
	}
}

// CheckTransition is one check's status change between two scans.
type CheckTransition struct {
	Check  string `json:"check"`
	From   string `json:"from"`
	To     string `json:"to"`
	Change string `json:"change"` // regressed | improved | unchanged | added | removed
}

// DiffChecks compares two reports and returns per-check transitions, sorted by
// check id. "regressed" means the check became less ready; "improved" more.
func DiffChecks(from, to *Report) []CheckTransition {
	fromS := from.statusMap()
	toS := to.statusMap()
	seen := map[string]bool{}
	var ids []string
	for id := range fromS {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for id := range toS {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := make([]CheckTransition, 0, len(ids))
	for _, id := range ids {
		f, fok := fromS[id]
		t, tok := toS[id]
		var change string
		switch {
		case !fok && tok:
			change = "added"
		case fok && !tok:
			change = "removed"
		case statusRank(t) < statusRank(f):
			change = "regressed"
		case statusRank(t) > statusRank(f):
			change = "improved"
		default:
			change = "unchanged"
		}
		out = append(out, CheckTransition{Check: id, From: f, To: t, Change: change})
	}
	return out
}

// regressedChecks returns checks that went from pass to not-pass.
func regressedChecks(from, to *Report) []string {
	var out []string
	for _, tr := range DiffChecks(from, to) {
		if tr.Change == "regressed" && tr.From == "pass" {
			out = append(out, tr.Check)
		}
	}
	sort.Strings(out)
	return out
}

// GateResult is the outcome of a CI gate evaluation.
type GateResult struct {
	Pass        bool     `json:"pass"`
	URL         string   `json:"url"`
	Level       int      `json:"level"`
	LevelName   string   `json:"levelName"`
	MinLevel    int      `json:"minLevel"`
	SiteError   bool     `json:"siteError"`
	Regressions []string `json:"regressions,omitempty"`
	Reasons     []string `json:"reasons"`
}

// EvaluateGate decides whether a scan passes the gate. A target siteError is a
// distinct outcome: it does not fail the gate unless strict is set, so a
// transient target outage does not flap CI. prev may be nil (no baseline).
func EvaluateGate(latest, prev *Report, minLevel int, noRegress, strict bool) GateResult {
	res := GateResult{Pass: true, URL: latest.URL, Level: latest.Level, LevelName: latest.LevelName, MinLevel: minLevel}
	if latest.SiteError != nil {
		res.SiteError = true
		if strict {
			res.Pass = false
			res.Reasons = append(res.Reasons, fmt.Sprintf("target site error: HTTP %d %s", latest.SiteError.HTTPStatus, latest.SiteError.StatusText))
		} else {
			res.Reasons = append(res.Reasons, "target site unreachable (siteError); not failing gate without --strict")
		}
		return res
	}
	if minLevel > 0 && latest.Level < minLevel {
		res.Pass = false
		res.Reasons = append(res.Reasons, fmt.Sprintf("level %d is below the required minimum %d", latest.Level, minLevel))
	}
	if noRegress && prev != nil && prev.SiteError == nil {
		if regr := regressedChecks(prev, latest); len(regr) > 0 {
			res.Pass = false
			res.Regressions = regr
			res.Reasons = append(res.Reasons, fmt.Sprintf("%d check(s) regressed since the last scan: %s", len(regr), strings.Join(regr, ", ")))
		}
	}
	if res.Pass && len(res.Reasons) == 0 {
		if minLevel > 0 {
			res.Reasons = append(res.Reasons, fmt.Sprintf("level %d meets the required minimum %d", latest.Level, minLevel))
		} else {
			res.Reasons = append(res.Reasons, "scan completed; no gate condition failed")
		}
	}
	return res
}

// OpenItem is one still-open fix across the portfolio.
type OpenItem struct {
	URL         string `json:"url"`
	Level       int    `json:"level"`
	LevelName   string `json:"levelName"`
	Check       string `json:"check"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	SkillURL    string `json:"skillUrl,omitempty"`
}

// OpenAdvice returns every still-open fix (next-level requirement) across the
// given latest-per-URL scans. filterCheck and filterURL narrow the result when
// non-empty.
func OpenAdvice(latestPerURL []ScanRecord, filterURL, filterCheck string) []OpenItem {
	out := make([]OpenItem, 0)
	for _, rec := range latestPerURL {
		if filterURL != "" && !MatchURL(rec.URL, filterURL) {
			continue
		}
		rep, err := ParseReport(rec.Raw)
		if err != nil {
			continue
		}
		for _, req := range rep.NextLevel.Requirements {
			if filterCheck != "" && req.Check != filterCheck {
				continue
			}
			out = append(out, OpenItem{
				URL:         rep.URL,
				Level:       rep.Level,
				LevelName:   rep.LevelName,
				Check:       req.Check,
				Description: req.Description,
				Prompt:      req.Prompt,
				SkillURL:    req.SkillURL,
			})
		}
	}
	return out
}

// failingCount parses a record and returns its number of failing checks.
func failingCount(rec ScanRecord) int {
	rep, err := ParseReport(rec.Raw)
	if err != nil {
		return 0
	}
	return len(rep.FailingChecks())
}

// RankRecords sorts scans worst-first. by="failing" ranks by failing-check
// count (desc); any other value ranks by level (asc).
func RankRecords(recs []ScanRecord, by string) []ScanRecord {
	out := append([]ScanRecord(nil), recs...)
	if by == "failing" {
		fc := map[string]int{}
		for _, r := range out {
			fc[NormalizeURL(r.URL)] = failingCount(r)
		}
		sort.SliceStable(out, func(i, j int) bool {
			fi, fj := fc[NormalizeURL(out[i].URL)], fc[NormalizeURL(out[j].URL)]
			if fi != fj {
				return fi > fj
			}
			return out[i].Level < out[j].Level
		})
		return out
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Level != out[j].Level {
			return out[i].Level < out[j].Level
		}
		return NormalizeURL(out[i].URL) < NormalizeURL(out[j].URL)
	})
	return out
}

// HistoryEntry is one row of a URL's readiness timeline.
type HistoryEntry struct {
	ScannedAt string            `json:"scannedAt"`
	Level     int               `json:"level"`
	LevelName string            `json:"levelName"`
	SiteError bool              `json:"siteError,omitempty"`
	Flips     []CheckTransition `json:"flips,omitempty"`
}

// BuildHistory turns an ascending history into timeline entries, computing the
// per-check flips against the previous scan. filterCheck restricts flips to one
// check when non-empty.
func BuildHistory(history []ScanRecord, filterCheck string) []HistoryEntry {
	out := make([]HistoryEntry, 0, len(history))
	var prev *Report
	for _, rec := range history {
		rep, err := ParseReport(rec.Raw)
		if err != nil {
			continue
		}
		entry := HistoryEntry{ScannedAt: rec.ScannedAt, Level: rep.Level, LevelName: rep.LevelName, SiteError: rep.SiteError != nil}
		if prev != nil && prev.SiteError == nil && rep.SiteError == nil {
			for _, tr := range DiffChecks(prev, rep) {
				if tr.Change == "unchanged" {
					continue
				}
				if filterCheck != "" && tr.Check != filterCheck {
					continue
				}
				entry.Flips = append(entry.Flips, tr)
			}
		}
		out = append(out, entry)
		prev = rep
	}
	return out
}

// CompareSite is one site's headline in a comparison.
type CompareSite struct {
	URL       string `json:"url"`
	Level     int    `json:"level"`
	LevelName string `json:"levelName"`
	SiteError bool   `json:"siteError,omitempty"`
}

// CompareRow is one check's status across the compared sites.
type CompareRow struct {
	Check    string            `json:"check"`
	Category string            `json:"category"`
	Statuses map[string]string `json:"statuses"` // normalized url -> status
}

// CompareResult is the full comparison matrix.
type CompareResult struct {
	Sites  []CompareSite `json:"sites"`
	Checks []CompareRow  `json:"checks"`
}

// BuildCompare builds a check-by-check matrix across the given reports, in the
// order provided.
func BuildCompare(reports []*Report) CompareResult {
	res := CompareResult{}
	category := map[string]string{}
	seen := map[string]bool{}
	var checkIDs []string
	for _, rep := range reports {
		res.Sites = append(res.Sites, CompareSite{URL: rep.URL, Level: rep.Level, LevelName: rep.LevelName, SiteError: rep.SiteError != nil})
		for _, cr := range rep.FlatChecks() {
			if !seen[cr.ID] {
				seen[cr.ID] = true
				checkIDs = append(checkIDs, cr.ID)
				category[cr.ID] = cr.Category
			}
		}
	}
	sort.SliceStable(checkIDs, func(i, j int) bool {
		if category[checkIDs[i]] != category[checkIDs[j]] {
			return category[checkIDs[i]] < category[checkIDs[j]]
		}
		return checkIDs[i] < checkIDs[j]
	})
	for _, id := range checkIDs {
		row := CompareRow{Check: id, Category: category[id], Statuses: map[string]string{}}
		for _, rep := range reports {
			st := rep.statusMap()
			if s, ok := st[id]; ok {
				row.Statuses[NormalizeURL(rep.URL)] = s
			} else {
				row.Statuses[NormalizeURL(rep.URL)] = "-"
			}
		}
		res.Checks = append(res.Checks, row)
	}
	return res
}
