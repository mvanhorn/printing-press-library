package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// cobraTestHelper wraps *cobra.Command so we can pass it through a map
// without importing cobra everywhere in the test.
type cobraTestHelper struct {
	cmd *cobra.Command
}

// TestSoldFlagsFor locks the Stingray-accepted sf-parameter values per
// --sold-window. Regression for issue #482: the prior default of
// "1,3,5,7,9" was rejected by Stingray with "Invalid arguments"
// (resultCode 101), breaking every sold-data path. The 3y combo
// "1,2,3,5,6,7" is what redfin.com fires for the "include sold past
// 3 years" filter button — verified accepted on 2026-05-12.
func TestSoldFlagsFor(t *testing.T) {
	cases := []struct {
		window string
		want   string
	}{
		{"1mo", "1"},
		{"3mo", "3"},
		{"6mo", "5"},
		{"1y", "7"},
		{"2y", "9"},
		{"3y", "1,2,3,5,6,7"},
		{"", "1,2,3,5,6,7"},        // empty == default 3y
		{"unknown", "1,2,3,5,6,7"}, // unknown falls back to 3y, NOT the broken 1,3,5,7,9
		{"7y", "1,2,3,5,6,7"},      // outside the named set: same fallback
	}
	for _, tc := range cases {
		t.Run("window="+tc.window, func(t *testing.T) {
			got := soldFlagsFor(tc.window)
			if got != tc.want {
				t.Errorf("soldFlagsFor(%q) = %q, want %q", tc.window, got, tc.want)
			}
		})
	}
}

// TestSoldFlagsFor_NoBrokenDefault is a paranoid second pass that
// explicitly asserts the prior broken value never reappears as the
// default. If a future refactor accidentally restores "1,3,5,7,9",
// this test will catch it before another shipcheck-passing PR
// reintroduces the Stingray rejection.
func TestSoldFlagsFor_NoBrokenDefault(t *testing.T) {
	brokenValue := "1,3,5,7,9"
	inputs := []string{"", "3y", "unknown", "7y", "10y"}
	for _, in := range inputs {
		got := soldFlagsFor(in)
		if got == brokenValue {
			t.Errorf("soldFlagsFor(%q) returned the issue #482 broken value %q", in, got)
		}
	}
}

// TestOptsFromFlags_SoldWindowAndSF covers the precedence rules: --sf
// wins over --sold-window, --sold-window picks the right code, status
// other than "sold" leaves SoldFlags empty regardless of either flag,
// and a typo (`1yr`, `12mo`, etc.) surfaces a usage error instead of
// silently resolving to the 3y default. Regression for Greptile P1
// #3230445517.
func TestOptsFromFlags_SoldWindowAndSF(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		soldWindow string
		sf         string
		want       string
		wantErr    bool
	}{
		{"for-sale ignores valid sold-window value", "for-sale", "1y", "", "", false},
		{"for-sale ignores raw sf", "for-sale", "", "1,2,3", "", false},
		{"sold default uses 3y combo", "sold", "", "", "1,2,3,5,6,7", false},
		{"sold + 1y window uses single code 7", "sold", "1y", "", "7", false},
		{"sold + 2y window uses single code 9", "sold", "2y", "", "9", false},
		{"sold + raw sf wins over window", "sold", "1y", "1,3", "1,3", false},
		{"sold + raw sf, no window", "sold", "", "9", "9", false},
		{"sold + raw sf, bad window — sf bypasses validation", "sold", "1yr", "1,2,3", "1,2,3", false},
		// --sold-window is validated regardless of --status: invalid values
		// surface as usage errors even when the flag is moot for the chosen
		// status. Better to fail loudly on a typo than to silently ignore.
		{"for-sale + bad window still errors (flag-level validation)", "for-sale", "bogus", "", "", true},
		{"sold + typo 1yr returns usage error", "sold", "1yr", "", "", true},
		{"sold + typo 12mo returns usage error", "sold", "12mo", "", "", true},
		{"sold + unknown window 5y returns usage error", "sold", "5y", "", "", true},
		{"sold + bogus window returns usage error", "sold", "bogus", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hf := &homesFlags{
				status:     tc.status,
				soldWindow: tc.soldWindow,
				sf:         tc.sf,
				regionID:   1,
				regionType: 6,
			}
			opts, err := optsFromFlags(hf)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for soldWindow=%q, got SoldFlags=%q", tc.soldWindow, opts.SoldFlags)
				}
				if !strings.Contains(err.Error(), tc.soldWindow) {
					t.Errorf("error should name the offending value %q; got: %v", tc.soldWindow, err)
				}
				if !strings.Contains(err.Error(), "1mo|3mo|6mo|1y|2y|3y") {
					t.Errorf("error should list valid values; got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("optsFromFlags: %v", err)
			}
			if opts.SoldFlags != tc.want {
				t.Errorf("SoldFlags = %q, want %q", opts.SoldFlags, tc.want)
			}
		})
	}
}

// TestSoldWindowFlagsRegistered sanity-checks that both newHomesCmd and
// newSoldCmd register --sold-window and --sf, and that the --sold-window
// help text names every value soldFlagsFor accepts so agents and users
// don't need to read source to discover them.
func TestSoldWindowFlagsRegistered(t *testing.T) {
	cmds := map[string]*cobraTestHelper{
		"homes": {cmd: newHomesCmd(&rootFlags{})},
		"sold":  {cmd: newSoldCmd(&rootFlags{})},
	}
	for name, h := range cmds {
		t.Run(name, func(t *testing.T) {
			if h.cmd.Flag("sold-window") == nil {
				t.Errorf("%s: --sold-window flag not registered", name)
			}
			if h.cmd.Flag("sf") == nil {
				t.Errorf("%s: --sf flag not registered", name)
			}
			if flag := h.cmd.Flag("sold-window"); flag != nil {
				for _, value := range []string{"1mo", "3mo", "6mo", "1y", "2y", "3y"} {
					if !strings.Contains(flag.Usage, value) {
						t.Errorf("%s: --sold-window usage missing %q: %s", name, value, flag.Usage)
					}
				}
			}
		})
	}
}
