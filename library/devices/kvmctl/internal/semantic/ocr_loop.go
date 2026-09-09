package semantic

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/ocr"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/results"
)

const (
	highConfidence    = 80.0
	hidReleaseTimeout = 15 * time.Second
)

var (
	observationStoreOnce sync.Once
	observationStoreInst *ocr.ObservationStore
)

func observationStore() *ocr.ObservationStore {
	observationStoreOnce.Do(func() {
		observationStoreInst = ocr.NewObservationStore(60*time.Second, 64, time.Now)
		if dir, err := cliutil.CacheDir(); err == nil && dir != "" {
			observationStoreInst.SetPersistPath(filepath.Join(dir, "ocr-observations.json"))
		}
	})
	return observationStoreInst
}

func opObserve(ctx context.Context, c *client.Client) (results.Operation, error) {
	observation, unavailable := captureObservation(ctx, c)
	if unavailable != nil {
		return unavailableOperation("observe", true, nil, unavailable), nil
	}
	return results.Build("observe", "kvm", true, "", true, false, "observed", map[string]any{"observation": observation}, nil), nil
}

func opClickText(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	observation, err := requiredFreshObservation(ctx, c, args)
	if err != nil {
		return results.Operation{}, err
	}
	text := stringArg(args, "text")
	if normalizeText(text) == "" {
		return results.Operation{}, fmt.Errorf("text is required")
	}
	region, outcome := exactHighConfidenceRegion(observation, text)
	if outcome != "match" {
		return results.Build("click-text", "kvm", false, "", false, false, "refused", map[string]any{"observation_id": observation.ID, "text": text, "outcome": outcome}, &results.Error{Code: "text " + outcome}), nil
	}
	if err := c.KVMDMouseMove(ctx, pixelToKVMD(region.Pixel[0], observation.Width), pixelToKVMD(region.Pixel[1], observation.Height)); err != nil {
		return unavailableOperation("click-text", false, map[string]any{"observation_id": observation.ID, "region": region}, err), nil
	}
	if err := c.KVMDMouseButton(ctx, "left", true); err != nil {
		return unavailableOperation("click-text", false, map[string]any{"observation_id": observation.ID, "region": region}, err), nil
	}
	if err := releaseMouseButton(ctx, c, "left"); err != nil {
		return unavailableOperation("click-text", false, map[string]any{"observation_id": observation.ID, "region": region}, err), nil
	}
	post, unavailable := captureObservation(ctx, c)
	if unavailable != nil {
		return unavailableOperation("click-text", false, map[string]any{"observation_id": observation.ID, "region": region, "clicked": true}, unavailable), nil
	}
	return results.Build("click-text", "kvm", false, "", true, true, "completed", map[string]any{"observation_id": observation.ID, "region": region, "post_observation": post}, nil), nil
}

func opPressKey(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	observation, err := requiredFreshObservation(ctx, c, args)
	if err != nil {
		return results.Operation{}, err
	}
	key := stringArg(args, "key")
	if !allowedKey(key) {
		return results.Operation{}, fmt.Errorf("key is not allowed")
	}
	if err := c.KVMDKey(ctx, key, true); err != nil {
		return unavailableOperation("press-key", false, map[string]any{"observation_id": observation.ID, "key": key}, err), nil
	}
	if err := releaseKey(ctx, c, key); err != nil {
		return unavailableOperation("press-key", false, map[string]any{"observation_id": observation.ID, "key": key}, err), nil
	}
	post, unavailable := captureObservation(ctx, c)
	if unavailable != nil {
		return unavailableOperation("press-key", false, map[string]any{"observation_id": observation.ID, "key": key, "pressed": true}, unavailable), nil
	}
	return results.Build("press-key", "kvm", false, "", true, true, "completed", map[string]any{"observation_id": observation.ID, "key": key, "post_observation": post}, nil), nil
}

func opVerifyText(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	text := stringArg(args, "text")
	if normalizeText(text) == "" {
		return results.Operation{}, fmt.Errorf("text is required")
	}
	observation, unavailable := captureObservation(ctx, c)
	if unavailable != nil {
		return unavailableOperation("verify-text", true, map[string]any{"text": text}, unavailable), nil
	}
	region, outcome := exactHighConfidenceRegion(observation, text)
	ok := outcome == "match"
	var resultError *results.Error
	if !ok {
		resultError = &results.Error{Code: "text " + outcome}
	}
	evidence := map[string]any{"text": text, "outcome": outcome, "observation": observation}
	if ok {
		evidence["region"] = region
	}
	return results.Build("verify-text", "kvm", true, "", ok, false, "observed", evidence, resultError), nil
}

