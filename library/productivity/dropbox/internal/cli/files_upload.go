// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/platform"
	"github.com/spf13/cobra"
)

var uploadSizeLimit int64 = 150 << 20

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, _, err := root.Find([]string{"files"})
		if err == nil && parent != nil {
			addNovelCommandIfAbsent(parent, newFilesUploadCmd(flags))
		}
	})
}

func newFilesUploadCmd(flags *rootFlags) *cobra.Command {
	var mode string
	var autorename bool
	cmd := &cobra.Command{Use: "upload <local-file> <dropbox-path>", Short: "Upload a local file to Dropbox", Long: "Upload a local file of at most 150 MiB to a Dropbox path. Upload sessions are not supported.", Example: strings.Trim(`
  dropbox-pp-cli files upload report.pdf "/Documents/Taxes/report.pdf" --agent
	  dropbox-pp-cli files upload photo.jpg "/Camera Uploads/photo.jpg" --mode overwrite --yes --agent`, "\n"), Annotations: map[string]string{"pp:data-source": "live", "pp:no-error-path-probe": "true", "mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "files upload")
			}
			if len(args) != 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("files upload requires a local file and Dropbox path"))
			}
			if mode != "add" && mode != "overwrite" {
				return usageErr(fmt.Errorf("--mode must be add or overwrite"))
			}
			if mode == "overwrite" && !flags.yes {
				return usageErr(fmt.Errorf("--mode overwrite requires --yes"))
			}
			if !strings.HasPrefix(args[1], "/") || args[1] == "/" {
				return usageErr(fmt.Errorf("Dropbox path must start with / and name a file"))
			}
			if err := liveSourceOnly(flags); err != nil {
				return err
			}
			if err := safeUploadSource(args[0], flags); err != nil {
				return usageErr(err)
			}
			if cliutil.IsAnyHarness() {
				result := map[string]any{"verify_noop": true, "status": "noop", "reason": "verify_short_circuit", "success": false}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "harness mode: upload skipped")
				return err
			}
			file, err := os.Open(args[0])
			if err != nil {
				return usageErr(err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				return err
			}
			if err := validateUploadHandle(file, args[0], flags); err != nil {
				return usageErr(err)
			}
			if !info.Mode().IsRegular() {
				return usageErr(fmt.Errorf("local file must be a regular file"))
			}
			if info.Size() > uploadSizeLimit {
				return usageErr(fmt.Errorf("file exceeds 150 MiB; upload sessions are not supported"))
			}
			ctx := cmd.Context()
			cancel := func() {}
			if cmd.Flags().Changed("timeout") {
				ctx, cancel = boundCtx(ctx, flags)
			}
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			account, err := fetchDropboxAccount(ctx, c)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			arg, err := dropbox.EncodeAPIArg(map[string]any{"path": args[1], "mode": map[string]string{".tag": mode}, "autorename": autorename, "mute": false, "strict_conflict": false})
			if err != nil {
				return err
			}
			resp, err := contentRequest(ctx, c, flags, http.MethodPost, "/files/upload", file, info.Size(), map[string]string{"Content-Type": "application/octet-stream", "Dropbox-API-Arg": arg}, pathRootHeaders(account))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				return contentError(resp, "upload")
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			if err != nil {
				return err
			}
			var meta transferMetadata
			if err := json.Unmarshal(body, &meta); err != nil {
				return fmt.Errorf("invalid upload metadata: %w", err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), meta, flags)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %s (%d bytes, rev %s)\n", meta.Path, meta.Size, meta.Rev)
			return err
		}}
	cmd.Flags().StringVar(&mode, "mode", "add", "Write mode: add or overwrite")
	cmd.Flags().BoolVar(&autorename, "autorename", false, "Rename on conflict")
	return cmd
}

func safeUploadSource(local string, flags *rootFlags) error {
	if credentialLikeUploadName(local) {
		return fmt.Errorf("refusing to upload a credential-like file")
	}
	local, err := filepath.Abs(local)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(local); err == nil {
		local = resolved
	}
	if credentialLikeUploadName(local) {
		return fmt.Errorf("refusing to upload a credential-like file")
	}
	configDir, err := cliutil.ConfigDir()
	if err != nil {
		return err
	}
	dataDir, err := cliutil.DataDir()
	if err != nil {
		return err
	}
	credentialsPath, err := cliutil.CredentialsFilePath()
	if err != nil {
		return err
	}
	stateDir, err := cliutil.StateDir()
	if err != nil {
		return err
	}
	cacheDir, err := cliutil.CacheDir()
	if err != nil {
		return err
	}
	protected := []string{configDir, dataDir, stateDir, cacheDir, filepath.Dir(credentialsPath), filepath.Join(configDir, "clients"), filepath.Join(dataDir, "clients")}
	// PathsFor applies the shared platform root rules, including XDG overrides.
	shared, err := platform.PathsFor("upload-guard", "dropbox-pp-cli", "dropbox")
	if err != nil {
		return err
	}
	protected = append(protected,
		filepath.Dir(filepath.Dir(filepath.Dir(shared.ConfigFile))),
		filepath.Dir(filepath.Dir(filepath.Dir(shared.DataFile))),
		filepath.Dir(filepath.Dir(shared.StateDir)),
		filepath.Dir(filepath.Dir(filepath.Dir(shared.CacheDir))),
	)
	if flags.configPath != "" {
		protected = append(protected, filepath.Dir(flags.configPath))
	}
	for _, dir := range protected {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return err
		}
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		compareDir, compareLocal := dir, local
		if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
			compareDir, compareLocal = strings.ToLower(dir), strings.ToLower(local)
		}
		rel, err := filepath.Rel(compareDir, compareLocal)
		if err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))) {
			return fmt.Errorf("refusing to upload a file from CLI config or data directories")
		}
	}
	return nil
}

func validateUploadHandle(file *os.File, local string, flags *rootFlags) error {
	if err := safeUploadSource(local, flags); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	configDir, err := cliutil.ConfigDir()
	if err != nil {
		return err
	}
	credentialsPath, err := cliutil.CredentialsFilePath()
	if err != nil {
		return err
	}
	paths := []string{filepath.Join(configDir, "config.json"), credentialsPath}
	legacyPath, err := config.LegacyConfigPath()
	if err != nil {
		return err
	}
	paths = append(paths, legacyPath)
	if flags.configPath != "" {
		paths = append(paths, flags.configPath)
		if p, err := cliutil.CredentialsFilePathForConfig(flags.configPath); err == nil {
			paths = append(paths, p)
		}
	}
	for _, p := range paths {
		protected, err := os.Stat(p)
		if err == nil && os.SameFile(info, protected) {
			return fmt.Errorf("refusing to upload a CLI config or credential file")
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func credentialLikeUploadName(local string) bool {
	name := strings.ToLower(filepath.Base(local))
	normalized := "/" + strings.ToLower(filepath.ToSlash(local)) + "/"
	return strings.Contains(normalized, "/.aws/") || strings.Contains(normalized, "/.ssh/") ||
		name == "config.json" || name == "credentials.toml" || name == ".env" || strings.HasPrefix(name, ".env.") ||
		strings.HasPrefix(name, "id_rsa") || strings.HasPrefix(name, "id_ed25519") ||
		name == ".netrc" || name == ".git-credentials" || strings.HasSuffix(name, ".pem") ||
		strings.HasSuffix(name, ".key") || strings.HasSuffix(name, ".p12") || strings.HasSuffix(name, ".pfx")
}
