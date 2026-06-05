// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const paginatedGetMaxPages = 100

// paginatedGet fetches pages and concatenates array results. The headers
// argument carries per-endpoint required headers (e.g. cal-api-version) that
// must be sent on every page request, including the first; pass nil when the
// endpoint has no per-endpoint header overrides.
func paginatedGet(ctx context.Context, c interface {
	GetWithHeaders(ctx context.Context, path string, params map[string]string, headers map[string]string) (json.RawMessage, error)
}, path string, params map[string]string, headers map[string]string, fetchAll bool, cursorParam, paginationType, limitParam, nextCursorPath, hasMoreField string) (json.RawMessage, error) {
	// Cursor params are exempt from the "0"/"false" strip: offset-paginated
	// APIs send offset=0 on the first page.
	clean := map[string]string{}
	for k, v := range params {
		if v == "" {
			continue
		}
		if k == cursorParam || (v != "0" && v != "false") {
			clean[k] = v
		}
	}

	if !fetchAll {
		data, err := c.GetWithHeaders(ctx, path, clean, headers)
		if err != nil {
			return nil, err
		}
		emitTruncationWarning(data, nextCursorPath, hasMoreField, paginationType)
		return data, nil
	}

	// Fetch all pages
	allItems := make([]json.RawMessage, 0)
	page := 0
	for {
		page++
		if humanFriendly {
			fmt.Fprintf(os.Stderr, "fetching page %d...\n", page)
		} else {
			fmt.Fprintf(os.Stderr, `{"event":"page_fetch","page":%d}`+"\n", page)
		}

		data, err := c.GetWithHeaders(ctx, path, clean, headers)
		if err != nil {
			return nil, err
		}

		// Try to extract items array
		var items []json.RawMessage
		if json.Unmarshal(data, &items) == nil {
			allItems = append(allItems, items...)
		} else {
			// Response is an object - look for array inside
			var obj map[string]json.RawMessage
			if json.Unmarshal(data, &obj) == nil {
				if nested, ok := extractPaginatedItems(obj); ok {
					allItems = append(allItems, nested...)
				}

				// Check for next cursor
				if nextCursorPath != "" {
					if tokenRaw, ok := rawAtPath(obj, nextCursorPath); ok {
						if token := paginationCursorToken(tokenRaw); token != "" {
							if page >= paginatedGetMaxPages {
								emitPaginatedGetMaxPagesWarning()
								break
							}
							clean[cursorParam] = token
							continue
						}
					}
				}

				// Check has_more. Page and offset paginators can advance
				// client-side; cursor-based APIs still need a body cursor.
				if hasMoreField != "" {
					if moreRaw, ok := rawAtPath(obj, hasMoreField); ok {
						var more bool
						if json.Unmarshal(moreRaw, &more) == nil && more {
							if next, ok := nextClientSidePaginationCursor(clean, cursorParam, paginationType, limitParam); ok {
								if page >= paginatedGetMaxPages {
									emitPaginatedGetMaxPagesWarning()
									break
								}
								clean[cursorParam] = next
								continue
							}
							emitMissingPaginationCursorWarning(nextCursorPath)
							break
						}
					}
				}
			}
			// No more pages
			break
		}

		// For direct arrays, can't paginate without cursor
		break
	}

	if fetchAll && page == 1 && nextCursorPath == "" && hasMoreField == "" {
		emitMissingPaginationSignalWarning()
	}
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "fetched %d items across %d pages\n", len(allItems), page)
	} else {
		fmt.Fprintf(os.Stderr, `{"event":"complete","total":%d,"pages":%d}`+"\n", len(allItems), page)
	}
	result, _ := json.Marshal(allItems)
	return json.RawMessage(result), nil
}

func nextClientSidePaginationCursor(params map[string]string, cursorParam, paginationType, limitParam string) (string, bool) {
	if cursorParam == "" {
		return "", false
	}
	switch paginationType {
	case "page":
		current := params[cursorParam]
		if current == "" {
			current = "1"
		}
		n, err := strconv.Atoi(current)
		if err != nil {
			return "", false
		}
		return strconv.Itoa(n + 1), true
	case "offset":
		current := params[cursorParam]
		if current == "" {
			current = "0"
		}
		n, err := strconv.Atoi(current)
		if err != nil {
			return "", false
		}
		limit, err := strconv.Atoi(params[limitParam])
		if err != nil || limit <= 0 {
			return "", false
		}
		return strconv.Itoa(n + limit), true
	default:
		return "", false
	}
}

