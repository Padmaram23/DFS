package storage

import "os"

type StoreMetadata struct {
	Name string
	Path string
}

type Metadata struct {
	Name     string
	Shards   int
	Pairty   int
	Checksum string
	Parts    []string
}

func (m Metadata) GetShardSum() int {
	return m.Pairty + m.Shards
}

func ReadFile(path string) ([]byte, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
