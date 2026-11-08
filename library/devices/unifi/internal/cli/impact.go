// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. pp:data-source local.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

// There is deliberately no "safe" verdict. The mirror is a past snapshot and cannot
// see every cause of path loss, so the strongest claim available is "no known risk".
const (
	verdictBlocksPath      = "blocks-path"
	verdictAtRisk          = "at-risk"
	verdictNoKnownRisk     = "no-known-risk"
	verdictCannotDetermine = "cannot-determine"
)

const impactLimits = "This reasons from the last sync and cannot see every cause of path loss: ISP faults, an upstream device outside UniFi, or an unsynced change can all cut the path regardless of this verdict."

type impactAffected struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type impactControlPath struct {
	Transport string `json:"transport"`
	GatewayID string `json:"gateway_device_id,omitempty"`
	Gateway   string `json:"gateway_device,omitempty"`
	WANCount  int    `json:"wan_count"`
	Note      string `json:"note"`
}

type impactResult struct {
	ConsoleID      string            `json:"console_id,omitempty"`
	Kind           string            `json:"kind"`
	Action         string            `json:"action"`
	Target         string            `json:"target"`
	Verdict        string            `json:"verdict"`
	Reasons        []string          `json:"reasons"`
	Affected       []impactAffected  `json:"affected"`
	AffectedCounts map[string]int    `json:"affected_counts"`
	ControlPath    impactControlPath `json:"control_path"`
	MirrorTakenAt  string            `json:"mirror_snapshot_at,omitempty"`
	Limits         string            `json:"limits"`
}

// impactData is the local join input, passed in so the verdict logic is pure.
type impactData struct {
	Devices, Clients, Cameras, WANs, Wifi, Policies, Zones []mirrorRow
	Transport                                              string
}

func impactVerdictBase(kind, action, target string) impactResult {
	return impactResult{Kind: kind, Action: action, Target: target, Reasons: []string{}, Affected: []impactAffected{}, AffectedCounts: map[string]int{}, Limits: impactLimits}
}

func deviceHydrated(d mirrorRow) bool {
	if v, _ := d.Data[deviceHydratedField].(bool); v {
		return true
	}
	_, has := d.Data["uplink"]
	return has
}

// gatewayDevices returns hydrated devices with no uplink: the top of the tree, which
// is the device the connector's outbound path depends on.
func gatewayDevices(devices []mirrorRow) []mirrorRow {
	out := make([]mirrorRow, 0, 1)
	for _, d := range devices {
		if deviceHydrated(d) && nestedString(d.Data, "uplink", "deviceId") == "" {
			out = append(out, d)
		}
	}
	return out
}

// findRows matches id, then name or MAC. A name can repeat, and acting on a guess is
// worse than reporting that the target is ambiguous.
func findRows(rows []mirrorRow, ref string) []mirrorRow {
	matched := make([]mirrorRow, 0, 1)
	for _, r := range rows {
		if r.ID == ref {
			return []mirrorRow{r}
		}
	}
	for _, r := range rows {
		if strings.EqualFold(r.str("name"), ref) || (r.str("macAddress") != "" && strings.EqualFold(r.str("macAddress"), ref)) {
			matched = append(matched, r)
		}
	}
	return matched
}

func resolveImpactDevice(d impactData, ref string) ([]mirrorRow, bool) {
	if found := findRows(d.Devices, ref); len(found) > 0 {
		return found, false
	}
	if strings.EqualFold(ref, "gateway") {
		return gatewayDevices(d.Devices), true
	}
	return nil, false
}

func assessImpact(kind, action, target, portIdx string, d impactData) impactResult {
	res := impactVerdictBase(kind, action, target)
	res.ControlPath = buildControlPath(d)
	switch kind {
	case "device":
		assessDevice(&res, target, d)
	case "port":
		assessPort(&res, target, portIdx, d)
	case "client":
		assessClient(&res, target, d)
	case "wifi":
		assessWifi(&res, target, d)
	case "wan":
		assessWAN(&res, target, d)
	case "firewall-policy":
		assessPolicy(&res, target, d)
	}
	return res
}

func buildControlPath(d impactData) impactControlPath {
	path := impactControlPath{Transport: d.Transport, WANCount: len(d.WANs), Note: "The cloud connector runs outbound from the console through the gateway's WAN, so losing either loses the connector."}
	if d.Transport == string(client.TransportLocal) {
		path.Note = "Local access runs through the gateway and whatever switches and access points sit between this machine and it. The mirror does not know where this machine is attached."
	}
	if gateways := gatewayDevices(d.Devices); len(gateways) == 1 {
		path.GatewayID, path.Gateway = gateways[0].ID, gateways[0].str("name")
	}
	return path
}

