// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// PATCH(amend-2026-11-03: confirm-button choice must not depend on map order) --
// the page-level "Submit Report" button's ref can change between snapshots, so
// it can slip past excludeRef; the dialog's "Proceed" must still win.
func TestWaitForEitherRefExcluding_PriorityBeatsRefOrder(t *testing.T) {
	tmpDir := t.TempDir()
	script := `#!/bin/bash
for a in "$@"; do
	if [ "$a" = "snapshot" ]; then
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Submit Report","role":"button"},"e2":{"name":"Cancel","role":"button"},"e9":{"name":"Proceed","role":"button"}}}}'
		exit 0
	fi
done
exit 0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "agent-browser"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+os.Getenv("PATH"))

	// The excluded ref (e5) no longer matches the page button's current ref (e1).
	for i := 0; i < 20; i++ {
		ref, matched, err := waitForEitherRefExcluding([]string{"Proceed", "Submit Report"}, "button", "e5", 2*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref != "e9" || matched != "Proceed" {
			t.Fatalf("iteration %d: got ref %q (%q), want e9 (Proceed)", i, ref, matched)
		}
	}
}
