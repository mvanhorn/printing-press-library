package client

import "strings"

// pathPrefix returns the version prefix that should be inserted between the
// base URL and the operation path for Roam endpoints. The merged OpenAPI spec
// declares per-operation `servers` overrides because Roam's surface is split
// across /v0 (HQ stable), /v1 (HQ alpha + On-Air), /scim/v2 (SCIM), and bare
// (e.g. compliance export). The generated do() uses a single BaseURL, so we
// reproduce the per-operation server choice here as a literal mapping.
//
// Returns "" when the path already begins with /v0, /v1, or /scim — the caller
// should not double-prefix in that case.
func pathPrefix(path string) string {
	switch {
	case strings.HasPrefix(path, "/v0/"), strings.HasPrefix(path, "/v1/"), strings.HasPrefix(path, "/scim/"):
		return ""
	}
	// SCIM (case-sensitive — RFC 7644 paths)
	switch path {
	case "/Users", "/Groups", "/ServiceProviderConfig", "/ResourceTypes", "/Schemas":
		return "/scim/v2"
	}
	if strings.HasPrefix(path, "/Users/") || strings.HasPrefix(path, "/Groups/") || strings.HasPrefix(path, "/Schemas/") {
		return "/scim/v2"
	}
	// v1 (alpha chat, groups admin, on-air, recording, magicast, messageevent, test)
	v1 := []string{
		"/chat.sendMessage", "/groups.list", "/recording.list", "/messageevent.export",
		"/magicast.list", "/magicast.info", "/test",
		"/onair.event.info", "/onair.event.list", "/onair.event.create",
		"/onair.event.update", "/onair.event.cancel",
		"/onair.guest.info", "/onair.guest.list", "/onair.guest.add",
		"/onair.guest.update", "/onair.guest.remove",
		"/onair.attendance.list",
	}
	for _, p := range v1 {
		if path == p {
			return "/v1"
		}
	}
	// v0 (HQ stable: chat CRUD, item upload, addr, group admin, lobby, meeting,
	// meeting links, reactions, transcript, user, audit log, app, token,
	// webhook subscribe/unsubscribe).
	if strings.HasPrefix(path, "/chat.") ||
		strings.HasPrefix(path, "/group.") ||
		strings.HasPrefix(path, "/lobby") ||
		strings.HasPrefix(path, "/meeting") ||
		strings.HasPrefix(path, "/meetinglink") ||
		strings.HasPrefix(path, "/reaction.") ||
		strings.HasPrefix(path, "/transcript.") ||
		strings.HasPrefix(path, "/user") ||
		strings.HasPrefix(path, "/userauditlog") ||
		strings.HasPrefix(path, "/app.") ||
		strings.HasPrefix(path, "/token.") ||
		strings.HasPrefix(path, "/webhook.") ||
		path == "/item.upload" || path == "/addr.info" {
		return "/v0"
	}
	return ""
}