func captureObservation(ctx context.Context, c *client.Client) (ocr.Observation, error) {
	imageBytes, err := c.KVMDSnapshot(ctx)
	if err != nil || len(imageBytes) == 0 {
		if err == nil {
			err = errors.New("empty snapshot")
		}
		return ocr.Observation{}, fmt.Errorf("snapshot unavailable: %w", err)
	}
	engine, err := ocr.CommandEngineFromEnvironment()
	if err != nil {
		return ocr.Observation{}, fmt.Errorf("ocr unavailable: %w", err)
	}
	width, height, words, err := engine.Recognize(imageBytes)
	if err != nil {
		return ocr.Observation{}, fmt.Errorf("ocr unavailable: %w", err)
	}
	regions := make([]ocr.Region, 0, len(words))
	for _, word := range words {
		regions = append(regions, ocr.Region{Text: strings.TrimSpace(word.Text), Confidence: word.Confidence, Box: [4]int{word.X, word.Y, word.Width, word.Height}, Pixel: [2]int{word.X + word.Width/2, word.Y + word.Height/2}})
	}
	observation, err := ocr.NewObservation(imageBytes, time.Now(), engine.Name(), width, height, regions)
	if err != nil {
		return ocr.Observation{}, fmt.Errorf("ocr unavailable: %w", err)
	}
	if err := observationStore().Put(observation); err != nil {
		return ocr.Observation{}, fmt.Errorf("ocr unavailable: persist observation: %w", err)
	}
	return observation, nil
}

func requiredFreshObservation(ctx context.Context, c *client.Client, args map[string]any) (ocr.Observation, error) {
	id := stringArg(args, "observation_id")
	if id == "" {
		return ocr.Observation{}, fmt.Errorf("observation_id is required")
	}
	stored, ok := observationStore().Get(id)
	if !ok {
		return ocr.Observation{}, fmt.Errorf("observation_id is stale: observation expired or is not local")
	}
	observation, unavailable := captureObservation(ctx, c)
	if unavailable != nil {
		return ocr.Observation{}, unavailable
	}
	if stored.ID != id || observation.ID != id {
		return ocr.Observation{}, fmt.Errorf("observation_id is stale: screen changed")
	}
	return observation, nil
}

func exactHighConfidenceRegion(observation ocr.Observation, text string) (ocr.Region, string) {
	wanted := normalizeText(text)
	matches := make([]ocr.Region, 0, 1)
	for _, region := range matchRegions(observation.OCR.Regions) {
		if region.Confidence >= highConfidence && normalizeText(region.Text) == wanted {
			matches = append(matches, region)
		}
	}
	switch len(matches) {
	case 0:
		return ocr.Region{}, "not_found"
	case 1:
		return matches[0], "match"
	default:
		return ocr.Region{}, "ambiguous"
	}
}

func releaseMouseButton(ctx context.Context, c *client.Client, button string) error {
	return releaseHeldInput(ctx, func(callCtx context.Context) error {
		return c.KVMDMouseButton(callCtx, button, false)
	})
}

func releaseKey(ctx context.Context, c *client.Client, key string) error {
	return releaseHeldInput(ctx, func(callCtx context.Context) error {
		return c.KVMDKey(callCtx, key, false)
	})
}

func releaseHeldInput(ctx context.Context, release func(context.Context) error) error {
	err := release(ctx)
	if err == nil {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), hidReleaseTimeout)
	defer cancel()
	_ = release(cleanupCtx)
	return err
}

func matchRegions(regions []ocr.Region) []ocr.Region {
	out := append([]ocr.Region(nil), regions...)
	for _, extra := range phraseRegions(regions) {
		if normalizeText(extra.Text) == "" {
			continue
		}
		if hasOverlappingSameText(out, extra) {
			continue
		}
		out = append(out, extra)
	}
	return out
}

func hasOverlappingSameText(regions []ocr.Region, extra ocr.Region) bool {
	key := normalizeText(extra.Text)
	for _, region := range regions {
		if normalizeText(region.Text) != key {
			continue
		}
		if boxesOverlap(region.Box, extra.Box) {
			return true
		}
	}
	return false
}

