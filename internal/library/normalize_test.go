package library

import "testing"

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic cases
		{"Abbey Road", "abbey road"},
		{"THRILLER", "thriller"},

		// Punctuation replaced with space (then normalized)
		{"Abbey Road: Remaster", "abbey road remaster"},
		{"What's Going On", "what s going on"},
		{"Rock 'n' Roll", "rock n roll"},
		{"Hello-World", "hello world"},

		// Multiple spaces normalized
		{"Abbey  Road", "abbey road"},
		{"  Thriller  ", "thriller"},

		// Mixed cases
		{"The Dark Side of the Moon (2011 Remaster)", "the dark side of the moon 2011 remaster"},

		// Empty and edge cases
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeTitle(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
