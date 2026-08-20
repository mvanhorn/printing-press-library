// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package svc

import "strconv"

// SharedDeck is one entry from the public shared-deck catalog
// (/svc/shared/list-decks). Field numbers are validated against the live
// website's Ratings column (upvotes+downvotes).
type SharedDeck struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Upvotes   int    `json:"upvotes"`
	Downvotes int    `json:"downvotes"`
	Modified  int    `json:"modified"` // unix seconds
	Notes     int    `json:"notes"`
	Audio     int    `json:"audio"`
	Images    int    `json:"images"`
}

// ApprovalRate returns upvotes/(upvotes+downvotes), or 0 when there are no
// votes. Range 0..1.
func (d SharedDeck) ApprovalRate() float64 {
	total := d.Upvotes + d.Downvotes
	if total == 0 {
		return 0
	}
	return float64(d.Upvotes) / float64(total)
}

// TotalVotes returns upvotes+downvotes — the website's "Ratings" column.
func (d SharedDeck) TotalVotes() int { return d.Upvotes + d.Downvotes }

// SharedDeckInfo is the best-effort detail view of one shared deck
// (/svc/shared/item-info). The exact field layout is only partially mapped, so
// description is extracted heuristically and review_count counts the repeated
// review sub-messages.
type SharedDeckInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Upvotes     int    `json:"upvotes,omitempty"`
	Downvotes   int    `json:"downvotes,omitempty"`
	Notes       int    `json:"notes,omitempty"`
	Audio       int    `json:"audio,omitempty"`
	Images      int    `json:"images,omitempty"`
	Modified    int    `json:"modified,omitempty"`
	ReviewCount int    `json:"review_count"`
}

// MyDeck is one of the logged-in user's synced decks
// (/svc/decks/deck-list-info). Field semantics beyond name are ambiguous in
// the protobuf, so any identified numbers are exposed under generic keys.
type MyDeck struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Counts []int  `json:"counts,omitempty"`
}

// DecodeListDecks parses a /svc/shared/list-decks response. The body is a
// sequence of top-level field #1 (length-delimited) entries, one per deck.
func DecodeListDecks(buf []byte) ([]SharedDeck, error) {
	top, err := Fields(buf)
	// Best-effort: a trailing truncation should still yield the decks parsed
	// so far, so we keep going with whatever Fields returned.
	decks := make([]SharedDeck, 0, len(top))
	for _, f := range top {
		if f.Num != 1 || f.WireType != wireBytes {
			continue
		}
		sub, subErr := Fields(f.Bytes)
		if subErr != nil && len(sub) == 0 {
			continue
		}
		d := SharedDeck{
			ID:        strconv.FormatUint(FirstVarint(sub, 1), 10),
			Title:     FirstString(sub, 2),
			Upvotes:   int(FirstVarint(sub, 3)),
			Downvotes: int(FirstVarint(sub, 4)),
			Modified:  int(FirstVarint(sub, 5)),
			Notes:     int(FirstVarint(sub, 6)),
			Audio:     int(FirstVarint(sub, 7)),
			Images:    int(FirstVarint(sub, 8)),
		}
		decks = append(decks, d)
	}
	if len(decks) == 0 && err != nil {
		return nil, err
	}
	return decks, nil
}

// DecodeItemInfo parses a /svc/shared/item-info response. Validated field map
// (detail is wrapped in top-level field #1):
//
//	#1  repeated  review sub-message (ts @1, rating @2, text @4)
//	#5  string    title
//	#6  string    tags / language
//	#8  varint    modified (unix seconds)
//	#9  string    description
//	#10 message    counts: @1 notes, @2 audio, @3 images
//	#18 varint    upvotes
//	#19 varint    downvotes
//
// Decoding is best-effort: a field we cannot map is skipped rather than
// guessed.
func DecodeItemInfo(id string, buf []byte) (SharedDeckInfo, error) {
	info := SharedDeckInfo{ID: id}
	top, err := Fields(buf)
	if len(top) == 0 {
		return info, err
	}

	// The detail is wrapped in field #1; fall back to the top level if no such
	// wrapper is present.
	detail := top
	for _, f := range top {
		if f.Num == 1 && f.WireType == wireBytes {
			if sub, e := Fields(f.Bytes); (e == nil || len(sub) > 0) && len(sub) > 0 {
				detail = sub
			}
			break
		}
	}

	info.Title = FirstString(detail, 5)
	info.Description = FirstString(detail, 9)
	info.Modified = int(FirstVarint(detail, 8))
	info.Upvotes = int(FirstVarint(detail, 18))
	info.Downvotes = int(FirstVarint(detail, 19))

	// Counts live in sub-message #10: @1 notes, @2 audio, @3 images.
	for _, cf := range CollectBytes(detail, 10) {
		counts, e := Fields(cf)
		if e != nil && len(counts) == 0 {
			continue
		}
		info.Notes = int(FirstVarint(counts, 1))
		info.Audio = int(FirstVarint(counts, 2))
		info.Images = int(FirstVarint(counts, 3))
		break
	}

	// Reviews are the repeated field #1 sub-messages (ts varint @1, text @4).
	for _, rb := range CollectBytes(detail, 1) {
		sub, e := Fields(rb)
		if e != nil && len(sub) == 0 {
			continue
		}
		if FirstVarint(sub, 1) > 0 {
			info.ReviewCount++
		}
	}

	return info, nil
}

// DecodeDeckList parses a /svc/decks/deck-list-info response best-effort. The
// protobuf's exact layout is ambiguous, so each top-level length-delimited
// field is treated as one deck: the first text string becomes the name and any
// varints become generic counts.
func DecodeDeckList(buf []byte) ([]MyDeck, error) {
	top, err := Fields(buf)
	if len(top) == 0 {
		return nil, err
	}
	// AnkiWeb wraps the deck list in nesting layers: the real shape is a top
	// wrapper message (#1) whose children are the repeated deck messages (#3),
	// each of which holds an id varint + a name string + per-state card-count
	// varints. The exact depth is not contractual, so walk recursively: a fields
	// set that "looks like a deck" (a text field plus at least one varint) is
	// extracted; anything else is descended into. This degrades correctly for a
	// flat list too.
	var decks []MyDeck
	var walk func(fields []Field, depth int)
	walk = func(fields []Field, depth int) {
		if depth > 6 {
			return
		}
		if d, ok := fieldsAsDeck(fields); ok {
			decks = append(decks, d)
			return
		}
		for _, f := range fields {
			if f.WireType != wireBytes {
				continue
			}
			sub, e := Fields(f.Bytes)
			if e != nil || len(sub) == 0 {
				continue
			}
			walk(sub, depth+1)
		}
	}
	walk(top, 0)
	return decks, nil
}

// fieldsAsDeck reports whether a decoded message looks like a single AnkiWeb
// deck record and, if so, extracts it. A deck has a human-readable name string
// and at least one varint (the id and/or card-state counts).
func fieldsAsDeck(fields []Field) (MyDeck, bool) {
	var d MyDeck
	var sawText, sawVarint bool
	for _, f := range fields {
		switch f.WireType {
		case wireBytes:
			if !sawText && isMostlyText(string(f.Bytes)) {
				d.Name = string(f.Bytes)
				sawText = true
			}
		case wireVarint:
			sawVarint = true
			if f.Num == 1 && d.ID == "" {
				d.ID = strconv.FormatUint(f.Varint, 10)
			} else {
				d.Counts = append(d.Counts, int(f.Varint))
			}
		}
	}
	if sawText && sawVarint {
		return d, true
	}
	return MyDeck{}, false
}
