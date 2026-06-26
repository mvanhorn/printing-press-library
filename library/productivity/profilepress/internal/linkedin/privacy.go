package linkedin

import "fmt"

type PrivacyStatus string

const (
	PrivacyPassed  PrivacyStatus = "passed"
	PrivacyFailed  PrivacyStatus = "failed"
	PrivacyUnknown PrivacyStatus = "unknown"
)

func EvaluatePrivacy(raw string) PrivacyStatus {
	switch raw {
	case "disabled", "off", "passed", "safe":
		return PrivacyPassed
	case "enabled", "on", "failed", "unsafe":
		return PrivacyFailed
	default:
		return PrivacyUnknown
	}
}

func RequirePrivacyPassed(raw string, override bool) (PrivacyStatus, error) {
	status := EvaluatePrivacy(raw)
	if status == PrivacyPassed {
		return status, nil
	}
	if override {
		return status, nil
	}
	return status, fmt.Errorf("privacy preflight blocked apply: profile-update broadcast status is %s", status)
}
