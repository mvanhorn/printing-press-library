// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. pp:data-source live.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

const zeroUUID = "00000000-0000-0000-0000-000000000000"

type reachSurface struct {
	Status string `json:"status"`
	HTTP   int    `json:"http,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type reachTransport struct {
	Available bool                    `json:"available"`
	Skipped   string                  `json:"skipped,omitempty"`
	PathShape string                  `json:"path_shape,omitempty"`
	Surfaces  map[string]reachSurface `json:"surfaces,omitempty"`
	Write     string                  `json:"write_status"`
	WriteWhy  string                  `json:"write_evidence,omitempty"`
}

type reachSecondPath struct {
	WANCount      *int     `json:"wan_count,omitempty"`
	VPNServers    []string `json:"vpn_servers,omitempty"`
	SiteID        string   `json:"site_id,omitempty"`
	Note          string   `json:"note"`
	FetchFailures []string `json:"fetch_failures,omitempty"`
}

type reachConsole struct {
	ConsoleID  string                    `json:"console_id"`
	Name       string                    `json:"name,omitempty"`
	Transports map[string]reachTransport `json:"transports"`
	SecondPath *reachSecondPath          `json:"second_path,omitempty"`
}

type reachView struct {
	SiteManager reachSurface   `json:"site_manager"`
	Consoles    []reachConsole `json:"consoles"`
	Notes       []string       `json:"notes"`
	Failures    []fleetFailure `json:"fetch_failures,omitempty"`
}

var reachNotes = []string{
	"write_status is yes only when the local journal shows a verified write on this console over this transport. A probe alone never yields yes: a 404 may come from the proxy before auth.",
	"Configured VPN servers and WANs are facts about config. Configured does not mean tested.",
}

func classifyReachError(err error, status int) reachSurface {
	if err == nil {
		return reachSurface{Status: "ok", HTTP: status}
	}
	var apiError *client.APIError
	if errors.As(err, &apiError) {
		status = apiError.StatusCode
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "returned HTML instead of JSON"):
		return reachSurface{Status: "console-page", HTTP: status, Detail: "the console answered with an HTML page, so this path missed the API"}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return reachSurface{Status: "auth-refused", HTTP: status}
	case status == http.StatusNotFound:
		return reachSurface{Status: "not-found", HTTP: status}
	case status == http.StatusRequestTimeout:
		return reachSurface{Status: "timeout", HTTP: status, Detail: "connector 25 second limit"}
	case errors.Is(err, client.ErrLocalKeyMissing):
		return reachSurface{Status: "no-local-key", Detail: "set UNIFI_LOCAL_API_KEY"}
	case status == 0:
		return reachSurface{Status: "unreachable", Detail: truncate(msg, 160)}
	default:
		return reachSurface{Status: "error", HTTP: status, Detail: truncate(msg, 160)}
	}
}

// surface probes, per console. The classic path is unofficial and its site name is
// a guess, so not-found there says little; the rest are documented endpoints.
var reachSurfacePaths = []struct{ name, path string }{
	{"network", "/network/integration/v1/info"},
	{"protect", "/protect/integration/v1/meta/info"},
	{"innerspace", "/innerspace/integration/v1/project"},
	{"controller_classic", "/proxy/network/api/s/default/stat/health"},
}

// writeVerdictFromProbe never returns "yes". 401, 403 and 405 are definite refusals.
// Everything else is unproven, including 404 and 2xx, because neither shows that a
// real write would be authorized.
func writeVerdictFromProbe(status int, err error) (string, string) {
	var apiError *client.APIError
	if errors.As(err, &apiError) {
		status = apiError.StatusCode
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusMethodNotAllowed:
		return "no", fmt.Sprintf("a mutating verb returned HTTP %d", status)
	case 0:
		return "unproven", "probe did not get an HTTP answer"
	default:
		return "unproven", fmt.Sprintf("a mutating verb against an all-zero id returned HTTP %d, which does not prove a real write is allowed", status)
	}
}

// writeStatusFromJournal is the only route to "yes": a write that happened and
// read back correctly on this console over this transport.
func writeStatusFromJournal(entries []store.JournalEntry, console, transport string) (string, string) {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.ConsoleID == console && e.Transport == transport && e.State == journalVerified {
			return "yes", fmt.Sprintf("journal entry %s verified a write at %s", e.ID, e.UpdatedAt)
		}
	}
	return "", ""
}

func firstSiteID(data json.RawMessage) string {
	var page struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &page) != nil || len(page.Data) == 0 {
		return ""
	}
	return page.Data[0].ID
}

func countAndNames(data json.RawMessage) (int, []string, bool) {
	var page struct {
		TotalCount *int `json:"totalCount"`
		Data       []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &page) != nil {
		return 0, nil, false
	}
	names := make([]string, 0, len(page.Data))
	for _, item := range page.Data {
		names = append(names, firstNonEmpty(item.Name, item.Type))
	}
	count := len(page.Data)
	if page.TotalCount != nil {
		count = *page.TotalCount
	}
	return count, names, true
}

// probeConnector tries the current shape, then the other if the first gets 404 on
// Network. The spec contradicts itself, and this is how the live answer is learned.
func probeConnector(ctx context.Context, c *client.Client, console string, probeWrites bool) reachTransport {
	t := reachTransport{Surfaces: map[string]reachSurface{}, Write: "unproven", WriteWhy: "not probed; pass --probe-writes"}
	segments := []string{client.ConnectorProxySegment(), client.AlternateConnectorProxySegment()}
	working := ""
	shapeFound := false
	for _, segment := range segments {
		_, status, err := c.ProbeConnectorShape(ctx, console, "/network/integration/v1/info", segment)
		surface := classifyReachError(err, status)
		if surface.Status == "ok" {
			working, shapeFound = segment, true
			t.Surfaces["network"] = surface
			break
		}
		t.Surfaces["network"] = surface
		if surface.Status != "not-found" && surface.Status != "console-page" {
			break
		}
	}
	if shapeFound {
		t.PathShape = "/v1/connector/consoles/{id}" + working + "/<product>/integration"
		t.Available = true
	} else {
		t.PathShape = "unknown"
		working = client.ConnectorProxySegment()
	}
	for _, s := range reachSurfacePaths {
		if s.name == "network" {
			continue
		}
		_, status, err := c.ProbeConnectorShape(ctx, console, s.path, working)
		surface := classifyReachError(err, status)
		t.Surfaces[s.name] = surface
		if surface.Status == "ok" && s.name != "controller_classic" {
			t.Available = true
		}
	}
	if probeWrites && t.Available {
		sites, _, err := c.ProbeConnectorShape(ctx, console, "/network/integration/v1/sites", working)
		site := firstSiteID(sites)
		if err != nil || site == "" {
			t.WriteWhy = "could not read a site to aim the probe at"
			return t
		}
		status, err := c.ProbeWrite(ctx, http.MethodDelete, client.ConnectorPathFor(console, "/network/integration/v1/sites/"+site+"/acl-rules/"+zeroUUID, working), nil)
		t.Write, t.WriteWhy = writeVerdictFromProbe(status, err)
	}
	return t
}

func probeLocal(ctx context.Context, local *client.Client, probeWrites bool) reachTransport {
	t := reachTransport{Surfaces: map[string]reachSurface{}, Write: "unproven", WriteWhy: "not probed; pass --probe-writes"}
	for _, s := range reachSurfacePaths {
		_, status, err := local.Do(ctx, http.MethodGet, s.path, nil, nil)
		surface := classifyReachError(err, status)
		t.Surfaces[s.name] = surface
		if surface.Status == "ok" && s.name != "controller_classic" {
			t.Available = true
		}
	}
	if probeWrites && t.Available {
		sites, _, err := local.Do(ctx, http.MethodGet, "/network/integration/v1/sites", nil, nil)
		site := firstSiteID(sites)
		if err != nil || site == "" {
			t.WriteWhy = "could not read a site to aim the probe at"
			return t
		}
		status, err := local.ProbeWrite(ctx, http.MethodDelete, "/network/integration/v1/sites/"+site+"/acl-rules/"+zeroUUID, nil)
		t.Write, t.WriteWhy = writeVerdictFromProbe(status, err)
	}
	return t
}

func readSecondPath(ctx context.Context, get func(string) (json.RawMessage, error)) *reachSecondPath {
	out := &reachSecondPath{Note: "Configured does not mean tested: these show what exists, not that it would carry traffic if the connector failed."}
	sites, err := get("/network/integration/v1/sites")
	if err != nil {
		out.FetchFailures = append(out.FetchFailures, "sites: "+err.Error())
		return out
	}
	out.SiteID = firstSiteID(sites)
	if out.SiteID == "" {
		return out
	}
	if data, err := get("/network/integration/v1/sites/" + out.SiteID + "/wans"); err == nil {
		if count, _, ok := countAndNames(data); ok {
			out.WANCount = &count
		}
	} else {
		out.FetchFailures = append(out.FetchFailures, "wans: "+err.Error())
	}
	if data, err := get("/network/integration/v1/sites/" + out.SiteID + "/vpn/servers"); err == nil {
		if _, names, ok := countAndNames(data); ok {
			out.VPNServers = names
		}
	} else {
		out.FetchFailures = append(out.FetchFailures, "vpn servers: "+err.Error())
	}
	return out
}

func newNovelReachCmd(flags *rootFlags) *cobra.Command {
	var probeWrites, confirm bool
	var maxConsoles int
	cmd := &cobra.Command{
		Use:   "reach",
		Short: "See which transports and products answer on each console. Write status is yes, no or unproven, and a probe alone never reports yes.",
		Long: `Use this command to find out what works on a console right now, including whether writes
are possible. Do NOT use it to assess a specific proposed change; use 'impact' instead.

Spends one read per surface per console, and reports which connector path shape works,
since the published spec contradicts itself. --probe-writes sends a mutating verb against
an all-zero id, which matches nothing. A probe can prove writes are refused but never that
they work: write_status becomes yes only after a verified change on that console.`,
		Example: strings.Trim(`
  unifi-pp-cli reach --agent
  unifi-pp-cli reach --console EXAMPLE-CONSOLE-ID --probe-writes --json
  unifi-pp-cli reach --gateway 192.168.1.1 --json`, "\n"),
		// Not mcp:read-only: that also applies while --probe-writes is on offer, which
		// would tell hosts they may auto-approve a tool that can send DELETE. The
		// unless-flags form still gives read-only hosts this tool, minus these flags.
		// Without --probe-writes this only sends reads, so dogfood can run it for real
		// rather than as a dry run that proves nothing about the probes.
		Annotations: map[string]string{"mcp:read-only-unless-flags": "probe-writes,confirm", "pp:data-source": "live", "pp:preview-happy-path": "true", "pp:happy-args": "--agent"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reach")
			}
			// Tool arguments are attacker-influenced, and this sends a mutating verb. An
			// operator typing the flag has already decided, so the CLI is unchanged.
			if probeWrites && !confirm && cliutil.IsMCPChildEnv() {
				return usageErr(errors.New("probe-writes sends a mutating verb, so an MCP caller must also set confirm to boolean true; nothing was sent"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			summary := c.DescribeTransport()
			view := reachView{Consoles: make([]reachConsole, 0), Notes: reachNotes}
			// A gateway-only setup has no cloud key, so asking api.ui.com would only
			// produce a failure the operator did not ask to see.
			var hosts []fleetHost
			var hostErr error
			if summary.Gateway != "" && c.Config != nil && c.Config.AuthHeader() == "" {
				hostErr = errors.New("skipped: no cloud key configured")
				view.SiteManager = reachSurface{Status: "skipped", Detail: "no cloud key configured"}
			} else {
				hosts, hostErr = fetchAllPages(ctx, c, "/v1/hosts", parseFleetHosts)
				view.SiteManager = classifyReachError(hostErr, http.StatusOK)
			}
			if hostErr != nil && summary.Gateway == "" {
				return classifyAPIError(cmd.OutOrStdout(), hostErr, flags)
			}
			if summary.ConsoleID != "" {
				filtered := make([]fleetHost, 0, 1)
				for _, h := range hosts {
					if h.id == summary.ConsoleID {
						filtered = append(filtered, h)
					}
				}
				if len(filtered) == 0 {
					filtered = append(filtered, fleetHost{id: summary.ConsoleID})
				}
				hosts = filtered
			}
			if summary.Gateway != "" && summary.ConsoleID == "" {
				// Local access works without a cloud key, and without knowing which host
				// this gateway is, so it gets its own row instead of being matched by guess.
				// The same key the transport journals under, or a verified local write
				// would never be found here for a gateway spelled as a URL.
				key := client.GatewayMirrorKey(summary.Gateway)
				if key == "" {
					key = "gateway:" + summary.Gateway
				}
				hosts = []fleetHost{{id: key}}
			}
			if maxConsoles > 0 && len(hosts) > maxConsoles {
				hosts = hosts[:maxConsoles]
			}
			if cliutil.IsDogfoodEnv() && len(hosts) > 1 {
				hosts = hosts[:1]
			}
			journal := loadReachJournal(ctx)
			results, probeErrs := cliutil.FanoutRun(ctx, hosts, func(h fleetHost) string { return h.id },
				func(ctx context.Context, h fleetHost) (reachConsole, error) {
					row := reachConsole{ConsoleID: h.id, Name: h.name, Transports: map[string]reachTransport{}}
					isGatewayRow := strings.HasPrefix(h.id, "gateway:")
					if hostErr == nil && !isGatewayRow {
						t := probeConnector(ctx, c, h.id, probeWrites)
						if verdict, why := writeStatusFromJournal(journal, h.id, string(client.TransportConnector)); verdict != "" {
							t.Write, t.WriteWhy = verdict, why
						}
						row.Transports["connector"] = t
						if t.Available && t.Surfaces["network"].Status == "ok" {
							row.SecondPath = readSecondPath(ctx, func(path string) (json.RawMessage, error) {
								data, _, err := c.ProbeConnectorShape(ctx, h.id, path, connectorShapeOf(t))
								return data, err
							})
						}
					} else {
						row.Transports["connector"] = reachTransport{Skipped: "no cloud access or console id", Write: "unproven"}
					}
					if summary.Gateway != "" {
						local, err := flags.newClient()
						if err != nil {
							return row, err
						}
						local.SetTargetOverride(client.TargetOverride{ConsoleID: summary.ConsoleID, Gateway: summary.Gateway, Transport: "local"})
						t := probeLocal(ctx, local, probeWrites)
						if verdict, why := writeStatusFromJournal(journal, h.id, string(client.TransportLocal)); verdict != "" {
							t.Write, t.WriteWhy = verdict, why
						}
						row.Transports["local"] = t
						if row.SecondPath == nil && t.Available && t.Surfaces["network"].Status == "ok" {
							row.SecondPath = readSecondPath(ctx, func(path string) (json.RawMessage, error) {
								data, _, err := local.Do(ctx, http.MethodGet, path, nil, nil)
								return data, err
							})
						}
					} else {
						row.Transports["local"] = reachTransport{Skipped: "no gateway configured; use --gateway to test", Write: "unproven"}
					}
					return row, nil
				}, cliutil.WithConcurrency(4))
			for _, r := range results {
				view.Consoles = append(view.Consoles, r.Value)
			}
			for _, e := range probeErrs {
				view.Failures = append(view.Failures, fleetFailure{ConsoleID: e.Source, Source: "reach", Error: e.Err.Error()})
			}
			sort.Slice(view.Consoles, func(i, j int) bool { return view.Consoles[i].ConsoleID < view.Consoles[j].ConsoleID })
			if len(view.Failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d consoles could not be probed\n", len(view.Failures), len(hosts))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "site manager: %s\n", view.SiteManager.Status)
			if len(view.Consoles) == 0 {
				fmt.Fprintln(out, "No consoles visible for this key.")
			}
			for _, row := range view.Consoles {
				fmt.Fprintf(out, "\nconsole %s %s\n", row.ConsoleID, row.Name)
				names := []string{"connector", "local"}
				for _, name := range names {
					t := row.Transports[name]
					if t.Skipped != "" {
						fmt.Fprintf(out, "  %s: skipped (%s)\n", name, t.Skipped)
						continue
					}
					fmt.Fprintf(out, "  %s: write=%s path=%s\n", name, t.Write, t.PathShape)
					keys := make([]string, 0, len(t.Surfaces))
					for k := range t.Surfaces {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						fmt.Fprintf(out, "    %s: %s\n", k, t.Surfaces[k].Status)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&probeWrites, "probe-writes", false, "Also send a mutating verb against an all-zero id to detect refused writes; changes nothing")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required with --probe-writes for MCP callers; the CLI does not need it")
	cmd.Flags().IntVar(&maxConsoles, "max-consoles", 25, "Most consoles to probe, since each spends connector budget")
	return cmd
}

func connectorShapeOf(t reachTransport) string {
	if strings.Contains(t.PathShape, "/proxy/") {
		return "/proxy"
	}
	return ""
}

// loadReachJournal is best effort: no mirror simply means no proof of past writes.
func loadReachJournal(ctx context.Context) []store.JournalEntry {
	dbPath := defaultDBPath("unifi-pp-cli")
	if mirrorMissing(dbPath) {
		return nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()
	entries, err := db.ListJournalEntries(ctx, "", time.Time{})
	if err != nil {
		return nil
	}
	return entries
}
