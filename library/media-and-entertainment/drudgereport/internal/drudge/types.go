package drudge

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
)

// Slot identifies a Drudge Report editorial placement.
type Slot string

const (
	// SlotSplash is the main headline placement.
	SlotSplash Slot = "splash"
	// SlotTopLeft is the top-left headline placement.
	SlotTopLeft Slot = "top-left"
	// SlotColumn1 is the first headline column.
	SlotColumn1 Slot = "column1"
	// SlotColumn2 is the second headline column.
	SlotColumn2 Slot = "column2"
)

// SlotEventType identifies a story placement or styling transition.
type SlotEventType string

const (
	// EventAppeared means a story was not present in the prior snapshot.
	EventAppeared SlotEventType = "appeared"
	// EventDisappeared means a story was present in the prior snapshot but is absent now.
	EventDisappeared SlotEventType = "disappeared"
	// EventPromotedToSplash means a story moved into the splash slot.
	EventPromotedToSplash SlotEventType = "promoted_to_splash"
	// EventDemotedFromSplash means a story moved out of the splash slot.
	EventDemotedFromSplash SlotEventType = "demoted_from_splash"
	// EventWentRed means a story changed from black to red styling.
	EventWentRed SlotEventType = "went_red"
	// EventWentBlack means a story changed from red to black styling.
	EventWentBlack SlotEventType = "went_black"
)

// Story is a parsed Drudge Report headline with editorial placement metadata.
type Story struct {
	StoryID        string    `json:"story_id"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	Slot           Slot      `json:"slot"`
	SlotIndex      int       `json:"slot_index"`
	IsRed          bool      `json:"is_red"`
	HasImage       bool      `json:"has_image"`
	ImageURL       string    `json:"image_url,omitempty"`
	OutboundDomain string    `json:"outbound_domain"`
	CapturedAt     time.Time `json:"captured_at"`
}

// Snapshot is one captured Drudge Report page snapshot.
type Snapshot struct {
	SnapshotID string    `json:"snapshot_id"`
	CapturedAt time.Time `json:"captured_at"`
	SourceURL  string    `json:"source_url"`
	BodyHash   string    `json:"body_hash"`
	StoryCount int       `json:"story_count"`
}

// SlotEvent records a story transition between adjacent snapshots.
type SlotEvent struct {
	EventID    string        `json:"event_id"`
	StoryID    string        `json:"story_id"`
	EventType  SlotEventType `json:"event_type"`
	FromSlot   Slot          `json:"from_slot"`
	ToSlot     Slot          `json:"to_slot"`
	CapturedAt time.Time     `json:"captured_at"`
	SnapshotID string        `json:"snapshot_id"`
}

// SlotRank returns a composite editorial weight for a story placement.
func SlotRank(slot Slot, slotIndex int, isRed bool, hasImage bool) int {
	var rank int
	switch slot {
	case SlotSplash:
		rank = 100
	case SlotTopLeft:
		rank = 70
	case SlotColumn1, SlotColumn2:
		rank = 60 - slotIndex
		if rank < 40 {
			rank = 40
		}
	default:
		rank = 0
	}
	if isRed {
		rank += 25
	}
	if hasImage {
		rank += 5
	}
	return rank
}

// NormalizeTitle normalizes title text for stable story identity.
func NormalizeTitle(s string) string {
	trimmed := strings.TrimSpace(strings.ToLower(s))
	trimmed = strings.Join(strings.FieldsFunc(trimmed, unicode.IsSpace), " ")
	for strings.HasSuffix(trimmed, "...") {
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "..."))
	}
	for strings.HasSuffix(trimmed, "…") {
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "…"))
	}
	return trimmed
}

// StoryIDFromTitleURL returns a stable 16-character SHA-1 story identifier.
func StoryIDFromTitleURL(title, url string) string {
	h := sha1.Sum([]byte(NormalizeTitle(title) + "\x00" + strings.ToLower(strings.TrimSpace(url))))
	return hex.EncodeToString(h[:])[:16]
}
