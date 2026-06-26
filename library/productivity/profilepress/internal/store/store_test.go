package store

import (
	"testing"

	"profilepress-pp-cli/internal/message"
	"profilepress-pp-cli/internal/packet"
	"profilepress-pp-cli/internal/profile"
)

func TestSnapshotPacketApplyLogAndMessageRoundTrip(t *testing.T) {
	s, err := Open(t.TempDir() + "/profilepress.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	snap, err := s.CreateSnapshot("fixture", []profile.Section{{Name: "headline", RawText: "old"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSnapshot(snap.ID); err != nil {
		t.Fatal(err)
	}

	pkt, err := s.CreatePacket(packet.Packet{SnapshotID: snap.ID, Opportunity: "opp", Changes: []packet.Change{{Section: "headline", Before: "old", After: "new"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPacket(pkt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddApplyLog(packet.ApplyLog{PacketID: pkt.ID, PrivacyStatus: "passed", SensitiveStatus: "confirmed", Result: "dry-run-passed", DryRun: true}); err != nil {
		t.Fatal(err)
	}

	draft, err := s.CreateMessageDraft(message.Draft{To: "https://www.linkedin.com/in/example", Body: "hello", SourceNote: "test"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMessageDraft(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "draft" || got.Body != "hello" {
		t.Fatalf("bad draft: %+v", got)
	}
	listed, err := s.ListMessageDrafts(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one draft, got %d", len(listed))
	}
	sent, err := s.MarkMessageSent(draft.ID, "simulate-live", "SEND-MESSAGE")
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != "sent" || sent.SendMode != "simulate-live" {
		t.Fatalf("bad sent draft: %+v", sent)
	}
}
