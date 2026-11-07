// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output. Manifest absorbed row 10.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/spf13/cobra"
)

type fleetFailure struct {
	ConsoleID string `json:"console_id,omitempty"`
	Source    string `json:"source"`
	Error     string `json:"error"`
}

type fleetDevice struct {
	ConsoleID       string `json:"console_id"`
	ConsoleName     string `json:"console_name,omitempty"`
	ID              string `json:"id,omitempty"`
	Name            string `json:"name,omitempty"`
	Model           string `json:"model,omitempty"`
	MAC             string `json:"mac,omitempty"`
	IP              string `json:"ip,omitempty"`
	Status          string `json:"status,omitempty"`
	Version         string `json:"version,omitempty"`
	FirmwareStatus  string `json:"firmware_status,omitempty"`
	UpdateAvailable string `json:"update_available,omitempty"`
}

type fleetConsole struct {
	ConsoleID       string `json:"console_id"`
	Name            string `json:"name,omitempty"`
	State           string `json:"state,omitempty"`
	Version         string `json:"version,omitempty"`
	IsBlocked       bool   `json:"is_blocked"`
	LastStateChange string `json:"last_state_change,omitempty"`
	// DevicesKnown is false when the device list could not be fetched, which is what
	// separates the zeros below from a console that really has no devices.
	DevicesKnown   bool `json:"devices_known"`
	DevicesTotal   int  `json:"devices_total"`
	DevicesOnline  int  `json:"devices_online"`
	DevicesOffline int  `json:"devices_offline"`
	UpdatesPending int  `json:"updates_pending"`
	// Connector is ok, failed or skipped. Failed consoles stay listed because a
	// console that cannot be reached is the most important row in a fleet view.
	Connector string `json:"connector"`
}

type fleetView struct {
	Report        string         `json:"report"`
	Consoles      []fleetConsole `json:"consoles,omitempty"`
	Inventory     []fleetDevice  `json:"inventory,omitempty"`
	Firmware      []fleetDevice  `json:"firmware,omitempty"`
	Totals        map[string]int `json:"totals"`
	Truncated     bool           `json:"truncated,omitempty"`
	FetchFailures []fleetFailure `json:"fetch_failures,omitempty"`
}

type fleetHost struct {
	id, name, state, version, lastChange string
	blocked                              bool
}

func parseFleetHosts(data json.RawMessage) ([]fleetHost, string, error) {
	var envelope struct {
		Data []struct {
			ID                        string         `json:"id"`
			IsBlocked                 bool           `json:"isBlocked"`
			LastConnectionStateChange string         `json:"lastConnectionStateChange"`
			ReportedState             map[string]any `json:"reportedState"`
		} `json:"data"`
		NextToken string `json:"nextToken"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, "", err
	}
	hosts := make([]fleetHost, 0, len(envelope.Data))
	for _, h := range envelope.Data {
		if h.ID == "" {
			continue
		}
		state := func(key string) string {
			if v, ok := h.ReportedState[key].(string); ok {
				return v
			}
			return ""
		}
		name := state("name")
		if name == "" {
			name = state("hostname")
		}
		hosts = append(hosts, fleetHost{id: h.ID, name: name, state: state("state"), version: state("version"), lastChange: h.LastConnectionStateChange, blocked: h.IsBlocked})
	}
	return hosts, envelope.NextToken, nil
}

func parseFleetDevices(data json.RawMessage) ([]fleetDevice, string, error) {
	var envelope struct {
		Data []struct {
			HostID   string `json:"hostId"`
			HostName string `json:"hostName"`
			Devices  []struct {
				ID              string `json:"id"`
				MAC             string `json:"mac"`
				Name            string `json:"name"`
				Model           string `json:"model"`
				IP              string `json:"ip"`
				Status          string `json:"status"`
				Version         string `json:"version"`
				FirmwareStatus  string `json:"firmwareStatus"`
				UpdateAvailable string `json:"updateAvailable"`
			} `json:"devices"`
		} `json:"data"`
		NextToken string `json:"nextToken"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, "", err
	}
	devices := make([]fleetDevice, 0)
	for _, group := range envelope.Data {
		for _, d := range group.Devices {
			devices = append(devices, fleetDevice{
				ConsoleID: group.HostID, ConsoleName: group.HostName, ID: d.ID, Name: d.Name, Model: d.Model, MAC: d.MAC, IP: d.IP,
				Status: d.Status, Version: d.Version, FirmwareStatus: d.FirmwareStatus, UpdateAvailable: d.UpdateAvailable,
			})
		}
	}
	return devices, envelope.NextToken, nil
}

