// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	maxMessageParts     = 100
	maxPublicURLMedia   = 40
	maxTextValueChars   = 10000
	maxLinkValueChars   = 2048
	maxIdempotencyChars = 255
)

var validPreferredServices = map[string]bool{
	"":         true,
	"iMessage": true,
	"RCS":      true,
	"SMS":      true,
}

var screenEffects = map[string]bool{
	"confetti": true, "fireworks": true, "lasers": true, "sparkles": true,
	"celebration": true, "hearts": true, "love": true, "balloons": true,
	"happy_birthday": true, "echo": true, "spotlight": true,
}

var bubbleEffects = map[string]bool{
	"slam": true, "loud": true, "gentle": true, "invisible": true,
}

var textDecorationStyles = map[string]bool{
	"bold": true, "italic": true, "strikethrough": true, "underline": true,
}

var textDecorationAnimations = map[string]bool{
	"big": true, "small": true, "shake": true, "nod": true,
	"explode": true, "ripple": true, "bloom": true, "jitter": true,
}

type linqMessageBuildOptions struct {
	Texts               []string
	MediaURLs           []string
	AttachmentIDs       []string
	Link                string
	Effect              string
	Decorations         []string
	PreferredService    string
	ReplyToMessageID    string
	ReplyToPartIndex    int
	HasReplyToPartIndex bool
	IdempotencyKey      string
}

type linqMessageBuildResult struct {
	Body             map[string]any    `json:"body"`
	Sendable         bool              `json:"sendable"`
	Errors           []string          `json:"errors,omitempty"`
	ProtocolWarnings []string          `json:"protocol_warnings,omitempty"`
	Limits           map[string]string `json:"limits"`
	Docs             []string          `json:"docs,omitempty"`
}

func buildLinqMessageBody(opts linqMessageBuildOptions) linqMessageBuildResult {
	var errorsOut []string
	var warnings []string
	message := map[string]any{}
	parts := []map[string]any{}

	preferred := normalizePreferredService(opts.PreferredService)
	if !validPreferredServices[preferred] {
		errorsOut = append(errorsOut, "--preferred-service must be iMessage, RCS, SMS, or omitted")
	} else if preferred != "" {
		message["preferred_service"] = preferred
	}

	link := strings.TrimSpace(opts.Link)
	if link != "" {
		part := map[string]any{"type": "link", "value": link}
		parts = append(parts, part)
		if len([]rune(link)) > maxLinkValueChars {
			errorsOut = append(errorsOut, "link value exceeds 2,048 characters")
		}
		if !isHTTPSURL(link) {
			errorsOut = append(errorsOut, "link parts require an HTTPS URL")
		}
		if len(opts.Texts)+len(opts.MediaURLs)+len(opts.AttachmentIDs) > 0 {
			errorsOut = append(errorsOut, "link parts must be the only part in a message")
		}
		if preferred == "SMS" {
			warnings = append(warnings, "rich link previews fall back to a bare URL on SMS")
		}
	} else {
		for _, text := range opts.Texts {
			part := map[string]any{"type": "text", "value": text}
			parts = append(parts, part)
			if len([]rune(text)) > maxTextValueChars {
				errorsOut = append(errorsOut, "text value exceeds 10,000 characters")
			}
		}
		for _, mediaURL := range opts.MediaURLs {
			mediaURL = strings.TrimSpace(mediaURL)
			parts = append(parts, map[string]any{"type": "media", "url": mediaURL})
			if !isHTTPSURL(mediaURL) {
				errorsOut = append(errorsOut, "media URL parts require HTTPS")
			}
		}
		for _, attachmentID := range opts.AttachmentIDs {
			attachmentID = strings.TrimSpace(attachmentID)
			if attachmentID == "" {
				errorsOut = append(errorsOut, "attachment IDs must not be empty")
				continue
			}
			parts = append(parts, map[string]any{"type": "media", "attachment_id": attachmentID})
		}
	}

	if len(parts) == 0 {
		errorsOut = append(errorsOut, "at least one message part is required")
	}
	if len(parts) > maxMessageParts {
		errorsOut = append(errorsOut, "messages may contain at most 100 parts")
	}
	publicURLMedia := countPublicURLMedia(parts)
	if publicURLMedia > maxPublicURLMedia {
		errorsOut = append(errorsOut, "messages may contain at most 40 public-URL media parts")
	}
	for i := 1; i < len(parts); i++ {
		if parts[i-1]["type"] == "text" && parts[i]["type"] == "text" {
			errorsOut = append(errorsOut, "consecutive text parts are not allowed")
			break
		}
	}

	if len(opts.Decorations) > 0 {
		if link != "" {
			errorsOut = append(errorsOut, "text decorations require a text message part")
		} else if err := applyTextDecorations(parts, opts.Decorations); err != nil {
			errorsOut = append(errorsOut, err.Error())
		}
		if preferred != "" && preferred != "iMessage" {
			warnings = append(warnings, "text decorations are iMessage-only and are ignored on RCS/SMS")
		} else {
			warnings = append(warnings, "text decoration ranges are UTF-16 code units; decorations are iMessage-only")
		}
	}

	if strings.TrimSpace(opts.Effect) != "" {
		effect, err := parseMessageEffect(opts.Effect)
		if err != nil {
			errorsOut = append(errorsOut, err.Error())
		} else {
			message["effect"] = effect
		}
		if preferred != "" && preferred != "iMessage" {
			warnings = append(warnings, "message effects are iMessage-only and are ignored on RCS/SMS")
		} else {
			warnings = append(warnings, "message effects are iMessage-only")
		}
	}

	if replyID := strings.TrimSpace(opts.ReplyToMessageID); replyID != "" {
		partIndex := 0
		if opts.HasReplyToPartIndex {
			partIndex = opts.ReplyToPartIndex
		}
		if partIndex < 0 {
			errorsOut = append(errorsOut, "reply-to part index must be 0 or greater")
		}
		message["reply_to"] = map[string]any{"message_id": replyID, "part_index": partIndex}
	}
	if key := strings.TrimSpace(opts.IdempotencyKey); key != "" {
		if len([]rune(key)) > maxIdempotencyChars {
			errorsOut = append(errorsOut, "idempotency key exceeds 255 characters")
		}
		message["idempotency_key"] = key
	}

	message["parts"] = parts
	body := map[string]any{"message": message}
	sort.Strings(errorsOut)
	warnings = dedupeStrings(warnings)
	return linqMessageBuildResult{
		Body:             body,
		Sendable:         len(errorsOut) == 0,
		Errors:           errorsOut,
		ProtocolWarnings: warnings,
		Limits: map[string]string{
			"parts":            fmt.Sprintf("%d/%d", len(parts), maxMessageParts),
			"public_url_media": fmt.Sprintf("%d/%d", publicURLMedia, maxPublicURLMedia),
			"text_value":       fmt.Sprintf("max %d characters per text part", maxTextValueChars),
			"link_value":       fmt.Sprintf("max %d characters per link part", maxLinkValueChars),
			"idempotency_key":  fmt.Sprintf("max %d characters", maxIdempotencyChars),
		},
		Docs: []string{
			"https://docs.linqapp.com/guides/messaging/sending-messages/",
			"https://docs.linqapp.com/guides/messaging/protocol-selection/",
		},
	}
}

