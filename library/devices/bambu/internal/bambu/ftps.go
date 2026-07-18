package bambu

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jlaffaye/ftp"
)

const (
	MaxArchiveBytes          = 128 << 20
	MaxArchiveEntries        = 10000
	MaxCentralDirectoryBytes = 8 << 20
	MaxConfigBytes           = 2 << 20
	MaxThumbnailBytes        = 12 << 20
)

type File struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Size    uint64    `json:"size"`
	ModTime time.Time `json:"modified_at,omitempty"`
	Type    string    `json:"type"`
}

type Metadata struct {
	SourcePath     string   `json:"source_path,omitempty"`
	CandidatePaths []string `json:"candidate_paths"`
	ProjectName    string   `json:"project_name,omitempty"`
	ProfileName    string   `json:"profile_name,omitempty"`
	PlateNumber    *int     `json:"plate_number,omitempty"`
	WeightGrams    *float64 `json:"weight_grams,omitempty"`
	Thumbnail      []byte   `json:"-"`
	ThumbnailName  string   `json:"thumbnail_name,omitempty"`
	Objects        []Object `json:"objects,omitempty"`
}

type Object struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Skipped bool   `json:"skipped"`
}

type FTPS struct {
	conn *ftp.ServerConn
	ctx  context.Context
	done chan struct{}
	once sync.Once
}

func DialFTPS(ctx context.Context, host, serial, accessCode string) (*FTPS, error) {
	tlsConfig, err := TLSConfig(serial)
	if err != nil {
		return nil, err
	}
	conn, err := ftp.Dial(host+":990", ftp.DialWithContext(ctx), ftp.DialWithTLS(tlsConfig), ftp.DialWithTimeout(12*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect implicit FTPS: %w", err)
	}
	client := &FTPS{conn: conn, ctx: ctx, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = client.Close()
		case <-client.done:
		}
	}()
	if err := conn.Login("bblp", accessCode); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("authenticate implicit FTPS: %w", err)
	}
	if err := conn.Type(ftp.TransferTypeBinary); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("set FTPS binary mode: %w", err)
	}
	return client, nil
}

func (f *FTPS) Close() error {
	if f == nil || f.conn == nil {
		return nil
	}
	var closeErr error
	f.once.Do(func() {
		close(f.done)
		closeErr = f.conn.Quit()
	})
	return closeErr
}

func (f *FTPS) List(remotePath string, limit int) ([]File, error) {
	remotePath = safeRemotePath(remotePath)
	entries, err := f.conn.List(remotePath)
	if err != nil {
		return nil, fmt.Errorf("list FTPS path %q: %w", remotePath, err)
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	files := make([]File, 0, min(limit, len(entries)))
	for _, entry := range entries {
		if len(files) >= limit {
			break
		}
		kind := "file"
		if entry.Type == ftp.EntryTypeFolder {
			kind = "directory"
		}
		files = append(files, File{Path: strings.TrimRight(remotePath, "/") + "/" + entry.Name, Name: entry.Name, Size: entry.Size, ModTime: entry.Time, Type: kind})
	}
	return files, nil
}

func (f *FTPS) Download(remotePath, outputPath string, maxBytes int64) (int64, error) {
	remotePath = safeRemotePath(remotePath)
	if maxBytes <= 0 || maxBytes > MaxArchiveBytes {
		maxBytes = MaxArchiveBytes
	}
	if outputPath == "" || outputPath == "-" {
		return 0, fmt.Errorf("an explicit file output path is required")
	}
	cleanOutput := filepath.Clean(outputPath)
	if info, err := os.Lstat(cleanOutput); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("refusing to overwrite symlink %q", cleanOutput)
	}
	response, err := f.conn.Retr(remotePath)
	if err != nil {
		return 0, fmt.Errorf("retrieve FTPS path %q: %w", remotePath, err)
	}
	if deadline, ok := f.ctx.Deadline(); ok {
		_ = response.SetDeadline(deadline)
	}
	defer response.Close()
	if err := os.MkdirAll(filepath.Dir(cleanOutput), 0o700); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(cleanOutput), ".bambu-download-*")
	if err != nil {
		return 0, fmt.Errorf("create temporary output: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	written, copyErr := io.Copy(tmp, io.LimitReader(response, maxBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return written, fmt.Errorf("write download: %w", copyErr)
	}
	if closeErr != nil {
		return written, fmt.Errorf("close download: %w", closeErr)
	}
	if written > maxBytes {
		return written, fmt.Errorf("download exceeded %d-byte limit", maxBytes)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return written, fmt.Errorf("secure download permissions: %w", err)
	}
	if err := os.Rename(tmpName, cleanOutput); err != nil {
		return written, fmt.Errorf("install download: %w", err)
	}
	return written, nil
}

