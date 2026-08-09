// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Immich personal-library workflows. Kept separate from generated files so regeneration preserves it.

package cli

import (
	"archive/zip"
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/immich/internal/client"
	"github.com/spf13/cobra"
)

// registerNovelCommand is deliberately used instead of changing root.go. The
// generator's extension hook is stable across reprints.
func init() { registerNovelCommand(addPersonalLibraryRituals) }

func addPersonalLibraryRituals(root *cobra.Command, flags *rootFlags) {
	// Name the extension root consistently with generated wiring so the
	// Printing Press tree walker records these as top-level commands.
	rootCmd := root
	// Keep these registrations separate: the Printing Press command-tree
	// walker records each direct root registration and must see `library` as
	// distinct from the generated `libraries scan library` endpoint surface.
	rootCmd.AddCommand(newAlbumRitualsCmd(flags))
	rootCmd.AddCommand(newLibraryRitualsCmd(flags))
}

type importAsset struct {
	Path     string
	Name     string
	Created  time.Time
	Modified time.Time
	Metadata map[string]any
}

type importOptions struct {
	recursive     bool
	ignores       []string
	includeHidden bool
	maxFiles      int
	concurrency   int
	dryRun        bool
	album         string
}

func bindImportFlags(cmd *cobra.Command, opt *importOptions, includeWatch bool) {
	cmd.Flags().BoolVar(&opt.recursive, "recursive", true, "Walk nested folders")
	cmd.Flags().StringSliceVar(&opt.ignores, "ignore", nil, "Glob to exclude; may be repeated")
	cmd.Flags().BoolVar(&opt.includeHidden, "include-hidden", false, "Include dotfiles and dot-directories")
	cmd.Flags().IntVar(&opt.maxFiles, "max-files", 0, "Maximum files to select (0 is unlimited)")
	cmd.Flags().IntVar(&opt.concurrency, "concurrency", 4, "Concurrent upload workers (1-32)")
	cmd.Flags().BoolVar(&opt.dryRun, "dry-run", false, "Select and report files without contacting Immich")
	cmd.Flags().StringVar(&opt.album, "album-name", "", "Create an album with uploaded assets")
}

func newImportFolderCmd(flags *rootFlags) *cobra.Command {
	var opt importOptions
	cmd := &cobra.Command{Use: "folder <path>", Short: "Recursively import media from a local folder through duplicate-check and multipart upload", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		assets, skipped, err := collectFolder(args[0], opt)
		if err != nil {
			return err
		}
		return runImport(cmd, flags, assets, skipped, opt, "folder")
	}}
	bindImportFlags(cmd, &opt, false)
	return cmd
}

func newImportArchiveCmd(flags *rootFlags) *cobra.Command {
	var opt importOptions
	cmd := &cobra.Command{Use: "archive <zip>", Short: "Extract a ZIP safely and import its media through the local source adapter", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		tmp, err := unzipToTemp(args[0])
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		assets, skipped, err := collectFolder(tmp, opt)
		if err != nil {
			return err
		}
		return runImport(cmd, flags, assets, skipped, opt, "archive")
	}}
	bindImportFlags(cmd, &opt, false)
	return cmd
}

func newImportTakeoutCmd(flags *rootFlags) *cobra.Command {
	var opt importOptions
	cmd := &cobra.Command{Use: "takeout <dir-or-zip>", Short: "Import a Google Photos Takeout, reading JSON sidecars for explicit source metadata", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root := args[0]
		cleanup := func() {}
		if strings.EqualFold(filepath.Ext(root), ".zip") {
			var err error
			root, err = unzipToTemp(root)
			if err != nil {
				return err
			}
			cleanup = func() { _ = os.RemoveAll(root) }
		}
		defer cleanup()
		assets, skipped, err := collectFolder(root, opt)
		if err != nil {
			return err
		}
		applyTakeoutSidecars(root, assets)
		return runImport(cmd, flags, assets, skipped, opt, "takeout")
	}}
	bindImportFlags(cmd, &opt, false)
	return cmd
}

func newImportICloudCmd(flags *rootFlags) *cobra.Command {
	var opt importOptions
	cmd := &cobra.Command{Use: "icloud <export-dir>", Short: "Import an iCloud export and map supplied CSV date and album metadata", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		assets, skipped, err := collectFolder(args[0], opt)
		if err != nil {
			return err
		}
		applyICloudCSV(args[0], assets)
		return runImport(cmd, flags, assets, skipped, opt, "icloud")
	}}
	bindImportFlags(cmd, &opt, false)
	return cmd
}

func newImportWatchCmd(flags *rootFlags) *cobra.Command {
	var opt importOptions
	var duration, poll, stable time.Duration
	cmd := &cobra.Command{Use: "watch <path>", Short: "Watch a local folder for stabilized files and upload them until the bounded duration expires", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if duration <= 0 || poll <= 0 || stable < 0 {
			return usageErr(fmt.Errorf("--for and --poll must be positive; --stable-for cannot be negative"))
		}
		seen := map[string]fileStamp{}
		deadline := time.NewTimer(duration)
		defer deadline.Stop()
		tick := time.NewTicker(poll)
		defer tick.Stop()
		total := importSummary{Source: "watch"}
		for {
			assets, skipped, err := collectFolder(args[0], opt)
			if err != nil {
				return err
			}
			total.Skipped += skipped
			ready := readyWatchAssets(assets, seen, time.Now(), stable)
			if len(ready) > 0 {
				sum, err := uploadAssets(cmd.Context(), flags, ready, opt)
				if err != nil {
					return err
				}
				total.add(sum)
				if shouldMarkWatchAssetsUploaded(sum) {
					markWatchAssetsUploaded(ready, seen)
				}
			}
			select {
			case <-cmd.Context().Done():
				total.Cancelled = true
				return writeImportSummary(cmd, flags, total)
			case <-deadline.C:
				return writeImportSummary(cmd, flags, total)
			case <-tick.C:
			}
		}
	}}
	bindImportFlags(cmd, &opt, true)
	cmd.Flags().DurationVar(&duration, "for", 30*time.Minute, "Maximum watch duration")
	cmd.Flags().DurationVar(&poll, "poll", 10*time.Second, "Folder polling interval")
	cmd.Flags().DurationVar(&stable, "stable-for", 5*time.Second, "Required unchanged size/mtime duration")
	return cmd
}

