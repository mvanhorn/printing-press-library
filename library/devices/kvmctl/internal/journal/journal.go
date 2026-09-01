package journal

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"sync"
)

var secretKey = regexp.MustCompile(`(?i)(password|passwd|token|secret|private[_ -]?key|credential|api[_ -]?key|authorization|cookie)`)

type Journal struct {
	Path           string
	MaxRecordBytes int
	mu             sync.Mutex
}

func sanitize(v any, depth int) any {
	if depth > 8 {
		return "<maximum nesting depth exceeded>"
	}
	switch x := v.(type) {
	case nil, string, bool, int:
		return x
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
		return x
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			if !secretKey.MatchString(k) {
				out[k] = sanitize(v, depth+1)
			}
		}
		return out
	case []any:
		if len(x) > 256 {
			x = x[:256]
		}
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = sanitize(v, depth+1)
		}
		return out
	case []byte:
		return "<bytes omitted>"
	default:
		return "<unsupported>"
	}
}
func (j *Journal) Append(record map[string]any) error {
	max := j.MaxRecordBytes
	if max == 0 {
		max = 65536
	}
	data, err := json.Marshal(sanitize(record, 0))
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > max {
		return os.ErrInvalid
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := os.OpenFile(j.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// Checkpoint is the host-reboot style helper that prevents caller-supplied
// details from spoofing operation/target/transition.
func (j *Journal) Checkpoint(operation, target, transition string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	// Remove reserved keys if caller tried to spoof.
	delete(details, "operation")
	delete(details, "target")
	delete(details, "transition")
	rec := map[string]any{"operation": operation, "target": target, "transition": transition}
	for k, v := range details {
		rec[k] = v
	}
	return j.Append(rec)
}
