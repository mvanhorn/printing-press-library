// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/service"
	"github.com/spf13/cobra"
	"strings"
	"testing"
)

func TestServiceWarningDescribesActualOrigin(t *testing.T) {
	for _, tc := range []struct {
		transport, message string
		live               bool
	}{{"local", "Reinterpreted saved request/fee evidence; source observation time is unchanged.", false}, {"local_fallback", "Reinterpreted saved request/fee evidence; source observation time is unchanged.", false}, {"live", "cache is not writable", true}} {
		var err bytes.Buffer
		c := &cobra.Command{}
		c.SetErr(&err)
		serviceWarn(c, service.Service{ID: "fixture", Transport: tc.transport, CacheWarning: &tc.message})
		s := err.String()
		if !strings.Contains(s, tc.message) || (!tc.live && (strings.Contains(s, "source read succeeded") || strings.Contains(s, "local save failed"))) || (tc.live && !strings.Contains(s, "local save failed")) {
			t.Fatalf("false warning origin: %q", s)
		}
	}
}
