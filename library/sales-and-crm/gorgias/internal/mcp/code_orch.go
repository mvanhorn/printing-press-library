// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

// Package mcp — code-orchestration thin surface.
//
// Two tools cover the entire API: <api>_search to discover endpoints, and
// <api>_execute to invoke one. This collapses a large API (50+ endpoints)
// to ~1K tokens of tool definitions while preserving full coverage — the
// agent writes the composition logic in its own sandbox.
//
// Pattern source: Anthropic 2026-04-22 "Building agents that reach
// production systems with MCP" — Cloudflare's MCP server covers ~2,500
// endpoints in roughly 1K tokens via the same search+execute shape.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterCodeOrchestrationTools registers the two agent-facing tools that
// cover the whole API surface. Called from RegisterTools in place of the
// per-endpoint registrations when MCP.Orchestration is "code". Returns the
// number of tools registered so the caller can publish an accurate
// tool_count to introspecting agents.
func RegisterCodeOrchestrationTools(s *server.MCPServer) int {
	s.AddTool(
		mcplib.NewTool("gorgias_search",
			mcplib.WithDescription("Find the right Gorgias endpoint by natural-language query. Returns a ranked list of endpoint_ids (e.g. tickets.list, customers.merge) with HTTP method, path, and a one-line summary. ALWAYS call this before gorgias_execute — the endpoint_id is rarely guessable from English. Use specific phrasing (\"list cancellation tickets\", not \"find returns stuff\"). Returns at most 10 matches; narrow the query if the first call is ambiguous."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Natural-language description of what you want to do.")),
			mcplib.WithNumber("limit", mcplib.Description("Max endpoints to return (default 10).")),
		),
		handleCodeOrchSearch,
	)

	s.AddTool(
		mcplib.NewTool("gorgias_execute",
			mcplib.WithDescription("Call any Gorgias endpoint by endpoint_id with a flat params map. Required: endpoint_id from gorgias_search. params carries path placeholders, query, and body fields all together (the gateway routes each by the endpoint's spec). Common foot-guns: (1) customers.list rejects language or timezone together with cursor/limit/order_by — pick one approach per call. (2) events.list requires both object_type AND object_id. (3) Gorgias /search indexes customers/agents/tags/teams/integrations — NOT tickets or messages; for ticket text search, use the `search` tool against the local mirror after a `sync`. (4) limit caps at 100; paginate via cursor for more rows."),
			mcplib.WithString("endpoint_id", mcplib.Required(), mcplib.Description("Endpoint identifier returned by gorgias_search (e.g., \"tickets.list\", \"customers.merge\").")),
			mcplib.WithObject("params", mcplib.Description("Path placeholders + query/body fields, all as a flat map. The gateway resolves each by the endpoint's spec.")),
		),
		handleCodeOrchExecute,
	)
	return 2
}

// codeOrchEndpoint captures the small slice of endpoint metadata the
// search+execute pair needs at runtime. `keywords` is a precomputed
// lowercase stream of description + path tokens used for naive ranking;
// anything more sophisticated belongs on the agent side.
type codeOrchEndpoint struct {
	ID         string
	Method     string
	Path       string
	Tier       string
	Summary    string
	Positional []string
	keywords   []string
}

// codeOrchEndpoints is the generator-populated registry covering every
// endpoint declared in the spec. Kept flat on purpose — the agent queries
// via <api>_search, so hierarchy shows up as dotted IDs, not nested maps.
var codeOrchEndpoints = []codeOrchEndpoint{
	{
		ID:         "account.get",
		Method:     "GET",
		Path:       "/account",
		Summary:    "Retrieve the current Gorgias account's metadata: subdomain, plan, billing state, and account-level flags. Use this...",
		Positional: []string{},
		keywords:   codeOrchKeywords("account", "get", "Retrieve the current Gorgias account's metadata: subdomain, plan, billing state, and account-level flags. Use this...", "/account"),
	},
	{
		ID:         "account.settings-create",
		Method:     "POST",
		Path:       "/account/settings",
		Summary:    "Create a new account-level settings record for the current Gorgias tenant. Use when bootstrapping a fresh tenant or...",
		Positional: []string{},
		keywords:   codeOrchKeywords("account", "settings-create", "Create a new account-level settings record for the current Gorgias tenant. Use when bootstrapping a fresh tenant or...", "/account/settings"),
	},
	{
		ID:         "account.settings-list",
		Method:     "GET",
		Path:       "/account/settings",
		Summary:    "List the global settings on the current Gorgias account (business hours, language, default channels, notification...",
		Positional: []string{},
		keywords:   codeOrchKeywords("account", "settings-list", "List the global settings on the current Gorgias account (business hours, language, default channels, notification...", "/account/settings"),
	},
	{
		ID:         "account.settings-update",
		Method:     "PUT",
		Path:       "/account/settings/{id}",
		Summary:    "Update an account settings record by `id`. Use this to flip a tenant-wide flag, change business hours, or adjust a...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("account", "settings-update", "Update an account settings record by `id`. Use this to flip a tenant-wide flag, change business hours, or adjust a...", "/account/settings/{id}"),
	},
	{
		ID:         "custom-fields.create",
		Method:     "POST",
		Path:       "/custom-fields",
		Summary:    "Define a new custom field on tickets or customers (the only supported `object_type` values). Required body:...",
		Positional: []string{},
		keywords:   codeOrchKeywords("custom-fields", "create", "Define a new custom field on tickets or customers (the only supported `object_type` values). Required body:...", "/custom-fields"),
	},
	{
		ID:         "custom-fields.get",
		Method:     "GET",
		Path:       "/custom-fields/{id}",
		Summary:    "Fetch a single custom field definition by `id`, returning its data type, label, target object, and option list. Use...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("custom-fields", "get", "Fetch a single custom field definition by `id`, returning its data type, label, target object, and option list. Use...", "/custom-fields/{id}"),
	},
	{
		ID:         "custom-fields.list",
		Method:     "GET",
		Path:       "/custom-fields",
		Summary:    "List custom field definitions for a single `object_type` (`Ticket` or `Customer` — REQUIRED query param)....",
		Positional: []string{},
		keywords:   codeOrchKeywords("custom-fields", "list", "List custom field definitions for a single `object_type` (`Ticket` or `Customer` — REQUIRED query param)....", "/custom-fields"),
	},
	{
		ID:         "custom-fields.update",
		Method:     "PUT",
		Path:       "/custom-fields/{id}",
		Summary:    "Update one custom field definition by `id` — relabel it, change its options, or toggle visibility. Note: this...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("custom-fields", "update", "Update one custom field definition by `id` — relabel it, change its options, or toggle visibility. Note: this...", "/custom-fields/{id}"),
	},
	{
		ID:         "custom-fields.update-all",
		Method:     "PUT",
		Path:       "/custom-fields",
		Summary:    "Bulk-update multiple custom field definitions in one call (no path id). Useful when reordering picklist options or...",
		Positional: []string{},
		keywords:   codeOrchKeywords("custom-fields", "update-all", "Bulk-update multiple custom field definitions in one call (no path id). Useful when reordering picklist options or...", "/custom-fields"),
	},
	{
		ID:         "customers.create",
		Method:     "POST",
		Path:       "/customers",
		Summary:    "Create a new customer record. Pass `name`, `email`, optional `channels` (email/phone/social handles), and `data` for...",
		Positional: []string{},
		keywords:   codeOrchKeywords("customers", "create", "Create a new customer record. Pass `name`, `email`, optional `channels` (email/phone/social handles), and `data` for...", "/customers"),
	},
	{
		ID:         "customers.custom-fields-list",
		Method:     "GET",
		Path:       "/customers/{id}/custom-fields",
		Summary:    "List every custom field value attached to a single customer (`id`). Use to read CRM-style attributes (lifetime...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("customers", "custom-fields-list", "List every custom field value attached to a single customer (`id`). Use to read CRM-style attributes (lifetime...", "/customers/{id}/custom-fields"),
	},
	{
		ID:         "customers.custom-fields-set",
		Method:     "PUT",
		Path:       "/customers/{customer_id}/custom-fields/{id}",
		Summary:    "Set a single custom field value on a customer: first `{id}` is the customer, second `{id}` is the custom field. Use...",
		Positional: []string{"customer_id"},
		keywords:   codeOrchKeywords("customers", "custom-fields-set", "Set a single custom field value on a customer: first `{id}` is the customer, second `{id}` is the custom field. Use...", "/customers/{customer_id}/custom-fields/{id}"),
	},
	{
		ID:         "customers.custom-fields-set-all",
		Method:     "PUT",
		Path:       "/customers/{id}/custom-fields",
		Summary:    "Bulk-set custom field values on a single customer (`id`) — pass an array of field/value pairs. Preferred over the...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("customers", "custom-fields-set-all", "Bulk-set custom field values on a single customer (`id`) — pass an array of field/value pairs. Preferred over the...", "/customers/{id}/custom-fields"),
	},
	{
		ID:         "customers.custom-fields-unset",
		Method:     "DELETE",
		Path:       "/customers/{customer_id}/custom-fields/{id}",
		Summary:    "Clear a custom field value on a customer: first `{id}` is the customer ID, second `{id}` is the custom field ID. Use...",
		Positional: []string{"customer_id", "id"},
		keywords:   codeOrchKeywords("customers", "custom-fields-unset", "Clear a custom field value on a customer: first `{id}` is the customer ID, second `{id}` is the custom field ID. Use...", "/customers/{customer_id}/custom-fields/{id}"),
	},
	{
		ID:         "customers.data-update",
		Method:     "PUT",
		Path:       "/customers/{id}/data",
		Summary:    "Set a customer's `data` blob (`id` in path). Body: `data` (required) plus optional `version` for last-write-wins...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("customers", "data-update", "Set a customer's `data` blob (`id` in path). Body: `data` (required) plus optional `version` for last-write-wins...", "/customers/{id}/data"),
	},
	{
		ID:         "customers.delete",
		Method:     "DELETE",
		Path:       "/customers/{id}",
		Summary:    "Delete one customer by `id`. Hard-deletes the record and may cascade to associated tickets/messages depending on...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("customers", "delete", "Delete one customer by `id`. Hard-deletes the record and may cascade to associated tickets/messages depending on...", "/customers/{id}"),
	},
	{
		ID:         "customers.delete-all",
		Method:     "DELETE",
		Path:       "/customers",
		Summary:    "Bulk-delete customers. Required body: `ids` (array of customer IDs to delete). Does NOT accept query-style filters...",
		Positional: []string{},
		keywords:   codeOrchKeywords("customers", "delete-all", "Bulk-delete customers. Required body: `ids` (array of customer IDs to delete). Does NOT accept query-style filters...", "/customers"),
	},
	{
		ID:         "customers.get",
		Method:     "GET",
		Path:       "/customers/{id}",
		Summary:    "Fetch a single customer by `id`, including their channels (email, phone, social handles), `data` blob, and...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("customers", "get", "Fetch a single customer by `id`, including their channels (email, phone, social handles), `data` blob, and...", "/customers/{id}"),
	},
	{
		ID:         "customers.list",
		Method:     "GET",
		Path:       "/customers",
		Summary:    "List customers with pagination and optional filter params (`email`, `external_id`, `name`, `language`,...",
		Positional: []string{},
		keywords:   codeOrchKeywords("customers", "list", "List customers with pagination and optional filter params (`email`, `external_id`, `name`, `language`,...", "/customers"),
	},
	{
		ID:         "customers.merge",
		Method:     "PUT",
		Path:       "/customers/merge",
		Summary:    "Merge one customer into another. Required query params: `source_id` (the duplicate, will be merged in and deleted)...",
		Positional: []string{},
		keywords:   codeOrchKeywords("customers", "merge", "Merge one customer into another. Required query params: `source_id` (the duplicate, will be merged in and deleted)...", "/customers/merge"),
	},
	{
		ID:         "customers.update",
		Method:     "PUT",
		Path:       "/customers/{id}",
		Summary:    "Update a customer (`id`) — change name, add/remove channels, edit external IDs, or overwrite top-level fields. Use...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("customers", "update", "Update a customer (`id`) — change name, add/remove channels, edit external IDs, or overwrite top-level fields. Use...", "/customers/{id}"),
	},
	{
		ID:         "events.get",
		Method:     "GET",
		Path:       "/events/{id}",
		Summary:    "Retrieve a single audit event by `id` — captures who/what/when on ticket, customer, or settings mutations. Use to...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("events", "get", "Retrieve a single audit event by `id` — captures who/what/when on ticket, customer, or settings mutations. Use to...", "/events/{id}"),
	},
	{
		ID:         "events.list",
		Method:     "GET",
		Path:       "/events",
		Summary:    "List audit events. Documented filters: `object_type` (e.g. Ticket/Customer/User), `object_id`, `user_ids` (actor),...",
		Positional: []string{},
		keywords:   codeOrchKeywords("events", "list", "List audit events. Documented filters: `object_type` (e.g. Ticket/Customer/User), `object_id`, `user_ids` (actor),...", "/events"),
	},
	{
		ID:         "gorgias-jobs.create",
		Method:     "POST",
		Path:       "/jobs",
		Summary:    "Kick off an asynchronous Gorgias job. Required body: `type` (enum: applyMacro, deleteTicket, exportTicket,...",
		Positional: []string{},
		keywords:   codeOrchKeywords("gorgias-jobs", "create", "Kick off an asynchronous Gorgias job. Required body: `type` (enum: applyMacro, deleteTicket, exportTicket,...", "/jobs"),
	},
	{
		ID:         "gorgias-jobs.delete",
		Method:     "DELETE",
		Path:       "/jobs/{id}",
		Summary:    "Delete a job record by `id`. Useful for cleaning up completed or failed entries from listings; does not cancel an...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("gorgias-jobs", "delete", "Delete a job record by `id`. Useful for cleaning up completed or failed entries from listings; does not cancel an...", "/jobs/{id}"),
	},
	{
		ID:         "gorgias-jobs.get",
		Method:     "GET",
		Path:       "/jobs/{id}",
		Summary:    "Fetch a single async job (`id`) with its status, progress, params, and result/error fields. The polling endpoint...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("gorgias-jobs", "get", "Fetch a single async job (`id`) with its status, progress, params, and result/error fields. The polling endpoint...", "/jobs/{id}"),
	},
	{
		ID:         "gorgias-jobs.list",
		Method:     "GET",
		Path:       "/jobs",
		Summary:    "List async jobs with filters by type, status, and datetime. Use to find a recent export job by an agent or to...",
		Positional: []string{},
		keywords:   codeOrchKeywords("gorgias-jobs", "list", "List async jobs with filters by type, status, and datetime. Use to find a recent export job by an agent or to...", "/jobs"),
	},
	{
		ID:         "gorgias-jobs.update",
		Method:     "PUT",
		Path:       "/jobs/{id}",
		Summary:    "Update an async job (`id`) — typically to cancel it or adjust metadata. Reach for this only when you need to abort...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("gorgias-jobs", "update", "Update an async job (`id`) — typically to cancel it or adjust metadata. Reach for this only when you need to abort...", "/jobs/{id}"),
	},
	{
		ID:         "integrations.create",
		Method:     "POST",
		Path:       "/integrations",
		Summary:    "Install a new third-party integration on the Gorgias account (Shopify, Instagram, SMS provider, etc.). Pass `type`...",
		Positional: []string{},
		keywords:   codeOrchKeywords("integrations", "create", "Install a new third-party integration on the Gorgias account (Shopify, Instagram, SMS provider, etc.). Pass `type`...", "/integrations"),
	},
	{
		ID:         "integrations.delete",
		Method:     "DELETE",
		Path:       "/integrations/{id}",
		Summary:    "Uninstall an integration by `id`. Destructive — disconnects the channel and may stop syncing orders/messages from...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("integrations", "delete", "Uninstall an integration by `id`. Destructive — disconnects the channel and may stop syncing orders/messages from...", "/integrations/{id}"),
	},
	{
		ID:         "integrations.get",
		Method:     "GET",
		Path:       "/integrations/{id}",
		Summary:    "Fetch a single integration (`id`) including its type, status, last-sync time, and provider-specific config. Use to...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("integrations", "get", "Fetch a single integration (`id`) including its type, status, last-sync time, and provider-specific config. Use to...", "/integrations/{id}"),
	},
	{
		ID:         "integrations.list",
		Method:     "GET",
		Path:       "/integrations",
		Summary:    "List all installed integrations on the account — Shopify, Magento, Facebook, voice, etc. Use to discover what...",
		Positional: []string{},
		keywords:   codeOrchKeywords("integrations", "list", "List all installed integrations on the account — Shopify, Magento, Facebook, voice, etc. Use to discover what...", "/integrations"),
	},
	{
		ID:         "integrations.update",
		Method:     "PUT",
		Path:       "/integrations/{id}",
		Summary:    "Update an integration's config (`id`) — refresh credentials, toggle sync features, or rename. Reach for this when...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("integrations", "update", "Update an integration's config (`id`) — refresh credentials, toggle sync features, or rename. Reach for this when...", "/integrations/{id}"),
	},
	{
		ID:         "macros.archive",
		Method:     "PUT",
		Path:       "/macros/archive",
		Summary:    "Archive one or more macros (soft delete) — pass macro IDs in the body. Use this rather than `macros_delete` to...",
		Positional: []string{},
		keywords:   codeOrchKeywords("macros", "archive", "Archive one or more macros (soft delete) — pass macro IDs in the body. Use this rather than `macros_delete` to...", "/macros/archive"),
	},
	{
		ID:         "macros.create",
		Method:     "POST",
		Path:       "/macros",
		Summary:    "Create a new macro: a reusable reply/action template. Required body: `name`. Optional: `intent`, `language`,...",
		Positional: []string{},
		keywords:   codeOrchKeywords("macros", "create", "Create a new macro: a reusable reply/action template. Required body: `name`. Optional: `intent`, `language`,...", "/macros"),
	},
	{
		ID:         "macros.delete",
		Method:     "DELETE",
		Path:       "/macros/{id}",
		Summary:    "Delete a macro by `id`. Hard-deletes it from the macro library. Prefer `macros_archive` for soft removal so...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("macros", "delete", "Delete a macro by `id`. Hard-deletes it from the macro library. Prefer `macros_archive` for soft removal so...", "/macros/{id}"),
	},
	{
		ID:         "macros.get",
		Method:     "GET",
		Path:       "/macros/{id}",
		Summary:    "Fetch a single macro by `id`, returning its body, actions, and variable definitions. Use before applying a macro so...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("macros", "get", "Fetch a single macro by `id`, returning its body, actions, and variable definitions. Use before applying a macro so...", "/macros/{id}"),
	},
	{
		ID:         "macros.list",
		Method:     "GET",
		Path:       "/macros",
		Summary:    "List all macros, with optional filters (archived, name). The agent's discovery endpoint for available canned replies...",
		Positional: []string{},
		keywords:   codeOrchKeywords("macros", "list", "List all macros, with optional filters (archived, name). The agent's discovery endpoint for available canned replies...", "/macros"),
	},
	{
		ID:         "macros.unarchive",
		Method:     "PUT",
		Path:       "/macros/unarchive",
		Summary:    "Unarchive one or more macros, restoring them to the active library. Pass macro IDs in the body. The companion to...",
		Positional: []string{},
		keywords:   codeOrchKeywords("macros", "unarchive", "Unarchive one or more macros, restoring them to the active library. Pass macro IDs in the body. The companion to...", "/macros/unarchive"),
	},
	{
		ID:         "macros.update",
		Method:     "PUT",
		Path:       "/macros/{id}",
		Summary:    "Update a macro (`id`) — edit its body, variables, tags-to-add, or action list. Use when an agent is refining a...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("macros", "update", "Update a macro (`id`) — edit its body, variables, tags-to-add, or action list. Use when an agent is refining a...", "/macros/{id}"),
	},
	{
		ID:         "messages.list",
		Method:     "GET",
		Path:       "/messages",
		Summary:    "List messages account-wide, paginated. Supported filters are `ticket_id` only (plus `cursor`, `limit`, `order_by`);...",
		Positional: []string{},
		keywords:   codeOrchKeywords("messages", "list", "List messages account-wide, paginated. Supported filters are `ticket_id` only (plus `cursor`, `limit`, `order_by`);...", "/messages"),
	},
	{
		ID:         "phone.call-events-get",
		Method:     "GET",
		Path:       "/phone/voice-call-events/{id}",
		Summary:    "Fetch a single voice-call event by `id` — events capture call lifecycle (ringing, answered, hung-up, transferred)....",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("phone", "call-events-get", "Fetch a single voice-call event by `id` — events capture call lifecycle (ringing, answered, hung-up, transferred)....", "/phone/voice-call-events/{id}"),
	},
	{
		ID:         "phone.call-events-list",
		Method:     "GET",
		Path:       "/phone/voice-call-events",
		Summary:    "List voice-call lifecycle events. Documented filter is `call_id` only (plus `cursor`, `limit`). Use to inspect the...",
		Positional: []string{},
		keywords:   codeOrchKeywords("phone", "call-events-list", "List voice-call lifecycle events. Documented filter is `call_id` only (plus `cursor`, `limit`). Use to inspect the...", "/phone/voice-call-events"),
	},
	{
		ID:         "phone.call-recordings-delete",
		Method:     "DELETE",
		Path:       "/phone/voice-call-recordings/{id}",
		Summary:    "Delete a stored voice-call recording by `id`. Use to honor a customer privacy/erasure request or to scrub a test...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("phone", "call-recordings-delete", "Delete a stored voice-call recording by `id`. Use to honor a customer privacy/erasure request or to scrub a test...", "/phone/voice-call-recordings/{id}"),
	},
	{
		ID:         "phone.call-recordings-get",
		Method:     "GET",
		Path:       "/phone/voice-call-recordings/{id}",
		Summary:    "Fetch metadata for a single call recording (`id`) — duration, URL, related call/ticket. Pair with...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("phone", "call-recordings-get", "Fetch metadata for a single call recording (`id`) — duration, URL, related call/ticket. Pair with...", "/phone/voice-call-recordings/{id}"),
	},
	{
		ID:         "phone.call-recordings-list",
		Method:     "GET",
		Path:       "/phone/voice-call-recordings",
		Summary:    "List voice-call recordings. Documented filter is `call_id` only (plus `cursor`, `limit`). Use to find the...",
		Positional: []string{},
		keywords:   codeOrchKeywords("phone", "call-recordings-list", "List voice-call recordings. Documented filter is `call_id` only (plus `cursor`, `limit`). Use to find the...", "/phone/voice-call-recordings"),
	},
	{
		ID:         "phone.calls-get",
		Method:     "GET",
		Path:       "/phone/voice-calls/{id}",
		Summary:    "Fetch a single voice call (`id`) with direction, status, duration, participants, and the linked ticket. Use when an...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("phone", "calls-get", "Fetch a single voice call (`id`) with direction, status, duration, participants, and the linked ticket. Use when an...", "/phone/voice-calls/{id}"),
	},
	{
		ID:         "phone.calls-list",
		Method:     "GET",
		Path:       "/phone/voice-calls",
		Summary:    "List voice calls, paginated. Documented filter is `ticket_id` only (plus `cursor`, `limit`, `order_by`). Use to...",
		Positional: []string{},
		keywords:   codeOrchKeywords("phone", "calls-list", "List voice calls, paginated. Documented filter is `ticket_id` only (plus `cursor`, `limit`, `order_by`). Use to...", "/phone/voice-calls"),
	},
	{
		ID:         "pickups.delete",
		Method:     "DELETE",
		Path:       "/pickups/{id}",
		Summary:    "Delete a pickup record by `id`. Counterpart to `pickups_create_pickups`; use to cancel or remove a stale logistics...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("pickups", "delete", "Delete a pickup record by `id`. Counterpart to `pickups_create_pickups`; use to cancel or remove a stale logistics...", "/pickups/{id}"),
	},
	{
		ID:         "reporting.stats",
		Method:     "POST",
		Path:       "/reporting/stats",
		Summary:    "Run a Gorgias analytics query: POST a JSON body with `metric`, `dimensions`, `filters`, and a `period`. The single...",
		Positional: []string{},
		keywords:   codeOrchKeywords("reporting", "stats", "Run a Gorgias analytics query: POST a JSON body with `metric`, `dimensions`, `filters`, and a `period`. The single...", "/reporting/stats"),
	},
	{
		ID:         "rules.create",
		Method:     "POST",
		Path:       "/rules",
		Summary:    "Create a new automation rule. Required body: `name` and `code` (the rule logic written as JavaScript). Optional:...",
		Positional: []string{},
		keywords:   codeOrchKeywords("rules", "create", "Create a new automation rule. Required body: `name` and `code` (the rule logic written as JavaScript). Optional:...", "/rules"),
	},
	{
		ID:         "rules.delete",
		Method:     "DELETE",
		Path:       "/rules/{id}",
		Summary:    "Delete an automation rule by `id`. Stops the rule from firing on future tickets but does not undo past actions. Use...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("rules", "delete", "Delete an automation rule by `id`. Stops the rule from firing on future tickets but does not undo past actions. Use...", "/rules/{id}"),
	},
	{
		ID:         "rules.get",
		Method:     "GET",
		Path:       "/rules/{id}",
		Summary:    "Fetch a single automation rule (`id`) with its full conditions/actions tree and enabled state. Use to inspect why a...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("rules", "get", "Fetch a single automation rule (`id`) with its full conditions/actions tree and enabled state. Use to inspect why a...", "/rules/{id}"),
	},
	{
		ID:         "rules.list",
		Method:     "GET",
		Path:       "/rules",
		Summary:    "List all automation rules with their order, enabled state, and summary. The agent's map of what automations are...",
		Positional: []string{},
		keywords:   codeOrchKeywords("rules", "list", "List all automation rules with their order, enabled state, and summary. The agent's map of what automations are...", "/rules"),
	},
	{
		ID:         "rules.set-priorities",
		Method:     "POST",
		Path:       "/rules/priorities",
		Summary:    "Set the execution priorities of automation rules. Required body: `priorities` — an array of objects mapping rule...",
		Positional: []string{},
		keywords:   codeOrchKeywords("rules", "set-priorities", "Set the execution priorities of automation rules. Required body: `priorities` — an array of objects mapping rule...", "/rules/priorities"),
	},
	{
		ID:         "rules.update",
		Method:     "PUT",
		Path:       "/rules/{id}",
		Summary:    "Update an automation rule (`id`) — edit conditions, actions, or enabled flag. Use to tune an existing workflow...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("rules", "update", "Update an automation rule (`id`) — edit conditions, actions, or enabled flag. Use to tune an existing workflow...", "/rules/{id}"),
	},
	{
		ID:         "satisfaction-surveys.create",
		Method:     "POST",
		Path:       "/satisfaction-surveys",
		Summary:    "Create a satisfaction-survey instance attached to one ticket and customer. Required body: `customer_id`,...",
		Positional: []string{},
		keywords:   codeOrchKeywords("satisfaction-surveys", "create", "Create a satisfaction-survey instance attached to one ticket and customer. Required body: `customer_id`,...", "/satisfaction-surveys"),
	},
	{
		ID:         "satisfaction-surveys.get",
		Method:     "GET",
		Path:       "/satisfaction-surveys/{id}",
		Summary:    "Fetch a single satisfaction-survey instance by `id` — the linked ticket/customer, score, customer comment, and...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("satisfaction-surveys", "get", "Fetch a single satisfaction-survey instance by `id` — the linked ticket/customer, score, customer comment, and...", "/satisfaction-surveys/{id}"),
	},
	{
		ID:         "satisfaction-surveys.list",
		Method:     "GET",
		Path:       "/satisfaction-surveys",
		Summary:    "List satisfaction-survey instances (each one tied to a single ticket). Filter with `ticket_id` to fetch the survey...",
		Positional: []string{},
		keywords:   codeOrchKeywords("satisfaction-surveys", "list", "List satisfaction-survey instances (each one tied to a single ticket). Filter with `ticket_id` to fetch the survey...", "/satisfaction-surveys"),
	},
	{
		ID:         "satisfaction-surveys.update",
		Method:     "PUT",
		Path:       "/satisfaction-surveys/{id}",
		Summary:    "Update a satisfaction-survey instance (`id`) — typically to record/correct the customer's `score` (1–5),...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("satisfaction-surveys", "update", "Update a satisfaction-survey instance (`id`) — typically to record/correct the customer's `score` (1–5),...", "/satisfaction-surveys/{id}"),
	},
	{
		ID:         "tags.create",
		Method:     "POST",
		Path:       "/tags",
		Summary:    "Create a new tag in the account's tag library. Body: `name` (required, max 256 chars, case-sensitive), `description`...",
		Positional: []string{},
		keywords:   codeOrchKeywords("tags", "create", "Create a new tag in the account's tag library. Body: `name` (required, max 256 chars, case-sensitive), `description`...", "/tags"),
	},
	{
		ID:         "tags.delete",
		Method:     "DELETE",
		Path:       "/tags/{id}",
		Summary:    "Delete a tag by `id`. Removes it from the library and unassociates it from every ticket/customer that carries it....",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tags", "delete", "Delete a tag by `id`. Removes it from the library and unassociates it from every ticket/customer that carries it....", "/tags/{id}"),
	},
	{
		ID:         "tags.delete-all",
		Method:     "DELETE",
		Path:       "/tags",
		Summary:    "Bulk-delete tags. Required body: `ids` (array of tag IDs, min 1). Tags currently referenced by macros or rules...",
		Positional: []string{},
		keywords:   codeOrchKeywords("tags", "delete-all", "Bulk-delete tags. Required body: `ids` (array of tag IDs, min 1). Tags currently referenced by macros or rules...", "/tags"),
	},
	{
		ID:         "tags.get",
		Method:     "GET",
		Path:       "/tags/{id}",
		Summary:    "Fetch a single tag (`id`) with its name, decoration, and metadata. Use to verify a tag exists before applying it, or...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tags", "get", "Fetch a single tag (`id`) with its name, decoration, and metadata. Use to verify a tag exists before applying it, or...", "/tags/{id}"),
	},
	{
		ID:         "tags.list",
		Method:     "GET",
		Path:       "/tags",
		Summary:    "List all tags in the account, optionally filtered by name. The agent's lookup endpoint for finding the right...",
		Positional: []string{},
		keywords:   codeOrchKeywords("tags", "list", "List all tags in the account, optionally filtered by name. The agent's lookup endpoint for finding the right...", "/tags"),
	},
	{
		ID:         "tags.merge",
		Method:     "PUT",
		Path:       "/tags/{id}/merge",
		Summary:    "Merge other tags INTO this tag — path `{id}` is the destination (surviving) tag, and the body field...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tags", "merge", "Merge other tags INTO this tag — path `{id}` is the destination (surviving) tag, and the body field...", "/tags/{id}/merge"),
	},
	{
		ID:         "tags.update",
		Method:     "PUT",
		Path:       "/tags/{id}",
		Summary:    "Update a tag (`id`) — rename it or change its color/decoration. Affects every record currently carrying the tag,...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tags", "update", "Update a tag (`id`) — rename it or change its color/decoration. Affects every record currently carrying the tag,...", "/tags/{id}"),
	},
	{
		ID:         "teams.create",
		Method:     "POST",
		Path:       "/teams",
		Summary:    "Create a new team (group of agents) in the account. Pass `name` and optionally members. Use when organizing routing...",
		Positional: []string{},
		keywords:   codeOrchKeywords("teams", "create", "Create a new team (group of agents) in the account. Pass `name` and optionally members. Use when organizing routing...", "/teams"),
	},
	{
		ID:         "teams.delete",
		Method:     "DELETE",
		Path:       "/teams/{id}",
		Summary:    "Delete a team by `id`. Removes it from routing rules and views; members remain but lose the team grouping. Use when...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("teams", "delete", "Delete a team by `id`. Removes it from routing rules and views; members remain but lose the team grouping. Use when...", "/teams/{id}"),
	},
	{
		ID:         "teams.get",
		Method:     "GET",
		Path:       "/teams/{id}",
		Summary:    "Fetch a single team (`id`) with its members and metadata. Use when an agent needs to know who's on a team before...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("teams", "get", "Fetch a single team (`id`) with its members and metadata. Use when an agent needs to know who's on a team before...", "/teams/{id}"),
	},
	{
		ID:         "teams.list",
		Method:     "GET",
		Path:       "/teams",
		Summary:    "List all teams in the account. The agent's lookup for valid team IDs/names when assigning a ticket, routing via a...",
		Positional: []string{},
		keywords:   codeOrchKeywords("teams", "list", "List all teams in the account. The agent's lookup for valid team IDs/names when assigning a ticket, routing via a...", "/teams"),
	},
	{
		ID:         "teams.update",
		Method:     "PUT",
		Path:       "/teams/{id}",
		Summary:    "Update a team (`id`) — rename it or change its membership. Use to reorganize agents or correct a misconfigured team.",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("teams", "update", "Update a team (`id`) — rename it or change its membership. Use to reorganize agents or correct a misconfigured team.", "/teams/{id}"),
	},
	{
		ID:         "ticket-search.query",
		Method:     "POST",
		Path:       "/search",
		Summary:    "Full-text search across Gorgias tickets, customers, and messages. POST a JSON body with `query`, `resource_type`,...",
		Positional: []string{},
		keywords:   codeOrchKeywords("ticket-search", "query", "Full-text search across Gorgias tickets, customers, and messages. POST a JSON body with `query`, `resource_type`,...", "/search"),
	},
	{
		ID:         "tickets.create",
		Method:     "POST",
		Path:       "/tickets",
		Summary:    "Create a new ticket. Body specifies `channel`, `via`, `subject`, an initial `messages` array, the customer, and...",
		Positional: []string{},
		keywords:   codeOrchKeywords("tickets", "create", "Create a new ticket. Body specifies `channel`, `via`, `subject`, an initial `messages` array, the customer, and...", "/tickets"),
	},
	{
		ID:         "tickets.custom-fields-list",
		Method:     "GET",
		Path:       "/tickets/{id}/custom-fields",
		Summary:    "List every custom field value on ticket (`id`). Use to read structured metadata an agent or integration attached...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "custom-fields-list", "List every custom field value on ticket (`id`). Use to read structured metadata an agent or integration attached...", "/tickets/{id}/custom-fields"),
	},
	{
		ID:         "tickets.custom-fields-set",
		Method:     "PUT",
		Path:       "/tickets/{ticket_id}/custom-fields/{id}",
		Summary:    "Set a single custom field value on a ticket: first `{id}` is the ticket, second `{id}` is the custom field. Use to...",
		Positional: []string{"ticket_id"},
		keywords:   codeOrchKeywords("tickets", "custom-fields-set", "Set a single custom field value on a ticket: first `{id}` is the ticket, second `{id}` is the custom field. Use to...", "/tickets/{ticket_id}/custom-fields/{id}"),
	},
	{
		ID:         "tickets.custom-fields-set-all",
		Method:     "PUT",
		Path:       "/tickets/{id}/custom-fields",
		Summary:    "Bulk-set custom field values on ticket (`id`) — pass an array of field/value pairs. Preferred when an agent needs...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "custom-fields-set-all", "Bulk-set custom field values on ticket (`id`) — pass an array of field/value pairs. Preferred when an agent needs...", "/tickets/{id}/custom-fields"),
	},
	{
		ID:         "tickets.custom-fields-unset",
		Method:     "DELETE",
		Path:       "/tickets/{ticket_id}/custom-fields/{id}",
		Summary:    "Clear a custom field value on a ticket: first `{id}` is the ticket, second `{id}` is the custom field. Unsets (does...",
		Positional: []string{"ticket_id", "id"},
		keywords:   codeOrchKeywords("tickets", "custom-fields-unset", "Clear a custom field value on a ticket: first `{id}` is the ticket, second `{id}` is the custom field. Unsets (does...", "/tickets/{ticket_id}/custom-fields/{id}"),
	},
	{
		ID:         "tickets.delete",
		Method:     "DELETE",
		Path:       "/tickets/{id}",
		Summary:    "Delete a ticket by `id`. Hard-deletes the conversation and its messages — reserve for GDPR erasure, spam, or...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "delete", "Delete a ticket by `id`. Hard-deletes the conversation and its messages — reserve for GDPR erasure, spam, or...", "/tickets/{id}"),
	},
	{
		ID:         "tickets.get",
		Method:     "GET",
		Path:       "/tickets/{id}",
		Summary:    "Fetch a single ticket by `id` with status, channel, assignee, customer, tags, and summary fields. Use after...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "get", "Fetch a single ticket by `id` with status, channel, assignee, customer, tags, and summary fields. Use after...", "/tickets/{id}"),
	},
	{
		ID:         "tickets.list",
		Method:     "GET",
		Path:       "/tickets",
		Summary:    "List tickets with filters (status, assignee, customer, channel, datetime, tag). The agent's primary endpoint for...",
		Positional: []string{},
		keywords:   codeOrchKeywords("tickets", "list", "List tickets with filters (status, assignee, customer, channel, datetime, tag). The agent's primary endpoint for...", "/tickets"),
	},
	{
		ID:         "tickets.messages-create",
		Method:     "POST",
		Path:       "/tickets/{id}/messages",
		Summary:    "Post a new message on ticket (`id`) — used to reply to the customer or write an internal note. The body...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "messages-create", "Post a new message on ticket (`id`) — used to reply to the customer or write an internal note. The body...", "/tickets/{id}/messages"),
	},
	{
		ID:         "tickets.messages-delete",
		Method:     "DELETE",
		Path:       "/tickets/{ticket_id}/messages/{id}",
		Summary:    "Delete a message from a ticket: first `{id}` is the ticket, second `{id}` is the message. Use sparingly —...",
		Positional: []string{"ticket_id", "id"},
		keywords:   codeOrchKeywords("tickets", "messages-delete", "Delete a message from a ticket: first `{id}` is the ticket, second `{id}` is the message. Use sparingly —...", "/tickets/{ticket_id}/messages/{id}"),
	},
	{
		ID:         "tickets.messages-get",
		Method:     "GET",
		Path:       "/tickets/{ticket_id}/messages/{id}",
		Summary:    "Fetch a single message: first `{id}` is the ticket, second `{id}` is the message. Use to load full body and...",
		Positional: []string{"ticket_id", "id"},
		keywords:   codeOrchKeywords("tickets", "messages-get", "Fetch a single message: first `{id}` is the ticket, second `{id}` is the message. Use to load full body and...", "/tickets/{ticket_id}/messages/{id}"),
	},
	{
		ID:         "tickets.messages-list",
		Method:     "GET",
		Path:       "/tickets/{id}/messages",
		Summary:    "List all messages on ticket (`id`) in chronological order — both customer-sent and agent-sent, public and internal...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "messages-list", "List all messages on ticket (`id`) in chronological order — both customer-sent and agent-sent, public and internal...", "/tickets/{id}/messages"),
	},
	{
		ID:         "tickets.messages-update",
		Method:     "PUT",
		Path:       "/tickets/{ticket_id}/messages/{id}",
		Summary:    "Update a message: first `{id}` is the ticket, second `{id}` is the message. Typically used to edit an internal...",
		Positional: []string{"ticket_id", "id"},
		keywords:   codeOrchKeywords("tickets", "messages-update", "Update a message: first `{id}` is the ticket, second `{id}` is the message. Typically used to edit an internal...", "/tickets/{ticket_id}/messages/{id}"),
	},
	{
		ID:         "tickets.tags-add",
		Method:     "POST",
		Path:       "/tickets/{id}/tags",
		Summary:    "Add one or more tags to ticket (`id`). The body shape (tag IDs vs names; whether unknown names auto-create) is not...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "tags-add", "Add one or more tags to ticket (`id`). The body shape (tag IDs vs names; whether unknown names auto-create) is not...", "/tickets/{id}/tags"),
	},
	{
		ID:         "tickets.tags-list",
		Method:     "GET",
		Path:       "/tickets/{id}/tags",
		Summary:    "List the tags currently attached to ticket (`id`). Use to read the ticket's categorization before deciding what...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "tags-list", "List the tags currently attached to ticket (`id`). Use to read the ticket's categorization before deciding what...", "/tickets/{id}/tags"),
	},
	{
		ID:         "tickets.tags-remove",
		Method:     "DELETE",
		Path:       "/tickets/{id}/tags",
		Summary:    "Remove tags from ticket (`id`). Pass the tag IDs/names to detach. Use when re-categorizing a ticket or undoing an...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "tags-remove", "Remove tags from ticket (`id`). Pass the tag IDs/names to detach. Use when re-categorizing a ticket or undoing an...", "/tickets/{id}/tags"),
	},
	{
		ID:         "tickets.tags-replace",
		Method:     "PUT",
		Path:       "/tickets/{id}/tags",
		Summary:    "Replace ticket (`id`)'s entire tag set with the supplied list. Use for full re-tagging; for additive/subtractive...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "tags-replace", "Replace ticket (`id`)'s entire tag set with the supplied list. Use for full re-tagging; for additive/subtractive...", "/tickets/{id}/tags"),
	},
	{
		ID:         "tickets.update",
		Method:     "PUT",
		Path:       "/tickets/{id}",
		Summary:    "Update a ticket (`id`) — change status (`open`/`closed`/`resolved`), assignee, priority, subject, or `via`. The...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("tickets", "update", "Update a ticket (`id`) — change status (`open`/`closed`/`resolved`), assignee, priority, subject, or `via`. The...", "/tickets/{id}"),
	},
	{
		ID:         "users.create",
		Method:     "POST",
		Path:       "/users",
		Summary:    "Create a new user (Gorgias agent/operator). Pass name, email, role, and optionally team memberships. Use when...",
		Positional: []string{},
		keywords:   codeOrchKeywords("users", "create", "Create a new user (Gorgias agent/operator). Pass name, email, role, and optionally team memberships. Use when...", "/users"),
	},
	{
		ID:         "users.delete",
		Method:     "DELETE",
		Path:       "/users/{id}",
		Summary:    "Delete a user (`id`) — deactivates the agent account and removes them from routing. Their historical ticket...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("users", "delete", "Delete a user (`id`) — deactivates the agent account and removes them from routing. Their historical ticket...", "/users/{id}"),
	},
	{
		ID:         "users.get",
		Method:     "GET",
		Path:       "/users/{id}",
		Summary:    "Fetch a single user (`id`) — agent name, email, role, teams, status. Use to look up who an assignee is or to...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("users", "get", "Fetch a single user (`id`) — agent name, email, role, teams, status. Use to look up who an assignee is or to...", "/users/{id}"),
	},
	{
		ID:         "users.list",
		Method:     "GET",
		Path:       "/users",
		Summary:    "List users (Gorgias agents/operators) on the account, with filters for role, status, and team. The agent's lookup...",
		Positional: []string{},
		keywords:   codeOrchKeywords("users", "list", "List users (Gorgias agents/operators) on the account, with filters for role, status, and team. The agent's lookup...", "/users"),
	},
	{
		ID:         "users.update",
		Method:     "PUT",
		Path:       "/users/{id}",
		Summary:    "Update a user (`id`) — change role, name, team membership, or active state. Use for admin operations like...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("users", "update", "Update a user (`id`) — change role, name, team membership, or active state. Use for admin operations like...", "/users/{id}"),
	},
	{
		ID:         "views.create",
		Method:     "POST",
		Path:       "/views",
		Summary:    "Create a saved view — a filtered ticket list (e.g. 'My open tickets', 'Urgent + unassigned') defined by...",
		Positional: []string{},
		keywords:   codeOrchKeywords("views", "create", "Create a saved view — a filtered ticket list (e.g. 'My open tickets', 'Urgent + unassigned') defined by...", "/views"),
	},
	{
		ID:         "views.delete",
		Method:     "DELETE",
		Path:       "/views/{id}",
		Summary:    "Delete a saved view by `id`. Removes it from the sidebar for everyone who saw it. Use when retiring stale filters.",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("views", "delete", "Delete a saved view by `id`. Removes it from the sidebar for everyone who saw it. Use when retiring stale filters.", "/views/{id}"),
	},
	{
		ID:         "views.get",
		Method:     "GET",
		Path:       "/views/{id}",
		Summary:    "Fetch a single saved view (`id`) with its filter definition and metadata. Use to introspect what conditions a view...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("views", "get", "Fetch a single saved view (`id`) with its filter definition and metadata. Use to introspect what conditions a view...", "/views/{id}"),
	},
	{
		ID:         "views.items-list",
		Method:     "GET",
		Path:       "/views/{id}/items",
		Summary:    "Return the ticket items currently matching saved view (`id`). Required: `id` (path). Optional: `cursor`, `direction`...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("views", "items-list", "Return the ticket items currently matching saved view (`id`). Required: `id` (path). Optional: `cursor`, `direction`...", "/views/{id}/items"),
	},
	{
		ID:         "views.items-update",
		Method:     "PUT",
		Path:       "/views/{id}/items",
		Summary:    "Update the materialized items of a view (`id`) — used to reorder, bulk-mutate, or refresh the cached set depending...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("views", "items-update", "Update the materialized items of a view (`id`) — used to reorder, bulk-mutate, or refresh the cached set depending...", "/views/{id}/items"),
	},
	{
		ID:         "views.list",
		Method:     "GET",
		Path:       "/views",
		Summary:    "List all saved views on the account, including ownership and visibility. The agent's catalogue of pre-built ticket...",
		Positional: []string{},
		keywords:   codeOrchKeywords("views", "list", "List all saved views on the account, including ownership and visibility. The agent's catalogue of pre-built ticket...", "/views"),
	},
	{
		ID:         "views.update",
		Method:     "PUT",
		Path:       "/views/{id}",
		Summary:    "Update a saved view (`id`) — change its filter criteria, name, or sharing. Use to evolve a view's definition as...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("views", "update", "Update a saved view (`id`) — change its filter criteria, name, or sharing. Use to evolve a view's definition as...", "/views/{id}"),
	},
	{
		ID:         "widgets.create",
		Method:     "POST",
		Path:       "/widgets",
		Summary:    "Create a new agent-facing sidebar widget shown inside the Gorgias helpdesk (on ticket, customer, or user views —...",
		Positional: []string{},
		keywords:   codeOrchKeywords("widgets", "create", "Create a new agent-facing sidebar widget shown inside the Gorgias helpdesk (on ticket, customer, or user views —...", "/widgets"),
	},
	{
		ID:         "widgets.delete",
		Method:     "DELETE",
		Path:       "/widgets/{id}",
		Summary:    "Delete a sidebar widget config by `id`. After deletion the widget stops rendering in the helpdesk UI on the...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("widgets", "delete", "Delete a sidebar widget config by `id`. After deletion the widget stops rendering in the helpdesk UI on the...", "/widgets/{id}"),
	},
	{
		ID:         "widgets.get",
		Method:     "GET",
		Path:       "/widgets/{id}",
		Summary:    "Fetch a single sidebar widget (`id`) with its `context` (ticket/customer/user), `template` (data source), order, and...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("widgets", "get", "Fetch a single sidebar widget (`id`) with its `context` (ticket/customer/user), `template` (data source), order, and...", "/widgets/{id}"),
	},
	{
		ID:         "widgets.list",
		Method:     "GET",
		Path:       "/widgets",
		Summary:    "List all agent-facing sidebar widgets on the account, optionally filtered by `integration_id` or `app_id`. Use to...",
		Positional: []string{},
		keywords:   codeOrchKeywords("widgets", "list", "List all agent-facing sidebar widgets on the account, optionally filtered by `integration_id` or `app_id`. Use to...", "/widgets"),
	},
	{
		ID:         "widgets.update",
		Method:     "PUT",
		Path:       "/widgets/{id}",
		Summary:    "Update a sidebar widget (`id`) — typically to change its `template` (data source), `order` (display position),...",
		Positional: []string{"id"},
		keywords:   codeOrchKeywords("widgets", "update", "Update a sidebar widget (`id`) — typically to change its `template` (data source), `order` (display position),...", "/widgets/{id}"),
	},
}

