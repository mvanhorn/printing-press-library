package june

import "encoding/json"

// The command data payloads. Field order is inside the signed bytes, so each is
// built from an ordered struct, never a map.

// PreheatData builds the 11002 payload for a given primitive and milli-°C target.
func PreheatData(primitive string, milliC int) json.RawMessage {
	b, _ := compactJSON(struct {
		PrimitiveType     string `json:"primitive_type"`
		TemperatureCavity int    `json:"temperature_cavity"`
	}{primitive, milliC})
	return b
}

// TempData builds the 11005 change-target payload.
func TempData(milliC int) json.RawMessage {
	b, _ := compactJSON(struct {
		PlanID            int `json:"plan_id"`
		TemperatureCavity int `json:"temperature_cavity"`
	}{0, milliC})
	return b
}

// TimerData builds the 11006 set-timer payload (duration in milliseconds).
func TimerData(durationMS int) json.RawMessage {
	b, _ := compactJSON(struct {
		PlanID   int `json:"plan_id"`
		Duration int `json:"duration"`
	}{0, durationMS})
	return b
}

// CancelData builds the 11004 cancel payload.
func CancelData() json.RawMessage {
	b, _ := compactJSON(struct {
		PlanID int `json:"plan_id"`
	}{0})
	return b
}
