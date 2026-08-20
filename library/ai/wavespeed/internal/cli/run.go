package cli

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
	"github.com/spf13/cobra"
)

type runCommandOptions struct {
	input       string
	inputFile   string
	inputKV     []string
	setKV       []string
	prompt      string
	modelID     string
	image       string
	images      []string
	video       string
	endImage    string
	lastImage   string
	refImages   []string
	refVideos   []string
	syncMode    bool
	schema      bool
	helpModel   bool
	wait        bool
	waitTimeout time.Duration
	pollInitial time.Duration
	price       bool
	priceOnly   bool
	download    string
	downloadDir string
	record      bool
}

type wavespeedProjectConfig struct {
	Path         string                           `json:"-"`
	DefaultModel string                           `json:"defaultModel,omitempty"`
	OutputDir    string                           `json:"outputDir,omitempty"`
	Aliases      map[string]wavespeedProjectAlias `json:"aliases,omitempty"`
	// ActiveBrand names the brand profile (stored in library.db) that novel
	// commands and `run` merge by default. Set by `brand apply`.
	ActiveBrand string `json:"activeBrand,omitempty"`
	// Record is the library record policy: "always", "novel-only" (default),
	// or "never". See recordPolicyFor.
	Record string `json:"record,omitempty"`
}

type wavespeedProjectAlias struct {
	Model string         `json:"model,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

func newRunCmd(flags *rootFlags) *cobra.Command {
	var opts runCommandOptions

	cmd := &cobra.Command{
		Use:     "run [model-or-alias]",
		Aliases: []string{"submit"},
		Short:   "Submit a generation task to a WaveSpeed model",
		Long:    "Submit a generation task to a WaveSpeed model. Model IDs are slash-delimited API paths such as wavespeed-ai/hunyuan-video/t2v. Non-slash names resolve through wavespeed.json aliases.",
		Example: `  wavespeed-pp-cli run wavespeed-ai/flux-dev -p "a studio product photo" --wait
  wavespeed-pp-cli run hero -i size=1024 -i enable_base64_output=false --price --wait --download ./outputs/{index}.{ext}
  wavespeed-pp-cli run --model-id wavespeed-ai/flux-dev --prompt "agent-friendly MCP call" --price-only`,
		Args:        validateRunArgs(&opts),
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/{model_id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadWavespeedProjectConfig()
			if err != nil {
				return usageErr(err)
			}

			modelArgs, downloadArg := splitRunArgs(cmd, args, &opts)
			if downloadArg != "" {
				opts.download = downloadArg
			}

			modelToken := ""
			if cmd.Flags().Changed("model-id") {
				modelToken = opts.modelID
			} else if len(modelArgs) > 0 {
				modelToken = modelArgs[0]
			} else {
				modelToken = project.DefaultModel
			}
			modelID, aliasDefaults, err := resolveProjectModel(project, modelToken)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if opts.schema {
				return printModelSchema(cmd, flags, c, modelID)
			}
			if opts.helpModel {
				return printModelHelp(cmd, flags, c, modelID)
			}

			changed := runChangedFlags(cmd)
			inputs, err := readRunInputs(opts, aliasDefaults, changed)
			if err != nil {
				return usageErr(err)
			}
			if err := resolveRunInputRefs(cmd.Context(), c, inputs, cmd.ErrOrStderr()); err != nil {
				return err
			}

			// price-only short-circuits before submission.
			if opts.priceOnly {
				pricing, _, err := c.PostQueryWithParams(cmd.Context(), "/model/pricing", nil, map[string]any{
					"model_id": modelID,
					"inputs":   inputs,
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return printOutputWithFlags(cmd.OutOrStdout(), pricing, flags)
			}

			res, err := submitAndAwait(cmd.Context(), c, submitRequest{
				modelID:       modelID,
				inputs:        inputs,
				estimatePrice: opts.price,
				wait:          opts.wait,
				waitTimeout:   opts.waitTimeout,
				pollInitial:   opts.pollInitial,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if len(res.Pricing) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "Pricing estimate:")
				_ = printOutput(cmd.ErrOrStderr(), res.Pricing, true)
			}
			for _, item := range res.Downloads {
				fmt.Fprintf(cmd.ErrOrStderr(), "downloaded %s\n", item.Path)
			}
			if err := rememberDownloads(res.Downloads); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: recording last download failed: %v\n", err)
			}

			// run records to the library only when --record is passed (opt-in),
			// the inverse of novel commands which record by default. A record
			// failure is logged and never fails a successful generation.
			if opts.record {
				if recErr := recordRunGeneration(modelID, inputs, res); recErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: library record failed: %v\n", recErr)
				}
			}

			var downloadSpec string
			var plannedDownloads []downloadedFile
			if cmd.Flags().Changed("download") {
				downloadSpec = runDownloadSpec(opts, project, cmd.Flags().Changed("download-dir"))
				plannedDownloads = planRunDownloads(unwrapWaveSpeedData(res.Result), downloadSpec)
			}

			output := runOutputEnvelope(res.Pricing, res.Result, plannedDownloads)
			if err := printOutputWithFlags(cmd.OutOrStdout(), output, flags); err != nil {
				return err
			}
			if res.Failed {
				return apiErr(fmt.Errorf("prediction finished with status %q", res.Status))
			}
			if cmd.Flags().Changed("download") {
				downloads, err := downloadPlannedRunOutputs(cmd.Context(), c, plannedDownloads)
				for _, item := range downloads {
					fmt.Fprintf(cmd.ErrOrStderr(), "downloaded %s\n", item.Path)
				}
				if recErr := rememberDownloads(downloads); recErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: recording last download failed: %v\n", recErr)
				}
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: download failed: %v\n", err)
					return nil
				}
			}
			return nil
		},
	}

	addRunInputFlags(cmd, &opts)
	cmd.Flags().BoolVar(&opts.schema, "schema", false, "Print this model's request schema instead of submitting a run")
	cmd.Flags().BoolVar(&opts.helpModel, "help-model", false, "Print this model's request schema instead of submitting a run")
	cmd.Flags().BoolVar(&opts.wait, "wait", false, "Poll until the prediction reaches a terminal status")
	cmd.Flags().DurationVar(&opts.waitTimeout, "wait-timeout", 5*time.Minute, "Maximum time to wait for a terminal prediction status")
	cmd.Flags().DurationVar(&opts.pollInitial, "poll-interval", 2*time.Second, "Initial polling interval for --wait")
	cmd.Flags().BoolVar(&opts.price, "price", false, "Estimate model pricing before submitting")
	cmd.Flags().BoolVar(&opts.priceOnly, "price-only", false, "Estimate model pricing without submitting")
	cmd.Flags().StringVar(&opts.download, "download", "", "Download output URLs; optional path/template such as ./out/{index}.{ext}")
	if flag := cmd.Flags().Lookup("download"); flag != nil {
		flag.NoOptDefVal = "true"
	}
	cmd.Flags().StringVar(&opts.downloadDir, "download-dir", ".", "Directory for files saved by --download without a path")
	cmd.Flags().BoolVar(&opts.record, "record", false, "Record this generation to the library DB (novel commands record by default; run is opt-in)")
	installRunModelHelp(cmd, flags, &opts)

	return cmd
}

func validateRunArgs(opts *runCommandOptions) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		allowed := 1
		if acceptsDownloadPathArg(cmd, opts) && !cmd.Flags().Changed("model-id") {
			allowed = 2
		}
		if len(args) <= allowed {
			return nil
		}
		extra := args[allowed]
		if acceptsDownloadPathArg(cmd, opts) {
			return fmt.Errorf("accepts at most %d arg(s), received %d; unexpected extra arg %q (if this is a download path, pass it immediately after --download or use --download=%s)", allowed, len(args), extra, extra)
		}
		return fmt.Errorf("accepts at most %d arg(s), received %d; unexpected extra arg %q", allowed, len(args), extra)
	}
}

func splitRunArgs(cmd *cobra.Command, args []string, opts *runCommandOptions) ([]string, string) {
	if !acceptsDownloadPathArg(cmd, opts) || len(args) == 0 {
		return args, ""
	}
	if cmd.Flags().Changed("model-id") {
		return nil, args[0]
	}
	if len(args) >= 2 {
		return args[:1], args[1]
	}
	return args, ""
}

func acceptsDownloadPathArg(cmd *cobra.Command, opts *runCommandOptions) bool {
	return cmd.Flags().Changed("download") && opts.download == "true"
}

func installRunModelHelp(cmd *cobra.Command, flags *rootFlags, opts *runCommandOptions) {
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(helpCmd *cobra.Command, args []string) {
		defaultHelp(helpCmd, args)
		cleanArgs := args
		if len(cleanArgs) > 0 && cleanArgs[0] == helpCmd.Name() {
			cleanArgs = cleanArgs[1:]
		}
		filteredArgs := cleanArgs[:0]
		for _, arg := range cleanArgs {
			if arg == "--help" || arg == "-h" {
				continue
			}
			filteredArgs = append(filteredArgs, arg)
		}
		cleanArgs = filteredArgs
		modelToken := ""
		if helpCmd.Flags().Changed("model-id") {
			modelToken = opts.modelID
		} else if len(cleanArgs) > 0 {
			modelToken = cleanArgs[0]
		}
		if strings.TrimSpace(modelToken) == "" {
			return
		}
		project, err := loadWavespeedProjectConfig()
		if err != nil {
			fmt.Fprintf(helpCmd.ErrOrStderr(), "\nModel schema unavailable: %v\n", err)
			return
		}
		modelID, _, err := resolveProjectModel(project, modelToken)
		if err != nil {
			fmt.Fprintf(helpCmd.ErrOrStderr(), "\nModel schema unavailable: %v\n", err)
			return
		}
		c, err := flags.newClient()
		if err != nil {
			fmt.Fprintf(helpCmd.ErrOrStderr(), "\nModel schema unavailable: %v\n", err)
			return
		}
		models, err := c.Get(helpCmd.Context(), "/models", nil)
		if err != nil {
			fmt.Fprintf(helpCmd.ErrOrStderr(), "\nModel schema unavailable: %v\n", err)
			return
		}
		model, ok := findModelObject(models, modelID)
		if !ok {
			fmt.Fprintf(helpCmd.ErrOrStderr(), "\nModel schema unavailable: model %q not found in /models response\n", modelID)
			return
		}
		fmt.Fprintf(helpCmd.OutOrStdout(), "\n%s", modelHelpText(modelID, model))
	})
}

func newSchemaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "schema [model-or-alias]",
		Short:       "Print the request schema for a WaveSpeed model",
		Long:        "Print the request schema for a WaveSpeed model by fetching the dynamic /models catalog and reading api_schema.api_schemas[0].request_schema. Non-slash names resolve through wavespeed.json aliases.",
		Example:     "  wavespeed-pp-cli schema wavespeed-ai/flux-dev\n  wavespeed-pp-cli schema hero --json",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadWavespeedProjectConfig()
			if err != nil {
				return usageErr(err)
			}
			modelToken := project.DefaultModel
			if len(args) > 0 {
				modelToken = args[0]
			}
			modelID, _, err := resolveProjectModel(project, modelToken)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			return printModelSchema(cmd, flags, c, modelID)
		},
	}
	return cmd
}

func newPriceCmd(flags *rootFlags) *cobra.Command {
	var opts runCommandOptions
	cmd := &cobra.Command{
		Use:   "price [model-or-alias]",
		Short: "Estimate the price for a WaveSpeed model run",
		Long:  "Estimate the price for a WaveSpeed model run using the same input syntax as run. This calls WaveSpeed's pricing endpoint and does not submit a prediction.",
		Example: `  wavespeed-pp-cli price wavespeed-ai/z-image/turbo -p "a product photo"
  wavespeed-pp-cli price hero -i size=1024*1024 -i output_format=png --json`,
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadWavespeedProjectConfig()
			if err != nil {
				return usageErr(err)
			}
			modelToken := ""
			if cmd.Flags().Changed("model-id") {
				modelToken = opts.modelID
			} else if len(args) > 0 {
				modelToken = args[0]
			} else {
				modelToken = project.DefaultModel
			}
			modelID, aliasDefaults, err := resolveProjectModel(project, modelToken)
			if err != nil {
				return usageErr(err)
			}
			inputs, err := readRunInputs(opts, aliasDefaults, runChangedFlags(cmd))
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := resolveRunInputRefs(cmd.Context(), c, inputs, cmd.ErrOrStderr()); err != nil {
				return err
			}
			pricing, _, err := c.PostQueryWithParams(cmd.Context(), "/model/pricing", nil, map[string]any{
				"model_id": modelID,
				"inputs":   inputs,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), pricing, flags)
		},
	}
	addRunInputFlags(cmd, &opts)
	return cmd
}

type lastDownloadState struct {
	UpdatedAt string           `json:"updated_at"`
	Downloads []downloadedFile `json:"downloads"`
}

func lastDownloadStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	dir := filepath.Join(home, ".wavespeed-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating state dir: %w", err)
	}
	return filepath.Join(dir, "last-download.json"), nil
}

func rememberDownloads(downloads []downloadedFile) error {
	if len(downloads) == 0 {
		return nil
	}
	p, err := lastDownloadStatePath()
	if err != nil {
		return err
	}
	state := lastDownloadState{UpdatedAt: time.Now().Format(time.RFC3339), Downloads: downloads}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}

func loadLastDownloadState() (lastDownloadState, error) {
	p, err := lastDownloadStatePath()
	if err != nil {
		return lastDownloadState{}, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return lastDownloadState{}, err
	}
	var state lastDownloadState
	if err := json.Unmarshal(raw, &state); err != nil {
		return lastDownloadState{}, err
	}
	return state, nil
}

func lastDownloadedPath() (string, error) {
	state, err := loadLastDownloadState()
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no downloaded image recorded yet; run with --download first")
		}
		return "", err
	}
	for i := len(state.Downloads) - 1; i >= 0; i-- {
		if strings.TrimSpace(state.Downloads[i].Path) != "" {
			return state.Downloads[i].Path, nil
		}
	}
	return "", fmt.Errorf("last download record contains no local path")
}

func newLastCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "last",
		Short:       "Print the most recently downloaded WaveSpeed output path",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := loadLastDownloadState()
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no downloaded image recorded yet; run with --download first")
				}
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), state, flags)
			}
			p, err := lastDownloadedPath()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

func newOpenCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "open [path]",
		Short: "Open the most recently downloaded WaveSpeed output (or a supplied path)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := ""
			if len(args) > 0 {
				p = args[0]
			} else {
				var err error
				p, err = lastDownloadedPath()
				if err != nil {
					return err
				}
			}
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("opening %s: %w", p, err)
			}
			if flags.dryRun {
				name, args, err := openCommand(p, runtime.GOOS)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", name, strings.Join(args, " "))
				return nil
			}
			name, args, err := openCommand(p, runtime.GOOS)
			if err != nil {
				return err
			}
			return exec.CommandContext(cmd.Context(), name, args...).Run()
		},
	}
}

func openCommand(p, goos string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{p}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", p}, nil
	case "linux":
		return "xdg-open", []string{p}, nil
	default:
		return "", nil, fmt.Errorf("opening files is not supported on %s", goos)
	}
}

func newUploadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upload <file>...",
		Short:   "Upload local media files for use as model inputs",
		Long:    "Upload local image, video, or audio files to WaveSpeed media storage and print the returned URLs for use with run -i image=<url> or other model-specific media fields.",
		Example: "  wavespeed-pp-cli upload ./input.png\n  wavespeed-pp-cli upload ./a.png ./b.png --json",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results := make([]json.RawMessage, 0, len(args))
			urls := make([]string, 0, len(args))
			for _, file := range args {
				raw, err := uploadMediaBinary(cmd.Context(), c, file, cmd.ErrOrStderr())
				if err != nil {
					return classifyAPIError(err, flags)
				}
				results = append(results, raw)
				if url := uploadedMediaURL(raw); url != "" {
					urls = append(urls, url)
				}
			}
			if flags.quiet {
				for _, u := range urls {
					fmt.Fprintln(cmd.OutOrStdout(), u)
				}
				return nil
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				raw, err := json.MarshalIndent(map[string]any{
					"uploads": results,
					"urls":    urls,
				}, "", "  ")
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(raw), flags)
			}
			for _, u := range urls {
				fmt.Fprintln(cmd.OutOrStdout(), u)
			}
			return nil
		},
	}
	return cmd
}

func newDownloadCmd(flags *rootFlags) *cobra.Command {
	var output string
	var outputDir string
	cmd := &cobra.Command{
		Use:         "download <url>...",
		Short:       "Download one or more WaveSpeed output URLs",
		Long:        "Download one or more WaveSpeed output URLs. Use --output for an exact file path or template, or --output-dir for directory downloads.",
		Example:     "  wavespeed-pp-cli download https://example.com/output.png\n  wavespeed-pp-cli download https://example.com/a.png https://example.com/b.png --output-dir ./outputs\n  wavespeed-pp-cli download https://example.com/a.png --output ./out/final.png",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if !strings.HasPrefix(arg, "http://") && !strings.HasPrefix(arg, "https://") {
					return usageErr(fmt.Errorf("download URL must start with http:// or https://, got %q", arg))
				}
			}
			raw, err := json.Marshal(args)
			if err != nil {
				return err
			}
			spec := outputDir
			if cmd.Flags().Changed("output") {
				spec = output
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			downloads, err := downloadRunOutputs(cmd.Context(), c, json.RawMessage(raw), spec)
			if err != nil {
				return err
			}
			if err := rememberDownloads(downloads); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: recording last download failed: %v\n", err)
			}
			if flags.quiet {
				for _, item := range downloads {
					fmt.Fprintln(cmd.OutOrStdout(), item.Path)
				}
				return nil
			}
			raw, err = json.MarshalIndent(map[string]any{"downloads": downloads}, "", "  ")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(raw), flags)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Exact output file path or template such as ./out/{index}.{ext}")
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Directory for downloaded files")
	return cmd
}

func uploadMediaBinary(ctx context.Context, c *client.Client, filePath string, stderr io.Writer) (json.RawMessage, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading upload file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("reading upload file: %s is a directory", filePath)
	}
	if c.DryRun {
		fmt.Fprintf(stderr, "POST %s/media/upload/binary\n", strings.TrimRight(c.BaseURL, "/"))
		fmt.Fprintf(stderr, "  multipart field file=@%s\n", filePath)
		if c.Config != nil && c.Config.AuthHeader() != "" {
			fmt.Fprintln(stderr, "  Authorization: ****")
		}
		fmt.Fprintln(stderr, "\n(dry run - no request sent)")
		raw, _ := json.Marshal(map[string]any{"dry_run": true, "file": filePath})
		return raw, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening upload file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("creating multipart upload: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("reading upload file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finalizing multipart upload: %w", err)
	}

	target := strings.TrimRight(c.BaseURL, "/") + "/media/upload/binary"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, &body)
	if err != nil {
		return nil, fmt.Errorf("creating upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "wavespeed-pp-cli/1.0.0")
	if c.Config != nil {
		auth, err := c.AuthHeader(ctx)
		if err != nil {
			return nil, err
		}
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		for k, v := range c.Config.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.DoRaw(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &client.APIError{Method: http.MethodPost, Path: "/media/upload/binary", StatusCode: resp.StatusCode, Body: string(data)}
	}
	return json.RawMessage(data), nil
}

func resolveRunInputRefs(ctx context.Context, c *client.Client, inputs map[string]any, stderr io.Writer) error {
	for key, value := range inputs {
		resolved, err := resolveRunInputValue(ctx, c, key, value, stderr)
		if err != nil {
			return err
		}
		inputs[key] = resolved
	}
	return nil
}

func resolveRunInputValue(ctx context.Context, c *client.Client, key string, value any, stderr io.Writer) (any, error) {
	switch typed := value.(type) {
	case string:
		return resolveRunInputStringRef(ctx, c, key, typed, stderr)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			resolved, err := resolveRunInputValue(ctx, c, key, item, stderr)
			if err != nil {
				return nil, err
			}
			out = append(out, resolved)
		}
		return out, nil
	case map[string]any:
		for childKey, item := range typed {
			resolved, err := resolveRunInputValue(ctx, c, childKey, item, stderr)
			if err != nil {
				return nil, err
			}
			typed[childKey] = resolved
		}
		return typed, nil
	default:
		return value, nil
	}
}

func resolveRunInputStringRef(ctx context.Context, c *client.Client, key, value string, stderr io.Writer) (any, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "@") || len(value) == 1 {
		return value, nil
	}
	path := strings.TrimPrefix(value, "@")
	if shouldParseJSONInputRef(key, path) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s for %s: %w", path, key, err)
		}
		var parsed any
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.UseNumber()
		if err := dec.Decode(&parsed); err != nil {
			return nil, fmt.Errorf("parsing %s for %s: %w", path, key, err)
		}
		return normalizeJSONNumbers(parsed), nil
	}
	raw, err := uploadMediaBinary(ctx, c, path, stderr)
	if err != nil {
		return nil, err
	}
	if url := uploadedMediaURL(raw); url != "" {
		return url, nil
	}
	if c.DryRun {
		return "dry-run-upload://" + filepath.Base(path), nil
	}
	return nil, fmt.Errorf("upload response for %s did not include a media URL", path)
}

func shouldParseJSONInputRef(key, path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".json") {
		return false
	}
	return !isMediaInputKey(key)
}

func isMediaInputKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if strings.Contains(key, "image") || strings.Contains(key, "video") || strings.Contains(key, "audio") {
		return true
	}
	switch key {
	case "images", "videos", "reference_images", "reference_videos", "ref_videos", "reference_audios", "image", "video", "audio":
		return true
	default:
		return false
	}
}

func uploadedMediaURL(raw json.RawMessage) string {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	for _, candidate := range [][]string{
		{"data", "download_url"},
		{"data", "url"},
		{"download_url"},
		{"url"},
	} {
		if v, ok := nestedValue(value, candidate...); ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func newAliasesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "aliases",
		Short:       "List aliases from wavespeed.json",
		Long:        "List aliases from the nearest wavespeed.json found by walking from the current directory upward.",
		Example:     "  wavespeed-pp-cli aliases\n  wavespeed-pp-cli aliases --json",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadWavespeedProjectConfig()
			if err != nil {
				return usageErr(err)
			}
			type aliasRow struct {
				Name       string `json:"name"`
				Model      string `json:"model"`
				InputCount int    `json:"input_count"`
			}
			names := make([]string, 0, len(project.Aliases))
			for name := range project.Aliases {
				names = append(names, name)
			}
			sort.Strings(names)
			rows := make([]aliasRow, 0, len(names))
			for _, name := range names {
				alias := project.Aliases[name]
				rows = append(rows, aliasRow{Name: name, Model: alias.Model, InputCount: len(alias.Input)})
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				raw, err := json.MarshalIndent(map[string]any{
					"config_path": project.Path,
					"aliases":     rows,
				}, "", "  ")
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(raw), flags)
			}
			if len(rows) == 0 {
				if project.Path == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "No wavespeed.json found.")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No aliases in %s.\n", project.Path)
				}
				return nil
			}
			tableRows := make([][]string, 0, len(rows))
			for _, row := range rows {
				tableRows = append(tableRows, []string{row.Name, row.Model, strconv.Itoa(row.InputCount)})
			}
			return flags.printTable(cmd, []string{"Alias", "Model", "Inputs"}, tableRows)
		},
	}
	return cmd
}

func newInitCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter wavespeed.json project config",
		Long:  "Write a starter wavespeed.json project config in the current directory when one is not already present in the current directory or its parents.",
		Example: `  wavespeed-pp-cli init
  wavespeed-pp-cli init --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			existing, err := findWavespeedProjectConfig("")
			if err != nil {
				return usageErr(err)
			}
			if existing != "" {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					out, _ := json.MarshalIndent(map[string]any{"path": existing, "created": false}, "", "  ")
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(out), flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wavespeed.json already exists at %s\n", existing)
				return nil
			}
			starter := wavespeedProjectConfig{
				DefaultModel: "wavespeed-ai/example/model",
				OutputDir:    "wavespeed-output",
				Aliases: map[string]wavespeedProjectAlias{
					"hero": {
						Model: "wavespeed-ai/example/model",
						Input: map[string]any{
							"prompt": "a cinematic hero image",
						},
					},
				},
			}
			raw, err := json.MarshalIndent(starter, "", "  ")
			if err != nil {
				return err
			}
			raw = append(raw, '\n')
			path := filepath.Join(".", "wavespeed.json")
			if err := os.WriteFile(path, raw, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(map[string]any{"path": path, "created": true}, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(out), flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", path)
			return nil
		},
	}
	return cmd
}