// codeOrchStopwords filters two-letter and short common-word substrings
// that pollute ranking via the substring-contains rule. Without them, a
// search for "list links" matches every endpoint whose description
// contains "is enrolled" because "is" is two chars and the matcher
// accepts kw.contains(t) || t.contains(kw).
var codeOrchStopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true,
	"at": true, "be": true, "by": true, "for": true, "from": true,
	"has": true, "in": true, "is": true, "it": true, "its": true,
	"of": true, "on": true, "or": true, "that": true, "the": true,
	"this": true, "to": true, "was": true, "will": true, "with": true,
	"your": true, "you": true, "any": true, "all": true,
}

// codeOrchKeywords produces the lowercase token stream used for search
// ranking. Defined at package level so the registry initializer can call it
// inline above without pulling in a separate precompute step.
func codeOrchKeywords(resource, endpoint, summary, path string) []string {
	raw := strings.ToLower(resource + " " + endpoint + " " + summary + " " + path)
	raw = strings.Map(func(r rune) rune {
		switch r {
		case '_', '-', '/', '{', '}', '.', ',', ':', ';':
			return ' '
		}
		return r
	}, raw)
	out := make([]string, 0, 16)
	seen := map[string]bool{}
	for _, tok := range strings.Fields(raw) {
		if len(tok) < 3 || codeOrchStopwords[tok] || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

func handleCodeOrchSearch(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}
	limit := 10
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	terms := codeOrchKeywords("", "", query, "")
	type scored struct {
		ep    *codeOrchEndpoint
		score int
	}
	results := make([]scored, 0, len(codeOrchEndpoints))
	for i := range codeOrchEndpoints {
		ep := &codeOrchEndpoints[i]
		score := 0
		for _, t := range terms {
			for _, kw := range ep.keywords {
				if kw == t {
					score += 2
				} else if strings.Contains(kw, t) || strings.Contains(t, kw) {
					score++
				}
			}
		}
		if score > 0 {
			results = append(results, scored{ep: ep, score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"endpoint_id": r.ep.ID,
			"method":      r.ep.Method,
			"path":        r.ep.Path,
			"summary":     r.ep.Summary,
			"score":       r.score,
		})
	}
	data, _ := json.Marshal(map[string]any{"count": len(out), "results": out})
	return mcplib.NewToolResultText(string(data)), nil
}

func handleCodeOrchExecute(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	id, ok := args["endpoint_id"].(string)
	if !ok || id == "" {
		return mcplib.NewToolResultError("endpoint_id is required (call gorgias_search first)"), nil
	}

	var ep *codeOrchEndpoint
	for i := range codeOrchEndpoints {
		if codeOrchEndpoints[i].ID == id {
			ep = &codeOrchEndpoints[i]
			break
		}
	}
	if ep == nil {
		return mcplib.NewToolResultError(fmt.Sprintf("unknown endpoint_id %q — call gorgias_search to discover valid ids", id)), nil
	}

	params, _ := args["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	path := ep.Path
	for _, p := range ep.Positional {
		if v, ok := params[p]; ok {
			path = strings.ReplaceAll(path, "{"+p+"}", fmt.Sprintf("%v", v))
			delete(params, p)
		}
	}

	query := map[string]string{}
	if ep.Method == "GET" || ep.Method == "DELETE" {
		for k, v := range params {
			query[k] = fmt.Sprintf("%v", v)
		}
	}

	var data json.RawMessage
	switch ep.Method {
	case "GET":
		data, err = c.Get(path, query)
	case "DELETE":
		data, _, err = c.Delete(path)
	default:
		body, mErr := json.Marshal(params)
		if mErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("marshaling body: %v", mErr)), nil
		}
		switch ep.Method {
		case "POST":
			data, _, err = c.Post(path, body)
		case "PUT":
			data, _, err = c.Put(path, body)
		case "PATCH":
			data, _, err = c.Patch(path, body)
		default:
			return mcplib.NewToolResultError(fmt.Sprintf("unsupported method %q", ep.Method)), nil
		}
	}
	if err != nil {
		return classifyMCPAPIError(err, ep.Method), nil
	}
	return mcplib.NewToolResultText(string(data)), nil
}
