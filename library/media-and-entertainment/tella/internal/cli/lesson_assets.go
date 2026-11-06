// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func checkLessonTarget(target string, overwrite bool) (string, error) {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(absolute); err == nil {
		if !overwrite {
			return "", fmt.Errorf("delivery target already exists: %s (use --overwrite)", absolute)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("target is not a regular file: %s", absolute)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return absolute, nil
}
func commitLessonAsset(temp, target string, overwrite bool) error {
	if overwrite {
		return os.Rename(temp, target)
	}
	// Link is atomic and refuses a destination created since the preflight.
	return os.Link(temp, target)
}
func writeLessonAsset(target string, data []byte, overwrite bool) (string, error) {
	absolute, err := checkLessonTarget(target, overwrite)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(absolute), ".lesson-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err = commitLessonAsset(f.Name(), absolute, overwrite); err != nil {
		return "", err
	}
	return absolute, nil
}
func validMediaHeader(header []byte, kind string) bool {
	if kind == "webp" {
		return len(header) >= 12 && bytes.Equal(header[:4], []byte("RIFF")) && bytes.Equal(header[8:12], []byte("WEBP"))
	}
	return len(header) >= 12 && bytes.Equal(header[4:8], []byte("ftyp"))
}

// Download on an isolated, credential-free client. Redirects cannot leave the signed origin.
func downloadLessonMedia(ctx context.Context, location, target, kind string, overwrite bool, timeout time.Duration) (string, int64, error) {
	absolute, err := checkLessonTarget(target, overwrite)
	if err != nil {
		return "", 0, err
	}
	u, err := url.Parse(location)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", 0, fmt.Errorf("invalid media URL")
	}
	if u.Scheme != "https" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" {
		return "", 0, fmt.Errorf("media URL must use HTTPS")
	}
	hc := &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != u.Scheme || req.URL.Host != u.Host || req.URL.User != nil {
			return fmt.Errorf("media redirect refused")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, "GET", location, nil)
	if err != nil {
		return "", 0, err
	}
	req, err = client.PrepareRequest(hc, req)
	if err != nil {
		return "", 0, fmt.Errorf("media request refused")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("media download failed (signed URL omitted)")
	}
	defer resp.Body.Close()
	expected := "video/mp4"
	if kind == "webp" {
		expected = "image/webp"
	}
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("media download HTTP %d", resp.StatusCode)
	}
	contentType, _, typeErr := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if typeErr != nil {
		return "", 0, fmt.Errorf("invalid media content type")
	}
	if contentType != expected && contentType != "application/octet-stream" {
		return "", 0, fmt.Errorf("unexpected media content type %q", contentType)
	}
	header := make([]byte, 12)
	if _, err = io.ReadFull(resp.Body, header); err != nil || !validMediaHeader(header, kind) {
		return "", 0, fmt.Errorf("invalid %s signature", kind)
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		return "", 0, err
	}
	f, err := os.CreateTemp(filepath.Dir(absolute), ".lesson-media-*")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(header)
	}
	var count int64
	if err == nil {
		count, err = io.Copy(f, resp.Body)
	}
	closeErr := f.Close()
	if err != nil {
		return "", 0, fmt.Errorf("media transfer incomplete: %w", err)
	}
	if closeErr != nil {
		return "", 0, closeErr
	}
	count += 12
	if resp.ContentLength >= 0 && count != resp.ContentLength {
		return "", 0, fmt.Errorf("media transfer length mismatch")
	}
	if err = commitLessonAsset(f.Name(), absolute, overwrite); err != nil {
		return "", 0, err
	}
	return absolute, count, nil
}
func lessonMediaTimeout(flags *rootFlags) time.Duration {
	if flags.timeoutExplicit {
		return flags.timeout
	}
	return 10 * time.Minute
}