func addRunInputFlags(cmd *cobra.Command, opts *runCommandOptions) {
	cmd.Flags().StringVar(&opts.modelID, "model-id", "", "Model ID or alias to run; useful for MCP callers")
	cmd.Flags().StringVar(&opts.input, "input", "{}", "JSON object to merge into the model input")
	cmd.Flags().StringVar(&opts.inputFile, "input-file", "", "Read model input JSON from a file, or '-' for stdin")
	cmd.Flags().StringArrayVarP(&opts.inputKV, "input-kv", "i", nil, "Additional model input as key=value (repeatable); values parse as JSON when possible")
	cmd.Flags().StringArrayVar(&opts.setKV, "set", nil, "Additional model input as key=value (repeatable); alias for --input-kv")
	cmd.Flags().StringVarP(&opts.prompt, "prompt", "p", "", "Prompt text mapped to input.prompt")
	cmd.Flags().StringVar(&opts.image, "image", "", "Shortcut for input.image; accepts URL or @local-file")
	cmd.Flags().StringArrayVar(&opts.images, "images", nil, "Shortcut for input.images array; repeat for multiple URLs or @local-files")
	cmd.Flags().StringVar(&opts.video, "video", "", "Shortcut for input.video; accepts URL or @local-file")
	cmd.Flags().StringVar(&opts.endImage, "end-image", "", "Shortcut for input.end_image; accepts URL or @local-file")
	cmd.Flags().StringVar(&opts.lastImage, "last-image", "", "Shortcut for input.last_image; accepts URL or @local-file")
	cmd.Flags().StringArrayVar(&opts.refImages, "reference-image", nil, "Shortcut for input.reference_images array; repeat for multiple URLs or @local-files")
	cmd.Flags().StringArrayVar(&opts.refVideos, "reference-video", nil, "Shortcut for input.reference_videos array; repeat for multiple URLs or @local-files")
	cmd.Flags().BoolVar(&opts.syncMode, "sync", false, "Set input.enable_sync_mode=true for models that support synchronous responses")
}

