// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package cli

import "encoding/json"

// ipaEnvelopeOuter is the generator's response wrapper.
type ipaEnvelopeOuter struct {
	Data json.RawMessage `json:"data"`
}

// ipaEnvelopeInner is the IPA API response.
type ipaEnvelopeInner struct {
	Data   json.RawMessage `json:"data"`
	Result struct {
		CodErr   int    `json:"cod_err"`
		DescErr  string `json:"desc_err"`
		NumItems int    `json:"num_items"`
	} `json:"result"`
}

// ipaExtractItems extracts the data array from an IPA response.
// Handles both the generator envelope {"action","data":{"data":[...],...}}
// and the raw IPA response {"result":{...},"data":[...]}.
func ipaExtractItems(raw json.RawMessage) []map[string]any {
	if raw == nil {
		return nil
	}

	// Path 1: generator envelope — data.data is the IPA array
	var outer ipaEnvelopeOuter
	if json.Unmarshal(raw, &outer) == nil && outer.Data != nil {
		var inner ipaEnvelopeInner
		if json.Unmarshal(outer.Data, &inner) == nil && inner.Data != nil {
			var items []map[string]any
			if json.Unmarshal(inner.Data, &items) == nil {
				return items
			}
		}
		// Try outer.Data directly as array
		var items []map[string]any
		if json.Unmarshal(outer.Data, &items) == nil {
			return items
		}
	}

	// Path 2: raw IPA response — data is the array
	var ipaResp ipaEnvelopeInner
	if json.Unmarshal(raw, &ipaResp) == nil && ipaResp.Data != nil {
		var items []map[string]any
		if json.Unmarshal(ipaResp.Data, &items) == nil {
			return items
		}
	}

	// Path 3: direct array
	var items []map[string]any
	if json.Unmarshal(raw, &items) == nil {
		return items
	}

	return nil
}

// ipaExtractSingle extracts a single object from an IPA response.
func ipaExtractSingle(raw json.RawMessage) map[string]any {
	if raw == nil {
		return nil
	}

	// Path 1: generator envelope — data.data is the IPA object
	var outer ipaEnvelopeOuter
	if json.Unmarshal(raw, &outer) == nil && outer.Data != nil {
		var inner ipaEnvelopeInner
		if json.Unmarshal(outer.Data, &inner) == nil && inner.Data != nil {
			var item map[string]any
			if json.Unmarshal(inner.Data, &item) == nil {
				return item
			}
			// Try as array and return first element
			var items []map[string]any
			if json.Unmarshal(inner.Data, &items) == nil && len(items) > 0 {
				return items[0]
			}
		}
	}

	// Path 2: raw IPA response — data is the object
	var ipaResp ipaEnvelopeInner
	if json.Unmarshal(raw, &ipaResp) == nil && ipaResp.Data != nil {
		var item map[string]any
		if json.Unmarshal(ipaResp.Data, &item) == nil {
			return item
		}
		var items []map[string]any
		if json.Unmarshal(ipaResp.Data, &items) == nil && len(items) > 0 {
			return items[0]
		}
	}

	// Path 3: direct object
	var item map[string]any
	if json.Unmarshal(raw, &item) == nil {
		return item
	}

	return nil
}
