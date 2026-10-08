package dropbox

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EncodeAPIArg encodes a Dropbox-API-Arg header using ASCII-only JSON.
func EncodeAPIArg(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, r := range string(b) {
		if r <= 0x7e {
			out.WriteRune(r)
			continue
		}
		if r <= 0xffff {
			fmt.Fprintf(&out, "\\u%04x", r)
			continue
		}
		r -= 0x10000
		fmt.Fprintf(&out, "\\u%04x\\u%04x", 0xd800+(r>>10), 0xdc00+(r&0x3ff))
	}
	return out.String(), nil
}
