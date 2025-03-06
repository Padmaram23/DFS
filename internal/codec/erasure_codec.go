package codec

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"

	"obscure-fs-rebuild/internal/hashing"
	"obscure-fs-rebuild/internal/storage"
	"obscure-fs-rebuild/internal/utils"

	"github.com/klauspost/reedsolomon"
)

type Codec interface {
	Encode(metadata *storage.Metadata, src []byte) error
	Decode(metadata *storage.Metadata) error
}

type ErasureCodec struct{}

func (ErasureCodec) Encode(metadata *storage.Metadata, src []byte) (shardsData map[string][]byte, err error) {
	log.Println("beginning encoding with default configs..")
	log.Printf("shard size : %v\n", utils.Shards)
	log.Printf("parity size: %v\n", utils.Parity)

	if utils.Shards+utils.Parity > 256 {
		return nil, errors.New("sum of shard & parity cannot be > 256")
	}

	enc, err := reedsolomon.New(utils.Shards, utils.Parity)
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

	// basePath := fmt.Sprintf("%s/%s", utils.StoragePath, metadata.Checksum)
	// err = os.MkdirAll(basePath, 0755)
	// if err != nil && !errors.Is(err, os.ErrExist) {
	// 	return
	// }

	metadata.Parts = make([]string, len(shards))
	shardsData = make(map[string][]byte)
	for i, shard := range shards {
		shardFileName, _ := hashing.HashData(bytes.NewReader(shard))

		// updating metadata
		metadata.Parts[i] = shardFileName
		shardsData[shardFileName] = shard

		// _, err = os.Stat(shardFileName)
		// if err == nil {
		// 	log.Printf("chunk exists skipping: %s.%d\n", metadata.Checksum, i)
		// 	continue
		// }

		// log.Printf("saving chunk: %s.%d\n", metadata.Checksum, i)

		// err = os.WriteFile(shardFileName, shard, 0644)
		// if err != nil {
		// 	return shardsData, nil
		// }
	}

	// updating metadata
	metadata.Shards = utils.Shards
	metadata.Parity = utils.Parity

	return
}

func (ErasureCodec) Decode(metadata *storage.Metadata) (outfile string, err error) {
	log.Println("beginning decoding with default configs..")
	log.Printf("shard size : %v\n", metadata.Shards)
	log.Printf("parity size: %v\n", metadata.Parity)

	enc, _ := reedsolomon.New(metadata.Shards, metadata.Parity)

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

	log.Printf("file decoded & saved successfully : %s\n", outfile)

	return outfile, nil
}
