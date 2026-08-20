// Copyright 2026 ghltshubh and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertNovelDryRun runs a novel command in --dry-run mode and asserts it
// short-circuits cleanly without touching the network. It exercises the
// command's flag wiring and verify-friendly RunE shape.
func assertNovelDryRun(t *testing.T, args ...string) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(append(args, "--dry-run"))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	assert.NoError(t, cmd.Execute())
}

func TestParseBBox(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		want    []float64
		wantErr bool
	}{
		{name: "valid 4", in: "-122.5,37.7,-122.3,37.9", want: []float64{-122.5, 37.7, -122.3, 37.9}},
		{name: "valid 4 with spaces", in: " -122.5, 37.7 ,-122.3,37.9 ", want: []float64{-122.5, 37.7, -122.3, 37.9}},
		{name: "valid 6", in: "0,0,0,1,1,1", want: []float64{0, 0, 0, 1, 1, 1}},
		{name: "wrong arity 3", in: "1,2,3", wantErr: true},
		{name: "wrong arity 5", in: "1,2,3,4,5", wantErr: true},
		{name: "non-numeric", in: "a,b,c,d", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseBBox(tc.in)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBuildSearchBodyCloudQuery(t *testing.T) {
	t.Parallel()
	max := 20.0
	body := buildSearchBody([]string{"sentinel-2-l2a"}, []float64{-1, -2, 3, 4}, "2024-06-01/2024-06-30", &max, nil, 50, true)

	assert.Equal(t, []string{"sentinel-2-l2a"}, body["collections"])
	assert.Equal(t, []float64{-1, -2, 3, 4}, body["bbox"])
	assert.Equal(t, 50, body["limit"])
	// datetime is expanded to RFC3339.
	assert.Equal(t, "2024-06-01T00:00:00Z/2024-06-30T23:59:59Z", body["datetime"])

	// Cloud bound uses the query extension, not CQL2.
	query, ok := body["query"].(map[string]any)
	require.True(t, ok, "query should be present")
	cc, ok := query["eo:cloud_cover"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 20.0, cc["lt"])

	// Sortby uses the object form {field,direction}.
	sortby, ok := body["sortby"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, sortby, 1)
	assert.Equal(t, "properties.eo:cloud_cover", sortby[0]["field"])
	assert.Equal(t, "asc", sortby[0]["direction"])
}

func TestBuildSearchBodyNoFilters(t *testing.T) {
	t.Parallel()
	body := buildSearchBody([]string{"x"}, nil, "", nil, nil, 0, false)
	_, hasQuery := body["query"]
	_, hasSort := body["sortby"]
	_, hasLimit := body["limit"]
	assert.False(t, hasQuery, "no cloud bound → no query")
	assert.False(t, hasSort, "no sort requested → no sortby")
	assert.False(t, hasLimit, "limit 0 → omitted")
}

func TestNormalizeDatetime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"2024-06-01", "2024-06-01T00:00:00Z"},
		{"2024-06-01/2024-06-30", "2024-06-01T00:00:00Z/2024-06-30T23:59:59Z"},
		{"2024-06-01T12:00:00Z/2024-06-30T00:00:00Z", "2024-06-01T12:00:00Z/2024-06-30T00:00:00Z"},
		{"2024-06-01/..", "2024-06-01T00:00:00Z/.."},
		{"../2024-06-30", "../2024-06-30T23:59:59Z"},
		{"", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, normalizeDatetime(tc.in), tc.in)
	}
}

func TestResolveAssetsCOGOverJP2(t *testing.T) {
	t.Parallel()
	assets := map[string]json.RawMessage{
		"red":     json.RawMessage(`{"href":"https://x/B04.tif","type":"image/tiff","roles":["data","reflectance"]}`),
		"red-jp2": json.RawMessage(`{"href":"https://x/B04.jp2","type":"image/jp2","roles":["data"]}`),
		"nir":     json.RawMessage(`{"href":"https://x/B08.tif","type":"image/tiff","roles":["data"]}`),
	}

	// Requested band prefers the COG (non-jp2) key.
	got := resolveAssets(assets, []string{"red"}, "", false)
	require.Len(t, got, 1)
	assert.Equal(t, "red", got[0].Key)
	assert.Equal(t, "https://x/B04.tif", got[0].Href)

	// With includeJP2 the COG is still preferred when present.
	gotJP2 := resolveAssets(assets, []string{"red"}, "", true)
	require.Len(t, gotJP2, 1)
	assert.Equal(t, "red", gotJP2[0].Key)

	// Unknown band resolves to nothing.
	none := resolveAssets(assets, []string{"made-up"}, "", false)
	assert.Empty(t, none)

	// All-bands (empty list) skips -jp2 twins by default.
	all := resolveAssets(assets, nil, "", false)
	for _, a := range all {
		assert.NotContains(t, a.Key, "jp2")
	}
	assert.Len(t, all, 2)
}

func TestFeatureAccessors(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"id":"S2_X","collection":"sentinel-2-l2a",
		"properties":{"datetime":"2024-06-27T19:03:58Z","eo:cloud_cover":3.5,"sat:relative_orbit":42,"grid:code":"MGRS-10SEH","platform":"sentinel-2b"},
		"assets":{}
	}`)
	feats := parseFeatures([]json.RawMessage{raw})
	require.Len(t, feats, 1)
	f := feats[0]
	assert.Equal(t, "S2_X", f.ID)
	assert.Equal(t, "2024-06-27T19:03:58Z", f.datetime())
	assert.Equal(t, "2024-06-27", f.dateOnly())
	cloud, ok := f.cloud()
	assert.True(t, ok)
	assert.InDelta(t, 3.5, cloud, 1e-9)
	ro, ok := f.relOrbit()
	assert.True(t, ok)
	assert.Equal(t, 42, ro)
	assert.Equal(t, "MGRS-10SEH", f.gridCode())
	assert.Equal(t, "sentinel-2b", f.stringProp("platform"))
}
