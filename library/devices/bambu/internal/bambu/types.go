package bambu

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"
	"time"
)

const CancelError = 50348044

var activeStates = map[string]bool{"PREPARE": true, "RUNNING": true, "PAUSE": true}
var terminalStates = map[string]bool{"IDLE": true, "FINISH": true, "FAILED": true}

type Discovery struct {
	Host   string `json:"host"`
	Serial string `json:"serial"`
	Model  string `json:"model,omitempty"`
	Name   string `json:"name,omitempty"`
}

type Snapshot struct {
	ObservedAt       time.Time      `json:"observed_at"`
	Serial           string         `json:"serial"`
	PrinterKey       string         `json:"printer_key,omitempty"`
	State            string         `json:"state"`
	JobName          string         `json:"job_name"`
	SubtaskName      string         `json:"subtask_name,omitempty"`
	GCodeFile        string         `json:"gcode_file,omitempty"`
	TaskID           string         `json:"task_id,omitempty"`
	SubtaskID        string         `json:"subtask_id,omitempty"`
	PlateNumber      *int           `json:"plate_number,omitempty"`
	Percent          *int           `json:"percent,omitempty"`
	RemainingMinutes *int           `json:"remaining_minutes,omitempty"`
	CurrentLayer     *int           `json:"current_layer,omitempty"`
	TotalLayers      *int           `json:"total_layers,omitempty"`
	PrintError       int64          `json:"print_error"`
	Raw              map[string]any `json:"raw,omitempty"`
}

type Event struct {
	Kind     string   `json:"kind"`
	Snapshot Snapshot `json:"snapshot"`
}

type Monitor struct {
	snapshot       Snapshot
	initialized    bool
	cancelReported bool
}

func NewMonitor(serial string) *Monitor {
	return &Monitor{snapshot: Snapshot{Serial: serial, State: "UNKNOWN", JobName: "unnamed print", Raw: map[string]any{}}}
}

func (m *Monitor) Snapshot() Snapshot { return m.snapshot }

func (m *Monitor) Ingest(payload []byte) ([]Event, error) {
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil, fmt.Errorf("decode MQTT report: %w", err)
	}
	report, ok := root["print"].(map[string]any)
	if !ok {
		return nil, nil
	}
	previous := m.snapshot
	current := mergeSnapshot(previous, report)
	m.snapshot = current
	_, stateObserved := report["gcode_state"]
	if !m.initialized {
		if stateObserved && current.State != "UNKNOWN" {
			m.initialized = true
			m.cancelReported = current.PrintError == CancelError
		}
		return nil, nil
	}
	events := make([]Event, 0, 1)
	canceledNow := current.PrintError == CancelError && previous.PrintError != CancelError
	if canceledNow {
		m.cancelReported = true
		events = append(events, Event{Kind: "canceled", Snapshot: current})
	}
	if current.State == previous.State {
		return events, nil
	}
	switch {
	case current.State == "IDLE":
		m.cancelReported = false
	case current.State == "FAILED" && !m.cancelReported:
		events = append(events, Event{Kind: "failed", Snapshot: current})
	case current.State == "FINISH" && !m.cancelReported:
		events = append(events, Event{Kind: "finished", Snapshot: current})
	case previous.State == "PAUSE" && current.State == "RUNNING":
		events = append(events, Event{Kind: "resumed", Snapshot: current})
	case current.State == "PAUSE":
		events = append(events, Event{Kind: "paused", Snapshot: current})
	case current.State == "RUNNING" && (terminalStates[previous.State] || previous.State == "PREPARE"):
		m.cancelReported = false
		events = append(events, Event{Kind: "started", Snapshot: current})
	}
	return events, nil
}

