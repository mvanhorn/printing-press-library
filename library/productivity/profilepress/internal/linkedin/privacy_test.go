package linkedin

import "testing"

func TestRequirePrivacyPassedBlocksUnknown(t *testing.T) {
	if _, err := RequirePrivacyPassed("unknown", false); err == nil {
		t.Fatal("unknown privacy status should block")
	}
}

func TestRequirePrivacyPassedAllowsDisabled(t *testing.T) {
	status, err := RequirePrivacyPassed("disabled", false)
	if err != nil {
		t.Fatal(err)
	}
	if status != PrivacyPassed {
		t.Fatalf("status=%s", status)
	}
}

func TestRequirePrivacyOverride(t *testing.T) {
	status, err := RequirePrivacyPassed("enabled", true)
	if err != nil {
		t.Fatal(err)
	}
	if status != PrivacyFailed {
		t.Fatalf("status=%s", status)
	}
}
