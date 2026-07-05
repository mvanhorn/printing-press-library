// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UploadImage runs Airbnb's three-step message-photo upload flow and returns the
// media item ID to attach to a message:
//  1. GetSignedUrlsMutation  — request a signed upload URL for the file
//  2. HTTP PUT               — upload the raw bytes to that URL
//  3. CreateMediaItemsMutation — register the uploaded object as a media item
//
// The exact request/response shapes for steps 1 and 3 are validated against the
// live API; on shape drift the returned error names the failing step so
// `ops refresh` / a re-capture can fix it.
func (c *Client) UploadImage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading image: %w", err)
	}
	name := filepath.Base(path)
	contentType := contentTypeForImage(name)

	// Step 1: signed URL.
	signVars := map[string]any{
		"request": map[string]any{
			"contentType": contentType,
			"filename":    name,
			"uploadType":  "MESSAGE_IMAGE",
		},
	}
	signResp, err := c.Mutation("GetSignedUrlsMutation", signVars)
	if err != nil {
		return "", fmt.Errorf("get signed url: %w", err)
	}
	signedURL, objectName := parseSignedURL(signResp)
	if signedURL == "" {
		return "", fmt.Errorf("get signed url: no upload URL in response: %s", truncate(string(signResp), 300))
	}

	// Step 2: PUT the bytes (through the client's rate limiter).
	status, err := c.putBytes(signedURL, contentType, data)
	if err != nil {
		return "", fmt.Errorf("uploading image bytes: %w", err)
	}
	if status >= 400 {
		return "", fmt.Errorf("uploading image bytes: HTTP %d", status)
	}

	// Step 3: register the media item.
	createVars := map[string]any{
		"request": map[string]any{
			"mediaItems": []map[string]any{
				{"objectName": objectName, "contentType": contentType},
			},
		},
	}
	createResp, err := c.Mutation("CreateMediaItemsMutation", createVars)
	if err != nil {
		return "", fmt.Errorf("register media item: %w", err)
	}
	mediaID := parseMediaItemID(createResp)
	if mediaID == "" {
		return "", fmt.Errorf("register media item: no media id in response: %s", truncate(string(createResp), 300))
	}
	return mediaID, nil
}

func parseSignedURL(data json.RawMessage) (signedURL, objectName string) {
	// Response shape is discovered/validated live; probe common field names.
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return "", ""
	}
	signedURL = findStringDeep(m, "signedUrl", "signed_url", "uploadUrl", "url")
	objectName = findStringDeep(m, "objectName", "object_name", "key", "objectKey")
	return signedURL, objectName
}

func parseMediaItemID(data json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	return findStringDeep(m, "mediaItemId", "id", "mediaId")
}

// findStringDeep does a bounded depth-first search for the first non-empty
// string value under any of the given keys.
func findStringDeep(v any, keys ...string) string {
	switch t := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if s, ok := t[k].(string); ok && s != "" {
				return s
			}
		}
		for _, child := range t {
			if s := findStringDeep(child, keys...); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := findStringDeep(child, keys...); s != "" {
				return s
			}
		}
	}
	return ""
}

func contentTypeForImage(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	default:
		return "image/jpeg"
	}
}