func newImportImmichCmd(flags *rootFlags) *cobra.Command {
	var opt importOptions
	var sourceURL, sourceEnv string
	cmd := &cobra.Command{Use: "immich", Short: "Migrate originals from another Immich server through its authenticated REST API", RunE: func(cmd *cobra.Command, _ []string) error {
		if sourceURL == "" || sourceEnv == "" {
			return usageErr(fmt.Errorf("--source-url and --source-api-key-env are required"))
		}
		key := os.Getenv(sourceEnv)
		if key == "" {
			return fmt.Errorf("source API key environment variable %q is empty", sourceEnv)
		}
		assets, unmapped, err := fetchSourceImmich(cmd.Context(), sourceURL, key, opt.maxFiles)
		if err != nil {
			return err
		}
		if len(assets) > 0 {
			if dir, ok := assets[0].Metadata["source_temp_dir"].(string); ok {
				defer os.RemoveAll(dir)
			}
		}
		sum, err := uploadAssets(cmd.Context(), flags, assets, opt)
		if err != nil {
			return err
		}
		sum.Source = "immich"
		sum.UnmappedMetadata = append(sum.UnmappedMetadata, unmapped...)
		if !opt.dryRun && len(sum.AssetMapping) > 0 {
			if err := mapSourceCollections(cmd.Context(), flags, sourceURL, key, sum.AssetMapping); err != nil {
				return err
			}
		}
		return writeImportSummary(cmd, flags, sum)
	}}
	bindImportFlags(cmd, &opt, false)
	cmd.Flags().StringVar(&sourceURL, "source-url", "", "Source Immich base URL")
	cmd.Flags().StringVar(&sourceEnv, "source-api-key-env", "", "Environment variable holding source API key")
	return cmd
}

func mapSourceCollections(ctx context.Context, flags *rootFlags, base, key string, mapping map[string]string) error {
	base = strings.TrimRight(base, "/")
	get := func(path string) ([]byte, error) {
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("x-api-key", key)
		r, e := http.DefaultClient.Do(req)
		if e != nil {
			return nil, e
		}
		defer r.Body.Close()
		b, e := io.ReadAll(r.Body)
		if e != nil {
			return nil, e
		}
		if r.StatusCode < 200 || r.StatusCode >= 300 {
			return nil, fmt.Errorf("source %s returned HTTP %d", path, r.StatusCode)
		}
		return b, nil
	}
	albums, e := get("/albums")
	if e != nil {
		return e
	}
	var albumList []sourceAlbum
	if err := json.Unmarshal(albums, &albumList); err != nil {
		return fmt.Errorf("decode source albums: %w", err)
	}
	tags, e := get("/tags")
	if e != nil {
		return e
	}
	var tagList []sourceTag
	if err := json.Unmarshal(tags, &tagList); err != nil {
		return fmt.Errorf("decode source tags: %w", err)
	}

	// Source reads and every mapping validation happen before the first
	// destination mutation, so a malformed or incomplete later collection
	// cannot leave an apparently successful partial migration behind.
	albumMutations := make([]collectionMutation, 0, len(albumList))
	for _, src := range albumList {
		if src.AlbumName == "" || len(src.Assets) == 0 {
			continue
		}
		ids, err := mappedCollectionAssetIDs("album", src.AlbumName, src.Assets, mapping)
		if err != nil {
			return err
		}
		albumMutations = append(albumMutations, collectionMutation{Name: src.AlbumName, AssetIDs: ids})
	}
	tagMutations := make([]collectionMutation, 0, len(tagList))
	for _, src := range tagList {
		if src.Name == "" || len(src.Assets) == 0 {
			continue
		}
		ids, err := mappedCollectionAssetIDs("tag", src.Name, src.Assets, mapping)
		if err != nil {
			return err
		}
		tagMutations = append(tagMutations, collectionMutation{Name: src.Name, AssetIDs: ids})
	}
	c, e := flags.newClient()
	if e != nil {
		return e
	}
	existingAlbums, err := destinationCollectionIDs(ctx, c, "/albums", "albumName")
	if err != nil {
		return fmt.Errorf("list destination albums: %w", err)
	}
	existingTags, err := destinationCollectionIDs(ctx, c, "/tags", "name")
	if err != nil {
		return fmt.Errorf("list destination tags: %w", err)
	}
	for _, mutation := range albumMutations {
		id := existingAlbums[mutation.Name]
		if id == "" {
			d, _, createErr := c.Post(ctx, "/albums", map[string]any{"albumName": mutation.Name})
			if createErr != nil {
				return createErr
			}
			id = jsonID(d)
			if id == "" {
				return fmt.Errorf("create destination album %q returned no id", mutation.Name)
			}
			existingAlbums[mutation.Name] = id
		}
		if _, _, e = c.Put(ctx, "/albums/"+url.PathEscape(id)+"/assets", map[string]any{"ids": mutation.AssetIDs}); e != nil {
			return e
		}
	}
	for _, mutation := range tagMutations {
		id := existingTags[mutation.Name]
		if id == "" {
			d, _, createErr := c.Post(ctx, "/tags", map[string]any{"name": mutation.Name})
			if createErr != nil {
				return createErr
			}
			id = jsonID(d)
			if id == "" {
				return fmt.Errorf("create destination tag %q returned no id", mutation.Name)
			}
			existingTags[mutation.Name] = id
		}
		if _, _, e = c.Put(ctx, "/tags/"+url.PathEscape(id)+"/assets", map[string]any{"ids": mutation.AssetIDs}); e != nil {
			return e
		}
	}
	return nil
}

func destinationCollectionIDs(ctx context.Context, c *client.Client, path, nameField string) (map[string]string, error) {
	data, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", path, err)
	}
	ids := make(map[string]string, len(rows))
	for _, row := range rows {
		name, _ := row[nameField].(string)
		id, _ := row["id"].(string)
		if name != "" && id != "" {
			if _, exists := ids[name]; !exists {
				ids[name] = id
			}
		}
	}
	return ids, nil
}

type sourceCollectionAsset struct {
	ID string `json:"id"`
}

type sourceAlbum struct {
	AlbumName string                  `json:"albumName"`
	Assets    []sourceCollectionAsset `json:"assets"`
}

