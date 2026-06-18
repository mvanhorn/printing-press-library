package socketio

import (
	"testing"
)

func TestWSURLFromGraphQL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://affine.example.com/graphql", "https://affine.example.com"},
		{"http://localhost:3010/graphql", "http://localhost:3010"},
		{"https://affine.selfhosted.example/graphql", "https://affine.selfhosted.example"},
		{"http://localhost:3010/graphql/", "http://localhost:3010"},
	}
	for _, tt := range tests {
		got := WSURLFromGraphQL(tt.input)
		if got != tt.want {
			t.Errorf("WSURLFromGraphQL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
