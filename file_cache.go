package filesyncer

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type FileCache struct {
	data      map[string]fileCacheData
	directory string
}

type fileCacheData struct {
	md5    string
	synced bool
}

// Returns a filecached with files scanned
func CreateFileCache(directory string) (*FileCache, error) {
	fc := FileCache{directory: directory, data: map[string]fileCacheData{}}

	c, err := os.ReadDir(directory)
	if err != nil {
		return nil, errors.Join(errors.New("Failed to open directory"), err)
	}

	for _, entry := range c {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		f, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			slog.Error("Failed to open file", "error", err)
			return nil, fmt.Errorf("Failed to open file %s: %w", entry.Name(), err)
		}

		h := md5.New()
		if _, err := io.Copy(h, f); err != nil {
			slog.Error("Failed to hash file", "error", err)
			f.Close()
			return nil, fmt.Errorf("Failed to hash file %s: %w", entry.Name(), err)
		}

		hash := hex.EncodeToString(h.Sum(nil))
		fc.data[entry.Name()] = fileCacheData{md5: hash, synced: false}
		f.Close()
	}
	return &fc, nil
}
