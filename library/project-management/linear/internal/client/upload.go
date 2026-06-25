package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxLinearUploadBytes = 100 * 1024 * 1024

type UploadedFile struct {
	Filename    string `json:"filename"`
	AssetURL    string `json:"assetUrl"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}

// UploadFileFromPath runs Linear's two-step file upload flow and returns the
// uploaded asset URL that can be embedded in Markdown bodies.
func (c *Client) UploadFileFromPath(path string, makePublic bool) (UploadedFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return UploadedFile{}, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return UploadedFile{}, fmt.Errorf("%s is a directory", path)
	}
	if info.Size() > maxLinearUploadBytes {
		return UploadedFile{}, fmt.Errorf("%s is too large: %d bytes (max %d)", path, info.Size(), maxLinearUploadBytes)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return UploadedFile{}, fmt.Errorf("read %s: %w", path, err)
	}
	filename := filepath.Base(path)
	contentType := detectUploadContentType(path, content)
	if c.DryRun {
		return UploadedFile{
			Filename:    filename,
			AssetURL:    "dry-run://" + strings.ReplaceAll(filename, " ", "%20"),
			ContentType: contentType,
			Size:        len(content),
		}, nil
	}

	upload, err := c.createUpload(len(content), filename, contentType, makePublic)
	if err != nil {
		return UploadedFile{}, err
	}
	if err := c.putUpload(upload.UploadURL, content, contentType, upload.Headers); err != nil {
		return UploadedFile{}, err
	}
	return UploadedFile{
		Filename:    filename,
		AssetURL:    upload.AssetURL,
		ContentType: contentType,
		Size:        len(content),
	}, nil
}

type uploadTarget struct {
	UploadURL string
	AssetURL  string
	Headers   map[string]string
}

func (c *Client) createUpload(size int, filename, contentType string, makePublic bool) (uploadTarget, error) {
	const mutation = `mutation($size: Int!, $filename: String!, $contentType: String!, $makePublic: Boolean) {
		fileUpload(size: $size, filename: $filename, contentType: $contentType, makePublic: $makePublic) {
			success
			uploadFile {
				uploadUrl
				assetUrl
				headers { key value }
			}
		}
	}`
	var resp struct {
		FileUpload struct {
			Success    bool `json:"success"`
			UploadFile struct {
				UploadURL string `json:"uploadUrl"`
				AssetURL  string `json:"assetUrl"`
				Headers   []struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"headers"`
			} `json:"uploadFile"`
		} `json:"fileUpload"`
	}
	if err := c.QueryInto(mutation, map[string]any{
		"size":        size,
		"filename":    filename,
		"contentType": contentType,
		"makePublic":  makePublic,
	}, &resp); err != nil {
		return uploadTarget{}, fmt.Errorf("fileUpload failed: %w", err)
	}
	if !resp.FileUpload.Success || resp.FileUpload.UploadFile.UploadURL == "" || resp.FileUpload.UploadFile.AssetURL == "" {
		return uploadTarget{}, fmt.Errorf("fileUpload did not return an upload target")
	}
	headers := make(map[string]string, len(resp.FileUpload.UploadFile.Headers))
	for _, h := range resp.FileUpload.UploadFile.Headers {
		headers[h.Key] = h.Value
	}
	return uploadTarget{
		UploadURL: resp.FileUpload.UploadFile.UploadURL,
		AssetURL:  resp.FileUpload.UploadFile.AssetURL,
		Headers:   headers,
	}, nil
}

func (c *Client) putUpload(uploadURL string, content []byte, contentType string, headers map[string]string) error {
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("creating upload request: %w", err)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	ctx := req.Context()
	if httpClient.Timeout == 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("upload failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func detectUploadContentType(path string, content []byte) string {
	if extType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); extType != "" {
		return extType
	}
	if len(content) > 0 {
		return http.DetectContentType(content)
	}
	return "application/octet-stream"
}
