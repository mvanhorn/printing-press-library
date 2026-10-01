package cli

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/client"
)

type downloadTransport func(*http.Request) (*http.Response, error)

func (f downloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDocsDownloadWritesPDFBytes(t *testing.T) {
	t.Setenv("AVERUSA_HOME", t.TempDir())
	old := clientHooks
	t.Cleanup(func() { clientHooks = old })
	pdf := "%PDF-1.7\n\x00\xff binary payload"
	requests := 0
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient.Transport = downloadTransport(func(r *http.Request) (*http.Response, error) {
			requests++
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/pdf"}}, Body: io.NopCloser(strings.NewReader(pdf)), Request: r}, nil
		})
		return nil
	})
	cmd := newDocsDownloadCmd(&rootFlags{dataSource: "live", noCache: true, timeout: time.Second})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"article-id"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || output.String() != pdf {
		t.Fatalf("requests=%d output=%q", requests, output.String())
	}
}