func assessDevice(res *impactResult, ref string, d impactData) {
	targets, _ := resolveImpactDevice(d, ref)
	switch len(targets) {
	case 0:
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, fmt.Sprintf("device %q is not in the mirror; sync first or check the name", ref))
		return
	case 1:
	default:
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, fmt.Sprintf("%q matches %d devices; use a device id", ref, len(targets)))
		return
	}
	target := targets[0]
	// No client rows at all means clients were never synced, not that none exist, and
	// "nothing is known to depend on it" would then be ignorance presented as a finding.
	if len(d.Clients) == 0 && nestedString(target.Data, "uplink", "deviceId") != "" {
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, "the mirror holds no clients, so what depends on this device is unknown; run sync first")
		return
	}
	unhydrated := 0
	for _, device := range d.Devices {
		if !deviceHydrated(device) {
			unhydrated++
		}
	}
	if unhydrated > 0 {
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, fmt.Sprintf("%d devices have no uplink data, so what sits below this one is unknown; run sync without --no-hydrate", unhydrated))
		return
	}
	children := map[string][]mirrorRow{}
	for _, device := range d.Devices {
		if parent := nestedString(device.Data, "uplink", "deviceId"); parent != "" {
			children[parent] = append(children[parent], device)
		}
	}
	subtree := map[string]bool{target.ID: true}
	queue := []string{target.ID}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		for _, child := range children[next] {
			if !subtree[child.ID] {
				subtree[child.ID] = true
				queue = append(queue, child.ID)
				res.Affected = append(res.Affected, impactAffected{Kind: "device", ID: child.ID, Name: child.str("name")})
				res.AffectedCounts["devices"]++
			}
		}
	}
	macOwners := map[string]mirrorRow{}
	for _, c := range d.Clients {
		if subtree[c.str("uplinkDeviceId")] {
			res.Affected = append(res.Affected, impactAffected{Kind: "client", ID: c.ID, Name: c.str("name")})
			res.AffectedCounts["clients"]++
			if mac := strings.ToLower(c.str("macAddress")); mac != "" {
				macOwners[mac] = c
			}
		}
	}
	// Cameras are joined by MAC to the client that carries them: Protect does not say
	// which switch a camera hangs from. A camera absent from the client list is left
	// out of the count rather than assumed either way.
	for _, cam := range d.Cameras {
		if _, ok := macOwners[strings.ToLower(cam.str("mac"))]; ok {
			res.Affected = append(res.Affected, impactAffected{Kind: "camera", ID: cam.ID, Name: cam.str("name")})
			res.AffectedCounts["cameras"]++
		}
	}
	sort.SliceStable(res.Affected, func(i, j int) bool { return res.Affected[i].Kind < res.Affected[j].Kind })
	if nestedString(target.Data, "uplink", "deviceId") == "" {
		res.Verdict = verdictBlocksPath
		res.Reasons = append(res.Reasons, "this device has no uplink, so it is the gateway the control path runs through; the path drops until it returns")
		return
	}
	if len(res.Affected) > 0 {
		res.Verdict = verdictAtRisk
		res.Reasons = append(res.Reasons, fmt.Sprintf("%d devices, %d clients and %d cameras depend on it", res.AffectedCounts["devices"], res.AffectedCounts["clients"], res.AffectedCounts["cameras"]))
		if d.Transport == string(client.TransportLocal) {
			res.Reasons = append(res.Reasons, "over local access, this machine may be one of the things below it")
		}
		return
	}
	res.Verdict = verdictNoKnownRisk
	res.Reasons = append(res.Reasons, "nothing is known to depend on it, and the gateway is not affected")
}

// assessPort is always cannot-determine: Network v11 gives a device's parent but not
// the parent's port index, so what hangs off a given port is not knowable from it.
func assessPort(res *impactResult, ref, portIdx string, d impactData) {
	res.Verdict = verdictCannotDetermine
	res.Reasons = append(res.Reasons, "the API reports a device's parent but not which parent port it uses, so what is attached to a port cannot be determined; classic controller data has not been proven available")
	if portIdx == "" {
		res.Reasons = append(res.Reasons, "no port index given")
	}
	if targets, _ := resolveImpactDevice(d, ref); len(targets) == 1 {
		for _, c := range d.Clients {
			if c.str("uplinkDeviceId") == targets[0].ID {
				res.AffectedCounts["clients_on_this_device_any_port"]++
			}
		}
	}
}

