// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/internal/store"
)

var e164RE = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)

func validateE164(s string) bool {
	return e164RE.MatchString(strings.TrimSpace(s))
}

func loadLocalMessages(limit int) ([]map[string]any, error) {
	db, err := store.Open(defaultDBPath("linq-pp-cli"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	raw, err := db.List("messages", limit)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(raw))
	for _, msg := range raw {
		var obj map[string]any
		if json.Unmarshal(msg, &obj) == nil {
			items = append(items, obj)
		}
	}
	return items, nil
}

func localMessageEvidence(chatID string, messages []map[string]any) map[string]any {
	var inbound, outbound int
	var lastInbound, lastOutbound string
	for _, msg := range messages {
		if !messageMatchesChat(msg, chatID) {
			continue
		}
		direction := strings.ToLower(firstString(msg, "direction", "type", "kind"))
		if strings.Contains(direction, "inbound") || strings.Contains(direction, "incoming") {
			inbound++
			lastInbound = firstString(msg, "created_at", "createdAt", "timestamp", "updated_at")
		}
		if strings.Contains(direction, "outbound") || strings.Contains(direction, "sent") {
			outbound++
			lastOutbound = firstString(msg, "created_at", "createdAt", "timestamp", "updated_at")
		}
	}
	return map[string]any{
		"prior_inbound_messages":  inbound,
		"prior_outbound_messages": outbound,
		"last_inbound_at":         lastInbound,
		"last_outbound_at":        lastOutbound,
	}
}

func messageMatchesChat(msg map[string]any, chatID string) bool {
	if chatID == "" {
		return false
	}
	for _, key := range []string{"chat_id", "chatId", "chat", "conversation_id"} {
		if fmt.Sprint(msg[key]) == chatID {
			return true
		}
	}
	return strings.Contains(fmt.Sprint(msg), chatID)
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := obj[key]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}
