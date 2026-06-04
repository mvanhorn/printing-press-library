package advisor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogEntry struct {
	SchemaVersion int             `json:"schema_version"`
	AdvisedAt     time.Time       `json:"advised_at"`
	PromptHash    string          `json:"prompt_hash"`
	PromptBytes   int             `json:"prompt_bytes"`
	TaskHint      string          `json:"task_hint,omitempty"`
	Features      Features        `json:"features"`
	Recommended   string          `json:"recommended"`
	Alternatives  []Candidate     `json:"alternatives,omitempty"`
	EstCostUSD    float64         `json:"est_cost_usd"`
	EstLatencyMs  int             `json:"est_latency_ms"`
	TiebreakUsed  bool            `json:"tiebreak_used,omitempty"`
	ActualChosen  string          `json:"actual_chosen,omitempty"`
	JudgeScore    float64         `json:"judge_score,omitempty"`
	Extra         json.RawMessage `json:"extra,omitempty"`
}

var logMu sync.Mutex

func AppendLog(path string, entry LogEntry) error {
	if path == "" {
		return nil
	}
	if entry.SchemaVersion == 0 {
		entry.SchemaVersion = SchemaVersion
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("advisor log: mkdir: %w", err)
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("advisor log: marshal: %w", err)
	}
	if len(b) > 3500 {
		return fmt.Errorf("advisor log: entry %d bytes exceeds atomic-append safety (~3500)", len(b))
	}
	logMu.Lock()
	defer logMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("advisor log: open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("advisor log: write: %w", err)
	}
	return nil
}

func DefaultLogPath() string {
	if v := os.Getenv("OLLAMA_CLOUD_PP_CLI_ADVISOR_LOG"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "ollama-cloud-pp-cli", "advisor-log.jsonl")
}
