// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Approved absorb-manifest command surface. Configuration-registry operations
// are intentionally capability-gated because stock Home Assistant exposes them
// through its authenticated WebSocket command API rather than a REST endpoint.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/home-assistant/internal/client"
	"github.com/spf13/cobra"
)

// pp:data-source live
func init() { registerNovelCommand(addManifestSurface) }

func addManifestSurface(root *cobra.Command, flags *rootFlags) {
	for group, leaves := range manifestGroups {
		parent := ensureManifestGroup(root, strings.Fields(group), flags)
		for _, leaf := range strings.Fields(leaves) {
			parent.AddCommand(newManifestLeaf(group+" "+leaf, flags))
		}
	}
	for _, name := range []string{"find", "overview", "logs", "reload", "restart"} {
		root.AddCommand(newManifestLeaf(name, flags))
	}
}

// Every path below is named by the approved absorb manifest. A command that
// has no stock REST equivalent fails explicitly with a typed capability error
// instead of claiming a request succeeded or synthesizing a response.
var manifestGroups = map[string]string{
	"area":               "list set remove",
	"floor":              "list set remove",
	"device":             "list get set remove assign",
	"entity":             "list get set remove",
	"label":              "list get set remove",
	"category":           "list get set remove",
	"zone":               "list get set remove",
	"bulk":               "plan apply status",
	"automation":         "get set remove run",
	"scene":              "get set remove activate",
	"script":             "get set remove run",
	"trace":              "list get",
	"integration":        "list get flow options enable disable remove",
	"helper":             "list get set remove",
	"group":              "list set remove",
	"dashboard":          "list get search set remove screenshot",
	"dashboard resource": "list get set remove",
	"blueprint":          "list get import",
	"calendar":           "events create update remove",
	"todo":               "list items add update remove",
	"camera":             "snapshot",
	"energy":             "get plan apply",
	"addon":              "list get plan apply call",
	"hacs":               "search get install update add-repository",
	"radio":              "inspect plan apply",
	"assist pipeline":    "list get set remove",
	"exposure":           "list set",
	"file":               "list read write remove",
	"yaml":               "get plan apply",
	"theme":              "list get set remove",
	"backup":             "list get create restore remove",
	"update":             "list plan apply skip unskip",
	"raw":                "rest websocket",
	"diagnostics":        "bundle",
	"system":             "info",
	"statistics":         "list",
	"event":              "watch",
	"config":             "get",
	"history":            "list",
	"logbook":            "list",
}

func ensureManifestGroup(root *cobra.Command, parts []string, flags *rootFlags) *cobra.Command {
	current := root
	for _, part := range parts {
		var next *cobra.Command
		for _, existing := range current.Commands() {
			if existing.Name() == part {
				next = existing
				break
			}
		}
		if next == nil {
			next = &cobra.Command{Use: part, Short: "Home Assistant " + part + " operations", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: parentNoSubcommandRunE(flags)}
			current.AddCommand(next)
		}
		current = next
	}
	return current
}

