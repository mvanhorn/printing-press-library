package cli

import "testing"

func TestMergeWordPressFields(t *testing.T) {
	tests := []struct {
		name   string
		fields string
		embed  bool
		want   string
	}{
		{name: "projection only", fields: "id,title,link", want: "id,title,link"},
		{name: "embed appends requirements", fields: "id,title,link", embed: true, want: "id,title,link,_links,_embedded"},
		{name: "does not duplicate", fields: "id,_links,_embedded", embed: true, want: "id,_links,_embedded"},
		{name: "trims and deduplicates", fields: " id, title, id ", want: "id,title"},
		{name: "empty explicit projection with embed", fields: "", embed: true, want: "_links,_embedded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeWordPressFields(tt.fields, tt.embed); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
