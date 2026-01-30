package lyrics

import (
	"strings"
	"testing"
	"time"
)

func TestParseLRC_Basic(t *testing.T) {
	lrc := `[ar:Test Artist]
[ti:Test Title]
[al:Test Album]
[00:12.34]First line
[00:15.67]Second line
[00:20.00]Third line`

	lyrics, err := ParseLRC(strings.NewReader(lrc))
	if err != nil {
		t.Fatalf("ParseLRC error: %v", err)
	}

	// Check metadata
	if lyrics.Artist != "Test Artist" {
		t.Errorf("Artist = %q, want %q", lyrics.Artist, "Test Artist")
	}
	if lyrics.Title != "Test Title" {
		t.Errorf("Title = %q, want %q", lyrics.Title, "Test Title")
	}
	if lyrics.Album != "Test Album" {
		t.Errorf("Album = %q, want %q", lyrics.Album, "Test Album")
	}

	// Check lines
	if len(lyrics.Lines) != 3 {
		t.Fatalf("len(Lines) = %d, want 3", len(lyrics.Lines))
	}

	expected := []struct {
		time time.Duration
		text string
	}{
		{12*time.Second + 340*time.Millisecond, "First line"},
		{15*time.Second + 670*time.Millisecond, "Second line"},
		{20 * time.Second, "Third line"},
	}

	for i, exp := range expected {
		if lyrics.Lines[i].Time != exp.time {
			t.Errorf("Lines[%d].Time = %v, want %v", i, lyrics.Lines[i].Time, exp.time)
		}
		if lyrics.Lines[i].Text != exp.text {
			t.Errorf("Lines[%d].Text = %q, want %q", i, lyrics.Lines[i].Text, exp.text)
		}
	}
}

func TestParseLRC_MultipleTimestamps(t *testing.T) {
	// Same text with multiple timestamps (chorus repeat)
	lrc := `[00:30.00][01:30.00][02:30.00]Chorus line`

	lyrics, err := ParseLRC(strings.NewReader(lrc))
	if err != nil {
		t.Fatalf("ParseLRC error: %v", err)
	}

	if len(lyrics.Lines) != 3 {
		t.Fatalf("len(Lines) = %d, want 3", len(lyrics.Lines))
	}

	// All three should have the same text
	for i, line := range lyrics.Lines {
		if line.Text != "Chorus line" {
			t.Errorf("Lines[%d].Text = %q, want %q", i, line.Text, "Chorus line")
		}
	}

	// Should be sorted by time
	if lyrics.Lines[0].Time != 30*time.Second {
		t.Errorf("Lines[0].Time = %v, want 30s", lyrics.Lines[0].Time)
	}
	if lyrics.Lines[1].Time != 90*time.Second {
		t.Errorf("Lines[1].Time = %v, want 90s", lyrics.Lines[1].Time)
	}
	if lyrics.Lines[2].Time != 150*time.Second {
		t.Errorf("Lines[2].Time = %v, want 150s", lyrics.Lines[2].Time)
	}
}

func TestParseLRC_VariousFormats(t *testing.T) {
	// Test different timestamp formats
	lrc := `[00:10]No decimal
[00:20.5]One digit decimal
[00:30.50]Two digit decimal
[00:40.500]Three digit decimal
[01:00:00]Colon separator`

	lyrics, err := ParseLRC(strings.NewReader(lrc))
	if err != nil {
		t.Fatalf("ParseLRC error: %v", err)
	}

	if len(lyrics.Lines) != 5 {
		t.Fatalf("len(Lines) = %d, want 5", len(lyrics.Lines))
	}

	// Check first line (no decimal)
	if lyrics.Lines[0].Time != 10*time.Second {
		t.Errorf("Lines[0].Time = %v, want 10s", lyrics.Lines[0].Time)
	}
}

func TestParseLRC_EmptyLines(t *testing.T) {
	lrc := `[00:10.00]First

[00:20.00]Second

[00:30.00]Third`

	lyrics, err := ParseLRC(strings.NewReader(lrc))
	if err != nil {
		t.Fatalf("ParseLRC error: %v", err)
	}

	if len(lyrics.Lines) != 3 {
		t.Fatalf("len(Lines) = %d, want 3", len(lyrics.Lines))
	}
}

func TestParseLRC_NoMetadata(t *testing.T) {
	lrc := `[00:10.00]Just lyrics
[00:20.00]No metadata`

	lyrics, err := ParseLRC(strings.NewReader(lrc))
	if err != nil {
		t.Fatalf("ParseLRC error: %v", err)
	}

	if lyrics.Artist != "" {
		t.Errorf("Artist = %q, want empty", lyrics.Artist)
	}
	if lyrics.Title != "" {
		t.Errorf("Title = %q, want empty", lyrics.Title)
	}
	if len(lyrics.Lines) != 2 {
		t.Fatalf("len(Lines) = %d, want 2", len(lyrics.Lines))
	}
}

func TestLyrics_LineAt(t *testing.T) {
	lyrics := &Lyrics{
		Lines: []Line{
			{Time: 10 * time.Second, Text: "First"},
			{Time: 20 * time.Second, Text: "Second"},
			{Time: 30 * time.Second, Text: "Third"},
		},
	}

	tests := []struct {
		pos  time.Duration
		want int
	}{
		{0, -1},               // Before any line
		{5 * time.Second, -1}, // Still before first line
		{10 * time.Second, 0}, // Exactly at first line
		{15 * time.Second, 0}, // Between first and second
		{20 * time.Second, 1}, // Exactly at second line
		{25 * time.Second, 1}, // Between second and third
		{30 * time.Second, 2}, // Exactly at third line
		{60 * time.Second, 2}, // After all lines
	}

	for _, tt := range tests {
		got := lyrics.LineAt(tt.pos)
		if got != tt.want {
			t.Errorf("LineAt(%v) = %d, want %d", tt.pos, got, tt.want)
		}
	}
}

func TestLyrics_LineAt_Empty(t *testing.T) {
	lyrics := &Lyrics{}
	if got := lyrics.LineAt(10 * time.Second); got != -1 {
		t.Errorf("LineAt on empty lyrics = %d, want -1", got)
	}
}