func newManifestLeaf(path string, flags *rootFlags) *cobra.Command {
	parts := strings.Fields(path)
	leaf := parts[len(parts)-1]
	cmd := &cobra.Command{
		Use:         leaf,
		Short:       "Run Home Assistant " + path + " through the supported installation capability",
		Example:     "home-assistant-pp-cli " + path + " --dry-run --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	var data string
	cmd.Flags().StringVar(&data, "data", "", "JSON command data required by Home Assistant WebSocket mutations")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return workflowOutput(map[string]any{"dry_run": true, "command": path}, flags, cmd.OutOrStdout())
		}
		supplied := map[string]any{}
		if data != "" {
			if err := json.Unmarshal([]byte(data), &supplied); err != nil {
				return usageErr(fmt.Errorf("--data must be a JSON object: %w", err))
			}
		}
		if path == "raw rest" {
			return runRawREST(cmd, flags, supplied)
		}
		if path == "raw websocket" {
			return runRawWebSocket(cmd, flags, supplied)
		}
		if path == "camera snapshot" {
			return runCameraSnapshot(cmd, flags, supplied)
		}
		if path == "diagnostics bundle" {
			return runDiagnosticsBundle(cmd, flags)
		}
		if path == "config get" {
			return runConfigGet(cmd, flags)
		}
		if path == "find" {
			return runFind(cmd, flags, supplied)
		}
		if path == "overview" {
			return runOverview(cmd, flags)
		}
		if strings.HasPrefix(path, "bulk ") {
			return runBulk(cmd, flags, path, supplied)
		}
		if path == "calendar events" {
			return runCalendarEvents(cmd, flags, supplied)
		}
		if path == "logs" {
			return runLogs(cmd, flags)
		}
		if path == "event watch" {
			return runEventWatch(cmd, flags, supplied)
		}
		if strings.HasPrefix(path, "helper ") {
			return runHelper(cmd, flags, path, supplied)
		}
		if strings.HasPrefix(path, "radio ") {
			return runRadio(cmd, flags, path, supplied)
		}
		if strings.HasPrefix(path, "zone ") {
			return runZone(cmd, flags, path, supplied)
		}
		if strings.HasPrefix(path, "file ") || strings.HasPrefix(path, "yaml ") || strings.HasPrefix(path, "theme ") {
			return runLocalConfig(cmd, flags, path, supplied)
		}
		if path == "group list" {
			return runGroupList(cmd, flags)
		}
		if service, ok := manifestServiceOps[path]; ok {
			segments := strings.Split(service, ".")
			result, err := callHAService(cmd.Context(), flags, segments[0], segments[1], supplied)
			if err != nil {
				return err
			}
			return workflowOutput(map[string]any{"command": path, "service": service, "result": json.RawMessage(result)}, flags, cmd.OutOrStdout())
		}
		if op, ok := manifestRESTOps[path]; ok {
			return runManifestREST(cmd, flags, path, op, supplied)
		}
		if op, ok := manifestSupervisorOps[path]; ok {
			route, err := manifestRoute(op.path, supplied)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result, err := c.SupervisorCall(cmd.Context(), op.method, route, supplied)
			if err != nil {
				var capability *client.CapabilityError
				if errors.As(err, &capability) {
					return capabilityUnavailable(path)
				}
				return classifyAPIError(err, flags)
			}
			return workflowOutput(map[string]any{"command": path, "supervisor_path": route, "result": json.RawMessage(result)}, flags, cmd.OutOrStdout())
		}
		if messageType, ok := manifestWSOps[path]; ok {
			message := map[string]any{"type": messageType}
			for key, value := range supplied {
				message[key] = value
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result, err := c.HomeAssistantWSCall(cmd.Context(), message)
			if err != nil {
				var capability *client.CapabilityError
				if errors.As(err, &capability) {
					return capabilityUnavailable(path)
				}
				return classifyAPIError(err, flags)
			}
			return workflowOutput(map[string]any{"command": path, "websocket_type": messageType, "result": json.RawMessage(result)}, flags, cmd.OutOrStdout())
		}
		return probedCapabilityUnavailable(cmd, flags, path)
	}
	return cmd
}

