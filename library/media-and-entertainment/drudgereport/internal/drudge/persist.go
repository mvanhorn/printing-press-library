package drudge

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
)

// FetchAndPersist fetches Drudge content, parses stories, records a snapshot, and emits slot events.
func FetchAndPersist(ctx context.Context, db *sql.DB, htmlURL, rssURL string) (snapshotID string, stories []Story, events []SlotEvent, err error) {
	if htmlURL == "" {
		htmlURL = DefaultHTMLURL
	}
	if rssURL == "" {
		rssURL = DefaultRSSURL
	}
	if err := store.EnsureDrudgeSchema(ctx, db); err != nil {
		return "", nil, nil, fmt.Errorf("ensure drudge schema: %w", err)
	}

	var body []byte
	var htmlBody []byte
	var parseErr error
	body, err = FetchHTML(ctx, htmlURL)
	htmlBody = body
	if err == nil {
		stories, parseErr = ParseHTML(body)
		if parseErr != nil {
			err = fmt.Errorf("parse drudge HTML: %w", parseErr)
		}
	}
	sourceURL := htmlURL
	if err != nil || len(stories) == 0 {
		rssBody, rssErr := FetchRSS(ctx, rssURL)
		if rssErr != nil {
			if err != nil {
				return "", nil, nil, fmt.Errorf("fetch drudge HTML: %w; fetch drudge RSS: %w", err, rssErr)
			}
			return "", nil, nil, fmt.Errorf("fetch drudge RSS: %w", rssErr)
		}
		rssStories, rssParseErr := ParseRSS(rssBody)
		if rssParseErr != nil {
			if err != nil {
				return "", nil, nil, fmt.Errorf("fetch/parse drudge HTML: %w; parse drudge RSS: %w", err, rssParseErr)
			}
			return "", nil, nil, fmt.Errorf("parse drudge RSS: %w", rssParseErr)
		}
		if len(rssStories) == 0 {
			if err != nil {
				return "", nil, nil, fmt.Errorf("fetch/parse drudge HTML: %w; parse drudge RSS: no stories", err)
			}
			return "", nil, nil, fmt.Errorf("parse drudge RSS: no stories")
		}
		body = rssBody
		stories = rssStories
		sourceURL = rssURL
	}
	if len(stories) == 0 {
		return "", nil, nil, fmt.Errorf("parse drudge content: no stories")
	}

	capturedAt := time.Now().UTC()
	hashBody := htmlBody
	if len(hashBody) == 0 {
		hashBody = body
	}
	bodyHash := shortSHA1(hashBody)
	snapshotID = shortSHA1([]byte(bodyHash + "|" + capturedAt.Format(time.RFC3339Nano)))
	for i := range stories {
		stories[i].CapturedAt = stories[i].CapturedAt.UTC()
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, nil, fmt.Errorf("begin drudge snapshot transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	prior, err := loadPriorStories(ctx, tx)
	if err != nil {
		return "", nil, nil, err
	}
	events = computeEvents(snapshotID, stories, prior, capturedAt)

	if _, err = tx.ExecContext(ctx,
		`INSERT INTO drudge_snapshot (snapshot_id, captured_at, source_url, body_hash, story_count) VALUES (?, ?, ?, ?, ?)`,
		snapshotID, capturedAt.Format(time.RFC3339Nano), sourceURL, bodyHash, len(stories),
	); err != nil {
		return "", nil, nil, fmt.Errorf("insert drudge snapshot: %w", err)
	}
	for _, story := range stories {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO drudge_story (snapshot_id, story_id, title, url, slot, slot_index, is_red, has_image, image_url, outbound_domain, captured_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			snapshotID, story.StoryID, story.Title, story.URL, string(story.Slot), story.SlotIndex, boolInt(story.IsRed), boolInt(story.HasImage), nullableString(story.ImageURL), story.OutboundDomain, story.CapturedAt.Format(time.RFC3339Nano),
		); err != nil {
			return "", nil, nil, fmt.Errorf("insert drudge story %s: %w", story.StoryID, err)
		}
	}
	for _, event := range events {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO drudge_slot_event (event_id, snapshot_id, story_id, event_type, from_slot, to_slot, captured_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			event.EventID, event.SnapshotID, event.StoryID, string(event.EventType), nullableSlot(event.FromSlot), nullableSlot(event.ToSlot), event.CapturedAt.Format(time.RFC3339Nano),
		); err != nil {
			return "", nil, nil, fmt.Errorf("insert drudge slot event %s: %w", event.EventID, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return "", nil, nil, fmt.Errorf("commit drudge snapshot transaction: %w", err)
	}
	return snapshotID, stories, events, nil
}

type priorStory struct {
	slot  Slot
	isRed bool
}

func loadPriorStories(ctx context.Context, tx *sql.Tx) (map[string]priorStory, error) {
	var priorSnapshotID string
	err := tx.QueryRowContext(ctx, `SELECT snapshot_id FROM drudge_snapshot ORDER BY captured_at DESC LIMIT 1`).Scan(&priorSnapshotID)
	if err == sql.ErrNoRows {
		return map[string]priorStory{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load prior drudge snapshot: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `SELECT story_id, slot, is_red FROM drudge_story WHERE snapshot_id = ?`, priorSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("load prior drudge stories: %w", err)
	}
	defer rows.Close()

	prior := map[string]priorStory{}
	for rows.Next() {
		var storyID, slot string
		var isRed int
		if err := rows.Scan(&storyID, &slot, &isRed); err != nil {
			return nil, fmt.Errorf("scan prior drudge story: %w", err)
		}
		prior[storyID] = priorStory{slot: Slot(slot), isRed: isRed != 0}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prior drudge stories: %w", err)
	}
	return prior, nil
}

func computeEvents(snapshotID string, stories []Story, prior map[string]priorStory, capturedAt time.Time) []SlotEvent {
	current := map[string]Story{}
	var events []SlotEvent
	for _, story := range stories {
		current[story.StoryID] = story
		before, ok := prior[story.StoryID]
		if !ok {
			events = append(events, newEvent(snapshotID, story.StoryID, EventAppeared, "", story.Slot, capturedAt))
			continue
		}
		if before.slot != SlotSplash && story.Slot == SlotSplash {
			events = append(events, newEvent(snapshotID, story.StoryID, EventPromotedToSplash, before.slot, story.Slot, capturedAt))
		}
		if before.slot == SlotSplash && story.Slot != SlotSplash {
			events = append(events, newEvent(snapshotID, story.StoryID, EventDemotedFromSplash, before.slot, story.Slot, capturedAt))
		}
		if !before.isRed && story.IsRed {
			events = append(events, newEvent(snapshotID, story.StoryID, EventWentRed, before.slot, story.Slot, capturedAt))
		}
		if before.isRed && !story.IsRed {
			events = append(events, newEvent(snapshotID, story.StoryID, EventWentBlack, before.slot, story.Slot, capturedAt))
		}
	}
	for storyID, before := range prior {
		if _, ok := current[storyID]; !ok {
			events = append(events, newEvent(snapshotID, storyID, EventDisappeared, before.slot, "", capturedAt))
		}
	}
	return events
}

func newEvent(snapshotID, storyID string, eventType SlotEventType, fromSlot, toSlot Slot, capturedAt time.Time) SlotEvent {
	event := SlotEvent{
		StoryID:    storyID,
		EventType:  eventType,
		FromSlot:   fromSlot,
		ToSlot:     toSlot,
		CapturedAt: capturedAt.UTC(),
		SnapshotID: snapshotID,
	}
	event.EventID = shortSHA1([]byte(storyID + "|" + string(eventType) + "|" + string(fromSlot) + "|" + string(toSlot) + "|" + snapshotID))
	return event
}

func shortSHA1(body []byte) string {
	h := sha1.Sum(body)
	return hex.EncodeToString(h[:])[:16]
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}

func nullableSlot(v Slot) sql.NullString {
	return sql.NullString{String: string(v), Valid: v != ""}
}
