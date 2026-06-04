package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type personalizationBatchRow struct {
	Row      int    `json:"row"`
	Output   string `json:"output"`
	Title    string `json:"title,omitempty"`
	ImageID  string `json:"image_id,omitempty"`
	TextUsed bool   `json:"text_used"`
}

func newPersonalizationBatchCmd(flags *rootFlags) *cobra.Command {
	var templateFile, csvFile, outDir string
	cmd := &cobra.Command{
		Use:     "personalization-batch",
		Short:   "Compile per-row personalized product manifests from a template and CSV",
		Example: "  printify-pp-cli personalization-batch --template ./examples/template-product.json --csv ./examples/personalization.csv --out ./examples/generated-manifests --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (templateFile == "" || csvFile == "" || outDir == "") && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			rows, err := buildPersonalizationBatch(templateFile, csvFile, outDir)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&templateFile, "template", "", "Template product manifest JSON")
	cmd.Flags().StringVar(&csvFile, "csv", "", "CSV rows to merge into the template")
	cmd.Flags().StringVar(&outDir, "out", "", "Directory for generated manifests")
	return cmd
}

func buildPersonalizationBatch(templateFile, csvFile, outDir string) ([]personalizationBatchRow, error) {
	rawTemplate, err := ppLoadJSONFile(templateFile)
	if err != nil {
		return nil, err
	}
	if err := ppEnsureOutDir(outDir); err != nil {
		return nil, err
	}
	csvRows, err := readCSVRows(csvFile)
	if err != nil {
		return nil, err
	}
	results := make([]personalizationBatchRow, 0, len(csvRows))
	usedOutputNames := map[string]int{}
	for index, row := range csvRows {
		var manifest any
		if err := json.Unmarshal(rawTemplate, &manifest); err != nil {
			return nil, err
		}
		manifest = replaceManifestTokens(manifest, row)
		outputName := uniqueBatchOutputName(ppSafeOutputName(row, index), usedOutputNames) + ".json"
		outputPath := filepath.Join(outDir, outputName)
		if err := ppWriteJSONFile(outputPath, manifest); err != nil {
			return nil, err
		}
		results = append(results, personalizationBatchRow{
			Row:      index + 1,
			Output:   outputPath,
			Title:    row["title"],
			ImageID:  firstNonEmpty(row["image_id"], row["upload_id"]),
			TextUsed: rowHasText(row),
		})
	}
	return results, nil
}

func uniqueBatchOutputName(base string, used map[string]int) string {
	if used[base] == 0 {
		used[base] = 1
		return base
	}
	for suffix := used[base] + 1; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if used[candidate] == 0 {
			used[base] = suffix
			used[candidate] = 1
			return candidate
		}
	}
}

func readCSVRows(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must include a header row and at least one data row")
	}
	headers := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := map[string]string{}
		for index, header := range headers {
			value := ""
			if index < len(record) {
				value = record[index]
			}
			row[strings.TrimSpace(header)] = strings.TrimSpace(value)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func replaceManifestTokens(value any, row map[string]string) any {
	switch typed := value.(type) {
	case string:
		result := typed
		for key, replacement := range row {
			result = strings.ReplaceAll(result, "{{"+key+"}}", replacement)
			result = strings.ReplaceAll(result, "{{ "+key+" }}", replacement)
		}
		return result
	case []any:
		for index, item := range typed {
			typed[index] = replaceManifestTokens(item, row)
		}
		return typed
	case map[string]any:
		for key, item := range typed {
			typed[key] = replaceManifestTokens(item, row)
		}
		return typed
	default:
		return value
	}
}

func rowHasText(row map[string]string) bool {
	for key, value := range row {
		if value != "" && strings.Contains(strings.ToLower(key), "text") {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
