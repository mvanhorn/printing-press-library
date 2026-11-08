// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // register decoder
	_ "image/jpeg" // register decoder
	_ "image/png"  // register decoder
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func strconvItoa(n int) string { return strconv.Itoa(n) }

// ImageInfo is what an uploaded image needs declared.
type ImageInfo struct {
	Ext    string
	Mime   string
	Width  int
	Height int
}

// SniffImage reads a PNG, JPEG, GIF or WebP file's type and pixel size.
func SniffImage(data []byte) (ImageInfo, error) {
	if len(data) >= 30 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		w, h, err := webpSize(data)
		return ImageInfo{Ext: "webp", Mime: "image/webp", Width: w, Height: h}, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageInfo{}, errors.New("not a PNG, JPEG, GIF or WebP image, or the file is damaged")
	}
	ext := format
	if format == "jpeg" {
		ext = "jpg"
	}
	return ImageInfo{Ext: ext, Mime: "image/" + format, Width: cfg.Width, Height: cfg.Height}, nil
}

func webpSize(d []byte) (int, int, error) {
	u24 := func(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
	switch string(d[12:16]) {
	case "VP8X":
		return 1 + u24(d[24:27]), 1 + u24(d[27:30]), nil
	case "VP8L":
		bits := binary.LittleEndian.Uint32(d[21:25])
		return int(bits&0x3FFF) + 1, int((bits>>14)&0x3FFF) + 1, nil
	case "VP8 ":
		return int(binary.LittleEndian.Uint16(d[26:28]) & 0x3FFF), int(binary.LittleEndian.Uint16(d[28:30]) & 0x3FFF), nil
	}
	return 0, 0, errors.New("unrecognised WebP file")
}

// mediaURL turns a presigned storage URL into the app's stable /media link
// for the same object.
func (c *Client) mediaURL(presigned string) (string, error) {
	u, err := url.Parse(presigned)
	if err != nil {
		return "", err
	}
	i := strings.Index(u.EscapedPath(), "/p/images/")
	if i < 0 {
		return "", fmt.Errorf("unexpected upload location %s", u.Host)
	}
	return DefaultBaseURL + "/media" + u.EscapedPath()[i:], nil
}

// AddImage creates an IMAGE element, uploads the bytes and fills in the
// element, in the order the web app does it.
func (c *Client) AddImage(ctx context.Context, w *Writer, boardID string, loc Location, width float64, filename string, data []byte) (string, ImageInfo, error) {
	info, err := SniffImage(data)
	if err != nil {
		return "", info, fmt.Errorf("%s: %w", filepath.Base(filename), err)
	}
	content := map[string]any{"image": nil}
	if width > 0 {
		content["width"] = Snap(width)
	}
	id, err := w.Create(ctx, boardID, TypeImage, loc, content, "DRAG")
	if err != nil {
		return "", info, err
	}
	name := filepath.Base(filename)
	var signed struct {
		PresignedURL string `json:"presignedUrl"`
	}
	body := map[string]any{"filename": name, "contentType": info.Mime, "elementId": id,
		"fileType": "image", "imageType": "ELEMENT", "fileSize": len(data)}
	if err := c.PostJSON(ctx, "POST", "/api/elements/attachments/presign", body, &signed); err != nil {
		return id, info, fmt.Errorf("requesting upload slot: %w", err)
	}
	if signed.PresignedURL == "" {
		return id, info, errors.New("Milanote gave no upload slot")
	}
	if err := c.PutBytes(ctx, signed.PresignedURL, info.Mime, data); err != nil {
		return id, info, err
	}
	original, err := c.mediaURL(signed.PresignedURL)
	if err != nil {
		return id, info, err
	}
	changes := map[string]any{
		"image": map[string]any{"width": info.Width, "height": info.Height, "original": original, "transparent": false},
		"file": map[string]any{"type": "IMAGE", "ext": info.Ext, "mime": info.Mime, "filename": name,
			"size": len(data), "lastModified": time.Now().UnixMilli()},
	}
	if err := w.Update(ctx, boardID, Change{ID: id, Changes: changes}); err != nil {
		return id, info, err
	}
	return id, info, nil
}

// --- board-level REST operations ---

// DuplicateBoard asks the server to copy every card of originalID into the
// freshly created board newID.
func (c *Client) DuplicateBoard(ctx context.Context, originalID, newID string, loc Location) (map[string]any, error) {
	body := map[string]any{"duplications": []any{map[string]any{
		"originalElementId": originalID, "newElementId": newID, "newElementLocation": loc, "elementType": TypeBoard}}}
	var out map[string]any
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		if err = c.PostJSON(ctx, "POST", "/api/elements/duplicate", body, &out); err == nil {
			return out, nil
		}
		if !strings.Contains(err.Error(), "JSON element definition") {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1500 * time.Millisecond): // the new board is not visible to every server yet
		}
	}
	return nil, err
}

