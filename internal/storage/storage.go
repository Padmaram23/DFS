package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type FileStore struct {
	files    map[string]string
	mu       sync.RWMutex
	metadata map[string]*Metadata
}

func NewFileStore() *FileStore {
	return &FileStore{
		files:    make(map[string]string),
		mu:       sync.RWMutex{},
		metadata: make(map[string]*Metadata),
	}
}

func (fs *FileStore) StoreFile(cid string, path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.files[cid] = path
	return nil
}

func (fs *FileStore) StoreMetadata(meta *Metadata) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.metadata[meta.Checksum] = meta
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

func (fs *FileStore) GetMetaData(cid string) (*Metadata, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	meta, exists := fs.metadata[cid]
	if !exists {
		return nil, fmt.Errorf("file not found for CID: %s", cid)
	}

	return meta, nil
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

func (fs *FileStore) ListMetaData() map[string]Metadata {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	// Return a copy of the map to prevent modification by callers.
	copy := make(map[string]Metadata, len(fs.metadata))
	for k, v := range fs.metadata {
		copy[k] = *v
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

func (fs *FileStore) SaveToFile(filePath string) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	fsData := map[string]interface{}{
		"files":    fs.files,
		"metaData": fs.metadata,
	}
	data, err := json.Marshal(fsData)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func (fs *FileStore) LoadFromFile(filePath string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Temporary structure to hold JSON data
	var fsData struct {
		Files    map[string]string
		MetaData map[string]*Metadata
	}

	// Unmarshal JSON into temporary struct
	if err := json.Unmarshal(data, &fsData); err != nil {
		return err
	}

	// Assign values back to FileStore
	fs.files = fsData.Files
	fs.metadata = fsData.MetaData

	return nil
}

func (fs *FileStore) GetFileCid() []string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	announcedIDs := make([]string, 0, len(fs.files))

	for id := range fs.files {
		announcedIDs = append(announcedIDs, id)
	}

	return announcedIDs
}
