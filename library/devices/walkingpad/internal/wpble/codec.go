// Package wpble implements the WalkingPad BLE GATT protocol: command framing,
// checksum, and status/record parsing. It is the single source of truth for
// the wire format and is pure Go (no BLE backend), so it builds and tests
// everywhere regardless of build tags.
//
// Protocol reverse-engineered by ph4r05/ph4-walkingpad and confirmed against a
// live WalkingPad C2. Commands are framed [0xF7, group, payload..., crc, 0xFD]
// where crc = sum(bytes between header and crc) % 256. Status notifications
// arrive on the notify characteristic prefixed 0xF8 0xA2.
package wpble

import (
	"encoding/hex"
	"fmt"
)

// GATT identifiers shared across the WalkingPad family (A1/C2/R1/R2/X21).
// Discovery matches the service UUID rather than the advertised name, because
// model names differ (a C2 advertises as "KS-BLC2", not "WalkingPad").
const (
	ServiceUUID = "0000fe00-0000-1000-8000-00805f9b34fb"
	NotifyUUID  = "0000fe01-0000-1000-8000-00805f9b34fb"
	WriteUUID   = "0000fe02-0000-1000-8000-00805f9b34fb"
)

const (
	frameHeader = 0xF7
	frameFooter = 0xFD

	groupControl = 0xA2 // start/stop/speed/mode/ask-stats
	groupProfile = 0xA5 // profile query (connection handshake)
	groupPrefs   = 0xA6 // preferences
	groupHistory = 0xA7 // ask last record

	statusPrefix0 = 0xF8
	statusPrefix1 = 0xA2 // current-status notification
	recordPrefix1 = 0xA7 // last-record notification
)

// Mode values for switch_mode.
const (
	ModeAuto    = 0
	ModeManual  = 1
	ModeStandby = 2
)

// BeltStateRunning is the belt_state value a status notification reports once the
// belt is actually moving, after the 3-2-1 start countdown (belt_state 9->8->7).
// A speed command sent before this is ignored, so callers wait for it before
// setting the target speed.
const BeltStateRunning = 1

// Preference keys (group 0xA6).
const (
	PrefTarget      = 1
	PrefMaxSpeed    = 3
	PrefStartSpeed  = 4
	PrefStartIntel  = 5
	PrefSensitivity = 6
	PrefDisplay     = 7
	PrefUnits       = 8
	PrefChildLock   = 9
)

// MinSpeedKmh and MaxSpeedKmh bound a running belt. 0 is allowed and means
// stop. The native app only permits 0.5 steps; this CLI permits any 0.1 step.
const (
	MinSpeedKmh = 0.5
	MaxSpeedKmh = 6.0
)

// frame builds a CRC-checked command frame from a group byte and its payload.
func frame(group byte, payload ...byte) []byte {
	body := make([]byte, 0, len(payload)+1)
	body = append(body, group)
	body = append(body, payload...)
	var crc byte
	for _, b := range body {
		crc += b // byte arithmetic wraps mod 256
	}
	out := make([]byte, 0, len(body)+3)
	out = append(out, frameHeader)
	out = append(out, body...)
	out = append(out, crc, frameFooter)
	return out
}

// PadSpeed converts km/h to the belt's 0.1-km/h integer units. A non-positive
// speed maps to 0 (stop); any running speed is clamped to [5, 60] (0.5-6.0).
func PadSpeed(kmh float64) byte {
	if kmh <= 0 {
		return 0
	}
	units := int(kmh*10 + 0.5)
	if units < 5 {
		units = 5
	}
	if units > 60 {
		units = 60
	}
	return byte(units)
}

// Command builders. Each returns the exact bytes written to the write
// characteristic (CRC already applied).
func CmdAskStats() []byte      { return frame(groupControl, 0x00, 0x00) }
func CmdStart() []byte         { return frame(groupControl, 0x04, 0x01) }
func CmdStop() []byte          { return frame(groupControl, 0x01, 0x00) }
func CmdSpeed(u byte) []byte   { return frame(groupControl, 0x01, u) }
func CmdMode(mode byte) []byte { return frame(groupControl, 0x02, mode) }
func CmdAskLastRecord() []byte { return frame(groupHistory, 0xAA, 0xFF) }

// CmdAskProfile and CmdInitBeep make up the connection handshake the belt needs
// before it will answer ask-stats with notifications. Reverse-engineered from
// ph4r05/ph4-walkingpad's connect ceremony (profile query on group 0xA5, then a
// beep on the control group), each followed by a short delay. Without this the
// belt accepts the connection and the subscription but never streams status.
func CmdAskProfile() []byte { return frame(groupProfile, 0x60, 0x4A, 0x4D, 0x93, 0x71, 0x29) }
func CmdInitBeep() []byte   { return frame(groupControl, 0x03, 0x07) }

