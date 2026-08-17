// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"regexp"
	"sort"
	"strings"
)

// Slack encodes mentions inside message text rather than in a structured
// field, so "did this message mention me?" is a text-parsing problem:
//
//	user mention:      <@U0EXAMPLE03>          or <@U0EXAMPLE03|alice>
//	usergroup mention: <!subteam^SAZ94GDB8>  or <!subteam^SAZ94GDB8|@eng>
//	broadcast:         <!here>, <!channel>, <!everyone>
var (
	userMentionRE      = regexp.MustCompile(`<@([A-Z0-9][A-Z0-9._-]{1,})(?:\|[^>]*)?>`)
	usergroupMentionRE = regexp.MustCompile(`<!subteam\^([A-Z0-9][A-Z0-9._-]{1,})(?:\|[^>]*)?>`)
	broadcastRE        = regexp.MustCompile(`<!(here|channel|everyone)(?:\|[^>]*)?>`)
)

// Mentions is the set of addressable targets found in one message body.
// Each slice is de-duplicated and sorted so callers get stable output.
type Mentions struct {
	Users      []string `json:"users"`
	Usergroups []string `json:"usergroups"`
	Broadcasts []string `json:"broadcasts"`
}

// Empty reports whether the message addressed nobody at all.
func (m Mentions) Empty() bool {
	return len(m.Users) == 0 && len(m.Usergroups) == 0 && len(m.Broadcasts) == 0
}

// ExtractMentions pulls every user, usergroup, and broadcast mention out of
// raw Slack message text. Unparseable or absent mentions yield empty slices,
// never nil, so JSON renderings stay [] instead of null.
func ExtractMentions(text string) Mentions {
	return Mentions{
		Users:      collect(userMentionRE, text, strings.ToUpper),
		Usergroups: collect(usergroupMentionRE, text, strings.ToUpper),
		Broadcasts: collect(broadcastRE, text, strings.ToLower),
	}
}

func collect(re *regexp.Regexp, text string, norm func(string) string) []string {
	out := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		value := norm(strings.TrimSpace(match[1]))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// MentionKind classifies why a message counts as addressed to a user.
type MentionKind string

const (
	// MentionNone means the message does not address the user.
	MentionNone MentionKind = ""
	// MentionDirect is an explicit <@Uxxx> of the user.
	MentionDirect MentionKind = "direct"
	// MentionUsergroup is a <!subteam^Sxxx> of a group the user belongs to.
	MentionUsergroup MentionKind = "usergroup"
	// MentionBroadcast is <!here>/<!channel>/<!everyone>.
	MentionBroadcast MentionKind = "broadcast"
)

// ClassifyMention reports how (if at all) text addresses userID, given the
// usergroup IDs that user belongs to. Direct mentions outrank usergroup
// mentions, which outrank channel-wide broadcasts. An empty userID can still
// match a usergroup or broadcast, so a caller without a resolved identity
// gets nothing rather than everything: pass no usergroups and
// includeBroadcasts=false in that case.
func ClassifyMention(text, userID string, usergroupIDs []string, includeBroadcasts bool) MentionKind {
	mentions := ExtractMentions(text)
	if id := strings.ToUpper(strings.TrimSpace(userID)); id != "" {
		for _, u := range mentions.Users {
			if u == id {
				return MentionDirect
			}
		}
	}
	if len(mentions.Usergroups) > 0 && len(usergroupIDs) > 0 {
		groups := map[string]bool{}
		for _, g := range usergroupIDs {
			if g = strings.ToUpper(strings.TrimSpace(g)); g != "" {
				groups[g] = true
			}
		}
		for _, g := range mentions.Usergroups {
			if groups[g] {
				return MentionUsergroup
			}
		}
	}
	if includeBroadcasts && len(mentions.Broadcasts) > 0 {
		return MentionBroadcast
	}
	return MentionNone
}
