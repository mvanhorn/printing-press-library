package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// decodeOutputJSON preserves JSON integer lexemes while output helpers inspect
// objects. An intermediate float64 would corrupt Google Ads int64 values.
func decodeOutputJSON(data []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("multiple JSON values in output")
	}
	return nil
}
