package code

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
)

// RenderOptions configures code screenshot rendering.
type RenderOptions struct {
	Theme        string // dracula, nord, monokai, github, solarized-dark
	Language     string // auto-detect from extension if empty
	FontFamily   string // optional font file path
	FontSize     float64
	LineNumbers  bool
	Padding      int
	Margin       int
	Shadow       bool
	WindowChrome bool // macOS-style traffic lights
}

type styledToken struct {
	text  string
	style chroma.StyleEntry
}

func Render(source string, output string, opts RenderOptions) error {
	opts = withDefaults(opts)
	lexer := detectLexer(opts.Language, source)
	iterator, err := lexer.Tokenise(nil, strings.ReplaceAll(source, "\t", "    "))
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	face, err := loadFontFace(opts)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer closeFontFace(face)

	style := resolveStyle(opts.Theme)
	lines := splitStyledLines(chroma.SplitTokensIntoLines(iterator.Tokens()), style)
	if len(lines) == 0 {
		lines = [][]styledToken{{}}
	}

	dc := gg.NewContext(16, 16)
	dc.SetFontFace(face)
	charWidth, _ := dc.MeasureString("0")
	lineHeight := math.Max(dc.FontHeight()*1.45, opts.FontSize*1.45)
	lineCount := len(lines)
	digits := len(fmt.Sprintf("%d", lineCount))
	gutterPad := 12.0
	gutterWidth := 0.0
	if opts.LineNumbers {
		gutterWidth = float64(digits)*charWidth + gutterPad*2
	}
	maxLineWidth := 0.0
	for _, line := range lines {
		width := 0.0
		for _, token := range line {
			w, _ := dc.MeasureString(token.text)
			width += w
		}
		if width > maxLineWidth {
			maxLineWidth = width
		}
	}

	pad := float64(opts.Padding)
	margin := float64(opts.Margin)
	chromeHeight := 0.0
	if opts.WindowChrome {
		chromeHeight = math.Max(30, opts.FontSize*2.1)
	}
	contentW := gutterWidth + maxLineWidth
	contentH := float64(lineCount) * lineHeight
	windowW := contentW + pad*2
	windowH := contentH + pad*2 + chromeHeight
	shadowPad := 0.0
	if opts.Shadow {
		shadowPad = 20
	}
	canvasW := int(math.Ceil(windowW + margin*2 + shadowPad))
	canvasH := int(math.Ceil(windowH + margin*2 + shadowPad))
	windowX := margin
	windowY := margin
	if opts.Shadow {
		windowX += 4
		windowY += 2
	}

	dc = gg.NewContext(canvasW, canvasH)
	dc.SetFontFace(face)
	dc.SetColor(color.NRGBA{0, 0, 0, 0})
	dc.Clear()

	bg := style.Get(chroma.Background)
	bgColor := toNRGBA(bg.Background, color.NRGBA{40, 42, 54, 255})
	textColor := toNRGBA(style.Get(chroma.Text).Colour, color.NRGBA{248, 248, 242, 255})
	lineNumColor := toNRGBA(style.Get(chroma.LineNumbers).Colour, withAlpha(textColor, 150))
	frameRadius := 14.0
	if opts.Shadow {
		drawShadow(dc, windowX, windowY, windowW, windowH, frameRadius)
	}
	dc.DrawRoundedRectangle(windowX, windowY, windowW, windowH, frameRadius)
	dc.SetColor(bgColor)
	dc.Fill()

	if opts.WindowChrome {
		drawWindowChrome(dc, windowX, windowY, windowW, chromeHeight, frameRadius)
	}

	baseX := windowX + pad
	textX := baseX + gutterWidth
	baselineY := windowY + pad + chromeHeight + dc.FontHeight()
	if opts.LineNumbers && gutterWidth > 0 {
		separatorX := textX - gutterPad
		dc.SetColor(withAlpha(textColor, 40))
		dc.DrawLine(separatorX, windowY+chromeHeight+pad/2, separatorX, windowY+windowH-pad/2)
		dc.SetLineWidth(1)
		dc.Stroke()
	}
	for i, line := range lines {
		y := baselineY + float64(i)*lineHeight
		if opts.LineNumbers {
			label := fmt.Sprintf("%*d", digits, i+1)
			w, _ := dc.MeasureString(label)
			dc.SetColor(lineNumColor)
			dc.DrawString(label, baseX+gutterWidth-gutterPad-w, y)
		}
		x := textX
		for _, token := range line {
			if token.text == "" {
				continue
			}
			dc.SetColor(toNRGBA(token.style.Colour, textColor))
			dc.DrawString(token.text, x, y)
			w, _ := dc.MeasureString(token.text)
			x += w
		}
	}
	if err := dc.SavePNG(output); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

func RenderFile(path string, output string, opts RenderOptions) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if opts.Language == "" {
		if lexer := lexers.Match(path); lexer != nil {
			opts.Language = lexer.Config().Name
		} else {
			opts.Language = strings.TrimPrefix(filepath.Ext(path), ".")
		}
	}
	return Render(string(src), output, opts)
}

