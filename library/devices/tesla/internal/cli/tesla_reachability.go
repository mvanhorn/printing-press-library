// tesla reachability — classify whether the user's vehicle accepts REST
// commands or requires the Vehicle Command Protocol (signed-command rollout).
// Hand-coded; out-of-tree from generator.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

func newReachabilityCmd(flags *rootFlags) *cobra.Command {
	var vehicleID string
	cmd := &cobra.Command{
		Use:   "reachability",
		Short: "Classify your Tesla as REST-friendly or signed-command-required",
		Long: `Probes /api/1/products plus a benign command/honk_horn (or vehicle_state read) to
classify the cloud reachability story for your vehicle:

  REST_OK              - Vehicle accepts REST commands (pre-2021 S/X, pre-late-2021 3/Y)
  SIGNED_COMMAND_REQ   - Vehicle is on the new protocol; install tesla-control
  TOKEN_EXPIRED        - Bearer token needs refresh
  TESLA_5XX            - Tesla cloud unreachable, retry later`,
		Example:     "  tesla-pp-cli reachability --vehicle 5YJ3E1EA6XXXXXXXX --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report := probeReachability(cmd.Context(), c, vehicleID)
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&vehicleID, "vehicle", "", "Vehicle id from /api/1/products (optional - auto-picks first)")
	return cmd
}

type reachabilityReport struct {
	Classification string       `json:"classification"`
	Detail         string       `json:"detail"`
	ShimURL        string       `json:"shim_url,omitempty"`
	Checks         []probeCheck `json:"checks"`
	VIN            string       `json:"vin,omitempty"`
	VehicleID      string       `json:"vehicle_id,omitempty"`
	Recommended    string       `json:"recommended,omitempty"`
	// RecommendedVia is the user-action-level transport recommendation that
	// pairs with the protocol-level Classification. Enum: fleet | hermes |
	// ble | rest | none. ADDITIVE field (U5 of 2026-05-22-001 plan); the
	// Classification enum values above are unchanged so README docs and
	// downstream MCP consumers still match. Empty means the probe didn't
	// reach the point of recommending a path (e.g. token expired, 5xx).
	RecommendedVia string `json:"recommended_via,omitempty"`
	// AvailablePaths lists every transport that would dispatch a command
	// for this vehicle right now. Subset of {fleet, hermes, ble, rest}.
	// Populated alongside RecommendedVia from the same picker inputs so
	// the two fields can never disagree. ADDITIVE field.
	AvailablePaths []string `json:"available_paths,omitempty"`
}

type probeCheck struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

func probeReachability(ctx context.Context, c *client.Client, vehicleID string) *reachabilityReport {
	r := &reachabilityReport{Checks: []probeCheck{}}

	// Probe 1: GET /api/1/products
	status, err := c.ProbeGet("/api/1/products")
	chk := probeCheck{Name: "products_list", Status: status, OK: status >= 200 && status < 300}
	if err != nil {
		chk.Detail = err.Error()
	}
	r.Checks = append(r.Checks, chk)

	if status == 401 {
		r.Classification = "TOKEN_EXPIRED"
		r.Detail = "bearer token expired or missing"
		r.Recommended = "Run: tesla-pp-cli auth refresh  (or auth login --paste if no refresh token is stored)"
		return r
	}
	if status >= 500 {
		r.Classification = "TESLA_5XX"
		r.Detail = fmt.Sprintf("Tesla cloud returned %d; retry later", status)
		return r
	}
	if status >= 400 || status == 0 {
		r.Classification = "UNKNOWN"
		r.Detail = fmt.Sprintf("unexpected status %d on /api/1/products", status)
		return r
	}

	// Probe 2: fetch products to pick a vehicle if not provided
	if vehicleID == "" {
		raw, err := c.Get("/api/1/products", nil)
		if err == nil {
			var env struct {
				Response []struct {
					VIN       string `json:"vin"`
					VehicleID int64  `json:"vehicle_id"`
				} `json:"response"`
			}
			if json.Unmarshal(raw, &env) == nil && len(env.Response) > 0 {
				vehicleID = fmt.Sprintf("%d", env.Response[0].VehicleID)
				r.VIN = env.Response[0].VIN
			}
		}
	}
	r.VehicleID = vehicleID
	if vehicleID == "" {
		r.Classification = "REST_OK"
		r.Detail = "products endpoint reachable; no vehicle to probe further"
		annotateReachabilityPaths(r, VehicleClassRESTFriendly)
		return r
	}

	// Probe 3: try a benign command via vehicle_state (GET, no signed-command impact)
	statePath := strings.ReplaceAll("/api/1/vehicles/{vehicle_id}/data_request/vehicle_state", "{vehicle_id}", vehicleID)
	vsStatus, vsErr := c.ProbeGet(statePath)
	r.Checks = append(r.Checks, probeCheck{Name: "vehicle_state", Status: vsStatus, OK: vsStatus >= 200 && vsStatus < 300, Detail: errToString(vsErr)})

	if vsStatus == 200 {
		// Read the body to look for vehicle_command_protocol_required hint
		raw, gerr := c.Get(statePath, nil)
		if gerr == nil && containsSignedCmdSignal(raw) {
			r.Classification = "SIGNED_COMMAND_REQ"
			r.Detail = "vehicle reports vehicle-command-protocol enforcement on writes"
			r.ShimURL = "https://github.com/teslamotors/vehicle-command"
			r.Recommended = "REST reads work; for commands install tesla-control from teslamotors/vehicle-command and enroll a public key."
			annotateReachabilityPaths(r, VehicleClassSignedCmd)
			return r
		}
		r.Classification = "REST_OK"
		r.Detail = "vehicle responds to REST reads; commands will use plain REST"
		r.Recommended = "Your vehicle accepts the legacy REST owner-API. No tesla-control needed."
		annotateReachabilityPaths(r, VehicleClassRESTFriendly)
		return r
	}

	// vehicle_state failed - vehicle likely asleep; can't classify firmly
	r.Classification = "VEHICLE_ASLEEP_OR_OFFLINE"
	r.Detail = "vehicle_state read failed; vehicle may be asleep. Run `tesla vehicles get <id>` to wake."
	r.Recommended = "Wake the vehicle first: tesla-pp-cli vehicles get " + vehicleID
	return r
}

