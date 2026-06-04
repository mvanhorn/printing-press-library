package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func updateDriftHistory(current scoredKeyword, days int) ([]scoredKeywordSnapshot, error) {
	path := filepath.Join(filepath.Dir(defaultDBPath("erank-pp-cli")), "keyword-drift.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	unlock, err := lockDriftHistory(path + ".lock")
	if err != nil {
		return nil, err
	}
	defer unlock()
	var history []scoredKeywordSnapshot
	// #nosec G304 -- path is a fixed file under the CLI-controlled data directory, not user input.
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &history); err != nil {
			return nil, fmt.Errorf("parse drift history: %w", err)
		}
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	filtered := make([]scoredKeywordSnapshot, 0, len(history)+1)
	for _, item := range history {
		if item.CapturedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, item)
	}
	filtered = append(filtered, scoredKeywordSnapshot{scoredKeyword: current, CapturedAt: time.Now().UTC()})
	raw, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, err
	}
	return filtered, nil
}

func lockDriftHistory(path string) (func(), error) {
	lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		_ = lockFile.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}, nil
}

type scoredKeywordSnapshot struct {
	scoredKeyword
	CapturedAt time.Time `json:"captured_at"`
}

func driftSummary(history []scoredKeywordSnapshot, current scoredKeyword, threshold float64) map[string]any {
	if len(history) < 2 {
		return map[string]any{"status": "baseline", "message": "First snapshot recorded; rerun later to detect drift."}
	}
	var oldest *scoredKeywordSnapshot
	for i := range history {
		if history[i].Keyword != current.Keyword || history[i].Source != current.Source || history[i].Country != current.Country {
			continue
		}
		if oldest == nil || history[i].CapturedAt.Before(oldest.CapturedAt) {
			oldest = &history[i]
		}
	}
	if oldest == nil {
		return map[string]any{"status": "baseline"}
	}
	change := math.Round((current.Score-oldest.Score)*10) / 10
	status := "stable"
	if math.Abs(change) >= threshold {
		status = "drift"
	}
	return map[string]any{"status": status, "score_change": change, "oldest_score": oldest.Score, "current_score": current.Score, "since": oldest.CapturedAt}
}
