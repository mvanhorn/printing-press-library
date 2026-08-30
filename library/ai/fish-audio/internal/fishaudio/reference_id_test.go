package fishaudio

import "testing"

func TestValidReferenceID(t *testing.T) {
	cases := map[string]bool{"7f92f8afb8ec43bf81429cc1c9199cb1": true, "abc_-1": true, "": false, "<model_id>": false, "has space": false}
	for in, want := range cases {
		if got := ValidReferenceID(in); got != want {
			t.Errorf("ValidReferenceID(%q)=%v want %v", in, got, want)
		}
	}
}
