package hashing

import (
	"crypto/sha256"
	"io"
	"os"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Calculate SHA256 hash
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	hash := hasher.Sum(nil)

	// Create multihash
	mh, err := multihash.Encode(hash, multihash.SHA2_256)
	if err != nil {
		return "", err
	}

	// Create CID
	c := cid.NewCidV1(cid.Raw, mh)

	return c.String(), nil
}
