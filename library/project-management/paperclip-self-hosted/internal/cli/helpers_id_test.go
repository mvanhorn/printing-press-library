package cli

import "testing"

func TestIsLikelyID(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{value: "550e8400-e29b-41d4-a716-446655440000", want: true},
		{value: "local-board", want: true},
		{value: "local-", want: false},
		{value: "__printing_press_invalid__", want: false},
		{value: "arbitrary", want: false},
		{value: "", want: false},
	}
	for _, tc := range cases {
		if got := isLikelyID(tc.value); got != tc.want {
			t.Errorf("isLikelyID(%q) = %t, want %t", tc.value, got, tc.want)
		}
	}
}
