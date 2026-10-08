package mcp

// mcpDefaultPageSizes caps first-page size for read-only list tools whose
// endpoint takes a page-size input. Dropbox continuation cursors carry the
// page size of the call that issued them, so a bounded first page keeps
// every later page under the MCP result budget and the upstream cursor
// resumes with nothing omitted. Sizes assume worst-typical entry JSON
// (long paths, sharing info) and leave room below bound.MaxBytes.
// Only endpoints with an exposed continuation tool belong here; a smaller
// page on an endpoint agents cannot continue would hide the remaining rows.
var mcpDefaultPageSizes = map[string]struct {
	arg  string
	size int
}{
	"/files/list_folder":                   {"limit", 50},
	"/files/list_folder/get_latest_cursor": {"limit", 50},
	"/files/search_v2":                     {"options-max-results", 25},
	"/sharing/list_folders":                {"limit", 25},
	"/file_requests/list_v2":               {"limit", 50},
}

// withMCPDefaultPageSize fills in a page size the caller left unset. An
// explicit caller value always wins.
func withMCPDefaultPageSize(pathTemplate string, args map[string]any) map[string]any {
	page, ok := mcpDefaultPageSizes[pathTemplate]
	if !ok {
		return args
	}
	if args == nil {
		args = map[string]any{}
	}
	if _, set := args[page.arg]; !set {
		args[page.arg] = page.size
	}
	return args
}
