// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package pexels

import "sort"

// SizeCandidate is one selectable rendition of a photo or video.
type SizeCandidate struct {
	Label string
	URL   string
	W     int
	H     int
}

// photoSrcOrder lists the Pexels src keys whose pixel dimensions are derivable.
// The portrait/landscape/tiny crops are excluded from resolution picking
// because they change the aspect ratio (they are crops, not scaled copies).
var photoSrcOrder = []string{"small", "medium", "large", "large2x", "original"}

// photoCandidates builds the resolution-ordered candidate list from a src map.
func photoCandidates(src map[string]string, photoW, photoH int) []SizeCandidate {
	aspect := 0.0
	if photoH > 0 {
		aspect = float64(photoW) / float64(photoH)
	}
	out := make([]SizeCandidate, 0, len(photoSrcOrder))
	for _, label := range photoSrcOrder {
		u, ok := src[label]
		if !ok || u == "" {
			continue
		}
		var w, h int
		switch label {
		case "original":
			w, h = photoW, photoH
		case "large2x":
			w, h = scaledToFit(photoW, photoH, 1880, 1300)
		case "large":
			w, h = scaledToFit(photoW, photoH, 940, 650)
		case "medium":
			h = 350
			w = int(aspect * 350)
		case "small":
			h = 130
			w = int(aspect * 130)
		}
		out = append(out, SizeCandidate{Label: label, URL: u, W: w, H: h})
	}
	return out
}

func scaledToFit(photoW, photoH, maxW, maxH int) (int, int) {
	if photoW <= 0 || photoH <= 0 {
		return maxW, maxH
	}
	if photoW*maxH > photoH*maxW {
		return maxW, maxW * photoH / photoW
	}
	return maxH * photoW / photoH, maxH
}

// PickPhotoSize chooses the smallest photo rendition whose width and height
// both meet the (non-zero) targets. A target of 0 means "no constraint" on
// that axis. When nothing qualifies it falls back to the largest candidate
// (original). It returns empty strings when src has no usable entries.
func PickPhotoSize(src map[string]string, photoW, photoH, targetW, targetH int) (label, url string, w, h int) {
	cands := photoCandidates(src, photoW, photoH)
	if len(cands) == 0 {
		return "", "", 0, 0
	}
	sort.SliceStable(cands, func(i, j int) bool {
		return cands[i].W*cands[i].H < cands[j].W*cands[j].H
	})
	for _, c := range cands {
		if (targetW == 0 || c.W >= targetW) && (targetH == 0 || c.H >= targetH) {
			return c.Label, c.URL, c.W, c.H
		}
	}
	last := cands[len(cands)-1]
	return last.Label, last.URL, last.W, last.H
}

// VideoFile is the subset of a Pexels video_files entry needed for picking.
type VideoFile struct {
	Quality  string
	FileType string
	Width    int
	Height   int
	Link     string
}

// PickVideoFile chooses the smallest video file whose width and height meet
// the (non-zero) targets. Files with zero dimensions (HLS manifests) are not
// eligible for sizing but are kept as a last-resort fallback. When nothing
// meets the target it returns the largest sized file, or the first file if
// none have dimensions.
func PickVideoFile(files []VideoFile, targetW, targetH int) (VideoFile, bool) {
	if len(files) == 0 {
		return VideoFile{}, false
	}
	sized := make([]VideoFile, 0, len(files))
	for _, f := range files {
		if f.Width > 0 && f.Height > 0 {
			sized = append(sized, f)
		}
	}
	if len(sized) == 0 {
		// Only manifests/unknown sizes — return the first as a fallback.
		return files[0], true
	}
	sort.SliceStable(sized, func(i, j int) bool {
		return sized[i].Width*sized[i].Height < sized[j].Width*sized[j].Height
	})
	for _, f := range sized {
		if (targetW == 0 || f.Width >= targetW) && (targetH == 0 || f.Height >= targetH) {
			return f, true
		}
	}
	return sized[len(sized)-1], true
}

// PickVideoFileByQuality returns the first file matching the requested quality
// label (e.g. "hd", "sd", "uhd"); if none match it returns the largest sized
// file. The bool is false only when files is empty.
func PickVideoFileByQuality(files []VideoFile, quality string) (VideoFile, bool) {
	if len(files) == 0 {
		return VideoFile{}, false
	}
	if quality != "" {
		var best VideoFile
		found := false
		for _, f := range files {
			if f.Quality == quality && f.Width > 0 {
				if !found || f.Width*f.Height > best.Width*best.Height {
					best = f
					found = true
				}
			}
		}
		if found {
			return best, true
		}
	}
	// Fall back to the largest sized file.
	return largestVideoFile(files)
}

func largestVideoFile(files []VideoFile) (VideoFile, bool) {
	if len(files) == 0 {
		return VideoFile{}, false
	}
	var best VideoFile
	found := false
	for _, f := range files {
		if f.Width > 0 && f.Height > 0 {
			if !found || f.Width*f.Height > best.Width*best.Height {
				best = f
				found = true
			}
		}
	}
	if !found {
		return files[0], true
	}
	return best, true
}
