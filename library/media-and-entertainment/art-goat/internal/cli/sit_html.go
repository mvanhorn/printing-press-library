// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package cli

import (
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"
)

// writeSitHTML renders a minimal contemplative HTML page for one work and
// writes it to a temp file. The file lives in os.TempDir so the OS can
// clean it up on reboot; the path is stable per work ID so reopening the
// same sit reuses the same URL in the browser tab.
func writeSitHTML(work *store.Work, prompt string) (string, error) {
	if work == nil {
		return "", fmt.Errorf("nil work")
	}
	safeID := strings.NewReplacer(":", "-", "/", "-", "\\", "-").Replace(work.ID)
	path := filepath.Join(os.TempDir(), "art-goat-sit-"+safeID+".html")
	body := renderSitHTMLBody(work, prompt)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", fmt.Errorf("write sit html: %w", err)
	}
	return path, nil
}

// safeHTTPURL accepts only http:// and https:// URLs and returns an
// HTML-escaped form. Any other scheme — javascript:, data:, file:, etc.
// — returns the empty string so a compromised upstream museum-API
// record can't inject an active scheme into the local sit-HTML page.
// html.EscapeString alone is not sufficient: it preserves the colon and
// the dangerous payload after it.
func safeHTTPURL(u string) string {
	trimmed := strings.TrimSpace(u)
	low := strings.ToLower(trimmed)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return ""
	}
	return html.EscapeString(trimmed)
}

func renderSitHTMLBody(work *store.Work, prompt string) string {
	title := html.EscapeString(coalesce(work.Title, "(untitled)"))
	creator := html.EscapeString(work.Creator)
	date := html.EscapeString(work.DateText)
	medium := html.EscapeString(work.Medium)
	region := html.EscapeString(work.CultureRegion)
	source := html.EscapeString(work.Source)
	sourceURL := safeHTTPURL(work.SourceURL)
	imageURL := safeHTTPURL(work.ImageURL)
	description := html.EscapeString(work.Description)
	promptEsc := html.EscapeString(prompt)

	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>art-goat — `)
	b.WriteString(title)
	b.WriteString(`</title><style>
html,body{margin:0;background:#0f0f10;color:#e8e6e1;font-family:Georgia,'Times New Roman',serif}
.wrap{max-width:920px;margin:0 auto;padding:48px 32px}
header{margin-bottom:32px}
h1{font-size:1.6em;font-weight:normal;margin:0 0 8px}
.byline{color:#9c9a93;font-size:0.95em;margin:0 0 4px}
.meta{color:#7e7c75;font-size:0.85em;margin:0}
.image{margin:32px 0;text-align:center}
.image img{max-width:100%;max-height:75vh;border-radius:2px}
.prompt{margin:32px 0;padding:24px;background:#17171a;border-left:3px solid #b2855a;font-style:italic}
.description{line-height:1.6;color:#cfcdc6;font-size:0.98em}
.footer{margin-top:48px;font-size:0.8em;color:#6b6962}
.footer a{color:#9c9a93}
</style></head><body><div class="wrap">`)
	b.WriteString(`<header><h1>`)
	b.WriteString(title)
	b.WriteString(`</h1>`)
	if creator != "" {
		b.WriteString(`<p class="byline">`)
		b.WriteString(creator)
		b.WriteString(`</p>`)
	}
	parts := []string{}
	if date != "" {
		parts = append(parts, date)
	}
	if medium != "" {
		parts = append(parts, medium)
	}
	if region != "" {
		parts = append(parts, region)
	}
	if len(parts) > 0 {
		b.WriteString(`<p class="meta">`)
		b.WriteString(html.EscapeString(strings.Join(parts, " · ")))
		b.WriteString(`</p>`)
	}
	b.WriteString(`</header>`)
	if imageURL != "" {
		b.WriteString(`<div class="image"><img src="`)
		b.WriteString(imageURL)
		b.WriteString(`" alt="`)
		b.WriteString(title)
		b.WriteString(`"></div>`)
	}
	if promptEsc != "" {
		b.WriteString(`<div class="prompt">`)
		b.WriteString(promptEsc)
		b.WriteString(`</div>`)
	}
	if description != "" {
		b.WriteString(`<div class="description">`)
		b.WriteString(description)
		b.WriteString(`</div>`)
	}
	b.WriteString(`<p class="footer">`)
	b.WriteString(source)
	if sourceURL != "" {
		b.WriteString(` · <a href="`)
		b.WriteString(sourceURL)
		b.WriteString(`">source</a>`)
	}
	b.WriteString(`</p></div></body></html>`)
	return b.String()
}

// openInBrowser shells out to the platform-native opener. Returns the
// error from the open command verbatim so the caller can surface it.
// Designed to be called only after IsVerifyEnv has been checked.
func openInBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// emitInlineImage writes an iTerm2 inline-image escape sequence to out
// when stdout is iTerm2 (TERM_PROGRAM=iTerm.app). For Kitty (kitty's icat
// protocol) and Sixel-capable terminals, the easiest path is to delegate
// to an external `imgcat`/`kitty +kitten icat` binary if present; we
// shell to it. When no inline-graphics path is available, emits nothing
// and returns nil. Quiet by design — inline graphics are a "if it works,
// it works" courtesy, never a required surface.
func emitInlineImage(out io.Writer, imageURL string) error {
	if imageURL == "" {
		return nil
	}

	// iTerm2 native inline image protocol (OSC 1337). Requires base64'd
	// image bytes inline, so we download the image first. Cap the
	// download at 8MB to avoid pulling enormous TIFFs.
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
		data, err := downloadCapped(imageURL, 8*1024*1024)
		if err == nil && len(data) > 0 {
			b64 := base64.StdEncoding.EncodeToString(data)
			fmt.Fprintf(out, "\x1b]1337;File=inline=1;width=auto;height=auto;preserveAspectRatio=1:%s\x07\n", b64)
			return nil
		}
	}

	// Kitty: prefer the bundled kitten if the user has it on PATH.
	if kitty, err := exec.LookPath("kitty"); err == nil && os.Getenv("TERM") == "xterm-kitty" {
		cmd := exec.Command(kitty, "+kitten", "icat", "--align", "left", imageURL)
		cmd.Stdout = out
		cmd.Stderr = io.Discard
		_ = cmd.Run() // best effort
		return nil
	}

	// imgcat (iTerm2 helper or third-party): if installed, hand off.
	if imgcat, err := exec.LookPath("imgcat"); err == nil {
		cmd := exec.Command(imgcat, imageURL)
		cmd.Stdout = out
		cmd.Stderr = io.Discard
		_ = cmd.Run() // best effort
		return nil
	}

	// No inline graphics path — silently no-op. The caller already
	// printed the image URL so the user can open it themselves.
	return nil
}

func downloadCapped(rawURL string, max int64) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "art-goat-pp-cli (contemplative art practice)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, max))
}
