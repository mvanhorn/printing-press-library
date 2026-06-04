// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package svc

import "testing"

// deckMsg builds one inner deck sub-message.
func deckMsg(id uint64, title string, up, down, mod, notes, audio, images int) []byte {
	var b []byte
	b = varintField(b, 1, id)
	b = bytesField(b, 2, []byte(title))
	b = varintField(b, 3, uint64(up))
	b = varintField(b, 4, uint64(down))
	b = varintField(b, 5, uint64(mod))
	b = varintField(b, 6, uint64(notes))
	if audio > 0 {
		b = varintField(b, 7, uint64(audio))
	}
	if images > 0 {
		b = varintField(b, 8, uint64(images))
	}
	return b
}

func TestDecodeListDecks(t *testing.T) {
	var payload []byte
	payload = bytesField(payload, 1, deckMsg(241428882, "Spanish Top 5000", 96, 3, 1700000000, 5000, 5000, 0))
	payload = bytesField(payload, 1, deckMsg(815543631, "Japanese Core", 153, 8, 1710000000, 2000, 2000, 100))

	decks, err := DecodeListDecks(payload)
	if err != nil {
		t.Fatalf("DecodeListDecks: %v", err)
	}
	if len(decks) != 2 {
		t.Fatalf("got %d decks, want 2", len(decks))
	}

	d0 := decks[0]
	if d0.ID != "241428882" {
		t.Errorf("d0.ID = %q", d0.ID)
	}
	if d0.Title != "Spanish Top 5000" {
		t.Errorf("d0.Title = %q", d0.Title)
	}
	if d0.Upvotes != 96 || d0.Downvotes != 3 {
		t.Errorf("d0 votes = %d/%d", d0.Upvotes, d0.Downvotes)
	}
	if d0.Notes != 5000 || d0.Audio != 5000 || d0.Images != 0 {
		t.Errorf("d0 counts = notes %d audio %d images %d", d0.Notes, d0.Audio, d0.Images)
	}
	// Validated against the website: Ratings == up+down (96+3=99, 153+8=161).
	if d0.TotalVotes() != 99 {
		t.Errorf("d0.TotalVotes = %d, want 99", d0.TotalVotes())
	}
	if decks[1].TotalVotes() != 161 {
		t.Errorf("d1.TotalVotes = %d, want 161", decks[1].TotalVotes())
	}
	if decks[1].Images != 100 {
		t.Errorf("d1.Images = %d, want 100", decks[1].Images)
	}
}

