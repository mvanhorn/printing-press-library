// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output: mirror access shared by the novel commands,
// plus the post-sync step that turns the mirror into history.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

const (
	rtSites        = "network-integration-v1-sites"
	rtDevices      = "network_integration_v1_sites_devices"
	rtClients      = "network_integration_v1_sites_clients"
	rtNetworks     = "network_integration_v1_sites_networks"
	rtFirewall     = "network_integration_v1_sites_firewall_policies"
	rtZones        = "network_integration_v1_sites_firewall_zones"
	rtWifi         = "network_integration_v1_sites_wifi_broadcasts"
	rtDNS          = "network_integration_v1_sites_dns_policies"
	rtACL          = "network_integration_v1_sites_acl_rules"
	rtTML          = "network_integration_v1_sites_traffic_matching_lists"
	rtWANs         = "network_integration_v1_sites_wans"
	rtVPNServers   = "network_integration_v1_sites_vpn_servers"
	rtCameras      = "protect-integration-v1-cameras"
	rtHosts        = "hosts"
	rtCloudDevices = "cloud-devices"
	rtISPSamples   = "isp-samples"
)

// snapshotResourceTypes is what a snapshot copies. Config types feed drift; the
// state types feed digest and newcomer.
var snapshotResourceTypes = []string{rtNetworks, rtFirewall, rtZones, rtWifi, rtDNS, rtACL, rtTML, rtWANs, rtVPNServers, rtDevices, rtClients, rtCameras}

var driftResourceTypes = map[string]string{
	rtNetworks: "network", rtFirewall: "firewall-policy", rtZones: "firewall-zone",
	rtWifi: "wifi", rtDNS: "dns-policy", rtACL: "acl-rule", rtTML: "traffic-matching-list",
}

const deviceHydratedField = "_detail_fetched"

// mirrorRow is one decoded mirror row with its key parts split out.
type mirrorRow struct {
	Type    string
	ID      string
	Parent  string
	Console string
	Data    map[string]any
}

func (r mirrorRow) str(key string) string {
	if v, ok := r.Data[key].(string); ok {
		return v
	}
	return ""
}

func nestedString(obj map[string]any, keys ...string) string {
	var cur any = obj
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[key]
	}
	s, _ := cur.(string)
	return s
}

func splitStorageID(storageID string) (bare, parent, console string) {
	console = store.ConsoleOfStorageID(storageID)
	rest := store.StripConsoleKey(storageID)
	bare = store.BareResourceID(rest)
	if i := strings.IndexByte(rest, 0); i >= 0 {
		parent = rest[i+1:]
	}
	return bare, parent, console
}

// loadMirrorRows drains fully before returning, so callers may run follow-up
// queries on SQLite's single connection. Console filtering happens in Go because
// the composite key contains a NUL that SQLite string functions truncate at.
func loadMirrorRows(ctx context.Context, db *store.Store, console string, resourceTypes ...string) ([]mirrorRow, error) {
	if len(resourceTypes) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(resourceTypes)), ",")
	args := make([]any, 0, len(resourceTypes))
	for _, rt := range resourceTypes {
		args = append(args, rt)
	}
	rows, err := db.DB().QueryContext(ctx, `SELECT resource_type, id, data FROM resources WHERE resource_type IN (`+placeholders+`) ORDER BY resource_type, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("reading mirror: %w", err)
	}
	type rawRow struct{ rt, id, data string }
	raw := make([]rawRow, 0)
	for rows.Next() {
		var r rawRow
		if err := rows.Scan(&r.rt, &r.id, &r.data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning mirror: %w", err)
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating mirror: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	out := make([]mirrorRow, 0, len(raw))
	for _, r := range raw {
		bare, parent, rowConsole := splitStorageID(r.id)
		if console != "" && rowConsole != console {
			continue
		}
		var data map[string]any
		if json.Unmarshal([]byte(r.data), &data) != nil {
			continue
		}
		out = append(out, mirrorRow{Type: r.rt, ID: bare, Parent: parent, Console: rowConsole, Data: data})
	}
	return out, nil
}

func mirrorConsoles(ctx context.Context, db *store.Store) ([]string, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT DISTINCT console_id FROM unifi_snapshots ORDER BY console_id`)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, c := range out {
		seen[c] = true
	}
	mirrored, err := loadMirrorRows(ctx, db, "", rtSites)
	if err != nil {
		return nil, err
	}
	for _, r := range mirrored {
		if !seen[r.Console] {
			seen[r.Console] = true
			out = append(out, r.Console)
		}
	}
	sort.Strings(out)
	if len(out) > 1 && out[0] == "" {
		out = out[1:]
	}
	return out, nil
}

