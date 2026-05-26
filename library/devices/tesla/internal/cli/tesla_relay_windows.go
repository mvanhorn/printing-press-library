//go:build windows

// Windows stub for relay subprocess management. The relay binary
// (`tesla-http-proxy`) does not have a documented Windows deployment story
// for this CLI; the graceful-degrade path is for `relay start` to print a
// hint and exit zero (see runRelayStart). This stub exists so the package
// builds on Windows but the launcher never gets called - the windows path
// short-circuits earlier.

package cli

import "errors"

func launchRelayDetached(spec relayLaunchSpec) (int, error) {
	return 0, errors.New("Hermes relay subprocess management not supported on Windows; use Fleet API via `tesla command --via=fleet`")
}