// buildFleetView is pure so the aggregation rules are testable: a console whose
// device list is unknown must not count as "zero devices, all healthy".
func buildFleetView(report string, hosts []fleetHost, devices []fleetDevice, devicesKnown bool, connector map[string]string, failures []fleetFailure, limit int) fleetView {
	view := fleetView{Report: report, Totals: map[string]int{}, FetchFailures: failures}
	byConsole := map[string][]fleetDevice{}
	for _, d := range devices {
		byConsole[d.ConsoleID] = append(byConsole[d.ConsoleID], d)
	}
	if report == "all" || report == "health" {
		view.Consoles = make([]fleetConsole, 0, len(hosts))
	}
	for _, host := range hosts {
		row := fleetConsole{ConsoleID: host.id, Name: host.name, State: host.state, Version: host.version, IsBlocked: host.blocked, LastStateChange: host.lastChange, Connector: connector[host.id], DevicesKnown: devicesKnown}
		if row.Connector == "" {
			row.Connector = "skipped"
		}
		for _, d := range byConsole[host.id] {
			row.DevicesTotal++
			switch {
			case strings.EqualFold(d.Status, "online"):
				row.DevicesOnline++
			case strings.EqualFold(d.Status, "offline"):
				row.DevicesOffline++
			}
			if d.UpdateAvailable != "" {
				row.UpdatesPending++
			}
		}
		view.Totals["consoles"]++
		view.Totals["devices"] += row.DevicesTotal
		view.Totals["devices_online"] += row.DevicesOnline
		view.Totals["devices_offline"] += row.DevicesOffline
		view.Totals["updates_pending"] += row.UpdatesPending
		if row.Connector == "failed" {
			view.Totals["connector_failed"]++
		}
		if report == "all" || report == "health" {
			view.Consoles = append(view.Consoles, row)
		}
	}
	if !devicesKnown {
		for _, key := range []string{"devices", "devices_online", "devices_offline", "updates_pending"} {
			delete(view.Totals, key)
		}
	}
	known := map[string]bool{}
	for _, host := range hosts {
		known[host.id] = true
	}
	scoped := make([]fleetDevice, 0, len(devices))
	for _, d := range devices {
		if known[d.ConsoleID] {
			scoped = append(scoped, d)
		}
	}
	sort.SliceStable(scoped, func(i, j int) bool {
		if scoped[i].ConsoleID != scoped[j].ConsoleID {
			return scoped[i].ConsoleID < scoped[j].ConsoleID
		}
		return scoped[i].Name < scoped[j].Name
	})
	capped := func(rows []fleetDevice) []fleetDevice {
		if limit > 0 && len(rows) > limit {
			view.Truncated = true
			return rows[:limit]
		}
		return rows
	}
	if report == "all" || report == "inventory" {
		view.Inventory = capped(scoped)
		if view.Inventory == nil {
			view.Inventory = []fleetDevice{}
		}
	}
	if report == "all" || report == "firmware" {
		pending := make([]fleetDevice, 0)
		for _, d := range scoped {
			if d.UpdateAvailable != "" {
				pending = append(pending, d)
			}
		}
		view.Firmware = capped(pending)
	}
	return view
}

// maxFleetPages guards a server whose tokens never end; 50 pages of 100 is far beyond
// any fleet this CLI targets.
const maxFleetPages = 50

