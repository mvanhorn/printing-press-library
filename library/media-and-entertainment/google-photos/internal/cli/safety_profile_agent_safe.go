// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

//go:build safety_agent_safe

package cli

const bakedSafetyProfileName = "agent-safe"

var bakedAllowCommands = []string{
	"agent-context",
	"albums.get",
	"albums.list",
	"analytics",
	"context",
	"auth.list",
	"auth.status",
	"doctor",
	"export",
	"media-items.batch-get",
	"media-items.get",
	"media-items.list",
	"media-items.search",
	"picker.get-session",
	"picker.list-media-items",
	"picker.wait",
	"profile.list",
	"profile.show",
	"profile.use",
	"schema",
	"search",
	"sql",
	"sync",
	"which",
	"workflow.archive",
	"workflow.status",
}

var bakedDenyCommands = []string{
	"albums.add-enrichment",
	"albums.batch-add-media-items",
	"albums.batch-remove-media-items",
	"albums.create",
	"albums.patch",
	"auth.login",
	"auth.logout",
	"auth.remove",
	"auth.use",
	"import",
	"media-items.batch-create",
	"media-items.patch",
	"picker.create-session",
	"picker.delete-session",
	"upload",
}
