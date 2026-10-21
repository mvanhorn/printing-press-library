// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// determinePaginationDefaults applies Pinterest's bookmark cursor on top of the
// generated table. Every v5 list endpoint pages with ?bookmark= and returns
// {"items":[...],"bookmark":"..."}; the generator does not recognize that name
// and emits an offset paginator with no cursor, which silently stops sync after
// one page.
func determinePaginationDefaults(resource string) paginationDefaults {
	d := determineGeneratedPaginationDefaults(resource)
	if d.cursorParam == "" && d.limitParam == "page_size" {
		d.cursorParam = "bookmark"
		d.cursorType = "cursor"
		d.nextCursorPath = "bookmark"
		d.initialCursor = ""
	}
	return d
}

// pinterestBookmarkCursor upgrades the generated offset/no-cursor paginator to
// Pinterest's bookmark cursor for list endpoints, so --all follows ?bookmark=.
func pinterestBookmarkCursor(cursorParam, paginationType, limitParam, nextCursorPath string) (string, string, string) {
	if cursorParam != "" || limitParam != "page_size" {
		return cursorParam, paginationType, nextCursorPath
	}
	if nextCursorPath == "" {
		nextCursorPath = "bookmark"
	}
	return "bookmark", "cursor", nextCursorPath
}
