package cli

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/config"
	"github.com/spf13/cobra"
)

func TestKeywordsCleanIsHiddenFromMCPButStillAcceptsCLIFile(t *testing.T) {
	flags := &rootFlags{}
	cmd := newKeywordsCleanCmd(flags)
	if cmd.Annotations["mcp:hidden"] != "true" {
		t.Fatalf("mcp:hidden annotation = %q, want true", cmd.Annotations["mcp:hidden"])
	}

	input := t.TempDir() + "/keywords.txt"
	if err := os.WriteFile(input, []byte("STUMP_GRINDING\n!!!\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{input})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("keywords clean CLI execution failed: %v", err)
	}
	if got := stdout.String(); got != "stump grinding\n" {
		t.Fatalf("stdout = %q, want cleaned CLI file contents", got)
	}
}

func TestCallerSelectedFilesystemReadersAreHiddenFromMCP(t *testing.T) {
	root := RootCmd()
	paths := [][]string{
		{"keywords", "clean"},
		{"rank", "track"},
		{"cost", "estimate"},
		{"import"},
		{"ai-visibility", "track"},
		{"keywords", "volume"},
		{"task", "bundle"},
		{"profile"},
	}
	for _, path := range paths {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			cmd := root
			for _, name := range path {
				var next *cobra.Command
				for _, child := range cmd.Commands() {
					if child.Name() == name {
						next = child
						break
					}
				}
				if next == nil {
					t.Fatalf("command %q not found beneath %q", name, cmd.CommandPath())
				}
				cmd = next
			}
			if cmd.Annotations["mcp:hidden"] != "true" {
				t.Fatalf("%s mcp:hidden annotation = %q, want true", cmd.CommandPath(), cmd.Annotations["mcp:hidden"])
			}
		})
	}
}

func TestAuthSetTokenReadsPasswordFromStdin(t *testing.T) {
	t.Setenv("DATAFORSEO_LOGIN", "")
	t.Setenv("DATAFORSEO_PASSWORD", "")
	path := t.TempDir() + "/config.toml"
	flags := &rootFlags{configPath: path}
	cmd := newAuthSetTokenCmd(flags)
	cmd.SetArgs([]string{"account@example.com", "password-must-not-be-an-argument"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("set-token unexpectedly accepted a password argument")
	}

	cmd = newAuthSetTokenCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader("  secret-password-value  \r\n"))
	cmd.SetArgs([]string{"account@example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-token with stdin password failed: %v", err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DataforseoDocumentationUsername != "account@example.com" || loaded.DataforseoDocumentationPassword != "  secret-password-value  " {
		t.Fatalf("stored Basic credentials = %q / %q", loaded.DataforseoDocumentationUsername, loaded.DataforseoDocumentationPassword)
	}
}

func TestAuthSetTokenRejectsEmptyOrOversizedStdin(t *testing.T) {
	path := t.TempDir() + "/config.toml"
	for _, tt := range []struct {
		name  string
		input string
	}{
		{name: "empty", input: "\r\n"},
		{name: "oversized", input: strings.Repeat("x", maxPasswordInputBytes+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newAuthSetTokenCmd(&rootFlags{configPath: path})
			cmd.SetIn(strings.NewReader(tt.input))
			cmd.SetArgs([]string{"account@example.com"})
			if err := cmd.Execute(); err == nil {
				t.Fatal("set-token unexpectedly accepted invalid stdin")
			}
		})
	}
}

func TestSitemapClientAllowsPublicHTTPWithoutExternalNetwork(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<urlset><url><loc>https://example.com/tree-service/</loc></url></urlset>`)
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}

	client := newSafeSitemapClient(
		func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host != "public.example" {
				return nil, fmt.Errorf("unexpected host %q", host)
			}
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
		func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, backendURL.Host)
		},
	)
	client.Timeout = 2 * time.Second

	pairs, err := loadSitemapPairs(client, "http://public.example/sitemap.xml")
	if err != nil {
		t.Fatalf("public sitemap fetch failed: %v", err)
	}
	if len(pairs) != 1 || pairs[0].keyword != "tree service" {
		t.Fatalf("pairs = %#v", pairs)
	}
}

func TestSitemapClientBlocksNonPublicTargetsAndRedirects(t *testing.T) {
	tests := []struct {
		name string
		host string
		ip   string
	}{
		{name: "loopback", host: "localhost", ip: "127.0.0.1"},
		{name: "private", host: "private.example", ip: "10.0.0.7"},
		{name: "link local", host: "metadata.example", ip: "169.254.169.254"},
		{name: "reserved", host: "reserved.example", ip: "192.0.2.10"},
		{name: "ipv6 loopback", host: "v6.example", ip: "::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newSafeSitemapClient(
				func(context.Context, string) ([]net.IPAddr, error) {
					return []net.IPAddr{{IP: net.ParseIP(tt.ip)}}, nil
				},
				func(context.Context, string, string) (net.Conn, error) {
					t.Fatal("dial attempted for blocked target")
					return nil, nil
				},
			)
			_, err := loadSitemapPairs(client, "http://"+tt.host+"/sitemap.xml")
			if err == nil || !strings.Contains(err.Error(), "not publicly routable") {
				t.Fatalf("error = %v, want public-address rejection", err)
			}
		})
	}

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, "http://private.example/sitemap.xml", http.StatusFound)
	}))
	defer redirectServer.Close()
	redirectURL, err := url.Parse(redirectServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := newSafeSitemapClient(
		func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host == "public.example" {
				return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
			}
			return []net.IPAddr{{IP: net.ParseIP("10.0.0.7")}}, nil
		},
		func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, redirectURL.Host)
		},
	)
	_, err = loadSitemapPairs(client, "http://public.example/sitemap.xml")
	if err == nil || !strings.Contains(err.Error(), "not publicly routable") {
		t.Fatalf("redirect error = %v, want private redirect rejection", err)
	}
}

func TestSitemapClientRejectsUnsafeSchemes(t *testing.T) {
	client := newSafeSitemapClient(nil, nil)
	for _, target := range []string{"file:///etc/passwd", "ftp://example.com/sitemap.xml", "http://"} {
		if _, err := loadSitemapPairs(client, target); err == nil {
			t.Fatalf("loadSitemapPairs(%q) unexpectedly succeeded", target)
		}
	}
}
