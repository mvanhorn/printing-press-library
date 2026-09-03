package semantic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/ocr"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/results"
)

const highConfidence = 80.0

var observations = ocr.NewObservationStore(60*time.Second, 64, time.Now)

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
	if err := c.KVMDMouseMove(ctx, region.Pixel[0], region.Pixel[1]); err != nil {
		return unavailableOperation("click-text", false, map[string]any{"observation_id": observation.ID, "region": region}, err), nil
	}
	if err := c.KVMDMouseButton(ctx, "left", true); err != nil {
		return unavailableOperation("click-text", false, map[string]any{"observation_id": observation.ID, "region": region}, err), nil
	}
	if err := c.KVMDMouseButton(ctx, "left", false); err != nil {
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
	if err := c.KVMDKey(ctx, key, false); err != nil {
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
	imageBytes, err := c.GetWithHeadersNoCache(ctx, "/api/streamer/snapshot", nil, map[string]string{"Accept": "image/jpeg", client.BinaryResponseHeader: "true"})
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
	observations.Put(observation)
	return observation, nil
}

func requiredFreshObservation(ctx context.Context, c *client.Client, args map[string]any) (ocr.Observation, error) {
	id := stringArg(args, "observation_id")
	if id == "" {
		return ocr.Observation{}, fmt.Errorf("observation_id is required")
	}
	observation, unavailable := captureObservation(ctx, c)
	if unavailable != nil {
		return ocr.Observation{}, unavailable
	}
	if observation.ID != id {
		return ocr.Observation{}, fmt.Errorf("observation_id is stale: screen changed")
	}
	return observation, nil
}

func exactHighConfidenceRegion(observation ocr.Observation, text string) (ocr.Region, string) {
	wanted := normalizeText(text)
	matches := make([]ocr.Region, 0, 1)
	for _, region := range observation.OCR.Regions {
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

func normalizeText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func unavailableOperation(operation string, readOnly bool, evidence map[string]any, err error) results.Operation {
	if evidence == nil {
		evidence = map[string]any{}
	}
	evidence["unavailable"] = true
	return results.Build(operation, "kvm", readOnly, "", false, false, "unavailable", evidence, &results.Error{Code: err.Error(), Retryable: true})
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
