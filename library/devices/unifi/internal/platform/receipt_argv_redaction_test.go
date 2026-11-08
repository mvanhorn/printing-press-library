package platform

import (
	"reflect"
	"testing"
)

// Receipts are audit records people share, so a secret typed on the command
// line must not survive into one, whichever flag form carried it.
func TestSensitiveArgvPositions(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want []int
	}{
		{"body value", []string{"cli", "raw", "POST", "p", "--body", `{"passphrase":"x"}`, "--confirm"}, []int{5}},
		{"equals form", []string{"cli", "raw", "POST", "p", `--body={"x_passphrase":"x"}`}, []int{4}},
		{"query pair", []string{"cli", "raw", "GET", "p", "--query", "key=abc"}, []int{5}},
		{"named secret flag", []string{"cli", "x", "--x-passphrase", "abc", "--api-key=abc", "--token", "abc"}, []int{3, 4, 6}},
		{"query string in positional path", []string{"cli", "raw", "GET", "network/api?key=abc"}, []int{3}},
		{"nothing sensitive", []string{"cli", "devices", "list", "--json", "--limit", "5", "--body-file", "change.json"}, nil},
		{"auth subcommand is not a flag", []string{"cli", "auth", "status"}, nil},
		{"trailing flag with no value", []string{"cli", "x", "--token"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SensitiveArgvPositions(tc.argv); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SensitiveArgvPositions(%q) = %v, want %v", tc.argv, got, tc.want)
			}
		})
	}
}
