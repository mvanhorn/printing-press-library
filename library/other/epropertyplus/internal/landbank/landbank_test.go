// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

package landbank

import (
	"encoding/json"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name          string
		structureType string
		propertyClass string
		want          Kind
	}{
		{"vacant lot", "", "Residential Vacant", KindLot},
		{"vacant side lot", "", "Residential Vacant - Side Lot", KindLot},
		{"vacant lowercase", "", "residential vacant", KindLot},
		{"structure via structureType", "Single Family", "Residential Vacant", KindStructure},
		{"structure via non-vacant class", "", "Commercial Improved", KindStructure},
		{"empty class is structure", "", "", KindStructure},
		{"whitespace structureType ignored", "   ", "Residential Vacant", KindLot},
		{"mixed case Vacant in middle", "", "Some VACANT thing", KindLot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.structureType, tt.propertyClass); got != tt.want {
				t.Errorf("Classify(%q,%q) = %q, want %q", tt.structureType, tt.propertyClass, got, tt.want)
			}
		})
	}
}

func TestClassifyDisjoint(t *testing.T) {
	// Every property must be exactly one of structure or lot — never both,
	// never neither. This guards the disjointness the acceptance test asserts.
	cases := [][2]string{
		{"", "Residential Vacant"},
		{"Single Family", "Residential Vacant"},
		{"", ""},
		{"", "Commercial"},
		{"Duplex", ""},
	}
	for _, c := range cases {
		p := Property{StructureType: c[0], PropertyClass: c[1]}
		if p.IsStructure() == p.IsLot() {
			t.Errorf("property %+v is both/neither structure and lot", c)
		}
	}
}

func TestMatchesKind(t *testing.T) {
	lot := Property{PropertyClass: "Residential Vacant"}
	str := Property{StructureType: "Single Family", PropertyClass: "Residential Vacant"}

	if !lot.MatchesKind("lot") || lot.MatchesKind("structure") {
		t.Error("lot kind matching wrong")
	}
	if !str.MatchesKind("structure") || str.MatchesKind("lot") {
		t.Error("structure kind matching wrong")
	}
	if !lot.MatchesKind("all") || !str.MatchesKind("") {
		t.Error("all/empty should match everything")
	}
	if lot.MatchesKind("bogus") {
		t.Error("unknown kind should not match")
	}
}

func TestParseIndex(t *testing.T) {
	raw := `{"success":true,"size":2,"rows":[{"id":47394,"lt":39.04,"lg":-94.47,"s":null,"c":null},{"id":47395,"lt":39.05,"lg":-94.46,"s":null,"c":null}]}`
	idx, err := ParseIndex([]byte(raw))
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if idx.Size != 2 || len(idx.Rows) != 2 {
		t.Fatalf("got size=%d rows=%d", idx.Size, len(idx.Rows))
	}
	if idx.Rows[0].ID.String() != "47394" {
		t.Errorf("row id = %q", idx.Rows[0].ID.String())
	}
	if idx.Rows[0].Lat == nil || *idx.Rows[0].Lat != 39.04 {
		t.Errorf("row lat wrong")
	}
}

func TestParseDetail(t *testing.T) {
	raw := `{"success":true,"returnVal":{"id":47394,"propertyClass":"Residential Vacant","structureType":null,"latitude":39.04,"longitude":-94.47,"zonedAs":"R-2-5"}}`
	inner, err := ParseDetail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDetail: %v", err)
	}
	p, err := ParseProperty(inner)
	if err != nil {
		t.Fatalf("ParseProperty: %v", err)
	}
	if p.ID.String() != "47394" || p.ZonedAs != "R-2-5" {
		t.Errorf("parsed wrong: %+v", p)
	}
	if !p.IsLot() {
		t.Error("expected lot")
	}
}

func TestParseDetailBareObject(t *testing.T) {
	// When given a bare property object (no envelope), ParseDetail returns it as-is.
	raw := `{"id":1,"propertyClass":"Commercial"}`
	inner, err := ParseDetail([]byte(raw))
	if err != nil {
		t.Fatalf("ParseDetail bare: %v", err)
	}
	p, _ := ParseProperty(inner)
	if !p.IsStructure() {
		t.Error("commercial should be structure")
	}
}

