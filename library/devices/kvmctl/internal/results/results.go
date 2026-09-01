// Package results defines the stable JSON envelope shared by user-facing operations.
package results

import "regexp"

type Error struct {
	Code          string `json:"code"`
	Retryable     bool   `json:"retryable"`
	RequiresHuman bool   `json:"requires_human"`
}

type Operation struct {
	Operation   string         `json:"operation"`
	Target      *string        `json:"target"`
	Transport   string         `json:"transport"`
	ReadOnly    bool           `json:"read_only"`
	OK          bool           `json:"ok"`
	Changed     bool           `json:"changed"`
	State       string         `json:"state"`
	Evidence    map[string]any `json:"evidence"`
	Warnings    []string       `json:"warnings"`
	Error       *Error         `json:"error"`
	NextActions []string       `json:"next_actions"`
}

var sensitive = regexp.MustCompile(`(?i)(password|passwd|token|secret|private[_ -]?key|credential|api[_ -]?key|authorization|cookie)`)

func safe(v any) any {
	switch x := v.(type) {
	case map[string]any:
		o := make(map[string]any, len(x))
		for k, v := range x {
			if !sensitive.MatchString(k) {
				o[k] = safe(v)
			}
		}
		return o
	case []any:
		o := make([]any, len(x))
		for i, v := range x {
			o[i] = safe(v)
		}
		return o
	default:
		return v
	}
}

func Build(operation, transport string, readOnly bool, target string, ok, changed bool, state string, evidence map[string]any, err *Error) Operation {
	var tp *string
	if target != "" {
		tp = &target
	}
	if evidence == nil {
		evidence = map[string]any{}
	}
	return Operation{Operation: operation, Target: tp, Transport: transport, ReadOnly: readOnly, OK: ok, Changed: changed, State: state, Evidence: safe(evidence).(map[string]any), Warnings: []string{}, Error: err, NextActions: []string{}}
}

func NormalizeError(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok && e != nil {
		return e.Code
	}
	return "operation failed"
}

func FromLegacy(operation, transport string, readOnly bool, target string, ok bool, evidence map[string]any, err *Error) Operation {
	return Build(operation, transport, readOnly, target, ok, false, "unknown", evidence, err)
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}
