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
		{"The Beatles", "the beatles"},

		// Punctuation dropped, separators folded to a single space
		{"Abbey Road: Remaster", "abbey road remaster"},
		{"What's Going On", "whats going on"},
		{"Rock 'n' Roll", "rock n roll"},
		{"Hello-World", "hello world"},
		{"AC/DC", "acdc"},
		{"Guns N' Roses", "guns n roses"},
		{"P!nk", "pnk"},
		{"Ke$ha", "keha"},
		{"Under_Score", "under score"},

		// Typographic apostrophes equivalent to ASCII
		{"Guns N\u2019 Roses", "guns n roses"},
		{"Guns N\u02bc Roses", "guns n roses"},
		{"Guns N\u0060 Roses", "guns n roses"},

		// Whitespace normalized
		{"Abbey  Road", "abbey road"},
		{"  Thriller  ", "thriller"},
		{"  Multiple   Spaces  ", "multiple spaces"},

		// Remaster suffixes
		{"OK Computer (Remastered)", "ok computer"},
		{"Song Title (Remaster)", "song title"},
		{"Song Title - Remastered", "song title"},
		{"Song Title [Remastered]", "song title"},
		{"Song Title (2023 Remaster)", "song title 2023 remaster"}, // only exact suffixes trimmed
		{"The Dark Side of the Moon (2011 Remaster)", "the dark side of the moon 2011 remaster"},

		// Accents folded
		{"Sigur Rós", "sigur ros"},
		{"Björk", "bjork"},
		{"Café Tacvba", "cafe tacvba"},
		{"Мотörhead", "мотorhead"},

		// Non-latin scripts preserved
		{"日本語", "日本語"},
		{"坂本龍一", "坂本龍一"},
		{"Кино", "кино"},
		{"עידן רייכל", "עידן רייכל"},

		// Empty and edge cases
		{"", ""},
		{"   ", ""},
		{"!!!", ""},
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

// The original bug: non-latin names all normalized to "" and collided.
func TestNormalizeTitleNonLatinDistinct(t *testing.T) {
	inputs := []string{"日本語", "坂本龍一", "Кино", "Мумий Тролль", "עידן רייכל", "משינה"}

	seen := make(map[string]string, len(inputs))
	for _, in := range inputs {
		got := NormalizeTitle(in)
		if got == "" {
			t.Errorf("NormalizeTitle(%q) = %q, want non-empty", in, got)
			continue
		}
		if prev, ok := seen[got]; ok {
			t.Errorf("NormalizeTitle(%q) collides with NormalizeTitle(%q) = %q", in, prev, got)
		}
		seen[got] = in
	}
}
