// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared helpers for the hand-written Tailscale admin commands: tailnet
// selection, device fetch and selector resolution, the local `tailscale
// status --json` bridge, and plan/apply output conventions.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/config"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

// tsDefaultTailnet returns TAILSCALE_TAILNET when set, else "-" (the access
// token's own tailnet).
func tsDefaultTailnet() string {
	if v := strings.TrimSpace(cliutil.EnvOverride("TAILSCALE_TAILNET")); v != "" {
		return v
	}
	return "-"
}

func addTailnetFlag(cmd *cobra.Command, dst *string) {
	cmd.Flags().StringVar(dst, "tailnet", tsDefaultTailnet(), "Tailnet ID; '-' means the access token's own tailnet (env: TAILSCALE_TAILNET)")
}

func tailnetPath(tailnet, suffix string) string {
	if strings.TrimSpace(tailnet) == "" {
		tailnet = "-"
	}
	return "/tailnet/" + cliutil.EscapePathParam(tailnet) + suffix
}

// tsLiveClient returns a client that performs real reads even when --dry-run
// is set, so plan output reflects current server state. Callers must not
// issue writes through it unless the user confirmed with --yes. It is also
// the single place hand-written commands resolve config and credentials.
func tsLiveClient(ctx context.Context, flags *rootFlags) (*client.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	c.DryRun = false
	c.NoCache = true
	if err := tsEnsureOAuth(ctx, c.Config, c.HTTPClient); err != nil {
		return nil, err
	}
	if _, err := tsRequireAuth(c.Config); err != nil {
		return nil, err
	}
	return c, nil
}

// tsRequireAuth returns the Authorization header value, or a typed auth error
// that prefers the generated "stored credentials refused" message.
func tsRequireAuth(cfg *config.Config) (string, error) {
	if auth := strings.TrimSpace(cfg.AuthHeader()); auth != "" {
		return auth, nil
	}
	if cfg.HasCredentialRefusals() {
		return "", authErr(cfg.CredentialRefusalError())
	}
	return "", authErr(errors.New("no Tailscale credentials: export TAILSCALE_API_KEY (an API access token from https://login.tailscale.com/admin/settings/keys) or set TAILSCALE_OAUTH_CLIENT_ID and TAILSCALE_OAUTH_CLIENT_SECRET"))
}

// tsPlanOnly reports whether a hand-written mutation should stop after
// printing its plan. Writes need an explicit --yes; --dry-run always wins.
func tsPlanOnly(flags *rootFlags) bool {
	return flags.dryRun || !flags.yes
}

const tsApplyHint = "plan only; nothing was changed. Re-run with --yes to apply."

// getLive performs an uncached GET and decodes the JSON body into T.
func getLive[T any](ctx context.Context, c *client.Client, path string, params map[string]string, what string) (T, error) {
	var v T
	data, err := c.GetNoCache(ctx, path, params)
	if err != nil {
		return v, classifyAPIErrorOnly(err)
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return v, apiErr(fmt.Errorf("decoding %s: %w", what, err))
	}
	return v, nil
}

func devicePath(id, suffix string) string {
	return "/device/" + cliutil.EscapePathParam(id) + suffix
}

func fetchDevicesLive(ctx context.Context, c *client.Client, tailnet string, allFields bool) ([]tsadmin.Device, error) {
	params := map[string]string{}
	if allFields {
		params["fields"] = "all"
	}
	resp, err := getLive[struct {
		Devices []tsadmin.Device `json:"devices"`
	}](ctx, c, tailnetPath(tailnet, "/devices"), params, "devices")
	return resp.Devices, err
}

func fetchDeviceLive(ctx context.Context, c *client.Client, id string) (tsadmin.Device, error) {
	return getLive[tsadmin.Device](ctx, c, devicePath(id, ""), map[string]string{"fields": "all"}, "device")
}

type deviceRoutes struct {
	AdvertisedRoutes []string `json:"advertisedRoutes"`
	EnabledRoutes    []string `json:"enabledRoutes"`
}

func fetchDeviceRoutes(ctx context.Context, c *client.Client, id string) (deviceRoutes, error) {
	return getLive[deviceRoutes](ctx, c, devicePath(id, "/routes"), nil, "routes")
}

func fetchDeviceInvites(ctx context.Context, c *client.Client, id string) ([]tsadmin.Invite, error) {
	return getLive[[]tsadmin.Invite](ctx, c, devicePath(id, "/device-invites"), nil, "device invites")
}

