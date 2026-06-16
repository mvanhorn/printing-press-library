// Copyright 2026 Kieran and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: U10 Text formatting utility tests

package cliutil

import (
	"strings"
	"testing"
)

// TestTruncate tests string truncation
func TestTruncate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		maxLen  int
		want    string
	}{
		{
			name:   "short_string_no_truncate",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact_length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "truncate_simple",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "truncate_long_string",
			input:  "The quick brown fox jumps over the lazy dog",
			maxLen: 20,
			want:   "The quick brown f...",
		},
		{
			name:   "max_len_equals_3",
			input:  "hello",
			maxLen: 3,
			want:   "...",
		},
		{
			name:   "max_len_less_than_3",
			input:  "hello",
			maxLen: 2,
			want:   "...",
		},
		{
			name:   "empty_string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "single_char",
			input:  "a",
			maxLen: 1,
			want:   "a",
		},
		{
			name:   "max_len_zero",
			input:  "hello",
			maxLen: 0,
			want:   "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(tt.input, tt.maxLen)
			if result != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.want)
			}
		})
	}
}

// TestPadRight tests right padding
func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "short_string_pad_right",
			input: "hello",
			width: 10,
			want:  "hello     ",
		},
		{
			name:  "exact_width",
			input: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "string_longer_than_width",
			input: "hello world",
			width: 5,
			want:  "hello world",
		},
		{
			name:  "empty_string",
			input: "",
			width: 5,
			want:  "     ",
		},
		{
			name:  "zero_width",
			input: "hello",
			width: 0,
			want:  "hello",
		},
		{
			name:  "single_char",
			input: "a",
			width: 3,
			want:  "a  ",
		},
		{
			name:  "large_width",
			input: "test",
			width: 100,
			want:  "test" + strings.Repeat(" ", 96),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadRight(tt.input, tt.width)
			if result != tt.want {
				t.Errorf("PadRight(%q, %d) = %q (len=%d), want %q (len=%d)",
					tt.input, tt.width, result, len(result), tt.want, len(tt.want))
			}
			if len(result) < tt.width {
				t.Errorf("PadRight result length %d < width %d", len(result), tt.width)
			}
		})
	}
}

// TestPadLeft tests left padding
func TestPadLeft(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "short_string_pad_left",
			input: "hello",
			width: 10,
			want:  "     hello",
		},
		{
			name:  "exact_width",
			input: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "string_longer_than_width",
			input: "hello world",
			width: 5,
			want:  "hello world",
		},
		{
			name:  "empty_string",
			input: "",
			width: 5,
			want:  "     ",
		},
		{
			name:  "zero_width",
			input: "hello",
			width: 0,
			want:  "hello",
		},
		{
			name:  "single_char",
			input: "5",
			width: 3,
			want:  "  5",
		},
		{
			name:  "numeric_string",
			input: "42",
			width: 5,
			want:  "   42",
		},
		{
			name:  "large_width",
			input: "test",
			width: 100,
			want:  strings.Repeat(" ", 96) + "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadLeft(tt.input, tt.width)
			if result != tt.want {
				t.Errorf("PadLeft(%q, %d) = %q (len=%d), want %q (len=%d)",
					tt.input, tt.width, result, len(result), tt.want, len(tt.want))
			}
			if len(result) < tt.width {
				t.Errorf("PadLeft result length %d < width %d", len(result), tt.width)
			}
		})
	}
}

// TestTruncateAndPadCombination tests combining truncate and padding
func TestTruncateAndPadCombination(t *testing.T) {
	// A common pattern: truncate to a max length, then pad to exact width
	input := "Very long database name that should be truncated"
	maxLen := 20
	padWidth := 25

	truncated := Truncate(input, maxLen)
	padded := PadRight(truncated, padWidth)

	if len(padded) != padWidth {
		t.Errorf("Combined length = %d, want %d", len(padded), padWidth)
	}
	if !strings.HasPrefix(padded, truncated) {
		t.Errorf("Padded result doesn't start with truncated string")
	}
}

// TestPadRightPreserveContent tests that PadRight preserves original content
func TestPadRightPreserveContent(t *testing.T) {
	input := "test"
	result := PadRight(input, 10)
	if !strings.HasPrefix(result, input) {
		t.Errorf("PadRight didn't preserve original content at start")
	}
	if len(result) != 10 {
		t.Errorf("PadRight result length = %d, want 10", len(result))
	}
}

// TestPadLeftPreserveContent tests that PadLeft preserves original content
func TestPadLeftPreserveContent(t *testing.T) {
	input := "test"
	result := PadLeft(input, 10)
	if !strings.HasSuffix(result, input) {
		t.Errorf("PadLeft didn't preserve original content at end")
	}
	if len(result) != 10 {
		t.Errorf("PadLeft result length = %d, want 10", len(result))
	}
}

