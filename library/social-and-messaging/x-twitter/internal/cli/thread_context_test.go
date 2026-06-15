// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

func TestThreadParticipantsHandlesOptionalSections(t *testing.T) {
	result := &threadContextResult{
		FocusTweet: &resolvedPostRecord{
			TweetID: "1",
			Author:  &postAuthorSummary{ID: "10", Username: "alice"},
		},
		Ancestors: []resolvedPostRecord{{
			TweetID: "0",
			Author:  &postAuthorSummary{ID: "11", Username: "bob"},
		}},
		Replies: []threadContextReply{{
			resolvedPostRecord: resolvedPostRecord{
				TweetID: "2",
				Author:  &postAuthorSummary{ID: "10", Username: "alice"},
			},
			InReplyTo: "1",
			Depth:     1,
		}},
	}

	participants := threadParticipants(result)
	if len(participants) != 2 {
		t.Fatalf("participants len = %d, want 2: %+v", len(participants), participants)
	}
	if participants[0].ID != "10" || participants[1].ID != "11" {
		t.Fatalf("participants = %+v", participants)
	}
}

func TestLoadLocalContextRepliesReadsTweetsTableAndSkipsSeen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-twitter.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	for _, raw := range []string{
		`{"id":"root","conversation_id":"root","created_at":"2026-01-01T00:00:00Z","text":"root"}`,
		`{"id":"parent","conversation_id":"root","created_at":"2026-01-01T00:01:00Z","text":"parent","referenced_tweets":[{"type":"replied_to","id":"root"}]}`,
		`{"id":"focus","conversation_id":"root","created_at":"2026-01-01T00:02:00Z","text":"focus","referenced_tweets":[{"type":"replied_to","id":"parent"}]}`,
		`{"id":"reply","conversation_id":"root","created_at":"2026-01-01T00:03:00Z","text":"reply","referenced_tweets":[{"type":"replied_to","id":"focus"}]}`,
	} {
		if err := db.UpsertTweets(json.RawMessage(raw)); err != nil {
			t.Fatalf("upsert tweet %s: %v", raw, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	focus := &resolvedPostRecord{Input: "focus", TweetID: "focus", ConversationID: "root"}
	replies, err := loadLocalContextReplies(cmd, focus, dbPath, parseIncludeSet("refs"), 10, map[string]bool{
		"root":   true,
		"parent": true,
		"focus":  true,
	})
	if err != nil {
		t.Fatalf("loadLocalContextReplies returned error: %v", err)
	}
	if len(replies) != 1 {
		t.Fatalf("replies len = %d, want 1: %+v", len(replies), replies)
	}
	if replies[0].TweetID != "reply" || replies[0].InReplyTo != "focus" {
		t.Fatalf("reply = %+v", replies[0])
	}
	if replies[0].Input != "reply" {
		t.Fatalf("reply Input = %q, want reply tweet ID", replies[0].Input)
	}
}

func TestAssignReplyDepthsComputesNestedDepthAndFilters(t *testing.T) {
	replies := []threadContextReply{
		{
			resolvedPostRecord: resolvedPostRecord{TweetID: "direct"},
			InReplyTo:          "focus",
		},
		{
			resolvedPostRecord: resolvedPostRecord{TweetID: "nested"},
			InReplyTo:          "direct",
		},
	}

	shallow := assignReplyDepths(append([]threadContextReply(nil), replies...), "focus", 1)
	if len(shallow) != 1 || shallow[0].TweetID != "direct" || shallow[0].Depth != 1 {
		t.Fatalf("shallow replies = %+v", shallow)
	}

	deep := assignReplyDepths(append([]threadContextReply(nil), replies...), "focus", 2)
	if len(deep) != 2 || deep[0].Depth != 1 || deep[1].Depth != 2 {
		t.Fatalf("deep replies = %+v", deep)
	}
}
