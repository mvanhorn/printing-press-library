// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 youtube adapter via yt-dlp subprocess + VTT parser.

package youtube

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

const adapterName = "youtube"

type Adapter struct {
	Bin       string // path to yt-dlp (override for tests)
	Bilingual bool   // v0.2 mode; v0.1 errors out
}

func New() *Adapter {
	return &Adapter{Bin: "yt-dlp"}
}

func (a *Adapter) Name() string          { return adapterName }
func (a *Adapter) Tier() transcript.Tier { return transcript.TierFree }

var ytRE = regexp.MustCompile(`(?i)^https?://(www\.|m\.)?(youtube\.com/watch|youtu\.be/|youtube\.com/shorts/|youtube\.com/embed/)`)

func (a *Adapter) Match(url string) bool {
	return ytRE.MatchString(url)
}

// ensureBin returns the resolved yt-dlp binary path or a typed error.
func (a *Adapter) ensureBin() (string, error) {
	bin := a.Bin
	if bin == "" {
		bin = "yt-dlp"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", &source.NotImplementedError{
			Adapter: adapterName,
			Detail:  "yt-dlp is not installed (`brew install yt-dlp` or `pip install yt-dlp`)",
		}
	}
	return path, nil
}

func (a *Adapter) Fetch(ctx context.Context, url string) (*transcript.Transcript, error) {
	if a.Bilingual {
		return nil, &source.NotImplementedError{
			Adapter: adapterName,
			Detail:  "the bilingual aligner (--bilingual zh-Hans,en) ships in v0.2",
		}
	}
	bin, err := a.ensureBin()
	if err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", "podcast-goat-yt-")
	if err != nil {
		return nil, fmt.Errorf("yt-dlp tempdir: %w", err)
	}
	defer os.RemoveAll(dir)

	// First pass: metadata via --print
	metaCmd := exec.CommandContext(ctx, bin,
		"--no-warnings",
		"--skip-download",
		"--print", "%(id)s\t%(title)s\t%(uploader)s\t%(duration)s\t%(upload_date)s",
		url,
	)
	metaOut, err := metaCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp metadata for %s: %w", url, err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(metaOut)), "\t", 5)
	for len(parts) < 5 {
		parts = append(parts, "")
	}
	videoID, title, uploader, durationStr, uploadDate := parts[0], parts[1], parts[2], parts[3], parts[4]
	durSec, _ := strconv.Atoi(durationStr)

	// Second pass: subtitles
	subCmd := exec.CommandContext(ctx, bin,
		"--no-warnings",
		"--write-auto-subs",
		"--sub-langs", "en",
		"--sub-format", "vtt",
		"--skip-download",
		"-o", filepath.Join(dir, "%(id)s.%(ext)s"),
		url,
	)
	if out, err := subCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("yt-dlp subs for %s: %w (%s)", url, err, strings.TrimSpace(string(out)))
	}

	// Find the VTT file. Common shapes: <id>.en.vtt, <id>.en-orig.vtt, etc.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read yt-dlp tempdir: %w", err)
	}
	var vttPath string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".vtt") && strings.HasPrefix(n, videoID+".") {
			vttPath = filepath.Join(dir, n)
			break
		}
	}
	if vttPath == "" {
		return nil, &source.NotApplicableError{
			Source: adapterName,
			URL:    url,
			Reason: "yt-dlp returned no English auto-subs (try a different episode or --paid)",
		}
	}

	raw, err := os.ReadFile(vttPath)
	if err != nil {
		return nil, fmt.Errorf("read VTT: %w", err)
	}

	segs := parseVTT(string(raw), uploader)
	if len(segs) == 0 {
		return nil, &source.NotApplicableError{Source: adapterName, URL: url, Reason: "VTT parsed to zero segments"}
	}

	pub := ""
	if len(uploadDate) == 8 {
		pub = uploadDate[0:4] + "-" + uploadDate[4:6] + "-" + uploadDate[6:8]
	}

	return &transcript.Transcript{
		ID:          transcript.IDFor(url),
		Source:      adapterName,
		Show:        slugifyShow(uploader),
		Tier:        transcript.TierFree,
		URL:         url,
		Title:       title,
		Host:        uploader,
		Published:   pub,
		DurationSec: durSec,
		Provider:    adapterName,
		Segments:    segs,
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// VTT parser tuned for YouTube auto-captions: stamps repeat per word so we
// collapse to one segment per cue.
var vttTimeRE = regexp.MustCompile(`^(\d{1,2}):(\d{2}):(\d{2})\.\d{3}\s+-->\s+\d{1,2}:\d{2}:\d{2}\.\d{3}`)
var vttTagRE = regexp.MustCompile(`<[^>]+>`)

func parseVTT(s, speaker string) []transcript.Segment {
	lines := strings.Split(s, "\n")
	if speaker == "" {
		speaker = "Narrator"
	}
	var out []transcript.Segment
	curTS := -1
	var buf strings.Builder
	seen := map[string]bool{}

	flush := func() {
		text := strings.TrimSpace(buf.String())
		buf.Reset()
		if text == "" || curTS < 0 {
			return
		}
		// Dedupe rolling-window YouTube captions (each line repeats the prior).
		if seen[text] {
			return
		}
		seen[text] = true
		out = append(out, transcript.Segment{TsSec: curTS, Speaker: speaker, Text: text})
	}

	for _, ln := range lines {
		ln = strings.TrimRight(ln, "\r")
		if strings.HasPrefix(ln, "WEBVTT") || strings.HasPrefix(ln, "Kind:") || strings.HasPrefix(ln, "Language:") || strings.HasPrefix(ln, "NOTE") {
			continue
		}
		if m := vttTimeRE.FindStringSubmatch(ln); m != nil {
			flush()
			h, _ := strconv.Atoi(m[1])
			mn, _ := strconv.Atoi(m[2])
			sec, _ := strconv.Atoi(m[3])
			curTS = h*3600 + mn*60 + sec
			continue
		}
		if ln == "" {
			continue
		}
		clean := vttTagRE.ReplaceAllString(ln, "")
		clean = strings.TrimSpace(clean)
		if clean == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(clean)
	}
	flush()
	return out
}

func slugifyShow(uploader string) string {
	s := strings.ToLower(strings.TrimSpace(uploader))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

var _ source.Adapter = (*Adapter)(nil)
