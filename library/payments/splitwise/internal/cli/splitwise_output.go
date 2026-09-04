package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func partialFailureErr(err error) error { return &cliError{code: 6, err: err} }

// emitStructured renders a hand-built structured value honoring output-format
// flags. It is the analytics-command counterpart to get-* commands that route
// typed output through printOutputWithFlags.
func (f *rootFlags) emitStructured(cmd *cobra.Command, v any) error {
	rawBytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	raw := json.RawMessage(rawBytes)
	if f.csv || f.plain {
		if f.selectFields != "" {
			raw = filterFields(raw, f.selectFields)
		}
		if f.compact {
			raw = compactFields(raw)
		}
	}

	if f.csv {
		if table, ok := structuredTableData(raw); ok {
			return printCSV(cmd.OutOrStdout(), table)
		}
		// No single tabular projection (e.g. an envelope with several arrays):
		// emit the JSON document rather than a one-row CSV whose cells are
		// Go-formatted slices (the 4.31.7 printCSV flattens any object).
		_, err := fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return err
	}
	if f.plain {
		if table, ok := structuredTableData(raw); ok {
			return printPlain(cmd.OutOrStdout(), table)
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return err
	}

	return f.printJSON(cmd, v)
}

func structuredTableData(raw json.RawMessage) (json.RawMessage, bool) {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return raw, tabularScalarRows(arr)
	}
	if unwrapped, ok := unwrapSingleArrayField(raw); ok {
		var rows []map[string]any
		if json.Unmarshal(unwrapped, &rows) == nil && tabularScalarRows(rows) {
			return unwrapped, true
		}
	}
	return nil, false
}

func tabularScalarRows(rows []map[string]any) bool {
	for _, row := range rows {
		for _, value := range row {
			switch value.(type) {
			case map[string]any, []any:
				return false
			}
		}
	}
	return true
}

func unwrapSingleArrayField(raw json.RawMessage) (json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	var arrayRaw json.RawMessage
	arrayCount := 0
	for k, v := range obj {
		if envelopeMetadataKeys[k] || envelopeMetadataArrayKeys[k] {
			continue
		}
		trimmed := string(json.RawMessage(v))
		if trimmed == "null" || trimmed == "[]" {
			continue
		}
		var arr []map[string]any
		if err := json.Unmarshal(v, &arr); err == nil {
			arrayRaw = v
			arrayCount++
		}
	}
	if arrayCount != 1 {
		return nil, false
	}
	return arrayRaw, true
}