// resolveMirrorConsole picks the console for commands that reason about exactly
// one. With several in the mirror it asks rather than guessing, since a verdict
// about the wrong console would read as authoritative.
func resolveMirrorConsole(ctx context.Context, db *store.Store) (string, error) {
	if active := store.ActiveConsole(); active != "" {
		return active, nil
	}
	consoles, err := mirrorConsoles(ctx, db)
	if err != nil {
		return "", err
	}
	if len(consoles) > 1 {
		return "", usageErr(fmt.Errorf("the mirror holds %d consoles, choose one with --console: %s", len(consoles), strings.Join(consoles, ", ")))
	}
	if len(consoles) == 1 {
		return consoles[0], nil
	}
	return "", nil
}

func mirrorDBPath(dbFlag string) string {
	if dbFlag != "" {
		return dbFlag
	}
	return defaultDBPath("unifi-pp-cli")
}

func mirrorMissing(dbPath string) bool {
	_, err := os.Stat(dbPath)
	return os.IsNotExist(err)
}

func openMirror(ctx context.Context, dbPath string) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local mirror: %w", err)
	}
	return db, nil
}

func snapshotEntityID(r mirrorRow) string {
	if r.Parent != "" {
		return r.Parent + "/" + r.ID
	}
	return r.ID
}

func firstSeenTracked(resourceType string) bool {
	return resourceType == rtClients || resourceType == rtDevices || resourceType == rtCameras
}

func firstSeenFieldsFor(r mirrorRow) store.FirstSeenFields {
	mac := r.str("macAddress")
	if mac == "" {
		mac = r.str("mac")
	}
	return store.FirstSeenFields{Label: r.str("name"), MAC: mac, IP: r.str("ipAddress")}
}

// captureSnapshot copies one console's mirror state into history.
func captureSnapshot(ctx context.Context, db *store.Store, console string, now time.Time) (int64, error) {
	rows, err := loadMirrorRows(ctx, db, console, snapshotResourceTypes...)
	if err != nil {
		return 0, err
	}
	snapshot := make([]store.SnapshotRow, 0, len(rows))
	fields := make(map[string]store.FirstSeenFields, len(rows))
	for _, r := range rows {
		// An unscoped sync must not sweep other consoles' rows into its snapshot.
		if r.Console != console {
			continue
		}
		delete(r.Data, store.ConsoleIDField)
		data, err := json.Marshal(r.Data)
		if err != nil {
			continue
		}
		entity := snapshotEntityID(r)
		snapshot = append(snapshot, store.SnapshotRow{ResourceType: r.Type, EntityID: entity, Data: data})
		fields[r.Type+"|"+entity] = firstSeenFieldsFor(r)
	}
	return db.SaveSnapshot(ctx, console, now, snapshot, fields, firstSeenTracked)
}

// hydrateDeviceDetails fills what the list endpoint leaves out. uplink.deviceId
// exists only on GET device, and topology and impact are built on it. It spends
// one connector request per device, so it is capped and reports what it skipped.
func hydrateDeviceDetails(ctx context.Context, c *client.Client, db *store.Store, console string, limit int, warn io.Writer) (hydrated, skipped int) {
	rows, err := loadMirrorRows(ctx, db, console, rtDevices)
	if err != nil {
		fmt.Fprintf(warn, "warning: device hydration skipped: %v\n", err)
		return 0, 0
	}
	items := make([]json.RawMessage, 0, len(rows))
	failures := 0
	for _, r := range rows {
		if limit > 0 && hydrated+failures >= limit {
			skipped++
			continue
		}
		site := r.Parent
		if site == "" {
			site = r.str("parent_id")
		}
		data, err := c.GetNoCache(ctx, "/network/integration/v1/sites/"+site+"/devices/"+r.ID, nil)
		if err != nil {
			failures++
			continue
		}
		var detail map[string]any
		if json.Unmarshal(data, &detail) != nil {
			failures++
			continue
		}
		detail["parent_id"] = site
		// Marks that the detail endpoint answered. A gateway legitimately has no
		// uplink, so the field's absence alone cannot separate "root" from "not fetched".
		detail[deviceHydratedField] = true
		merged, err := json.Marshal(detail)
		if err != nil {
			failures++
			continue
		}
		items = append(items, merged)
		hydrated++
	}
	if len(items) > 0 {
		if _, _, err := db.UpsertBatch(rtDevices, items); err != nil {
			fmt.Fprintf(warn, "warning: storing device details failed: %v\n", err)
			return 0, skipped
		}
	}
	if failures > 0 || skipped > 0 {
		fmt.Fprintf(warn, "warning: device details: %d fetched, %d failed, %d skipped over the cap; topology and impact report those devices as unknown\n", hydrated, failures, skipped)
	}
	return hydrated, skipped
}

