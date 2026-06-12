package store

import "testing"

// TestResolveMultilingual covers both TED v3 multilingual shapes: language
// values that are arrays (e.g. title-lot) and language values that are scalar
// strings (e.g. title-proc). Regression for the 2026-06-09 amend: scalar-string
// language values previously resolved to "" and were silently skipped.
func TestResolveMultilingual(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want string
	}{
		{
			name: "array language values (title-lot shape)",
			in:   map[string]interface{}{"nld": []interface{}{"EPV/2026/03/TP - 1"}},
			want: "EPV/2026/03/TP - 1",
		},
		{
			name: "scalar string language values (title-proc shape)",
			in:   map[string]interface{}{"nld": "Het tijdelijk plaatsen van verplaatsbare verticale wegsignalisatie"},
			want: "Het tijdelijk plaatsen van verplaatsbare verticale wegsignalisatie",
		},
		{
			name: "language preference order prefers eng over nld",
			in: map[string]interface{}{
				"nld": "Nederlandse titel",
				"eng": "English title",
			},
			want: "English title",
		},
		{
			name: "array empty string falls through to next language",
			in: map[string]interface{}{
				"eng": []interface{}{""},
				"nld": "Nederlandse titel",
			},
			want: "Nederlandse titel",
		},
		{
			name: "scalar empty string falls through to next language",
			in: map[string]interface{}{
				"eng": "",
				"nld": "Nederlandse titel",
			},
			want: "Nederlandse titel",
		},
		{
			name: "top-level array",
			in:   []interface{}{"plain array title"},
			want: "plain array title",
		},
		{
			name: "top-level string",
			in:   "plain string title",
			want: "plain string title",
		},
		{
			name: "nil",
			in:   nil,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveMultilingual(tc.in); got != tc.want {
				t.Errorf("ResolveMultilingual() = %q, want %q", got, tc.want)
			}
		})
	}
}
