// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
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
	Bin string // path to yt-dlp (override for tests)
	// Lang is a single subtitle language code passed to yt-dlp --sub-langs.
	// Empty means "en". The live catalog of available tracks is per-video,
	// so non-English shows need this settable rather than the historical
	// hardcoded "en". One code per fetch in v0.1: multiple codes would make
	// yt-dlp write several VTTs and the file picker below would grab an
	// arbitrary one. Note: the rolling-window de-dup (collapseRollingWindow)
	// operates on space-separated words, so it applies to space-tokenized
	// languages; captions in languages written without spaces (zh, ja, th)
	// pass through without rolling-window collapsing.
	Lang string
}

func New() *Adapter {
	return &Adapter{Bin: "yt-dlp"}
}

func (a *Adapter) Name() string          { return adapterName }
func (a *Adapter) Tier() transcript.Tier { return transcript.TierFree }

var ytRE = regexp.MustCompile(`(?i)^https?://(www\.|m\.)?(youtube\.com/watch|youtu\.be/|youtube\.com/shorts/|youtube\.com/embed/)`)

// ytSearchRE accepts yt-dlp's native search pseudo-URLs (ytsearch1:<query>),
// used by the Spotify cookie-missing fallback hint to resolve the same
// episode on YouTube without the user hand-hunting for the video URL.
var ytSearchRE = regexp.MustCompile(`(?i)^ytsearch\d*:`)

func (a *Adapter) Match(url string) bool {
	return ytRE.MatchString(url) || ytSearchRE.MatchString(url)
}

func (a *Adapter) Fetch(ctx context.Context, url string) (*transcript.Transcript, error) {
	bin, err := EnsureYtDlp(ctx, a.Bin, os.Stderr)
	if err != nil {
		return nil, &source.NotImplementedError{
			Adapter: adapterName,
			Detail:  fmt.Sprintf("yt-dlp unavailable: %v", err),
		}
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

	// Canonicalize ytsearch pseudo-URLs to the resolved watch URL so the
	// cached transcript is keyed by the real video, not the search query.
	if ytSearchRE.MatchString(url) && videoID != "" {
		url = "https://www.youtube.com/watch?v=" + videoID
	}

	langs := strings.TrimSpace(a.Lang)
	if langs == "" {
		langs = "en"
	}

	// Second pass: subtitles
	subCmd := exec.CommandContext(ctx, bin,
		"--no-warnings",
		"--write-auto-subs",
		"--sub-langs", langs,
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
			Reason: fmt.Sprintf("yt-dlp returned no %q auto-subs (try --lang <code> for non-English shows, or --paid)", langs),
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
	var draft []transcript.Segment
	curTS := -1
	var buf strings.Builder

	flush := func() {
		text := strings.TrimSpace(buf.String())
		buf.Reset()
		if text == "" || curTS < 0 {
			return
		}
		draft = append(draft, transcript.Segment{TsSec: curTS, Speaker: speaker, Text: text})
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

	return collapseRollingWindow(draft)
}

// collapseRollingWindow merges YouTube auto-sub rolling-window cues. Each new
// cue typically contains the previous cue's text plus a few more words (the
// caption stream builds up sentence-by-sentence). Without collapsing, the
// canonical markdown output has the same words repeated 3-5 times per second.
//
// Rules per (prev, cur) pair:
//   - cur.Text is identical to prev.Text         → drop cur
//   - prev.Text is a prefix of cur.Text          → replace prev with cur (cur is the longer form)
//   - cur.Text is a prefix of prev.Text          → drop cur (prev already has more)
//   - a suffix of prev.Text (≥3 words) is a prefix of cur.Text
//     → emit cur with the overlapping words removed (sliding window)
//   - otherwise                                  → emit cur
//
// The sliding-window rule is what actually fires on live auto-subs: YouTube
// emits cues whose first half repeats the second half of the previous cue
// ("...more likely to experience anxiety and" / "more likely to experience
// anxiety and depression than..."), which the prefix rules alone never merge —
// the result was every word appearing twice in the canonical markdown. The
// 3-word minimum keeps natural repetition ("of the", "and the") intact.
func collapseRollingWindow(segs []transcript.Segment) []transcript.Segment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]transcript.Segment, 0, len(segs))
	out = append(out, segs[0])
	// prevOrig is the ORIGINAL text of the previous cue, before any trim.
	// Every comparison runs against it, never against the trimmed remainder
	// stored in out: when a cue advances fewer words than it repeats, the
	// remainder is shorter than the window and comparing against it lets the
	// repeated run back in ("a b c d e" / "c d e f" / "d e f g" must yield
	// "a b c d e f g", not re-emit "d e f").
	prevOrig := segs[0].Text
	for i := 1; i < len(segs); i++ {
		cur := segs[i]
		prev := &out[len(out)-1]
		if cur.Text == prevOrig {
			continue
		}
		if strings.HasPrefix(cur.Text, prevOrig) {
			// cur extends the previous window. When the emitted segment is
			// the untouched window, replace it wholesale (preserves exact
			// spacing/punctuation); when it is a trimmed remainder, append
			// only the new tail.
			if prev.Text == prevOrig {
				prev.Text = cur.Text
			} else if tail := strings.TrimSpace(cur.Text[len(prevOrig):]); tail != "" {
				if prev.Text != "" {
					prev.Text += " "
				}
				prev.Text += tail
			}
			prev.TsSec = cur.TsSec
			prevOrig = cur.Text
			continue
		}
		if strings.HasPrefix(prevOrig, cur.Text) {
			// The previous window already contains cur — drop, keep the
			// larger window as context.
			continue
		}
		if rem, trimmed := trimWordOverlap(prevOrig, cur.Text); trimmed {
			prevOrig = cur.Text
			if rem == "" {
				// cur was entirely overlap — nothing new.
				continue
			}
			cur.Text = rem
			out = append(out, cur)
			continue
		}
		prevOrig = cur.Text
		out = append(out, cur)
	}
	return out
}

// minOverlapWords is the shortest prev-suffix / cur-prefix word run treated as
// a rolling-window artifact rather than natural repetition.
const minOverlapWords = 3

// trimWordOverlap finds the longest run of whole words that is both a suffix
// of prev and a prefix of cur. When the run is at least minOverlapWords long,
// it returns cur with that run removed and trimmed=true. Matching is
// word-exact (case-sensitive): auto-sub rolling windows repeat the words
// verbatim, so anything looser risks eating real speech.
//
// Words are strings.Fields tokens, so this only fires for space-tokenized
// languages. Captions written without spaces (zh, ja, th) come through as one
// token per cue and are deliberately left untouched — a wrong merge is worse
// than a duplicate, and the limitation is documented on --lang.
func trimWordOverlap(prev, cur string) (rem string, trimmed bool) {
	pw := strings.Fields(prev)
	cw := strings.Fields(cur)
	max := len(pw)
	if len(cw) < max {
		max = len(cw)
	}
	for k := max; k >= minOverlapWords; k-- {
		match := true
		for j := 0; j < k; j++ {
			if pw[len(pw)-k+j] != cw[j] {
				match = false
				break
			}
		}
		if match {
			return strings.Join(cw[k:], " "), true
		}
	}
	return cur, false
}

func slugifyShow(uploader string) string {
	s := strings.ToLower(strings.TrimSpace(uploader))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

var _ source.Adapter = (*Adapter)(nil)
