package tags

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
)

// A cover-like payload: JPEG magic bytes, enough for detectMimeType.
var testFrontCover = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x01}

// A back-cover-like payload: PNG magic bytes.
var testBackCover = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x02}

// mp3Pictures reads the attached pictures of the ID3v2 tag of path.
func mp3Pictures(t *testing.T, path string) []id3v2.PictureFrame {
	t.Helper()
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer tag.Close()
	frames := tag.GetFrames("APIC")
	pictures := make([]id3v2.PictureFrame, 0, len(frames))
	for _, f := range frames {
		pic, ok := f.(id3v2.PictureFrame)
		if !ok {
			t.Fatalf("APIC frame is %T, want id3v2.PictureFrame", f)
		}
		pictures = append(pictures, pic)
	}
	return pictures
}

// samePicture reports whether got carries want's picture: data, MIME type,
// picture type and description.
func samePicture(got, want id3v2.PictureFrame) bool {
	return bytes.Equal(got.Picture, want.Picture) &&
		got.MimeType == want.MimeType &&
		got.PictureType == want.PictureType &&
		got.Description == want.Description
}

func TestWriteMP3_NoNewCoverKeepsEmbeddedCover(t *testing.T) {
	path := createTestMP3(t, t.TempDir(), &Tag{Title: "Old", CoverArt: testFrontCover})

	if err := Write(path, &Tag{Title: "New"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	data, mimeType, err := ExtractEmbeddedArt(path)
	if err != nil {
		t.Fatalf("ExtractEmbeddedArt() error: %v", err)
	}
	if !bytes.Equal(data, testFrontCover) {
		t.Errorf("cover = %x, want %x", data, testFrontCover)
	}
	if mimeType != mimeJPEG {
		t.Errorf("mimeType = %q, want %q", mimeType, mimeJPEG)
	}
}

func TestWriteMP3_NoNewCoverKeepsEveryPicture(t *testing.T) {
	path := createTestMP3(t, t.TempDir(), nil)
	front := id3v2.PictureFrame{
		Encoding:    id3v2.EncodingISO,
		MimeType:    mimeJPEG,
		PictureType: id3v2.PTFrontCover,
		Description: "front",
		Picture:     testFrontCover,
	}
	back := id3v2.PictureFrame{
		Encoding:    id3v2.EncodingISO,
		MimeType:    mimePNG,
		PictureType: id3v2.PTBackCover,
		Description: "back",
		Picture:     testBackCover,
	}
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	tag.SetVersion(3)
	tag.AddAttachedPicture(front)
	tag.AddAttachedPicture(back)
	if err := tag.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	tag.Close()

	if err := Write(path, &Tag{Title: "New"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	got := mp3Pictures(t, path)
	if len(got) != 2 {
		t.Fatalf("got %d pictures, want 2: %+v", len(got), got)
	}
	for _, want := range []id3v2.PictureFrame{front, back} {
		found := false
		for _, pic := range got {
			found = found || samePicture(pic, want)
		}
		if !found {
			t.Errorf("picture %q lost, got %+v", want.Description, got)
		}
	}
}

func TestWriteMP3_NewCoverReplacesEveryPicture(t *testing.T) {
	path := createTestMP3(t, t.TempDir(), nil)
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, pt := range []byte{id3v2.PTFrontCover, id3v2.PTBackCover} {
		tag.AddAttachedPicture(id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    mimePNG,
			PictureType: pt,
			Description: "old",
			Picture:     testBackCover,
		})
	}
	if err := tag.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	tag.Close()

	if err := Write(path, &Tag{Title: "New", CoverArt: testFrontCover}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	got := mp3Pictures(t, path)
	want := id3v2.PictureFrame{
		MimeType:    mimeJPEG,
		PictureType: id3v2.PTFrontCover,
		Description: "Front Cover",
		Picture:     testFrontCover,
	}
	if len(got) != 1 || !samePicture(got[0], want) {
		t.Errorf("pictures = %+v, want only the new front cover", got)
	}
}

func TestWriteMP3_NoPictureStaysWithoutPicture(t *testing.T) {
	path := createTestMP3(t, t.TempDir(), &Tag{Title: "Old"})

	if err := Write(path, &Tag{Title: "New"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if got := mp3Pictures(t, path); len(got) != 0 {
		t.Errorf("pictures = %+v, want none", got)
	}
	data, _, err := ExtractEmbeddedArt(path)
	if err != nil {
		t.Fatalf("ExtractEmbeddedArt() error: %v", err)
	}
	if data != nil {
		t.Errorf("cover = %x, want none", data)
	}
}

// picBody is the body of an ID3v2.2 PIC frame holding a front cover: text
// encoding, image format, picture type, description, picture data.
func picBody(format, description string, cover []byte) []byte {
	body := append([]byte{0x00}, format...)
	body = append(body, 0x03)
	body = append(body, description...)
	body = append(body, 0x00)
	return append(body, cover...)
}

// writeID3v22MP3 creates an MP3 whose tag is ID3v2.2, holding one PIC frame
// with body, then padding: dhowden/tag reads no frame of a tag under 10 bytes.
func writeID3v22MP3(t *testing.T, path string, body []byte) {
	t.Helper()
	frame := append([]byte{'P', 'I', 'C', 0x00, 0x00, byte(len(body))}, body...)
	padding := make([]byte, 16)
	size := len(frame) + len(padding) // under 128 bytes: its syncsafe form is the byte itself
	audio := make([]byte, 417)
	audio[0], audio[1], audio[2] = 0xff, 0xfb, 0x90

	data := make([]byte, 0, 10+size+len(audio))
	data = append(data, 'I', 'D', '3', 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, byte(size))
	data = append(data, frame...)
	data = append(data, padding...)
	data = append(data, audio...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestWriteMP3_NoNewCoverKeepsID3v22Picture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.mp3")
	writeID3v22MP3(t, path, picBody("JPG", "cov", testFrontCover))
	before, _, err := ExtractEmbeddedArt(path)
	if err != nil {
		t.Fatalf("ExtractEmbeddedArt() before the write: %v", err)
	}
	if !bytes.Equal(before, testFrontCover) {
		t.Fatalf("cover before the write = %x, want %x", before, testFrontCover)
	}

	if err := Write(path, &Tag{Title: "New"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	after, _, err := ExtractEmbeddedArt(path)
	if err != nil {
		t.Fatalf("ExtractEmbeddedArt() after the write: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("cover after the write = %x, want %x", after, before)
	}
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer tag.Close()
	if tag.Version() != 4 {
		t.Errorf("tag version = %d, want 4", tag.Version())
	}
}

func TestWriteMP3_ID3v22PictureKeepsDescriptionAndImageFormat(t *testing.T) {
	gif := []byte{'G', 'I', 'F', '8', '9', 'a', 0x01}
	path := filepath.Join(t.TempDir(), "test.mp3")
	writeID3v22MP3(t, path, picBody("GIF", "scan", gif))

	if err := Write(path, &Tag{Title: "New"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	got := mp3Pictures(t, path)
	want := id3v2.PictureFrame{
		MimeType:    "image/gif",
		PictureType: id3v2.PTFrontCover,
		Description: "scan",
		Picture:     gif,
	}
	if len(got) != 1 || !samePicture(got[0], want) {
		t.Errorf("pictures = %+v, want %+v", got, want)
	}
}

func TestWriteMP3_UnreadableID3v22PictureIsDroppedWithTheTag(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cover []byte
		want  []byte
	}{
		{"no new cover", nil, nil},
		{"new cover", testFrontCover, testFrontCover},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.mp3")
			writeID3v22MP3(t, path, []byte{0x00, 'J', 'P'}) // too short a PIC
			if _, _, err := ExtractEmbeddedArt(path); err == nil {
				t.Fatal("ExtractEmbeddedArt() reads the PIC frame, want an error")
			}

			if err := Write(path, &Tag{Title: "New", CoverArt: tc.cover}); err != nil {
				t.Fatalf("Write() error: %v", err)
			}

			data, _, err := ExtractEmbeddedArt(path)
			if err != nil {
				t.Fatalf("ExtractEmbeddedArt() error: %v", err)
			}
			if !bytes.Equal(data, tc.want) {
				t.Errorf("cover = %x, want %x", data, tc.want)
			}
		})
	}
}