func TestBuildFeatureCollection(t *testing.T) {
	lat1, lng1 := 39.04, -94.47
	records := []FeatureInput{
		{Lat: lat1, Lng: lng1, HasCoords: true, Props: map[string]any{"id": "1"}},
		{HasCoords: false, Props: map[string]any{"id": "2"}}, // skipped: no coords
		{Lat: 40.0, Lng: -90.0, HasCoords: true, Props: map[string]any{"id": "3"}},
	}
	fc := BuildFeatureCollection(records)
	if fc.Type != "FeatureCollection" {
		t.Fatalf("type = %q", fc.Type)
	}
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 features (coordless skipped), got %d", len(fc.Features))
	}
	f := fc.Features[0]
	if f.Type != "Feature" || f.Geometry.Type != "Point" {
		t.Errorf("feature/geometry type wrong: %+v", f)
	}
	// GeoJSON axis order: [lng, lat].
	if f.Geometry.Coordinates[0] != lng1 || f.Geometry.Coordinates[1] != lat1 {
		t.Errorf("coords = %v, want [%v %v]", f.Geometry.Coordinates, lng1, lat1)
	}
	// Round-trips as valid JSON.
	b, err := MarshalFeatureCollection(fc)
	if err != nil || !json.Valid(b) {
		t.Fatalf("marshal invalid: %v", err)
	}
}

func TestFeatureInputFromPropertySelect(t *testing.T) {
	lat, lng := 39.0, -94.0
	p := Property{
		ID:            json.Number("7"),
		PropertyClass: "Residential Vacant",
		Latitude:      &lat,
		Longitude:     &lng,
		ZonedAs:       "R-2-5",
	}
	in := FeatureInputFromProperty(p, map[string]bool{"id": true, "zonedAs": true})
	if !in.HasCoords {
		t.Fatal("expected coords")
	}
	if _, ok := in.Props["id"]; !ok {
		t.Error("id should be kept")
	}
	if _, ok := in.Props["city"]; ok {
		t.Error("city should be filtered out by select")
	}
}

func TestResolveBaseURL(t *testing.T) {
	tests := []struct {
		name       string
		in         ResolveInput
		wantSource string
		wantURL    string
	}{
		{
			"flag wins over everything",
			ResolveInput{FlagInstance: "detroit", EnvBaseURL: "https://x", EnvInstance: "y", RegistryCurr: "z"},
			"flag", BaseURLForSlug("detroit"),
		},
		{
			"env base url beats env instance",
			ResolveInput{EnvBaseURL: "https://mock.test/api/", EnvInstance: "y", RegistryCurr: "z"},
			"env_base_url", "https://mock.test/api",
		},
		{
			"env instance beats registry",
			ResolveInput{EnvInstance: "buffalo", RegistryCurr: "z"},
			"env_instance", BaseURLForSlug("buffalo"),
		},
		{
			"registry beats default",
			ResolveInput{RegistryCurr: "flint", DefaultSlugIn: "kclb"},
			"registry", BaseURLForSlug("flint"),
		},
		{
			"default fallback",
			ResolveInput{},
			"default", BaseURLForSlug(DefaultSlug),
		},
		{
			"whitespace treated as unset",
			ResolveInput{FlagInstance: "  ", EnvInstance: "  ", RegistryCurr: "real"},
			"registry", BaseURLForSlug("real"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBaseURL(tt.in)
			if got.Source != tt.wantSource {
				t.Errorf("source = %q, want %q", got.Source, tt.wantSource)
			}
			if got.BaseURL != tt.wantURL {
				t.Errorf("url = %q, want %q", got.BaseURL, tt.wantURL)
			}
		})
	}
}

func TestBaseURLForSlug(t *testing.T) {
	want := "https://public-kclb.epropertyplus.com/landmgmtpub/remote/public"
	if got := BaseURLForSlug("kclb"); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
