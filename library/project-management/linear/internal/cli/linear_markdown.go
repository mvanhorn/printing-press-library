package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"

	"github.com/spf13/cobra"
)

type markdownBodySpec struct {
	InlineFlag string
	Inline     string
	FileFlag   string
	File       string
	StdinFlag  string
	Stdin      bool
	Label      string
}

func readMarkdownBody(cmd *cobra.Command, spec markdownBodySpec) (string, bool, error) {
	sources := 0
	inlineSet := spec.InlineFlag != "" && cmd.Flags().Changed(spec.InlineFlag)
	fileSet := spec.FileFlag != "" && cmd.Flags().Changed(spec.FileFlag)
	stdinSet := spec.StdinFlag != "" && spec.Stdin
	if inlineSet {
		sources++
	}
	if fileSet {
		sources++
	}
	if stdinSet {
		sources++
	}
	if sources > 1 {
		return "", false, fmt.Errorf("pass only one %s source: --%s, --%s, or --%s", spec.Label, spec.InlineFlag, spec.FileFlag, spec.StdinFlag)
	}
	switch {
	case inlineSet:
		return spec.Inline, true, nil
	case fileSet:
		data, err := os.ReadFile(spec.File)
		if err != nil {
			return "", false, fmt.Errorf("reading --%s: %w", spec.FileFlag, err)
		}
		if strings.TrimSpace(string(data)) == "" {
			return "", false, usageErr(fmt.Errorf("%s from --%s is empty", spec.Label, spec.FileFlag))
		}
		return string(data), true, nil
	case stdinSet:
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", false, fmt.Errorf("reading --%s: %w", spec.StdinFlag, err)
		}
		if strings.TrimSpace(string(data)) == "" {
			return "", false, usageErr(fmt.Errorf("%s from --%s is empty", spec.Label, spec.StdinFlag))
		}
		return string(data), true, nil
	default:
		return "", false, nil
	}
}

func uploadMediaAndAppend(c *client.Client, body string, media []string, makePublic bool) (string, []client.UploadedFile, error) {
	if len(media) == 0 {
		return body, nil, nil
	}
	uploaded := make([]client.UploadedFile, 0, len(media))
	for _, path := range media {
		file, err := c.UploadFileFromPath(path, makePublic)
		if err != nil {
			return "", uploaded, fmt.Errorf("uploading %s: %w", path, err)
		}
		uploaded = append(uploaded, file)
		body = appendMediaMarkdown(body, file)
	}
	return body, uploaded, nil
}

func appendMediaMarkdown(body string, file client.UploadedFile) string {
	if strings.TrimSpace(body) != "" {
		body += "\n\n"
	}
	name := file.Filename
	if name == "" {
		name = filepath.Base(file.AssetURL)
	}
	if strings.HasPrefix(file.ContentType, "image/") {
		return body + fmt.Sprintf("![%s](%s)", name, file.AssetURL)
	}
	return body + fmt.Sprintf("[%s](%s)", name, file.AssetURL)
}

func mediaUploadFailure(err error, uploaded []client.UploadedFile) error {
	if err == nil || len(uploaded) == 0 {
		return err
	}
	urls := make([]string, 0, len(uploaded))
	for _, file := range uploaded {
		if file.AssetURL != "" {
			urls = append(urls, file.AssetURL)
		}
	}
	if len(urls) == 0 {
		return err
	}
	return fmt.Errorf("%w (uploaded before failure: %s)", err, strings.Join(urls, ", "))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func renderLiveObject(cmd *cobra.Command, flags *rootFlags, data json.RawMessage, resourceType string) error {
	prov := DataProvenance{Source: "live", ResourceType: resourceType, Reason: "user_requested"}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		} else if flags.compact {
			data = compactFields(data)
		}
		wrapped, err := wrapWithProvenance(data, prov)
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func renderLivePayload(cmd *cobra.Command, flags *rootFlags, data json.RawMessage, resourceType string, compact bool) error {
	prov := DataProvenance{Source: "live", ResourceType: resourceType, Reason: "user_requested"}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		} else if compact && flags.compact {
			data = compactFields(data)
		}
		wrapped, err := wrapWithProvenance(data, prov)
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}
