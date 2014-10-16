package filesystem

import (
	"io"
	"os"
)

type MemoryFile struct {
	data []byte
	name string
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (f *MemoryFile) ReadAt(b []byte, off int64) (int, error) {
	fsize := int64(len(f.data))
	bsize := int64(len(b))

	if off < 0 {
		return -1, os.ErrInvalid
	}
	if off >= fsize {
		return 0, io.EOF
	}
	end := min(off+bsize, fsize)
	return copy(b, f.data[off:end]), nil
}

func (f *MemoryFile) WriteAt(b []byte, off int64) (int, error) {
	fsize := int64(len(f.data))
	bsize := int64(len(b))

	if off < 0 {
		return -1, os.ErrInvalid
	}
	if off+bsize > fsize {
		// expand the memory file or override the end of it
		f.data = append(f.data[:off], b...)
	} else {
		// override part of the memory file
		f.data = append(f.data[:off], append(b, f.data[off+bsize:]...)...)
	}
	return int(bsize), nil
}

func (f *MemoryFile) Name() string { return f.name }
func (f *MemoryFile) IsDir() bool  { return false }
func (f *MemoryFile) Sync() error  { return nil }
func (f *MemoryFile) Close() error { return nil }