func runRawREST(cmd *cobra.Command, flags *rootFlags, supplied map[string]any) error {
	method := strings.ToUpper(strings.TrimSpace(stringData(supplied, "method")))
	if method == "" {
		method = "GET"
	}
	path := strings.TrimSpace(stringData(supplied, "path"))
	if !strings.HasPrefix(path, "/api/") {
		return usageErr(fmt.Errorf("--data.path must begin with /api/"))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	body := supplied["body"]
	var result json.RawMessage
	switch method {
	case "GET":
		result, err = c.Get(cmd.Context(), path, nil)
	case "POST":
		result, _, err = c.Post(cmd.Context(), path, body)
	case "PUT":
		result, _, err = c.Put(cmd.Context(), path, body)
	case "DELETE":
		result, _, err = c.DeleteWithBody(cmd.Context(), path, body)
	default:
		return usageErr(fmt.Errorf("--data.method must be GET, POST, PUT, or DELETE"))
	}
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": "raw rest", "method": method, "path": path, "result": result}, flags, cmd.OutOrStdout())
}

func runRawWebSocket(cmd *cobra.Command, flags *rootFlags, supplied map[string]any) error {
	message, ok := supplied["command"].(map[string]any)
	if !ok || strings.TrimSpace(fmt.Sprint(message["type"])) == "" {
		return usageErr(fmt.Errorf("--data.command must be a JSON object with a non-empty type"))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	result, err := c.HomeAssistantWSCall(cmd.Context(), message)
	if err != nil {
		var capability *client.CapabilityError
		if errors.As(err, &capability) {
			return capabilityUnavailable("raw websocket command " + fmt.Sprint(message["type"]))
		}
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": "raw websocket", "websocket_type": message["type"], "result": result}, flags, cmd.OutOrStdout())
}

func runCameraSnapshot(cmd *cobra.Command, flags *rootFlags, supplied map[string]any) error {
	entityID := strings.TrimSpace(stringData(supplied, "entity_id"))
	if entityID == "" {
		return usageErr(fmt.Errorf("--data must contain non-empty entity_id"))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	result, err := c.GetWithHeaders(cmd.Context(), "/api/camera_proxy/"+url.PathEscape(entityID), nil, map[string]string{client.BinaryResponseHeader: "true"})
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": "camera snapshot", "entity_id": entityID, "snapshot": result}, flags, cmd.OutOrStdout())
}

func runDiagnosticsBundle(cmd *cobra.Command, flags *rootFlags) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	config, err := c.Get(cmd.Context(), "/api/config", nil)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	components, err := c.Get(cmd.Context(), "/api/components", nil)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": "diagnostics bundle", "config": config, "components": components}, flags, cmd.OutOrStdout())
}

func runConfigGet(cmd *cobra.Command, flags *rootFlags) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	result, err := c.Get(cmd.Context(), "/api/config", nil)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": "config get", "result": result}, flags, cmd.OutOrStdout())
}

func runFind(cmd *cobra.Command, flags *rootFlags, supplied map[string]any) error {
	query := strings.ToLower(strings.TrimSpace(stringData(supplied, "query")))
	if query == "" {
		return usageErr(fmt.Errorf("--data must contain non-empty query"))
	}
	states, err := householdStates(cmd.Context(), flags)
	if err != nil {
		return err
	}
	matches := make([]map[string]any, 0)
	for _, state := range states {
		if strings.Contains(strings.ToLower(entityID(state)), query) || strings.Contains(strings.ToLower(friendlyName(state)), query) {
			matches = append(matches, state)
		}
	}
	return workflowOutput(map[string]any{"command": "find", "query": query, "matches": matches}, flags, cmd.OutOrStdout())
}

func runOverview(cmd *cobra.Command, flags *rootFlags) error {
	states, err := householdStates(cmd.Context(), flags)
	if err != nil {
		return err
	}
	byDomain := map[string]int{}
	for _, state := range states {
		domain, _, found := strings.Cut(entityID(state), ".")
		if found {
			byDomain[domain]++
		}
	}
	return workflowOutput(map[string]any{"command": "overview", "entity_count": len(states), "entities_by_domain": byDomain}, flags, cmd.OutOrStdout())
}

func runBulk(cmd *cobra.Command, flags *rootFlags, path string, supplied map[string]any) error {
	states, err := householdStates(cmd.Context(), flags)
	if err != nil {
		return err
	}
	if path == "bulk apply" {
		domain, service := strings.TrimSpace(stringData(supplied, "domain")), strings.TrimSpace(stringData(supplied, "service"))
		if domain == "" || service == "" {
			return usageErr(fmt.Errorf("--data must contain non-empty domain and service for bulk apply"))
		}
		result, err := callHAService(cmd.Context(), flags, domain, service, supplied)
		if err != nil {
			return err
		}
		return workflowOutput(map[string]any{"command": path, "service": domain + "." + service, "result": result}, flags, cmd.OutOrStdout())
	}
	return workflowOutput(map[string]any{"command": path, "entity_count": len(states), "states": states}, flags, cmd.OutOrStdout())
}

func runCalendarEvents(cmd *cobra.Command, flags *rootFlags, supplied map[string]any) error {
	calendarID := strings.TrimSpace(stringData(supplied, "calendar_id"))
	start, end := strings.TrimSpace(stringData(supplied, "start")), strings.TrimSpace(stringData(supplied, "end"))
	if calendarID == "" || start == "" || end == "" {
		return usageErr(fmt.Errorf("--data must contain calendar_id, start, and end"))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	result, err := c.Get(cmd.Context(), "/api/calendars/"+url.PathEscape(calendarID), map[string]string{"start": start, "end": end})
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": "calendar events", "calendar_id": calendarID, "result": result}, flags, cmd.OutOrStdout())
}

func runLogs(cmd *cobra.Command, flags *rootFlags) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	result, err := c.Get(cmd.Context(), "/api/error_log", nil)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": "logs", "result": result}, flags, cmd.OutOrStdout())
}

func runEventWatch(cmd *cobra.Command, flags *rootFlags, supplied map[string]any) error {
	eventType := strings.TrimSpace(stringData(supplied, "event_type"))
	if eventType == "" {
		return usageErr(fmt.Errorf("--data must contain non-empty event_type"))
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), flags.timeout)
	defer cancel()
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	event, err := c.HomeAssistantWSWatchEvent(ctx, eventType)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": "event watch", "event_type": eventType, "event": event}, flags, cmd.OutOrStdout())
}

func runHelper(cmd *cobra.Command, flags *rootFlags, path string, supplied map[string]any) error {
	message := map[string]any{}
	for key, value := range supplied {
		message[key] = value
	}
	if path == "helper list" {
		message["type"] = "ha_mcp_tools/helpers_list"
	} else {
		helperType := strings.TrimSpace(stringData(supplied, "helper_type"))
		if helperType == "" || strings.ContainsAny(helperType, "/ ") {
			return usageErr(fmt.Errorf("--data must contain a helper_type without spaces or slashes"))
		}
		switch path {
		case "helper get":
			message["type"] = helperType + "/list"
		case "helper set":
			if strings.TrimSpace(stringData(supplied, "id")) == "" {
				message["type"] = helperType + "/create"
			} else {
				message["type"] = helperType + "/update"
			}
		case "helper remove":
			message["type"] = helperType + "/delete"
		}
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	result, err := c.HomeAssistantWSCall(cmd.Context(), message)
	if err != nil {
		var capability *client.CapabilityError
		if errors.As(err, &capability) {
			return capabilityUnavailable(path)
		}
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": path, "websocket_type": message["type"], "result": result}, flags, cmd.OutOrStdout())
}

var radioApplyTypes = map[string]bool{
	"zha/devices/permit": true, "zha/devices/reconfigure": true,
	"zwave_js/add_node": true, "zwave_js/remove_node": true, "zwave_js/refresh_node_info": true,
	"matter/commission": true, "matter/remove_matter_fabric": true,
	"thread/add_dataset_tlv": true, "thread/set_preferred_border_agent": true,
}

func runRadio(cmd *cobra.Command, flags *rootFlags, path string, supplied map[string]any) error {
	message := map[string]any{}
	for key, value := range supplied {
		message[key] = value
	}
	switch path {
	case "radio inspect", "radio plan":
		message["type"] = "zha/devices"
	case "radio apply":
		typeName := strings.TrimSpace(stringData(supplied, "command_type"))
		if !radioApplyTypes[typeName] {
			return usageErr(fmt.Errorf("--data.command_type must be one of the documented ZHA, Z-Wave JS, Matter, or Thread operations"))
		}
		message["type"] = typeName
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	result, err := c.HomeAssistantWSCall(cmd.Context(), message)
	if err != nil {
		var capability *client.CapabilityError
		if errors.As(err, &capability) {
			return capabilityUnavailable(path)
		}
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": path, "websocket_type": message["type"], "result": result}, flags, cmd.OutOrStdout())
}

func runZone(cmd *cobra.Command, flags *rootFlags, path string, supplied map[string]any) error {
	message := map[string]any{}
	for key, value := range supplied {
		message[key] = value
	}
	switch path {
	case "zone list", "zone get":
		message["type"] = "zone/list"
	case "zone set":
		if strings.TrimSpace(stringData(supplied, "id")) == "" {
			message["type"] = "zone/create"
		} else {
			message["type"] = "zone/update"
		}
	case "zone remove":
		message["type"] = "zone/delete"
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	result, err := c.HomeAssistantWSCall(cmd.Context(), message)
	if err != nil {
		var capability *client.CapabilityError
		if errors.As(err, &capability) {
			return capabilityUnavailable(path)
		}
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": path, "websocket_type": message["type"], "result": result}, flags, cmd.OutOrStdout())
}

func runLocalConfig(cmd *cobra.Command, flags *rootFlags, command string, supplied map[string]any) error {
	path := strings.TrimSpace(stringData(supplied, "path"))
	if command == "theme list" && path == "" {
		path = "themes"
	}
	if path == "" {
		return usageErr(fmt.Errorf("--data must contain path relative to HASS_CONFIG_DIR"))
	}
	var result any
	var err error
	switch command {
	case "file list", "theme list":
		var entries []os.DirEntry
		entries, err = client.LocalConfigList(path)
		if err == nil {
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			result = names
		}
	case "file read", "yaml get", "yaml plan", "theme get":
		var contents []byte
		contents, err = client.LocalConfigRead(path)
		result = string(contents)
	case "file write", "yaml apply", "theme set":
		contents := stringData(supplied, "contents")
		if contents == "" {
			return usageErr(fmt.Errorf("--data must contain contents for %s", command))
		}
		err = client.LocalConfigWrite(path, []byte(contents))
		result = map[string]any{"written": true}
	case "file remove", "theme remove":
		err = client.LocalConfigDelete(path)
		result = map[string]any{"removed": true}
	default:
		return capabilityUnavailable(command)
	}
	if err != nil {
		var unavailable *client.LocalConfigUnavailableError
		if errors.As(err, &unavailable) {
			return capabilityUnavailable(command)
		}
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": command, "path": path, "result": result}, flags, cmd.OutOrStdout())
}

func runGroupList(cmd *cobra.Command, flags *rootFlags) error {
	states, err := householdStates(cmd.Context(), flags)
	if err != nil {
		return err
	}
	groups := make([]map[string]any, 0)
	for _, state := range states {
		if strings.HasPrefix(entityID(state), "group.") {
			groups = append(groups, state)
		}
	}
	return workflowOutput(map[string]any{"command": "group list", "groups": groups}, flags, cmd.OutOrStdout())
}

type restOperation struct{ method, path string }

// These paths are part of the approved absorb contract. Configuration entry
// flows are registered by Core's config-entry manager and require an admin
// bearer token; the routine configuration routes are intentionally explicit.
var manifestRESTOps = map[string]restOperation{
	"automation get": {"GET", "/api/config/automation/config/{id}"}, "automation set": {"POST", "/api/config/automation/config/{id}"}, "automation remove": {"DELETE", "/api/config/automation/config/{id}"},
	"scene get": {"GET", "/api/config/scene/config/{id}"}, "scene set": {"POST", "/api/config/scene/config/{id}"}, "scene remove": {"DELETE", "/api/config/scene/config/{id}"},
	"script get": {"GET", "/api/config/script/config/{id}"}, "script set": {"POST", "/api/config/script/config/{id}"}, "script remove": {"DELETE", "/api/config/script/config/{id}"},
	"integration flow": {"POST", "/api/config/config_entries/flow"}, "integration options": {"POST", "/api/config/config_entries/options/flow"},
}

func runManifestREST(cmd *cobra.Command, flags *rootFlags, command string, op restOperation, supplied map[string]any) error {
	route, err := manifestRoute(op.path, supplied)
	if err != nil {
		return usageErr(err)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	var result json.RawMessage
	switch op.method {
	case "GET":
		result, err = c.Get(cmd.Context(), route, nil)
	case "POST":
		result, _, err = c.Post(cmd.Context(), route, supplied)
	case "DELETE":
		result, _, err = c.DeleteWithBody(cmd.Context(), route, supplied)
	default:
		return fmt.Errorf("unsupported manifest REST method %q", op.method)
	}
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return workflowOutput(map[string]any{"command": command, "method": op.method, "path": route, "result": result}, flags, cmd.OutOrStdout())
}

func stringData(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}

type supervisorOperation struct{ method, path string }

var manifestSupervisorOps = map[string]supervisorOperation{
	"addon list": {"GET", "/addons"}, "addon get": {"GET", "/addons/{id}/info"}, "addon apply": {"POST", "/addons/{id}/options"}, "addon call": {"POST", "/addons/{id}/{action}"},
	"backup list": {"GET", "/backups"}, "backup get": {"GET", "/backups/{id}/info"}, "backup create": {"POST", "/backups/new/full"}, "backup restore": {"POST", "/backups/{id}/restore/full"}, "backup remove": {"DELETE", "/backups/{id}"},
	"update list": {"GET", "/available_updates"}, "update apply": {"POST", "/{target}/update"}, "update skip": {"POST", "/{target}/skip"}, "update unskip": {"POST", "/{target}/unskip"},
}

func manifestRoute(pattern string, supplied map[string]any) (string, error) {
	for _, key := range []string{"id", "action", "target"} {
		needle := "{" + key + "}"
		if strings.Contains(pattern, needle) {
			value, _ := supplied[key].(string)
			if strings.TrimSpace(value) == "" {
				return "", fmt.Errorf("--data must contain non-empty %q for %s", key, pattern)
			}
			pattern = strings.ReplaceAll(pattern, needle, url.PathEscape(value))
		}
	}
	return pattern, nil
}

// These are stock, documented WebSocket command types. Mutating commands take
// their documented command body through --data so the client never guesses a
// registry identifier or field value.
var manifestWSOps = map[string]string{
	"area list": "config/area_registry/list", "area set": "config/area_registry/update", "area remove": "config/area_registry/delete",
	"floor list": "config/floor_registry/list", "floor set": "config/floor_registry/update", "floor remove": "config/floor_registry/delete",
	"device list": "config/device_registry/list", "device set": "config/device_registry/update", "device remove": "config/device_registry/remove", "device assign": "config/device_registry/update",
	"entity list": "config/entity_registry/list", "entity set": "config/entity_registry/update", "entity remove": "config/entity_registry/remove",
	"label list": "config/label_registry/list", "label set": "config/label_registry/update", "label remove": "config/label_registry/delete",
	"category list": "config/category_registry/list", "category set": "config/category_registry/update", "category remove": "config/category_registry/delete",
	"zone list": "zone/list", "zone get": "zone/list", "zone set": "zone/update", "zone remove": "zone/delete",
	"integration list": "config_entries/get", "integration get": "config_entries/get", "integration enable": "config_entries/update", "integration disable": "config_entries/update", "integration remove": "config_entries/disable",
	"trace list": "trace/list", "trace get": "trace/get",
	"assist pipeline list": "assist_pipeline/pipeline/list", "assist pipeline get": "assist_pipeline/pipeline/get", "assist pipeline set": "assist_pipeline/pipeline/set", "assist pipeline remove": "assist_pipeline/pipeline/delete",
	"exposure list": "homeassistant/expose_entity/list", "exposure set": "homeassistant/expose_entity",
	"dashboard list": "lovelace/dashboards", "dashboard get": "lovelace/config", "dashboard search": "ha_mcp_tools/dashboards", "dashboard set": "lovelace/config/save", "dashboard remove": "lovelace/config/delete", "dashboard screenshot": "ha_mcp_tools/dashboards",
	"dashboard resource list": "lovelace/resources", "dashboard resource get": "lovelace/resources", "dashboard resource set": "lovelace/resources/create", "dashboard resource remove": "lovelace/resources/delete",
	"blueprint list": "blueprint/list", "blueprint get": "blueprint/list", "blueprint import": "blueprint/import",
	"calendar create": "calendar/event/create", "calendar update": "calendar/event/update", "calendar remove": "calendar/event/delete",
	"statistics list": "recorder/statistics_during_period",
	"energy get":      "energy/get_prefs", "energy plan": "energy/get_prefs", "energy apply": "energy/save_prefs",
	"hacs search": "hacs/repositories/list", "hacs get": "hacs/repository/info", "hacs install": "hacs/repository/download", "hacs update": "hacs/repository/download", "hacs add-repository": "hacs/repositories/add",
	"radio inspect": "zha/devices", "radio plan": "zha/devices", "radio apply": "zha/devices",
	"backup list": "backup/info", "backup get": "backup/info", "backup create": "backup/generate", "backup restore": "backup/restore", "backup remove": "backup/remove",
	"system info": "system_health/info",
}

var manifestServiceOps = map[string]string{
	"automation run": "automation.trigger",
	"scene activate": "scene.turn_on",
	"script run":     "script.turn_on",
	"update apply":   "update.install",
	"backup create":  "backup.create",
	"todo list":      "todo.get_items",
	"todo items":     "todo.get_items",
	"todo add":       "todo.add_item",
	"todo update":    "todo.update_item",
	"todo remove":    "todo.remove_item",
	"group set":      "group.set",
	"group remove":   "group.remove",
	"reload":         "homeassistant.reload_all",
	"restart":        "homeassistant.restart",
}

func capabilityUnavailable(path string) error {
	return apiErr(fmt.Errorf("capability unavailable: %s requires the Home Assistant WebSocket, Supervisor, or integration-specific API surface; configure an installation that exposes that capability", path))
}

func probedCapabilityUnavailable(cmd *cobra.Command, flags *rootFlags, path string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	if _, err := c.HomeAssistantWSCall(cmd.Context(), map[string]any{"type": "get_config"}); err != nil {
		return classifyAPIError(err, flags)
	}
	return capabilityUnavailable(path)
}
