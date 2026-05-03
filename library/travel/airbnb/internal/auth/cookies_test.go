package auth

import "testing"

func TestParseCookieHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   []string
	}{
		{name: "happy path", header: "a=1; b=two; malformed; c=3", want: []string{"a=1", "b=two", "c=3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCookieHeader(tt.header)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i, ck := range got {
				if ck.Name+"="+ck.Value != tt.want[i] {
					t.Fatalf("cookie %d = %s=%s, want %s", i, ck.Name, ck.Value, tt.want[i])
				}
			}
		})
	}
}
