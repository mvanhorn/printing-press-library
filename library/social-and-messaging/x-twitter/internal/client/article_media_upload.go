package client

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

func (c *Client) UploadArticleImage(ctx context.Context, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	mediaType := http.DetectContentType(data)
	if mediaType != "image/png" && mediaType != "image/jpeg" && mediaType != "image/gif" && mediaType != "image/webp" {
		return "", fmt.Errorf("unsupported image type %q", mediaType)
	}
	mediaID, err := c.initArticleMediaUpload(ctx, len(data), mediaType)
	if err != nil {
		return "", err
	}
	if err := c.appendArticleMediaUpload(ctx, mediaID, filepath.Base(path), data); err != nil {
		return "", err
	}
	if err := c.finalizeArticleMediaUpload(ctx, mediaID, data); err != nil {
		return "", err
	}
	return mediaID, nil
}

func (c *Client) initArticleMediaUpload(ctx context.Context, totalBytes int, mediaType string) (string, error) {
	u, err := mediaUploadURL(map[string]string{
		"command":        "INIT",
		"total_bytes":    strconv.Itoa(totalBytes),
		"media_type":     mediaType,
		"media_category": "tweet_image",
	})
	if err != nil {
		return "", err
	}
	data, _, err := c.postRaw(ctx, u, nil, "")
	if err != nil {
		return "", err
	}
	var response struct {
		MediaID       any    `json:"media_id"`
		MediaIDString string `json:"media_id_string"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("parse INIT response: %w", err)
	}
	if response.MediaIDString != "" {
		return response.MediaIDString, nil
	}
	switch mediaID := response.MediaID.(type) {
	case string:
		return mediaID, nil
	case float64:
		return strconv.FormatInt(int64(mediaID), 10), nil
	default:
		return "", fmt.Errorf("INIT response did not include media_id_string")
	}
}

func (c *Client) appendArticleMediaUpload(ctx context.Context, mediaID, filename string, data []byte) error {
	sum := md5.Sum(data)
	u, err := mediaUploadURL(map[string]string{
		"command":          "APPENDMULTI",
		"media_id":         mediaID,
		"segment_indexes":  "0",
		"max_segment_size": strconv.Itoa(len(data)),
		"media_md5":        hex.EncodeToString(sum[:]),
	})
	if err != nil {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", filename)
	if err != nil {
		return fmt.Errorf("create multipart media field: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("write multipart media field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart body: %w", err)
	}
	_, _, err = c.postRaw(ctx, u, &body, writer.FormDataContentType())
	return err
}

func (c *Client) finalizeArticleMediaUpload(ctx context.Context, mediaID string, data []byte) error {
	sum := md5.Sum(data)
	u, err := mediaUploadURL(map[string]string{
		"command":      "FINALIZE",
		"media_id":     mediaID,
		"original_md5": hex.EncodeToString(sum[:]),
	})
	if err != nil {
		return err
	}
	_, _, err = c.postRaw(ctx, u, nil, "")
	return err
}

func mediaUploadURL(params map[string]string) (string, error) {
	u, err := url.Parse(MediaUploadURL())
	if err != nil {
		return "", err
	}
	q := u.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) postRaw(ctx context.Context, targetURL string, body io.Reader, contentType string) (json.RawMessage, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, body)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	cookies, err := LoadCookieAuth()
	if err != nil {
		return nil, 0, fmt.Errorf("cookie auth required for upload.x.com: %w", err)
	}
	cookies.apply(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("POST %s: %w", targetURL, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("reading response: %w", err)
	}
	respBody = sanitizeJSONResponse(respBody)
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, &APIError{Method: http.MethodPost, Path: targetURL, StatusCode: resp.StatusCode, Body: truncateBody(respBody)}
	}
	if resp.StatusCode < 400 {
		c.invalidateCache()
	}
	return json.RawMessage(respBody), resp.StatusCode, nil
}
