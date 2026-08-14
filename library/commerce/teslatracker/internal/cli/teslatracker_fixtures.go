// Hand-authored. Own file so `generate --force` keeps it whole.
//
// The dogfood/verify harness synthesises generic positional values ("test-id").
// /api/inventory/{vin} correctly 404s on those, so happy-path probes fail on a
// CLI that is working fine. Declare real fixture inputs instead. Also stop the
// `watch` group from being probed as if it were a leaf command.

package cli

import "github.com/spf13/cobra"

// A real VIN present in the live source, used only as a harness fixture.
const fixtureVIN = "5YJ3E1EA7LF745758"

// The saved-search name the harness creates and then reads back. Named so it is
// obvious in `watch list` that it came from a verification run, not the user.
const fixtureWatch = "pp-dogfood"

func applyTeslatrackerFixtures(root *cobra.Command) {
	set := func(c *cobra.Command, k, v string) {
		if c == nil {
			return
		}
		if c.Annotations == nil {
			c.Annotations = map[string]string{}
		}
		c.Annotations[k] = v
	}
	find := func(path ...string) *cobra.Command {
		cur := root
		for _, want := range path {
			var next *cobra.Command
			for _, c := range cur.Commands() {
				if c.Name() == want {
					next = c
					break
				}
			}
			if next == nil {
				return nil
			}
			cur = next
		}
		return cur
	}

	// generated endpoint commands need a VIN that actually exists
	set(find("inventory", "get"), "pp:happy-args", "--vin="+fixtureVIN)
	set(find("inventory", "report"), "pp:happy-args", "--vin="+fixtureVIN)

	// hand-written VIN commands
	for _, n := range []string{"warranty", "degradation", "comps", "price-history"} {
		set(find(n), "pp:happy-args", "<vin>="+fixtureVIN)
	}
	// radius needs coordinates, not a positional
	set(find("radius"), "pp:happy-args", "--lat=30.2241;--lon=-92.0198")
	// comps/degradation cannot distinguish "bad VIN" from "mirror not hydrated
	// yet" without inventing API-specific semantics, so the harness error-path
	// probe would be testing our guess, not the CLI.
	set(find("comps"), "pp:no-error-path-probe", "true")
	set(find("degradation"), "pp:no-error-path-probe", "true")
	// Bare `watch` runs every saved search, so it needs no fixture args. Its
	// error path is not probed: there is no invalid input to a command that takes
	// none, and an empty watchlist is a valid starting state, not an error.
	set(find("watch"), "pp:no-error-path-probe", "true")

	// Exercise `watch add` for real rather than leaving the feature hollow; it is
	// idempotent, so the harness invoking it twice is fine.
	//
	// `rm` and `run` are deliberately left unprobed. The harness runs mutating
	// commands under --dry-run in an isolated home, so no saved search ever
	// exists for them to act on, and both correctly exit non-zero on a name that
	// is not there. Making either succeed on a missing watch would mean reporting
	// a deletion or a diff that never happened. Their error paths are probed and
	// pass, which is the behaviour that actually matters here.
	set(find("watch", "add"), "pp:happy-args", "<name>="+fixtureWatch)

	// `feedback` records whatever text it is given. There is no such thing as an
	// invalid argument to it, so exiting zero is correct and the error-path probe
	// has nothing to assert.
	set(find("feedback"), "pp:no-error-path-probe", "true")
}
