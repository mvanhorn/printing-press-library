// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. pp:data-source local.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

type topologyClient struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	MAC  string `json:"mac,omitempty"`
	IP   string `json:"ip,omitempty"`
	Type string `json:"type,omitempty"`
}

type topologyNode struct {
	ID          string           `json:"id"`
	Name        string           `json:"name,omitempty"`
	Model       string           `json:"model,omitempty"`
	MAC         string           `json:"mac,omitempty"`
	IP          string           `json:"ip,omitempty"`
	State       string           `json:"state,omitempty"`
	UplinkKnown bool             `json:"uplink_known"`
	ClientCount int              `json:"client_count"`
	Clients     []topologyClient `json:"clients,omitempty"`
	Children    []*topologyNode  `json:"children,omitempty"`
}

type topologySite struct {
	SiteID string          `json:"site_id"`
	Roots  []*topologyNode `json:"roots"`
	// UnparentedClients have an uplink device the mirror does not hold, usually a
	// client moved since the last sync. They are listed so counts still add up.
	UnparentedClients int `json:"unparented_clients,omitempty"`
}

type topologyConsole struct {
	ConsoleID string         `json:"console_id"`
	Sites     []topologySite `json:"sites"`
}

type topologyView struct {
	Consoles []topologyConsole `json:"consoles"`
	Note     string            `json:"note,omitempty"`
}

// buildTopologySite turns uplink links into a tree. Three things in real data
// would otherwise break it: a parent missing from the mirror (stale row), a cycle
// (mid-sync snapshot), and a device with no uplink field at all. The list endpoint
// omits uplink, so "no uplink" can mean "not hydrated" as well as "this is the root";
// callers use uplink_known and the gateway heuristic to tell the two apart.
func buildTopologySite(siteID string, devices, clients []mirrorRow, withClients bool) topologySite {
	nodes := make(map[string]*topologyNode, len(devices))
	parents := make(map[string]string, len(devices))
	order := make([]string, 0, len(devices))
	for _, d := range devices {
		uplink := nestedString(d.Data, "uplink", "deviceId")
		_, hasUplinkField := d.Data["uplink"]
		if hydrated, _ := d.Data[deviceHydratedField].(bool); hydrated {
			hasUplinkField = true
		}
		nodes[d.ID] = &topologyNode{ID: d.ID, Name: d.str("name"), Model: d.str("model"), MAC: d.str("macAddress"), IP: d.str("ipAddress"), State: d.str("state"), UplinkKnown: hasUplinkField}
		parents[d.ID] = uplink
		order = append(order, d.ID)
	}
	sort.Strings(order)
	site := topologySite{SiteID: siteID, Roots: make([]*topologyNode, 0)}
	for _, c := range clients {
		node, ok := nodes[c.str("uplinkDeviceId")]
		if !ok {
			if c.str("uplinkDeviceId") != "" {
				site.UnparentedClients++
			}
			continue
		}
		node.ClientCount++
		if withClients {
			node.Clients = append(node.Clients, topologyClient{ID: c.ID, Name: c.str("name"), MAC: c.str("macAddress"), IP: c.str("ipAddress"), Type: c.str("type")})
		}
	}
	inCycle := func(id string) bool {
		seen := map[string]bool{id: true}
		for cur := parents[id]; cur != ""; cur = parents[cur] {
			if seen[cur] {
				return true
			}
			seen[cur] = true
		}
		return false
	}
	for _, id := range order {
		parent := parents[id]
		if _, known := nodes[parent]; known && parent != id && !inCycle(id) {
			nodes[parent].Children = append(nodes[parent].Children, nodes[id])
			continue
		}
		site.Roots = append(site.Roots, nodes[id])
	}
	return site
}

