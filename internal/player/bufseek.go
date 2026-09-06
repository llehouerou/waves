package player

import (
	"errors"
	"io"
	"os"
)

// bufBlock is the read-ahead block size. Decoders pull frame-sized reads
// (~265 bytes on MP3); unbuffered that is one syscall — one network round
// trip on a remote mount — per frame. See issue #36.
const bufBlock = 256 * 1024

type sourceFile interface {
	io.ReaderAt
	io.Closer
	Stat() (os.FileInfo, error)
}

// bufferedFile is a seekable read-ahead buffer over a file: it caches one
// aligned block, so the decoders' small reads and short backward seeks are
// served from memory. bufio.Reader can't be used, it loses Seek and re-reads
// the whole file on every backward seek.
type bufferedFile struct {
	f     sourceFile
	buf   []byte
	start int64 // file offset of buf[0]
	n     int   // valid bytes in buf
	pos   int64 // logical read offset
	size  int64
}

func newBufferedFile(f sourceFile) (*bufferedFile, error) {
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &bufferedFile{f: f, buf: make([]byte, bufBlock), size: st.Size()}, nil
}

func (b *bufferedFile) Read(p []byte) (int, error) {
	if b.pos >= b.size {
		return 0, io.EOF
	}
	if b.pos < b.start || b.pos >= b.start+int64(b.n) {
		if err := b.fill(b.pos); err != nil {
			return 0, err
		}
	}
	n := copy(p, b.buf[b.pos-b.start:b.n])
	b.pos += int64(n)
	return n, nil
}

// fill loads the aligned block containing off.
func (b *bufferedFile) fill(off int64) error {
	start := off - off%bufBlock
	n, err := b.f.ReadAt(b.buf, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	b.start, b.n = start, n
	if n == 0 {
		return io.EOF
	}
	return nil
}

func (b *bufferedFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		offset += b.pos
	case io.SeekEnd:
		offset += b.size
	}
	if offset < 0 {
		return 0, os.ErrInvalid
	}
	b.pos = offset
	return offset, nil
}

func (b *bufferedFile) Close() error { return b.f.Close() }
