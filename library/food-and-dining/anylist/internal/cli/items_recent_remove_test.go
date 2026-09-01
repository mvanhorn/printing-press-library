package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func recentStoreFixture(chunks ...*pb.StarterList) *pb.PBUserDataResponse {
	batch := &pb.StarterListBatchResponse{}
	for _, list := range chunks {
		batch.ListResponses = append(batch.ListResponses, &pb.StarterListResponse{StarterList: list})
	}
	return &pb.PBUserDataResponse{
		StarterListsResponse: &pb.StarterListsResponseV2{RecentItemListsResponse: batch},
	}
}

func TestResolveRecentEntry_AmbiguousNameAcrossChunks(t *testing.T) {
	data := recentStoreFixture(
		&pb.StarterList{Identifier: "chunk-a", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-a", Name: "bell peppers"}}},
		&pb.StarterList{Identifier: "chunk-b", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-b", Name: "Bell Peppers"}}},
	)
	chunk, matches, err := resolveRecentEntry(data, "bell peppers", "")
	if err == nil {
		t.Fatalf("expected ambiguity error, got chunk %v", chunk)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(matches))
	}
	if !strings.Contains(err.Error(), "disambiguate") {
		t.Fatalf("error must point at the disambiguation path: %v", err)
	}
}

func TestResolveRecentEntry_FindsEntryInLaterChunk(t *testing.T) {
	data := recentStoreFixture(
		&pb.StarterList{Identifier: "chunk-1", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-1", Name: "milk"}}},
		&pb.StarterList{Identifier: "chunk-2", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-2", Name: "eggs"}}},
		&pb.StarterList{Identifier: "chunk-3", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-3", Name: "tortillas"}}},
	)
	chunk, _, err := resolveRecentEntry(data, "tortillas", "")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if chunk.list.GetIdentifier() != "chunk-3" || chunk.item.GetIdentifier() != "id-3" {
		t.Fatalf("resolved wrong entry: chunk=%s item=%s", chunk.list.GetIdentifier(), chunk.item.GetIdentifier())
	}
}

func TestResolveRecentEntry_ExactIDMatch(t *testing.T) {
	data := recentStoreFixture(
		&pb.StarterList{Identifier: "chunk-a", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-x", Name: "milk"}, {Identifier: "target", Name: "Milk"}}},
	)
	chunk, _, err := resolveRecentEntry(data, "target", "")
	if err != nil {
		t.Fatalf("exact ID must resolve even when a differently-cased name collides: %v", err)
	}
	if chunk.item.GetName() != "Milk" {
		t.Fatalf("resolved wrong item: %q", chunk.item.GetName())
	}
}

func TestResolveRecentEntry_ChunkSelectorDisambiguates(t *testing.T) {
	data := recentStoreFixture(
		&pb.StarterList{Identifier: "chunk-a", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-a", Name: "milk"}}},
		&pb.StarterList{Identifier: "chunk-b", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-b", Name: "milk"}}},
	)
	chunk, _, err := resolveRecentEntry(data, "milk", "chunk-b")
	if err != nil {
		t.Fatalf("chunk selector must disambiguate: %v", err)
	}
	if chunk.list.GetIdentifier() != "chunk-b" {
		t.Fatalf("resolved chunk %s, want chunk-b", chunk.list.GetIdentifier())
	}
	if _, _, err := resolveRecentEntry(data, "milk", "chunk-c"); err == nil {
		t.Fatal("unknown chunk selector with a matching name elsewhere must report no match")
	}
}

func TestVerifyRecentAbsence(t *testing.T) {
	// A removal that silently failed leaves the ID in several chunks. The
	// original verification treated resolveRecentEntry's ambiguity error as
	// absence and would have reported success; absence must be positive.
	multi := recentStoreFixture(
		&pb.StarterList{Identifier: "chunk-a", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "victim", Name: "milk"}}},
		&pb.StarterList{Identifier: "chunk-b", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "victim", Name: "milk"}}},
	)
	if err := verifyRecentAbsence(multi, "victim", "milk"); err == nil {
		t.Fatal("ID present in two chunks must fail verification")
	} else if !strings.Contains(err.Error(), "chunk-a") || !strings.Contains(err.Error(), "chunk-b") {
		t.Fatalf("error must name the surviving chunks: %v", err)
	}

	single := recentStoreFixture(
		&pb.StarterList{Identifier: "chunk-a", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "victim", Name: "milk"}}},
	)
	if err := verifyRecentAbsence(single, "victim", "milk"); err == nil {
		t.Fatal("ID present in one chunk must fail verification")
	}

	clean := recentStoreFixture(
		&pb.StarterList{Identifier: "chunk-a", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "other", Name: "milk"}}},
		&pb.StarterList{Identifier: "chunk-b", Name: "Recent Items"},
	)
	if err := verifyRecentAbsence(clean, "victim", "milk"); err != nil {
		t.Fatalf("genuinely absent ID must pass: %v", err)
	}

	// A read-back without any recent chunks is inconclusive, not verified.
	if err := verifyRecentAbsence(&pb.PBUserDataResponse{}, "victim", "milk"); err == nil {
		t.Fatal("read-back with zero chunks must fail verification")
	}
}

func TestResolveRecentEntry_EmptySelector(t *testing.T) {
	data := recentStoreFixture(
		&pb.StarterList{Identifier: "chunk-a", Name: "Recent Items", Items: []*pb.ListItem{{Identifier: "id-a", Name: "milk"}}},
	)
	if _, _, err := resolveRecentEntry(data, "   ", ""); err == nil {
		t.Fatal("blank selector must be rejected")
	}
	if _, _, err := resolveRecentEntry(data, "missing", ""); err == nil {
		t.Fatal("unknown selector must be rejected")
	}
}