// TestTruncateEdgeCases tests edge cases for truncate
func TestTruncateEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
	}{
		{
			name:   "negative_max_len",
			input:  "hello",
			maxLen: -1,
		},
		{
			name:   "very_large_max_len",
			input:  "hello",
			maxLen: 1000000,
		},
		{
			name:   "special_chars",
			input:  "hello!@#$%^&*()",
			maxLen: 8,
		},
		{
			name:   "whitespace",
			input:  "hello   world",
			maxLen: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := Truncate(tt.input, tt.maxLen)
			if result == "" && tt.input != "" && tt.maxLen > 0 {
				// Should have content if input has content and maxLen allows
				if tt.maxLen >= len(tt.input) {
					t.Errorf("Truncate returned empty string unexpectedly")
				}
			}
		})
	}
}

// TestPadFunctionsWithSpecialChars tests padding with special characters
func TestPadFunctionsWithSpecialChars(t *testing.T) {
	specialChars := "hello→world"
	rightPadded := PadRight(specialChars, 20)
	leftPadded := PadLeft(specialChars, 20)

	if len(rightPadded) != 20 {
		t.Errorf("Right pad length = %d, want 20", len(rightPadded))
	}
	if len(leftPadded) != 20 {
		t.Errorf("Left pad length = %d, want 20", len(leftPadded))
	}
	if !strings.Contains(rightPadded, specialChars) {
		t.Errorf("Right padded doesn't contain original content")
	}
	if !strings.Contains(leftPadded, specialChars) {
		t.Errorf("Left padded doesn't contain original content")
	}
}

// TestTruncateWithNumbers tests truncate with numeric strings
func TestTruncateWithNumbers(t *testing.T) {
	numeric := "1234567890"
	result := Truncate(numeric, 5)
	if result != "12..." {
		t.Errorf("Truncate(%q, 5) = %q, want %q", numeric, result, "12...")
	}
}

// BenchmarkTruncate benchmarks the Truncate function
func BenchmarkTruncate(b *testing.B) {
	input := "This is a long string that might need truncation for display purposes"
	for i := 0; i < b.N; i++ {
		Truncate(input, 20)
	}
}

// BenchmarkPadRight benchmarks the PadRight function
func BenchmarkPadRight(b *testing.B) {
	input := "test"
	for i := 0; i < b.N; i++ {
		PadRight(input, 50)
	}
}

// BenchmarkPadLeft benchmarks the PadLeft function
func BenchmarkPadLeft(b *testing.B) {
	input := "test"
	for i := 0; i < b.N; i++ {
		PadLeft(input, 50)
	}
}

// TestPadRightSpaceCount verifies exact space count
func TestPadRightSpaceCount(t *testing.T) {
	result := PadRight("abc", 10)
	expectedSpaces := 7
	actualSpaces := len(result) - 3
	if actualSpaces != expectedSpaces {
		t.Errorf("PadRight space count = %d, want %d", actualSpaces, expectedSpaces)
	}
}

// TestPadLeftSpaceCount verifies exact space count
func TestPadLeftSpaceCount(t *testing.T) {
	result := PadLeft("abc", 10)
	expectedSpaces := 7
	actualSpaces := len(result) - 3
	if actualSpaces != expectedSpaces {
		t.Errorf("PadLeft space count = %d, want %d", actualSpaces, expectedSpaces)
	}
}

// TestTruncateAllowsMaxLenChange tests that truncate doesn't alter input
func TestTruncateImmutability(t *testing.T) {
	original := "hello world"
	originalCopy := original
	Truncate(original, 5)
	if original != originalCopy {
		t.Errorf("Truncate modified input string")
	}
}

// TestPadRightImmutability tests that PadRight doesn't alter input
func TestPadRightImmutability(t *testing.T) {
	original := "hello"
	originalCopy := original
	PadRight(original, 10)
	if original != originalCopy {
		t.Errorf("PadRight modified input string")
	}
}

// TestPadLeftImmutability tests that PadLeft doesn't alter input
func TestPadLeftImmutability(t *testing.T) {
	original := "hello"
	originalCopy := original
	PadLeft(original, 10)
	if original != originalCopy {
		t.Errorf("PadLeft modified input string")
	}
}

// TestFormattingChain tests chaining formatting operations
func TestFormattingChain(t *testing.T) {
	input := "Database name with a very long string that needs formatting"

	// Truncate then pad
	truncated := Truncate(input, 30)
	padded := PadRight(truncated, 40)

	if len(padded) != 40 {
		t.Errorf("Chained format result length = %d, want 40", len(padded))
	}
	if !strings.HasPrefix(padded, "Database name with") {
		t.Errorf("Chained format lost prefix content: %q", padded)
	}
}
