package recordings

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Folder is one parsed local Zoom recording folder. The desktop client names
// each session's folder with a "YYYY-MM-DD HH.MM.SS <topic>" pattern (the
// topic may include separators and unicode). One folder typically contains:
//
//   - zoom_0.mp4 / video1*.mp4 (or several segments)
//   - audio_only.m4a or m4a*.m4a
//   - meeting_saved_chat.txt (in-meeting chat transcript)
//   - meeting_saved_closed_caption.txt or *.vtt (transcript / closed captions)
//   - playback.m3u (playlist for segmented recordings)
//   - double_click_to_convert files (recordings that didn't finish converting)
type Folder struct {
	Path           string    `json:"path"`
	Name           string    `json:"name"`
	Topic          string    `json:"topic"`
	Start          time.Time `json:"start"`
	TotalBytes     int64     `json:"total_bytes"`
	HasVideo       bool      `json:"has_video"`
	HasAudio       bool      `json:"has_audio"`
	HasChat        bool      `json:"has_chat"`
	HasTranscript  bool      `json:"has_transcript"`
	HasPartial     bool      `json:"has_partial"`
	TranscriptPath string    `json:"transcript_path,omitempty"`
	VideoPaths     []string  `json:"video_paths,omitempty"`
	AudioPaths     []string  `json:"audio_paths,omitempty"`
	ChatPath       string    `json:"chat_path,omitempty"`
	PartialPaths   []string  `json:"partial_paths,omitempty"`
	MeetingID      string    `json:"meeting_id,omitempty"` // best-effort from folder name
}

// DefaultLocalRoot returns ~/Documents/Zoom, the install-default location for
// local recordings on macOS/Windows/Linux. Callers can override.
func DefaultLocalRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "Zoom")
}

var folderNamePattern = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})\s+(\d{2})\.(\d{2})\.(\d{2})\s+(.+?)(?:\s+(\d{9,12}))?$`)

// Scan walks root and returns one Folder per recording session, sorted newest
// first. Folders that don't match Zoom's naming convention are skipped; the
// caller can lower expectations by adjusting `loose`.
func Scan(root string) ([]Folder, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("scan: reading %s: %w", root, err)
	}
	var folders []Folder
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		f, ok := parseFolder(filepath.Join(root, e.Name()), e.Name())
		if !ok {
			continue
		}
		folders = append(folders, f)
	}
	// Newest first.
	for i, j := 0, len(folders)-1; i < j; i, j = i+1, j-1 {
		// O(n) selection-sort by start desc — folder counts are typically
		// well under a few thousand even for heavy users.
	}
	// Bubble-sort the slice in place. Real users have hundreds, not millions,
	// of recording folders; O(n²) is fine.
	for i := 0; i < len(folders); i++ {
		for j := i + 1; j < len(folders); j++ {
			if folders[j].Start.After(folders[i].Start) {
				folders[i], folders[j] = folders[j], folders[i]
			}
		}
	}
	return folders, nil
}

func parseFolder(path, name string) (Folder, bool) {
	m := folderNamePattern.FindStringSubmatch(name)
	if m == nil {
		return Folder{}, false
	}
	var ts time.Time
	if t, err := time.ParseInLocation("2006-01-02 15.04.05", m[1]+"-"+m[2]+"-"+m[3]+" "+m[4]+"."+m[5]+"."+m[6], time.Local); err == nil {
		ts = t
	}
	f := Folder{
		Path:      path,
		Name:      name,
		Topic:     strings.TrimSpace(m[7]),
		Start:     ts,
		MeetingID: m[8],
	}
	files, err := os.ReadDir(path)
	if err != nil {
		return f, true
	}
	for _, fe := range files {
		if fe.IsDir() {
			continue
		}
		fp := filepath.Join(path, fe.Name())
		info, ierr := fe.Info()
		if ierr == nil {
			f.TotalBytes += info.Size()
		}
		lower := strings.ToLower(fe.Name())
		switch {
		case strings.HasSuffix(lower, ".mp4"):
			f.HasVideo = true
			f.VideoPaths = append(f.VideoPaths, fp)
		case strings.HasSuffix(lower, ".m4a"):
			f.HasAudio = true
			f.AudioPaths = append(f.AudioPaths, fp)
		case strings.HasSuffix(lower, ".vtt"):
			f.HasTranscript = true
			f.TranscriptPath = fp
		case strings.HasSuffix(lower, "_closed_caption.txt"):
			// Closed-caption text file. Treat as a transcript fallback if no VTT.
			if f.TranscriptPath == "" {
				f.HasTranscript = true
				f.TranscriptPath = fp
			}
		case strings.HasSuffix(lower, "_chat.txt"), strings.Contains(lower, "meeting_saved_chat"):
			f.HasChat = true
			f.ChatPath = fp
		case strings.Contains(lower, "double_click_to_convert"):
			f.HasPartial = true
			f.PartialPaths = append(f.PartialPaths, fp)
		}
	}
	return f, true
}
