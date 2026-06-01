package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestITOverviewJSONDescribesSupportedAPIsAndGaps(t *testing.T) {
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"it", "overview", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("it overview --json returned error: %v\noutput:\n%s", err, out.String())
	}

	var got struct {
		Summary             string `json:"summary"`
		SupportedPublicAPIs []struct {
			Area     string   `json:"area"`
			Commands []string `json:"commands"`
		} `json:"supported_public_apis"`
		KnownPublicAPIGaps []string `json:"known_public_api_gaps"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("overview output is not JSON: %v\n%s", err, out.String())
	}
	if !strings.Contains(got.Summary, "draft hires") || !strings.Contains(got.Summary, "software deployments") {
		t.Fatalf("summary should name public IT APIs, got %q", got.Summary)
	}
	if len(got.SupportedPublicAPIs) < 3 {
		t.Fatalf("expected at least three supported/adjacent IT areas, got %+v", got.SupportedPublicAPIs)
	}
	if len(got.KnownPublicAPIGaps) == 0 {
		t.Fatalf("expected public API gaps to be explicit")
	}
}

func TestWhichRoutesITQueriesToITWorkflow(t *testing.T) {
	matches := rankWhich(whichIndex, "order laptop for new hire device inventory", 5)
	if len(matches) == 0 {
		t.Fatalf("expected IT workflow match, got none")
	}
	for _, match := range matches {
		if strings.HasPrefix(match.Entry.Command, "it ") {
			return
		}
	}
	t.Fatalf("expected at least one it workflow command in top matches, got %+v", matches)
}
