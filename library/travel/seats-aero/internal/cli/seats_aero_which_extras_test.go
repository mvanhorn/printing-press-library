package cli

import (
	"encoding/json"
	"testing"
)

func TestAppendSeatsAeroEndpointCapabilitiesIsIdempotent(t *testing.T) {
	original := whichIndex
	whichIndex = nil
	t.Cleanup(func() { whichIndex = original })

	appendSeatsAeroEndpointCapabilities()
	appendSeatsAeroEndpointCapabilities()
	counts := map[string]int{}
	for _, entry := range whichIndex {
		counts[entry.Command]++
	}
	for _, entry := range seatsAeroEndpointCapabilities {
		if counts[entry.Command] != 1 {
			t.Fatalf("command %q count=%d, want 1", entry.Command, counts[entry.Command])
		}
	}
}

func TestWhichFindsEndpointCapabilities(t *testing.T) {
	tests := []struct {
		query        string
		want         string
		wantTopMatch bool
	}{
		{query: "search award availability", want: "awards", wantTopMatch: true},
		{query: "find award seats to Tokyo in business", want: "awards"},
		{query: "bulk availability calendar for united", want: "availability"},
		{query: "trip details for an availability id", want: "trips"},
		{query: "which routes does aeroplan track", want: "routes"},
		{query: "nonstop destinations from JFK", want: "destinations"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			matches := rankWhich(whichIndex, tt.query, 3)
			if tt.wantTopMatch {
				if len(matches) == 0 || matches[0].Entry.Command != tt.want {
					t.Fatalf("rankWhich(%q) top match = %v, want %q", tt.query, matches, tt.want)
				}
				return
			}

			for _, match := range matches {
				if match.Entry.Command == tt.want {
					return
				}
			}
			t.Fatalf("rankWhich(%q) top 3 = %v, want %q included", tt.query, matches, tt.want)
		})
	}

	novelCommands := map[string]bool{
		"new-since":   false,
		"calendar":    false,
		"direct-scan": false,
		"reach":       false,
		"recheck":     false,
	}
	for _, entry := range whichIndex {
		if _, ok := novelCommands[entry.Command]; ok {
			novelCommands[entry.Command] = true
		}
	}
	for command, found := range novelCommands {
		if !found {
			t.Errorf("whichIndex no longer contains novel command %q", command)
		}
	}

	isolateNovelTest(t)
	stdout, stderr, err := executeRoot("which", "search award availability", "--agent")
	if err != nil {
		t.Fatalf("which command returned error: %v (stderr=%q)", err, stderr.String())
	}
	var envelope struct {
		Results struct {
			Matches []struct {
				Entry whichEntry `json:"entry"`
			} `json:"matches"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode which JSON %q: %v", stdout.String(), err)
	}
	if len(envelope.Results.Matches) == 0 || envelope.Results.Matches[0].Entry.Command != "awards" {
		t.Fatalf("which command matches = %+v, want awards first", envelope.Results.Matches)
	}
}
