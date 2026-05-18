// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/rss"
)

// hotFixtureXML mirrors the real frontpage feed structure: 3 items with
// thumbs 22 / 18 / 53. Tests can vary --min-thumbs and --limit and assert
// the right subset survives.
const hotFixtureXML = `<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>SD</title>
    <item>
      <title>Lavazza coffee deal</title>
      <link>https://slickdeals.net/f/19510173-lavazza?utm_source=rss</link>
      <description>coffee</description>
      <content:encoded><![CDATA[<div>Thumb Score: +22 </div><a data-store-slug="amazon">x</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:23:37 +0000</pubDate>
      <guid>thread-19510173</guid>
    </item>
    <item>
      <title>Costco desk</title>
      <link>https://slickdeals.net/f/19484037-desk?utm_source=rss</link>
      <description>desk</description>
      <content:encoded><![CDATA[<div>Thumb Score: +18 </div><a data-store-slug="costco-wholesale">x</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:21:19 +0000</pubDate>
      <guid>thread-19484037</guid>
    </item>
    <item>
      <title>ASHGOOB casters</title>
      <link>https://slickdeals.net/f/19465413-casters?utm_source=rss</link>
      <description>wheels</description>
      <content:encoded><![CDATA[<div>Thumb Score: +53 </div><a data-store-slug="amazon">x</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:10:48 +0000</pubDate>
      <guid>thread-19465413</guid>
    </item>
  </channel>
</rss>`

// hotPipeline reproduces what newHotCmd does internally, minus cobra wiring,
// so we can assert filter/sort/limit/wrap behavior end-to-end against a mock
// RSS server. The command's RunE delegates to rss.LiveHot; this helper does
// the same thing through FetchURL so we can point at httptest.
func hotPipeline(ctx context.Context, hc *http.Client, feedURL string, minThumbs, limit int) ([]rss.Item, error) {
	items, err := rss.FetchURL(ctx, feedURL, hc)
	if err != nil {
		return nil, err
	}
	return rss.FilterHot(items, minThumbs, limit), nil
}

func TestHot_FilterAndSort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(hotFixtureXML))
	}))
	defer server.Close()

	// min-thumbs=20: should keep +22 and +53, sorted DESC, limit 5
	got, err := hotPipeline(context.Background(), server.Client(), server.URL, 20, 5)
	if err != nil {
		t.Fatalf("hotPipeline: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d hot items want 2", len(got))
	}
	if got[0].Thumbs != 53 || got[1].Thumbs != 22 {
		t.Errorf("not sorted DESC: %+v", got)
	}
	if got[0].DealID != "19465413" {
		t.Errorf("top deal id = %q want 19465413", got[0].DealID)
	}
}

func TestHot_LimitTruncates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(hotFixtureXML))
	}))
	defer server.Close()

	got, err := hotPipeline(context.Background(), server.Client(), server.URL, 0, 1)
	if err != nil {
		t.Fatalf("hotPipeline: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d want 1 (limit enforced)", len(got))
	}
	if got[0].Thumbs != 53 {
		t.Errorf("limit 1 should keep top-thumbs; got %d", got[0].Thumbs)
	}
}

func TestHot_HighMinThumbsReturnsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(hotFixtureXML))
	}))
	defer server.Close()

	got, err := hotPipeline(context.Background(), server.Client(), server.URL, 1000, 25)
	if err != nil {
		t.Fatalf("hotPipeline: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d items, expected 0 (no deal meets min-thumbs=1000)", len(got))
	}
}

func TestHot_ConstructorRegisters(t *testing.T) {
	// Smoke: the constructor must produce a valid cobra command with the
	// promised flags and annotations. Catches drift if someone renames the
	// flag without thinking about the integration wiring.
	cmd := newHotCmd(&rootFlags{})
	if cmd.Use != "hot" {
		t.Errorf("Use = %q want hot", cmd.Use)
	}
	if cmd.Annotations["mcp:read-only"] != "true" {
		t.Errorf("missing mcp:read-only annotation")
	}
	if cmd.Flag("limit") == nil {
		t.Errorf("missing --limit flag")
	}
	if cmd.Flag("min-thumbs") == nil {
		t.Errorf("missing --min-thumbs flag")
	}
}

// TestHot_WrapShape verifies the JSON envelope wrapWithProvenance produces
// is parseable and carries the expected meta fields. This is the contract
// the integration command tree depends on.
func TestHot_WrapShape(t *testing.T) {
	items := []rss.Item{{DealID: "1", Thumbs: 50}}
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	prov := DataProvenance{Source: "live", Reason: "user_requested", ResourceType: "hot"}
	wrapped, err := wrapWithProvenance(raw, prov)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	var env struct {
		Results []rss.Item `json:"results"`
		Meta    struct {
			Source       string `json:"source"`
			ResourceType string `json:"resource_type"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(wrapped, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.Meta.Source != "live" || env.Meta.ResourceType != "hot" {
		t.Errorf("meta mismatch: %+v", env.Meta)
	}
	if len(env.Results) != 1 || env.Results[0].DealID != "1" {
		t.Errorf("results payload wrong: %+v", env.Results)
	}
}