func assessClient(res *impactResult, ref string, d impactData) {
	targets := findRows(d.Clients, ref)
	if len(targets) != 1 {
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, fmt.Sprintf("client %q matches %d mirror rows; it may have joined since the last sync, or use an id", ref, len(targets)))
		return
	}
	target := targets[0]
	res.Affected = append(res.Affected, impactAffected{Kind: "client", ID: target.ID, Name: target.str("name")})
	res.AffectedCounts["clients"] = 1
	res.Verdict = verdictNoKnownRisk
	res.Reasons = append(res.Reasons, "only this client is affected")
	for _, cam := range d.Cameras {
		if cam.str("mac") != "" && strings.EqualFold(cam.str("mac"), target.str("macAddress")) {
			res.Affected = append(res.Affected, impactAffected{Kind: "camera", ID: cam.ID, Name: cam.str("name")})
			res.AffectedCounts["cameras"]++
			res.Verdict = verdictAtRisk
			res.Reasons = append(res.Reasons, "this client is a Protect camera")
		}
	}
	switch strings.ToUpper(target.str("type")) {
	case "VPN", "TELEPORT":
		res.Verdict = verdictAtRisk
		res.Reasons = append(res.Reasons, "this is a remote-access client and may be your own session")
	}
	if d.Transport == string(client.TransportLocal) {
		res.Verdict = verdictAtRisk
		res.Reasons = append(res.Reasons, "over local access, this client may be the machine running this command")
	}
}

func assessWifi(res *impactResult, ref string, d impactData) {
	targets := findRows(d.Wifi, ref)
	if len(targets) != 1 {
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, fmt.Sprintf("WiFi broadcast %q matches %d mirror rows", ref, len(targets)))
		return
	}
	// See assessDevice: an empty client list is missing data, not an empty network.
	if len(d.Clients) == 0 {
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, "the mirror holds no clients, so who is on this broadcast is unknown; run sync first")
		return
	}
	site := targets[0].Parent
	for _, c := range d.Clients {
		if c.Parent == site && strings.EqualFold(c.str("type"), "WIRELESS") {
			res.AffectedCounts["wireless_clients_on_site_upper_bound"]++
		}
	}
	if res.AffectedCounts["wireless_clients_on_site_upper_bound"] > 0 {
		res.Verdict = verdictAtRisk
		res.Reasons = append(res.Reasons, "the client list does not say which SSID a client uses, so every wireless client on the site is an upper bound")
		return
	}
	res.Verdict = verdictNoKnownRisk
	res.Reasons = append(res.Reasons, "no wireless clients were on this site at the last sync")
}

func assessWAN(res *impactResult, ref string, d impactData) {
	targets := findRows(d.WANs, ref)
	if len(targets) != 1 {
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, fmt.Sprintf("WAN %q matches %d mirror rows", ref, len(targets)))
		return
	}
	siteWANs := 0
	for _, wan := range d.WANs {
		if wan.Parent == targets[0].Parent {
			siteWANs++
		}
	}
	res.AffectedCounts["wans_on_site"] = siteWANs
	if siteWANs <= 1 {
		res.Verdict = verdictBlocksPath
		res.Reasons = append(res.Reasons, "this is the only WAN on the site, and the connector leaves through it")
		return
	}
	res.Verdict = verdictAtRisk
	res.Reasons = append(res.Reasons, fmt.Sprintf("%d WANs are configured, but configured does not mean failover has been tested", siteWANs))
}

func assessPolicy(res *impactResult, ref string, d impactData) {
	targets := findRows(d.Policies, ref)
	if len(targets) != 1 {
		res.Verdict = verdictCannotDetermine
		res.Reasons = append(res.Reasons, fmt.Sprintf("firewall policy %q matches %d mirror rows", ref, len(targets)))
		return
	}
	policy := targets[0]
	zoneNames := map[string]string{}
	for _, zone := range d.Zones {
		if zone.Parent == policy.Parent {
			zoneNames[zone.ID] = zone.str("name")
		}
	}
	zones := []string{nestedString(policy.Data, "source", "zoneId"), nestedString(policy.Data, "destination", "zoneId")}
	for _, zoneID := range zones {
		name, known := zoneNames[zoneID]
		if !known {
			res.Verdict = verdictCannotDetermine
			res.Reasons = append(res.Reasons, fmt.Sprintf("zone %q is not in the mirror, so the policy's reach is unknown", zoneID))
			return
		}
		if strings.EqualFold(name, "External") || strings.EqualFold(name, "Gateway") {
			res.Verdict = verdictAtRisk
			res.Reasons = append(res.Reasons, fmt.Sprintf("it touches the %s zone, which the control path uses", name))
			return
		}
	}
	res.Verdict = verdictNoKnownRisk
	res.Reasons = append(res.Reasons, "it touches neither the External nor the Gateway zone")
}

