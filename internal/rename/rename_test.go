package rename

import (
	"path/filepath"
	"testing"
)

// Test basic text transformations

func TestRemoveQuestionMarks(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"What Kind of Fool Am I?", "What Kind of Fool Am I"},
		{"Why?", "Why"},
		{"¿Qué Pasa?", "Qué Pasa"},
		{"No question", "No question"},
		{"Multiple??? questions", "Multiple questions"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := removeQuestionMarks(tt.input)
			if got != tt.expected {
				t.Errorf("removeQuestionMarks(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestReplaceQuoteMarks(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`Say "Hello"`, `Say 'Hello'`},
		{`"Quotes"`, `'Quotes'`},
		{`'Already single'`, `'Already single'`},
		{`"Double" and 'single'`, `'Double' and 'single'`},
		{`"Fancy quotes"`, `'Fancy quotes'`},
		{`'Fancy single'`, `'Fancy single'`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := replaceQuoteMarks(tt.input)
			if got != tt.expected {
				t.Errorf("replaceQuoteMarks(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestReplaceIllegalFileChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Some Band: Greatest Hits", "Some Band - Greatest Hits"},
		{"AC/DC", "AC - DC"},
		{"File\\Name", "File - Name"},
		{"Bigger > Than", "Bigger - Than"},
		{"Less < Than", "Less - Than"},
		{"Star*Power", "Star - Power"},
		{"Pipe|Line", "Pipe - Line"},
		{"Under_score", "Under - score"},
		{"Multiple:  spaces", "Multiple - spaces"},
		{"No illegal chars", "No illegal chars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := replaceIllegalFileChars(tt.input)
			if got != tt.expected {
				t.Errorf("replaceIllegalFileChars(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRemoveEndPeriod(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Mr.", "Mr"},
		{"Album Name.", "Album Name"},
		{"No period", "No period"},
		{"Period. in middle", "Period. in middle"},
		{"Jr.", "Jr"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := removeEndPeriod(tt.input)
			if got != tt.expected {
				t.Errorf("removeEndPeriod(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeSpaces(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Multiple   spaces", "Multiple spaces"},
		{"Tab\there", "Tab here"},
		{"Mix  of\t spaces", "Mix of spaces"},
		{"Single space", "Single space"},
		{"  Leading", "Leading"},
		{"Trailing  ", "Trailing"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeSpaces(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeSpaces(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRemoveFeatPatterns(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Song feat. Artist", "Song"},
		{"Song feat Artist", "Song"},
		{"Song ft. Artist", "Song"},
		{"Song ft Artist", "Song"},
		{"Song (feat. Artist)", "Song"},
		{"Song (feat Artist)", "Song"},
		{"Song [feat. Artist]", "Song"},
		{"Song {feat. Artist}", "Song"},
		{"Song (ft. Someone)", "Song"},
		{"Song Featuring Artist", "Song Featuring Artist"}, // "featuring" not matched
		{"Featured", "Featured"},                           // word starts with feat
		{"Defeat", "Defeat"},                               // "feat" not preceded by space
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := removeFeatPatterns(tt.input)
			if got != tt.expected {
				t.Errorf("removeFeatPatterns(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestReplace3DotsWithEllipsis(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Wait...", "Wait…"},
		{"What... is this...", "What… is this…"},
		{"No dots", "No dots"},
		{"One.", "One."},
		{"Two..", "Two.."},
		{"Four....", "Four…."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := replace3DotsWithEllipsis(tt.input)
			if got != tt.expected {
				t.Errorf("replace3DotsWithEllipsis(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestReplaceAndWithAmpersand(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Rock and Roll", "Rock & Roll"},
		{"Simon and Garfunkel", "Simon & Garfunkel"},
		{"Guns and Roses", "Guns & Roses"},
		{"Band", "Band"},                     // "and" not standalone
		{"Anderson", "Anderson"},             // "and" not standalone
		{"Andrew", "Andrew"},                 // "and" not standalone
		{"Salt AND Pepper", "Salt & Pepper"}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := replaceAndWithAmpersand(tt.input)
			if got != tt.expected {
				t.Errorf("replaceAndWithAmpersand(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// Test full path generation

func TestGeneratePath(t *testing.T) {
	tests := []struct {
		name     string
		meta     TrackMetadata
		expected string
	}{
		{
			name: "basic track",
			meta: TrackMetadata{
				Artist:      "Pink Floyd",
				AlbumArtist: "Pink Floyd",
				Album:       "The Dark Side of the Moon",
				Title:       "Time",
				TrackNumber: 4,
				Date:        "1973",
			},
			// Track filename: Artist • Album • TrackNum · Title
			expected: filepath.Join("Pink Floyd", "1973 • The Dark Side of the Moon", "Pink Floyd • The Dark Side of the Moon • 04 · Time"),
		},
		{
			name: "various artists",
			meta: TrackMetadata{
				Artist:      "Queen",
				AlbumArtist: "Various Artists",
				Album:       "Greatest Movie Hits",
				Title:       "Bohemian Rhapsody",
				TrackNumber: 1,
				Date:        "2001",
			},
			// VA folder is bracketed, track artist is Queen
			expected: filepath.Join("[Various Artists]", "2001 • Greatest Movie Hits", "Queen • Greatest Movie Hits • 01 · Bohemian Rhapsody"),
		},
		{
			name: "with illegal chars",
			meta: TrackMetadata{
				Artist:      "AC/DC",
				AlbumArtist: "AC/DC",
				Album:       "Back in Black",
				Title:       "You Shook Me All Night Long",
				TrackNumber: 6,
				Date:        "1980",
			},
			expected: filepath.Join("AC - DC", "1980 • Back in Black", "AC - DC • Back in Black • 06 · You Shook Me All Night Long"),
		},
		{
			name: "with feat pattern",
			meta: TrackMetadata{
				Artist:      "Drake feat. Rihanna",
				AlbumArtist: "Drake",
				Album:       "Take Care",
				Title:       "Take Care (feat. Rihanna)",
				TrackNumber: 5,
				Date:        "2011",
			},
			// feat patterns removed from artist and title
			expected: filepath.Join("Drake", "2011 • Take Care", "Drake • Take Care • 05 · Take Care"),
		},
		{
			name: "live album",
			meta: TrackMetadata{
				Artist:               "Nirvana",
				AlbumArtist:          "Nirvana",
				Album:                "MTV Unplugged in New York",
				Title:                "About a Girl",
				TrackNumber:          1,
				Date:                 "1994",
				SecondaryReleaseType: "live",
			},
			// live is a track note, goes on title not album folder
			expected: filepath.Join("Nirvana", "1994 • MTV Unplugged in New York", "Nirvana • MTV Unplugged in New York • 01 · About a Girl [live]"),
		},
		{
			name: "soundtrack",
			meta: TrackMetadata{
				Artist:               "Hans Zimmer",
				AlbumArtist:          "Hans Zimmer",
				Album:                "Inception: Music from the Motion Picture",
				Title:                "Time",
				TrackNumber:          12,
				Date:                 "2010",
				SecondaryReleaseType: "soundtrack",
			},
			// soundtrack is an album note
			expected: filepath.Join("Hans Zimmer", "2010 • Inception - Music from the Motion Picture [soundtrack]", "Hans Zimmer • Inception - Music from the Motion Picture [soundtrack] • 12 · Time"),
		},
		{
			name: "multi-disc",
			meta: TrackMetadata{
				Artist:      "The Beatles",
				AlbumArtist: "The Beatles",
				Album:       "The White Album",
				Title:       "Back in the U.S.S.R.",
				TrackNumber: 1,
				DiscNumber:  1,
				TotalDiscs:  2,
				Date:        "1968",
			},
			expected: filepath.Join("The Beatles", "1968 • The White Album", "The Beatles • The White Album • 01.01 · Back in the U.S.S.R."),
		},
		{
			name: "reissue with different date",
			meta: TrackMetadata{
				Artist:       "Joy Division",
				AlbumArtist:  "Joy Division",
				Album:        "Unknown Pleasures",
				Title:        "Disorder",
				TrackNumber:  1,
				Date:         "2007",
				OriginalDate: "1979",
			},
			// Reissue note appears in both folder AND track filename (per Picard behavior)
			expected: filepath.Join("Joy Division", "1979 • Unknown Pleasures [2007 reissue]", "Joy Division • Unknown Pleasures [2007 reissue] • 01 · Disorder"),
		},
		{
			name: "EP release",
			meta: TrackMetadata{
				Artist:      "Radiohead",
				AlbumArtist: "Radiohead",
				Album:       "My Iron Lung",
				Title:       "My Iron Lung",
				TrackNumber: 1,
				Date:        "1994",
				ReleaseType: "ep",
			},
			// EP is an album note
			expected: filepath.Join("Radiohead", "1994 • My Iron Lung [ep]", "Radiohead • My Iron Lung [ep] • 01 · My Iron Lung"),
		},
		{
			name: "single track",
			meta: TrackMetadata{
				Artist:      "Daft Punk",
				AlbumArtist: "Daft Punk",
				Album:       "[singles]",
				Title:       "Get Lucky",
				TrackNumber: 1,
				Date:        "2013",
				ReleaseType: "single",
			},
			// Singles: no album in track name, no track number
			expected: filepath.Join("Daft Punk", "2013 • [singles]", "Daft Punk • Get Lucky"),
		},
		{
			name: "unknown artist",
			meta: TrackMetadata{
				Album:       "Mystery Album",
				Title:       "Unknown Song",
				TrackNumber: 1,
				Date:        "2020",
			},
			expected: filepath.Join("[unknown artist]", "2020 • Mystery Album", "unknown artist • Mystery Album • 01 · Unknown Song"),
		},
		{
			name: "question marks and quotes",
			meta: TrackMetadata{
				Artist:      "The Who",
				AlbumArtist: "The Who",
				Album:       "Who Are You",
				Title:       "Who Are You?",
				TrackNumber: 9,
				Date:        "1978",
			},
			expected: filepath.Join("The Who", "1978 • Who Are You", "The Who • Who Are You • 09 · Who Are You"),
		},
		{
			name: "and replaced with ampersand",
			meta: TrackMetadata{
				Artist:      "Simon and Garfunkel",
				AlbumArtist: "Simon and Garfunkel",
				Album:       "Bridge over Troubled Water",
				Title:       "The Boxer",
				TrackNumber: 2,
				Date:        "1970",
			},
			expected: filepath.Join("Simon & Garfunkel", "1970 • Bridge over Troubled Water", "Simon & Garfunkel • Bridge over Troubled Water • 02 · The Boxer"),
		},
		{
			name: "no date",
			meta: TrackMetadata{
				Artist:      "Unknown Band",
				AlbumArtist: "Unknown Band",
				Album:       "Mystery Album",
				Title:       "Mystery Song",
				TrackNumber: 1,
			},
			// No year prefix when no date
			expected: filepath.Join("Unknown Band", "Mystery Album", "Unknown Band • Mystery Album • 01 · Mystery Song"),
		},
		{
			name: "ellipsis replacement",
			meta: TrackMetadata{
				Artist:      "The Band",
				AlbumArtist: "The Band",
				Album:       "Wait...",
				Title:       "Song...",
				TrackNumber: 1,
				Date:        "2000",
			},
			expected: filepath.Join("The Band", "2000 • Wait…", "The Band • Wait… • 01 · Song…"),
		},
		{
			name: "multiple notes - soundtrack and live",
			meta: TrackMetadata{
				Artist:               "John Williams",
				AlbumArtist:          "John Williams",
				Album:                "Star Wars Concert",
				Title:                "Imperial March",
				TrackNumber:          5,
				Date:                 "1999",
				SecondaryReleaseType: "soundtrack; live",
			},
			// soundtrack is album note, live is track note
			expected: filepath.Join("John Williams", "1999 • Star Wars Concert [soundtrack]", "John Williams • Star Wars Concert [soundtrack] • 05 · Imperial March [live]"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePath(tt.meta)
			if got != tt.expected {
				t.Errorf("GeneratePath() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Test release type extraction

func TestExtractReleaseNotes(t *testing.T) {
	tests := []struct {
		name               string
		releaseType        string
		secondaryType      string
		isVariousArtists   bool
		expectedAlbumNotes string
		expectedTrackNotes string
	}{
		{
			name:               "soundtrack",
			secondaryType:      "soundtrack",
			expectedAlbumNotes: "soundtrack",
		},
		{
			name:               "audiobook",
			secondaryType:      "audiobook",
			expectedAlbumNotes: "audiobook",
		},
		{
			name:               "mixtape/street",
			secondaryType:      "mixtape/street",
			expectedAlbumNotes: "mixtape/street",
		},
		{
			name:               "compilation non-VA",
			secondaryType:      "compilation",
			isVariousArtists:   false,
			expectedAlbumNotes: "compilation",
		},
		{
			name:               "compilation VA - no note",
			secondaryType:      "compilation",
			isVariousArtists:   true,
			expectedAlbumNotes: "",
		},
		{
			name:               "ep",
			releaseType:        "ep",
			expectedAlbumNotes: "ep",
		},
		{
			name:               "live",
			secondaryType:      "live",
			expectedTrackNotes: "live",
		},
		{
			name:               "broadcast",
			releaseType:        "broadcast",
			expectedTrackNotes: "broadcast",
		},
		{
			name:               "spokenword",
			secondaryType:      "spokenword",
			expectedTrackNotes: "spokenword",
		},
		{
			name:               "interview",
			secondaryType:      "interview",
			expectedTrackNotes: "interview",
		},
		{
			name:               "remix",
			secondaryType:      "remix",
			expectedTrackNotes: "remix",
		},
		{
			name:               "dj-mix",
			secondaryType:      "dj-mix",
			expectedTrackNotes: "dj-mix",
		},
		{
			name:               "multiple types",
			secondaryType:      "soundtrack; live",
			expectedAlbumNotes: "soundtrack",
			expectedTrackNotes: "live",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			albumNotes, trackNotes := extractReleaseNotes(tt.releaseType, tt.secondaryType, tt.isVariousArtists)
			if albumNotes != tt.expectedAlbumNotes {
				t.Errorf("albumNotes = %q, want %q", albumNotes, tt.expectedAlbumNotes)
			}
			if trackNotes != tt.expectedTrackNotes {
				t.Errorf("trackNotes = %q, want %q", trackNotes, tt.expectedTrackNotes)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Check templates match current hardcoded behavior
	if cfg.Folder == "" {
		t.Error("Folder template should not be empty")
	}
	if cfg.Filename == "" {
		t.Error("Filename template should not be empty")
	}

	// Check all toggles default to true
	if !cfg.ReissueNotation {
		t.Error("ReissueNotation should default to true")
	}
	if !cfg.VABrackets {
		t.Error("VABrackets should default to true")
	}
	if !cfg.SinglesHandling {
		t.Error("SinglesHandling should default to true")
	}
	if !cfg.ReleaseTypeNotes {
		t.Error("ReleaseTypeNotes should default to true")
	}
	if !cfg.AndToAmpersand {
		t.Error("AndToAmpersand should default to true")
	}
	if !cfg.RemoveFeat {
		t.Error("RemoveFeat should default to true")
	}
	if !cfg.EllipsisNormalize {
		t.Error("EllipsisNormalize should default to true")
	}
}

func TestGeneratePathWithConfig(t *testing.T) {
	meta := TrackMetadata{
		Artist:      "Pink Floyd",
		AlbumArtist: "Pink Floyd",
		Album:       "The Dark Side of the Moon",
		Title:       "Time",
		TrackNumber: 4,
		Date:        "1973",
	}

	// Custom simple template
	cfg := Config{
		Folder:            "{albumartist}/{album}",
		Filename:          "{tracknumber} - {title}",
		ReissueNotation:   true,
		VABrackets:        true,
		SinglesHandling:   true,
		ReleaseTypeNotes:  true,
		AndToAmpersand:    true,
		RemoveFeat:        true,
		EllipsisNormalize: true,
	}

	got := GeneratePathWithConfig(meta, cfg)
	want := filepath.Join("Pink Floyd", "The Dark Side of the Moon", "04 - Time")

	if got != want {
		t.Errorf("GeneratePathWithConfig() = %q, want %q", got, want)
	}
}

func TestGeneratePathWithConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		meta     TrackMetadata
		cfg      Config
		expected string
	}{
		{
			name: "empty album uses unknown album",
			meta: TrackMetadata{
				Artist:      "Test Artist",
				AlbumArtist: "Test Artist",
				Album:       "",
				Title:       "Test Song",
				TrackNumber: 1,
				Date:        "2020",
			},
			cfg:      DefaultConfig(),
			expected: filepath.Join("Test Artist", "2020 • [unknown album]", "Test Artist • [unknown album] • 01 · Test Song"),
		},
		{
			name: "empty title uses unknown title",
			meta: TrackMetadata{
				Artist:      "Test Artist",
				AlbumArtist: "Test Artist",
				Album:       "Test Album",
				Title:       "",
				TrackNumber: 1,
				Date:        "2020",
			},
			cfg:      DefaultConfig(),
			expected: filepath.Join("Test Artist", "2020 • Test Album", "Test Artist • Test Album • 01 · unknown title"),
		},
		{
			name: "zero track number uses 00",
			meta: TrackMetadata{
				Artist:      "Test Artist",
				AlbumArtist: "Test Artist",
				Album:       "Test Album",
				Title:       "Test Song",
				TrackNumber: 0,
				Date:        "2020",
			},
			cfg:      DefaultConfig(),
			expected: filepath.Join("Test Artist", "2020 • Test Album", "Test Artist • Test Album • 00 · Test Song"),
		},
		{
			name: "negative track number uses 00",
			meta: TrackMetadata{
				Artist:      "Test Artist",
				AlbumArtist: "Test Artist",
				Album:       "Test Album",
				Title:       "Test Song",
				TrackNumber: -1,
				Date:        "2020",
			},
			cfg:      DefaultConfig(),
			expected: filepath.Join("Test Artist", "2020 • Test Album", "Test Artist • Test Album • 00 · Test Song"),
		},
		{
			name: "zero disc number uses default",
			meta: TrackMetadata{
				Artist:      "Test Artist",
				AlbumArtist: "Test Artist",
				Album:       "Test Album",
				Title:       "Test Song",
				TrackNumber: 1,
				DiscNumber:  0,
				Date:        "2020",
			},
			cfg: Config{
				Folder:   "{albumartist}",
				Filename: "{discnumber}-{tracknumber} {title}",
			},
			expected: filepath.Join("Test Artist", "1-01 Test Song"),
		},
		{
			name: "negative disc number uses default",
			meta: TrackMetadata{
				Artist:      "Test Artist",
				AlbumArtist: "Test Artist",
				Album:       "Test Album",
				Title:       "Test Song",
				TrackNumber: 1,
				DiscNumber:  -1,
				Date:        "2020",
			},
			cfg: Config{
				Folder:   "{albumartist}",
				Filename: "{discnumber}-{tracknumber} {title}",
			},
			expected: filepath.Join("Test Artist", "1-01 Test Song"),
		},
		{
			name: "all empty metadata uses defaults",
			meta: TrackMetadata{},
			cfg:  DefaultConfig(),
			// Empty everything: unknown artist, no year, unknown album, unknown title
			expected: filepath.Join("[unknown artist]", "[unknown album]", "unknown artist • [unknown album] • 00 · unknown title"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePathWithConfig(tt.meta, tt.cfg)
			if got != tt.expected {
				t.Errorf("GeneratePathWithConfig() = %q, want %q", got, tt.expected)
			}
		})
	}
}