// DescribeCommand returns a short human label for an outbound command frame
// (e.g. "start", "mode=manual", "speed=20") for diagnostics and tracing, or the
// hex encoding for a frame it does not recognize. It is the single place that
// maps the wire layout back to names, so callers never re-derive group/sub-code
// bytes.
func DescribeCommand(p []byte) string {
	if len(p) < 4 || p[0] != frameHeader {
		return hex.EncodeToString(p)
	}
	switch p[1] {
	case groupProfile:
		return "profile"
	case groupControl:
		switch p[2] {
		case 0x00:
			return "ask_stats"
		case 0x04:
			return "start"
		case 0x03:
			return "beep"
		case 0x02:
			return "mode=" + ModeName(int(p[3]))
		case 0x01:
			if p[3] == 0 {
				return "stop(speed0)"
			}
			return fmt.Sprintf("speed=%d", p[3])
		}
	}
	return hex.EncodeToString(p)
}

// CmdSetPref encodes set_pref_int(key, value, stype): a sub-type byte followed
// by a 3-byte big-endian value.
func CmdSetPref(key byte, value int, stype byte) []byte {
	return frame(groupPrefs, key, stype, byte(value>>16), byte(value>>8), byte(value))
}

// ModeFromName maps a user-facing mode name to its protocol value.
func ModeFromName(name string) (byte, error) {
	switch name {
	case "auto":
		return ModeAuto, nil
	case "manual":
		return ModeManual, nil
	case "standby":
		return ModeStandby, nil
	default:
		return 0, fmt.Errorf("unknown mode %q: want auto, manual, or standby", name)
	}
}

// ModeName maps a protocol mode value to its user-facing name.
func ModeName(mode int) string {
	switch mode {
	case ModeAuto:
		return "auto"
	case ModeManual:
		return "manual"
	case ModeStandby:
		return "standby"
	default:
		return "unknown"
	}
}

// Status is a decoded current-status notification.
type Status struct {
	BeltState   int     `json:"belt_state"`
	SpeedKmh    float64 `json:"speed_kmh"`
	Mode        int     `json:"mode"`
	ModeName    string  `json:"mode_name"`
	TimeS       int     `json:"time_s"`
	DistanceM   int     `json:"distance_m"`
	Steps       int     `json:"steps"`
	AppSpeedKmh float64 `json:"app_speed_kmh"`
	Button      int     `json:"button"`
	RawHex      string  `json:"raw_hex"`
}

// LastRecord is a decoded last-stored-run notification.
type LastRecord struct {
	TimeS     int    `json:"time_s"`
	DistanceM int    `json:"distance_m"`
	Steps     int    `json:"steps"`
	RawHex    string `json:"raw_hex"`
}

func be3(b []byte) int {
	return int(b[0])<<16 | int(b[1])<<8 | int(b[2])
}

// IsStatus reports whether data is a current-status notification frame.
func IsStatus(data []byte) bool {
	return len(data) >= 18 && data[0] == statusPrefix0 && data[1] == statusPrefix1
}

// IsLastRecord reports whether data is a last-record notification frame.
func IsLastRecord(data []byte) bool {
	return len(data) >= 17 && data[0] == statusPrefix0 && data[1] == recordPrefix1
}

// ParseStatus decodes a current-status notification. The bool is false when the
// frame is not a status message (wrong prefix or too short).
func ParseStatus(data []byte) (Status, bool) {
	if !IsStatus(data) {
		return Status{}, false
	}
	s := Status{
		BeltState:   int(data[2]),
		SpeedKmh:    float64(data[3]) / 10,
		Mode:        int(data[4]),
		TimeS:       be3(data[5:8]),
		DistanceM:   be3(data[8:11]) * 10, // belt reports distance in 10m units
		Steps:       be3(data[11:14]),
		AppSpeedKmh: float64(data[14]) / 30,
		Button:      int(data[16]),
		RawHex:      hex.EncodeToString(data),
	}
	s.ModeName = ModeName(s.Mode)
	return s, true
}

// ParseLastRecord decodes a last-stored-run notification.
func ParseLastRecord(data []byte) (LastRecord, bool) {
	if !IsLastRecord(data) {
		return LastRecord{}, false
	}
	return LastRecord{
		TimeS:     be3(data[8:11]),
		DistanceM: be3(data[11:14]) * 10,
		Steps:     be3(data[14:17]),
		RawHex:    hex.EncodeToString(data),
	}, true
}
