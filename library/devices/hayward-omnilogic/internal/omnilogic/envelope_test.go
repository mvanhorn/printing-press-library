package omnilogic

import (
	"strings"
	"testing"
)

func TestBuildRequest_TokenSkipped(t *testing.T) {
	xml, err := buildRequest("GetSiteList", map[string]any{
		"Token":  "abc123",
		"UserID": "42",
	})
	if err != nil {
		t.Fatalf("buildRequest err: %v", err)
	}
	if strings.Contains(xml, "abc123") {
		t.Errorf("Token leaked into XML body: %s", xml)
	}
	if !strings.Contains(xml, "<Name>GetSiteList</Name>") {
		t.Errorf("op name missing: %s", xml)
	}
	if !strings.Contains(xml, `name="UserID"`) {
		t.Errorf("UserID param missing: %s", xml)
	}
}

func TestBuildRequest_DataTypeInference(t *testing.T) {
	xml, err := buildRequest("SetHeaterEnable", map[string]any{
		"PoolID":   1,
		"HeaterID": 5,
		"Enabled":  true,
		"Version":  "0",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	tests := []struct{ frag, label string }{
		{`name="PoolID" dataType="int">1<`, "int param"},
		{`name="Enabled" dataType="bool">true<`, "bool param"},
		{`name="Version" dataType="string">0<`, "string param"},
	}
	for _, tc := range tests {
		if !strings.Contains(xml, tc.frag) {
			t.Errorf("%s missing: %s", tc.label, xml)
		}
	}
}

func TestBuildChlorRequest_OrderAndTypo(t *testing.T) {
	timed := 60
	xml := buildChlorRequest(map[string]any{
		"MspSystemID":  12345,
		"PoolID":       1,
		"ChlorID":      9,
		"TimedPercent": timed,
		"CellType":     3,
	})
	// Hayward's typo preserved
	if !strings.Contains(xml, `name="ORPTimout"`) {
		t.Errorf("expected ORPTimout typo preserved: %s", xml)
	}
	// Defaults for missing SCTimeout / ORPTimout = 4
	if !strings.Contains(xml, `name="SCTimeout" dataType="byte">4<`) {
		t.Errorf("expected default SCTimeout=4: %s", xml)
	}
	if !strings.Contains(xml, `name="ORPTimout" dataType="byte">4<`) {
		t.Errorf("expected default ORPTimout=4: %s", xml)
	}
}

func TestParseSiteList(t *testing.T) {
	resp := `<?xml version="1.0"?>
<Response>
  <Parameters>
    <Parameter name="Status" dataType="int">0</Parameter>
    <Parameter name="List" dataType="Array">
      <Item>
        <Property name="MspSystemID" dataType="int">12345</Property>
        <Property name="BackyardName" dataType="string">Main Pool</Property>
      </Item>
      <Item>
        <Property name="MspSystemID" dataType="int">67890</Property>
        <Property name="BackyardName" dataType="string">Vacation Home</Property>
      </Item>
    </Parameter>
  </Parameters>
</Response>`
	sites, err := parseSiteList(resp)
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
	if sites[0].MspSystemID != 12345 || sites[0].BackyardName != "Main Pool" {
		t.Errorf("site 0 wrong: %+v", sites[0])
	}
	if sites[1].MspSystemID != 67890 || sites[1].BackyardName != "Vacation Home" {
		t.Errorf("site 1 wrong: %+v", sites[1])
	}
}

func TestStatusFromResponse(t *testing.T) {
	ok := `<Response><Parameters><Parameter name="Status" dataType="int">0</Parameter></Parameters></Response>`
	status, msg, hasStatus := statusFromResponse(ok)
	if !hasStatus || status != 0 || msg != "" {
		t.Errorf("ok parse wrong: %d, %q, %v", status, msg, hasStatus)
	}
	fail := `<Response><Parameters><Parameter name="Status" dataType="int">42</Parameter><Parameter name="StatusMessage" dataType="string">bad request</Parameter></Parameters></Response>`
	status, msg, hasStatus = statusFromResponse(fail)
	if !hasStatus || status != 42 || msg != "bad request" {
		t.Errorf("fail parse wrong: %d, %q, %v", status, msg, hasStatus)
	}
}

func TestResolveShow_Numeric(t *testing.T) {
	s, ok := ResolveShow("3")
	if !ok || s.Name != "Royal Blue" {
		t.Errorf("expected Royal Blue, got %+v ok=%v", s, ok)
	}
}

func TestResolveShow_Name(t *testing.T) {
	tests := []string{"Deep Blue Sea", "deep blue sea", "deep-blue-sea", "deepbluesea"}
	for _, in := range tests {
		s, ok := ResolveShow(in)
		if !ok || s.ID != 2 {
			t.Errorf("%q didn't match Deep Blue Sea: got %+v ok=%v", in, s, ok)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want int
		err  bool
	}{
		{"", 0, false},
		{"30m", 30, false},
		{"1h", 60, false},
		{"2h30m", 150, false},
		{"45", 45, false},
		{"bad", 0, true},
	}
	for _, tc := range tests {
		got, err := ParseDuration(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("%q expected err, got %d", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%q: want %d, got %d, err=%v", tc.in, tc.want, got, err)
		}
	}
}

func TestChemistryVerdict_AllOk(t *testing.T) {
	ph := 7.5
	orp := 700
	salt := 3000
	v, r := ChemistryVerdict(&ph, &orp, &salt)
	if v != "ok" || len(r) != 0 {
		t.Errorf("expected ok with no reasons, got %s %v", v, r)
	}
}

func TestChemistryVerdict_PhLow(t *testing.T) {
	ph := 7.0
	v, r := ChemistryVerdict(&ph, nil, nil)
	if v != "low" || len(r) != 1 {
		t.Errorf("expected low with one reason, got %s %v", v, r)
	}
}

func TestChemistryVerdict_AllNil(t *testing.T) {
	v, _ := ChemistryVerdict(nil, nil, nil)
	if v != "unknown" {
		t.Errorf("expected unknown, got %s", v)
	}
}

// TestChemistryVerdict_MixedDirections covers the Greptile review finding
// that the original bumpVerdict collapsed pH-low + ORP-high into "low" and
// silently dropped the second finding from the verdict string.
func TestChemistryVerdict_MixedDirections(t *testing.T) {
	ph := 7.0  // low (want >=7.2)
	orp := 850 // high (want <=800)
	v, r := ChemistryVerdict(&ph, &orp, nil)
	if v != "mixed" {
		t.Errorf("expected mixed verdict when pH low + ORP high, got %s", v)
	}
	if len(r) != 2 {
		t.Errorf("expected 2 reasons (one per out-of-range metric), got %d: %v", len(r), r)
	}
}

// TestChemistryVerdict_SameDirection ensures the verdict stays single-word
// when every out-of-range metric is on the same side (e.g. pH low + salt low).
func TestChemistryVerdict_SameDirection(t *testing.T) {
	ph := 7.0      // low
	salt := 2400   // low
	v, r := ChemistryVerdict(&ph, nil, &salt)
	if v != "low" {
		t.Errorf("expected low when every metric is low, got %s", v)
	}
	if len(r) != 2 {
		t.Errorf("expected 2 reasons, got %d", len(r))
	}
}

// TestBuildRequest_SortedOrder ensures buildRequest emits parameters in a
// deterministic alphabetical sequence regardless of map iteration order.
// Hayward's .NET handler is order-sensitive on at least one Set* operation
// (SetCHLORParams), so deterministic output is the floor.
func TestBuildRequest_SortedOrder(t *testing.T) {
	// Run several iterations because Go map iteration is randomized;
	// without sort, this test would flake roughly (N-1)! / N! ~= 1/N of
	// the time. With sort, output is byte-identical every iteration.
	want := ""
	for i := 0; i < 20; i++ {
		got, err := buildRequest("GetSomething", map[string]any{
			"Zeta":  1,
			"Alpha": 2,
			"Mu":    "x",
			"Beta":  true,
		})
		if err != nil {
			t.Fatalf("iter %d: build err: %v", i, err)
		}
		if i == 0 {
			want = got
			// Confirm the order is alphabetical.
			expected := `<Parameter name="Alpha" dataType="int">2</Parameter><Parameter name="Beta" dataType="bool">true</Parameter><Parameter name="Mu" dataType="string">x</Parameter><Parameter name="Zeta" dataType="int">1</Parameter>`
			if !strings.Contains(got, expected) {
				t.Errorf("expected alphabetical order, got: %s", got)
			}
			continue
		}
		if got != want {
			t.Errorf("iter %d: byte-identical XML expected, got divergence:\nwant: %s\n got: %s", i, want, got)
		}
	}
}

// TestIsVSPPump verifies the type-prefix detection used by `equipment on/off`
// to route VSP pumps through the IsOn=int path instead of IsOn=bool.
// Hayward overloads IsOn on SetUIEquipmentCmd: int for VSPs, bool for
// everything else. Getting this wrong returns "Input string was not in a
// correct format" from the .NET handler.
func TestIsVSPPump(t *testing.T) {
	cfg := &MspConfig{
		BodiesOfWater: []BodyOfWater{
			{
				SystemID: "1",
				Name:     "Pool",
				Pumps: []Equipment{
					{SystemID: "3", Name: "Filter Pump", Type: "FMT_VARIABLE_SPEED_PUMP"},
					{SystemID: "6", Name: "Pressure", Type: "PMP_SINGLE_SPEED"},
					{SystemID: "7", Name: "Water Ft", Type: "PMP_SINGLE_SPEED"},
				},
			},
		},
	}
	tests := []struct {
		equipID int
		want    bool
		label   string
	}{
		{3, true, "FMT_VARIABLE_SPEED_PUMP → VSP"},
		{6, false, "PMP_SINGLE_SPEED → not VSP"},
		{7, false, "PMP_SINGLE_SPEED → not VSP"},
		{999, false, "unknown equipment ID → false (don't false-positive)"},
	}
	for _, tc := range tests {
		got := IsVSPPump(cfg, tc.equipID)
		if got != tc.want {
			t.Errorf("%s: want %v, got %v", tc.label, tc.want, got)
		}
	}

	// nil cfg defensive check
	if IsVSPPump(nil, 3) {
		t.Errorf("nil cfg should return false (no MSP config = can't classify)")
	}
}

// TestSetPumpSpeed_IsOnAsInt locks the canonical envelope shape for VSP
// pump speed: IsOn must be dataType="int" with the speed value, NOT a
// separate Speed param with IsOn=bool. Verified against the live Hayward
// API by side-by-side capture with djtimca/omnilogic-api 0.6.1.
func TestSetPumpSpeed_IsOnAsInt(t *testing.T) {
	ordered := []orderedParam{
		{"MspSystemID", "int", 12345},
		{"Version", "string", "0"},
		{"PoolID", "int", 1},
		{"EquipmentID", "int", 3},
		{"IsOn", "int", 50},
		{"IsCountDownTimer", "bool", false},
		{"StartTimeHours", "int", 0},
		{"StartTimeMinutes", "int", 0},
		{"EndTimeHours", "int", 0},
		{"EndTimeMinutes", "int", 0},
		{"DaysActive", "int", 0},
		{"Recurring", "bool", false},
	}
	xml, err := buildOrderedRequest("SetUIEquipmentCmd", ordered)
	if err != nil {
		t.Fatalf("buildOrderedRequest: %v", err)
	}
	// The IsOn-as-int signal is what Hayward parses to set a VSP's speed.
	if !strings.Contains(xml, `name="IsOn" dataType="int">50<`) {
		t.Errorf("expected IsOn as int=50, got: %s", xml)
	}
	// There must NOT be a separate Speed param — sending Speed alongside
	// IsOn=bool was the original bug that returned the .NET parse error.
	if strings.Contains(xml, `name="Speed"`) {
		t.Errorf("Speed param must not appear in VSP envelope; IsOn is overloaded: %s", xml)
	}
}
