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
		{`name="Enabled" dataType="bool">True<`, "bool param"},
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