func boxesOverlap(a, b [4]int) bool {
	aRight, aBottom := a[0]+a[2], a[1]+a[3]
	bRight, bBottom := b[0]+b[2], b[1]+b[3]
	return a[0] < bRight && b[0] < aRight && a[1] < bBottom && b[1] < aBottom
}

func phraseRegions(regions []ocr.Region) []ocr.Region {
	words := make([]ocr.Region, 0, len(regions))
	for _, region := range regions {
		if strings.Contains(strings.TrimSpace(region.Text), " ") {
			continue
		}
		words = append(words, region)
	}
	groups := sameLineGroups(words)
	extra := make([]ocr.Region, 0)
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				extra = append(extra, concatRegions(group[i:j+1]))
			}
		}
	}
	return extra
}

func sameLineGroups(regions []ocr.Region) [][]ocr.Region {
	sorted := append([]ocr.Region(nil), regions...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Box[1] == sorted[j].Box[1] {
			return sorted[i].Box[0] < sorted[j].Box[0]
		}
		return sorted[i].Box[1] < sorted[j].Box[1]
	})
	groups := make([][]ocr.Region, 0)
	for _, region := range sorted {
		if normalizeText(region.Text) == "" {
			continue
		}
		placed := false
		for i := range groups {
			if verticallyOverlaps(groups[i][len(groups[i])-1], region) {
				groups[i] = append(groups[i], region)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, []ocr.Region{region})
		}
	}
	return groups
}

func verticallyOverlaps(a, b ocr.Region) bool {
	aBottom := a.Box[1] + a.Box[3]
	bBottom := b.Box[1] + b.Box[3]
	return a.Box[1] < bBottom && b.Box[1] < aBottom
}

func concatRegions(regions []ocr.Region) ocr.Region {
	left, top := regions[0].Box[0], regions[0].Box[1]
	right, bottom := regions[0].Box[0]+regions[0].Box[2], regions[0].Box[1]+regions[0].Box[3]
	conf := regions[0].Confidence
	parts := make([]string, 0, len(regions))
	for _, region := range regions {
		if region.Box[0] < left {
			left = region.Box[0]
		}
		if region.Box[1] < top {
			top = region.Box[1]
		}
		if region.Box[0]+region.Box[2] > right {
			right = region.Box[0] + region.Box[2]
		}
		if region.Box[1]+region.Box[3] > bottom {
			bottom = region.Box[1] + region.Box[3]
		}
		if region.Confidence < conf {
			conf = region.Confidence
		}
		parts = append(parts, strings.TrimSpace(region.Text))
	}
	return ocr.Region{
		Text:       strings.Join(parts, " "),
		Confidence: conf,
		Box:        [4]int{left, top, right - left, bottom - top},
		Pixel:      [2]int{left + (right-left)/2, top + (bottom-top)/2},
	}
}

func pixelToKVMD(pixel, dimension int) int {
	if dimension <= 1 {
		return 0
	}
	if pixel < 0 {
		pixel = 0
	}
	if pixel >= dimension {
		pixel = dimension - 1
	}
	return int(math.Round((float64(pixel)/float64(dimension-1))*65535.0 - 32768.0))
}

func normalizeText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func unavailableOperation(operation string, readOnly bool, evidence map[string]any, err error) results.Operation {
	return unavailableTransport(operation, "kvm", readOnly, evidence, err)
}

func unavailableTransport(operation, transport string, readOnly bool, evidence map[string]any, err error) results.Operation {
	if evidence == nil {
		evidence = map[string]any{}
	}
	evidence["unavailable"] = true
	return results.Build(operation, transport, readOnly, "", false, false, "unavailable", evidence, &results.Error{Code: err.Error(), Retryable: true})
}

func allowedKey(key string) bool {
	if strings.HasPrefix(key, "Key") && len(key) == 4 && key[3] >= 'A' && key[3] <= 'Z' {
		return true
	}
	if strings.HasPrefix(key, "Digit") && len(key) == 6 && key[5] >= '0' && key[5] <= '9' {
		return true
	}
	if strings.HasPrefix(key, "F") && len(key) >= 2 && len(key) <= 3 {
		if key == "F1" || key == "F2" || key == "F3" || key == "F4" || key == "F5" || key == "F6" || key == "F7" || key == "F8" || key == "F9" || key == "F10" || key == "F11" || key == "F12" {
			return true
		}
	}
	switch key {
	case "Enter", "Escape", "Tab", "Space", "Backspace", "Delete", "Insert", "Home", "End", "PageUp", "PageDown", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight":
		return true
	default:
		return false
	}
}
