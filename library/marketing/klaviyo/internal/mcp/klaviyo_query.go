// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"net/url"
	"strings"
)

func klaviyoMCPQueryArgument(name string) bool {
	return name == "page_cursor" || strings.HasPrefix(name, "page[") || strings.HasPrefix(name, "fields[") || name == "include" || name == "filter" || name == "sort"
}

func splitKlaviyoMCPQuery(path string, params map[string]any) string {
	query := url.Values{}
	for name, value := range params {
		if klaviyoMCPQueryArgument(name) {
			query.Set(name, formatMCPParamValue(value))
			delete(params, name)
		}
	}
	if len(query) == 0 {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + query.Encode()
}
