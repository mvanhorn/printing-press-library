package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/store"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

// openYTDB opens the local store with the YouTube-specific schema applied.
func openYTDB(ctx context.Context, flags *rootFlags, customPath string) (*store.Store, error) {
	path := customPath
	if path == "" {
		path = defaultDBPath("yt-studio-pp-cli")
	}
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("opening store at %s: %w", path, err)
	}
	if err := ytstore.EnsureSchema(ctx, s.DB()); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("ensuring yt schema: %w", err)
	}
	return s, nil
}

// loadOAuthToken reads the access token from config and surfaces a typed
// auth error if it's missing or expired.
func loadOAuthToken(flags *rootFlags) (string, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return "", fmt.Errorf("loading config: %w", err)
	}
	if cfg.AccessToken == "" {
		return "", authErr(fmt.Errorf("no OAuth access token; run `yt-studio-pp-cli auth login`"))
	}
	return cfg.AccessToken, nil
}

// ensureDB returns the *sql.DB and a close func from openYTDB.
func ensureDB(ctx context.Context, flags *rootFlags, customPath string) (*sql.DB, func(), error) {
	s, err := openYTDB(ctx, flags, customPath)
	if err != nil {
		return nil, nil, err
	}
	return s.DB(), func() { _ = s.Close() }, nil
}

// asciiSparkline renders a 100-bucket retention curve as a fixed-width ASCII chart.
// Width defaults to 80 cols; height is 8 rows.
func asciiSparkline(points []float64, width int) string {
	if width <= 0 {
		width = 80
	}
	if len(points) == 0 {
		return "(empty)"
	}
	// Down-sample to width buckets
	buckets := make([]float64, width)
	per := float64(len(points)) / float64(width)
	for i := 0; i < width; i++ {
		start := int(float64(i) * per)
		end := int(float64(i+1) * per)
		if end > len(points) {
			end = len(points)
		}
		sum := 0.0
		n := 0
		for j := start; j < end; j++ {
			sum += points[j]
			n++
		}
		if n > 0 {
			buckets[i] = sum / float64(n)
		}
	}
	// Find max for normalization
	max := buckets[0]
	for _, v := range buckets {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}
	// 8 levels using unicode block chars
	levels := []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	out := make([]rune, width)
	for i, v := range buckets {
		idx := int((v / max) * float64(len(levels)-1))
		if idx < 0 {
			idx = 0
		} else if idx >= len(levels) {
			idx = len(levels) - 1
		}
		out[i] = levels[idx]
	}
	return string(out)
}

// FindSharpestDrops returns the top-N positions where retention dropped fastest.
// A "drop" is a positive value of (points[i-1] - points[i]).
func findSharpestDrops(points []float64, n int) []DropAnnotation {
	if len(points) < 2 || n <= 0 {
		return nil
	}
	deltas := make([]struct {
		idx   int
		delta float64
	}, 0, len(points)-1)
	for i := 1; i < len(points); i++ {
		d := points[i-1] - points[i]
		if d > 0 {
			deltas = append(deltas, struct {
				idx   int
				delta float64
			}{i, d})
		}
	}
	// sort by delta desc (insertion sort is fine for ~99 items)
	for i := 1; i < len(deltas); i++ {
		for j := i; j > 0 && deltas[j-1].delta < deltas[j].delta; j-- {
			deltas[j-1], deltas[j] = deltas[j], deltas[j-1]
		}
	}
	if n > len(deltas) {
		n = len(deltas)
	}
	out := make([]DropAnnotation, n)
	for i := 0; i < n; i++ {
		out[i] = DropAnnotation{
			BucketIndex:    deltas[i].idx,
			VideoTimeRatio: float64(deltas[i].idx) / 100.0,
			DropMagnitude:  deltas[i].delta,
			BeforeRatio:    points[deltas[i].idx-1],
			AfterRatio:     points[deltas[i].idx],
		}
	}
	return out
}

// DropAnnotation describes one auto-annotated drop in a retention curve.
type DropAnnotation struct {
	BucketIndex    int     `json:"bucket_index"`
	VideoTimeRatio float64 `json:"video_time_ratio"`
	BeforeRatio    float64 `json:"before_ratio"`
	AfterRatio     float64 `json:"after_ratio"`
	DropMagnitude  float64 `json:"drop_magnitude"`
}

// ytStateDir is the standard local state directory for yt-studio.
func ytStateDir() string {
	home, _ := os.UserHomeDir()
	return home + "/.openclaw/state/yt-studio"
}
