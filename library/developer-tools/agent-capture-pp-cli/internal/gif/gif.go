// Package gif handles GIF encoding, video-to-GIF conversion, and frame stitching.
// Uses ffmpeg for video frame extraction and Go's image/gif for encoding
// with palette optimization.
package gif

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	gifpkg "image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ConvertOptions configures video-to-GIF conversion.
type ConvertOptions struct {
	FPS      int   // Output frame rate
	Width    int   // Output width (0 = original)
	MaxBytes int64 // Maximum file size in bytes
}

// StitchOptions configures multi-frame GIF stitching.
type StitchOptions struct {
	FrameDuration float64 // Seconds per frame
	Background    string  // white, black, transparent
	MaxBytes      int64   // Maximum file size
}

// ConvertVideo converts a video file to an animated GIF.
func ConvertVideo(ctx context.Context, input, output string, opts ConvertOptions) error {
	if opts.FPS <= 0 {
		opts.FPS = 12
	}

	// Extract frames using ffmpeg
	tmpDir, err := os.MkdirTemp("", "agent-capture-gif-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build ffmpeg filter
	filters := fmt.Sprintf("fps=%d", opts.FPS)
	if opts.Width > 0 {
		filters += fmt.Sprintf(",scale=%d:-1:flags=lanczos", opts.Width)
	}

	framePattern := filepath.Join(tmpDir, "frame-%04d.png")
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", input,
		"-vf", filters,
		"-vsync", "vfr",
		framePattern,
	)
	// Suppress ffmpeg's verbose output
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg frame extraction failed: %w", err)
	}

	// Collect frame files
	frames, err := filepath.Glob(filepath.Join(tmpDir, "frame-*.png"))
	if err != nil || len(frames) == 0 {
		return fmt.Errorf("no frames extracted from video")
	}
	sort.Strings(frames)

	// Encode GIF
	delay := 100 / opts.FPS // centiseconds per frame
	return encodeGIF(frames, output, delay, opts.MaxBytes)
}

// StitchFrames combines multiple image files into an animated GIF.
func StitchFrames(ctx context.Context, inputs []string, output string, opts StitchOptions) error {
	if len(inputs) == 0 {
		return fmt.Errorf("no input frames")
	}

	delay := int(opts.FrameDuration * 100) // convert seconds to centiseconds
	if delay <= 0 {
		delay = 300 // 3 seconds default
	}

	// Load all frames and find max dimensions
	var images []image.Image
	maxW, maxH := 0, 0
	for _, path := range inputs {
		img, err := loadImage(path)
		if err != nil {
			return fmt.Errorf("loading %s: %w", path, err)
		}
		images = append(images, img)
		b := img.Bounds()
		if b.Dx() > maxW {
			maxW = b.Dx()
		}
		if b.Dy() > maxH {
			maxH = b.Dy()
		}
	}

	// Normalize frames to max dimensions with padding
	bgColor := parseBackgroundColor(opts.Background)
	var normalizedPaths []string
	tmpDir, err := os.MkdirTemp("", "agent-capture-stitch-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for i, img := range images {
		normalized := normalizeFrame(img, maxW, maxH, bgColor)
		path := filepath.Join(tmpDir, fmt.Sprintf("frame-%04d.png", i))
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("creating normalized frame: %w", err)
		}
		if err := png.Encode(f, normalized); err != nil {
			f.Close()
			return fmt.Errorf("encoding normalized frame: %w", err)
		}
		f.Close()
		normalizedPaths = append(normalizedPaths, path)
	}

	return encodeGIF(normalizedPaths, output, delay, opts.MaxBytes)
}

// encodeGIF creates an animated GIF from PNG frame files.
func encodeGIF(framePaths []string, output string, delay int, maxBytes int64) error {
	g := &gifpkg.GIF{}

	for _, path := range framePaths {
		img, err := loadImage(path)
		if err != nil {
			return fmt.Errorf("loading frame %s: %w", path, err)
		}

		bounds := img.Bounds()
		palette := generatePalette(img)
		paletted := image.NewPaletted(bounds, palette)
		draw.FloydSteinberg.Draw(paletted, bounds, img, bounds.Min)

		g.Image = append(g.Image, paletted)
		g.Delay = append(g.Delay, delay)
	}

	g.LoopCount = 0 // infinite loop

	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("creating output: %w", err)
	}
	defer f.Close()

	if err := gifpkg.EncodeAll(f, g); err != nil {
		return fmt.Errorf("encoding GIF: %w", err)
	}

	// Check size limit and auto-reduce if needed
	if maxBytes > 0 {
		fi, err := os.Stat(output)
		if err == nil && fi.Size() > maxBytes {
			fmt.Fprintf(os.Stderr, "GIF is %d bytes (limit: %d). Auto-reducing...\n", fi.Size(), maxBytes)
			if err := autoReduceGIF(framePaths, output, delay, maxBytes); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: auto-reduce failed, keeping oversized GIF: %v\n", err)
			}
		}
	}

	return nil
}