type sourceTag struct {
	Name   string                  `json:"name"`
	Assets []sourceCollectionAsset `json:"assets"`
}

type collectionMutation struct {
	Name     string
	AssetIDs []string
}

func mappedCollectionAssetIDs(kind, name string, assets []sourceCollectionAsset, mapping map[string]string) ([]string, error) {
	ids := make([]string, 0, len(assets))
	for _, asset := range assets {
		id := mapping[asset.ID]
		if id == "" {
			return nil, fmt.Errorf("source %s %q has no destination mapping for asset %q", kind, name, asset.ID)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func collectFolder(root string, opt importOptions) ([]importAsset, int, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, 0, fmt.Errorf("stat import source: %w", err)
	}
	if !info.IsDir() {
		return nil, 0, fmt.Errorf("import source %q is not a directory", root)
	}
	var assets []importAsset
	skipped := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if !opt.recursive {
				return filepath.SkipDir
			}
			if !opt.includeHidden && hiddenPath(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			skipped++
			return nil
		}
		if (!opt.includeHidden && hiddenPath(rel)) || ignored(rel, opt.ignores) || junkFile(d.Name()) || !mediaFile(d.Name()) {
			skipped++
			return nil
		}
		if opt.maxFiles > 0 && len(assets) >= opt.maxFiles {
			skipped++
			return nil
		}
		fi, e := d.Info()
		if e != nil {
			return e
		}
		assets = append(assets, importAsset{Path: path, Name: d.Name(), Created: fi.ModTime(), Modified: fi.ModTime(), Metadata: map[string]any{"source_relative_path": filepath.ToSlash(rel)}})
		return nil
	})
	return assets, skipped, err
}

func hiddenPath(rel string) bool {
	for _, p := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(p, ".") {
			return true
		}
	}
	return false
}
func ignored(rel string, patterns []string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, filepath.ToSlash(rel)); ok {
			return true
		}
		if ok, _ := filepath.Match(p, filepath.Base(rel)); ok {
			return true
		}
	}
	return false
}
func junkFile(n string) bool {
	lower := strings.ToLower(n)
	return strings.HasPrefix(n, "._") || n == ".DS_Store" || strings.HasPrefix(n, "Thumbs.db") || strings.HasSuffix(lower, ".tmp")
}
func mediaFile(n string) bool {
	switch strings.ToLower(filepath.Ext(n)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".heif", ".tif", ".tiff", ".dng", ".cr2", ".nef", ".arw", ".mp4", ".mov", ".mkv", ".avi", ".webm", ".3gp":
		return true
	}
	return false
}

func unzipToTemp(path string) (string, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer z.Close()
	dst, err := os.MkdirTemp("", "immich-archive-")
	if err != nil {
		return "", err
	}
	for _, f := range z.File {
		target := filepath.Join(dst, f.Name)
		clean := filepath.Clean(target)
		if !strings.HasPrefix(clean, dst+string(os.PathSeparator)) {
			_ = os.RemoveAll(dst)
			return "", fmt.Errorf("archive entry escapes destination: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(clean, 0750); err != nil {
				_ = os.RemoveAll(dst)
				return "", err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(clean), 0750); err != nil {
			_ = os.RemoveAll(dst)
			return "", err
		}
		in, e := f.Open()
		if e != nil {
			_ = os.RemoveAll(dst)
			return "", e
		}
		out, e := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if e == nil {
			_, e = io.Copy(out, in)
			_ = out.Close()
		}
		_ = in.Close()
		if e != nil {
			_ = os.RemoveAll(dst)
			return "", e
		}
	}
	return dst, nil
}

func applyTakeoutSidecars(root string, assets []importAsset) {
	byRelative := map[string]*importAsset{}
	for i := range assets {
		if rel, _ := assets[i].Metadata["source_relative_path"].(string); rel != "" {
			byRelative[rel] = &assets[i]
		}
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return nil
		}
		var m map[string]any
		if json.Unmarshal(b, &m) != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		target := strings.TrimSuffix(filepath.ToSlash(rel), ".json")
		a := byRelative[target]
		if a == nil {
			return nil
		}
		if a.Metadata == nil {
			a.Metadata = map[string]any{}
		}
		for _, k := range []string{"description", "albumData", "geoData", "photoTakenTime"} {
			if v, ok := m[k]; ok {
				a.Metadata[k] = v
			}
		}
		return nil
	})
}

func applyICloudCSV(root string, assets []importAsset) {
	byName := map[string]*importAsset{}
	for i := range assets {
		byName[assets[i].Name] = &assets[i]
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil || d.IsDir() || strings.ToLower(filepath.Ext(d.Name())) != ".csv" {
			return nil
		}
		f, e := os.Open(path)
		if e != nil {
			return nil
		}
		defer f.Close()
		r := csv.NewReader(f)
		rows, e := r.ReadAll()
		if e != nil || len(rows) < 2 {
			return nil
		}
		head := map[string]int{}
		for i, h := range rows[0] {
			head[strings.ToLower(strings.TrimSpace(h))] = i
		}
		nameIdx, ok := head["filename"]
		if !ok {
			nameIdx = head["file name"]
		}
		for _, row := range rows[1:] {
			if nameIdx >= len(row) {
				continue
			}
			a := byName[row[nameIdx]]
			if a == nil {
				continue
			}
			if a.Metadata == nil {
				a.Metadata = map[string]any{}
			}
			for _, key := range []string{"album", "date", "description"} {
				if ix, ok := head[key]; ok && ix < len(row) && row[ix] != "" {
					a.Metadata[key] = row[ix]
				}
			}
		}
		return nil
	})
}

type importSummary struct {
	Source                string            `json:"source"`
	Selected              int               `json:"selected"`
	Uploaded              int               `json:"uploaded"`
	Duplicate             int               `json:"duplicate"`
	Skipped               int               `json:"skipped"`
	Failed                int               `json:"failed"`
	Cancelled             bool              `json:"cancelled,omitempty"`
	UnmappedMetadata      []string          `json:"unmapped_metadata,omitempty"`
	Warnings              []string          `json:"warnings,omitempty"`
	Errors                []string          `json:"errors,omitempty"`
	AssetMapping          map[string]string `json:"asset_mapping,omitempty"`
	UploadedAssetIDs      []string          `json:"uploaded_asset_ids,omitempty"`
	AlbumAssignmentFailed bool              `json:"-"`
}

