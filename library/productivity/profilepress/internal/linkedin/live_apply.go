package linkedin

import (
	"errors"
	"fmt"
	"strings"
)

const (
	RequestSaveProfileIntro = "com.linkedin.sdui.requests.profile.saveProfileIntroForm"
	RequestSaveProfileAbout = "com.linkedin.sdui.requests.profile.saveProfileAboutForm"
)

// SectionChange is the narrow shape the LinkedIn live writer needs from a
// ProfilePress packet. It deliberately avoids importing packet to keep the
// LinkedIn adapter independent of storage/packet concerns.
type SectionChange struct {
	Section string
	After   string
}

// RequireLiveApplySupported validates every section before the first live
// LinkedIn write. This prevents partial writes when a packet mixes supported
// top-card/about fields with unsupported blob sections like experience.
func RequireLiveApplySupported(changes []SectionChange) error {
	var unsupported []string
	for _, ch := range changes {
		switch canonicalApplySection(ch.Section) {
		case "headline", "about":
			// supported
		default:
			unsupported = append(unsupported, ch.Section)
		}
	}
	if len(unsupported) > 0 {
		return fmt.Errorf("live LinkedIn apply does not support section(s) %s yet; supported sections are headline and about; experience requires position-level packets", strings.Join(unsupported, ", "))
	}
	return nil
}

func canonicalApplySection(section string) string {
	return strings.ToLower(strings.TrimSpace(section))
}

// BuildSDUIServerRequest builds the request object LinkedIn's SDUI runtime
// posts to /rsc-action/actions/server-request?sduiid=<requestId>.
func BuildSDUIServerRequest(requestID string, payload map[string]any) (map[string]any, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, errors.New("request ID is required")
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return map[string]any{
		"requestId": requestID,
		"requestedArguments": map[string]any{
			"$type":              "proto.sdui.actions.requests.RequestedArguments",
			"payload":            payload,
			"requestedStateKeys": []any{},
			"requestMetadata": map[string]any{
				"$type": "proto.sdui.common.RequestMetadata",
			},
		},
		"isStreaming":       false,
		"isApfcEnabled":     false,
		"rumPageKey":        "",
		"maxRetries":        0,
		"backOffMultiplier": 0,
		"maxSeconds":        0,
	}, nil
}

func IntroPayload(profileID, vanityName, firstName, lastName, headline, initialHeadline string) map[string]any {
	return map[string]any{
		"profileId":                          profileID,
		"vanityName":                         vanityName,
		"firstName":                          firstName,
		"lastName":                           lastName,
		"headline":                           headline,
		"initialHeadline":                    initialHeadline,
		"hasChanges":                         true,
		"premiumUpsellEligible":              false,
		"verificationNbaEligible":            false,
		"isRefreshRequiredAfterSave":         true,
		"showCurrentPosition":                true,
		"showEducation":                      false,
		"showOpenProfile":                    false,
		"showPremiumBadge":                   false,
		"additionalName":                     "",
		"customPronouns":                     "",
		"pronouns":                           []any{},
		"additionalNameVisibilitySetting":    "AdditionalNamePronunciationVisibilityEnumValue_HIDDEN",
		"pronounsVisibilitySetting":          "PronounVisibilityEnumValue_MEMBERS",
		"namePronunciationVisibilitySetting": "NamePronunciationVisibilityEnumValue_MEMBERS",
	}
}

func AboutPayload(profileID, vanityName, about, initialAbout string) map[string]any {
	return map[string]any{
		"profileId":             profileID,
		"vanityName":            vanityName,
		"about":                 about,
		"initialAbout":          initialAbout,
		"skills":                []any{},
		"initialSkills":         []any{},
		"hasChanges":            true,
		"premiumUpsellEligible": false,
		"hasAtLeastOneTopSkillLackingAssociations": false,
	}
}
