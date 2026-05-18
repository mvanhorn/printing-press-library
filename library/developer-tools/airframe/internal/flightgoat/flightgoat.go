// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

// Package flightgoat is a thin subprocess wrapper around flight-goat-pp-cli.
// airframe uses it to resolve a commercial flight ident (e.g., "UA1234") to
// the aircraft tail number that flight-goat reports — then airframe enriches
// that tail with FAA registry + NTSB history from the local store.
package flightgoat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ExecutableName is the binary airframe shells out to.
const ExecutableName = "flight-goat-pp-cli"

// Subcommand is the exact flight-goat invocation path that returns owner +
// registration metadata for a flight ident. Confirmed via
// `flight-goat-pp-cli which ident` against the published catalog: the API
// surface is `aircraft owner get-aircraft`, hitting the AeroAPI endpoint
// `/aircraft/{ident}/owner`. It requires FLIGHT_GOAT_API_KEY_AUTH.
var Subcommand = []string{"aircraft", "owner", "get-aircraft"}

// ErrNotInstalled is returned when flight-goat-pp-cli is not on PATH.
var ErrNotInstalled = errors.New("flight-goat-pp-cli not installed on PATH")

// ErrNoRegistration is returned when flight-goat returned a successful
// response but airframe couldn't find a tail/registration field in it.
var ErrNoRegistration = errors.New("flight-goat returned no registration for the requested ident")

// InstallHint is a one-line install instruction surfaced to users when
// the binary is missing.
const InstallHint = "Install: go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-cli@latest"

// AuthHint is shown when flight-goat 401s on AeroAPI; FLIGHT_GOAT_API_KEY_AUTH
// is the env var flight-goat reads.
const AuthHint = "Set FLIGHT_GOAT_API_KEY_AUTH=<your AeroAPI key> — flight-goat needs FlightAware credentials to map idents to tails."

// FlightLookup is the merged result airframe surfaces to its callers.
// Registration is the canonical N-number; Raw preserves the full flight-goat
// payload so downstream consumers can pivot on extra fields.
type FlightLookup struct {
	Ident        string         `json:"ident"`
	Registration string         `json:"registration"`
	Source       string         `json:"source"`
	Raw          map[string]any `json:"raw,omitempty"`
}

// IsInstalled returns true when flight-goat-pp-cli is on PATH.
func IsInstalled() bool {
	_, err := exec.LookPath(ExecutableName)
	return err == nil
}

// ResolveIdent invokes flight-goat to look up registration metadata for the
// given flight ident (e.g., "UA1234", "N628TS", or an fa_flight_id).
// Returns ErrNotInstalled when the binary is missing; wraps the flight-goat
// stderr verbatim when an upstream error occurs.
func ResolveIdent(ctx context.Context, ident string) (*FlightLookup, error) {
	path, err := exec.LookPath(ExecutableName)
	if err != nil {
		return nil, ErrNotInstalled
	}

	args := append([]string{}, Subcommand...)
	args = append(args, ident, "--agent")
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "INVALID_API_KEY") || strings.Contains(msg, "401") {
			return nil, fmt.Errorf("%w\n%s", errors.New(AuthHint), msg)
		}
		if msg != "" {
			return nil, fmt.Errorf("flight-goat invocation failed: %w\n%s", err, msg)
		}
		return nil, fmt.Errorf("flight-goat invocation failed: %w", err)
	}

	raw := map[string]any{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("parsing flight-goat output: %w", err)
	}
	// PATCH: unwrap the standard Printing Press envelope when present.
	// Previously the code tried a second Unmarshal *only* when the first
	// failed — but `json.Unmarshal` into map[string]any succeeds for both
	// flat and envelope shapes, so the fallback was unreachable. An
	// envelope response would leave `raw` holding {"results":…,"meta":…}
	// and pickRegistration would search the wrong layer, always returning
	// ErrNoRegistration.
	if results, ok := raw["results"].(map[string]any); ok {
		raw = results
	}

	registration := pickRegistration(raw)
	if registration == "" {
		return nil, ErrNoRegistration
	}

	return &FlightLookup{
		Ident:        ident,
		Registration: registration,
		Source:       "flight-goat",
		Raw:          raw,
	}, nil
}

// pickRegistration searches the flight-goat payload for the first field
// that looks like an N-number / registration. AeroAPI uses several names
// across endpoints (`registration`, `aircraft_registration`, `tail`,
// `tail_number`); accept any of them, plus an `owner_aircraft_registration`
// variant that the `aircraft owner` endpoint may emit.
func pickRegistration(m map[string]any) string {
	keys := []string{
		"registration", "aircraft_registration",
		"tail", "tail_number",
		"owner_aircraft_registration",
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return strings.ToUpper(strings.TrimSpace(s))
			}
		}
	}
	// Some envelopes nest under `aircraft.registration`.
	if nested, ok := m["aircraft"].(map[string]any); ok {
		if s, ok := nested["registration"].(string); ok && s != "" {
			return strings.ToUpper(strings.TrimSpace(s))
		}
	}
	return ""
}