func (s *importSummary) add(o importSummary) {
	s.Selected += o.Selected
	s.Uploaded += o.Uploaded
	s.Duplicate += o.Duplicate
	s.Skipped += o.Skipped
	s.Failed += o.Failed
	s.UnmappedMetadata = append(s.UnmappedMetadata, o.UnmappedMetadata...)
	s.Warnings = append(s.Warnings, o.Warnings...)
	s.Errors = append(s.Errors, o.Errors...)
	s.UploadedAssetIDs = append(s.UploadedAssetIDs, o.UploadedAssetIDs...)
	s.AlbumAssignmentFailed = s.AlbumAssignmentFailed || o.AlbumAssignmentFailed
	if s.AssetMapping == nil {
		s.AssetMapping = map[string]string{}
	}
	for key, value := range o.AssetMapping {
		s.AssetMapping[key] = value
	}
}
func runImport(cmd *cobra.Command, flags *rootFlags, assets []importAsset, skipped int, opt importOptions, source string) error {
	sum, err := uploadAssets(cmd.Context(), flags, assets, opt)
	if err != nil {
		return err
	}
	sum.Source = source
	sum.Skipped += skipped
	return writeImportSummary(cmd, flags, sum)
}
func writeImportSummary(cmd *cobra.Command, flags *rootFlags, sum importSummary) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), sum, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %d selected, %d uploaded, %d duplicates, %d skipped, %d failed\n", sum.Source, sum.Selected, sum.Uploaded, sum.Duplicate, sum.Skipped, sum.Failed)
	if sum.Failed > 0 {
		return apiErr(fmt.Errorf("%d import operations failed", sum.Failed))
	}
	return nil
}

func uploadAssets(ctx context.Context, flags *rootFlags, assets []importAsset, opt importOptions) (importSummary, error) {
	sum := importSummary{Selected: len(assets), AssetMapping: map[string]string{}}
	if opt.dryRun {
		sum.Skipped = len(assets)
		return sum, nil
	}
	if opt.concurrency < 1 || opt.concurrency > 32 {
		return sum, usageErr(fmt.Errorf("--concurrency must be between 1 and 32"))
	}
	c, err := flags.newClient()
	if err != nil {
		return sum, err
	}
	var mu sync.Mutex
	jobs := make(chan importAsset)
	var wg sync.WaitGroup
	uploadedIDs := []string{}
	for i := 0; i < opt.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for a := range jobs {
				duplicate, id, unmapped, warnings, uploadErr := uploadOne(ctx, c, a)
				mu.Lock()
				if uploadErr != nil {
					sum.Failed++
					sum.Errors = append(sum.Errors, uploadErr.Error())
				} else if duplicate {
					sum.Duplicate++
					if id != "" && opt.album != "" {
						uploadedIDs = append(uploadedIDs, id)
					}
					if sourceID, _ := a.Metadata["source_asset_id"].(string); sourceID != "" && id != "" {
						sum.AssetMapping[sourceID] = id
					}
				} else {
					sum.Uploaded++
					if id != "" {
						uploadedIDs = append(uploadedIDs, id)
						sum.UploadedAssetIDs = append(sum.UploadedAssetIDs, id)
						if sourceID, _ := a.Metadata["source_asset_id"].(string); sourceID != "" {
							sum.AssetMapping[sourceID] = id
						}
					}
					sum.UnmappedMetadata = append(sum.UnmappedMetadata, unmapped...)
					sum.Warnings = append(sum.Warnings, warnings...)
				}
				mu.Unlock()
			}
		}()
	}
	for _, a := range assets {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return sum, ctx.Err()
		case jobs <- a:
		}
	}
	close(jobs)
	wg.Wait()
	if opt.album != "" && len(uploadedIDs) > 0 {
		if err := assignAlbum(ctx, c, opt.album, uploadedIDs); err != nil {
			// The assets are already committed. Preserve their IDs in the
			// summary and make the membership failure explicit so an operator
			// can repair it without re-uploading duplicate assets.
			sum.Failed++
			sum.AlbumAssignmentFailed = true
			sum.Errors = append(sum.Errors, fmt.Sprintf("assign uploaded assets to album %q: %v", opt.album, err))
		}
	}
	return sum, nil
}

