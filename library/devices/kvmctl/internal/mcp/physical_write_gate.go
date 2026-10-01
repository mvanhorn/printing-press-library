package mcp

import "strings"

// Raw device writes cannot consume the target/plan-bound workflow token.
// Keep these routes closed until they share that authorization protocol.
func physicalWriteBlocked(method, path string) bool {
	return method != "GET" && method != "HEAD" && (strings.HasPrefix(path, "/api/hid/") || path == "/api/streamer/set_params" || path == "/api/system/otg_functions")
}

const physicalWriteGuidance = "direct physical MCP writes are disabled because they lack target- and operation-bound authorization. Use an authorized sequence/workflow for supported keyboard and mouse actions. Raw HID reset, streamer parameters, and OTG configuration have no authorized MCP workflow support yet."
