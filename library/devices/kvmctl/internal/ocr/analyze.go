package ocr

import (
	"errors"
	"math"
	"sort"
	"strings"
)

// Analysis mirrors Python ocr.analyze return: width, height, sorted elements.
type Analysis struct {
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Elements []Match `json:"elements"`
}

// Analyze filters words by searchText (casefold, substring), confidence ≥30,
// validates bounds, computes pixel center, box, x_pct/y_pct rounded to 1 decimal,
// and sorts by -confidence, y_pct, x_pct.
func Analyze(words []Word, imageWidth, imageHeight int, searchText string) (Analysis, error) {
	if imageWidth <= 0 || imageHeight <= 0 {
		return Analysis{}, errors.New("invalid image dimensions")
	}
	wanted := strings.ToLower(strings.TrimSpace(searchText))
	hasWanted := wanted != ""
	var elements []Match
	for _, w := range words {
		text := strings.TrimSpace(w.Text)
		if text == "" {
			continue
		}
		if hasWanted && !strings.Contains(strings.ToLower(text), wanted) {
			continue
		}
		if w.Confidence < 30 {
			continue
		}
		if w.X < 0 || w.Y < 0 || w.Width <= 0 || w.Height <= 0 || w.X+w.Width > imageWidth || w.Y+w.Height > imageHeight {
			return Analysis{}, errors.New("ocr match outside image bounds")
		}
		cx := w.X + w.Width/2
		cy := w.Y + w.Height/2
		xpct := math.Round(float64(cx)*1000/float64(imageWidth)) / 10
		ypct := math.Round(float64(cy)*1000/float64(imageHeight)) / 10
		conf := math.Round(w.Confidence*10) / 10
		elements = append(elements, Match{
			Text:       text,
			Confidence: conf,
			Pixel:      [2]int{cx, cy},
			Box:        [4]int{w.X, w.Y, w.Width, w.Height},
			XPct:       xpct,
			YPct:       ypct,
		})
	}
	sort.Slice(elements, func(i, j int) bool {
		if elements[i].Confidence != elements[j].Confidence {
			return elements[i].Confidence > elements[j].Confidence
		}
		if elements[i].YPct != elements[j].YPct {
			return elements[i].YPct < elements[j].YPct
		}
		return elements[i].XPct < elements[j].XPct
	})
	if elements == nil {
		elements = []Match{}
	}
	return Analysis{Width: imageWidth, Height: imageHeight, Elements: elements}, nil
}

// Engine is the injectable OCR data source; imageBytes are not decoded here
// so tests can run without tesseract/hardware.
type Engine interface {
	Recognize(imageBytes []byte) (width, height int, words []Word, err error)
}

// AnalyzeImage runs an Engine then filters/sorts via Analyze.
func AnalyzeImage(imageBytes []byte, searchText string, engine Engine) (Analysis, error) {
	if engine == nil {
		return Analysis{}, errors.New("ocr engine unavailable")
	}
	w, h, words, err := engine.Recognize(imageBytes)
	if err != nil {
		return Analysis{}, err
	}
	return Analyze(words, w, h, searchText)
}

// Click holds the HID-usable click coordinate for a Match.
type Click struct {
	X, Y int
	XPct float64
	YPct float64
}

// ClickForMatch maps a Match to a click coordinate, validating the match lies
// within the given image dimensions. Mirrors the pixel/box/x_pct/y_pct math.
func ClickForMatch(m Match, imageWidth, imageHeight int) (Click, error) {
	if imageWidth <= 0 || imageHeight <= 0 {
		return Click{}, errors.New("invalid image dimensions")
	}
	// Re-derive center from box to ensure consistency.
	x, y, w, h := m.Box[0], m.Box[1], m.Box[2], m.Box[3]
	if w <= 0 || h <= 0 || x < 0 || y < 0 || x+w > imageWidth || y+h > imageHeight {
		return Click{}, errors.New("ocr match outside image bounds")
	}
	cx, cy := x+w/2, y+h/2
	if m.Pixel != [2]int{cx, cy} {
		// allow slight mismatch but prefer box-derived
		cx, cy = m.Pixel[0], m.Pixel[1]
		if cx < 0 || cy < 0 || cx > imageWidth || cy > imageHeight {
			return Click{}, errors.New("click outside image bounds")
		}
	}
	return Click{X: cx, Y: cy, XPct: m.XPct, YPct: m.YPct}, nil
}

// ClickForAnalysis returns the click coordinate for the best (first sorted) element.
func ClickForAnalysis(a Analysis, searchText string) (Click, error) {
	filtered := a.Elements
	if s := strings.TrimSpace(searchText); s != "" {
		wanted := strings.ToLower(s)
		var keep []Match
		for _, e := range a.Elements {
			if strings.Contains(strings.ToLower(e.Text), wanted) {
				keep = append(keep, e)
			}
		}
		filtered = keep
	}
	if len(filtered) == 0 {
		return Click{}, errors.New("ocr assertion not found with sufficient confidence")
	}
	if len(filtered) != 1 {
		return Click{}, errors.New("ocr assertion ambiguous")
	}
	return ClickForMatch(filtered[0], a.Width, a.Height)
}
