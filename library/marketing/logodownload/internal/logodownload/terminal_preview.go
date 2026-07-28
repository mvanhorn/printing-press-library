package logodownload

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strings"
)

const (
	brailleDotWidth  = 2
	brailleDotHeight = 4
)

type TerminalPreviewOptions struct {
	Height int
	Width  int
	Limit  int
}

func RenderTerminalPreview(ctx context.Context, client *http.Client, results []LogoResult, options TerminalPreviewOptions) string {
	if options.Height <= 0 {
		options.Height = 12
	}
	if options.Width <= 0 {
		options.Width = 28
	}
	if options.Limit <= 0 || options.Limit > len(results) {
		options.Limit = len(results)
	}

	previews := make([][]string, 0, options.Limit)
	titles := make([]string, 0, options.Limit)

	for index := 0; index < options.Limit; index++ {
		result := results[index]
		if result.ImageURL == "" {
			continue
		}

		lines, err := renderImageURL(ctx, client, result.ImageURL, options.Width, options.Height)
		if err != nil {
			lines = renderErrorBox(options.Width, options.Height)
		}

		previews = append(previews, lines)
		titles = append(titles, fitText(fmt.Sprintf("%d. %s", index+1, result.Title), options.Width))
	}

	if len(previews) == 0 {
		return ""
	}

	var builder strings.Builder
	for index, title := range titles {
		if index > 0 {
			builder.WriteString("  ")
		}
		builder.WriteString(title)
	}
	builder.WriteByte('\n')

	for row := 0; row < options.Height; row++ {
		for index, preview := range previews {
			if index > 0 {
				builder.WriteString("  ")
			}
			builder.WriteString(preview[row])
		}
		builder.WriteByte('\n')
	}

	return builder.String()
}

func renderImageURL(ctx context.Context, client *http.Client, imageURL string, width int, height int) ([]string, error) {
	resp, err := request(ctx, client, imageURL, "image/*,*/*;q=0.8")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return renderImage(img, width, height), nil
}

func renderImage(img image.Image, width int, height int) []string {
	bounds := visibleBounds(img)
	matrixWidth := width * brailleDotWidth
	matrixHeight := height * brailleDotHeight
	matrix := make([][]bool, matrixHeight)
	for row := range matrix {
		matrix[row] = make([]bool, matrixWidth)
	}

	sourceWidth := bounds.Dx()
	sourceHeight := bounds.Dy()
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return renderErrorBox(width, height)
	}

	sourceAspect := float64(sourceWidth) / float64(sourceHeight)
	targetAspect := float64(matrixWidth) / float64(matrixHeight)

	drawWidth := matrixWidth
	drawHeight := matrixHeight
	if sourceAspect > targetAspect {
		drawHeight = max(1, int(math.Round(float64(matrixWidth)/sourceAspect)))
	} else {
		drawWidth = max(1, int(math.Round(float64(matrixHeight)*sourceAspect)))
	}

	offsetX := (matrixWidth - drawWidth) / 2
	offsetY := (matrixHeight - drawHeight) / 2

	for y := 0; y < drawHeight; y++ {
		for x := 0; x < drawWidth; x++ {
			sourceX := bounds.Min.X + int((float64(x)+0.5)*float64(sourceWidth)/float64(drawWidth))
			sourceY := bounds.Min.Y + int((float64(y)+0.5)*float64(sourceHeight)/float64(drawHeight))
			if sourceX >= bounds.Max.X {
				sourceX = bounds.Max.X - 1
			}
			if sourceY >= bounds.Max.Y {
				sourceY = bounds.Max.Y - 1
			}

			if pixelInk(img.At(sourceX, sourceY).RGBA()) > 0.16 {
				matrix[offsetY+y][offsetX+x] = true
			}
		}
	}

	return brailleRows(matrix, width, height)
}

func visibleBounds(img image.Image) image.Rectangle {
	bounds := img.Bounds()
	minX := bounds.Max.X
	minY := bounds.Max.Y
	maxX := bounds.Min.X
	maxY := bounds.Min.Y

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if pixelInk(img.At(x, y).RGBA()) <= 0.08 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}

	if minX >= maxX || minY >= maxY {
		return bounds
	}

	padX := max(1, (maxX-minX)/20)
	padY := max(1, (maxY-minY)/20)
	minX = max(bounds.Min.X, minX-padX)
	minY = max(bounds.Min.Y, minY-padY)
	maxX = min(bounds.Max.X, maxX+padX)
	maxY = min(bounds.Max.Y, maxY+padY)

	return image.Rect(minX, minY, maxX, maxY)
}

func pixelInk(r uint32, g uint32, b uint32, a uint32) float64 {
	alpha := float64(a) / 65535
	if alpha < 0.08 {
		return 0
	}

	rf := float64(r) / 65535
	gf := float64(g) / 65535
	bf := float64(b) / 65535
	luma := 0.299*rf + 0.587*gf + 0.114*bf
	chroma := maxFloat(rf, gf, bf) - minFloat(rf, gf, bf)
	contrast := math.Max(1-luma, chroma*0.9)

	if alpha < 0.99 {
		return alpha
	}

	return contrast
}

func brailleRows(matrix [][]bool, width int, height int) []string {
	rows := make([]string, 0, height)
	for row := 0; row < height; row++ {
		var builder strings.Builder
		for column := 0; column < width; column++ {
			mask := 0
			for dotY := 0; dotY < brailleDotHeight; dotY++ {
				for dotX := 0; dotX < brailleDotWidth; dotX++ {
					if !matrix[row*brailleDotHeight+dotY][column*brailleDotWidth+dotX] {
						continue
					}
					mask |= brailleMask(dotX, dotY)
				}
			}
			if mask == 0 {
				builder.WriteRune(' ')
				continue
			}
			builder.WriteRune(rune(0x2800 + mask))
		}
		rows = append(rows, builder.String())
	}
	return rows
}

func brailleMask(x int, y int) int {
	switch {
	case x == 0 && y == 0:
		return 1 << 0
	case x == 0 && y == 1:
		return 1 << 1
	case x == 0 && y == 2:
		return 1 << 2
	case x == 1 && y == 0:
		return 1 << 3
	case x == 1 && y == 1:
		return 1 << 4
	case x == 1 && y == 2:
		return 1 << 5
	case x == 0 && y == 3:
		return 1 << 6
	case x == 1 && y == 3:
		return 1 << 7
	default:
		return 0
	}
}

func renderErrorBox(width int, height int) []string {
	rows := make([]string, 0, height)
	for row := 0; row < height; row++ {
		if row == 0 || row == height-1 {
			rows = append(rows, strings.Repeat("?", width))
			continue
		}
		rows = append(rows, "?"+strings.Repeat(" ", max(0, width-2))+"?")
	}
	return rows
}

func fitText(value string, width int) string {
	runes := []rune(value)
	if len(runes) > width {
		if width <= 1 {
			return string(runes[:width])
		}
		value = string(runes[:width-1]) + "…"
	}

	padding := width - len([]rune(value))
	if padding > 0 {
		value += strings.Repeat(" ", padding)
	}

	return value
}

func maxFloat(values ...float64) float64 {
	result := values[0]
	for _, value := range values[1:] {
		if value > result {
			result = value
		}
	}
	return result
}

func minFloat(values ...float64) float64 {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}
