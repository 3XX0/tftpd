package filesystem

import (
	"os"
	"sync"
)

type Root struct {
	sync.RWMutex
	files map[string]File
}

type File interface {
	ReadAt([]byte, int64) (int, error)
	WriteAt([]byte, int64) (int, error)
	Name() string
	IsDir() bool
	Sync() error
	Close() error
}

// Allocates a new root filesystem.
func New() *Root {
	return &Root{
		files: make(map[string]File),
	}
}

// Creates an in-memory file.
// Note that it overrides the file if it already exists.
func (r *Root) CreateMemoryFile(path string) File {
	r.Lock()
	delete(r.files, path)
	r.Unlock()

	return &MemoryFile{
		data: make([]byte, 0),
		name: path,
	}
}

// Open a file present in the filesystem.
func (r *Root) Open(name string) (File, error) {
	r.RLock()
	defer r.RUnlock()
	if f := r.files[name]; f != nil {
		return f, nil
	}
	return nil, os.ErrNotExist
}

// Save a file in the filesystem.
func (r *Root) Save(file File) {
	r.Lock()
	r.files[file.Name()] = file
	r.Unlock()
}
