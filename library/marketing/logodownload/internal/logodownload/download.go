package logodownload

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func DownloadImages(ctx context.Context, client *http.Client, results []LogoResult, selection Selection, outputDir string) error {
	indexes, err := selectedIndexes(results, selection)
	if err != nil {
		return err
	}
	if len(indexes) == 0 {
		return nil
	}

	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("não foi possível criar diretório de download: %w", err)
	}

	for _, index := range indexes {
		if results[index].ImageURL == "" {
			continue
		}

		path, err := downloadImage(ctx, client, results[index], outputDir)
		if err != nil {
			return err
		}
		results[index].DownloadPath = path
	}

	return nil
}

func downloadImage(ctx context.Context, client *http.Client, result LogoResult, outputDir string) (string, error) {
	resp, err := request(ctx, client, result.ImageURL, "image/*,*/*;q=0.8")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	extension := extensionFromURL(result.ImageURL)
	if extension == "" {
		extension = extensionFromContentType(resp.Header.Get("Content-Type"))
	}
	if extension == "" {
		extension = ".png"
	}

	filename := sanitizeFilename(result.Title)
	if filename == "" {
		filename = "logo"
	}

	path := filepath.Join(outputDir, filename+extension)
	path = uniquePath(path)

	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("não foi possível criar arquivo de download: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("não foi possível salvar imagem: %w", err)
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}

	return absolutePath, nil
}

func selectedIndexes(results []LogoResult, selection Selection) ([]int, error) {
	if len(results) == 0 {
		return []int{}, nil
	}

	switch selection.Mode {
	case SelectFirst:
		return []int{0}, nil
	case SelectAll:
		indexes := make([]int, 0, len(results))
		for index := range results {
			indexes = append(indexes, index)
		}
		return indexes, nil
	case SelectIndex:
		if selection.Index < 1 || selection.Index > len(results) {
			return nil, fmt.Errorf("índice %d fora do intervalo de resultados", selection.Index)
		}
		return []int{selection.Index - 1}, nil
	default:
		return nil, fmt.Errorf("seleção desconhecida: %s", selection.Mode)
	}
}

func extensionFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	extension := strings.ToLower(filepath.Ext(parsed.Path))
	switch extension {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg":
		return extension
	default:
		return ""
	}
}

func extensionFromContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}

	switch mediaType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	default:
		return ""
	}
}

func sanitizeFilename(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "&", " and ")

	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	value = re.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-._")

	if len(value) > 80 {
		value = strings.Trim(value[:80], "-._")
	}

	return value
}

func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	extension := filepath.Ext(path)
	base := strings.TrimSuffix(path, extension)
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d%s", base, index, extension)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
