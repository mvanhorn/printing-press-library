// Copyright 2026 Peter Yang and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/substack-notes/internal/client"
)

const maxNoteImageBytes int64 = 10 * 1024 * 1024

var supportedNoteImageMIMEs = map[string]bool{
	"image/gif":  true,
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

func attachNoteImages(ctx context.Context, c *client.Client, body map[string]any, imagePaths []string) (map[string]any, error) {
	if len(imagePaths) == 0 {
		return body, nil
	}
	if c != nil && c.DryRun {
		return nil, usageErr(fmt.Errorf("--dry-run with --image cannot create Substack attachment IDs; omit --image to preview the text request, or run without --dry-run to upload media"))
	}
	attachmentIDs := make([]string, 0, len(imagePaths))
	for _, imagePath := range imagePaths {
		dataURL, err := imageDataURL(imagePath)
		if err != nil {
			return nil, err
		}
		uploadedURL, err := uploadNoteImage(ctx, c, dataURL)
		if err != nil {
			return nil, err
		}
		attachmentID, err := createNoteImageAttachment(ctx, c, uploadedURL)
		if err != nil {
			return nil, err
		}
		attachmentIDs = append(attachmentIDs, attachmentID)
	}
	body["attachmentIds"] = attachmentIDs
	return body, nil
}

func imageDataURL(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", usageErr(fmt.Errorf("--image path is empty"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("reading --image %q: %w", path, err)
	}
	if info.IsDir() {
		return "", usageErr(fmt.Errorf("--image %q is a directory", path))
	}
	if info.Size() == 0 {
		return "", usageErr(fmt.Errorf("--image %q is empty", path))
	}
	if info.Size() > maxNoteImageBytes {
		return "", usageErr(fmt.Errorf("--image %q is %.1f MB; maximum is %.1f MB", path, float64(info.Size())/(1024*1024), float64(maxNoteImageBytes)/(1024*1024)))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading --image %q: %w", path, err)
	}
	mimeType := detectImageMIME(path, data)
	if !supportedNoteImageMIMEs[mimeType] {
		return "", usageErr(fmt.Errorf("--image %q must be PNG, JPEG, GIF, or WebP", path))
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func detectImageMIME(path string, data []byte) string {
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	if limit > 0 {
		detected := http.DetectContentType(data[:limit])
		if supportedNoteImageMIMEs[detected] {
			return detected
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".gif":
		return "image/gif"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func uploadNoteImage(ctx context.Context, c *client.Client, dataURL string) (string, error) {
	data, _, err := c.PostWithParams(ctx, "/api/v1/image", map[string]string{}, map[string]any{"image": dataURL})
	if err != nil {
		return "", classifyAPIError(err, nil)
	}
	uploadedURL, err := imageUploadURL(data)
	if err != nil {
		return "", err
	}
	return uploadedURL, nil
}

func imageUploadURL(data json.RawMessage) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", fmt.Errorf("parsing image upload response: %w", err)
	}
	for _, key := range []string{"url", "image_url", "imageUrl"} {
		if value, ok := obj[key].(string); ok {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			parsed, err := url.Parse(value)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				return "", fmt.Errorf("image upload response %q is not a valid URL", key)
			}
			return value, nil
		}
	}
	return "", fmt.Errorf("image upload response did not include a URL")
}

func createNoteImageAttachment(ctx context.Context, c *client.Client, imageURL string) (string, error) {
	data, _, err := c.PostWithParams(ctx, "/api/v1/comment/attachment", map[string]string{}, map[string]any{
		"type": "image",
		"url":  imageURL,
	})
	if err != nil {
		return "", classifyAPIError(err, nil)
	}
	attachmentID, err := imageAttachmentID(data)
	if err != nil {
		return "", err
	}
	return attachmentID, nil
}

func imageAttachmentID(data json.RawMessage) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", fmt.Errorf("parsing image attachment response: %w", err)
	}
	value, ok := obj["id"]
	if !ok {
		return "", fmt.Errorf("image attachment response did not include an id")
	}
	id := strings.TrimSpace(fmt.Sprintf("%v", value))
	if id == "" || id == "<nil>" {
		return "", fmt.Errorf("image attachment response included an empty id")
	}
	if len(id) > 128 {
		return "", fmt.Errorf("image attachment id is too long")
	}
	return id, nil
}