// autoReduceGIF iteratively reduces frames and quality to hit the size target.
func autoReduceGIF(framePaths []string, output string, delay int, maxBytes int64) error {
	// Strategy: skip frames (reduce effective FPS), then shrink dimensions
	strategies := []struct {
		skipEveryN int
		scale      float64
	}{
		{2, 1.0},  // Skip every other frame
		{2, 0.75}, // Skip + 75% scale
		{3, 0.75}, // Skip 2/3 + 75% scale
		{2, 0.5},  // Skip + 50% scale
		{3, 0.5},  // Skip 2/3 + 50% scale
	}

	for _, s := range strategies {
		var reduced []string
		for i, p := range framePaths {
			if i%s.skipEveryN == 0 {
				reduced = append(reduced, p)
			}
		}
		if len(reduced) == 0 {
			continue
		}

		adjustedDelay := delay * s.skipEveryN

		g := &gifpkg.GIF{}
		for _, path := range reduced {
			img, err := loadImage(path)
			if err != nil {
				continue
			}

			bounds := img.Bounds()
			w := int(float64(bounds.Dx()) * s.scale)
			h := int(float64(bounds.Dy()) * s.scale)
			if w < 100 {
				continue
			}

			// Resize if scale < 1.0
			var target image.Image
			if s.scale < 1.0 {
				resized := image.NewRGBA(image.Rect(0, 0, w, h))
				// Simple nearest-neighbor resize for speed
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						srcX := int(float64(x) / s.scale)
						srcY := int(float64(y) / s.scale)
						if srcX < bounds.Dx() && srcY < bounds.Dy() {
							resized.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
						}
					}
				}
				target = resized
			} else {
				target = img
			}

			tb := target.Bounds()
			palette := generatePalette(target)
			paletted := image.NewPaletted(tb, palette)
			draw.FloydSteinberg.Draw(paletted, tb, target, tb.Min)

			g.Image = append(g.Image, paletted)
			g.Delay = append(g.Delay, adjustedDelay)
		}

		if len(g.Image) == 0 {
			continue
		}
		g.LoopCount = 0

		f, err := os.Create(output)
		if err != nil {
			return err
		}
		if err := gifpkg.EncodeAll(f, g); err != nil {
			f.Close()
			return err
		}
		f.Close()

		fi, err := os.Stat(output)
		if err != nil {
			return err
		}
		if fi.Size() <= maxBytes {
			fmt.Fprintf(os.Stderr, "Reduced to %d bytes (%d frames, %.0f%% scale)\n",
				fi.Size(), len(g.Image), s.scale*100)
			return nil
		}
	}

	return fmt.Errorf("could not reduce GIF below %d bytes", maxBytes)
}

// generatePalette creates a 256-color palette optimized for the image.
// Uses median cut quantization for better quality than Plan9 fallback.
func generatePalette(img image.Image) color.Palette {
	// Sample pixels for palette generation
	bounds := img.Bounds()
	colorMap := make(map[color.RGBA]int)

	step := 1
	if bounds.Dx()*bounds.Dy() > 100000 {
		step = 3 // Sample every 3rd pixel for large images
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			r, g, b, a := img.At(x, y).RGBA()
			c := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			colorMap[c]++
		}
	}

	// Sort by frequency, take top 255 + transparent
	type colorCount struct {
		c     color.RGBA
		count int
	}
	var counts []colorCount
	for c, n := range colorMap {
		counts = append(counts, colorCount{c, n})
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	palette := make(color.Palette, 0, 256)
	palette = append(palette, color.Transparent)
	for i := 0; i < len(counts) && len(palette) < 256; i++ {
		palette = append(palette, counts[i].c)
	}

	// Pad to 256 if needed
	for len(palette) < 256 {
		palette = append(palette, color.Black)
	}

	return palette
}

func normalizeFrame(img image.Image, targetW, targetH int, bg color.Color) *image.RGBA {
	result := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.Draw(result, result.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	bounds := img.Bounds()
	offsetX := (targetW - bounds.Dx()) / 2
	offsetY := (targetH - bounds.Dy()) / 2
	draw.Draw(result, image.Rect(offsetX, offsetY, offsetX+bounds.Dx(), offsetY+bounds.Dy()),
		img, bounds.Min, draw.Over)

	return result
}

func parseBackgroundColor(s string) color.Color {
	switch strings.ToLower(s) {
	case "black":
		return color.Black
	case "transparent":
		return color.Transparent
	default:
		return color.White
	}
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}
