package client

import "testing"

func TestCreditACLPrivate(t *testing.T) {
	trusted := creditACLRule{allow: true, trusted: true}
	untrusted := creditACLRule{allow: true}
	cases := []struct {
		name  string
		owner bool
		dacl  bool
		rules []creditACLRule
		want  bool
	}{
		{"owner system administrators", true, true, []creditACLRule{trusted, trusted, trusted}, true},
		{"private owner only", true, true, []creditACLRule{trusted}, true},
		{"broad everyone or users allow", true, true, []creditACLRule{trusted, untrusted}, false},
		{"deny broad user", true, true, []creditACLRule{{deny: true}, trusted}, true},
		{"unknown ace", true, true, []creditACLRule{{}}, false},
		{"wrong owner", false, true, []creditACLRule{trusted}, false},
		{"null dacl", true, false, []creditACLRule{trusted}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := creditACLPrivate(tc.owner, tc.dacl, tc.rules)
			if (err == nil) != tc.want {
				t.Fatalf("private=%t, want %t (error=%v)", err == nil, tc.want, err)
			}
		})
	}
}
