package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil/testenv"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTravelokaDataFileRequiresBoundedObject(t *testing.T) {
	if _, err := readTravelokaDataFile(t.TempDir()); err == nil {
		t.Fatal("non-regular source input accepted")
	}
	for _, raw := range []string{"null", "[]", "{bad}", "{} {}", strings.Repeat(" ", (2<<20)+1)} {
		p := filepath.Join(t.TempDir(), "public.json")
		if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readTravelokaDataFile(p); err == nil {
			t.Fatal("invalid source file accepted")
		}
	}
	p := filepath.Join(t.TempDir(), "public.json")
	os.WriteFile(p, []byte(`{"query":"Singapore"}`), 0600)
	if _, err := readTravelokaDataFile(p); err != nil {
		t.Fatal(err)
	}
}

func TestTravelokaSourceDataFileMatchesExplicitData(t *testing.T) {
	p := filepath.Join(t.TempDir(), "public.json")
	raw := `{"query":"Singapore"}`
	os.WriteFile(p, []byte(raw), 0600)
	called := false
	c := &cobra.Command{Use: "source"}
	c.Flags().String("data", "", "source data")
	c.Flags().Bool("stdin", false, "stdin body")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		called = true
		got, _ := cmd.Flags().GetString("data")
		if got != raw || !cmd.Flags().Changed("data") {
			t.Fatalf("file object did not become explicit source data: %q", got)
		}
		return nil
	}
	addTravelokaDataFile(c, &rootFlags{})
	c.SetArgs([]string{"--data-file", p})
	if err := c.Execute(); err != nil || !called {
		t.Fatalf("ordinary source handler skipped: %v", err)
	}
	for _, other := range []string{"--data={}", "--stdin"} {
		called = false
		c := &cobra.Command{Use: "source"}
		c.Flags().String("data", "", "source data")
		c.Flags().Bool("stdin", false, "stdin body")
		c.RunE = func(cmd *cobra.Command, args []string) error { called = true; return nil }
		addTravelokaDataFile(c, &rootFlags{})
		c.SetArgs([]string{"--data-file", p, other})
		if err := c.Execute(); err == nil || called {
			t.Fatal("conflicting source inputs reached handler")
		}
	}
}

// A source preview must describe its operation without opening data/session files
// or falling through to the generated HTTP client sentinel.
func TestTravelokaAdvancedSourceDryRunNamesOperationBeforeIO(t *testing.T) {
	testenv.Isolate(t)
	cases := []struct {
		command []string
		path    string
	}{
		{[]string{"airport"}, "/api/v2/airport/search-nexus"},
		{[]string{"flight", "initial"}, "/api/v2/flight/search/initial"},
		{[]string{"flight", "poll"}, "/api/v2/flight/search/poll"},
		{[]string{"flight", "prefetch"}, "/api/v2/flight/search/redirection"},
		{[]string{"hotel", "lookup"}, "/api/v1/hotel/autocomplete"},
		{[]string{"hotel", "features"}, "/api/v2/hotel/autocomplete/features"},
		{[]string{"hotel", "catalog"}, "/api/v2/hotel/searchList"},
		{[]string{"hotel", "rooms"}, "/api/v2/hotel/search/rooms"},
	}
	for _, tt := range cases {
		t.Run(strings.Join(tt.command, "-"), func(t *testing.T) {
			argv := append(append([]string{}, tt.command...), "--dry-run", "--data-file", "/SIMULATED/missing-public.json", "--session-file", "/SIMULATED/missing-session.json")
			output, err := runTravelokaTest(t, argv...)
			action, _ := output["action"].(string)
			if err != nil || output["dry_run"] != true || action != "POST "+tt.path+" (read-only search)" {
				t.Fatalf("source preview lost operation or performed input/session IO: %v %v", output, err)
			}
		})
	}
}