// Silent page-1 truncation is the worst-possible mode for agents,
// who otherwise compute totals against an incomplete set without
// passing --all.
func emitTruncationWarning(data json.RawMessage, nextCursorPath, hasMoreField, paginationType string) {
	if nextCursorPath == "" && hasMoreField == "" {
		return
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	var nextCursor string
	if nextCursorPath != "" {
		if tokenRaw, ok := rawAtPath(obj, nextCursorPath); ok {
			nextCursor = paginationCursorToken(tokenRaw)
		}
	}
	var hasMore bool
	if hasMoreField != "" {
		if moreRaw, ok := rawAtPath(obj, hasMoreField); ok {
			_ = json.Unmarshal(moreRaw, &hasMore)
		}
	}
	if nextCursor == "" && !hasMore {
		return
	}
	// --all advances when a next-cursor is configured, or when the endpoint
	// uses client-side numeric page/offset advancement. Opaque cursor APIs
	// still need a returned cursor to advance safely.
	if nextCursor != "" || ((paginationType == "page" || paginationType == "offset") && hasMore) {
		if humanFriendly {
			fmt.Fprintf(os.Stderr, "warning: results truncated; more pages available. Re-run with --all to fetch every page.\n")
		} else {
			fmt.Fprintf(os.Stderr, `{"event":"truncated","hint":"pass --all to fetch every page"}`+"\n")
		}
		return
	}

	if humanFriendly {
		fmt.Fprintf(os.Stderr, "warning: results truncated; more pages available.\n")
	} else {
		fmt.Fprintf(os.Stderr, `{"event":"truncated"}`+"\n")
	}
}

func emitMissingPaginationSignalWarning() {
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "warning: --all requested, but this endpoint does not declare a next cursor or has-more field; returning page 1 only.\n")
	} else {
		fmt.Fprintf(os.Stderr, `{"event":"truncated","reason":"pagination_signal_missing","message":"--all requested but this endpoint does not declare a next cursor or has-more field; returning page 1 only"}`+"\n")
	}
}

func emitPaginatedGetMaxPagesWarning() {
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "warning: --all reached the %d-page safety limit; returning fetched pages only.\n", paginatedGetMaxPages)
	} else {
		fmt.Fprintf(os.Stderr, `{"event":"truncated","reason":"max_pages_cap_hit","message":"--all reached the %d-page safety limit; returning fetched pages only"}`+"\n", paginatedGetMaxPages)
	}
}

func emitMissingPaginationCursorWarning(nextCursorPath string) {
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "warning: --all requested, but the response indicated more pages without a usable next cursor; returning fetched pages only.\n")
	} else if nextCursorPath != "" {
		fmt.Fprintf(os.Stderr, `{"event":"truncated","reason":"pagination_cursor_missing","next_cursor_path":%q,"message":"--all requested but the response indicated more pages without a usable next cursor; returning fetched pages only"}`+"\n", nextCursorPath)
	} else {
		fmt.Fprintf(os.Stderr, `{"event":"truncated","reason":"pagination_cursor_missing","message":"--all requested but the response indicated more pages without a usable next cursor; returning fetched pages only"}`+"\n")
	}
}

func paginationCursorToken(raw json.RawMessage) string {
	var token string
	if json.Unmarshal(raw, &token) == nil && token != "" {
		return token
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		if n, err := number.Int64(); err == nil && n > 0 {
			return number.String()
		}
	}
	return ""
}

func extractPaginatedItems(obj map[string]json.RawMessage) ([]json.RawMessage, bool) {
	for _, field := range []string{"data", "items", "results", "messages", "members", "values"} {
		if arr, ok := obj[field]; ok {
			var nested []json.RawMessage
			if json.Unmarshal(arr, &nested) == nil {
				return nested, true
			}
		}
	}

	var onlyArray []json.RawMessage
	arrayCount := 0
	for _, raw := range obj {
		var candidate []json.RawMessage
		if json.Unmarshal(raw, &candidate) == nil {
			onlyArray = candidate
			arrayCount++
		}
	}
	if arrayCount == 1 {
		return onlyArray, true
	}
	return nil, false
}

func rawAtPath(obj map[string]json.RawMessage, path string) (json.RawMessage, bool) {
	if raw, ok := obj[path]; ok {
		return raw, true
	}

	current := obj
	parts := strings.Split(path, ".")
	for i, part := range parts {
		raw, ok := current[part]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return raw, true
		}
		if err := json.Unmarshal(raw, &current); err != nil {
			return nil, false
		}
	}
	return nil, false
}