func runChangedFlags(cmd *cobra.Command) map[string]bool {
	changed := map[string]bool{}
	for _, name := range []string{"input", "input-file", "prompt", "image", "images", "video", "end-image", "last-image", "reference-image", "reference-video", "sync"} {
		changed[name] = cmd.Flags().Changed(name)
	}
	return changed
}

func readRunInputs(opts runCommandOptions, defaults map[string]any, changed map[string]bool) (map[string]any, error) {
	inputs := cloneMap(defaults)
	if inputs == nil {
		inputs = map[string]any{}
	}
	if changed["input-file"] {
		fileInputs, err := readRunInputJSON(opts.inputFile, "input file")
		if err != nil {
			return nil, err
		}
		mergeMap(inputs, fileInputs)
	}
	if changed["input"] {
		inlineInputs, err := parseRunInputJSONObject([]byte(opts.input), "--input JSON")
		if err != nil {
			return nil, err
		}
		mergeMap(inputs, inlineInputs)
	}
	for _, item := range append(append([]string{}, opts.inputKV...), opts.setKV...) {
		key, value, err := parseInputKV(item)
		if err != nil {
			return nil, err
		}
		inputs[key] = value
	}
	if changed["prompt"] {
		inputs["prompt"] = opts.prompt
	}
	if changed["image"] {
		inputs["image"] = opts.image
	}
	if changed["images"] {
		inputs["images"] = stringListValue(opts.images)
	}
	if changed["video"] {
		inputs["video"] = opts.video
	}
	if changed["end-image"] {
		inputs["end_image"] = opts.endImage
	}
	if changed["last-image"] {
		inputs["last_image"] = opts.lastImage
	}
	if changed["reference-image"] {
		inputs["reference_images"] = stringListValue(opts.refImages)
	}
	if changed["reference-video"] {
		inputs["reference_videos"] = stringListValue(opts.refVideos)
	}
	if changed["sync"] {
		inputs["enable_sync_mode"] = opts.syncMode
	}
	return inputs, nil
}

