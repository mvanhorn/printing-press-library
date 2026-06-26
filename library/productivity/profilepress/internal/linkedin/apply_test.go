package linkedin

import "testing"

func TestNotImplementedAdapterBlocksLiveApply(t *testing.T) {
	if err := (NotImplementedAdapter{}).Apply("headline", "new"); err == nil {
		t.Fatal("live apply should be blocked until adapter is implemented")
	}
}

func TestDryRunAdapterAllowsChecksOnly(t *testing.T) {
	if err := (DryRunAdapter{}).Apply("headline", "new"); err != nil {
		t.Fatal(err)
	}
}
