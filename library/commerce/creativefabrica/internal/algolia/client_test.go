package algolia

import (
	"encoding/json"
	"testing"
)

func TestFlexStringUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"2.99"`, "2.99"},
		{`false`, ""},
		{`null`, ""},
		{`true`, "true"},
		{`5`, "5"},
	}
	for _, c := range cases {
		var f flexString
		if err := json.Unmarshal([]byte(c.in), &f); err != nil {
			t.Fatalf("unmarshal %s: %v", c.in, err)
		}
		if f.String() != c.want {
			t.Errorf("flexString(%s) = %q, want %q", c.in, f.String(), c.want)
		}
	}
}

func TestFlexFloatUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{`0.4`, 0.4},
		{`7`, 7},
		{`"2.99"`, 2.99},
		{`false`, 0},
		{`null`, 0},
		{`""`, 0},
	}
	for _, c := range cases {
		var f flexFloat
		if err := json.Unmarshal([]byte(c.in), &f); err != nil {
			t.Fatalf("unmarshal %s: %v", c.in, err)
		}
		if f.Float() != c.want {
			t.Errorf("flexFloat(%s) = %v, want %v", c.in, f.Float(), c.want)
		}
	}
}

func TestHitDecodesMixedTypes(t *testing.T) {
	// The live index returns price as a number for some hits and a string for
	// others, and regularPrice as false when there is no sale price.
	raw := `{"objectID":"1","name_en":"X","price":"3.5","regularPrice":false,"isFree":false,"popularity":12}`
	var h Hit
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		t.Fatalf("unmarshal hit: %v", err)
	}
	if h.Price.Float() != 3.5 {
		t.Errorf("price = %v, want 3.5", h.Price.Float())
	}
	if h.RegularPrice.String() != "" {
		t.Errorf("regularPrice = %q, want empty", h.RegularPrice.String())
	}
}

func TestClientHostDefault(t *testing.T) {
	c := New(0)
	c.Creds = Creds{AppID: "ABC123"}
	if got := c.host(); got != "https://ABC123-dsn.algolia.net" {
		t.Errorf("host = %q", got)
	}
	c.BaseURL = "http://localhost:9999"
	if got := c.host(); got != "http://localhost:9999" {
		t.Errorf("override host = %q", got)
	}
}