func readRunInputJSON(path, label string) (map[string]any, error) {
	if strings.TrimSpace(path) == "" {
		return map[string]any{}, nil
	}
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", label, err)
	}
	return parseRunInputJSONObject(raw, label)
}

func parseRunInputJSONObject(raw []byte, label string) (map[string]any, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var inputs map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&inputs); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", label, err)
	}
	return inputs, nil
}

func parseInputKV(raw string) (string, any, error) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return "", nil, fmt.Errorf("input-kv must be key=value, got %q", raw)
	}
	key = strings.TrimSpace(key)
	var parsed any
	dec := json.NewDecoder(strings.NewReader(value))
	dec.UseNumber()
	if err := dec.Decode(&parsed); err == nil && dec.Decode(&struct{}{}) == io.EOF {
		return key, normalizeJSONNumbers(parsed), nil
	}
	if strings.Contains(value, ",") && strings.Contains(value, "@") {
		return key, stringListValue(strings.Split(value, ",")), nil
	}
	return key, value, nil
}

func stringListValue(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeJSONNumbers(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return i
		}
		if f, err := typed.Float64(); err == nil {
			return f
		}
		return typed.String()
	case []any:
		for i, item := range typed {
			typed[i] = normalizeJSONNumbers(item)
		}
		return typed
	case map[string]any:
		for k, item := range typed {
			typed[k] = normalizeJSONNumbers(item)
		}
		return typed
	default:
		return value
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch typed := v.(type) {
		case map[string]any:
			out[k] = cloneMap(typed)
		default:
			out[k] = typed
		}
	}
	return out
}

