package wpble

import (
	"encoding/hex"
	"testing"
)

func TestCommandFrames(t *testing.T) {
	cases := []struct {
		name string
		got  []byte
		want string
	}{
		{"ask_stats", CmdAskStats(), "f7a20000a2fd"},
		{"start", CmdStart(), "f7a20401a7fd"},
		{"stop", CmdStop(), "f7a20100a3fd"},
		{"mode_manual", CmdMode(ModeManual), "f7a20201a5fd"},
		{"mode_auto", CmdMode(ModeAuto), "f7a20200a4fd"},
		{"mode_standby", CmdMode(ModeStandby), "f7a20202a6fd"},
		{"ask_last_record", CmdAskLastRecord(), "f7a7aaff50fd"},
		// speed 3.0 km/h -> 30 units (0x1e): crc = a2+01+1e = c1
		{"speed_3kmh", CmdSpeed(PadSpeed(3.0)), "f7a2011ec1fd"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hex.EncodeToString(c.got); got != c.want {
				t.Fatalf("frame = %s, want %s", got, c.want)
			}
		})
	}
}

func TestDescribeCommand(t *testing.T) {
	cases := []struct {
		name string
		got  []byte
		want string
	}{
		{"ask_stats", CmdAskStats(), "ask_stats"},
		{"start", CmdStart(), "start"},
		{"stop_speed0", CmdStop(), "stop(speed0)"},
		{"speed", CmdSpeed(PadSpeed(2.0)), "speed=20"},
		{"mode_manual", CmdMode(ModeManual), "mode=manual"},
		{"mode_standby", CmdMode(ModeStandby), "mode=standby"},
		{"profile", CmdAskProfile(), "profile"},
		{"beep", CmdInitBeep(), "beep"},
		{"unknown", []byte{0x01, 0x02, 0x03, 0x04}, "01020304"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DescribeCommand(c.got); got != c.want {
				t.Fatalf("DescribeCommand = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPadSpeedClamp(t *testing.T) {
	cases := []struct {
		kmh  float64
		want byte
	}{
		{-1, 0},
		{0, 0},
		{0.1, 5}, // below running minimum clamps up to 5
		{0.5, 5},
		{3.0, 30},
		{6.0, 60},
		{9.9, 60}, // above maximum clamps down
	}
	for _, c := range cases {
		if got := PadSpeed(c.kmh); got != c.want {
			t.Errorf("PadSpeed(%v) = %d, want %d", c.kmh, got, c.want)
		}
	}
}

func TestSetPrefFrame(t *testing.T) {
	// max speed 6.0 km/h -> 60 units, stype 0: body = a6,03,00,00,00,3c
	// crc = a6+03+00+00+00+3c = e5
	got := hex.EncodeToString(CmdSetPref(PrefMaxSpeed, 60, 0))
	if want := "f7a6030000003ce5fd"; got != want {
		t.Fatalf("CmdSetPref = %s, want %s", got, want)
	}
}

func TestParseStatus(t *testing.T) {
	// Documented frame from ph4 README:
	// f8a2010f01000fd10000ab0012ae3c0000003afd
	raw, err := hex.DecodeString("f8a2010f01000fd10000ab0012ae3c0000003afd")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := ParseStatus(raw)
	if !ok {
		t.Fatal("ParseStatus returned ok=false for a valid frame")
	}
	if s.BeltState != 1 {
		t.Errorf("BeltState = %d, want 1", s.BeltState)
	}
	if s.SpeedKmh != 1.5 {
		t.Errorf("SpeedKmh = %v, want 1.5", s.SpeedKmh)
	}
	if s.Mode != ModeManual || s.ModeName != "manual" {
		t.Errorf("Mode = %d (%s), want manual", s.Mode, s.ModeName)
	}
	if s.TimeS != 4049 { // 0x000fd1
		t.Errorf("TimeS = %d, want 4049", s.TimeS)
	}
	if s.DistanceM != 1710 { // 0x0000ab = 171 -> 1710 m
		t.Errorf("DistanceM = %d, want 1710", s.DistanceM)
	}
	if s.Steps != 4782 { // 0x0012ae
		t.Errorf("Steps = %d, want 4782", s.Steps)
	}
}

func TestParseStatusRejectsNonStatus(t *testing.T) {
	if _, ok := ParseStatus([]byte{0xf8, 0xa7, 0x00}); ok {
		t.Error("ParseStatus accepted a last-record frame")
	}
	if _, ok := ParseStatus([]byte{0xf8, 0xa2}); ok {
		t.Error("ParseStatus accepted a too-short frame")
	}
}

func TestParseLastRecord(t *testing.T) {
	// Build a record frame: f8 a7 + 6 filler + time(000064=100) dist(00000a=10->100m) steps(0003e8=1000) + pad
	raw, err := hex.DecodeString("f8a700000000000000006400000a0003e800fd")
	if err != nil {
		t.Fatal(err)
	}
	r, ok := ParseLastRecord(raw)
	if !ok {
		t.Fatal("ParseLastRecord ok=false for valid frame")
	}
	if r.TimeS != 100 {
		t.Errorf("TimeS = %d, want 100", r.TimeS)
	}
	if r.DistanceM != 100 { // 10 * 10m
		t.Errorf("DistanceM = %d, want 100", r.DistanceM)
	}
	if r.Steps != 1000 {
		t.Errorf("Steps = %d, want 1000", r.Steps)
	}
}

func TestModeFromName(t *testing.T) {
	for name, want := range map[string]byte{"auto": ModeAuto, "manual": ModeManual, "standby": ModeStandby} {
		got, err := ModeFromName(name)
		if err != nil || got != want {
			t.Errorf("ModeFromName(%q) = %d, %v; want %d", name, got, err, want)
		}
	}
	if _, err := ModeFromName("turbo"); err == nil {
		t.Error("ModeFromName(turbo) should error")
	}
}
