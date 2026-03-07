package logging

import (
	"fmt"
	"os"
	"sync"
)

// RotatingWriter is an io.Writer that writes to a file and rotates it
// when it exceeds maxSize bytes, keeping up to maxFiles old copies.
type RotatingWriter struct {
	mu       sync.Mutex
	path     string
	maxSize  int64
	maxFiles int
	file     *os.File
	size     int64
}

// NewRotatingWriter opens (or creates) the log file at path and returns
// a writer that rotates when the file exceeds maxSize.
func NewRotatingWriter(path string, maxSize int64, maxFiles int) (*RotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat log file: %w", err)
	}

	return &RotatingWriter{
		path:     path,
		maxSize:  maxSize,
		maxFiles: maxFiles,
		file:     f,
		size:     info.Size(),
	}, nil
}

func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotatingWriter) rotate() error {
	w.file.Close()

	// Shift existing rotated files: .2 -> .3, .1 -> .2, etc.
	for i := w.maxFiles - 1; i > 0; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i)
		dst := fmt.Sprintf("%s.%d", w.path, i+1)
		os.Rename(src, dst) // ignore errors — files may not exist
	}

	// Current file becomes .1
	os.Rename(w.path, fmt.Sprintf("%s.1", w.path))

	// Open a fresh file.
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("creating new log file: %w", err)
	}
	w.file = f
	w.size = 0
	return nil
}

// Close closes the underlying file.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