func mergeMap(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func resolveProjectModel(project wavespeedProjectConfig, token string) (string, map[string]any, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", nil, fmt.Errorf("model-or-alias is required unless wavespeed.json sets defaultModel")
	}
	if strings.Contains(token, "/") {
		return strings.Trim(token, "/"), nil, nil
	}
	alias, ok := project.Aliases[token]
	if !ok {
		return "", nil, fmt.Errorf("%q is not a slash-delimited model ID and is not an alias in wavespeed.json", token)
	}
	modelID := strings.TrimSpace(alias.Model)
	if modelID == "" {
		return "", nil, fmt.Errorf("alias %q does not set model", token)
	}
	return strings.Trim(modelID, "/"), alias.Input, nil
}

func loadWavespeedProjectConfig() (wavespeedProjectConfig, error) {
	cfgPath, err := findWavespeedProjectConfig("")
	if err != nil {
		return wavespeedProjectConfig{}, err
	}
	if cfgPath == "" {
		return wavespeedProjectConfig{}, nil
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return wavespeedProjectConfig{}, fmt.Errorf("reading %s: %w", cfgPath, err)
	}
	var cfg wavespeedProjectConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return wavespeedProjectConfig{}, fmt.Errorf("parsing %s: %w", cfgPath, err)
	}
	cfg.Path = cfgPath
	if cfg.Aliases == nil {
		cfg.Aliases = map[string]wavespeedProjectAlias{}
	}
	return cfg, nil
}

func findWavespeedProjectConfig(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "wavespeed.json")
		info, err := os.Stat(candidate)
		if err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("%s is a directory, expected a JSON file", candidate)
			}
			return candidate, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func runDownloadSpec(opts runCommandOptions, project wavespeedProjectConfig, downloadDirChanged bool) string {
	if opts.download != "" && opts.download != "true" {
		return opts.download
	}
	if downloadDirChanged {
		return opts.downloadDir
	}
	if strings.TrimSpace(project.OutputDir) != "" {
		return project.OutputDir
	}
	return opts.downloadDir
}

func printModelSchema(cmd *cobra.Command, flags *rootFlags, c *client.Client, modelID string) error {
	models, err := c.Get(cmd.Context(), "/models", nil)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	schema, err := requestSchemaForModel(models, modelID)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), schema, flags)
}

func printModelHelp(cmd *cobra.Command, flags *rootFlags, c *client.Client, modelID string) error {
	models, err := c.Get(cmd.Context(), "/models", nil)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	model, ok := findModelObject(models, modelID)
	if !ok {
		return fmt.Errorf("model %q not found in /models response", modelID)
	}
	if flags.asJSON {
		raw, err := json.MarshalIndent(modelHelpSummary(modelID, model), "", "  ")
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), raw, true)
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), modelHelpText(modelID, model))
	return err
}

func requestSchemaForModel(models json.RawMessage, modelID string) (json.RawMessage, error) {
	model, ok := findModelObject(models, modelID)
	if !ok {
		return nil, fmt.Errorf("model %q not found in /models response", modelID)
	}
	schema, ok := nestedValue(model, "api_schema", "api_schemas", "0", "request_schema")
	if !ok {
		return nil, fmt.Errorf("model %q does not include api_schema.api_schemas[0].request_schema", modelID)
	}
	raw, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

type modelHelpField struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Description string   `json:"description,omitempty"`
}

type modelHelpInfo struct {
	ModelID           string              `json:"model_id"`
	Name              string              `json:"name,omitempty"`
	Type              string              `json:"type,omitempty"`
	Price             string              `json:"price"`
	BasePrice         any                 `json:"base_price,omitempty"`
	Formula           string              `json:"formula,omitempty"`
	ResolutionOptions map[string][]string `json:"resolution_options,omitempty"`
	Fields            []modelHelpField    `json:"fields"`
}

func modelHelpSummary(modelID string, model map[string]any) modelHelpInfo {
	schema, _ := schemaFromModelObject(model)
	fields := schemaFields(schema)
	info := modelHelpInfo{
		ModelID:   modelID,
		Name:      stringFromAny(model["name"]),
		Type:      stringFromAny(model["type"]),
		Price:     modelPriceText(model),
		BasePrice: model["base_price"],
		Formula:   stringFromAny(model["formula"]),
		Fields:    fields,
	}
	resolution := map[string][]string{}
	for _, f := range fields {
		key := strings.ToLower(f.Name)
		if (strings.Contains(key, "resolution") || strings.Contains(key, "size")) && len(f.Enum) > 0 {
			resolution[f.Name] = f.Enum
		}
	}
	if len(resolution) > 0 {
		info.ResolutionOptions = resolution
	}
	return info
}

