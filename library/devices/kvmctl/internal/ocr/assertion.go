// Package ocr contains fail-closed screen assertion primitives.
package ocr

import (
	"errors"
	"fmt"
	"strings"
)

type Word struct {
	Text                string
	Confidence          float64
	X, Y, Width, Height int
}
type Match struct {
	Text       string
	Confidence float64
	Pixel      [2]int
	Box        [4]int
	XPct, YPct float64
}

func AssertContains(words []Word, wanted string, imageWidth, imageHeight int) (Match, error) {
	if imageWidth <= 0 || imageHeight <= 0 {
		return Match{}, errors.New("invalid image dimensions")
	}
	wanted = strings.TrimSpace(wanted)
	if wanted == "" {
		return Match{}, errors.New("assertion text must not be empty")
	}
	var matches []Match
	for _, w := range words {
		if w.Confidence < 30 || !strings.Contains(strings.ToLower(w.Text), strings.ToLower(wanted)) {
			continue
		}
		if w.X < 0 || w.Y < 0 || w.Width <= 0 || w.Height <= 0 || w.X+w.Width > imageWidth || w.Y+w.Height > imageHeight {
			return Match{}, fmt.Errorf("ocr match outside image bounds")
		}
		cx, cy := w.X+w.Width/2, w.Y+w.Height/2
		matches = append(matches, Match{Text: w.Text, Confidence: w.Confidence, Pixel: [2]int{cx, cy}, Box: [4]int{w.X, w.Y, w.Width, w.Height}, XPct: float64(cx) * 100 / float64(imageWidth), YPct: float64(cy) * 100 / float64(imageHeight)})
	}
	if len(matches) == 0 {
		return Match{}, errors.New("ocr assertion not found with sufficient confidence")
	}
	if len(matches) != 1 {
		return Match{}, fmt.Errorf("ocr assertion ambiguous: %d matches", len(matches))
	}
	return matches[0], nil
}