func (f *FTPS) JobMetadata(snapshot Snapshot) (Metadata, error) {
	candidates := ArchiveCandidates(snapshot)
	fallbackCandidates := map[string]bool{}
	if f != nil && f.conn != nil {
		files, err := f.List("/", 1000)
		if err == nil {
			for _, candidate := range LegacyArchiveCandidates(snapshot, files) {
				if !containsString(candidates, candidate) {
					candidates = append(candidates, candidate)
					fallbackCandidates[candidate] = true
				}
			}
		}
	}
	metadata := Metadata{CandidatePaths: candidates, PlateNumber: snapshot.PlateNumber, Objects: []Object{}}
	for _, candidate := range candidates {
		size, err := f.conn.FileSize(candidate)
		if err != nil || size <= 0 || size > MaxArchiveBytes {
			continue
		}
		response, err := f.conn.Retr(candidate)
		if err != nil {
			continue
		}
		if deadline, ok := f.ctx.Deadline(); ok {
			_ = response.SetDeadline(deadline)
		}
		payload, readErr := io.ReadAll(io.LimitReader(response, MaxArchiveBytes+1))
		closeErr := response.Close()
		if readErr != nil || closeErr != nil || len(payload) > MaxArchiveBytes {
			continue
		}
		parsed, err := Extract3MF(payload, snapshot.PlateNumber)
		if err != nil {
			continue
		}
		if fallbackCandidates[candidate] && !MetadataMatchesSnapshot(parsed, snapshot) {
			continue
		}
		parsed.SourcePath = candidate
		parsed.CandidatePaths = candidates
		return parsed, nil
	}
	if rawPath := strings.TrimSpace(stringValue(snapshot.Raw["gcode_file"])); strings.HasPrefix(strings.ToLower(rawPath), "/data/metadata/") && strings.HasSuffix(strings.ToLower(rawPath), ".gcode") {
		return metadata, fmt.Errorf("current print exposes printer-resident G-code (%s), not a 3MF available over FTPS; display-started built-in prints may not provide weight or preview metadata", safeBasename(rawPath))
	}
	return metadata, fmt.Errorf("current 3MF metadata unavailable after checking %d candidate paths", len(candidates))
}

func LegacyArchiveCandidates(snapshot Snapshot, files []File) []string {
	observedAt := snapshot.ObservedAt
	if observedAt.IsZero() {
		return nil
	}
	maxAge := 48 * time.Hour
	if snapshot.Percent != nil && *snapshot.Percent > 0 && *snapshot.Percent < 100 && snapshot.RemainingMinutes != nil {
		elapsed := time.Duration(float64(*snapshot.RemainingMinutes)*float64(*snapshot.Percent)/float64(100-*snapshot.Percent)) * time.Minute
		if elapsed+6*time.Hour > maxAge {
			maxAge = elapsed + 6*time.Hour
		}
	}
	oldest := observedAt.Add(-maxAge)
	eligible := make([]File, 0, len(files))
	for _, file := range files {
		if file.Type != "file" || file.Size == 0 || file.Size > MaxArchiveBytes || !strings.HasSuffix(strings.ToLower(file.Name), ".3mf") {
			continue
		}
		if !file.ModTime.IsZero() && file.ModTime.Before(oldest) {
			continue
		}
		eligible = append(eligible, file)
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].ModTime.IsZero() != eligible[j].ModTime.IsZero() {
			return eligible[i].ModTime.IsZero()
		}
		if eligible[i].ModTime.Equal(eligible[j].ModTime) {
			return eligible[i].Path < eligible[j].Path
		}
		return eligible[i].ModTime.After(eligible[j].ModTime)
	})
	paths := make([]string, 0, len(eligible))
	for _, file := range eligible {
		paths = append(paths, file.Path)
	}
	return paths
}

func MetadataMatchesSnapshot(metadata Metadata, snapshot Snapshot) bool {
	want := []string{canonicalJobLabel(snapshot.JobName), canonicalJobLabel(snapshot.SubtaskName)}
	got := []string{canonicalJobLabel(metadata.ProfileName), canonicalJobLabel(metadata.ProjectName)}
	for _, candidate := range got {
		if candidate == "" {
			continue
		}
		for _, expected := range want {
			if expected != "" && candidate == expected {
				return true
			}
		}
	}
	return false
}

