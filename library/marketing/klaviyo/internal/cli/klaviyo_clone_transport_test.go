// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
)

type klaviyoCloneTransport func(*http.Request) (*http.Response, error)

func (f klaviyoCloneTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestKlaviyoTemplateCloneLegacyFlagsOnFreshFramework(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("KLAVIYO_API_KEY", "synthetic")
	previousSource, previousHooks := registeredPlatformSource, clientHooks
	registeredPlatformSource = nil
	t.Cleanup(func() { registeredPlatformSource, clientHooks = previousSource, previousHooks })
	calls := 0
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient.Transport = klaviyoCloneTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Header.Get("revision") != "2026-10-15" {
				t.Fatal("wrong revision")
			}
			body := `{"data":{"id":"source","attributes":{"html":"{% unsubscribe %}"}}}`
			switch r.Method + " " + r.URL.Path {
			case "GET /api/templates/source":
				if calls != 1 {
					t.Fatal("source must be read before cloning")
				}
			case "POST /api/template-clone":
				var posted map[string]any
				if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
					t.Fatal(err)
				}
				want := map[string]any{"data": map[string]any{"type": "template", "id": "source", "attributes": map[string]any{"name": "Copy"}}}
				if !reflect.DeepEqual(posted, want) {
					t.Fatalf("clone body=%#v, want %#v", posted, want)
				}
			default:
				t.Fatalf("unexpected request %s %s", r.Method, r.URL)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})
		return nil
	})
	cmd := newTemplateClonePromotedCmd(&rootFlags{asJSON: true, noCache: true})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--template-id", "source", "--name", "Copy"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("requests=%d, want source read and clone", calls)
	}
}
