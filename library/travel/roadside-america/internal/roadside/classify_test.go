package roadside

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		want []string // categories that must be present
	}{
		{"Swampy: World's Largest Alligator", []string{"biggest", "animals"}},
		{"World's Smallest Police Station", []string{"smallest"}},
		{"Tallest Filing Cabinet", []string{"tallest"}},
		{"Giant Donut", []string{"biggest", "weird-food"}},
		{"Muffler Man Holding a Hot Dog", []string{"muffler-men", "weird-food"}},
		{"Vintage Neon Motel Sign", []string{"signs"}},
		{"Dinosaur Park", []string{"animals"}},
		{"Just a Normal House", nil},
	}
	for _, c := range cases {
		got := Classify(Attraction{Name: c.name})
		for _, w := range c.want {
			if !contains(got, w) {
				t.Errorf("Classify(%q) = %v, missing %q", c.name, got, w)
			}
		}
		if c.want == nil && len(got) != 0 {
			t.Errorf("Classify(%q) = %v, want none", c.name, got)
		}
	}
}

func TestNormalizeCategory(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"biggest", "biggest", true},
		{"big", "biggest", true},
		{"largest", "biggest", true},
		{"weird-food", "weird-food", true},
		{"food", "weird-food", true},
		{"WEIRD FOOD", "weird-food", true},
		{"muffler man", "muffler-men", true},
		{"tallest", "tallest", true},
		{"nonsense", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeCategory(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("NormalizeCategory(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestMatchesCategory(t *testing.T) {
	a := Attraction{Name: "World's Largest Ball of Twine"}
	if !MatchesCategory(a, "biggest") {
		t.Errorf("expected %q to match biggest", a.Name)
	}
	if MatchesCategory(a, "smallest") {
		t.Errorf("did not expect %q to match smallest", a.Name)
	}
}

func TestCategories(t *testing.T) {
	cats := Categories()
	if len(cats) == 0 {
		t.Fatal("Categories() returned none")
	}
	if cats[0].Name != "biggest" {
		t.Errorf("first category should be biggest, got %q", cats[0].Name)
	}
	for _, c := range cats {
		if c.re != nil {
			t.Errorf("Categories() must not leak the compiled regexp for %q", c.Name)
		}
	}
}
