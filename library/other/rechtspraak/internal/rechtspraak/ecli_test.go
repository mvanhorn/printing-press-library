package rechtspraak

import "testing"

func TestParseECLI(t *testing.T) {
	tests := []struct {
		in      string
		country string
		court   string
		year    string
		variant string
		wantErr bool
	}{
		{"ECLI:NL:HR:2024:1", "NL", "HR", "2024", "standard", false},
		{"ECLI:NL:RBAMS:2023:3197", "NL", "RBAMS", "2023", "standard", false},
		{"ECLI:NL:PHR:2023:1057", "NL", "PHR", "2023", "standard", false},
		{"ECLI:EU:C:2002:118", "EU", "C", "2002", "eu", false},
		{"ECLI:CE:ECHR:2002:1203JUD004939299", "CE", "ECHR", "2002", "echr", false},
		{"NL:HR:2024:1", "NL", "HR", "2024", "standard", false}, // no ECLI: prefix accepted
		{"ECLI:NL:HR:24:1", "", "", "", "", true},               // bad year
		{"banana", "", "", "", "", true},                        // bogus
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseECLI(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %+v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Country != tc.country || got.Court != tc.court || got.Year != tc.year {
				t.Errorf("got %+v; want country=%s court=%s year=%s", got, tc.country, tc.court, tc.year)
			}
			if got.Variant != tc.variant {
				t.Errorf("variant mismatch: got %s want %s", got.Variant, tc.variant)
			}
			if got.URL == "" {
				t.Errorf("URL not set for %s", tc.in)
			}
		})
	}
}

func TestIsLJN(t *testing.T) {
	cases := map[string]bool{
		"AA1005":            true,
		"LJN:AA1005":        true,
		"AF0535":            true,
		"ECLI:NL:HR:2024:1": false,
		"hello":             false,
	}
	for in, want := range cases {
		if got := IsLJN(in); got != want {
			t.Errorf("IsLJN(%q)=%v want %v", in, got, want)
		}
	}
}

func TestDeeplinkURL(t *testing.T) {
	url := DeeplinkURL("ECLI:NL:HR:2024:1")
	if url == "" {
		t.Fatal("expected deeplink for NL ECLI")
	}
	if DeeplinkURL("ECLI:EU:C:2002:118") != "" {
		t.Fatal("expected empty deeplink for non-NL ECLI")
	}
}