func withDefaults(opts RenderOptions) RenderOptions {
	if opts.Theme == "" {
		opts.Theme = "dracula"
	}
	if opts.FontSize <= 0 {
		opts.FontSize = 14
	}
	if opts.Padding == 0 {
		opts.Padding = 20
	}
	if opts.Margin == 0 {
		opts.Margin = 40
	}
	if opts.Theme != "" && opts.Padding == 20 && opts.Margin == 40 && opts.FontSize == 14 && opts.Language == "" && opts.FontFamily == "" && !opts.Shadow && !opts.WindowChrome {
		if !opts.LineNumbers {
			opts.LineNumbers = true
		}
	}
	return opts
}

func resolveStyle(name string) *chroma.Style {
	mapped := map[string]string{"solarized": "solarized-dark", "solarized-dark": "solarized-dark", "github": "github", "monokai": "monokai", "dracula": "dracula", "nord": "nord"}
	if style := styles.Get(mapped[strings.ToLower(name)]); style != nil {
		return style
	}
	return styles.Get("dracula")
}

func detectLexer(language, source string) chroma.Lexer {
	if language != "" {
		if lexer := lexers.Get(language); lexer != nil {
			return chroma.Coalesce(lexer)
		}
	}
	if lexer := lexers.Analyse(source); lexer != nil {
		return chroma.Coalesce(lexer)
	}
	return chroma.Coalesce(lexers.Fallback)
}

func splitStyledLines(lines [][]chroma.Token, style *chroma.Style) [][]styledToken {
	out := make([][]styledToken, 0, len(lines))
	for _, line := range lines {
		styled := make([]styledToken, 0, len(line))
		for _, token := range line {
			text := strings.TrimSuffix(token.Value, "\n")
			if text == "" {
				continue
			}
			styled = append(styled, styledToken{text: text, style: style.Get(token.Type)})
		}
		out = append(out, styled)
	}
	return out
}

func drawShadow(dc *gg.Context, x, y, w, h, r float64) {
	for i, alpha := range []uint8{18, 12, 8, 5} {
		off := float64(i+1) * 3
		dc.DrawRoundedRectangle(x+off, y+off*1.25, w, h, r+off/2)
		dc.SetColor(color.NRGBA{0, 0, 0, alpha})
		dc.Fill()
	}
}

func drawWindowChrome(dc *gg.Context, x, y, w, h, r float64) {
	dc.DrawRoundedRectangle(x, y, w, h, r)
	dc.Clip()
	dc.DrawRectangle(x, y, w, h)
	dc.SetColor(color.NRGBA{255, 255, 255, 10})
	dc.Fill()
	dc.ResetClip()
	cy := y + h/2
	cx := x + 20
	for i, c := range []color.NRGBA{{255, 95, 86, 255}, {255, 189, 46, 255}, {39, 201, 63, 255}} {
		dc.DrawCircle(cx+float64(i)*16, cy, 5.5)
		dc.SetColor(c)
		dc.Fill()
	}
}

func loadFontFace(opts RenderOptions) (font.Face, error) {
	if opts.FontFamily != "" {
		face, err := gg.LoadFontFace(opts.FontFamily, opts.FontSize)
		if err == nil {
			return face, nil
		}
	}
	fontBytes := gomono.TTF
	if opts.WindowChrome {
		fontBytes = gomonobold.TTF
	}
	parsed, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(parsed, &opentype.FaceOptions{Size: opts.FontSize, DPI: 72, Hinting: font.HintingNone})
}

func closeFontFace(face font.Face) {
	if closer, ok := face.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func toNRGBA(c chroma.Colour, fallback color.NRGBA) color.NRGBA {
	if !c.IsSet() {
		return fallback
	}
	return color.NRGBA{R: c.Red(), G: c.Green(), B: c.Blue(), A: 255}
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}
