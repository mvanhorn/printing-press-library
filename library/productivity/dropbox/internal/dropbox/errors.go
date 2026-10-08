package dropbox

import (
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
	"strings"
)

// ParseError extracts Dropbox's endpoint error summary from a response body.
func ParseError(body string) (string, bool) {
	var v struct {
		Summary string `json:"error_summary"`
	}
	if json.Unmarshal([]byte(body), &v) != nil || v.Summary == "" {
		return "", false
	}
	return v.Summary, true
}

func HasSummaryPrefix(err error, prefix string) bool {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 {
		return false
	}
	summary, ok := ParseError(apiErr.Body)
	return ok && strings.HasPrefix(strings.TrimRight(summary, "."), prefix)
}