func uploadOne(ctx context.Context, c *client.Client, a importAsset) (bool, string, []string, []string, error) {
	checksum, err := sha1File(a.Path)
	if err != nil {
		return false, "", nil, nil, err
	}
	assetID := hex.EncodeToString(checksum)
	check := map[string]any{"assets": []map[string]any{{"id": assetID, "checksum": assetID}}}
	data, _, err := c.Post(ctx, "/assets/bulk-upload-check", check)
	if err != nil {
		return false, "", nil, nil, err
	}
	duplicate, existingID, err := duplicateCheckResult(data)
	if err != nil {
		return false, "", nil, nil, err
	}
	if duplicate {
		return true, existingID, nil, nil, nil
	}
	fields := map[string]string{"fileCreatedAt": a.Created.UTC().Format(time.RFC3339), "fileModifiedAt": a.Modified.UTC().Format(time.RFC3339), "filename": a.Name}
	data, _, err = c.PostMultipart(ctx, "/assets", fields, map[string]string{"assetData": a.Path})
	if err != nil {
		return false, "", nil, nil, err
	}
	id, unmapped := jsonID(data), []string{}
	if len(a.Metadata) == 0 {
		return false, id, unmapped, nil, nil
	}
	if id == "" {
		for key := range a.Metadata {
			unmapped = append(unmapped, key)
		}
		return false, id, unmapped, nil, nil
	}
	mapped := map[string]any{"ids": []string{id}}
	if value, ok := a.Metadata["description"].(string); ok && value != "" {
		mapped["description"] = value
	}
	if value, ok := a.Metadata["date"].(string); ok && value != "" {
		mapped["dateTimeOriginal"] = value
	}
	if photo, ok := a.Metadata["photoTakenTime"].(map[string]any); ok {
		if value, ok := photo["timestamp"].(string); ok && value != "" {
			mapped["dateTimeOriginal"] = value
		}
	}
	if geo, ok := a.Metadata["geoData"].(map[string]any); ok {
		if v, ok := geo["latitude"].(float64); ok {
			mapped["latitude"] = v
		}
		if v, ok := geo["longitude"].(float64); ok {
			mapped["longitude"] = v
		}
	}
	for key := range a.Metadata {
		if _, ok := mapped[key]; !ok {
			unmapped = append(unmapped, key)
		}
	}
	if len(mapped) > 1 {
		if _, _, err := c.Put(ctx, "/assets", mapped); err != nil {
			return false, id, unmapped, []string{fmt.Sprintf("asset %s uploaded but metadata update failed: %v", id, err)}, nil
		}
	}
	return false, id, unmapped, nil, nil
}
func sha1File(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
func duplicateCheck(data []byte) (bool, error) {
	duplicate, _, err := duplicateCheckResult(data)
	return duplicate, err
}

func duplicateCheckResult(data []byte) (bool, string, error) {
	var response struct {
		Results []struct {
			ID      string `json:"id"`
			AssetID string `json:"assetId"`
			Action  string `json:"action"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return false, "", fmt.Errorf("decode bulk-upload-check response: %w", err)
	}
	items := response.Results
	if len(items) != 1 || items[0].Action == "" {
		return false, "", fmt.Errorf("decode bulk-upload-check response: expected one action")
	}
	switch strings.ToLower(items[0].Action) {
	case "duplicate", "reject":
		// `id` is the client-supplied upload/checksum ID; `assetId` is the
		// existing Immich asset UUID required by album membership endpoints.
		if items[0].AssetID != "" {
			return true, items[0].AssetID, nil
		}
		return true, items[0].ID, nil
	case "accept":
		return false, "", nil
	default:
		return false, "", fmt.Errorf("decode bulk-upload-check response: unknown action %q", items[0].Action)
	}
}
func jsonID(data []byte) string {
	var m map[string]any
	if json.Unmarshal(data, &m) == nil {
		if s, _ := m["id"].(string); s != "" {
			return s
		}
		if a, ok := m["asset"].(map[string]any); ok {
			if s, _ := a["id"].(string); s != "" {
				return s
			}
		}
	}
	return ""
}
func assignAlbum(ctx context.Context, c *client.Client, name string, ids []string) error {
	data, _, e := c.Post(ctx, "/albums", map[string]any{"albumName": name})
	if e != nil {
		return classifyAPIError(e, nil)
	}
	id := jsonID(data)
	if id == "" {
		return fmt.Errorf("create album %q returned no id", name)
	}
	_, _, e = c.Put(ctx, "/albums/"+url.PathEscape(id)+"/assets", map[string]any{"ids": ids})
	if e != nil {
		return classifyAPIError(e, nil)
	}
	return nil
}
func fileSize(path string) int64 {
	if i, e := os.Stat(path); e == nil {
		return i.Size()
	}
	return -1
}

type fileStamp struct {
	size     int64
	mod      time.Time
	first    time.Time
	uploaded bool
}

// readyWatchAssets records the current stamp for each candidate and returns
// only assets that have remained unchanged for stable. A completed stamp is
// never returned again unless size or mtime changes, which starts a new stable
// interval and makes the changed file eligible for one additional upload.
func readyWatchAssets(assets []importAsset, seen map[string]fileStamp, now time.Time, stable time.Duration) []importAsset {
	ready := make([]importAsset, 0, len(assets))
	for _, a := range assets {
		st := fileStamp{size: fileSize(a.Path), mod: a.Modified, first: now}
		if old, ok := seen[a.Path]; ok && old.size == st.size && old.mod.Equal(st.mod) {
			st.first, st.uploaded = old.first, old.uploaded
		}
		seen[a.Path] = st
		if !st.uploaded && now.Sub(st.first) >= stable {
			ready = append(ready, a)
		}
	}
	return ready
}

func markWatchAssetsUploaded(assets []importAsset, seen map[string]fileStamp) {
	for _, a := range assets {
		st := seen[a.Path]
		st.uploaded = true
		seen[a.Path] = st
	}
}

func shouldMarkWatchAssetsUploaded(sum importSummary) bool {
	return sum.Failed == 0
}

func fetchSourceImmich(ctx context.Context, base, key string, max int) ([]importAsset, []string, error) {
	base = strings.TrimRight(base, "/")
	tmp, e := os.MkdirTemp("", "immich-migrate-")
	if e != nil {
		return nil, nil, e
	}
	assets := []importAsset{}
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, nil, err
		}
		body, _ := json.Marshal(map[string]any{"size": 1000, "page": page})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/search/metadata", strings.NewReader(string(body)))
		if err != nil {
			_ = os.RemoveAll(tmp)
			return nil, nil, err
		}
		req.Header.Set("x-api-key", key)
		req.Header.Set("content-type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			_ = os.RemoveAll(tmp)
			return nil, nil, err
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			_ = os.RemoveAll(tmp)
			return nil, nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = os.RemoveAll(tmp)
			return nil, nil, fmt.Errorf("source Immich search returned HTTP %d", resp.StatusCode)
		}
		var out struct {
			Assets struct {
				Items []map[string]any `json:"items"`
			} `json:"assets"`
		}
		if json.Unmarshal(b, &out) != nil {
			_ = os.RemoveAll(tmp)
			return nil, nil, fmt.Errorf("decode source Immich assets")
		}
		if len(out.Assets.Items) == 0 {
			break
		}
		for _, item := range out.Assets.Items {
			if max > 0 && len(assets) >= max {
				break
			}
			id, _ := item["id"].(string)
			if id == "" {
				continue
			}
			r, e := http.NewRequestWithContext(ctx, http.MethodGet, base+"/assets/"+url.PathEscape(id)+"/original", nil)
			if e != nil {
				_ = os.RemoveAll(tmp)
				return nil, nil, e
			}
			r.Header.Set("x-api-key", key)
			rr, e := http.DefaultClient.Do(r)
			if e != nil || rr.StatusCode < 200 || rr.StatusCode >= 300 {
				if rr != nil {
					rr.Body.Close()
				}
				_ = os.RemoveAll(tmp)
				if e != nil {
					return nil, nil, e
				}
				return nil, nil, fmt.Errorf("source original %q returned non-success", id)
			}
			name := id + ".bin"
			if v, _ := item["originalFileName"].(string); v != "" {
				name = filepath.Base(v)
			}
			path := filepath.Join(tmp, id+"-"+name)
			f, e := os.Create(path)
			if e == nil {
				_, e = io.Copy(f, rr.Body)
				_ = f.Close()
			}
			rr.Body.Close()
			if e != nil {
				_ = os.RemoveAll(tmp)
				return nil, nil, e
			}
			assets = append(assets, importAsset{Path: path, Name: name, Created: time.Now(), Modified: time.Now(), Metadata: map[string]any{"source_asset_id": id, "source_temp_dir": tmp}})
		}
		if max > 0 && len(assets) >= max {
			break
		}
	}
	return assets, []string{"source albums and tags are mapped to destination collections after upload"}, nil
}

// pp:data-source live
func newDuplicatePlanCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "plan", Short: "Preview native duplicate groups without mutation", RunE: func(cmd *cobra.Command, _ []string) error {
		c, e := flags.newClient()
		if e != nil {
			return e
		}
		d, e := c.Get(cmd.Context(), "/duplicates", nil)
		if e != nil {
			return classifyAPIError(e, flags)
		}
		plans, err := duplicatePlans(d)
		if err != nil {
			return err
		}
		if limit > 0 && len(plans) > limit {
			plans = plans[:limit]
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": "plan", "groups": plans, "mutates": false}, flags)
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum duplicate groups to include")
	return cmd
}

// pp:data-source live
func newDuplicateApplyCmd(flags *rootFlags) *cobra.Command {
	var groups string
	var apply bool
	cmd := &cobra.Command{Use: "apply [group-id]", Short: "Apply an explicitly reviewed duplicate resolution plan", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !apply {
			return usageErr(fmt.Errorf("--apply is required because duplicate resolution mutates the library"))
		}
		var requested []duplicatePlan
		if groups != "" {
			if json.Unmarshal([]byte(groups), &requested) != nil {
				return usageErr(fmt.Errorf("--groups must be a JSON array"))
			}
		} else if len(args) == 1 {
			requested = []duplicatePlan{{GroupID: args[0]}}
		} else {
			return usageErr(fmt.Errorf("provide a group ID or --groups JSON"))
		}
		c, e := flags.newClient()
		if e != nil {
			return e
		}
		live, e := c.Get(cmd.Context(), "/duplicates", nil)
		if e != nil {
			return classifyAPIError(e, flags)
		}
		current, err := duplicatePlans(live)
		if err != nil {
			return err
		}
		byID := map[string]duplicatePlan{}
		for _, p := range current {
			byID[p.GroupID] = p
		}
		resolve := make([]map[string]any, 0, len(requested))
		for _, want := range requested {
			got, ok := byID[want.GroupID]
			if !ok {
				return fmt.Errorf("duplicate group %q no longer exists", want.GroupID)
			}
			if len(want.Evidence) == 0 {
				return usageErr(fmt.Errorf("duplicate group %q requires the reviewed evidence array from duplicates plan", want.GroupID))
			}
			if !sameStrings(want.Evidence, got.Evidence) {
				return fmt.Errorf("duplicate group %q evidence changed since plan; rerun duplicates plan", want.GroupID)
			}
			if got.KeeperRequired {
				if want.Keeper == "" {
					return usageErr(fmt.Errorf("duplicate group %q has no server keeper recommendation; provide an explicit reviewed keeper", want.GroupID))
				}
				if !containsString(got.Evidence, want.Keeper) {
					return usageErr(fmt.Errorf("keeper %q is not an asset in duplicate group %q", want.Keeper, want.GroupID))
				}
				got.Keep = []string{want.Keeper}
				got.Keeper = want.Keeper
				got.Trash = got.Trash[:0]
				for _, id := range got.Evidence {
					if id != want.Keeper {
						got.Trash = append(got.Trash, id)
					}
				}
				got.KeeperRequired = false
			}
			if (len(want.Keep) > 0 && !sameStrings(want.Keep, got.Keep)) || (want.Keeper != "" && want.Keeper != got.Keeper) || (len(want.Trash) > 0 && !sameStrings(want.Trash, got.Trash)) {
				return fmt.Errorf("duplicate group %q changed since plan; rerun duplicates plan", want.GroupID)
			}
			resolve = append(resolve, map[string]any{"duplicateId": got.GroupID, "keepAssetIds": got.Keep, "trashAssetIds": got.Trash})
		}
		d, _, e := c.Post(cmd.Context(), "/duplicates/resolve", map[string]any{"groups": resolve})
		if e != nil {
			return classifyAPIError(e, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": "apply", "result": json.RawMessage(d)}, flags)
	}}
	cmd.Flags().StringVar(&groups, "groups", "", "Reviewed duplicate group JSON from duplicates plan")
	cmd.Flags().BoolVar(&apply, "apply", false, "Perform the mutation")
	return cmd
}

type duplicatePlan struct {
	GroupID  string   `json:"group_id"`
	Keep     []string `json:"keep"`
	Keeper   string   `json:"keeper"`
	Trash    []string `json:"trash"`
	Evidence []string `json:"evidence"`
	// KeeperRequired is set when Immich supplied no preferred original. The
	// plan remains non-destructive until apply receives a reviewed keeper.
	KeeperRequired bool `json:"keeper_required,omitempty"`
}

func duplicatePlans(data []byte) ([]duplicatePlan, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	groups := extractDuplicateGroups(raw)
	out := make([]duplicatePlan, 0, len(groups))
	for _, g := range groups {
		gid, _ := g["duplicateId"].(string)
		ids := stringValues(g["assets"])
		if len(ids) == 0 {
			ids = stringValues(g["assetIds"])
		}
		sort.Strings(ids)
		if gid == "" || len(ids) < 2 {
			continue
		}
		keep := make([]string, 0, len(ids))
		for _, suggested := range stringValues(g["suggestedKeepAssetIds"]) {
			if containsString(ids, suggested) && !containsString(keep, suggested) {
				keep = append(keep, suggested)
			}
		}
		if len(keep) == 0 {
			// Never invent a destructive keeper choice. Preserve the group in
			// the plan so an operator can explicitly review and select a keeper.
			out = append(out, duplicatePlan{GroupID: gid, Evidence: ids, KeeperRequired: true})
			continue
		}
		trash := make([]string, 0, len(ids)-1)
		for _, id := range ids {
			if !containsString(keep, id) {
				trash = append(trash, id)
			}
		}
		out = append(out, duplicatePlan{GroupID: gid, Keep: keep, Keeper: keep[0], Trash: trash, Evidence: ids})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupID < out[j].GroupID })
	return out, nil
}
func extractDuplicateGroups(v any) []map[string]any {
	switch x := v.(type) {
	case []any:
		out := []map[string]any{}
		for _, e := range x {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		for _, k := range []string{"groups", "items", "data"} {
			if a, ok := x[k].([]any); ok {
				return extractDuplicateGroups(a)
			}
		}
	}
	return nil
}
func stringValues(v any) []string {
	out := []string{}
	if a, ok := v.([]any); ok {
		for _, x := range a {
			switch z := x.(type) {
			case string:
				out = append(out, z)
			case map[string]any:
				if id, _ := z["id"].(string); id != "" {
					out = append(out, id)
				}
			}
		}
	}
	return out
}
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// pp:data-source live
func newPeopleJulyCmd(flags *rootFlags) *cobra.Command {
	var people []string
	var years int
	cmd := &cobra.Command{Use: "july", Short: "Find photos of named people from July across bounded years", RunE: func(cmd *cobra.Command, _ []string) error {
		if len(people) == 0 {
			return usageErr(fmt.Errorf("--person is required"))
		}
		if years < 1 || years > 30 {
			return usageErr(fmt.Errorf("--years must be between 1 and 30"))
		}
		c, e := flags.newClient()
		if e != nil {
			return e
		}
		ids := make([]string, 0, len(people))
		for _, name := range people {
			data, err := c.Get(cmd.Context(), "/search/person", map[string]string{"name": name})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			matches := personMatches(data)
			if len(matches) != 1 {
				return usageErr(fmt.Errorf("person %q resolved to %d matches; use an unambiguous display name", name, len(matches)))
			}
			ids = append(ids, matches[0])
		}
		now := time.Now().UTC()
		seen := map[string]json.RawMessage{}
		periods := make([]map[string]string, 0, years)
		for year := now.Year() - years + 1; year <= now.Year(); year++ {
			from := time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC)
			to := from.AddDate(0, 1, 0)
			data, _, err := c.Post(cmd.Context(), "/search/metadata", map[string]any{"personIds": ids, "takenAfter": from.Format(time.RFC3339), "takenBefore": to.Format(time.RFC3339)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			for id, raw := range assetResultMap(data) {
				seen[id] = raw
			}
			periods = append(periods, map[string]string{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02")})
		}
		result := make([]json.RawMessage, 0, len(seen))
		for _, raw := range seen {
			result = append(result, raw)
		}
		sort.Slice(result, func(i, j int) bool { return string(result[i]) < string(result[j]) })
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"people": ids, "month": "july", "periods": periods, "assets": result, "count": len(result)}, flags)
	}}
	cmd.Flags().StringSliceVar(&people, "person", nil, "Person IDs to include")
	cmd.Flags().IntVar(&years, "years", 5, "Number of July periods to search")
	return cmd
}

func personMatches(data []byte) []string {
	var people []struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &people) != nil {
		return nil
	}
	out := make([]string, 0, len(people))
	for _, p := range people {
		if p.ID != "" {
			out = append(out, p.ID)
		}
	}
	sort.Strings(out)
	return uniqueStrings(out)
}
func assetResultMap(data []byte) map[string]json.RawMessage {
	var x struct {
		Assets struct {
			Items []json.RawMessage `json:"items"`
		} `json:"assets"`
	}
	out := map[string]json.RawMessage{}
	if json.Unmarshal(data, &x) != nil {
		return out
	}
	for _, raw := range x.Assets.Items {
		if id := jsonID(raw); id != "" {
			out[id] = raw
		}
	}
	return out
}
func uniqueStrings(in []string) []string {
	out := in[:0]
	for _, v := range in {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
}

// pp:data-source live
func newMemoriesReviewCmd(flags *rootFlags) *cobra.Command {
	var days int
	var limit int
	cmd := &cobra.Command{Use: "review", Short: "Review a bounded queue of dated memories and memory statistics", RunE: func(cmd *cobra.Command, _ []string) error {
		if days < 1 || days > 3650 {
			return usageErr(fmt.Errorf("--days must be between 1 and 3650"))
		}
		c, e := flags.newClient()
		if e != nil {
			return e
		}
		memoryDate := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
		m, e := c.Get(cmd.Context(), "/memories", map[string]string{"for": memoryDate, "size": fmt.Sprint(limit)})
		if e != nil {
			return classifyAPIError(e, flags)
		}
		s, e := c.Get(cmd.Context(), "/memories/statistics", nil)
		if e != nil {
			return classifyAPIError(e, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"days": days, "memories": json.RawMessage(m), "statistics": json.RawMessage(s)}, flags)
	}}
	cmd.Flags().IntVar(&days, "days", 30, "Window to review")
	cmd.Flags().IntVar(&limit, "limit", 12, "Maximum memories to include")
	return cmd
}

// pp:data-source live
func newStacksReviewCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "review", Short: "Report stack hygiene facts without changing stacks", RunE: func(cmd *cobra.Command, _ []string) error {
		c, e := flags.newClient()
		if e != nil {
			return e
		}
		d, e := c.Get(cmd.Context(), "/stacks", nil)
		if e != nil {
			return classifyAPIError(e, flags)
		}
		var listed []struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(d, &listed) != nil {
			var wrapped struct {
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
			}
			_ = json.Unmarshal(d, &wrapped)
			listed = wrapped.Items
		}
		if limit > 0 && len(listed) > limit {
			listed = listed[:limit]
		}
		review := make([]map[string]any, 0, len(listed))
		for _, stack := range listed {
			if stack.ID == "" {
				continue
			}
			detail, err := c.Get(cmd.Context(), "/stacks/"+url.PathEscape(stack.ID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			n := stackAssetCount(detail)
			class := "normal"
			if n == 0 {
				class = "empty"
			} else if n == 1 {
				class = "singleton"
			} else if n >= 20 {
				class = "large"
			}
			review = append(review, map[string]any{"id": stack.ID, "asset_count": n, "classification": class})
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"limit": limit, "stacks": review, "mutates": false}, flags)
	}}
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum stacks to inspect")
	return cmd
}
func stackAssetCount(raw []byte) int {
	var v struct {
		Assets []json.RawMessage `json:"assets"`
	}
	if json.Unmarshal(raw, &v) == nil && v.Assets != nil {
		return len(v.Assets)
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) == nil {
		for _, key := range []string{"assetIds", "assets"} {
			if a, ok := m[key].([]any); ok {
				return len(a)
			}
		}
	}
	return 0
}
func newAlbumRitualsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "album", Short: "Reviewable personal album workflows", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newAlbumEventCmd(flags))
	return cmd
}

// pp:data-source live
func newAlbumEventCmd(flags *rootFlags) *cobra.Command {
	var name, query string
	var share []string
	var from, to string
	var apply bool
	cmd := &cobra.Command{Use: "event [name]", Short: "Create a reviewable shared event album from explicit date or search filters", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" && len(args) == 1 {
			name = args[0]
		}
		if name == "" || (query == "" && (from == "" || to == "")) {
			return usageErr(fmt.Errorf("provide an event name and either --query or both --from and --to"))
		}
		c, e := flags.newClient()
		if e != nil {
			return e
		}
		searchBody := map[string]any{}
		searchPath := "/search/smart"
		if query != "" {
			searchBody["query"] = query
		} else {
			searchPath = "/search/metadata"
			searchBody["takenAfter"] = from
			searchBody["takenBefore"] = to
		}
		search, _, e := c.Post(cmd.Context(), searchPath, searchBody)
		if e != nil {
			return classifyAPIError(e, flags)
		}
		ids := assetIDs(search)
		proposal := map[string]any{"album_name": name, "asset_ids": ids, "matched_assets": len(ids), "share_with": share, "preview": !apply}
		if !apply {
			return printJSONFiltered(cmd.OutOrStdout(), proposal, flags)
		}
		album, _, e := c.Post(cmd.Context(), "/albums", map[string]any{"albumName": name, "assetIds": ids})
		if e != nil {
			return classifyAPIError(e, flags)
		}
		id := jsonID(album)
		if len(share) > 0 && id != "" {
			albumUsers := make([]map[string]string, 0, len(share))
			for _, userID := range share {
				albumUsers = append(albumUsers, map[string]string{"userId": userID})
			}
			_, _, e = c.Put(cmd.Context(), "/albums/"+url.PathEscape(id)+"/users", map[string]any{"albumUsers": albumUsers})
			if e != nil {
				return classifyAPIError(e, flags)
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"album": json.RawMessage(album), "matched_assets": len(ids), "shared_with": share}, flags)
	}}
	cmd.Flags().StringVar(&name, "name", "", "Album name")
	cmd.Flags().StringVar(&query, "query", "", "Explicit Smart Search query")
	cmd.Flags().StringVar(&from, "from", "", "Inclusive ISO-8601 start date")
	cmd.Flags().StringVar(&to, "to", "", "Exclusive ISO-8601 end date")
	cmd.Flags().StringSliceVar(&share, "share-with", nil, "User IDs to share with")
	cmd.Flags().BoolVar(&apply, "apply", false, "Create the album and optional share after reviewing the preview")
	return cmd
}
func assetIDs(d []byte) []string {
	var response struct {
		Assets struct {
			Items []struct {
				ID               string `json:"id"`
				OriginalFileName string `json:"originalFileName"`
			} `json:"items"`
		} `json:"assets"`
	}
	if json.Unmarshal(d, &response) != nil {
		return nil
	}
	out := make([]string, 0, len(response.Assets.Items))
	for _, asset := range response.Assets.Items {
		if asset.ID != "" {
			out = append(out, asset.ID)
		}
	}
	sort.Strings(out)
	return out
}
func newLibraryRitualsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "library", Short: "Personal-library review and health rituals", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newLibraryReviewCmd(flags))
	cmd.AddCommand(newLibraryHealthCmd(flags))
	return cmd
}

// pp:data-source live
func newLibraryReviewCmd(flags *rootFlags) *cobra.Command {
	var favorite, archived bool
	var mode string
	var limit int
	cmd := &cobra.Command{Use: "review", Short: "Review favorites or archived assets without mutation", RunE: func(cmd *cobra.Command, _ []string) error {
		if mode != "" {
			switch mode {
			case "favorites":
				favorite = true
				archived = false
			case "archived":
				favorite = false
				archived = true
			default:
				return usageErr(fmt.Errorf("--mode must be favorites or archived"))
			}
		}
		if favorite == archived {
			return usageErr(fmt.Errorf("set exactly one of --favorites or --archived"))
		}
		c, e := flags.newClient()
		if e != nil {
			return e
		}
		search := map[string]any{"size": limit}
		if favorite {
			search["isFavorite"] = true
		} else {
			search["visibility"] = "archive"
		}
		d, _, e := c.Post(cmd.Context(), "/search/metadata", search)
		if e != nil {
			return classifyAPIError(e, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"favorites": favorite, "archived": archived, "results": json.RawMessage(d), "mutates": false}, flags)
	}}
	cmd.Flags().BoolVar(&favorite, "favorites", false, "Review favorite assets")
	cmd.Flags().BoolVar(&archived, "archived", false, "Review archived assets")
	cmd.Flags().StringVar(&mode, "mode", "", "Review mode: favorites or archived")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum assets")
	return cmd
}

// pp:data-source live
func newLibraryHealthCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "health", Short: "Report partner sharing and worker-pressure facts", Example: "  immich-pp-cli library health --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		c, e := flags.newClient()
		if e != nil {
			return e
		}
		sharedBy, e := c.Get(cmd.Context(), "/partners", map[string]string{"direction": "shared-by"})
		if e != nil {
			return classifyAPIError(e, flags)
		}
		sharedWith, e := c.Get(cmd.Context(), "/partners", map[string]string{"direction": "shared-with"})
		if e != nil {
			return classifyAPIError(e, flags)
		}
		j, e := c.Get(cmd.Context(), "/jobs", nil)
		if e != nil {
			return classifyAPIError(e, flags)
		}
		q, e := c.Get(cmd.Context(), "/queues", nil)
		if e != nil {
			return classifyAPIError(e, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"partners_shared_by": json.RawMessage(sharedBy), "partners_shared_with": json.RawMessage(sharedWith), "jobs": json.RawMessage(j), "queues": json.RawMessage(q)}, flags)
	}}
}