func fetchAllPages[T any](ctx context.Context, c *client.Client, path string, parse func(json.RawMessage) ([]T, string, error)) ([]T, error) {
	all := make([]T, 0)
	token := ""
	for page := 0; page < maxFleetPages; page++ {
		params := map[string]string{"pageSize": "100"}
		if token != "" {
			params["nextToken"] = token
		}
		data, err := c.GetNoCache(ctx, path, params)
		if err != nil {
			return nil, err
		}
		items, next, err := parse(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		all = append(all, items...)
		if next == "" {
			return all, nil
		}
		if next == token {
			return nil, fmt.Errorf("listing %s: the server repeated page token after %d pages, so the list is incomplete", path, page+1)
		}
		token = next
	}
	// An error, not the partial list: fleet's totals and "every console" framing would
	// present a truncated inventory as the whole fleet.
	return nil, fmt.Errorf("listing %s: still more pages after %d, so the list is incomplete", path, maxFleetPages)
}

func newFleetCmd(flags *rootFlags) *cobra.Command {
	var noProbe bool
	var limit, maxProbes int
	cmd := &cobra.Command{
		Use:   "fleet [health|inventory|firmware]",
		Short: "Health, inventory and firmware across every visible console, reporting partial failures",
		Long: `Reads hosts and devices from Site Manager, then probes each console through the
connector. A console that does not answer stays in the report with connector=failed
and is named under fetch_failures; it never counts as healthy. Bare 'fleet' prints
all three reports.`,
		Example: strings.Trim(`
  unifi-pp-cli fleet --agent
  unifi-pp-cli fleet health --no-probe --json
  unifi-pp-cli fleet firmware --json --select firmware.console_id,firmware.update_available`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fleet")
			}
			report := "all"
			if len(args) > 0 {
				report = strings.ToLower(args[0])
			}
			switch report {
			case "all", "health", "inventory", "firmware":
			default:
				return usageErr(fmt.Errorf("report %q must be health, inventory or firmware", args[0]))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			hosts, err := fetchAllPages(ctx, c, "/v1/hosts", parseFleetHosts)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if only := strings.TrimSpace(c.DescribeTransport().ConsoleID); only != "" {
				filtered := hosts[:0]
				for _, h := range hosts {
					if h.id == only {
						filtered = append(filtered, h)
					}
				}
				hosts = filtered
			}
			failures := make([]fleetFailure, 0)
			devices, devErr := fetchAllPages(ctx, c, "/v1/devices", parseFleetDevices)
			devicesKnown := devErr == nil
			if devErr != nil {
				failures = append(failures, fleetFailure{Source: "/v1/devices", Error: devErr.Error()})
			}
			connector := map[string]string{}
			if !noProbe && report != "inventory" && report != "firmware" {
				probeHosts := hosts
				if maxProbes > 0 && len(probeHosts) > maxProbes {
					probeHosts = probeHosts[:maxProbes]
				}
				if cliutil.IsDogfoodEnv() && len(probeHosts) > 2 {
					probeHosts = probeHosts[:2]
				}
				results, probeErrs := cliutil.FanoutRun(ctx, probeHosts,
					func(h fleetHost) string { return h.id },
					func(ctx context.Context, h fleetHost) (bool, error) {
						_, _, err := c.ProbeConnectorShape(ctx, h.id, "/network/integration/v1/info", client.ConnectorProxySegment())
						return err == nil, err
					}, cliutil.WithConcurrency(4))
				for _, r := range results {
					connector[r.Source] = "ok"
				}
				for _, e := range probeErrs {
					connector[e.Source] = "failed"
					failures = append(failures, fleetFailure{ConsoleID: e.Source, Source: "connector", Error: e.Err.Error()})
				}
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d console probes or lookups failed; see fetch_failures. Totals count only what answered.\n", len(failures), len(hosts))
			}
			view := buildFleetView(report, hosts, devices, devicesKnown, connector, failures, limit)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printFleetHuman(cmd, view)
		},
	}
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "Skip the per-console connector probe and use Site Manager data only")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum device rows in inventory and firmware reports; 0 for all")
	cmd.Flags().IntVar(&maxProbes, "max-probes", 50, "Maximum consoles to probe, since each probe spends connector budget")
	return cmd
}

func printFleetHuman(cmd *cobra.Command, view fleetView) error {
	out := cmd.OutOrStdout()
	if view.Consoles != nil {
		tw := newTabWriter(out)
		fmt.Fprintln(tw, "CONSOLE\tNAME\tSTATE\tVERSION\tDEVICES\tONLINE\tOFFLINE\tUPDATES\tCONNECTOR")
		for _, row := range view.Consoles {
			if !row.DevicesKnown {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t?\t?\t?\t?\t%s\n", row.ConsoleID, row.Name, row.State, row.Version, row.Connector)
				continue
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%s\n", row.ConsoleID, row.Name, row.State, row.Version, row.DevicesTotal, row.DevicesOnline, row.DevicesOffline, row.UpdatesPending, row.Connector)
		}
		_ = tw.Flush()
	}
	printDevices := func(title string, rows []fleetDevice) {
		if rows == nil {
			return
		}
		fmt.Fprintf(out, "\n%s (%d)\n", title, len(rows))
		tw := newTabWriter(out)
		fmt.Fprintln(tw, "CONSOLE\tNAME\tMODEL\tSTATUS\tVERSION\tUPDATE")
		for _, d := range rows {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", d.ConsoleID, d.Name, d.Model, d.Status, d.Version, d.UpdateAvailable)
		}
		_ = tw.Flush()
	}
	printDevices("Inventory", view.Inventory)
	printDevices("Firmware updates", view.Firmware)
	if len(view.FetchFailures) > 0 {
		fmt.Fprintf(out, "\npartial results: %d lookups failed\n", len(view.FetchFailures))
		for _, f := range view.FetchFailures {
			fmt.Fprintf(out, "  %s %s: %s\n", f.Source, f.ConsoleID, f.Error)
		}
	}
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newFleetCmd(flags))
	})
}
