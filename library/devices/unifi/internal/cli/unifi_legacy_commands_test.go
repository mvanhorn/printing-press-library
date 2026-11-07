package cli

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// Scripts written against the earlier release must keep working after the commands moved,
// including with global flags in front and the arguments and flags carried across.
func TestLegacyCommandsRewriteToTheirNewNames(t *testing.T) {
	root := &cobra.Command{Use: "unifi-pp-cli"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().String("console", "", "")
	root.AddCommand(&cobra.Command{Use: "sites"})
	addLegacyCommands(root)
	cases := []struct {
		in, want []string
	}{
		{[]string{"sites", "clients", "get-connected-overview-page", "SITE", "--limit", "5"}, []string{"network", "list-clients", "SITE", "--limit", "5"}},
		{[]string{"--json", "--console", "C1", "sites", "firewall", "get-policy", "S", "P"}, []string{"--json", "--console", "C1", "network", "get-firewall-policy", "S", "P"}},
		{[]string{"countries", "--all"}, []string{"network", "list-countries", "--all"}},
		{[]string{"dpi", "get-applications"}, []string{"network", "list-dpi-applications"}},
		{[]string{"info"}, []string{"network", "get-info"}},
		{[]string{"pending-devices"}, []string{"network", "list-pending-devices"}},
		{[]string{"sites", "hotspot", "delete-voucher", "S", "V"}, []string{"network", "delete-hotspot-voucher", "S", "V"}},
		{[]string{"sites", "hotspot", "delete-vouchers", "S"}, []string{"network", "delete-hotspot-vouchers", "S"}},
	}
	for _, tc := range cases {
		got, note := rewriteLegacyArgs(root, tc.in)
		if !reflect.DeepEqual(got, tc.want) || note == "" {
			t.Errorf("rewriteLegacyArgs(%q) = %q, note %q; want %q", tc.in, got, note, tc.want)
		}
	}
}

// A current command, or a value that merely looks like an old name, must pass through untouched.
func TestCurrentCommandsAreNotRewritten(t *testing.T) {
	root := &cobra.Command{Use: "unifi-pp-cli"}
	root.PersistentFlags().String("console", "", "")
	root.AddCommand(&cobra.Command{Use: "sites"}, &cobra.Command{Use: "hosts"})
	addLegacyCommands(root)
	for _, args := range [][]string{{"hosts", "list"}, {"sites"}, {"--console", "info", "hosts", "list"}, {"network", "get-info"}} {
		got, note := rewriteLegacyArgs(root, args)
		if !reflect.DeepEqual(got, args) || note != "" {
			t.Errorf("%q changed to %q (%q)", args, got, note)
		}
	}
}

func TestRemovedCommandsSayWhatReplacesThem(t *testing.T) {
	for _, args := range [][]string{{"port-audit"}, {"guest", "report"}} {
		if _, _, err := legacyInvocation(args); err == nil || !strings.Contains(err.Error(), "network ") {
			t.Errorf("%v: err=%v", args, err)
		}
	}
}

// Dogfood and the MCP server walk the real tree, so an alias there becomes a command they
// try to exercise. This failed a live gate with 72 false failures.
func TestLegacyNamesStayOutOfTheRealCommandTree(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	for _, name := range []string{"countries", "info", "dpi", "guest", "port-audit", "pending-devices"} {
		for _, child := range root.Commands() {
			if child.Name() == name {
				t.Errorf("%q is in the real tree", name)
			}
		}
	}
	sites, _, err := root.Find([]string{"sites"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sites.Commands()) != 0 {
		t.Errorf("sites has subcommands: %d", len(sites.Commands()))
	}
}

// End to end through the probe, since the real tree no longer holds these names.
func TestLegacyInvocationRewritesOnTheRealRoot(t *testing.T) {
	got, note, err := legacyInvocation([]string{"--json", "sites", "clients", "get-connected-overview-page", "S"})
	want := []string{"--json", "network", "list-clients", "S"}
	if err != nil || note == "" || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q note %q err %v", got, note, err)
	}
	for _, args := range [][]string{{"sites"}, {"hosts", "list"}, {"network", "get-info"}, {"--console", "info", "hosts", "list"}} {
		if got, note, err := legacyInvocation(args); err != nil || note != "" || !reflect.DeepEqual(got, args) {
			t.Errorf("%q changed to %q note %q err %v", args, got, note, err)
		}
	}
}

// Old LAN setups must reach the gateway through its self-signed certificate, with the
// key in the header and the product once in the path. Unit tests of the URL cannot show this.
func TestLegacyLANSettingsReachASelfSignedGateway(t *testing.T) {
	var mu sync.Mutex
	var paths, keys []string
	gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths, keys = append(paths, r.URL.Path), append(keys, r.Header.Get("X-API-KEY"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"offset":0,"limit":25,"count":0,"totalCount":0}`))
	}))
	defer gateway.Close()
	check := func(t *testing.T) {
		t.Helper()
		run := runUniFiCLI(t, "network", "list-sites", "--json")
		mu.Lock()
		defer mu.Unlock()
		if run.err != nil || len(paths) == 0 {
			t.Fatalf("err=%v stderr=%q paths=%v", run.err, run.stderr, paths)
		}
		if paths[0] != "/proxy/network/integration/v1/sites" || keys[0] != "legacy-key" {
			t.Fatalf("path %q key sent %v", paths[0], keys[0] == "legacy-key")
		}
		paths, keys = nil, nil
	}
	t.Run("UNIFI_GATEWAY_HOST", func(t *testing.T) {
		fixtureEnv(t, "https://api.ui.com")
		t.Setenv("UNIFI_GATEWAY_HOST", strings.TrimPrefix(gateway.URL, "https://"))
		t.Setenv("UNIFI_API_KEY", "legacy-key")
		check(t)
	})
	t.Run("saved base_url", func(t *testing.T) {
		fixtureEnv(t, gateway.URL+"/proxy/network")
		t.Setenv("UNIFI_API_KEY", "legacy-key")
		check(t)
	})
}
