// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import "testing"

func TestParseUserRef(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		in        string
		wantKind  RefKind
		wantValue string
	}{
		{"empty", "  ", RefEmpty, ""},
		{"slack id", "U04AB9XYZ", RefID, "U04AB9XYZ"},
		{"lowercase slack id", "u04ab9xyz", RefID, "U04AB9XYZ"},
		{"enterprise id", "W123456", RefID, "W123456"},
		{"bot id", "B0EXAMPLE01", RefID, "B0EXAMPLE01"},
		{"handle", "alice", RefHandle, "alice"},
		{"at handle", "@Alice", RefHandle, "alice"},
		{"at prefixed id stays handle-matchable", "@U04AB9XYZ", RefHandle, "u04ab9xyz"},
		{"email", "Alice@Example.com", RefEmail, "alice@example.com"},
		{"mention", "<@U04AB9XYZ|alice>", RefID, "U04AB9XYZ"},
		{"channel id is not a user id", "C0GENERAL", RefHandle, "c0general"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ParseUserRef(tc.in)
			if got.Kind != tc.wantKind || got.Value != tc.wantValue {
				t.Fatalf("ParseUserRef(%q) = {%q %q}, want {%q %q}", tc.in, got.Kind, got.Value, tc.wantKind, tc.wantValue)
			}
		})
	}
}

func TestUserIdentityMatches(t *testing.T) {
	t.Parallel()
	alice := UserIdentity{
		ID:          "U0ALICE",
		Handle:      "alice",
		DisplayName: "Alice A",
		RealName:    "Alice Adams",
		Email:       "alice@example.com",
	}
	cases := []struct {
		name string
		ref  string
		want bool
	}{
		{"exact id", "U0ALICE", true},
		{"lowercase id", "u0alice", true},
		{"at handle", "@alice", true},
		{"bare handle", "alice", true},
		{"display name", "Alice A", true},
		{"real name", "alice adams", true},
		{"email", "ALICE@example.com", true},
		{"at prefixed id", "@U0ALICE", true},
		{"other person", "bob", false},
		{"other email", "bob@example.com", false},
		{"other id", "U0BOB", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := alice.Matches(ParseUserRef(tc.ref)); got != tc.want {
				t.Fatalf("alice.Matches(ParseUserRef(%q)) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestUserIdentityMatchesEmptyFields(t *testing.T) {
	t.Parallel()
	sparse := UserIdentity{ID: "U0GHOST"}
	if sparse.Matches(ParseUserRef("alice@example.com")) {
		t.Fatal("a user with no email must not match an email reference")
	}
	if !sparse.Matches(ParseUserRef("U0GHOST")) {
		t.Fatal("a user with only an ID must still match its ID")
	}
}

func TestUserIdentityDisplayLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   UserIdentity
		want string
	}{
		{"prefers display name", UserIdentity{ID: "U1", Handle: "a", RealName: "A B", DisplayName: "Ace"}, "Ace"},
		{"falls back to real name", UserIdentity{ID: "U1", Handle: "a", RealName: "A B"}, "A B"},
		{"falls back to handle", UserIdentity{ID: "U1", Handle: "a"}, "a"},
		{"falls back to id", UserIdentity{ID: "U1"}, "U1"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.DisplayLabel(); got != tc.want {
				t.Fatalf("DisplayLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}
