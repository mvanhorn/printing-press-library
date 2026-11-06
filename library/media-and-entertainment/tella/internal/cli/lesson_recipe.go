// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

type lessonRecipe struct {
	Silences        string          `yaml:"silences" json:"silences"`
	Fillers         *bool           `yaml:"fillers" json:"fillers,omitempty"`
	Zooms           bool            `yaml:"zooms" json:"zooms"`
	Crop            bool            `yaml:"crop" json:"crop"`
	ZoomStyle       string          `yaml:"zoomStyle" json:"zoomStyle,omitempty"`
	SubtitlePreset  string          `yaml:"subtitlePreset" json:"subtitlePreset,omitempty"`
	Captions        *lessonCaptions `yaml:"captions" json:"captions,omitempty"`
	BackgroundMusic *lessonMusic    `yaml:"backgroundMusic" json:"backgroundMusic,omitempty"`
	Intro           *lessonMedia    `yaml:"intro" json:"intro,omitempty"`
	Outro           *lessonMedia    `yaml:"outro" json:"outro,omitempty"`
	Thumbnail       lessonThumbnail `yaml:"thumbnail" json:"thumbnail"`
}
type lessonCaptions struct {
	FontFamily     string   `yaml:"fontFamily" json:"fontFamily,omitempty"`
	FontSize       *float64 `yaml:"fontSize" json:"fontSize,omitempty"`
	Size           string   `yaml:"size" json:"size,omitempty"`
	Enabled        *bool    `yaml:"enabled" json:"enabled,omitempty"`
	DefaultEnabled *bool    `yaml:"defaultEnabled" json:"defaultEnabled,omitempty"`
}
type lessonMusic struct {
	PresetID string   `yaml:"presetId" json:"presetId,omitempty"`
	SourceID string   `yaml:"sourceId" json:"sourceId,omitempty"`
	Volume   *float64 `yaml:"volume" json:"volume,omitempty"`
}
type lessonMedia struct {
	SourceID string `yaml:"sourceId" json:"sourceId"`
	Name     string `yaml:"name" json:"name,omitempty"`
}
type lessonThumbnail struct {
	Rule      string `yaml:"rule" json:"rule"`
	InpointMs *int   `yaml:"inpointMs" json:"inpointMs,omitempty"`
}

func readLessonRecipe(filename string) (lessonRecipe, error) {
	r := lessonRecipe{Silences: "natural", Thumbnail: lessonThumbnail{Rule: "first-spoken"}}
	if filename != "" {
		f, err := os.Open(filename)
		if err != nil {
			return r, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		if err != nil {
			return r, err
		}
		if len(data) > 1<<20 {
			return r, fmt.Errorf("recipe exceeds 1 MiB")
		}
		d := yaml.NewDecoder(bytes.NewReader(data))
		d.KnownFields(true)
		if err = d.Decode(&r); err != nil {
			return r, fmt.Errorf("recipe: %w", err)
		}
		var extra any
		if err = d.Decode(&extra); err != io.EOF {
			return r, fmt.Errorf("recipe must contain exactly one YAML document")
		}
	}
	return r, r.validate()
}
func (r lessonRecipe) automatics() autoOptions {
	fillers := true
	if r.Fillers != nil {
		fillers = *r.Fillers
	}
	return autoOptions{Fillers: fillers, Silences: r.Silences, Zooms: r.Zooms || r.ZoomStyle != "", ZoomStyle: r.ZoomStyle, Crop: r.Crop}
}
func finiteRange(n float64, min, max float64) bool {
	return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= min && n <= max
}
func (r lessonRecipe) validate() error {
	if err := r.automatics().validate(); err != nil {
		return err
	}
	if r.SubtitlePreset != "" && strings.TrimSpace(r.SubtitlePreset) == "" {
		return fmt.Errorf("subtitlePreset must not be blank")
	}
	if c := r.Captions; c != nil {
		if c.FontFamily != "" && strings.TrimSpace(c.FontFamily) == "" {
			return fmt.Errorf("caption fontFamily must not be blank")
		}
		if c.FontSize != nil && !finiteRange(*c.FontSize, 40, 200) {
			return fmt.Errorf("captions.fontSize must be 40–200")
		}
		if c.Size != "" && c.Size != "small" && c.Size != "medium" && c.Size != "large" {
			return fmt.Errorf("captions.size must be small, medium or large")
		}
		if c.Size != "" && c.FontSize != nil {
			return fmt.Errorf("captions.size and fontSize are mutually exclusive")
		}
		if c.Enabled != nil && !*c.Enabled && c.DefaultEnabled != nil && *c.DefaultEnabled {
			return fmt.Errorf("captions.defaultEnabled conflicts with enabled:false")
		}
	}
	if m := r.BackgroundMusic; m != nil {
		if (strings.TrimSpace(m.PresetID) == "") == (strings.TrimSpace(m.SourceID) == "") {
			return fmt.Errorf("backgroundMusic requires exactly one presetId or sourceId")
		}
		if m.Volume == nil || !finiteRange(*m.Volume, 0, 1) {
			return fmt.Errorf("backgroundMusic.volume is required within 0–1")
		}
	}
	for _, m := range []*lessonMedia{r.Intro, r.Outro} {
		if m == nil {
			continue
		}
		if strings.TrimSpace(m.SourceID) == "" || strings.ContainsAny(m.SourceID, "/\\?#\x00") || strings.Contains(m.SourceID, ":") {
			return fmt.Errorf("intro/outro requires an existing sourceId, never a local path")
		}
		if utf8.RuneCountInString(m.Name) > 255 || strings.ContainsAny(m.Name, "\r\n") {
			return fmt.Errorf("intro/outro name must be a single line of at most 255 characters")
		}
	}
	if r.Thumbnail.Rule != "first-spoken" && r.Thumbnail.Rule != "fixed" {
		return fmt.Errorf("thumbnail.rule must be first-spoken or fixed")
	}
	if r.Thumbnail.Rule == "fixed" && (r.Thumbnail.InpointMs == nil || *r.Thumbnail.InpointMs < 0) {
		return fmt.Errorf("fixed thumbnail requires nonnegative inpointMs")
	}
	if r.Thumbnail.Rule == "first-spoken" && r.Thumbnail.InpointMs != nil {
		return fmt.Errorf("first-spoken thumbnail cannot specify inpointMs")
	}
	return nil
}
func (c lessonCaptions) patch() map[string]any {
	b := map[string]any{}
	if c.FontFamily != "" {
		b["captionFontFamily"] = c.FontFamily
	}
	if c.FontSize != nil {
		b["captionFontSize"] = *c.FontSize
	}
	if c.Size != "" {
		b["captionSize"] = c.Size
	}
	if c.Enabled != nil {
		b["subtitlesEnabled"] = *c.Enabled
	}
	if c.DefaultEnabled != nil {
		b["captionsDefaultEnabled"] = *c.DefaultEnabled
	}
	return b
}