// ExportFormats maps the CLI's format names to the Export menu's requests.
var ExportFormats = map[string]map[string]any{
	"markdown": {"format": "linear", "fileType": "text/markdown; charset=UTF-8", "includeChildBoards": false},
	"text":     {"format": "linear", "fileType": "text/plain; charset=UTF-8", "includeChildBoards": false},
	"word":     {"format": "linear", "fileType": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "includeChildBoards": false},
	"pdf":      {"format": "canvas", "fileType": "application/pdf", "quality": "regular"},
	"png":      {"format": "canvas", "fileType": "image/png", "quality": "regular"},
}

// ExportBoard runs Milanote's exporter and returns the file link.
func (c *Client) ExportBoard(ctx context.Context, boardID, format, timezone string) (string, error) {
	spec, ok := ExportFormats[format]
	if !ok {
		return "", fmt.Errorf("unknown format %q", format)
	}
	body := map[string]any{"permissionId": nil, "locale": "en-us", "timezone": timezone, "emojiStyle": "twitter"}
	for k, v := range spec {
		body[k] = v
	}
	var out struct {
		FileURL string `json:"fileUrl"`
	}
	if err := c.PostJSON(ctx, "POST", "/media-service/api/export/"+boardID, body, &out); err != nil {
		return "", err
	}
	if out.FileURL == "" {
		return "", errors.New("Milanote did not produce an export")
	}
	return out.FileURL, nil
}

// Permission bits Milanote stores on a board's acl.
const (
	aclOwner  = 31
	aclEditor = 15
)

// PublicLink returns the board's read-only link, or "".
func (c *Client) PublicLink(ctx context.Context, boardID string) (string, error) {
	rows, err := c.Permissions(ctx, boardID)
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if r["type"] == "PUBLIC" && r["status"] == "ACTIVE" {
			id, _ := r["_id"].(string)
			return DefaultBaseURL + "/" + boardID + "?p=" + id, nil
		}
	}
	return "", nil
}

// Publish turns the read-only link on.
func (c *Client) Publish(ctx context.Context, boardID string, comments bool) error {
	body := map[string]any{"addUserIds": []any{}, "addNewUserEmails": []any{}, "updateUserIds": []any{},
		"removeUserIds": []any{}, "locale": "en-us", "commentsEnabled": comments}
	return c.PostJSON(ctx, "PUT", "/api/elements/"+boardID+"/publish", body, nil)
}

// Unpublish turns the read-only link off.
func (c *Client) Unpublish(ctx context.Context, boardID string) error {
	return c.PostJSON(ctx, "DELETE", "/api/elements/"+boardID+"/publish", nil, nil)
}

// InviteEditor invites an email address to edit the board.
func (c *Client) InviteEditor(ctx context.Context, boardID, email string) error {
	body := map[string]any{"addUserIds": []any{}, "updateUserIds": []any{}, "removeUserIds": []any{},
		"addNewUserEmails": []any{email}, "locale": "en-us", "permissions": aclEditor}
	return c.PostJSON(ctx, "PUT", "/api/elements/"+boardID+"/permissions-invite", body, nil)
}

// RemoveEditor removes a user id from the board.
func (c *Client) RemoveEditor(ctx context.Context, boardID, userID string) error {
	body := map[string]any{"removeUserIds": []any{userID}, "updateUserIds": []any{}}
	return c.PostJSON(ctx, "PUT", "/api/elements/"+boardID+"/permissions", body, nil)
}

// ACLRole names a stored permission value.
func ACLRole(bits any) string {
	switch int(num(bits)) {
	case aclOwner:
		return "owner"
	case aclEditor:
		return "editor"
	}
	return "viewer"
}
