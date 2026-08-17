package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFindUnroutableTypes(t *testing.T) {
	tests := []struct {
		name   string
		types  map[string]wordpressPostType
		routes map[string]json.RawMessage
		want   []unroutableType
	}{
		{
			name: "respects custom namespaces",
			types: map[string]wordpressPostType{
				"post":         {RestBase: "posts", RestNamespace: "wp/v2"},
				"product":      {RestBase: "products", RestNamespace: "wc/v3"},
				"global_style": {RestBase: "global-styles", RestNamespace: "wp/v2"},
				"secret":       {RestBase: "secrets", RestNamespace: "wp/v2"},
			},
			routes: map[string]json.RawMessage{
				"/wp/v2/posts":    json.RawMessage(`{}`),
				"/wc/v3/products": json.RawMessage(`{}`),
				"/wp/v2/global-styles/(?P<id>[\\/\\w-]+)": json.RawMessage(`{}`),
			},
			want: []unroutableType{{Type: "secret", RestBase: "secrets", RestNamespace: "wp/v2"}},
		},
		{
			name:   "empty registry",
			types:  map[string]wordpressPostType{},
			routes: map[string]json.RawMessage{},
			want:   []unroutableType{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findUnroutableTypes(tt.types, tt.routes); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("findUnroutableTypes() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestFieldsNeverCarryingData(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]any
		samples    []map[string]any
		want       []string
	}{
		{
			name: "missing null and empty are empty while false and zero are data",
			properties: map[string]any{
				"id": map[string]any{}, "title": map[string]any{}, "excerpt": map[string]any{},
				"featured_media": map[string]any{}, "sticky": map[string]any{}, "missing": map[string]any{},
			},
			samples: []map[string]any{
				{"id": float64(0), "title": map[string]any{}, "excerpt": nil, "featured_media": "", "sticky": false},
				{"id": float64(1), "title": map[string]any{"rendered": ""}, "excerpt": nil, "featured_media": "", "sticky": false},
			},
			want: []string{"excerpt", "featured_media", "missing", "title"},
		},
		{
			name:       "no properties",
			properties: map[string]any{},
			samples:    []map[string]any{},
			want:       []string{},
		},
		{
			name:       "no rows is inconclusive",
			properties: map[string]any{"title": map[string]any{}},
			samples:    []map[string]any{},
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fieldsNeverCarryingData(tt.properties, tt.samples); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("fieldsNeverCarryingData() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMetaKeysNeverCarryingData(t *testing.T) {
	properties := map[string]any{
		"meta": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"rating": map[string]any{"type": "number"},
				"color":  map[string]any{"type": "string"},
			},
		},
	}
	tests := []struct {
		name    string
		samples []map[string]any
		want    []string
	}{
		{name: "one populated key", samples: []map[string]any{{"meta": map[string]any{"rating": float64(5), "color": ""}}}, want: []string{"color"}},
		{name: "empty meta object", samples: []map[string]any{{"meta": map[string]any{}}}, want: []string{"color", "rating"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := metaKeysNeverCarryingData(properties, tt.samples); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("metaKeysNeverCarryingData() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