// deviceRef identifies a device in command output.
type deviceRef struct {
	NodeID   string `json:"node_id"`
	Hostname string `json:"hostname"`
	Name     string `json:"name,omitempty"`
}

func refOf(d tsadmin.Device) deviceRef {
	return deviceRef{NodeID: d.PreferredID(), Hostname: d.Hostname, Name: d.Name}
}

// resolveDevice maps a selector (self, nodeId, legacy id, Tailscale IP,
// hostname, MagicDNS name) to exactly one device.
func resolveDevice(ctx context.Context, devs []tsadmin.Device, sel string) (tsadmin.Device, error) {
	sel = strings.TrimSpace(sel)
	if strings.EqualFold(sel, "self") {
		selfID, err := readLocalSelfID(ctx)
		if err != nil {
			return tsadmin.Device{}, usageErr(fmt.Errorf("'self' needs the local tailscale command (%v); pass a hostname, MagicDNS name, Tailscale IP, or nodeId instead", err))
		}
		sel = selfID
	}
	matches := tsadmin.MatchDevices(devs, sel)
	switch len(matches) {
	case 0:
		return tsadmin.Device{}, notFoundErr(fmt.Errorf("no device matches %q; try a hostname, MagicDNS name, Tailscale IP, or nodeId", sel))
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s (%s)", m.Label(), m.PreferredID()))
		}
		return tsadmin.Device{}, usageErr(fmt.Errorf("%q matches %d devices: %s; pass a nodeId to pick one", sel, len(matches), strings.Join(names, ", ")))
	}
}

var tailscaleBinaryCandidates = []string{
	"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
	"/opt/homebrew/bin/tailscale",
	"/usr/local/bin/tailscale",
	"/usr/bin/tailscale",
}

func findTailscaleBinary() string {
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	for _, p := range tailscaleBinaryCandidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// readLocalSelfID returns this machine's ID from `tailscale status --json`,
// which equals the admin API's device nodeId. TAILSCALE_PP_STATUS_FILE points
// at a saved status document instead (used by tests and offline runs).
func readLocalSelfID(ctx context.Context) (string, error) {
	var raw []byte
	if f := strings.TrimSpace(os.Getenv("TAILSCALE_PP_STATUS_FILE")); f != "" {
		b, err := os.ReadFile(filepath.Clean(f))
		if err != nil {
			return "", fmt.Errorf("reading TAILSCALE_PP_STATUS_FILE: %w", err)
		}
		raw = b
	} else {
		bin := findTailscaleBinary()
		if bin == "" {
			return "", errors.New("tailscale command not found")
		}
		run := func(args ...string) ([]byte, error) {
			cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(cctx, bin, args...) // #nosec G204 -- fixed binary path and arguments.
			cmd.Env = append(os.Environ(), "TAILSCALE_BE_CLI=1")
			return cmd.Output()
		}
		// --peers=false skips building and decoding the whole peer map; very
		// old clients reject the flag, so fall back to the full document.
		out, err := run("status", "--json", "--peers=false")
		if err != nil {
			out, err = run("status", "--json")
		}
		if err != nil {
			return "", fmt.Errorf("tailscale status --json failed: %w", err)
		}
		raw = out
	}
	var doc struct {
		Self *struct {
			ID string `json:"ID"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("parsing tailscale status: %w", err)
	}
	if doc.Self == nil || doc.Self.ID == "" {
		return "", errors.New("tailscale status has no Self ID (is this machine logged in?)")
	}
	return doc.Self.ID, nil
}

// localSelfID is readLocalSelfID for callers that treat "unknown" as "not this
// machine".
func localSelfID(ctx context.Context) string {
	id, _ := readLocalSelfID(ctx)
	return id
}

// emitMachine writes v as JSON for machine callers, or returns false so the
// caller renders its human view.
func emitMachine(cmd *cobra.Command, flags *rootFlags, v any) (bool, error) {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return true, printJSONFiltered(cmd.OutOrStdout(), v, flags)
	}
	return false, nil
}

// emitOrRender prints v for machine callers or runs render for humans, then
// returns held, an error the command decided on after building v (for
// example a failed write that must still be reported).
func emitOrRender(cmd *cobra.Command, flags *rootFlags, v any, render func(), held error) error {
	if ok, err := emitMachine(cmd, flags, v); ok {
		if err != nil {
			return err
		}
		return held
	}
	render()
	return held
}

func joinOrDash(in []string) string {
	if len(in) == 0 {
		return "-"
	}
	return strings.Join(in, ", ")
}