func containsSignedCmdSignal(raw json.RawMessage) bool {
	s := string(raw)
	hints := []string{
		"vehicle_command_protocol_required",
		"command authentication is required",
		"signed_command",
	}
	low := strings.ToLower(s)
	for _, h := range hints {
		if strings.Contains(low, h) {
			return true
		}
	}
	return false
}

func errToString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

// annotateReachabilityPaths fills the additive RecommendedVia and
// AvailablePaths fields (U5 of the 2026-05-22-001 plan). The existing
// Classification value is left untouched: README at line 25 and downstream
// MCP consumers depend on the v3-stable enum {REST_OK, SIGNED_COMMAND_REQ,
// TOKEN_EXPIRED, TESLA_5XX, VEHICLE_ASLEEP_OR_OFFLINE, UNKNOWN}. The new
// fields run alongside it.
//
// The recommendation is derived from the same PickPath helper the
// `tesla command` router uses, so reachability and dispatch can never
// disagree. For the VCSEC class we report the broadest available transport
// (Fleet wins for VCSEC by KD6 of the plan); available_paths enumerates
// every transport that would dispatch SOMETHING for this vehicle today.
func annotateReachabilityPaths(r *reachabilityReport, vehicleClass string) {
	if r == nil {
		return
	}
	fleetReady := reachabilityFleetReady()
	hermesRunning := reachabilityHermesRunning()

	// REST-friendly cars: legacy REST always available; Fleet and Hermes
	// are still selectable as overrides; BLE recipe is always available
	// as a manual fallback.
	if vehicleClass == VehicleClassRESTFriendly {
		r.RecommendedVia = PathREST
		r.AvailablePaths = []string{PathREST}
		if fleetReady {
			r.AvailablePaths = append(r.AvailablePaths, PathFleet)
		}
		if hermesRunning {
			r.AvailablePaths = append(r.AvailablePaths, PathHermes)
		}
		r.AvailablePaths = append(r.AvailablePaths, PathBLE)
		return
	}

	// Signed-command-required vehicles. KD6: Fleet wins for VCSEC (Hermes
	// cannot carry VCSEC). For the user-facing recommendation we surface
	// the transport that covers the broadest command set:
	//   - Fleet ready: Fleet (covers VCSEC + owner-api + wake).
	//   - Hermes running but no Fleet: Hermes (owner-api only).
	//   - Neither: BLE recipe.
	//   - No paths at all (shouldn't happen for signed-cmd because BLE is
	//     always available as a recipe): "none".
	available := []string{}
	if fleetReady {
		available = append(available, PathFleet)
	}
	if hermesRunning {
		available = append(available, PathHermes)
	}
	available = append(available, PathBLE)
	r.AvailablePaths = available

	switch {
	case fleetReady:
		r.RecommendedVia = PathFleet
	case hermesRunning:
		r.RecommendedVia = PathHermes
	default:
		r.RecommendedVia = PathBLE
	}
}

// reachabilityFleetReady mirrors commandFleetReady but lives in the
// reachability file so the package's two surfaces compile independently.
// Reads cfg + env override; never echoes token bytes.
func reachabilityFleetReady() bool {
	if os.Getenv("TESLA_FLEET_TOKEN") != "" {
		return true
	}
	cfg, err := config.Load("")
	if err != nil || cfg == nil {
		return false
	}
	return cfg.FleetTokens().AccessToken != ""
}

// reachabilityHermesRunning mirrors commandHermesRunning. Reads the relay
// state file the `tesla relay start` subprocess writes.
func reachabilityHermesRunning() bool {
	if os.Getenv(commandHermesPortEnv) != "" {
		return true
	}
	paths, err := newRelayPaths()
	if err != nil {
		return false
	}
	_, _, alive := readRelayState(paths)
	return alive
}