// syncCloudExtras stores Site Manager device groups and ISP samples, which the
// generated sync cannot: /v1/devices nests devices under hosts with no stable row
// id, and isp-metrics needs a type path parameter.
func syncCloudExtras(ctx context.Context, c *client.Client, db *store.Store, warn io.Writer) {
	devices, err := fetchAllPages(ctx, c, "/v1/devices", parseFleetDevices)
	if err != nil {
		fmt.Fprintf(warn, "warning: Site Manager devices not synced: %v\n", err)
	} else {
		items := make([]json.RawMessage, 0, len(devices))
		for _, d := range devices {
			raw, err := json.Marshal(map[string]any{"id": d.ConsoleID + "/" + firstNonEmpty(d.ID, d.MAC, d.Name), "hostId": d.ConsoleID, "hostName": d.ConsoleName, "name": d.Name, "model": d.Model, "mac": d.MAC, "ip": d.IP, "status": d.Status, "version": d.Version, "updateAvailable": d.UpdateAvailable})
			if err == nil {
				items = append(items, raw)
			}
		}
		if len(items) > 0 {
			if _, _, err := db.UpsertBatch(rtCloudDevices, items); err != nil {
				fmt.Fprintf(warn, "warning: storing Site Manager devices failed: %v\n", err)
			}
		}
	}
	data, err := c.GetNoCache(ctx, "/v1/isp-metrics/5m", map[string]string{"duration": "24h"})
	if err != nil {
		fmt.Fprintf(warn, "warning: ISP metrics not synced: %v\n", err)
		return
	}
	samples := parseISPSamples(data)
	if len(samples) > 0 {
		if _, _, err := db.UpsertBatch(rtISPSamples, samples); err != nil {
			fmt.Fprintf(warn, "warning: storing ISP metrics failed: %v\n", err)
		}
	}
}

func parseISPSamples(data json.RawMessage) []json.RawMessage {
	var envelope struct {
		Data []struct {
			MetricType string `json:"metricType"`
			HostID     string `json:"hostId"`
			SiteID     string `json:"siteId"`
			Periods    []struct {
				Data       map[string]any `json:"data"`
				MetricTime string         `json:"metricTime"`
			} `json:"periods"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return nil
	}
	out := make([]json.RawMessage, 0)
	for _, group := range envelope.Data {
		for _, period := range group.Periods {
			if period.MetricTime == "" {
				continue
			}
			raw, err := json.Marshal(map[string]any{"id": group.HostID + "/" + group.SiteID + "/" + group.MetricType + "/" + period.MetricTime, "hostId": group.HostID, "siteId": group.SiteID, "metricType": group.MetricType, "metricTime": period.MetricTime, "wan": period.Data["wan"]})
			if err == nil {
				out = append(out, raw)
			}
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// postSync runs after a successful sync. A failed sync takes no snapshot: rows it
// did not reach would look deleted to drift, which is a wrong answer rather than a
// stale one. These steps are best effort and never change sync's exit code.
func postSync(cmd *cobra.Command, flags *rootFlags, dbFlag string, noHydrate bool, maxHydrate int) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	warn := cmd.ErrOrStderr()
	dbPath := mirrorDBPath(dbFlag)
	if mirrorMissing(dbPath) {
		return
	}
	db, err := openMirror(ctx, dbPath)
	if err != nil {
		fmt.Fprintf(warn, "warning: snapshot skipped: %v\n", err)
		return
	}
	defer db.Close()
	c, err := flags.newClient()
	if err != nil {
		fmt.Fprintf(warn, "warning: post-sync steps skipped: %v\n", err)
		return
	}
	console := store.ActiveConsole()
	if cliutil.IsDogfoodEnv() && maxHydrate > 2 {
		maxHydrate = 2
	}
	if !noHydrate && !cliutil.IsVerifyEnv() && console != "" {
		hydrateDeviceDetails(ctx, c, db, console, maxHydrate, warn)
	}
	if !cliutil.IsVerifyEnv() {
		syncCloudExtras(ctx, c, db, warn)
	}
	if _, err := captureSnapshot(ctx, db, console, time.Now()); err != nil {
		fmt.Fprintf(warn, "warning: snapshot failed: %v\n", err)
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		syncCmd, _, err := root.Find([]string{"sync"})
		if err != nil || syncCmd == nil || syncCmd.Name() != "sync" {
			return
		}
		var noHydrate bool
		var maxHydrate int
		syncCmd.Flags().BoolVar(&noHydrate, "no-hydrate", false, "Skip per-device detail requests; topology and impact then lack uplink data")
		syncCmd.Flags().IntVar(&maxHydrate, "max-hydrate", 100, "Most device details to fetch per sync, since each spends connector budget; 0 for no cap")
		generated := syncCmd.RunE
		syncCmd.RunE = func(cmd *cobra.Command, args []string) error {
			err := generated(cmd, args)
			if err != nil || flags.dryRun {
				return err
			}
			dbFlag, _ := cmd.Flags().GetString("db")
			postSync(cmd, flags, dbFlag, noHydrate, maxHydrate)
			return nil
		}
	})
}

// Journal states. drift, reach and digest read these, so they live with the mirror
// helpers rather than with the apply command.
const (
	journalPending  = "pending"
	journalVerified = "verified"
	journalMismatch = "mismatch"
	journalFailed   = "failed"
	// journalUnresolved means the true state is unknown or the change could not be
	// reverted. It always pairs with a non-zero exit.
	journalUnresolved = "unresolved"
	journalReverted   = "reverted"
	journalUndone     = "undone"
)
