// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. Artistly's image-to-image edit tools live in a SEPARATE app,
// editor.artistly.ai, with a clean JSON API authenticated by a JWT in a custom
// `token:` header (NOT the app.artistly.ai session cookie). Contracts verified
// from a user HAR + live probing:
//
//   Auth (auto-mint): the editor JWT is embedded, freshly minted per request, in
//   the app page /ai-image-modifier as props.redirectUrlWithToken
//   (https://editor.artistly.ai/landing?token=<JWT>). The CLI reads it with the
//   existing app cookie session — no manual paste, no separate mint endpoint.
//
//   user_id: POST editor /api/users/get-user-data {token} -> data.user.id (UUID).
//   upscale:  POST editor /api/ai/upscaler  {baseImageBase64, face_quality, tool_used, user_id}
//   bg-remove: POST editor /api/ai/replace-bg {image, mode:"bg_remove", user_id}
//   Both are async: response carries insertedRow.id; poll
//   GET /api/ai/ai-text-to-image/image/{id} until status != "processing".
//
// pp:data-source live

package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/client"
)

const editorBaseURL = "https://editor.artistly.ai"

// mintEditorToken reads the freshly-minted editor JWT from the app page's
// redirectUrlWithToken prop, using the authenticated app cookie session.
func mintEditorToken(ctx context.Context, c *client.Client) (string, error) {
	raw, err := c.GetWithHeadersNoCache(ctx, "/ai-image-modifier", map[string]string{}, map[string]string{"Accept": "text/html"})
	if err != nil {
		return "", err
	}
	props, err := extractDataPageProps(string(raw))
	if err != nil {
		return "", fmt.Errorf("could not read editor token (are you authenticated? run: artistly-pp-cli auth login --chrome): %w", err)
	}
	redirectURL := stringProp(props, "redirectUrlWithToken")
	if redirectURL == "" {
		return "", authErr(fmt.Errorf("editor token not found on the app page; run: artistly-pp-cli auth login --chrome"))
	}
	u, err := url.Parse(redirectURL)
	if err != nil {
		return "", fmt.Errorf("parsing editor redirect URL: %w", err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		return "", authErr(fmt.Errorf("editor token missing from redirect URL"))
	}
	return tok, nil
}

// editorRequest performs a JSON request against editor.artistly.ai with the
// editor JWT in the `token` header. The editor uses no cookies, so this does not
// share the app cookie jar.
func editorRequest(ctx context.Context, token, method, path string, body any) (json.RawMessage, int, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, editorBaseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("token", token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := &http.Client{Timeout: 120 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return data, resp.StatusCode, authErr(fmt.Errorf("editor rejected the token (re-run: artistly-pp-cli auth login --chrome)"))
	}
	if resp.StatusCode >= 400 {
		return data, resp.StatusCode, fmt.Errorf("editor %s %s returned HTTP %d", method, path, resp.StatusCode)
	}
	return data, resp.StatusCode, nil
}

// editorUserID resolves the editor user UUID (required in edit request bodies).
func editorUserID(ctx context.Context, token string) (string, error) {
	data, _, err := editorRequest(ctx, token, http.MethodPost, "/api/users/get-user-data", map[string]any{"token": token})
	if err != nil {
		return "", err
	}
	// get-user-data returns {"user":{"id":"<uuid>",...},"expires":...} with no
	// "data" wrapper (unlike the image-poll endpoint).
	var resp struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if json.Unmarshal(data, &resp) == nil && resp.User.ID != "" {
		return resp.User.ID, nil
	}
	return "", fmt.Errorf("could not resolve editor user id")
}

// imageToDataURI turns an edit input (design id/uuid, image URL, or local file
// path) into a base64 data URI for the editor's image-input fields.
func imageToDataURI(ctx context.Context, c *client.Client, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	var raw []byte
	switch {
	case strings.HasPrefix(arg, "http://"), strings.HasPrefix(arg, "https://"):
		b, err := fetchBytes(ctx, arg)
		if err != nil {
			return "", err
		}
		raw = b
	case fileExists(arg):
		// #nosec G304 -- arg is the user-supplied local image path passed
		// explicitly to the edit command; reading the caller's own file from
		// their own machine is the intended behavior, not an untrusted-input
		// boundary.
		b, err := os.ReadFile(arg)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", arg, err)
		}
		raw = b
	default:
		// Treat as a design id/uuid; resolve to its first rendered CDN image.
		d, err := findDesign(ctx, c, arg)
		if err != nil {
			return "", err
		}
		urls := designImageURLs(*d)
		if len(urls) == 0 {
			return "", apiErr(fmt.Errorf("design %q has no rendered image to edit (status: %s)", arg, d.Status))
		}
		b, err := fetchBytes(ctx, urls[0])
		if err != nil {
			return "", err
		}
		raw = b
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("input image is empty")
	}
	mime := http.DetectContentType(raw)
	if !strings.HasPrefix(mime, "image/") {
		mime = "image/png"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func fetchBytes(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// editorInserted is the shared response shape for upscaler / replace-bg.
type editorInserted struct {
	InsertedRow struct {
		ID string `json:"id"`
	} `json:"insertedRow"`
}

// submitEditorOp posts an edit operation and returns the new design id to poll.
func submitEditorOp(ctx context.Context, token, path string, body map[string]any) (string, error) {
	data, _, err := editorRequest(ctx, token, http.MethodPost, path, body)
	if err != nil {
		return "", err
	}
	var ins editorInserted
	if json.Unmarshal(data, &ins) != nil || ins.InsertedRow.ID == "" {
		return "", fmt.Errorf("edit submitted but no result id was returned")
	}
	return ins.InsertedRow.ID, nil
}

// pollEditorImage polls the editor result endpoint until the image renders,
// returning the real (non-placeholder) CDN image URLs.
func pollEditorImage(ctx context.Context, token, id string, timeout time.Duration, progress io.Writer) ([]string, error) {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		data, _, err := editorRequest(ctx, token, http.MethodGet, "/api/ai/ai-text-to-image/image/"+id, nil)
		if err != nil {
			return nil, err
		}
		// While processing: {"data":[],"status":"processing"}. When done:
		// {"data":{...,"images":[<real url>]}}.
		var done struct {
			Data struct {
				Images []string `json:"images"`
			} `json:"data"`
		}
		if json.Unmarshal(data, &done) == nil {
			var real []string
			for _, u := range done.Data.Images {
				if u != "" && !strings.Contains(u, "placehold.co") {
					real = append(real, u)
				}
			}
			if len(real) > 0 {
				return real, nil
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out after %s waiting for the edit to render", timeout)
		}
		if progress != nil {
			fmt.Fprintln(progress, "  ...rendering")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// downloadURLs saves a list of image URLs into dir with a name prefix, returning
// the written paths.
func downloadURLs(ctx context.Context, urls []string, dir, prefix string) ([]string, error) {
	var written []string
	for i, u := range urls {
		name := prefix
		if len(urls) > 1 {
			name = fmt.Sprintf("%s-%d", prefix, i+1)
		}
		dest := dir + "/" + sanitizeFilename(name) + imageExt(u)
		if err := downloadImage(ctx, u, dest); err != nil {
			return written, err
		}
		written = append(written, dest)
	}
	return written, nil
}
