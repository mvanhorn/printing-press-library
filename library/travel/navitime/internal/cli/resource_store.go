package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/store"
	"strings"
)

// Retain the framework's single-object identity normalization independently
// of unsupported bulk-sync commands.

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

type discriminatorDispatch struct {
	Field  string
	Values map[string]string
}

var discriminatorDispatchers = map[string]discriminatorDispatch{}

func resolveDiscriminatedResource(resource string, obj map[string]any) string {
	dispatcher, ok := discriminatorDispatchers[resource]
	if !ok || dispatcher.Field == "" {
		return resource
	}
	value := store.LookupFieldValue(obj, dispatcher.Field)
	if value == nil {
		return resource
	}
	if target, ok := dispatcher.Values[fmt.Sprintf("%v", value)]; ok && target != "" {
		return target
	}
	return resource
}

func upsertSingleObject(db *store.Store, resource string, data json.RawMessage) error {
	if !json.Valid(bytes.TrimSpace(data)) || isJSONNull(data) {
		return fmt.Errorf("%s response is not JSON; refusing to store a non-JSON body", resource)
	}
	obj, err := store.DecodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("%s response is not a JSON object: %w", resource, err)
	}

	resource = resolveDiscriminatedResource(resource, obj)

	var forceTypedID bool
	var id string
	if htmlPageModeResources[resource] {
		id, forceTypedID = resourceIDFieldOverrideID(resource, obj)
		if id == "" {
			id = store.ExtractResourceID(resource, obj)
			forceTypedID = id != ""
		}
		if id == "" {
			id = synthesizeHTMLPageModeID(resource, obj)
			forceTypedID = id != ""
		}
	} else {
		id = extractID(resource, obj)
		if id == "" {
			id = resource
		}
	}

	switch resource {
	case "raw":
		typedData, err := typedSingleObjectData(data, obj, id, forceTypedID)
		if err != nil {
			return err
		}
		return db.UpsertRaw(typedData)
	default:
		return db.Upsert(resource, id, data)
	}
}

var resourceIDFieldOverrides = map[string]string{}

var htmlPageModeResources = map[string]bool{
	"raw":        true,
	"raw-result": true,
}

func extractID(resource string, obj map[string]any) string {
	if s, ok := resourceIDFieldOverrideID(resource, obj); ok {
		return s
	}
	return store.ExtractResourceID(resource, obj)
}

func resourceIDFieldOverrideID(resource string, obj map[string]any) (string, bool) {
	if override, ok := resourceIDFieldOverrides[resource]; ok && override != "" {
		if v := store.LookupFieldValue(obj, override); v != nil {
			if s := store.CanonicalResourceID(v); s != "" {
				return s, true
			}
		}
	}
	return "", false
}

func synthesizeHTMLPageModeID(resource string, obj map[string]any) string {
	if !htmlPageModeResources[resource] {
		return ""
	}
	for _, key := range []string{"canonical_url", "url", "image_url"} {
		if v := store.LookupFieldValue(obj, key); v != nil {
			if s := store.CanonicalResourceID(v); s != "" {
				return s
			}
		}
	}
	return resource
}

func typedSingleObjectData(data json.RawMessage, obj map[string]any, id string, forceTypedID bool) (json.RawMessage, error) {
	if !forceTypedID {
		return data, nil
	}
	return objectWithSyntheticID(obj, id)
}

func objectWithSyntheticID(obj map[string]any, id string) (json.RawMessage, error) {
	withID := make(map[string]any, len(obj)+1)
	for key, value := range obj {
		withID[key] = value
	}
	withID["id"] = id
	return json.Marshal(withID)
}
