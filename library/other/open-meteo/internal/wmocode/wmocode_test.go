package wmocode

import "testing"

func TestDescribeCovers(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "Clear sky"},
		{1, "Mainly clear"},
		{61, "Slight rain"},
		{95, "Thunderstorm"},
		{99, "Thunderstorm with heavy hail"},
		{1234, "Code 1234"},
	}
	for _, c := range cases {
		if got := Describe(c.code); got != c.want {
			t.Errorf("Describe(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestBucket(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "clear"},
		{1, "clear"},
		{2, "partly_cloudy"},
		{3, "overcast"},
		{45, "fog"},
		{53, "drizzle"},
		{63, "rain"},
		{73, "snow"},
		{81, "showers"},
		{96, "thunderstorm"},
		{1000, "unknown"},
	}
	for _, c := range cases {
		if got := Bucket(c.code); got != c.want {
			t.Errorf("Bucket(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestAllReturnsCopy(t *testing.T) {
	a := All()
	b := All()
	if len(a) != len(b) {
		t.Fatalf("All() returned different sizes: %d vs %d", len(a), len(b))
	}
	a[0] = "mutated"
	if All()[0] != "Clear sky" {
		t.Error("mutating returned map corrupted package state")
	}
}
