package codec

import (
	"errors"
	"fmt"
	"log"
	"os"

	"obscure-fs-rebuild/internal/storage"
	"obscure-fs-rebuild/internal/utils"

	"github.com/klauspost/reedsolomon"
)

type Codec interface {
	Encode(metadata *storage.Metadata, src []byte) error
	Decode(metadata *storage.Metadata) error
}

type ErasureCodec struct{}

func (ErasureCodec) Encode(metadata *storage.Metadata, src []byte) (err error) {
	log.Println("beginning encoding with default configs..")
	log.Printf("shard size : %v\n", utils.Shards)
	log.Printf("pairty size: %v\n", utils.Pairty)

	if utils.Shards+utils.Pairty > 256 {
		return errors.New("sum of shard & pairty cannot be > 256")
	}

	enc, err := reedsolomon.New(utils.Shards, utils.Pairty)
	if err != nil {
		return
	}

	shards, err := enc.Split(src)
	if err != nil {
		return
	}

	err = enc.Encode(shards)
	if err != nil {
		return
	}

	basePath := fmt.Sprintf("%s/%s", utils.StoragePath, metadata.Checksum)
	err = os.MkdirAll(basePath, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return
	}

	metadata.Parts = make([]string, len(shards))
	for i, shard := range shards {
		shardFileName := fmt.Sprintf("%s/%s.%d", basePath, metadata.Checksum, i)

		// updating metadata
		metadata.Parts[i] = shardFileName

		_, err = os.Stat(shardFileName)
		if err == nil {
			log.Printf("chunk exists skipping: %s.%d\n", metadata.Checksum, i)
			continue
		}

		log.Printf("saving chunk: %s.%d\n", metadata.Checksum, i)

		err = os.WriteFile(shardFileName, shard, 0644)
		if err != nil {
			return
		}
	}

	// updating metadata
	metadata.Shards = utils.Shards
	metadata.Pairty = utils.Pairty

	return
}

func (ErasureCodec) Decode(metadata *storage.Metadata) (outfile string, err error) {
	log.Println("beginning decoding with default configs..")
	log.Printf("shard size : %v\n", metadata.Shards)
	log.Printf("pairty size: %v\n", metadata.Pairty)

	enc, _ := reedsolomon.New(metadata.Shards, metadata.Pairty)

	shards := make([][]byte, metadata.GetShardSum())
	for i, part := range metadata.Parts {
		fmt.Printf("part: %v\n", part)
		shards[i], err = os.ReadFile(part)
		if err != nil {
			log.Printf("malformed shard: %s.%d\n", metadata.Checksum, i)
			shards[i] = nil
		}
	}

	// verify
	ok, _ := enc.Verify(shards)
	if ok {
		log.Println("reconstruction success!!!", metadata.Checksum)
	} else {

		// retry block
		log.Printf("unable to verify shard %s, trying to reconstruct...", metadata.Checksum)
		err = enc.Reconstruct(shards)
		if err != nil {
			log.Println("failed to reconstruct!", err)
			return
		}

		ok, _ = enc.Verify(shards)
		if ok {
			log.Println("reconstruction success!!!", metadata.Checksum)
		}
	}

	outfile = fmt.Sprintf("%s/%s/%s", utils.StoragePath, metadata.Checksum, metadata.Name)
	f, err := os.Create(outfile)
	if err != nil {
		return
	}

	err = enc.Join(f, shards, len(shards[0])*utils.Shards)
	if err != nil {
		return
	}

	log.Printf("file decoded & saved sucessfully : %s\n", outfile)

	return outfile, nil
}