func requireSendableMessage(result linqMessageBuildResult) error {
	if result.Sendable {
		return nil
	}
	return usageErr(fmt.Errorf("message is not sendable: %s", strings.Join(result.Errors, "; ")))
}

func normalizePreferredService(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "imessage":
		return "iMessage"
	case "rcs":
		return "RCS"
	case "sms":
		return "SMS"
	default:
		return strings.TrimSpace(raw)
	}
}

func isHTTPSURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u != nil && u.Scheme == "https" && u.Host != ""
}

func countPublicURLMedia(parts []map[string]any) int {
	count := 0
	for _, part := range parts {
		if part["type"] == "media" {
			if _, ok := part["url"]; ok {
				count++
			}
		}
	}
	return count
}

func parseMessageEffect(raw string) (map[string]any, error) {
	effectType, name, ok := strings.Cut(strings.TrimSpace(raw), ":")
	if !ok || strings.TrimSpace(effectType) == "" || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("--effect must use TYPE:NAME, for example screen:confetti or bubble:slam")
	}
	effectType = strings.ToLower(strings.TrimSpace(effectType))
	name = strings.ToLower(strings.TrimSpace(name))
	switch effectType {
	case "screen":
		if !screenEffects[name] {
			return nil, fmt.Errorf("unsupported screen effect %q", name)
		}
	case "bubble":
		if !bubbleEffects[name] {
			return nil, fmt.Errorf("unsupported bubble effect %q", name)
		}
	default:
		return nil, fmt.Errorf("effect type must be screen or bubble")
	}
	return map[string]any{"type": effectType, "name": name}, nil
}

type parsedTextDecoration struct {
	Start     int
	End       int
	Style     string
	Animation string
}

func applyTextDecorations(parts []map[string]any, specs []string) error {
	var textPart map[string]any
	for _, part := range parts {
		if part["type"] == "text" {
			textPart = part
			break
		}
	}
	if textPart == nil {
		return fmt.Errorf("text decorations require a text part")
	}
	text, _ := textPart["value"].(string)
	textLen := len(utf16.Encode([]rune(text)))
	parsed := make([]parsedTextDecoration, 0, len(specs))
	for _, spec := range specs {
		decoration, err := parseTextDecoration(spec, textLen)
		if err != nil {
			return err
		}
		parsed = append(parsed, decoration)
	}
	for i, left := range parsed {
		if left.Animation == "" {
			continue
		}
		for j, right := range parsed {
			if i == j {
				continue
			}
			if rangesOverlap(left.Start, left.End, right.Start, right.End) {
				return fmt.Errorf("animation decoration ranges must not overlap with other animations or styles")
			}
		}
	}
	out := make([]map[string]any, 0, len(parsed))
	for _, decoration := range parsed {
		item := map[string]any{"range": []int{decoration.Start, decoration.End}}
		if decoration.Style != "" {
			item["style"] = decoration.Style
		}
		if decoration.Animation != "" {
			item["animation"] = decoration.Animation
		}
		out = append(out, item)
	}
	textPart["text_decorations"] = out
	return nil
}

func parseTextDecoration(raw string, textLen int) (parsedTextDecoration, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 3 {
		return parsedTextDecoration{}, fmt.Errorf("--decorate must use START:END:STYLE_OR_ANIMATION")
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return parsedTextDecoration{}, fmt.Errorf("decoration start must be an integer")
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return parsedTextDecoration{}, fmt.Errorf("decoration end must be an integer")
	}
	name := strings.ToLower(strings.TrimSpace(parts[2]))
	if start < 0 || end <= start {
		return parsedTextDecoration{}, fmt.Errorf("decoration ranges must use 0 <= start < end")
	}
	if end > textLen {
		return parsedTextDecoration{}, fmt.Errorf("decoration range end exceeds text length in UTF-16 code units")
	}
	decoration := parsedTextDecoration{Start: start, End: end}
	switch {
	case textDecorationStyles[name]:
		decoration.Style = name
	case textDecorationAnimations[name]:
		decoration.Animation = name
	default:
		return parsedTextDecoration{}, fmt.Errorf("unsupported text decoration %q", name)
	}
	return decoration, nil
}

func rangesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart < bEnd && bStart < aEnd
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
