package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Hand-written (not generated). wrapupAutoReleaseSeconds is 0 (off) or a whole number of seconds from 30 to 3600. pflag's IntVar would read
// 0x1e as 30 and let 29 or -1 through, and a JSON round trip turns 1e2 into 100, so the raw decimal text is checked before anything is sent.
var wrapupDigits = regexp.MustCompile(`^[0-9]{1,5}$`)

var errWrapupSeconds = errors.New("wrapupAutoReleaseSeconds must be 0 (off) or a whole number of seconds from 30 to 3600")

func parseWrapupSeconds(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if !wrapupDigits.MatchString(raw) {
		return 0, errWrapupSeconds
	}
	n, _ := strconv.Atoi(raw)
	if n != 0 && (n < 30 || n > 3600) {
		return 0, errWrapupSeconds
	}
	return n, nil
}

// wrapupSecondsFromJSON reads the field's raw JSON token from a --stdin body: only a plain digits-only number token is accepted
// (a string, null, 1e2, 12.5 or a negative is refused); nil when the body does not carry the field. A body that is not valid JSON is an error here (before any client exists), so it is a usage error, not a late parse failure.
func wrapupSecondsFromJSON(body []byte) (*int, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("parsing stdin JSON: %w", err)
	}
	raw, ok := fields["wrapupAutoReleaseSeconds"]
	if !ok {
		return nil, nil
	}
	n, err := parseWrapupSeconds(string(raw))
	if err != nil {
		return nil, err
	}
	return &n, nil
}
