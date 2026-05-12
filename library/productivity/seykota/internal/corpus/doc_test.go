// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package corpus

import "testing"

func TestDocLabel(t *testing.T) {
	cases := []struct {
		d    Doc
		want string
	}{
		{Doc{Source: SourceFAQ, Year: "2023", Month: "JAN"}, "Ed's FAQ JAN 2023"},
		{Doc{Source: SourceFAQ}, "Ed's FAQ"},
		{Doc{Source: SourceTSP, Title: "EA Crossover", Slug: "EA"}, "TSP — EA Crossover"},
		{Doc{Source: SourceTSP, Slug: "SR"}, "TSP — SR"},
		{Doc{Source: SourceRisk, Section: "The Kelly Formula"}, "Risk essay — The Kelly Formula"},
		{Doc{Source: SourceRisk}, "Risk essay"},
	}
	for _, c := range cases {
		if got := c.d.Label(); got != c.want {
			t.Errorf("Label(%+v) = %q; want %q", c.d, got, c.want)
		}
	}
}

func TestDocDateKey(t *testing.T) {
	cases := []struct {
		d    Doc
		want string
	}{
		{Doc{Source: SourceFAQ, Year: "2007", MonthN: 7}, "2007-07"},
		{Doc{Source: SourceFAQ, Year: "2019", MonthN: 11}, "2019-11"},
		{Doc{Source: SourceFAQ, Year: "2020", MonthN: 0}, "2020-01"}, // unknown month -> 01
		{Doc{Source: SourceFAQ}, ""},                                 // no year
		{Doc{Source: SourceTSP, Slug: "EA"}, ""},
		{Doc{Source: SourceRisk}, ""},
	}
	for _, c := range cases {
		if got := c.d.DateKey(); got != c.want {
			t.Errorf("DateKey(%+v) = %q; want %q", c.d, got, c.want)
		}
	}
}
