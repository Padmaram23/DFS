package tests

import (
	"os"
	"path/filepath"
	"testing"

	"obscure-fs-rebuild/internal/codec"
	"obscure-fs-rebuild/internal/hashing"
	"obscure-fs-rebuild/internal/storage"
	"obscure-fs-rebuild/internal/utils"

	"github.com/stretchr/testify/assert"
)

func TestCodec(t *testing.T) {
	filePath := "../README.md"
	fileName := filepath.Base(filePath)
	buf, _ := storage.ReadFile(filePath)

	hash, _ := hashing.HashFile(filePath)
	metadata := &storage.Metadata{
		Name:     fileName,
		Checksum: hash,
	}

	ec := codec.ErasureCodec{}
	_, err := ec.Encode(metadata, buf)
	if err != nil {
		panic(err)
	}

	outfile, err := ec.Decode(metadata)
	if err != nil {
		panic(err)
	}

	hash, err = hashing.HashFile(outfile)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, utils.Shards, metadata.Shards)
	assert.Equal(t, utils.Parity, metadata.Parity)
	assert.Equal(t, metadata.Checksum, hash)

	os.RemoveAll(filepath.Dir(outfile))
}