func canonicalJobLabel(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func ArchiveCandidates(snapshot Snapshot) []string {
	names := make([]string, 0, 3)
	addName := func(value string) {
		name := safeBasename(value)
		if name == "" || name == "." {
			return
		}
		if !strings.HasSuffix(strings.ToLower(name), ".3mf") {
			name += ".3mf"
		}
		for _, existing := range names {
			if existing == name {
				return
			}
		}
		names = append(names, name)
	}
	addName(snapshot.SubtaskName)
	if snapshot.SubtaskName != "" && !strings.HasSuffix(strings.ToLower(snapshot.SubtaskName), ".3mf") {
		addName(snapshot.SubtaskName + ".gcode")
	}
	addName(snapshot.GCodeFile)
	paths := make([]string, 0, len(names)*2)
	for _, name := range names {
		paths = append(paths, "/"+name, "/cache/"+name)
	}
	return paths
}

type sliceInfo struct {
	Plates []plate `xml:"plate"`
}

type plate struct {
	Metadata []metadataItem `xml:"metadata"`
	Objects  []plateObject  `xml:"object"`
}

type metadataItem struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

type plateObject struct {
	ID      string `xml:"identify_id,attr"`
	Name    string `xml:"name,attr"`
	Skipped bool   `xml:"skipped,attr"`
}

type modelPackage struct {
	Metadata []modelMetadata `xml:"metadata"`
}

type modelMetadata struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

func Extract3MF(payload []byte, wantedPlate *int) (Metadata, error) {
	if err := preflightZIPDirectory(payload); err != nil {
		return Metadata{}, err
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return Metadata{}, fmt.Errorf("open 3MF ZIP: %w", err)
	}
	if len(reader.File) > MaxArchiveEntries {
		return Metadata{}, fmt.Errorf("3MF ZIP has %d entries; maximum is %d", len(reader.File), MaxArchiveEntries)
	}
	var config, modelConfig []byte
	for _, file := range reader.File {
		name := strings.ToLower(file.Name)
		if (name != "metadata/slice_info.config" && name != "3d/3dmodel.model") || file.UncompressedSize64 > MaxConfigBytes {
			continue
		}
		opened, err := file.Open()
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(opened, MaxConfigBytes+1))
		_ = opened.Close()
		if readErr != nil || len(data) > MaxConfigBytes {
			continue
		}
		if name == "metadata/slice_info.config" {
			config = data
		} else {
			modelConfig = data
		}
	}
	if len(config) == 0 {
		return Metadata{}, fmt.Errorf("3MF has no bounded slice_info.config")
	}
	var info sliceInfo
	if err := xml.Unmarshal(config, &info); err != nil {
		return Metadata{}, fmt.Errorf("parse 3MF slice metadata: %w", err)
	}
	if len(info.Plates) == 0 {
		return Metadata{}, fmt.Errorf("3MF has no plate metadata")
	}
	selected := info.Plates[0]
	selectedIndex := metadataInt(selected.Metadata, "index")
	if wantedPlate != nil {
		found := false
		for _, candidate := range info.Plates {
			if value := metadataInt(candidate.Metadata, "index"); value != nil && *value == *wantedPlate {
				selected = candidate
				selectedIndex = value
				found = true
				break
			}
		}
		if !found {
			return Metadata{}, fmt.Errorf("3MF does not contain requested plate %d", *wantedPlate)
		}
	}
	result := Metadata{PlateNumber: selectedIndex, Objects: make([]Object, 0, len(selected.Objects))}
	if len(modelConfig) > 0 {
		var model modelPackage
		if err := xml.Unmarshal(modelConfig, &model); err == nil {
			for _, item := range model.Metadata {
				switch item.Name {
				case "Title":
					result.ProjectName = strings.TrimSpace(item.Value)
				case "ProfileTitle":
					result.ProfileName = strings.TrimSpace(item.Value)
				}
			}
		}
	}
	if weight := metadataFloat(selected.Metadata, "weight"); weight != nil && math.IsNaN(*weight) == false && math.IsInf(*weight, 0) == false && *weight > 0 {
		result.WeightGrams = weight
	}
	for _, object := range selected.Objects {
		result.Objects = append(result.Objects, Object{ID: object.ID, Name: object.Name, Skipped: object.Skipped})
	}
	if selectedIndex != nil {
		name := fmt.Sprintf("metadata/plate_%d.png", *selectedIndex)
		for _, file := range reader.File {
			if strings.ToLower(file.Name) != name || file.UncompressedSize64 > MaxThumbnailBytes {
				continue
			}
			opened, err := file.Open()
			if err != nil {
				break
			}
			imageData, readErr := io.ReadAll(io.LimitReader(opened, MaxThumbnailBytes+1))
			_ = opened.Close()
			if readErr == nil && len(imageData) <= MaxThumbnailBytes && validImage(imageData) {
				result.Thumbnail = imageData
				result.ThumbnailName = fmt.Sprintf("bambu_plate_%d.png", *selectedIndex)
			}
			break
		}
	}
	return result, nil
}

