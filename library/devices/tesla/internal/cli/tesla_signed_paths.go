// tesla signed-paths — pure path-picker for the unified `tesla command` router.
//
// Given (command class, vehicle classification, available credentials, relay
// state, --via override), PickPath returns the chosen transport plus a short
// human-readable reason. KD6 of the 2026-05-22-001 plan governs the defaults:
// Hermes-first for owner-api on signed-cmd vehicles (free), Fleet for VCSEC
// (Hermes does not support lock/unlock/trunk), legacy REST for pre-VCP cars,
// and BLE as the explicit local-only fallback.
//
// This helper is reused by U5 (doctor + reachability matrix); keeping the
// classification logic in one pure function means the user-facing decision in
// `tesla command` and the introspective surfaces (`tesla doctor`,
// `tesla reachability`) cannot drift.
//
// Hand-coded; lives outside the generator's emit set.
package cli

import (
	"fmt"
	"strings"
)

// CommandClass enumerates the three command families that route differently.
// owner_api is the broad infotainment + charge surface (signed when on a VCP
// car, plain REST when on a pre-VCP car). vcsec is the vehicle-security
// surface (lock/unlock/trunk/sentry); Hermes does NOT support it. wake is the
// pre-command wake_up endpoint, which is a read-shaped POST that Fleet API
// handles cleanly but the Hermes proxy has a known bug on.
type CommandClass int

const (
	// ClassOwnerAPI covers signed-or-REST commands in the owner-API surface:
	// charge_start, charge_stop, climate_on, climate_off, honk_horn,
	// flash_lights, set_charge_limit, media_*, etc.
	ClassOwnerAPI CommandClass = iota
	// ClassVCSEC covers vehicle-security commands: lock, unlock,
	// actuate_trunk, sentry_mode. Hermes proxy's VCSEC handshake is broken
	// upstream, so these can ONLY go via Fleet API or BLE on a signed-cmd
	// vehicle.
	ClassVCSEC
	// ClassWake covers the wake_up endpoint. Fleet handles it cleanly; the
	// Hermes proxy has a known wake_up bug. Default is Fleet on signed-cmd
	// vehicles, REST on REST-friendly vehicles.
	ClassWake
)

// String renders a CommandClass for log/error messages.
func (c CommandClass) String() string {
	switch c {
	case ClassOwnerAPI:
		return "owner_api"
	case ClassVCSEC:
		return "vcsec"
	case ClassWake:
		return "wake"
	default:
		return fmt.Sprintf("unknown(%d)", int(c))
	}
}

// Path enumerates the chosen transport returned by PickPath. The string
// constants are stable for JSON output and for the user-facing "would <verb>
// <vehicle> via <Path>" line.
const (
	PathFleet  = "fleet"
	PathHermes = "hermes"
	PathBLE    = "ble"
	PathREST   = "rest"
)

// VehicleClass is the reachability classification we already track per
// vehicle. The signed-paths picker only cares about REST-friendly vs
// signed-cmd-required; the underlying classification enum is richer (see
// tesla_reachability.go), so the caller maps any other value through to
// "signed_cmd" by default.
const (
	VehicleClassRESTFriendly = "rest_friendly"
	VehicleClassSignedCmd    = "signed_cmd"
)

// PathChoice is the structured result of a routing decision.
type PathChoice struct {
	// Path is one of PathFleet / PathHermes / PathBLE / PathREST.
	Path string `json:"path"`
	// Reason is a short user-facing string surfaced when --send is omitted.
	// Example: "would unlock via Fleet API". Never contains secrets.
	Reason string `json:"reason"`
}

