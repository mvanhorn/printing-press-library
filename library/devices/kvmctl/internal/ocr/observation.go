package ocr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	maxObservationTTL     = 60 * time.Second
	maxObservationEntries = 64
)

// Region is a bounded OCR word-level observation region.
type Region struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Box        [4]int  `json:"box"`
	Pixel      [2]int  `json:"pixel"`
}

// OCRObservation is the OCR evidence associated with a snapshot.
type OCRObservation struct {
	Engine  string   `json:"engine"`
	Regions []Region `json:"regions"`
}

// Observation is immutable, short-lived screen evidence. It contains no image
// bytes or credentials; ImageSHA256 binds it to the captured image.
type Observation struct {
	ID          string         `json:"observation_id"`
	CapturedAt  time.Time      `json:"captured_at"`
	ImageSHA256 string         `json:"image_sha256"`
	Width       int            `json:"width"`
	Height      int            `json:"height"`
	OCR         OCRObservation `json:"ocr"`
}

// NewObservation returns canonical SHA-256 identified OCR evidence.
func NewObservation(imageBytes []byte, capturedAt time.Time, engine string, width, height int, regions []Region) (Observation, error) {
	if len(imageBytes) == 0 || engine == "" || width <= 0 || height <= 0 {
		return Observation{}, errors.New("invalid OCR observation")
	}
	for _, region := range regions {
		x, y, w, h := region.Box[0], region.Box[1], region.Box[2], region.Box[3]
		if x < 0 || y < 0 || w <= 0 || h <= 0 || x+w > width || y+h > height || region.Pixel[0] < x || region.Pixel[0] > x+w || region.Pixel[1] < y || region.Pixel[1] > y+h {
			return Observation{}, errors.New("invalid OCR observation region")
		}
	}
	imageHash := sha256.Sum256(imageBytes)
	observation := Observation{
		CapturedAt:  capturedAt.UTC(),
		ImageSHA256: hex.EncodeToString(imageHash[:]),
		Width:       width,
		Height:      height,
		OCR: OCRObservation{
			Engine:  engine,
			Regions: cloneRegions(regions),
		},
	}
	// The identifier intentionally excludes CapturedAt: a fresh recapture of an
	// unchanged screen must validate the same observation ID, while timestamp
	// remains evidence metadata for callers.
	identity := struct {
		ImageSHA256 string         `json:"image_sha256"`
		Width       int            `json:"width"`
		Height      int            `json:"height"`
		OCR         OCRObservation `json:"ocr"`
	}{
		ImageSHA256: observation.ImageSHA256,
		Width:       observation.Width,
		Height:      observation.Height,
		OCR:         observation.OCR,
	}
	canonical, err := json.Marshal(identity)
	if err != nil {
		return Observation{}, fmt.Errorf("canonicalize OCR observation: %w", err)
	}
	idHash := sha256.Sum256(canonical)
	observation.ID = "sha256:" + hex.EncodeToString(idHash[:])
	return observation, nil
}

// ObservationStore keeps process-local OCR evidence. Its TTL never exceeds 60
// seconds and capacity never exceeds 64 entries.
type ObservationStore struct {
	mu       sync.Mutex
	ttl      time.Duration
	capacity int
	now      func() time.Time
	entries  map[string]storedObservation
	sequence uint64
}

type storedObservation struct {
	observation Observation
	expiresAt   time.Time
	sequence    uint64
}

// NewObservationStore creates a bounded in-memory store. Non-positive limits
// use safe defaults; larger requested limits are capped.
func NewObservationStore(ttl time.Duration, capacity int, now func() time.Time) *ObservationStore {
	if ttl <= 0 || ttl > maxObservationTTL {
		ttl = maxObservationTTL
	}
	if capacity <= 0 || capacity > maxObservationEntries {
		capacity = maxObservationEntries
	}
	if now == nil {
		now = time.Now
	}
	return &ObservationStore{ttl: ttl, capacity: capacity, now: now, entries: make(map[string]storedObservation)}
}

// Put stores a defensive copy unless the observation is already expired.
func (s *ObservationStore) Put(observation Observation) {
	if s == nil || observation.ID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.purgeExpired(now)
	expiresAt := observation.CapturedAt.Add(s.ttl)
	if !expiresAt.After(now) {
		return
	}
	s.sequence++
	s.entries[observation.ID] = storedObservation{observation: cloneObservation(observation), expiresAt: expiresAt, sequence: s.sequence}
	for len(s.entries) > s.capacity {
		var oldestID string
		var oldest uint64
		for id, entry := range s.entries {
			if oldestID == "" || entry.sequence < oldest {
				oldestID, oldest = id, entry.sequence
			}
		}
		delete(s.entries, oldestID)
	}
}

// Get returns a defensive copy only while the observation remains valid.
func (s *ObservationStore) Get(id string) (Observation, bool) {
	if s == nil || id == "" {
		return Observation{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.purgeExpired(now)
	entry, ok := s.entries[id]
	if !ok {
		return Observation{}, false
	}
	return cloneObservation(entry.observation), true
}

func (s *ObservationStore) purgeExpired(now time.Time) {
	for id, entry := range s.entries {
		if !entry.expiresAt.After(now) {
			delete(s.entries, id)
		}
	}
}

func cloneObservation(observation Observation) Observation {
	observation.OCR.Regions = cloneRegions(observation.OCR.Regions)
	return observation
}

func cloneRegions(regions []Region) []Region {
	if regions == nil {
		return nil
	}
	return append([]Region(nil), regions...)
}
