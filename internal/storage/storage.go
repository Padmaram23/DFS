package storage

import (
	"fmt"
	"os"
	"sync"
)

type FileStore struct {
	files map[string]string
	mu    sync.RWMutex
}

func NewFileStore() *FileStore {
	return &FileStore{
		files: make(map[string]string),
		mu:    sync.RWMutex{},
	}
}

func (fs *FileStore) StoreFile(cid string, path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.files[cid] = path
	return nil
}

func (fs *FileStore) GetFile(cid string) (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path, exists := fs.files[cid]
	if !exists {
		return "", fmt.Errorf("file not found for CID: %s", cid)
	}
	return path, nil
}

func (fs *FileStore) ListFiles() map[string]string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	// Return a copy of the map to prevent modification by callers.
	copy := make(map[string]string, len(fs.files))
	for k, v := range fs.files {
		copy[k] = v
	}
	return copy
}

func GetFileSize(path string) (int64, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}
