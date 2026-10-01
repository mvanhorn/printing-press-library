package cli

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEcboRefreshInvalidSelectPreservesSnapshot(t *testing.T) {
	requests := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		io.WriteString(w, `{"hits":{"total":0,"hits":[]}}`)
	}))
	defer srv.Close()
	previousTransport := http.DefaultTransport
	tr := srv.Client().Transport.(*http.Transport).Clone()
	tr.TLSClientConfig.ServerName = "example.com"
	tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	http.DefaultTransport = tr
	t.Cleanup(func() { http.DefaultTransport = previousTransport; tr.CloseIdleConnections() })
	cache := t.TempDir()
	path := filepath.Join(cache, "inventory.json")
	prior := []byte(`{"data":{"results":[]},"refreshed_at":"prior-observation"}`)
	if err := os.WriteFile(path, prior, 0600); err != nil {
		t.Fatal(err)
	}
	root := RootCmd()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"inventory", "refresh", "--lat", "35.6812", "--lon", "139.7671", "--cache-dir", cache, "--select", "unknown_field", "--agent"})
	err := root.Execute()
	if ExitCode(err) != 2 {
		t.Fatalf("expected usage error, got %v", err)
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, prior) {
		t.Fatalf("failed refresh replaced prior snapshot: %s, %v", current, err)
	}
	if requests != 0 {
		t.Fatalf("invalid projection made %d source requests", requests)
	}
}

func TestEcboFocusedOutputModesAreExplicit(t *testing.T) {
	for _, mode := range []string{"--quiet", "--csv", "--plain", "--compact"} {
		t.Run(mode, func(t *testing.T) {
			root := RootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"inventory", "list", "--cache-dir", t.TempDir(), mode})
			err := root.Execute()
			if ExitCode(err) != 2 || !strings.Contains(err.Error(), mode) || out.Len() != 0 {
				t.Fatalf("unsupported %s silently accepted: error=%v output=%s", mode, err, out.String())
			}
		})
	}
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"inventory", "list", "--cache-dir", t.TempDir(), "--agent"})
	if err := root.Execute(); err != nil || !strings.Contains(out.String(), `"results":[]`) {
		t.Fatalf("agent JSON failed: %v %s", err, out.String())
	}
}

type ecboCountReader struct {
	io.Reader
	read int
}

func (r *ecboCountReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func TestEcboSourceStdinBound(t *testing.T) {
	for _, makeCmd := range []struct {
		name  string
		price bool
	}{{"price", true}, {"validate", false}} {
		t.Run(makeCmd.name, func(t *testing.T) {
			body := `{"padding":"` + strings.Repeat("x", 3<<20) + `"}`
			stdin, err := os.CreateTemp(t.TempDir(), "stdin")
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			if _, err = stdin.WriteString(body); err != nil {
				t.Fatal(err)
			}
			stdin.Seek(0, io.SeekStart)
			old := os.Stdin
			os.Stdin = stdin
			defer func() { os.Stdin = old }()
			flags := &rootFlags{dryRun: true, asJSON: true}
			cmd := newSourceValidateCmd(flags)
			if makeCmd.price {
				cmd = newSourcePriceCmd(flags)
			}
			reader := &ecboCountReader{Reader: strings.NewReader(body)}
			cmd.SetIn(reader)
			cmd.SetOut(io.Discard)
			cmd.SetArgs([]string{"--stdin"})
			err = cmd.Execute()
			if ExitCode(err) != 2 || !strings.Contains(err.Error(), "2 MiB") {
				t.Fatalf("oversized stdin accepted: %v", err)
			}
			if reader.read != (2<<20)+1 {
				t.Fatalf("stdin read %d bytes; want bounded sentinel", reader.read)
			}
		})
	}
}