func modelHelpText(modelID string, model map[string]any) string {
	info := modelHelpSummary(modelID, model)
	var b strings.Builder
	fmt.Fprintf(&b, "Model Inputs for %s:\n", modelID)
	if info.Name != "" || info.Type != "" {
		fmt.Fprintf(&b, "  model: %s", info.Name)
		if info.Type != "" {
			fmt.Fprintf(&b, " (%s)", info.Type)
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "  price: %s", info.Price)
	if info.Formula != "" {
		fmt.Fprintf(&b, " (%s)", info.Formula)
	}
	b.WriteString("\n")
	if len(info.ResolutionOptions) > 0 {
		b.WriteString("  resolution/size options (affect cost when the formula references them):\n")
		keys := make([]string, 0, len(info.ResolutionOptions))
		for k := range info.ResolutionOptions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "    - %s: %s\n", k, strings.Join(info.ResolutionOptions[k], " | "))
		}
	}
	for _, f := range info.Fields {
		req := ""
		if f.Required {
			req = " required"
		}
		fmt.Fprintf(&b, "  - %s (%s%s)", f.Name, f.Type, req)
		if f.Default != nil {
			fmt.Fprintf(&b, " default=%v", f.Default)
		}
		if len(f.Enum) > 0 {
			fmt.Fprintf(&b, " enum=%s", strings.Join(f.Enum, "|"))
		}
		if f.Description != "" {
			fmt.Fprintf(&b, " - %s", f.Description)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func schemaFromModelObject(model map[string]any) (json.RawMessage, bool) {
	schema, ok := nestedValue(model, "api_schema", "api_schemas", "0", "request_schema")
	if !ok {
		return nil, false
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(raw), true
}

func schemaFields(schema json.RawMessage) []modelHelpField {
	var root map[string]any
	if len(schema) == 0 || json.Unmarshal(schema, &root) != nil {
		return nil
	}
	props, _ := root["properties"].(map[string]any)
	required := map[string]bool{}
	if items, ok := root["required"].([]any); ok {
		for _, item := range items {
			if s, ok := item.(string); ok {
				required[s] = true
			}
		}
	}
	names := orderedSchemaPropertyNames(root, props)
	fields := make([]modelHelpField, 0, len(names))
	for _, name := range names {
		prop, _ := props[name].(map[string]any)
		field := modelHelpField{Name: name, Type: schemaTypeName(prop), Required: required[name], Description: oneline(stringFromAny(prop["description"]))}
		if def, ok := prop["default"]; ok {
			field.Default = def
		}
		if enum, ok := prop["enum"].([]any); ok {
			for _, item := range enum {
				field.Enum = append(field.Enum, fmt.Sprintf("%v", item))
			}
		}
		fields = append(fields, field)
	}
	return fields
}

func modelPriceText(model map[string]any) string {
	for _, key := range []string{"unit_price", "price", "base_price", "cost"} {
		if v, ok := model[key]; ok {
			if s := stringFromAny(v); s != "" && s != "0" {
				return s
			}
			if n, ok := numberFromAny(v); ok {
				return strconv.FormatFloat(n, 'f', -1, 64)
			}
		}
	}
	if f := stringFromAny(model["formula"]); f != "" {
		return f
	}
	return "?"
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}

func numberFromAny(v any) (float64, bool) {
	switch t := v.(type) {
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case float64:
		return t, true
	case int:
		return float64(t), true
	default:
		return 0, false
	}
}

func schemaHelpText(schema json.RawMessage) string {
	var root map[string]any
	if err := json.Unmarshal(schema, &root); err != nil {
		return "  (schema is not a JSON object)\n"
	}
	props, _ := root["properties"].(map[string]any)
	if len(props) == 0 {
		return "  (no request properties advertised)\n"
	}
	required := map[string]bool{}
	if items, ok := root["required"].([]any); ok {
		for _, item := range items {
			if s, ok := item.(string); ok {
				required[s] = true
			}
		}
	}
	names := orderedSchemaPropertyNames(root, props)
	var b strings.Builder
	for _, name := range names {
		prop, _ := props[name].(map[string]any)
		typeName := schemaTypeName(prop)
		req := ""
		if required[name] {
			req = " required"
		}
		fmt.Fprintf(&b, "  - %s (%s%s)", name, typeName, req)
		if def, ok := prop["default"]; ok {
			fmt.Fprintf(&b, " default=%v", def)
		}
		if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
			parts := make([]string, 0, len(enum))
			for _, item := range enum {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
			fmt.Fprintf(&b, " enum=%s", strings.Join(parts, "|"))
		}
		if desc, ok := prop["description"].(string); ok && strings.TrimSpace(desc) != "" {
			fmt.Fprintf(&b, " - %s", oneline(desc))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func orderedSchemaPropertyNames(root map[string]any, props map[string]any) []string {
	seen := map[string]bool{}
	var names []string
	for _, key := range []string{"x-order-properties", "x_order_properties", "order"} {
		if raw, ok := root[key].([]any); ok {
			for _, item := range raw {
				if s, ok := item.(string); ok {
					if _, exists := props[s]; exists && !seen[s] {
						seen[s] = true
						names = append(names, s)
					}
				}
			}
		}
	}
	var rest []string
	for name := range props {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	names = append(names, rest...)
	return names
}

func schemaTypeName(prop map[string]any) string {
	if prop == nil {
		return "any"
	}
	if t, ok := prop["type"].(string); ok && t != "" {
		if t == "array" {
			if items, ok := prop["items"].(map[string]any); ok {
				return "[]" + schemaTypeName(items)
			}
		}
		return t
	}
	if anyOf, ok := prop["anyOf"].([]any); ok && len(anyOf) > 0 {
		parts := make([]string, 0, len(anyOf))
		for _, item := range anyOf {
			if obj, ok := item.(map[string]any); ok {
				parts = append(parts, schemaTypeName(obj))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "|")
		}
	}
	return "any"
}

func oneline(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func findModelObject(models json.RawMessage, modelID string) (map[string]any, bool) {
	for _, item := range modelItems(models) {
		for _, key := range []string{"model_id", "id", "name", "path", "model"} {
			if value, ok := item[key].(string); ok && value == modelID {
				return item, true
			}
		}
	}
	return nil, false
}

func modelItems(models json.RawMessage) []map[string]any {
	var root any
	dec := json.NewDecoder(strings.NewReader(string(unwrapWaveSpeedData(models))))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return nil
	}
	var rawItems []any
	switch typed := root.(type) {
	case []any:
		rawItems = typed
	case map[string]any:
		for _, key := range []string{"data", "items", "models", "results"} {
			if items, ok := typed[key].([]any); ok {
				rawItems = items
				break
			}
		}
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, item := range rawItems {
		if obj, ok := item.(map[string]any); ok {
			items = append(items, obj)
		}
	}
	return items
}

type modelCatalogSummary struct {
	ModelID     string              `json:"model_id"`
	Name        string              `json:"name,omitempty"`
	Type        string              `json:"type,omitempty"`
	Price       string              `json:"price"`
	BasePrice   any                 `json:"base_price,omitempty"`
	Formula     string              `json:"formula,omitempty"`
	Resolutions map[string][]string `json:"resolutions,omitempty"`
}

func summarizeModelsForCapability(data json.RawMessage, capability string) (json.RawMessage, error) {
	items := modelItems(data)
	out := make([]modelCatalogSummary, 0, len(items))
	for _, model := range items {
		if !modelMatchesCapability(model, capability) {
			continue
		}
		id := modelIDFromObject(model)
		if id == "" {
			continue
		}
		info := modelHelpSummary(id, model)
		out = append(out, modelCatalogSummary{
			ModelID:     id,
			Name:        info.Name,
			Type:        info.Type,
			Price:       info.Price,
			BasePrice:   info.BasePrice,
			Formula:     info.Formula,
			Resolutions: info.ResolutionOptions,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, iok := numericSummaryPrice(out[i])
		pj, jok := numericSummaryPrice(out[j])
		if iok && jok && pi != pj {
			return pi < pj
		}
		if iok != jok {
			return iok
		}
		return out[i].ModelID < out[j].ModelID
	})
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func modelIDFromObject(model map[string]any) string {
	for _, key := range []string{"model_id", "id", "path", "model", "name"} {
		if s := stringFromAny(model[key]); s != "" {
			return s
		}
	}
	return ""
}

func numericSummaryPrice(s modelCatalogSummary) (float64, bool) {
	switch v := s.BasePrice.(type) {
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	if s.Price != "" && s.Price != "?" {
		f, err := strconv.ParseFloat(strings.TrimPrefix(s.Price, "$"), 64)
		return f, err == nil
	}
	return 0, false
}

func modelMatchesCapability(model map[string]any, capability string) bool {
	capability = strings.ToLower(strings.TrimSpace(capability))
	if capability == "" {
		return true
	}
	textParts := []string{}
	for _, key := range []string{"model_id", "id", "name", "description", "type", "category"} {
		if s := stringFromAny(model[key]); s != "" {
			textParts = append(textParts, strings.ToLower(s))
		}
	}
	text := strings.Join(textParts, " ")
	schema, _ := schemaFromModelObject(model)
	fields := schemaFields(schema)
	fieldNames := map[string]bool{}
	for _, f := range fields {
		fieldNames[strings.ToLower(f.Name)] = true
	}
	switch capability {
	case "image-edit", "image_edit", "edit", "image-to-image", "i2i":
		if strings.Contains(text, "image-to-image") || strings.Contains(text, "image edit") || strings.Contains(text, "edit") {
			return true
		}
		return fieldNames["image"] || fieldNames["images"] || fieldNames["reference_images"] || fieldNames["reference_image"]
	case "text-to-image", "t2i", "image-generate", "image-generation":
		return strings.Contains(text, "text-to-image") || strings.Contains(text, "image") && fieldNames["prompt"]
	case "video", "text-to-video", "t2v":
		return strings.Contains(text, "video") || fieldNames["video"]
	default:
		needle := strings.ReplaceAll(capability, "-", " ")
		return strings.Contains(text, capability) || strings.Contains(text, needle)
	}
}

func filterModelsForCLI(data json.RawMessage, query, modelType, category string, popular bool) (json.RawMessage, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	filterType := strings.ToLower(strings.TrimSpace(modelType))
	if filterType == "" {
		filterType = strings.ToLower(strings.TrimSpace(category))
	}
	if query == "" && filterType == "" && !popular {
		return data, nil
	}
	var root any
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	items, setItems := extractMutableModelItems(root)
	if items == nil {
		return data, nil
	}
	filtered := make([]any, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if query != "" && !modelMatchesQuery(obj, query) {
			continue
		}
		if filterType != "" && !modelMatchesType(obj, filterType) {
			continue
		}
		filtered = append(filtered, item)
	}
	if popular {
		sort.SliceStable(filtered, func(i, j int) bool {
			return modelSortOrder(filtered[i]) < modelSortOrder(filtered[j])
		})
	}
	root = setItems(filtered)
	raw, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func extractMutableModelItems(root any) ([]any, func([]any) any) {
	switch typed := root.(type) {
	case []any:
		return typed, func(items []any) any { return items }
	case map[string]any:
		for _, key := range []string{"data", "items", "models", "results"} {
			if items, ok := typed[key].([]any); ok {
				return items, func(next []any) any {
					typed[key] = next
					return typed
				}
			}
		}
	}
	return nil, func([]any) any { return root }
}

func modelMatchesQuery(model map[string]any, query string) bool {
	for _, key := range []string{"model_id", "name", "description", "type"} {
		if value, ok := model[key].(string); ok && strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func modelMatchesType(model map[string]any, filterType string) bool {
	value, _ := model["type"].(string)
	value = strings.ToLower(strings.TrimSpace(value))
	return value == filterType || strings.Contains(value, filterType)
}

func modelSortOrder(item any) float64 {
	obj, _ := item.(map[string]any)
	switch v := obj["sort_order"].(type) {
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 1e12
}

func nestedValue(root any, path ...string) (any, bool) {
	cur := root
	for _, part := range path {
		switch typed := cur.(type) {
		case map[string]any:
			next, ok := typed[part]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(typed) {
				return nil, false
			}
			cur = typed[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

func modelRunPath(modelID string) string {
	trimmed := strings.Trim(strings.TrimSpace(modelID), "/")
	trimmed = strings.TrimPrefix(trimmed, "api/v3/")
	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return "/" + strings.Join(parts, "/")
}

// submitRequest is the full input to one generation: optional price estimate,
// model + resolved inputs, optional wait-to-terminal, optional download.
type submitRequest struct {
	modelID       string
	inputs        map[string]any
	estimatePrice bool
	// priceBestEffort makes a pricing-endpoint failure non-fatal: the
	// generation proceeds with no pricing rather than aborting. Producers
	// (pack/batch/variants/compose/refine) set this because pricing is only
	// for cost tracking; `run --price` leaves it false so an explicit price
	// request still surfaces the error.
	priceBestEffort bool
	wait            bool
	waitTimeout     time.Duration
	pollInitial     time.Duration
	download        bool
	downloadSpec    string
}

// submitResult is the structured outcome of submitAndAwait. It prints nothing
// itself; callers (run's RunE and every novel command) decide how to render or
// record it.
type submitResult struct {
	Pricing   json.RawMessage
	Result    json.RawMessage
	Downloads []downloadedFile
	Status    string
	// Failed reports a prediction that reached a failed terminal status. It is
	// NOT a transport error: the request succeeded, the model reported failure.
	// Callers can record the attempt to the library before surfacing it.
	Failed bool
}

// submitAndAwait runs the generation chain end-to-end and returns structured
// data WITHOUT printing. Transport/API errors are returned raw for the caller
// to classify via classifyAPIError; a failed prediction is reported through
// submitResult.Failed so the attempt can still be recorded. SQLite/library
// recording is deliberately not done here — that is the caller's concern, so a
// record failure can never abort a successful generation.
func submitAndAwait(ctx context.Context, c *client.Client, req submitRequest) (submitResult, error) {
	var res submitResult

	if req.estimatePrice {
		pricing, _, err := c.PostQueryWithParams(ctx, "/model/pricing", nil, map[string]any{
			"model_id": req.modelID,
			"inputs":   req.inputs,
		})
		switch {
		case err == nil:
			res.Pricing = pricing
		case req.priceBestEffort:
			// Pricing is advisory for cost tracking; a transient failure must
			// not abort the generation. Proceed with no pricing.
		default:
			return res, err
		}
	}

	result, _, err := c.PostWithParams(ctx, modelRunPath(req.modelID), nil, req.inputs)
	if err != nil {
		return res, err
	}
	res.Result = result

	if req.wait {
		taskID := extractPredictionID(result)
		if taskID == "" {
			return res, fmt.Errorf("run response did not include a prediction id")
		}
		result, err = waitForPrediction(ctx, c, taskID, req.waitTimeout, req.pollInitial)
		if err != nil {
			return res, err
		}
		res.Result = result
	}

	if req.download {
		downloads, err := downloadRunOutputs(ctx, c, unwrapWaveSpeedData(result), req.downloadSpec)
		if err != nil {
			return res, err
		}
		res.Downloads = downloads
	}

	res.Status = extractPredictionStatus(res.Result)
	res.Failed = isFailedPredictionStatus(res.Status)
	return res, nil
}

func waitForPrediction(ctx context.Context, c *client.Client, taskID string, timeout, initialInterval time.Duration) (json.RawMessage, error) {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if initialInterval <= 0 {
		initialInterval = 2 * time.Second
	}
	deadline := time.Now().Add(timeout)
	interval := initialInterval
	pollPath := "/predictions/" + url.PathEscape(taskID) + "/result"

	for {
		data, err := c.GetNoCache(ctx, pollPath, nil)
		if err != nil {
			return nil, err
		}
		status := extractPredictionStatus(data)
		if isTerminalPredictionStatus(status) {
			return data, nil
		}
		if time.Now().Add(interval).After(deadline) {
			return data, fmt.Errorf("timed out waiting for prediction %s; last status %q", taskID, status)
		}
		select {
		case <-ctx.Done():
			return data, ctx.Err()
		case <-time.After(interval):
		}
		interval = minDuration(10*time.Second, interval+interval/2)
	}
}

func extractPredictionID(data json.RawMessage) string {
	obj := decodeObject(unwrapWaveSpeedData(data))
	for _, key := range []string{"id", "task_id", "taskId", "prediction_id"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func extractPredictionStatus(data json.RawMessage) string {
	obj := decodeObject(unwrapWaveSpeedData(data))
	if value, ok := obj["status"].(string); ok {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
}

func unwrapWaveSpeedData(data json.RawMessage) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		if raw, ok := obj["data"]; ok && len(raw) > 0 && string(raw) != "null" {
			return raw
		}
	}
	return data
}

func decodeObject(data json.RawMessage) map[string]any {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return map[string]any{}
	}
	return obj
}

func isTerminalPredictionStatus(status string) bool {
	switch strings.ToLower(status) {
	case "completed", "complete", "succeeded", "success", "failed", "error", "canceled", "cancelled":
		return true
	default:
		return false
	}
}

func isFailedPredictionStatus(status string) bool {
	switch strings.ToLower(status) {
	case "failed", "error", "canceled", "cancelled":
		return true
	default:
		return false
	}
}

func runOutputEnvelope(pricing, result json.RawMessage, downloads []downloadedFile) json.RawMessage {
	if len(pricing) == 0 && len(downloads) == 0 {
		return result
	}
	payload := map[string]any{
		"prediction": json.RawMessage(result),
	}
	if len(pricing) > 0 {
		payload["pricing"] = json.RawMessage(pricing)
	}
	if len(downloads) > 0 {
		payload["downloads"] = downloads
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return result
	}
	return raw
}

type downloadedFile struct {
	URL  string `json:"url"`
	Path string `json:"path"`
}

func downloadRunOutputs(ctx context.Context, c *client.Client, data json.RawMessage, spec string) ([]downloadedFile, error) {
	return downloadPlannedRunOutputs(ctx, c, planRunDownloads(data, spec))
}

func downloadPlannedRunOutputs(ctx context.Context, c *client.Client, planned []downloadedFile) ([]downloadedFile, error) {
	downloads := make([]downloadedFile, 0, len(planned))
	for _, item := range planned {
		rawURL := item.URL
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return downloads, fmt.Errorf("building download request: %w", err)
		}
		if err := addDownloadRequestHeaders(ctx, c, req); err != nil {
			return downloads, err
		}
		resp, err := c.DoRaw(req)
		if err != nil {
			return downloads, fmt.Errorf("downloading %s: %w", rawURL, err)
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				err = fmt.Errorf("downloading %s returned HTTP %d", rawURL, resp.StatusCode)
				return
			}
			outPath := item.Path
			if err = os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				err = fmt.Errorf("creating download dir: %w", err)
				return
			}
			var out *os.File
			out, err = os.Create(outPath)
			if err != nil {
				err = fmt.Errorf("creating %s: %w", outPath, err)
				return
			}
			defer out.Close()
			if _, err = io.Copy(out, resp.Body); err != nil {
				err = fmt.Errorf("writing %s: %w", outPath, err)
				return
			}
			downloads = append(downloads, item)
		}()
		if err != nil {
			return downloads, err
		}
	}
	return downloads, nil
}

func planRunDownloads(data json.RawMessage, spec string) []downloadedFile {
	urls := collectURLStrings(data)
	if len(urls) == 0 {
		return nil
	}
	if spec == "" || spec == "true" {
		spec = "."
	}
	downloads := make([]downloadedFile, 0, len(urls))
	for i, rawURL := range urls {
		downloads = append(downloads, downloadedFile{
			URL:  rawURL,
			Path: downloadOutputPath(spec, rawURL, i, len(urls)),
		})
	}
	return downloads
}

func addDownloadRequestHeaders(ctx context.Context, c *client.Client, req *http.Request) error {
	if c == nil || c.Config == nil || !sameHost(req.URL, c.BaseURL) {
		return nil
	}
	auth, err := c.AuthHeader(ctx)
	if err != nil {
		return err
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	for k, v := range c.Config.Headers {
		req.Header.Set(k, v)
	}
	return nil
}

func sameHost(target *url.URL, baseURL string) bool {
	if target == nil {
		return false
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(target.Host, base.Host)
}

func downloadOutputPath(spec, rawURL string, index, total int) string {
	if spec == "" || spec == "true" {
		spec = "."
	}
	if strings.Contains(spec, "{") {
		name := outputFilename(rawURL, 0)
		ext := strings.TrimPrefix(filepath.Ext(name), ".")
		if ext == "" {
			ext = "bin"
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		replacer := strings.NewReplacer(
			"{index}", strconv.Itoa(index+1),
			"{zero_index}", strconv.Itoa(index),
			"{ext}", ext,
			"{name}", name,
			"{base}", base,
			"{basename}", base,
		)
		return filepath.Clean(replacer.Replace(spec))
	}
	if isDownloadDirSpec(spec) {
		return filepath.Join(spec, outputFilename(rawURL, index))
	}
	if total <= 1 || index == 0 {
		return filepath.Clean(spec)
	}
	ext := filepath.Ext(spec)
	base := strings.TrimSuffix(spec, ext)
	return filepath.Clean(fmt.Sprintf("%s-%d%s", base, index+1, ext))
}

func isDownloadDirSpec(spec string) bool {
	if spec == "." || spec == "" {
		return true
	}
	if strings.HasSuffix(spec, string(os.PathSeparator)) || strings.HasSuffix(spec, "/") {
		return true
	}
	info, err := os.Stat(spec)
	return err == nil && info.IsDir()
}

func collectURLStrings(data json.RawMessage) []string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var urls []string
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case string:
			if (strings.HasPrefix(typed, "https://") || strings.HasPrefix(typed, "http://")) && !seen[typed] {
				seen[typed] = true
				urls = append(urls, typed)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				item := typed[key]
				if isEchoedInputContainerKey(key) {
					continue
				}
				if isPredictionManagementURL(key, item) {
					continue
				}
				walk(item)
			}
		}
	}
	walk(value)
	return urls
}

func isEchoedInputContainerKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "input", "inputs":
		return true
	default:
		return false
	}
}

func isPredictionManagementURL(key string, item any) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "get", "self", "status", "result", "cancel", "delete":
	default:
		return false
	}
	rawURL, ok := item.(string)
	if !ok || !strings.HasPrefix(rawURL, "http") {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := strings.ToLower(parsed.Path)
	return strings.Contains(path, "/predictions/")
}

func outputFilename(rawURL string, index int) string {
	parsed, err := url.Parse(rawURL)
	name := ""
	if err == nil {
		name = path.Base(parsed.Path)
	}
	if name == "" || name == "." || name == "/" {
		sum := sha1.Sum([]byte(rawURL))
		name = "output-" + hex.EncodeToString(sum[:6])
	}
	if index > 0 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		name = fmt.Sprintf("%s-%d%s", base, index+1, ext)
	}
	return filepath.Base(name)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
