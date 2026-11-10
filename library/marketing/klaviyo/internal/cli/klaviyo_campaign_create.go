// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"strings"
)

// Only the flag-built body is translated; stdin is the caller's exact API contract.
func translateLegacyCampaignCreate(body map[string]any) error {
	data, _ := body["data"].(map[string]any)
	attrs, _ := data["attributes"].(map[string]any)
	if attrs == nil {
		return fmt.Errorf("campaign attributes are required")
	}
	definition, _ := attrs["definition"].(map[string]any)
	if definition == nil {
		definition = map[string]any{}
		attrs["definition"] = definition
	}
	if name, exists := attrs["name"]; exists {
		if existing, exists := definition["name"]; exists && existing != name {
			return fmt.Errorf("--data-attributes-name conflicts with definition.name")
		}
		definition["name"] = name
		delete(attrs, "name")
	}
	if strings.TrimSpace(stringFromMapPath(definition, "name")) == "" && attrs["campaign-messages"] != nil {
		return fmt.Errorf("legacy campaign messages require a campaign name in --data-attributes-name or definition.name")
	}
	if legacyAudience, ok := attrs["audiences"].(map[string]any); ok {
		for key := range legacyAudience {
			if key != "included" && key != "excluded" {
				return fmt.Errorf("cannot translate legacy audience field %q; use an inline audience array", key)
			}
		}
		audience := map[string]any{"temporary_id": "legacy-audience", "name": stringFromMapPath(definition, "name") + " audience"}
		for _, key := range []string{"included", "excluded"} {
			value := legacyAudience[key]
			if value == nil {
				audience[key] = []string{}
				continue
			}
			ids, ok := value.([]any)
			if !ok {
				return fmt.Errorf("legacy audience %s must be an array of IDs", key)
			}
			for _, id := range ids {
				if s, ok := id.(string); !ok || strings.TrimSpace(s) == "" {
					return fmt.Errorf("legacy audience %s must contain non-empty IDs", key)
				}
			}
			audience[key] = ids
		}
		attrs["audiences"] = []any{audience}
	}
	legacy, exists := attrs["campaign-messages"]
	if !exists {
		return nil
	}
	if _, exists := attrs["messages"]; exists {
		return fmt.Errorf("--data-attributes-campaign-messages cannot be combined with --data-attributes-messages")
	}
	envelope, ok := legacy.(map[string]any)
	if !ok {
		return fmt.Errorf("legacy campaign-messages must be an object containing a data array; use --data-attributes-messages for inline messages")
	}
	for key := range envelope {
		if key != "data" {
			return fmt.Errorf("cannot translate legacy campaign-messages field %q", key)
		}
	}
	entries, ok := envelope["data"].([]any)
	if !ok || len(entries) == 0 {
		return fmt.Errorf("legacy campaign-messages.data must be a non-empty array")
	}
	audiences := mapSlice(attrs["audiences"])
	if len(audiences) != 1 || stringFromMapPath(audiences[0], "temporary_id") == "" {
		return fmt.Errorf("legacy campaign messages require exactly one inline audience; use --data-attributes-messages for multiple audiences")
	}
	messages := []any{}
	for i, entry := range entries {
		resource, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("legacy campaign message %d must be an object", i+1)
		}
		for key := range resource {
			if key != "type" && key != "attributes" {
				return fmt.Errorf("cannot translate legacy campaign message field %q; use inline messages", key)
			}
		}
		if typ := stringFromMapPath(resource, "type"); typ != "" && typ != "campaign-message" {
			return fmt.Errorf("legacy campaign message type must be campaign-message")
		}
		msgAttrs, _ := resource["attributes"].(map[string]any)
		for key := range msgAttrs {
			if key != "definition" {
				return fmt.Errorf("legacy message attribute %q has no inline-create equivalent; use --data-attributes-messages", key)
			}
		}
		oldDefinition, _ := msgAttrs["definition"].(map[string]any)
		if oldDefinition == nil {
			return fmt.Errorf("legacy campaign message %d definition is required", i+1)
		}
		for key := range oldDefinition {
			if key != "channel" && key != "content" && key != "label" && key != "name" {
				return fmt.Errorf("legacy message definition.%s has no inline-create equivalent; use --data-attributes-messages", key)
			}
		}
		channel := stringFromMapPath(oldDefinition, "channel")
		if channel == "mobile_push" {
			channel = "push"
		}
		switch channel {
		case "email", "sms", "push", "whatsapp":
		default:
			return fmt.Errorf("legacy campaign message %d has unsupported channel %q", i+1, channel)
		}
		details := map[string]any{"channel": channel}
		if content, exists := oldDefinition["content"]; exists {
			fields, ok := content.(map[string]any)
			if !ok {
				return fmt.Errorf("legacy message content must be an object")
			}
			for key, value := range fields {
				if key == "channel" && value != channel {
					return fmt.Errorf("legacy content.channel conflicts with definition.channel")
				}
				details[key] = value
			}
		}
		name := firstNonEmptyString(stringFromMapPath(oldDefinition, "name"), stringFromMapPath(oldDefinition, "label"), stringFromMapPath(definition, "name"))
		messages = append(messages, map[string]any{"audience_temporary_id": audiences[0]["temporary_id"], "name": name, "variations": []any{map[string]any{"definition": map[string]any{"name": name, "details": details}}}})
	}
	delete(attrs, "campaign-messages")
	attrs["messages"] = messages
	return nil
}