func preflightZIPDirectory(payload []byte) error {
	const eocdSize = 22
	start := len(payload) - eocdSize
	minimum := max(0, len(payload)-(65535+eocdSize))
	for index := start; index >= minimum; index-- {
		if index+eocdSize > len(payload) || binary.LittleEndian.Uint32(payload[index:index+4]) != 0x06054b50 {
			continue
		}
		entries := int(binary.LittleEndian.Uint16(payload[index+10 : index+12]))
		directoryBytes := int(binary.LittleEndian.Uint32(payload[index+12 : index+16]))
		directoryOffset := int(binary.LittleEndian.Uint32(payload[index+16 : index+20]))
		commentBytes := int(binary.LittleEndian.Uint16(payload[index+20 : index+22]))
		if index+eocdSize+commentBytes != len(payload) {
			continue
		}
		if entries == 0xffff {
			return fmt.Errorf("ZIP64 3MF archives are not supported")
		}
		if entries > MaxArchiveEntries {
			return fmt.Errorf("3MF ZIP has %d entries; maximum is %d", entries, MaxArchiveEntries)
		}
		if directoryBytes > MaxCentralDirectoryBytes {
			return fmt.Errorf("3MF ZIP central directory exceeds %d bytes", MaxCentralDirectoryBytes)
		}
		directoryEnd := directoryOffset + directoryBytes
		if directoryOffset < 0 || directoryEnd != index || directoryEnd > len(payload) {
			return fmt.Errorf("3MF ZIP central directory bounds are invalid")
		}
		count := 0
		for cursor := directoryOffset; cursor < directoryEnd; {
			if cursor+46 > directoryEnd || binary.LittleEndian.Uint32(payload[cursor:cursor+4]) != 0x02014b50 {
				return fmt.Errorf("3MF ZIP central directory entry is malformed")
			}
			nameBytes := int(binary.LittleEndian.Uint16(payload[cursor+28 : cursor+30]))
			extraBytes := int(binary.LittleEndian.Uint16(payload[cursor+30 : cursor+32]))
			entryCommentBytes := int(binary.LittleEndian.Uint16(payload[cursor+32 : cursor+34]))
			cursor += 46 + nameBytes + extraBytes + entryCommentBytes
			if cursor > directoryEnd {
				return fmt.Errorf("3MF ZIP central directory entry exceeds its bounds")
			}
			count++
			if count > MaxArchiveEntries {
				return fmt.Errorf("3MF ZIP has more than %d entries", MaxArchiveEntries)
			}
		}
		if count != entries {
			return fmt.Errorf("3MF ZIP entry count mismatch")
		}
		return nil
	}
	return fmt.Errorf("3MF ZIP end-of-central-directory record was not found")
}

func metadataInt(items []metadataItem, key string) *int {
	for _, item := range items {
		if item.Key == key {
			value, err := strconv.Atoi(item.Value)
			if err == nil {
				return &value
			}
		}
	}
	return nil
}

func metadataFloat(items []metadataItem, key string) *float64 {
	for _, item := range items {
		if item.Key == key {
			value, err := strconv.ParseFloat(item.Value, 64)
			if err == nil {
				return &value
			}
		}
	}
	return nil
}

func validImage(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(payload))
	return err == nil && config.Width > 0 && config.Height > 0 && config.Width <= 8192 && config.Height <= 8192
}

func safeRemotePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "/"
	}
	clean := filepath.ToSlash(filepath.Clean("/" + strings.TrimPrefix(value, "/")))
	if strings.Contains(clean, "..") {
		return "/"
	}
	return clean
}
