// PATCH(library): verification policies parity with Python machines.py.
package machines

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type VerifyPolicy string

const (
	VerifyNone                VerifyPolicy = "none"
	VerifyFrameChangePolicy   VerifyPolicy = "frame_change"
	VerifyOCRIdentityPolicy   VerifyPolicy = "ocr_identity"
	VerifyPromptPatternPolicy VerifyPolicy = "prompt_pattern"
)

var DefaultVerifyPolicy = map[string]VerifyPolicy{
	"pve1":       VerifyPromptPatternPolicy,
	"pve2":       VerifyPromptPatternPolicy,
	"kodi-build": VerifyFrameChangePolicy,
	"pve3":       VerifyPromptPatternPolicy,
}

type SnapshotSource interface {
	Snapshot(context.Context) ([]byte, error)
}

type OCRSource interface {
	SnapshotSource
	OCR(context.Context, []byte) (string, error)
}

func FramesDiffer(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return string(a) != string(b)
	}
	if len(a) != len(b) {
		return true
	}
	// simple byte diff; Python uses perceptual mean >3.0 but bytes compare is safe fallback
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

func sleepCtx(ctx context.Context, d time.Duration, sleep func(context.Context, time.Duration) error) error {
	if d <= 0 {
		return nil
	}
	if sleep != nil {
		return sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func VerifyFrameChange(ctx context.Context, src SnapshotSource, baseline []byte, attempts int, delay time.Duration, sleep func(context.Context, time.Duration) error) (bool, error) {
	if len(baseline) == 0 {
		return false, fmt.Errorf("FRAME_CHANGE requires a baseline snapshot")
	}
	if attempts < 1 {
		attempts = 5
	}
	for i := 0; i < attempts; i++ {
		if i > 0 {
			if err := sleepCtx(ctx, delay, sleep); err != nil {
				return false, err
			}
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		frame, err := src.Snapshot(ctx)
		if err != nil {
			continue
		}
		if FramesDiffer(baseline, frame) {
			return true, nil
		}
	}
	return false, nil
}

func VerifyOCRIdentity(ctx context.Context, src OCRSource, target Target, attempts int, delay time.Duration, sleep func(context.Context, time.Duration) error) (bool, string, error) {
	if attempts < 1 {
		attempts = 5
	}
	last := ""
	for i := 0; i < attempts; i++ {
		if i > 0 {
			if err := sleepCtx(ctx, delay, sleep); err != nil {
				return false, last, err
			}
		}
		if err := ctx.Err(); err != nil {
			return false, last, err
		}
		frame, err := src.Snapshot(ctx)
		if err != nil {
			continue
		}
		text, err := src.OCR(ctx, frame)
		if err != nil {
			continue
		}
		last = text
		if ocrMatches(target, text) {
			return true, text, nil
		}
	}
	return false, last, nil
}

func VerifyPromptPattern(ctx context.Context, src OCRSource, target Target, attempts int, delay time.Duration, sleep func(context.Context, time.Duration) error) (bool, string, error) {
	if attempts < 1 {
		attempts = 5
	}
	last := ""
	for i := 0; i < attempts; i++ {
		if i > 0 {
			if err := sleepCtx(ctx, delay, sleep); err != nil {
				return false, last, err
			}
		}
		if err := ctx.Err(); err != nil {
			return false, last, err
		}
		frame, err := src.Snapshot(ctx)
		if err != nil {
			continue
		}
		text, err := src.OCR(ctx, frame)
		if err != nil {
			continue
		}
		last = text
		if promptMatches(target, text) {
			return true, text, nil
		}
	}
	return false, last, nil
}

func ocrMatches(t Target, text string) bool {
	if len(t.OCRPatterns) == 0 {
		return false
	}
	lower := strings.ToLower(text)
	for _, p := range t.OCRPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func promptMatches(t Target, text string) bool {
	if len(t.PromptPatterns) == 0 {
		return false
	}
	for _, p := range t.PromptPatterns {
		if ok, _ := regexp.MatchString("(?i)"+p, text); ok {
			// Python uses re.search(p, text, re.IGNORECASE)
			if matched, _ := regexp.MatchString(p, text); matched {
				// use case-insensitive via (?i) already
			}
			re, err := regexp.Compile("(?i)" + p)
			if err != nil {
				continue
			}
			if re.MatchString(text) {
				return true
			}
		}
	}
	return false
}

func RunVerifyPolicy(ctx context.Context, policy VerifyPolicy, src SnapshotSource, target Target, baseline []byte, attempts int, delay time.Duration, sleep func(context.Context, time.Duration) error) (bool, string, error) {
	switch policy {
	case VerifyNone:
		return false, "policy none: no automatic verification", nil
	case VerifyFrameChangePolicy:
		if len(baseline) == 0 {
			return false, "", fmt.Errorf("FRAME_CHANGE requires a baseline snapshot")
		}
		ok, err := VerifyFrameChange(ctx, src, baseline, attempts, delay, sleep)
		if err != nil {
			return false, "", err
		}
		if ok {
			return true, "screen changed", nil
		}
		return false, "no frame change detected", nil
	case VerifyOCRIdentityPolicy:
		oc, ok := src.(OCRSource)
		if !ok {
			return false, "", fmt.Errorf("ocr_identity requires OCR source")
		}
		ok2, text, err := VerifyOCRIdentity(ctx, oc, target, attempts, delay, sleep)
		if err != nil {
			return false, "", err
		}
		if ok2 {
			return true, "ocr identity match", nil
		}
		return false, fmt.Sprintf("ocr mismatch; last text: %q", truncate(text, 200)), nil
	case VerifyPromptPatternPolicy:
		oc, ok := src.(OCRSource)
		if !ok {
			return false, "", fmt.Errorf("prompt_pattern requires OCR source")
		}
		ok2, text, err := VerifyPromptPattern(ctx, oc, target, attempts, delay, sleep)
		if err != nil {
			return false, "", err
		}
		if ok2 {
			return true, "prompt pattern match", nil
		}
		return false, fmt.Sprintf("prompt not seen; last text: %q", truncate(text, 200)), nil
	default:
		return false, "", fmt.Errorf("unknown policy %q", policy)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
