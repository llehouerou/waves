// Package testutil provides common testing utilities for UI components.
package testutil

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StripANSI removes ANSI escape codes from a string for easier testing.
// This allows comparing rendered output without style interference.
func StripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// NormalizeWhitespace replaces multiple consecutive whitespace characters
// with a single space and trims leading/trailing whitespace.
func NormalizeWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

// MeasureWidth returns the visual width of a string, accounting for
// wide characters (CJK, emoji) and stripping ANSI codes.
func MeasureWidth(s string) int {
	return lipgloss.Width(StripANSI(s))
}

// ContainsLine checks if any line in the output contains the given substring.
func ContainsLine(output, substr string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

// FindLine returns the first line containing the given substring, or empty string.
func FindLine(output, substr string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	return ""
}

// CountLines returns the number of non-empty lines in the output.
func CountLines(output string) int {
	count := 0
	for line := range strings.SplitSeq(output, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// SplitLines splits output into lines, removing trailing empty lines.
func SplitLines(output string) []string {
	lines := strings.Split(output, "\n")
	// Trim trailing empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// AssertContains returns an error message if output doesn't contain substr,
// or empty string if it does. Useful for test assertions.
func AssertContains(output, substr string) string {
	stripped := StripANSI(output)
	if !strings.Contains(stripped, substr) {
		return "expected output to contain " + substr
	}
	return ""
}

// AssertNotContains returns an error message if output contains substr,
// or empty string if it doesn't. Useful for test assertions.
func AssertNotContains(output, substr string) string {
	stripped := StripANSI(output)
	if strings.Contains(stripped, substr) {
		return "expected output to NOT contain " + substr
	}
	return ""
}
