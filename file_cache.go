package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type fileCache struct {
	data      map[string]fileCacheData
	directory string
}

type fileCacheData struct {
	md5    string
	synced bool
}

type fileDetails struct {
	md5  string
	name string
}

// Returns a filecached with files scanned
func createFileCache(directory string) fileCache {
	fc := fileCache{directory: directory, data: map[string]fileCacheData{}}

	c, err := os.ReadDir(directory)
	if err != nil {
		panic(errors.Join(errors.New("Failed to open directory"), err))
	}

	for _, entry := range c {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		f, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			slog.Error("Failed to open file", "error", err)
			panic("File go boom") // TODO
		}

		h := md5.New()
		if _, err := io.Copy(h, f); err != nil {
			slog.Error("Failed to hash file", "error", err)
		}

		hash := hex.EncodeToString(h.Sum(nil))
		fc.data[entry.Name()] = fileCacheData{md5: hash, synced: false}
		f.Close()
	}
	return fc
}

// Iterates through a directory sending back fileDetails into the channel until it is funished
// walking the dir (level only 1)
func getFileDetails(directory string, ch chan<- fileDetails) {
	defer close(ch)

	c, err := os.ReadDir(directory)
	if err != nil {
		panic(errors.Join(errors.New("Failed to open directory"), err))
	}

	for _, entry := range c {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		f, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			slog.Error("Failed to open file", "error", err)
			panic("File go boom") // TODO
		}

		h := md5.New()
		if _, err := io.Copy(h, f); err != nil {
			slog.Error("Failed to hash file", "error", err)
		}

		hash := hex.EncodeToString(h.Sum(nil))
		ch <- fileDetails{md5: hash, name: entry.Name()}

		f.Close()
	}
}
