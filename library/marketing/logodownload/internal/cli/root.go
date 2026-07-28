package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/logodownload/internal/logodownload"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

type options struct {
	downloadSpec string
	outputDir    string
	preview      bool
	previewH     int
	previewW     int
	previewLimit int
	limit        int
	timeout      time.Duration
	asJSON       bool
}

// NewRootCommand builds the command tree used by both humans and Printing Press
// validation. The root command keeps the original "cli <term>" behavior.
func NewRootCommand() *cobra.Command {
	log.SetOutput(os.Stderr)

	opts := &options{}
	rootCmd := &cobra.Command{
		Use:           "logodownload-pp-cli [flags] <query>",
		Short:         "Search public logo entries on logodownload.org",
		Example:       "  logodownload-pp-cli nike\n  logodownload-pp-cli \"Bradesco Logo\" --preview\n  logodownload-pp-cli \"Banco Inter\" --download first --output-dir ./logos",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), opts, strings.Join(args, " "))
		},
	}

	addSearchFlags(rootCmd, opts)
	rootCmd.AddCommand(newSearchCmd(opts))
	rootCmd.AddCommand(newAgentContextCmd(rootCmd))
	return rootCmd
}

func newSearchCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "search [flags] <query>",
		Short:   "Search logo results by company or brand term",
		Example: "  logodownload-pp-cli search nike\n  logodownload-pp-cli search \"Bradesco Logo\" --preview\n  logodownload-pp-cli search \"Banco Inter\" --download first --output-dir ./logos",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<query>=nike",
		},
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), opts, strings.Join(args, " "))
		},
	}
	addSearchFlags(cmd, opts)
	return cmd
}

func addSearchFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().StringVar(&opts.downloadSpec, "download", "", "download image_url selection: first, all, or 1-based result index")
	cmd.Flags().StringVar(&opts.outputDir, "output-dir", ".", "directory used by --download")
	cmd.Flags().BoolVar(&opts.preview, "preview", false, "print a monochrome horizontal preview of returned image_url values to stderr")
	cmd.Flags().IntVar(&opts.previewH, "preview-h", 12, "height in terminal rows used by --preview")
	cmd.Flags().IntVar(&opts.previewW, "preview-w", 28, "width in terminal columns per logo used by --preview")
	cmd.Flags().IntVar(&opts.previewLimit, "preview-limit", 5, "maximum result previews printed by --preview")
	cmd.Flags().IntVar(&opts.limit, "limit", 10, "maximum WordPress API fallback results")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 20*time.Second, "HTTP timeout")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print results as JSON")
}

func runSearch(ctx context.Context, opts *options, query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("termo de busca obrigatório")
	}

	client := &http.Client{Timeout: opts.timeout}
	results, err := logodownload.Search(ctx, client, query, opts.limit)
	if err != nil {
		return fmt.Errorf("erro na busca: %w", err)
	}

	if opts.downloadSpec != "" {
		selection, err := parseSelection(opts.downloadSpec, len(results))
		if err != nil {
			return fmt.Errorf("parâmetro --download inválido: %w", err)
		}
		if err := logodownload.DownloadImages(ctx, client, results, selection, opts.outputDir); err != nil {
			return fmt.Errorf("erro no download: %w", err)
		}
	}

	if opts.preview {
		rendered := logodownload.RenderTerminalPreview(ctx, client, results, logodownload.TerminalPreviewOptions{
			Height: opts.previewH,
			Width:  opts.previewW,
			Limit:  opts.previewLimit,
		})
		if rendered != "" {
			fmt.Fprintln(os.Stderr, rendered)
		}
	}

	return printJSON(results)
}

func parseSelection(value string, resultCount int) (logodownload.Selection, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "first":
		return logodownload.Selection{Mode: logodownload.SelectFirst}, nil
	case "all":
		return logodownload.Selection{Mode: logodownload.SelectAll}, nil
	}

	index, err := strconv.Atoi(value)
	if err != nil || index < 1 {
		return logodownload.Selection{}, fmt.Errorf("use first, all, or a 1-based index")
	}
	if resultCount > 0 && index > resultCount {
		return logodownload.Selection{}, fmt.Errorf("índice %d fora do intervalo de resultados", index)
	}

	return logodownload.Selection{Mode: logodownload.SelectIndex, Index: index}, nil
}

func printJSON(results []logodownload.LogoResult) error {
	if results == nil {
		results = []logodownload.LogoResult{}
	}

	jsonOutput, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao formatar JSON: %w", err)
	}

	fmt.Println(string(jsonOutput))
	return nil
}
