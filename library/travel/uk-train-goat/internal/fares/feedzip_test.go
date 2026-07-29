// uk-train-goat hand-authored: tests for ParseFeedZip using an in-memory zip.
package fares

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFeedZip(t *testing.T) {
	// Build a tiny in-memory zip with one realistic record per supported extension,
	// plus one unknown extension that must be ignored.
	entries := []struct {
		name    string
		content string
	}{
		// LOC: two L records (PAD + London Terminals) and one M record.
		// locGroupMemberSample covers both ParseLOC (yields Locations) and
		// ParseLOCGroupMembers (yields GroupMembers) from the same file.
		{"RJFAF798.LOC", locGroupMemberSample},
		// FFL: one flow + two fares.
		{"RJFAF798.FFL", fflSample},
		// FSC: one cluster member.
		{"RJFAF798.FSC", fscSample},
		// NFO: one non-derivable fare (railcard row filtered out).
		{"RJFAF798.NFO", nfoSample},
		// TTY: two ticket types.
		{"RJFAF798.TTY", ttySample},
		// RLC: one railcard (blank-code row filtered out).
		{"RJFAF798.RLC", rlcSample},
		// RST: two restriction headers, one non-RRH line skipped.
		{"RJFAF798.RST", rstSample},
		// Unknown extension: must be silently ignored.
		{"RJFAF798.DAT", "some junk data"},
	}

	// Write the zip to a temp file.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "RJFAF798.ZIP")

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		fw, err := w.Create(e.name)
		if err != nil {
			t.Fatalf("zip.Create(%q): %v", e.name, err)
		}
		if _, err := fw.Write([]byte(e.content)); err != nil {
			t.Fatalf("zip write(%q): %v", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Call the function under test.
	data, err := ParseFeedZip(zipPath)
	if err != nil {
		t.Fatalf("ParseFeedZip returned error: %v", err)
	}

	// LOC: locGroupMemberSample has exactly 1 Location with a non-blank CRS (PAD);
	// London Terminals has blank CRS and is skipped by ParseLOC.
	if len(data.Locations) != 1 {
		t.Errorf("Locations: want 1, got %d", len(data.Locations))
	} else if data.Locations[0].CRS != "PAD" {
		t.Errorf("Locations[0].CRS = %q, want %q", data.Locations[0].CRS, "PAD")
	}

	// GroupMembers: locGroupMemberSample yields 1 member (PAD; VIC skipped; amend ignored).
	if len(data.GroupMembers) != 1 {
		t.Errorf("GroupMembers: want 1, got %d", len(data.GroupMembers))
	} else if data.GroupMembers[0].MemberNLC != "3087" {
		t.Errorf("GroupMembers[0].MemberNLC = %q, want %q", data.GroupMembers[0].MemberNLC, "3087")
	}

	// FFL: fflSample has 1 flow and 2 fares.
	if len(data.Flows) != 1 {
		t.Errorf("Flows: want 1, got %d", len(data.Flows))
	}
	if len(data.Fares) != 2 {
		t.Errorf("Fares: want 2, got %d", len(data.Fares))
	}

	// FSC: fscSample has 1 cluster member.
	if len(data.Clusters) != 1 {
		t.Errorf("Clusters: want 1, got %d", len(data.Clusters))
	} else if data.Clusters[0].ClusterID != "AC55" {
		t.Errorf("Clusters[0].ClusterID = %q, want %q", data.Clusters[0].ClusterID, "AC55")
	}

	// NFO: nfoSample has 1 non-derivable fare (railcard row filtered).
	if len(data.NDF) != 1 {
		t.Errorf("NDF: want 1, got %d", len(data.NDF))
	}

	// TTY: ttySample has 2 ticket types.
	if len(data.Tickets) != 2 {
		t.Errorf("Tickets: want 2, got %d", len(data.Tickets))
	}

	// RLC: rlcSample has 1 railcard (blank-code skipped).
	if len(data.Railcards) != 1 {
		t.Errorf("Railcards: want 1, got %d", len(data.Railcards))
	} else if data.Railcards[0].Code != "00D" {
		t.Errorf("Railcards[0].Code = %q, want %q", data.Railcards[0].Code, "00D")
	}

	// RST: rstSample has 2 restriction headers (non-RRH line skipped).
	if len(data.Restrictions) != 2 {
		t.Errorf("Restrictions: want 2, got %d", len(data.Restrictions))
	}
}
