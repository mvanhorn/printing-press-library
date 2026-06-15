package classroomwrite

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DescEquivalent compares two Skool [v2] TipTap desc strings semantically.
func DescEquivalent(got, want string) bool {
	gotNodes, err := parseDescNodes(got)
	if err != nil {
		return false
	}
	wantNodes, err := parseDescNodes(want)
	if err != nil {
		return false
	}
	gotJSON, err := json.Marshal(gotNodes)
	if err != nil {
		return false
	}
	wantJSON, err := json.Marshal(wantNodes)
	if err != nil {
		return false
	}
	return string(gotJSON) == string(wantJSON)
}

func parseDescNodes(desc string) ([]any, error) {
	desc = strings.TrimSpace(desc)
	if !strings.HasPrefix(desc, "[v2]") {
		return nil, fmt.Errorf("missing [v2] prefix")
	}
	payload := strings.TrimSpace(desc[4:])
	var nodes []any
	if err := json.Unmarshal([]byte(payload), &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}