func mergeSnapshot(previous Snapshot, report map[string]any) Snapshot {
	current := previous
	current.ObservedAt = time.Now().UTC()
	if current.Raw == nil {
		current.Raw = map[string]any{}
	}
	for key, value := range report {
		current.Raw[key] = value
	}
	if state := normalizeState(report["gcode_state"]); state != "" {
		current.State = state
	}
	if value := stringValue(report["subtask_name"]); value != "" {
		current.SubtaskName = safeBasename(value)
		current.JobName = SanitizeJobName(value)
	} else if current.JobName == "unnamed print" {
		current.JobName = SanitizeJobName(report["gcode_file"])
	}
	if value := stringValue(report["gcode_file"]); value != "" {
		current.GCodeFile = safeBasename(value)
	}
	if value := stringValue(report["task_id"]); value != "" {
		current.TaskID = truncate(value, 128)
	}
	if value := stringValue(report["subtask_id"]); value != "" {
		current.SubtaskID = truncate(value, 128)
	}
	current.PlateNumber = mergePlateNumber(current.PlateNumber, report)
	current.Percent = mergeInt(current.Percent, report, "mc_percent", 0, 100)
	if current.Percent == nil {
		current.Percent = mergeInt(current.Percent, report, "percent", 0, 100)
	}
	current.RemainingMinutes = mergeInt(current.RemainingMinutes, report, "mc_remaining_time", 0, 10080)
	if current.RemainingMinutes == nil {
		current.RemainingMinutes = mergeInt(current.RemainingMinutes, report, "remain_time", 0, 10080)
	}
	current.CurrentLayer = mergeInt(current.CurrentLayer, report, "layer_num", 0, 1000000)
	current.TotalLayers = mergeInt(current.TotalLayers, report, "total_layer_num", 0, 1000000)
	if value, ok := int64Value(report["print_error"]); ok && value >= 0 && value <= math.MaxUint32 {
		current.PrintError = value
	}
	for _, key := range []string{"fail_reason", "mc_print_error_code"} {
		if value, ok := int64Value(report[key]); ok && value == CancelError {
			current.PrintError = CancelError
			break
		}
	}
	return current
}

func mergePlateNumber(previous *int, report map[string]any) *int {
	if value := mergeInt(nil, report, "plate_num", 1, 999); value != nil {
		return value
	}
	for _, key := range []string{"plate_idx", "plate_id"} {
		if value := mergeInt(nil, report, key, 1, 999); value != nil {
			return value
		}
	}
	return previous
}

func SanitizeJobName(value any) string {
	text := safeBasename(stringValue(value))
	lower := strings.ToLower(text)
	for _, suffix := range []string{".gcode.3mf", ".3mf", ".gcode"} {
		if strings.HasSuffix(lower, suffix) {
			text = text[:len(text)-len(suffix)]
			break
		}
	}
	text = strings.NewReplacer("`", "", "_", " ", "@", "@\u200b").Replace(text)
	text = strings.Trim(strings.Join(strings.Fields(text), " "), " .")
	if text == "" {
		return "unnamed print"
	}
	return truncate(text, 120)
}

func normalizeState(value any) string {
	state := strings.ToUpper(strings.TrimSpace(stringValue(value)))
	if activeStates[state] || terminalStates[state] {
		return state
	}
	return ""
}

func mergeInt(previous *int, report map[string]any, key string, minimum, maximum int) *int {
	value, exists := report[key]
	if !exists {
		return previous
	}
	n, ok := int64Value(value)
	if !ok {
		return previous
	}
	if n < int64(minimum) {
		n = int64(minimum)
	}
	if n > int64(maximum) {
		n = int64(maximum)
	}
	result := int(n)
	return &result
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case json.Number:
		n, err := typed.Int64()
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return n, err == nil
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	default:
		return 0, false
	}
}

func safeBasename(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	var clean strings.Builder
	for _, r := range value {
		if r >= 32 && r != 127 {
			clean.WriteRune(r)
		}
	}
	return truncate(path.Base(clean.String()), 180)
}

func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

// Redact recursively removes credential and device-serial fields from values
// before they are persisted or returned through the raw escape hatch.
func Redact(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			lower := strings.ToLower(strings.TrimSpace(key))
			if lower == "sn" || strings.Contains(lower, "serial") || strings.Contains(lower, "access_code") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") || strings.Contains(lower, "token") {
				result[key] = "[REDACTED]"
				continue
			}
			result[key] = Redact(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, child := range typed {
			result[i] = Redact(child)
		}
		return result
	default:
		return value
	}
}

func SanitizeSnapshot(snapshot Snapshot) Snapshot {
	if snapshot.PrinterKey == "" && snapshot.Serial != "" {
		snapshot.PrinterKey = PrinterKey(snapshot.Serial)
	}
	snapshot.Serial = ""
	if redacted, ok := Redact(snapshot.Raw).(map[string]any); ok {
		snapshot.Raw = redacted
	}
	return snapshot
}

func PrinterKey(serial string) string {
	serial = strings.ToUpper(strings.TrimSpace(serial))
	if serial == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(serial))
	return fmt.Sprintf("printer-%x", sum[:16])
}

func LegacyPrinterKeys(serial string) []string {
	serial = strings.ToUpper(strings.TrimSpace(serial))
	if serial == "" {
		return nil
	}
	serialSum := sha256.Sum256([]byte(serial))
	short := fmt.Sprintf("printer-%x", serialSum[:4])
	expandedSum := sha256.Sum256([]byte(short))
	return []string{short, fmt.Sprintf("printer-%x", expandedSum[:16])}
}