// PickPath classifies a (command class, vehicle class, available creds, relay
// state, --via override) tuple into a routing decision. Pure function: same
// inputs always return the same output. No I/O, no globals.
//
// viaOverride: "", "auto", "fleet", "hermes", "ble" (case-insensitive). Empty
// is treated the same as "auto".
func PickPath(cmdClass CommandClass, vehicleClass string, fleetReady, hermesRunning bool, viaOverride string) (PathChoice, error) {
	via := strings.ToLower(strings.TrimSpace(viaOverride))
	if via == "" {
		via = "auto"
	}

	// REST-friendly cars (pre-2021 S/X, pre-late-2021 3/Y) accept the legacy
	// REST owner-API for every command class. We never route them through
	// Fleet or Hermes because their existing tokens are iOS-app bearers, not
	// Fleet user tokens, and Hermes would add no value over the REST path
	// they already use. BLE is still a valid explicit override for users who
	// want local-only operation.
	if vehicleClass == VehicleClassRESTFriendly {
		switch via {
		case "auto":
			return PathChoice{Path: PathREST, Reason: "REST-friendly vehicle: legacy owner-API"}, nil
		case "ble":
			return PathChoice{Path: PathBLE, Reason: "BLE override for REST-friendly vehicle: tesla-control -ble recipe"}, nil
		case "fleet":
			if !fleetReady {
				return PathChoice{}, fmt.Errorf("--via=fleet requested but Fleet API not configured; run `tesla auth fleet-login`")
			}
			return PathChoice{Path: PathFleet, Reason: "Fleet API override for REST-friendly vehicle"}, nil
		case "hermes":
			if !hermesRunning {
				return PathChoice{}, fmt.Errorf("--via=hermes requested but Hermes relay not running; run `tesla relay start`")
			}
			return PathChoice{Path: PathHermes, Reason: "Hermes relay override for REST-friendly vehicle"}, nil
		default:
			return PathChoice{}, fmt.Errorf("--via=%q: must be auto|fleet|hermes|ble", viaOverride)
		}
	}

	// Signed-command-required vehicles (Highland, refreshed S/X, Cybertruck,
	// and most 2021+ cars). Routing depends on command class.
	switch via {
	case "auto":
		switch cmdClass {
		case ClassOwnerAPI:
			// KD6 default: Hermes-first when running (free), Fleet
			// otherwise (paid), BLE recipe as last resort.
			if hermesRunning {
				return PathChoice{Path: PathHermes, Reason: "Hermes relay (free owner-API path)"}, nil
			}
			if fleetReady {
				return PathChoice{Path: PathFleet, Reason: "Fleet API (Hermes relay not running)"}, nil
			}
			return PathChoice{Path: PathBLE, Reason: "BLE recipe (neither Hermes nor Fleet available)"}, nil
		case ClassVCSEC:
			// Hermes proxy does NOT support VCSEC. Fleet is the only
			// internet path; BLE is the local-only fallback.
			if fleetReady {
				return PathChoice{Path: PathFleet, Reason: "Fleet API (only internet path for VCSEC; Hermes does not support lock/unlock/trunk)"}, nil
			}
			return PathChoice{Path: PathBLE, Reason: "BLE recipe (Fleet not configured; Hermes does not support VCSEC)"}, nil
		case ClassWake:
			// Fleet handles wake_up cleanly; Hermes has a known bug.
			if fleetReady {
				return PathChoice{Path: PathFleet, Reason: "Fleet API (Hermes proxy has a known wake_up bug)"}, nil
			}
			return PathChoice{Path: PathBLE, Reason: "BLE recipe (Fleet not configured; Hermes wake_up is broken)"}, nil
		}
	case "fleet":
		if !fleetReady {
			return PathChoice{}, fmt.Errorf("--via=fleet requested but Fleet API not configured; run `tesla auth fleet-login`")
		}
		return PathChoice{Path: PathFleet, Reason: "Fleet API (explicit --via=fleet)"}, nil
	case "hermes":
		if cmdClass == ClassVCSEC {
			return PathChoice{}, fmt.Errorf("Hermes does not support lock/unlock/trunk; use --via=fleet or --via=ble")
		}
		if cmdClass == ClassWake {
			return PathChoice{}, fmt.Errorf("Hermes proxy has a known wake_up bug; use --via=fleet or --via=ble")
		}
		if !hermesRunning {
			return PathChoice{}, fmt.Errorf("--via=hermes requested but Hermes relay not running; run `tesla relay start`")
		}
		return PathChoice{Path: PathHermes, Reason: "Hermes relay (explicit --via=hermes)"}, nil
	case "ble":
		return PathChoice{Path: PathBLE, Reason: "BLE recipe (explicit --via=ble)"}, nil
	default:
		return PathChoice{}, fmt.Errorf("--via=%q: must be auto|fleet|hermes|ble", viaOverride)
	}

	// Unreachable; the switch above is exhaustive.
	return PathChoice{}, fmt.Errorf("unhandled routing tuple: cmdClass=%s vehicleClass=%s via=%s", cmdClass, vehicleClass, via)
}

// commandCatalog maps the well-known command-name aliases the
// `tesla command <name>` router accepts to their CommandClass. Unknown names
// fall back to ClassOwnerAPI per the plan (most commands are owner-api
// signed-or-REST). The catalog is deliberately conservative; it covers the
// command set the CLI's own tests and the README cookbook exercise, plus the
// VCSEC + wake outliers that route differently.
var commandCatalog = map[string]CommandClass{
	// VCSEC: vehicle-security domain. Hermes cannot deliver these.
	"lock":            ClassVCSEC,
	"door_lock":       ClassVCSEC,
	"unlock":          ClassVCSEC,
	"door_unlock":     ClassVCSEC,
	"actuate_trunk":   ClassVCSEC,
	"trunk":           ClassVCSEC,
	"sentry_mode":     ClassVCSEC,
	"set_sentry_mode": ClassVCSEC,

	// Wake: the wake_up endpoint. Fleet handles it cleanly; Hermes is buggy.
	"wake_up": ClassWake,
	"wake":    ClassWake,

	// Owner-API: infotainment, charge, climate, media. The catalog covers
	// the names the README + SKILL.md cookbook surface; unknown names fall
	// through to ClassOwnerAPI in classifyCommand below.
	"honk_horn":         ClassOwnerAPI,
	"flash_lights":      ClassOwnerAPI,
	"climate_on":        ClassOwnerAPI,
	"climate_off":       ClassOwnerAPI,
	"set_charge_limit":  ClassOwnerAPI,
	"charge_start":      ClassOwnerAPI,
	"charge_stop":       ClassOwnerAPI,
	"set_charging_amps": ClassOwnerAPI,
	"set_temps":         ClassOwnerAPI,
	"media_next_track":  ClassOwnerAPI,
	"media_prev_track":  ClassOwnerAPI,
}

// classifyCommand returns the CommandClass for a well-known command name. The
// name is matched case-insensitively after stripping a leading slash so users
// who paste a path (`/command/unlock`) get the same routing as the bare name.
// Unknown names default to ClassOwnerAPI: the plan's "unknown command names
// go to owner_api by default" rule.
func classifyCommand(name string) CommandClass {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimPrefix(n, "/")
	if class, ok := commandCatalog[n]; ok {
		return class
	}
	return ClassOwnerAPI
}
