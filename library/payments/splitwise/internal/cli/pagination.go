// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
)

// PageResult is the byte-budgeted page envelope returned for list responses.
type PageResult struct {
	Items      []json.RawMessage `json:"items"`
	Total      int               `json:"total"`
	Offset     int               `json:"offset"`
	Returned   int               `json:"returned"`
	NextOffset *int              `json:"next_offset,omitempty"`
	Truncated  bool              `json:"truncated,omitempty"`
}

// PaginateByBytes returns items[offset:] up to the point where adding the next
// item's serialized length would exceed maxBytes. ALWAYS returns >=1 item when
// items remain (a single oversized item is returned alone with Truncated=true) so
// callers always make progress. A positive limit caps the count first; the byte
// budget trims further. NextOffset is set (to the next unreturned index) iff items
// remain beyond this page.
func PaginateByBytes(items []json.RawMessage, offset, limit, maxBytes int) PageResult {
	total := len(items)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}

	end := total
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	page := make([]json.RawMessage, 0)
	used := 0
	truncated := false
	i := offset
	for i < end {
		sz := len(items[i])
		if len(page) == 0 {
			page = append(page, items[i])
			used += sz
			i++
			if sz > maxBytes {
				truncated = true
				break
			}
			continue
		}
		if used+sz > maxBytes {
			break
		}
		page = append(page, items[i])
		used += sz
		i++
	}

	res := PageResult{
		Items:     page,
		Total:     total,
		Offset:    offset,
		Returned:  len(page),
		Truncated: truncated,
	}
	if i < total {
		next := i
		res.NextOffset = &next
	}
	return res
}

// ListFromResponse locates the list within a response body: a bare [...] array, or
// an array field of an object. If listField is non-empty it is used directly;
// otherwise the LONE array-valued field is auto-detected (more than one array field
// is ambiguous -> not a list). Returns (items, true) on success, (nil, false)
// otherwise (caller passes the body through unchanged).
func ListFromResponse(body json.RawMessage, listField string) ([]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, false
	}

	switch trimmed[0] {
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, false
		}
		return items, true
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil, false
		}

		if listField != "" {
			raw, ok := obj[listField]
			if !ok {
				return nil, false
			}
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, false
			}
			return items, true
		}

		var detected []json.RawMessage
		found := 0
		for _, raw := range obj {
			v := bytes.TrimSpace(raw)
			if len(v) == 0 || v[0] != '[' {
				continue
			}
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil {
				continue
			}
			detected = items
			found++
			if found > 1 {
				return nil, false
			}
		}
		if found == 1 {
			return detected, true
		}
		return nil, false
	default:
		return nil, false
	}
}

// PaginateBody locates a list in body and returns the byte-budgeted page envelope
// as JSON bytes plus paginated=true. When body is not a detectable list it returns
// (nil, false).
func PaginateBody(body json.RawMessage, offset, limit, maxBytes int, listField string) ([]byte, bool) {
	items, ok := ListFromResponse(body, listField)
	if !ok {
		return nil, false
	}

	paged := PaginateByBytes(items, offset, limit, maxBytes)
	out, err := json.Marshal(paged)
	if err != nil {
		return nil, false
	}
	return out, true
}
