package cliutil

import "testing"

func TestCheckProposalBody(t *testing.T) {
	ok := []map[string]any{{"decision": "not_sure"}, {"decision": "different"}, {"decision": "merge"}}
	for _, b := range ok {
		if err := CheckProposalBody(b); err != nil {
			t.Errorf("%v: %v", b, err)
		}
	}
	bad := []map[string]any{{}, {"decision": "merged"}, {"decision": "Merge"}, {"decision": ""}, {"decision": 1}, {"decision": "merge", "x": 1}, {"other": "merge"}}
	for _, b := range bad {
		if err := CheckProposalBody(b); err == nil {
			t.Errorf("%v accepted", b)
		}
	}
}

func TestCheckUUID(t *testing.T) {
	if err := CheckUUID("id", "3f8a0c4e-1111-4222-8333-444455556666"); err != nil {
		t.Error(err)
	}
	for _, v := range []string{"", "../x", "3f8a0c4e-1111-4222-8333-44445555666", " 3f8a0c4e-1111-4222-8333-444455556666"} {
		if CheckUUID("id", v) == nil {
			t.Errorf("%q accepted", v)
		}
	}
}
