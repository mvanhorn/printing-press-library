// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import (
	"path"
	"sort"
	"strings"
)

// FolderStat aggregates the files directly under one first-level folder of
// the stats target ("(root)" for files at the target's own level).
type FolderStat struct {
	Folder string `json:"folder"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
}

// ExtStat aggregates files sharing one lowercase extension ("(none)" for
// extensionless files).
type ExtStat struct {
	Ext   string `json:"ext"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

// FileStat is one file's path (relative to the stats target) and size.
type FileStat struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Stats is ComputeStats' aggregate view of a tree listing.
type Stats struct {
	ByFolder    []FolderStat `json:"by_folder"`
	ByExtension []ExtStat    `json:"by_extension"`
	Largest     []FileStat   `json:"largest"`
	TotalFiles  int          `json:"total_files"`
	TotalBytes  int64        `json:"total_bytes"`
}

// ComputeStats aggregates a WalkTree file listing into by-folder (direct
// children of basePath), by-extension, and largest-files views. Folder and
// extension buckets are sorted by bytes descending with name as the
// deterministic tie-break; Largest holds the top `top` files by size
// (top <= 0 defaults to 10).
func ComputeStats(files []TreeFile, basePath string, top int) Stats {
	folderAgg := map[string]*FolderStat{}
	extAgg := map[string]*ExtStat{}
	var totalBytes int64
	largestAll := make([]FileStat, 0, len(files))

	for _, f := range files {
		rel := f.RelTo(basePath)
		totalBytes += f.Size
		largestAll = append(largestAll, FileStat{Path: rel, Size: f.Size})

		folder := "(root)"
		if idx := strings.IndexByte(rel, '/'); idx >= 0 {
			folder = rel[:idx]
		}
		fs := folderAgg[folder]
		if fs == nil {
			fs = &FolderStat{Folder: folder}
			folderAgg[folder] = fs
		}
		fs.Files++
		fs.Bytes += f.Size

		ext := strings.ToLower(path.Ext(rel))
		if ext == "" {
			ext = "(none)"
		}
		es := extAgg[ext]
		if es == nil {
			es = &ExtStat{Ext: ext}
			extAgg[ext] = es
		}
		es.Files++
		es.Bytes += f.Size
	}

	byFolder := make([]FolderStat, 0, len(folderAgg))
	for _, fs := range folderAgg {
		byFolder = append(byFolder, *fs)
	}
	sort.Slice(byFolder, func(i, j int) bool {
		if byFolder[i].Bytes != byFolder[j].Bytes {
			return byFolder[i].Bytes > byFolder[j].Bytes
		}
		return byFolder[i].Folder < byFolder[j].Folder
	})

	byExtension := make([]ExtStat, 0, len(extAgg))
	for _, es := range extAgg {
		byExtension = append(byExtension, *es)
	}
	sort.Slice(byExtension, func(i, j int) bool {
		if byExtension[i].Bytes != byExtension[j].Bytes {
			return byExtension[i].Bytes > byExtension[j].Bytes
		}
		return byExtension[i].Ext < byExtension[j].Ext
	})

	sort.Slice(largestAll, func(i, j int) bool {
		if largestAll[i].Size != largestAll[j].Size {
			return largestAll[i].Size > largestAll[j].Size
		}
		return largestAll[i].Path < largestAll[j].Path
	})
	if top <= 0 {
		top = 10
	}
	if top > len(largestAll) {
		top = len(largestAll)
	}

	return Stats{
		ByFolder:    byFolder,
		ByExtension: byExtension,
		Largest:     largestAll[:top],
		TotalFiles:  len(files),
		TotalBytes:  totalBytes,
	}
}
