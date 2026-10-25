package cli

import "encoding/json"

// appResults unwraps AppTweak's {"result": {"<app id>": {...}}} response
// envelope and returns each app's raw payload. ok is false when the response
// does not carry that envelope.
func appResults(data json.RawMessage) (map[string]json.RawMessage, bool) {
	var top struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if json.Unmarshal(data, &top) != nil || top.Result == nil {
		return nil, false
	}
	return top.Result, true
}