func loadImpactData(ctx context.Context, db *store.Store, console, transport string) (impactData, error) {
	rows, err := loadMirrorRows(ctx, db, console, rtDevices, rtClients, rtCameras, rtWANs, rtWifi, rtFirewall, rtZones)
	if err != nil {
		return impactData{}, err
	}
	d := impactData{Transport: transport}
	for _, r := range rows {
		switch r.Type {
		case rtDevices:
			d.Devices = append(d.Devices, r)
		case rtClients:
			d.Clients = append(d.Clients, r)
		case rtCameras:
			d.Cameras = append(d.Cameras, r)
		case rtWANs:
			d.WANs = append(d.WANs, r)
		case rtWifi:
			d.Wifi = append(d.Wifi, r)
		case rtFirewall:
			d.Policies = append(d.Policies, r)
		case rtZones:
			d.Zones = append(d.Zones, r)
		}
	}
	return d, nil
}

var impactKinds = map[string]string{
	"device": "restart, remove", "port": "power-cycle", "client": "block, disconnect, reconnect",
	"wifi": "disable, delete, change", "wan": "disable, change", "firewall-policy": "enable, disable, change, delete",
}

func newNovelImpactCmd(flags *rootFlags) *cobra.Command {
	var dbFlag string
	cmd := &cobra.Command{
		Use:   "impact [kind] [action] [target] [port]",
		Short: "Estimate from the last sync what a proposed action would take down and whether it risks your way back in. It never reports safe.",
		Long: `Use this command to see what a proposed action would take down and whether it endangers
the control path. Do NOT use it to evaluate one packet against firewall policy; use
'rule-predict' instead. Do NOT use it to make the change; use 'change apply' instead.

Kinds: device, port, client, wifi, wan, firewall-policy. 'gateway' names the top device.
Verdicts: blocks-path, at-risk, no-known-risk, cannot-determine. No verdict means safe:
this reasons from the last sync and cannot see every cause of path loss.`,
		Example: strings.Trim(`
  unifi-pp-cli impact device restart gateway --agent
  unifi-pp-cli impact client block 02:00:00:00:00:01 --json
  unifi-pp-cli impact wan disable "Internet 1" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "kind=device;action=restart;target=gateway"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "impact")
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("expected <kind> <action> <target>, e.g. impact device restart gateway"))
			}
			kind, action, target := strings.ToLower(args[0]), strings.ToLower(args[1]), args[2]
			actions, ok := impactKinds[kind]
			if !ok {
				return usageErr(fmt.Errorf("kind %q must be device, port, client, wifi, wan or firewall-policy", args[0]))
			}
			if !strings.Contains(", "+actions+",", ", "+action+",") {
				return usageErr(fmt.Errorf("action %q for %s must be one of: %s", args[1], kind, actions))
			}
			portIdx := ""
			if len(args) > 3 {
				portIdx = args[3]
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := mirrorDBPath(dbFlag)
			if mirrorMissing(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync --console <id> --db %s\n", dbPath, dbPath)
				res := impactVerdictBase(kind, action, target)
				res.Verdict = verdictCannotDetermine
				res.Reasons = append(res.Reasons, "no local mirror; run sync first")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				return nil
			}
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			console, err := resolveMirrorConsole(ctx, db)
			if err != nil {
				return err
			}
			transport := string(client.TransportConnector)
			if c, err := flags.newClient(); err == nil {
				transport = c.DescribeTransport().Mode
			}
			data, err := loadImpactData(ctx, db, console, transport)
			if err != nil {
				return err
			}
			res := assessImpact(kind, action, target, portIdx, data)
			res.ConsoleID = console
			if snaps, err := db.ListSnapshots(ctx, console); err == nil && len(snaps) > 0 {
				res.MirrorTakenAt = snaps[len(snaps)-1].TakenAt
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "verdict: %s\n", res.Verdict)
			for _, reason := range res.Reasons {
				fmt.Fprintf(out, "  %s\n", reason)
			}
			for _, a := range res.Affected {
				fmt.Fprintf(out, "  affected %s: %s\n", a.Kind, firstNonEmpty(a.Name, a.ID))
			}
			fmt.Fprintf(out, "\n%s\n", res.Limits)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror path (default: the standard mirror)")
	return cmd
}
