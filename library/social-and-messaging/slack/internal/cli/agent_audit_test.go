// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

func TestSummarizeAuditLog(t *testing.T) {
	now := time.Now()
	entries := []store.AuditEntry{
		{ID: 3, TS: now.Add(-1 * time.Hour), Caller: "cron", Verb: "sync mirror", ChannelID: "D1", Detail: "im read"},
		{ID: 2, TS: now.Add(-2 * time.Hour), Caller: "cron", Verb: "dm-engagement", ChannelID: "D2", Detail: "dm volume"},
		{ID: 1, TS: now.Add(-200 * time.Hour), Caller: "agent-x", Verb: "unreads", ChannelID: "D3", Detail: "old read"},
	}

	// No cutoff — every entry kept.
	full := summarizeAuditLog(entries, time.Time{})
	if full.TotalReads != 3 {
		t.Errorf("TotalReads = %d, want 3", full.TotalReads)
	}
	if len(full.Callers) != 2 {
		t.Fatalf("expected 2 callers, got %d", len(full.Callers))
	}
	// cron has 2 reads, agent-x has 1 — cron ranks first.
	if full.Callers[0].Caller != "cron" || full.Callers[0].Reads != 2 {
		t.Errorf("top caller = %+v, want cron with 2 reads", full.Callers[0])
	}
	if len(full.Callers[0].Channels) != 2 {
		t.Errorf("cron channels = %v, want 2 distinct", full.Callers[0].Channels)
	}

	// Cutoff at 7 days drops the 200h-old entry.
	recent := summarizeAuditLog(entries, now.Add(-7*24*time.Hour))
	if recent.TotalReads != 2 {
		t.Errorf("windowed TotalReads = %d, want 2", recent.TotalReads)
	}
	if len(recent.Callers) != 1 {
		t.Errorf("windowed callers = %d, want 1 (cron only)", len(recent.Callers))
	}
}
