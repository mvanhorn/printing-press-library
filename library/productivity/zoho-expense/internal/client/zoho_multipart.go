package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

// PostMultipart posts a multipart/form-data body to path with the given
// text fields plus a single file part (fileField=filename:fileBytes). Used
// by `receipt upload` and `invoice ingest` for Zoho's POST /expenses
// receipt-attached path, which the generated JSON-only Post can't reach.
//
// Returns the raw response body, HTTP status, and an error. The error
// shape mirrors do(): an *APIError on 4xx/5xx so callers can classify
// status codes with errors.As.
func (c *Client) PostMultipart(ctx context.Context, path string, fields map[string]string, fileField, filename string, fileBytes []byte) (json.RawMessage, int, error) {
	targetURL := c.BaseURL + path

	if c.DryRun {
		fmt.Fprintf(os.Stderr, "POST %s (multipart)\n", targetURL)
		for k, v := range fields {
			fmt.Fprintf(os.Stderr, "  field %s=%s\n", k, v)
		}
		fmt.Fprintf(os.Stderr, "  file %s=%s (%d bytes)\n", fileField, filename, len(fileBytes))
		fmt.Fprintf(os.Stderr, "\n(dry run - no request sent)\n")
		return json.RawMessage(`{"dry_run": true}`), 0, nil
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return nil, 0, fmt.Errorf("writing form field %s: %w", k, err)
		}
	}
	if fileField != "" {
		part, err := writer.CreateFormFile(fileField, filename)
		if err != nil {
			return nil, 0, fmt.Errorf("creating form file: %w", err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			return nil, 0, fmt.Errorf("writing form file: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, 0, fmt.Errorf("closing multipart writer: %w", err)
	}

	// Wait on the limiter exactly the way do() does.
	c.limiter.Wait()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, &buf)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	authHeader, err := c.authHeader(ctx)
	if err != nil {
		return nil, 0, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if c.Config != nil {
		for k, v := range c.Config.Headers {
			req.Header.Set(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "zoho-expense-pp-cli/0.1.0")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, 0, ctxErr
		}
		return nil, 0, fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, &APIError{
			Method:     http.MethodPost,
			Path:       path,
			StatusCode: resp.StatusCode,
			Body:       truncateBody(body),
		}
	}

	c.limiter.OnSuccess()
	c.invalidateCache()
	return json.RawMessage(sanitizeJSONResponse(body)), resp.StatusCode, nil
}