func TestApprovalRate(t *testing.T) {
	cases := []struct {
		name string
		deck SharedDeck
		want float64
	}{
		{"no votes", SharedDeck{Upvotes: 0, Downvotes: 0}, 0},
		{"all up", SharedDeck{Upvotes: 10, Downvotes: 0}, 1},
		{"96 of 99", SharedDeck{Upvotes: 96, Downvotes: 3}, 96.0 / 99.0},
		{"half", SharedDeck{Upvotes: 5, Downvotes: 5}, 0.5},
	}
	for _, c := range cases {
		if got := c.deck.ApprovalRate(); got != c.want {
			t.Errorf("%s: ApprovalRate = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDecodeItemInfo(t *testing.T) {
	// Build a review sub-message: ts varint @1, rating @2, text @4. Reviews are
	// the repeated top-level detail field #1.
	review := func(ts uint64, text string) []byte {
		var b []byte
		b = varintField(b, 1, ts)
		b = varintField(b, 2, 1)
		b = bytesField(b, 4, []byte(text))
		return b
	}
	// Counts sub-message: @1 notes, @2 audio, @3 images.
	var counts []byte
	counts = varintField(counts, 1, 5001)
	counts = varintField(counts, 2, 4881)
	counts = varintField(counts, 3, 5000)

	var detail []byte
	detail = bytesField(detail, 1, review(1700000000, "Great deck, very helpful!"))
	detail = bytesField(detail, 1, review(1700001000, "Lots of useful audio."))
	detail = bytesField(detail, 5, []byte("Spanish Top 5000 Vocabulary"))
	detail = bytesField(detail, 9, []byte("A long human-readable description of this Spanish deck for learners."))
	detail = varintField(detail, 8, 1507987557) // modified
	detail = bytesField(detail, 10, counts)
	detail = varintField(detail, 18, 480) // upvotes
	detail = varintField(detail, 19, 89)  // downvotes

	var payload []byte
	payload = bytesField(payload, 1, detail)

	info, err := DecodeItemInfo("241428882", payload)
	if err != nil {
		t.Fatalf("DecodeItemInfo: %v", err)
	}
	if info.ID != "241428882" {
		t.Errorf("ID = %q", info.ID)
	}
	if info.Title != "Spanish Top 5000 Vocabulary" {
		t.Errorf("Title = %q", info.Title)
	}
	if info.Description != "A long human-readable description of this Spanish deck for learners." {
		t.Errorf("Description = %q", info.Description)
	}
	if info.Notes != 5001 || info.Audio != 4881 || info.Images != 5000 {
		t.Errorf("counts = notes %d audio %d images %d", info.Notes, info.Audio, info.Images)
	}
	if info.Upvotes != 480 || info.Downvotes != 89 {
		t.Errorf("votes = %d/%d", info.Upvotes, info.Downvotes)
	}
	if info.ReviewCount != 2 {
		t.Errorf("ReviewCount = %d, want 2", info.ReviewCount)
	}
}

func TestDecodeDeckList(t *testing.T) {
	var deck []byte
	deck = varintField(deck, 1, 1234567890)
	deck = bytesField(deck, 2, []byte("Default"))
	deck = varintField(deck, 3, 42)

	var payload []byte
	payload = bytesField(payload, 1, deck)

	decks, err := DecodeDeckList(payload)
	if err != nil {
		t.Fatalf("DecodeDeckList: %v", err)
	}
	if len(decks) != 1 {
		t.Fatalf("got %d decks, want 1", len(decks))
	}
	if decks[0].Name != "Default" {
		t.Errorf("Name = %q", decks[0].Name)
	}
	if decks[0].ID != "1234567890" {
		t.Errorf("ID = %q", decks[0].ID)
	}
	found := false
	for _, c := range decks[0].Counts {
		if c == 42 {
			found = true
		}
	}
	if !found {
		t.Errorf("counts = %v, want to contain 42", decks[0].Counts)
	}
}

// TestDecodeDeckListNested mirrors the real AnkiWeb wire shape captured during
// browser-sniff: a top wrapper message (#1) whose children are repeated deck
// messages (#3), each {id varint #1, name string #2, card-state count varints}.
func TestDecodeDeckListNested(t *testing.T) {
	mkDeck := func(id uint64, name string, counts ...int) []byte {
		var d []byte
		d = varintField(d, 1, id)
		d = bytesField(d, 2, []byte(name))
		for i, c := range counts {
			d = varintField(d, 4+i, uint64(c))
		}
		return d
	}
	// wrapper #1 contains repeated #3 deck messages
	var wrapper []byte
	wrapper = bytesField(wrapper, 3, mkDeck(111, "Spanish", 1, 20, 34))
	wrapper = bytesField(wrapper, 3, mkDeck(222, "Anatomy", 2, 44))
	wrapper = bytesField(wrapper, 3, mkDeck(333, "Kanji", 4, 6))

	var payload []byte
	payload = bytesField(payload, 1, wrapper)

	decks, err := DecodeDeckList(payload)
	if err != nil {
		t.Fatalf("DecodeDeckList: %v", err)
	}
	if len(decks) != 3 {
		t.Fatalf("got %d decks, want 3 (nested wrapper not walked)", len(decks))
	}
	wantNames := map[string]string{"111": "Spanish", "222": "Anatomy", "333": "Kanji"}
	for _, d := range decks {
		if wantNames[d.ID] != d.Name {
			t.Errorf("deck id=%s name=%q, want %q", d.ID, d.Name, wantNames[d.ID])
		}
	}
}
