// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"regexp"
	"strings"
)

// RefKind classifies the shape of a user reference typed on the command line.
type RefKind string

const (
	// RefID is an opaque Slack ID: U…, W… (enterprise), or B… (bot).
	RefID RefKind = "id"
	// RefEmail is an email address.
	RefEmail RefKind = "email"
	// RefHandle is an @handle / display name / real name fragment.
	RefHandle RefKind = "handle"
	// RefEmpty is the zero reference.
	RefEmpty RefKind = ""
)

// slackIDRE matches the ID forms Slack issues for people and bots. Channel
// IDs (C…, G…, D…) are deliberately excluded: a channel is not a user.
var slackIDRE = regexp.MustCompile(`^[UWB][A-Z0-9]{2,}$`)

// UserRef is a normalized user reference: the raw token the caller typed
// plus the resolved kind and a comparison-ready value.
type UserRef struct {
	Raw   string  `json:"raw"`
	Kind  RefKind `json:"kind"`
	Value string  `json:"value"`
}

// ParseUserRef classifies "U04AB9XYZ", "@alice", "alice", and
// "alice@example.com" into a comparable reference. IDs normalize to
// upper case; handles and emails to lower case with a leading @ stripped.
func ParseUserRef(s string) UserRef {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return UserRef{Raw: s, Kind: RefEmpty}
	}
	// Accept a pasted mention (<@U123|alice>) as an ID reference.
	if m := ExtractMentions(trimmed); len(m.Users) == 1 && strings.HasPrefix(trimmed, "<@") {
		return UserRef{Raw: s, Kind: RefID, Value: m.Users[0]}
	}
	bare := strings.TrimPrefix(trimmed, "@")
	if strings.Contains(bare, "@") && strings.Contains(bare, ".") {
		return UserRef{Raw: s, Kind: RefEmail, Value: strings.ToLower(bare)}
	}
	if slackIDRE.MatchString(strings.ToUpper(bare)) && !strings.HasPrefix(trimmed, "@") {
		return UserRef{Raw: s, Kind: RefID, Value: strings.ToUpper(bare)}
	}
	return UserRef{Raw: s, Kind: RefHandle, Value: strings.ToLower(bare)}
}

// UserIdentity is the subset of a Slack user record needed to match a
// reference against a locally mirrored user.
type UserIdentity struct {
	ID          string
	Handle      string
	DisplayName string
	RealName    string
	Email       string
}

// Matches reports whether ref identifies this user. ID references compare
// exactly; email references compare case-insensitively; handle references
// match the handle, display name, or real name case-insensitively (real
// names also match with internal whitespace collapsed, so "Alice Adams"
// resolves from "alice adams").
func (u UserIdentity) Matches(ref UserRef) bool {
	switch ref.Kind {
	case RefID:
		return strings.EqualFold(u.ID, ref.Value)
	case RefEmail:
		return u.Email != "" && strings.EqualFold(u.Email, ref.Value)
	case RefHandle:
		for _, candidate := range []string{u.Handle, u.DisplayName, u.RealName} {
			if candidate == "" {
				continue
			}
			if strings.EqualFold(strings.TrimPrefix(candidate, "@"), ref.Value) {
				return true
			}
		}
		// An ID typed with a leading @ still resolves rather than 404ing.
		return strings.EqualFold(u.ID, ref.Value)
	default:
		return false
	}
}

// DisplayLabel picks the friendliest available name for a user, falling back
// to the raw ID so output never renders an empty cell.
func (u UserIdentity) DisplayLabel() string {
	for _, candidate := range []string{u.DisplayName, u.RealName, u.Handle} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return u.ID
}
