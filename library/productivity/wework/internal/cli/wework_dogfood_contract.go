// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Printing Press live-dogfood metadata. This stays outside
// generated command files so a future reprint can reconcile it safely.

package cli

import "github.com/spf13/cobra"

const weworkGeoDogfoodHappyArgs = "--city=Austin;--boundnw-lat=30.4472;--boundnw-lng=-97.9631;--boundse-lat=30.0872;--boundse-lng=-97.5231;--user-latitude=30.2672;--user-longitude=-97.7431;--is-authenticated=true;--is-on-demand-user=true"

func materializeWeworkDogfoodContracts(root *cobra.Command) {
	if search, _, err := root.Find([]string{"search"}); err == nil {
		if search.Annotations == nil {
			search.Annotations = map[string]string{}
		}
		// Search accepts arbitrary text, so provide a stable positional fixture
		// that exercises the real read-only endpoint during full dogfood.
		search.Annotations["pp:happy-args"] = "query=Austin"
	}

	if geo, _, err := root.Find([]string{"wework-yardi", "list-locations-by-geo"}); err == nil {
		if geo.Annotations == nil {
			geo.Annotations = map[string]string{}
		}
		geo.Annotations["pp:happy-args"] = weworkGeoDogfoodHappyArgs
	}

	if details, _, err := root.Find([]string{"common-booking", "get-booking-details"}); err == nil {
		if details.Annotations == nil {
			details.Annotations = map[string]string{}
		}
		// A booking UUID is account-specific and short-lived. Exercise the real
		// endpoint with a stable invalid fixture and accept the CLI's typed
		// not-found result instead of publishing personal reservation data.
		details.Annotations["pp:happy-args"] = "--booking-uuid=example-id"
		details.Annotations["pp:typed-exit-codes"] = "0,3"
	}

	if feedback, _, err := root.Find([]string{"feedback"}); err == nil {
		if feedback.Annotations == nil {
			feedback.Annotations = map[string]string{}
		}
		// Any non-empty free-form text is valid feedback, so there is no
		// domain-invalid positional argument for the generic error-path probe.
		feedback.Annotations["pp:no-error-path-probe"] = "true"
		feedback.Example = `  wework-pp-cli feedback "The desks output was difficult to filter"
  printf '%s\n' "The auth hint was unclear" | wework-pp-cli feedback --stdin
  wework-pp-cli feedback list --limit 5 --json`
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, _ *rootFlags) {
		materializeWeworkDogfoodContracts(root)
	})
}
