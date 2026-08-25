// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"reflect"
	"testing"
)

func TestExtractMentions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		want Mentions
	}{
		{
			name: "no mentions",
			text: "shipping the deploy now",
			want: Mentions{Users: []string{}, Usergroups: []string{}, Broadcasts: []string{}},
		},
		{
			name: "bare user mention",
			text: "hey <@U0ALICE> can you look?",
			want: Mentions{Users: []string{"U0ALICE"}, Usergroups: []string{}, Broadcasts: []string{}},
		},
		{
			name: "labelled user mention",
			text: "<@U0BOB|bob> and <@U0ALICE|alice> both",
			want: Mentions{Users: []string{"U0ALICE", "U0BOB"}, Usergroups: []string{}, Broadcasts: []string{}},
		},
		{
			name: "duplicate user mention collapses",
			text: "<@U0ALICE> ping <@U0ALICE>",
			want: Mentions{Users: []string{"U0ALICE"}, Usergroups: []string{}, Broadcasts: []string{}},
		},
		{
			name: "usergroup mention",
			text: "<!subteam^SENG|@eng> please review",
			want: Mentions{Users: []string{}, Usergroups: []string{"SENG"}, Broadcasts: []string{}},
		},
		{
			name: "broadcast mention",
			text: "<!here> standup in 5",
			want: Mentions{Users: []string{}, Usergroups: []string{}, Broadcasts: []string{"here"}},
		},
		{
			name: "mixed",
			text: "<!channel> <@U0BOB> <!subteam^SOPS>",
			want: Mentions{Users: []string{"U0BOB"}, Usergroups: []string{"SOPS"}, Broadcasts: []string{"channel"}},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractMentions(tc.text)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ExtractMentions(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
		})
	}
}

func TestMentionsEmpty(t *testing.T) {
	t.Parallel()
	if !ExtractMentions("plain text").Empty() {
		t.Fatal("plain text should produce an empty mention set")
	}
	if ExtractMentions("<@U0ALICE>").Empty() {
		t.Fatal("a user mention should not be empty")
	}
}

func TestClassifyMention(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		text       string
		userID     string
		groups     []string
		broadcasts bool
		want       MentionKind
	}{
		{"direct", "ping <@U0ALICE> now", "U0ALICE", nil, false, MentionDirect},
		{"direct wins over group", "<@U0ALICE> <!subteam^SENG>", "U0ALICE", []string{"SENG"}, false, MentionDirect},
		{"usergroup member", "<!subteam^SENG> heads up", "U0ALICE", []string{"SENG"}, false, MentionUsergroup},
		{"usergroup non member", "<!subteam^SOPS> heads up", "U0ALICE", []string{"SENG"}, false, MentionNone},
		{"other person", "ping <@U0BOB>", "U0ALICE", nil, false, MentionNone},
		{"broadcast counted", "<!here> deploy done", "U0ALICE", nil, true, MentionBroadcast},
		{"broadcast ignored", "<!here> deploy done", "U0ALICE", nil, false, MentionNone},
		{"empty identity", "<@U0BOB>", "", nil, false, MentionNone},
		{"case insensitive id", "<@U0ALICE>", "u0alice", nil, false, MentionDirect},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyMention(tc.text, tc.userID, tc.groups, tc.broadcasts)
			if got != tc.want {
				t.Fatalf("ClassifyMention(%q, %q, %v, %v) = %q, want %q", tc.text, tc.userID, tc.groups, tc.broadcasts, got, tc.want)
			}
		})
	}
}
