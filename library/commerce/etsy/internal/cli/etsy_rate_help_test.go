package cli

import (
	"strings"
	"testing"
)

func TestEtsyRateLimitHelp(t *testing.T) {
	usage := RootCmd().PersistentFlags().Lookup("rate-limit").Usage
	if !strings.Contains(usage, "4.5") || !strings.Contains(usage, "x-limit-per-second") {
		t.Fatal(usage)
	}
}
