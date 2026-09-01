package results

import "testing"

func TestOperationResultHasStableEnvelopeAndRedactsSecrets(t *testing.T) {
	got := Build("kvm_send_keys", "kvm", false, "host-a", true, true, "completed", map[string]any{
		"approval_token": "secret-token", "combo": "Ctrl+Alt+T",
	}, nil)
	if got.Operation != "kvm_send_keys" || got.Target == nil || *got.Target != "host-a" || !got.Changed || !got.OK || got.State != "completed" {
		t.Fatalf("unexpected envelope: %+v", got)
	}
	if _, ok := got.Evidence["approval_token"]; ok {
		t.Fatal("secret leaked into evidence")
	}
	if got.Warnings == nil || got.NextActions == nil {
		t.Fatal("stable slices must be non-nil")
	}
}

func TestOperationResultNormalizesErrors(t *testing.T) {
	got := Build("select", "kvm", false, "host-a", false, false, "aborted", nil, &Error{Code: "target mismatch", Retryable: true})
	if got.Error == nil || got.Error.Code != "target mismatch" || !got.Error.Retryable {
		t.Fatalf("unexpected error: %+v", got.Error)
	}
}
