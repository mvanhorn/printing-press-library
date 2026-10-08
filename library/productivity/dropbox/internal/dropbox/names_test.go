package dropbox

import "testing"

func TestConflictedCopy(t *testing.T) {
	tests := []struct {
		name, original, owner, date string
		ok                          bool
	}{
		{"report (Dana's conflicted copy 2017-03-02).docx", "report.docx", "Dana", "2017-03-02", true},
		{"report (conflicted copy 2017-03-02 (1)).docx", "report.docx", "", "2017-03-02", true},
		{"notes (Dana's conflicted copy).txt", "notes.txt", "Dana", "", true},
		{"REPORT (DANA'S CONFLICTED COPY 2017-03-02).DOCX", "REPORT.DOCX", "DANA", "2017-03-02", true},
		{"conflict.txt", "", "", "", false},
		{"my copy.txt", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b, c, ok := ConflictedCopy(tt.name)
			if a != tt.original || b != tt.owner || c != tt.date || ok != tt.ok {
				t.Fatalf("got %q %q %q %t", a, b, c, ok)
			}
		})
	}
}

func TestConflictMarker(t *testing.T) {
	tests := []struct {
		name, original, kind, owner, date string
		depth                             int
	}{
		{"Projects (Selective Sync Conflict)", "Projects", "selective_sync_conflict", "", "", 1},
		{"Projects (Selective Sync Conflict) (Selective Sync Conflict)", "Projects", "selective_sync_conflict", "", "", 2},
		{"Share (Selective Sync Conflict)", "Share", "selective_sync_conflict", "", "", 1},
		{"Footage (Selective Sync Conflict)", "Footage", "selective_sync_conflict", "", "", 1},
		{"Zed (MacBook-Pro.local's conflicted copy 2024-02-19)", "Zed", "conflicted_copy", "MacBook-Pro.local", "2024-02-19", 1},
		{"1 (MacBook-Pro.local's invalid files)", "1", "invalid_files", "MacBook-Pro.local", "", 1},
		{"Roam-Export-1656960535117 (MacBook-Pro.local's invalid files)", "Roam-Export-1656960535117", "invalid_files", "MacBook-Pro.local", "", 1},
		{"notes (Selective Sync Conflict).txt", "notes.txt", "selective_sync_conflict", "", "", 1},
		{"notes (Selective Sync Conflict 1).txt", "notes.txt", "selective_sync_conflict", "", "", 1},
		{"Notes (Case Conflict)", "Notes", "case_conflict", "", "", 1},
		{"Notes (Case Conflict 1)", "Notes", "case_conflict", "", "", 1},
		{"Conflict Resolution.pdf", "", "", "", "", 0},
		{"Selective Sync Guide.txt", "", "", "", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, kind, owner, date, depth, ok := ConflictMarkerWithDepth(tt.name)
			if original != tt.original || kind != tt.kind || owner != tt.owner || date != tt.date || depth != tt.depth || ok != (tt.depth > 0) {
				t.Fatalf("got %q %q %q %q depth=%d ok=%t", original, kind, owner, date, depth, ok)
			}
			a, b, c, d, valid := ConflictMarker(tt.name)
			if a != original || b != kind || c != owner || d != date || valid != ok {
				t.Fatalf("ConflictMarker got %q %q %q %q %t", a, b, c, d, valid)
			}
		})
	}
}

func TestNormalizeAndJunkNames(t *testing.T) {
	if NormalizeFolderName("Taxes 2") != NormalizeFolderName("taxes") || NormalizeFolderName("Taxes 2") == NormalizeFolderName("Travel") {
		t.Fatal("folder normalization")
	}
	for _, name := range []string{"Copy of budget.xlsx", "budget (1).xlsx", "Untitled document", "New Folder", "budget copy.xlsx", "budget-copy.xlsx"} {
		if _, ok := JunkName(name); !ok {
			t.Errorf("not junk: %q", name)
		}
	}
}
