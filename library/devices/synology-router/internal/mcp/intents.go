package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-router/internal/store"
)

// RegisterIntentTools registers high-level intent tools that combine
// multiple endpoints or provide agent-friendly abstractions. These
// sit alongside the typed endpoint tools and reduce the number of
// round-trips an agent needs for common workflows.
func RegisterIntentTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("network_overview",
			mcplib.WithDescription("Single-call network snapshot: WAN IP, device counts, top talkers, Wi-Fi SSIDs, and mesh health."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleNetworkOverview,
	)
	s.AddTool(
		mcplib.NewTool("device_lookup",
			mcplib.WithDescription("Find a device by hostname, IP, or MAC across synced and live data."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Hostname, IP, or MAC to search")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleDeviceLookup,
	)
	s.AddTool(
		mcplib.NewTool("busiest_devices",
			mcplib.WithDescription("Top bandwidth consumers aggregated from traffic data. Returns ranked list with human-readable sizes."),
			mcplib.WithString("interval", mcplib.Description("live|day|week|month (default: day)")),
			mcplib.WithNumber("top", mcplib.Description("Number of results (default: 10)")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleBusiestDevices,
	)
	s.AddTool(
		mcplib.NewTool("router_health",
			mcplib.WithDescription("Router health: CPU, memory, WAN status, device counts, and mesh node status in one call."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleRouterHealth,
	)
	s.AddTool(
		mcplib.NewTool("security_audit",
			mcplib.WithDescription("Firewall rules, Wi-Fi security, access control groups, and open ports summary."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleSecurityAudit,
	)
	s.AddTool(
		mcplib.NewTool("wake_device",
			mcplib.WithDescription("Wake a device by hostname or MAC. Registers if needed, then sends WOL packet."),
			mcplib.WithString("mac", mcplib.Description("MAC address (overrides hostname)")),
			mcplib.WithString("hostname", mcplib.Description("Hostname to look up MAC")),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleWakeDevice,
	)
	s.AddTool(
		mcplib.NewTool("stale_resources",
			mcplib.WithDescription("List synced resources older than a threshold. Helps decide when to re-sync."),
			mcplib.WithString("older_than", mcplib.Description("Duration threshold (e.g. 1h, 24h, 7d). Default: 1h")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleStaleResources,
	)
	s.AddTool(
		mcplib.NewTool("wifi_toggle",
			mcplib.WithDescription("Enable or disable a Wi-Fi SSID by name without replacing the full profile."),
			mcplib.WithString("ssid", mcplib.Required(), mcplib.Description("SSID name to toggle")),
			mcplib.WithBoolean("enable", mcplib.Required(), mcplib.Description("true to enable, false to disable")),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleWifiToggle,
	)
	s.AddTool(
		mcplib.NewTool("device_online",
			mcplib.WithDescription("Check if a specific device is currently online by hostname, IP, or MAC."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Hostname, IP, or MAC address")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleDeviceOnline,
	)
	s.AddTool(
		mcplib.NewTool("mesh_health",
			mcplib.WithDescription("Mesh node status with connected device counts and link quality."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleMeshHealth,
	)
	s.AddTool(
		mcplib.NewTool("dns_summary",
			mcplib.WithDescription("External IP plus all DDNS records in one call."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleDNSSummary,
	)
	s.AddTool(
		mcplib.NewTool("reboot_check",
			mcplib.WithDescription("Check if router needs a reboot: uptime, firmware version, and pending changes."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleRebootCheck,
	)
}

func intentDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "synology-router-pp-cli", "data.db")
}

func handleNetworkOverview(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	overview := map[string]any{}

	wanData, wanErr := c.Get("/wan/status", nil)
	if wanErr == nil {
		overview["wan"] = json.RawMessage(wanData)
	}

	devData, devErr := c.Get("/devices", map[string]string{"conntype": "all"})
	if devErr == nil {
		var items []map[string]any
		if raw := extractMCPRawData(devData); json.Unmarshal(raw, &items) == nil {
			online, offline := countOnline(items)
			overview["devices"] = map[string]any{"total": len(items), "online": online, "offline": offline}
		}
	}

	trafficData, trafficErr := c.Get("/traffic", map[string]string{"interval": "day"})
	if trafficErr == nil {
		var items []map[string]any
		if raw := extractMCPRawData(trafficData); json.Unmarshal(raw, &items) == nil {
			if len(items) > 5 {
				items = items[:5]
			}
			overview["top_talkers_today"] = items
		}
	}

	wifiData, wifiErr := c.Get("/wifi/settings", nil)
	if wifiErr == nil {
		var obj map[string]any
		if raw := extractMCPRawData(wifiData); json.Unmarshal(raw, &obj) == nil {
			if profiles, ok := obj["profiles"].([]any); ok {
				ssids := make([]string, 0, len(profiles))
				for _, p := range profiles {
					if pm, ok := p.(map[string]any); ok {
						if ssid, ok := pm["ssid"].(string); ok && ssid != "" {
							ssids = append(ssids, ssid)
						} else if name, ok := pm["name"].(string); ok && name != "" {
							ssids = append(ssids, name)
						}
					}
				}
				overview["wifi_ssids"] = ssids
			}
		}
	}

	meshData, meshErr := c.Get("/mesh/nodes", nil)
	if meshErr == nil {
		var items []map[string]any
		if raw := extractMCPRawData(meshData); json.Unmarshal(raw, &items) == nil {
			nodes := make([]map[string]any, 0, len(items))
			for _, n := range items {
				nodes = append(nodes, map[string]any{
					"hostname": n["hostname"],
					"status":   n["status"],
				})
			}
			overview["mesh_nodes"] = nodes
		}
	}

	data, _ := json.MarshalIndent(overview, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}

func handleDeviceLookup(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	query, _ := args["query"].(string)
	if query == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}

	db, err := store.OpenReadOnly(intentDBPath())
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("db: %v", err)), nil
	}
	defer db.Close()

	results, err := db.Search(query, 10)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}

	filtered := make([]json.RawMessage, 0)
	for _, r := range results {
		var obj map[string]any
		if json.Unmarshal(r, &obj) == nil {
			if rt, ok := obj["resource_type"].(string); !ok || rt != "devices" {
				continue
			}
		}
		filtered = append(filtered, r)
	}

	if len(filtered) == 0 {
		c, clientErr := newMCPClient()
		if clientErr != nil {
			return mcplib.NewToolResultError(clientErr.Error()), nil
		}
		devData, apiErr := c.Get("/devices", nil)
		if apiErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("not found: %v", apiErr)), nil
		}
		var items []map[string]any
		if raw := extractMCPRawData(devData); json.Unmarshal(raw, &items) == nil {
			for _, item := range items {
				if matchesDevice(item, query) {
					data, _ := json.Marshal(item)
					filtered = append(filtered, data)
				}
			}
		}
	}

	data, _ := json.MarshalIndent(map[string]any{"count": len(filtered), "items": filtered}, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}

func handleBusiestDevices(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	interval := "day"
	if v, ok := args["interval"].(string); ok && v != "" {
		interval = v
	}
	top := 10
	if v, ok := args["top"].(float64); ok && v > 0 {
		top = int(v)
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	data, err := c.Get("/traffic", map[string]string{"interval": interval})
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	var items []map[string]any
	if raw := extractMCPRawData(data); json.Unmarshal(raw, &items) != nil {
		return mcplib.NewToolResultText(string(raw)), nil
	}

	enrichTrafficItemsMCP(items)
	if top < len(items) {
		items = items[:top]
	}

	out, _ := json.MarshalIndent(map[string]any{"interval": interval, "count": len(items), "devices": items}, "", "  ")
	return mcplib.NewToolResultText(string(out)), nil
}

func handleRouterHealth(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	health := map[string]any{}

	utilData, utilErr := c.Get("/utilization", map[string]string{"resource": `["cpu","memory","network"]`})
	if utilErr == nil {
		health["utilization"] = json.RawMessage(utilData)
	}

	wanData, wanErr := c.Get("/wan/status", nil)
	if wanErr == nil {
		health["wan"] = json.RawMessage(wanData)
	}

	devData, devErr := c.Get("/devices", nil)
	if devErr == nil {
		var items []map[string]any
		if raw := extractMCPRawData(devData); json.Unmarshal(raw, &items) == nil {
			online, offline := countOnline(items)
			health["devices"] = map[string]any{"online": online, "offline": offline, "total": len(items)}
		}
	}

	meshData, meshErr := c.Get("/mesh/nodes", nil)
	if meshErr == nil {
		health["mesh"] = json.RawMessage(meshData)
	}

	out, _ := json.MarshalIndent(health, "", "  ")
	return mcplib.NewToolResultText(string(out)), nil
}

func handleSecurityAudit(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	audit := map[string]any{}

	fwData, fwErr := c.Get("/firewall/rules", map[string]string{"type": "ipv4"})
	if fwErr == nil {
		audit["firewall"] = json.RawMessage(fwData)
	}

	wifiData, wifiErr := c.Get("/wifi/settings", nil)
	if wifiErr == nil {
		audit["wifi"] = json.RawMessage(wifiData)
	}

	acData, acErr := c.Get("/access-control/groups", nil)
	if acErr == nil {
		audit["access_control"] = json.RawMessage(acData)
	}

	out, _ := json.MarshalIndent(audit, "", "  ")
	return mcplib.NewToolResultText(string(out)), nil
}

func handleWakeDevice(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	mac, _ := args["mac"].(string)
	hostname, _ := args["hostname"].(string)
	if mac == "" && hostname == "" {
		return mcplib.NewToolResultError("mac or hostname required"), nil
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	if mac == "" && hostname != "" {
		wolData, wolErr := c.Get("/wol/devices", nil)
		if wolErr != nil {
			return mcplib.NewToolResultError(wolErr.Error()), nil
		}
		var items []map[string]any
		if raw := extractMCPRawData(wolData); json.Unmarshal(raw, &items) == nil {
			for _, item := range items {
				if h, ok := item["host"].(string); ok && h == hostname {
					if m, ok := item["mac"].(string); ok {
						mac = m
						break
					}
				}
			}
		}
		if mac == "" {
			return mcplib.NewToolResultError(fmt.Sprintf("hostname %q not found in WOL devices", hostname)), nil
		}
	}

	data, _, err := c.PostWithParams("/wol/wake", nil, map[string]any{"mac": mac})
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(string(data)), nil
}

func handleStaleResources(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	threshold := "1h"
	if v, ok := args["older_than"].(string); ok && v != "" {
		threshold = v
	}

	db, err := store.OpenReadOnly(intentDBPath())
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("db: %v", err)), nil
	}
	defer db.Close()

	rows, err := db.DB().Query(`SELECT resource_type, total_count, last_synced_at FROM sync_state ORDER BY resource_type`)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	defer rows.Close()

	stale := []map[string]any{}
	for rows.Next() {
		var rtype string
		var count int
		var lastSynced string
		if err := rows.Scan(&rtype, &count, &lastSynced); err != nil {
			continue
		}
		stale = append(stale, map[string]any{
			"resource":      rtype,
			"total_count":   count,
			"last_synced":   lastSynced,
			"stale_after":   threshold,
		})
	}

	out, _ := json.MarshalIndent(map[string]any{"threshold": threshold, "resources": stale}, "", "  ")
	return mcplib.NewToolResultText(string(out)), nil
}

func countOnline(items []map[string]any) (online, offline int) {
	for _, item := range items {
		for _, k := range []string{"is_online", "online", "connected", "status"} {
			if v, ok := item[k]; ok {
				switch val := v.(type) {
				case bool:
					if val {
						online++
					} else {
						offline++
					}
				case string:
					if val == "true" || val == "online" || val == "connected" {
						online++
					} else {
						offline++
					}
				}
				break
			}
		}
	}
	return
}

func matchesDevice(item map[string]any, query string) bool {
	q := query
	for _, k := range []string{"hostname", "ip", "mac"} {
		if v, ok := item[k]; ok {
			if s, ok := v.(string); ok && s == q {
				return true
			}
		}
	}
	return false
}

func extractMCPRawData(data json.RawMessage) json.RawMessage {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(data, &envelope) == nil && envelope.Data != nil {
		return envelope.Data
	}
	return data
}

func enrichTrafficItemsMCP(items []map[string]any) {
	for _, item := range items {
		for _, k := range []string{"download", "rx", "bytes_in"} {
			if v, ok := item[k]; ok {
				if f, ok := v.(float64); ok && f > 0 {
					item["download_hr"] = formatBytes(int64(f))
					break
				}
			}
		}
		for _, k := range []string{"upload", "tx", "bytes_out"} {
			if v, ok := item[k]; ok {
				if f, ok := v.(float64); ok && f > 0 {
					item["upload_hr"] = formatBytes(int64(f))
					break
				}
			}
		}
	}
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func handleWifiToggle(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	ssid, _ := args["ssid"].(string)
	enable, _ := args["enable"].(bool)
	if ssid == "" {
		return mcplib.NewToolResultError("ssid is required"), nil
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	wifiData, wifiErr := c.Get("/wifi/settings", nil)
	if wifiErr != nil {
		return mcplib.NewToolResultError(wifiErr.Error()), nil
	}

	var wifi map[string]any
	if json.Unmarshal(wifiData, &wifi) != nil {
		return mcplib.NewToolResultText(string(wifiData)), nil
	}

	profiles, _ := wifi["profiles"].([]any)
	if profiles == nil {
		if raw := wifi["data"]; raw != nil {
			if rawMap, ok := raw.(map[string]any); ok {
				profiles, _ = rawMap["profiles"].([]any)
			}
		}
	}

	changed := false
	for i, p := range profiles {
		if pm, ok := p.(map[string]any); ok {
			name, _ := pm["name"].(string)
			ssidVal, _ := pm["ssid"].(string)
			if name == ssid || ssidVal == ssid {
				pm["enable"] = enable
				profiles[i] = pm
				changed = true
				break
			}
		}
	}

	if !changed {
		return mcplib.NewToolResultError(fmt.Sprintf("SSID %q not found", ssid)), nil
	}

	wifi["profiles"] = profiles
	out, _ := json.Marshal(map[string]any{"profiles": profiles})
	_, _, putErr := c.PutWithParams("/wifi/settings", nil, map[string]any{"profiles": string(out)})
	if putErr != nil {
		return mcplib.NewToolResultError(putErr.Error()), nil
	}

	state := "disabled"
	if enable {
		state = "enabled"
	}
	return mcplib.NewToolResultText(fmt.Sprintf(`{"ssid":%q,"%s":true}`, ssid, state)), nil
}

func handleDeviceOnline(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	query, _ := args["query"].(string)
	if query == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	devData, devErr := c.Get("/devices", nil)
	if devErr != nil {
		return mcplib.NewToolResultError(devErr.Error()), nil
	}

	var items []map[string]any
	if raw := extractMCPRawData(devData); json.Unmarshal(raw, &items) != nil {
		return mcplib.NewToolResultError("failed to parse devices"), nil
	}

	for _, item := range items {
		if matchesDevice(item, query) {
			online := false
			if v, ok := item["is_online"].(bool); ok {
				online = v
			}
			out, _ := json.Marshal(map[string]any{"device": item, "online": online})
			return mcplib.NewToolResultText(string(out)), nil
		}
	}

	return mcplib.NewToolResultError(fmt.Sprintf("device %q not found", query)), nil
}

func handleMeshHealth(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	meshData, meshErr := c.Get("/mesh/nodes", nil)
	if meshErr != nil {
		return mcplib.NewToolResultError(meshErr.Error()), nil
	}

	var items []map[string]any
	if raw := extractMCPRawData(meshData); json.Unmarshal(raw, &items) != nil {
		return mcplib.NewToolResultText(string(meshData)), nil
	}

	nodes := make([]map[string]any, 0, len(items))
	totalDevices := 0
	onlineNodes := 0
	for _, n := range items {
		entry := map[string]any{
			"hostname": n["hostname"],
			"status":   n["status"],
		}
		if cd, ok := n["connected_devices"].(float64); ok {
			entry["connected_devices"] = int(cd)
			totalDevices += int(cd)
		}
		if s, ok := n["status"].(string); ok && (s == "online" || s == "connected") {
			onlineNodes++
		}
		nodes = append(nodes, entry)
	}

	out, _ := json.Marshal(map[string]any{
		"total_nodes":        len(nodes),
		"online_nodes":       onlineNodes,
		"total_connected_devices": totalDevices,
		"nodes":              nodes,
	})
	return mcplib.NewToolResultText(string(out)), nil
}

func handleDNSSummary(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	summary := map[string]any{}

	ipData, ipErr := c.Get("/dns/external-ip", nil)
	if ipErr == nil {
		summary["external_ip"] = json.RawMessage(ipData)
	}

	ddnsData, ddnsErr := c.Get("/dns/ddns", nil)
	if ddnsErr == nil {
		summary["ddns_records"] = json.RawMessage(ddnsData)
	}

	out, _ := json.MarshalIndent(summary, "", "  ")
	return mcplib.NewToolResultText(string(out)), nil
}

func handleRebootCheck(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	info := map[string]any{}

	meshData, meshErr := c.Get("/mesh/info", nil)
	if meshErr == nil {
		info["system_info"] = json.RawMessage(meshData)
	}

	utilData, utilErr := c.Get("/utilization", map[string]string{"resource": `["cpu","memory"]`})
	if utilErr == nil {
		info["utilization"] = json.RawMessage(utilData)
	}

	out, _ := json.MarshalIndent(info, "", "  ")
	return mcplib.NewToolResultText(string(out)), nil
}
