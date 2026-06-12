package cli

import (
	"encoding/json"
	"io"
)

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func jsonDecode(b []byte, v any) error { return json.Unmarshal(b, v) }
