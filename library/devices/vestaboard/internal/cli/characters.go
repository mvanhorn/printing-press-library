// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:novel-static-reference
//
// Curated reference data: the Vestaboard character-code table. An agent (or a
// person) building a `message send --body-json '{"characters": [[...]]}'`
// payload needs the integer code for each glyph, and `message preview` needs
// the inverse mapping to render a board layout as readable text. The board has
// no endpoint that returns this table, so it is transcribed from the docs:
// https://docs.vestaboard.com/docs/charactercodes (verified 2026-05-29).
// Re-transcribe on reprint if Vestaboard adds codes.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// vestaboardChar is one row of the character-code table.
type vestaboardChar struct {
	Code int    `json:"code"`
	Char string `json:"char"`
	Name string `json:"name"`
}

// vestaboardCharCodes is the full, ordered character-code table. Codes with no
// assigned glyph (43, 45, 51, 57, 58, 61) are intentionally absent.
var vestaboardCharCodes = []vestaboardChar{
	{0, " ", "Blank"},
	{1, "A", "A"}, {2, "B", "B"}, {3, "C", "C"}, {4, "D", "D"}, {5, "E", "E"},
	{6, "F", "F"}, {7, "G", "G"}, {8, "H", "H"}, {9, "I", "I"}, {10, "J", "J"},
	{11, "K", "K"}, {12, "L", "L"}, {13, "M", "M"}, {14, "N", "N"}, {15, "O", "O"},
	{16, "P", "P"}, {17, "Q", "Q"}, {18, "R", "R"}, {19, "S", "S"}, {20, "T", "T"},
	{21, "U", "U"}, {22, "V", "V"}, {23, "W", "W"}, {24, "X", "X"}, {25, "Y", "Y"},
	{26, "Z", "Z"},
	{27, "1", "One"}, {28, "2", "Two"}, {29, "3", "Three"}, {30, "4", "Four"},
	{31, "5", "Five"}, {32, "6", "Six"}, {33, "7", "Seven"}, {34, "8", "Eight"},
	{35, "9", "Nine"}, {36, "0", "Zero"},
	{37, "!", "Exclamation Mark"}, {38, "@", "At"}, {39, "#", "Pound"},
	{40, "$", "Dollar"}, {41, "(", "Left Parenthesis"}, {42, ")", "Right Parenthesis"},
	{44, "-", "Hyphen"}, {46, "+", "Plus"}, {47, "&", "Ampersand"}, {48, "=", "Equal"},
	{49, ";", "Semicolon"}, {50, ":", "Colon"}, {52, "'", "Single Quote"},
	{53, "\"", "Double Quote"}, {54, "%", "Percent"}, {55, ",", "Comma"},
	{56, ".", "Period"}, {59, "/", "Slash"}, {60, "?", "Question Mark"},
	{62, "°", "Degree (Flagship) / Heart (Note)"},
	{63, "r", "Red"}, {64, "o", "Orange"}, {65, "y", "Yellow"}, {66, "g", "Green"},
	{67, "b", "Blue"}, {68, "v", "Violet"}, {69, "w", "White"},
	{70, "k", "Black (Local API)"}, {71, "█", "Filled"},
}

// glyphByCode maps a code to the single character `message preview` renders for
// it. Color chips render as a lowercase initial so each cell stays one column
// wide and aligned; the preview prints a legend explaining them.
var glyphByCode = func() map[int]string {
	m := make(map[int]string, len(vestaboardCharCodes))
	for _, c := range vestaboardCharCodes {
		m[c.Code] = c.Char
	}
	return m
}()

// glyphForCode returns the display character for a code, or "?" for an unknown
// code so an unexpected payload renders visibly rather than silently blank.
func glyphForCode(code int) string {
	if g, ok := glyphByCode[code]; ok {
		return g
	}
	return "?"
}

func newNovelCharactersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "characters",
		Short: "Show the Vestaboard character-code table (code → glyph)",
		Long: "Show the Vestaboard character-code table mapping each integer code to its glyph and name.\n\n" +
			"Use this to build a 'message send --body-json' payload by hand, or to interpret a raw\n" +
			"layout array. Color chips (codes 63-70) render in 'message preview' as a lowercase initial\n" +
			"(r/o/y/g/b/v/w/k), and code 71 renders as a filled cell (█).",
		Example:     "  vestaboard-pp-cli characters\n  vestaboard-pp-cli characters --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := json.Marshal(vestaboardCharCodes)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					fmt.Fprintln(os.Stderr, "\nColor chips render in 'message preview' as: r=Red o=Orange y=Yellow g=Green b=Blue v=Violet w=White k=Black █=Filled")
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}
