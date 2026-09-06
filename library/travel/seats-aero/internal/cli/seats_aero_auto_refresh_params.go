package cli

import "strings"

var autoRefreshRequiredParams = map[string][]string{
	"availability": {"source"},
	"awards":       {"origin_airport", "destination_airport"},
	"routes":       {"source"},
}

func autoRefreshSkipReason(resource string) string {
	if required := autoRefreshRequiredParams[resource]; len(required) > 0 {
		return "missing_required_params: " + strings.Join(required, ", ")
	}
	return ""
}