func buildTopology(console string, devices, clients []mirrorRow, withClients bool) []topologyConsole {
	type key struct{ console, site string }
	devBy := map[key][]mirrorRow{}
	cliBy := map[key][]mirrorRow{}
	seen := map[key]bool{}
	for _, d := range devices {
		k := key{d.Console, d.Parent}
		devBy[k] = append(devBy[k], d)
		seen[k] = true
	}
	for _, c := range clients {
		k := key{c.Console, c.Parent}
		cliBy[k] = append(cliBy[k], c)
	}
	byConsole := map[string][]topologySite{}
	for k := range seen {
		byConsole[k.console] = append(byConsole[k.console], buildTopologySite(k.site, devBy[k], cliBy[k], withClients))
	}
	out := make([]topologyConsole, 0, len(byConsole))
	for id, sites := range byConsole {
		sort.Slice(sites, func(i, j int) bool { return sites[i].SiteID < sites[j].SiteID })
		out = append(out, topologyConsole{ConsoleID: id, Sites: sites})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ConsoleID < out[j].ConsoleID })
	return out
}

func topologyHasKnownUplinks(view []topologyConsole) bool {
	var walk func(nodes []*topologyNode) bool
	walk = func(nodes []*topologyNode) bool {
		for _, n := range nodes {
			if n.UplinkKnown || walk(n.Children) {
				return true
			}
		}
		return false
	}
	for _, c := range view {
		for _, s := range c.Sites {
			if walk(s.Roots) {
				return true
			}
		}
	}
	return false
}

func newNovelTopologyCmd(flags *rootFlags) *cobra.Command {
	var dbFlag string
	var withClients bool
	cmd := &cobra.Command{
		Use:   "topology",
		Short: "See gateway, switches and APs as a tree per console, from the last sync.",
		Long: `Use this command to see the current device layout. Do NOT use it to work out what a
change would affect; use 'impact' instead.

Computed from the local mirror using each device's uplink and each client's uplink
device. Parent links come from per-device detail requests made during sync, and the
API does not report which port a device hangs from.`,
		Example: strings.Trim(`
  unifi-pp-cli topology --agent
  unifi-pp-cli topology --clients --console EXAMPLE-CONSOLE-ID --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "topology")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := mirrorDBPath(dbFlag)
			if mirrorMissing(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync --console <id> --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), topologyView{Consoles: []topologyConsole{}, Note: "no local mirror; run sync first"}, flags)
				}
				return nil
			}
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			view, err := loadTopologyView(ctx, db, store.ActiveConsole(), withClients)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Consoles) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No devices in the mirror yet. Run sync first.")
				return nil
			}
			var printNode func(n *topologyNode, indent string)
			printNode = func(n *topologyNode, indent string) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s (%s) %s, %d clients\n", indent, firstNonEmpty(n.Name, n.ID), n.Model, n.State, n.ClientCount)
				for _, c := range n.Clients {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  - %s %s\n", indent, firstNonEmpty(c.Name, c.ID), c.IP)
				}
				for _, child := range n.Children {
					printNode(child, indent+"  ")
				}
			}
			for _, c := range view.Consoles {
				fmt.Fprintf(cmd.OutOrStdout(), "console %s\n", c.ConsoleID)
				for _, s := range c.Sites {
					fmt.Fprintf(cmd.OutOrStdout(), " site %s\n", s.SiteID)
					for _, root := range s.Roots {
						printNode(root, "  ")
					}
				}
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), "\nnote: "+view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror path (default: the standard mirror)")
	cmd.Flags().BoolVar(&withClients, "clients", false, "List clients under each device instead of only counting them")
	return cmd
}

func loadTopologyView(ctx context.Context, db *store.Store, console string, withClients bool) (topologyView, error) {
	devices, err := loadMirrorRows(ctx, db, console, rtDevices)
	if err != nil {
		return topologyView{}, err
	}
	clients, err := loadMirrorRows(ctx, db, console, rtClients)
	if err != nil {
		return topologyView{}, err
	}
	view := topologyView{Consoles: buildTopology(console, devices, clients, withClients)}
	// An empty tree is not a finding about the network, so it says what would fill it.
	if len(view.Consoles) == 0 {
		view.Note = "no devices in the mirror; run sync first"
	}
	if len(view.Consoles) > 0 && !topologyHasKnownUplinks(view.Consoles) {
		view.Note = "no device has uplink data, so every device shows as a root. Run sync without --no-hydrate; uplink comes only from per-device detail requests."
	}
	return view, nil
}
