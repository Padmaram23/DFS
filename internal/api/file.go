package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"obscure-fs-rebuild/internal/codec"
	"obscure-fs-rebuild/internal/hashing"
	"obscure-fs-rebuild/internal/storage"
	"obscure-fs-rebuild/utils"

	"github.com/gin-gonic/gin"
)

func (nc *NodeController) FileUploadsHandler(c *gin.Context, nodeId string) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upload file"})
		return
	}
	// uploadDir := fmt.Sprintf("./uploads/%s", nodeId)
	// filePath := fmt.Sprintf("%s/%s", uploadDir, file.Filename)
	// if err := c.SaveUploadedFile(file, filePath); err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
	// 	return
	// }

	// cid, err := nc.network.ShareFile(filePath)
	// buf, _ := storage.ReadFile(filePath)

	// read the file without saving
	fileData, err := file.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to open file"})
		return
	}
	defer fileData.Close()

	buf, _ := io.ReadAll(fileData)
	hash, _ := hashing.HashData(bytes.NewReader(buf))
	// hash, _ := hashing.HashFile(filePath)
	metadata := &storage.Metadata{
		Name:     file.Filename,
		Checksum: hash,
	}
	ec := codec.ErasureCodec{}
	data, err := ec.Encode(metadata, buf)
	if err != nil {
		panic(err)
	}
	// err = nc.network.ShareFile(metadata)
	nc.network.ShareFileToPeers(data)
	if err != nil {
		log.Printf("error: %s \n", err)
	}

	log.Printf("file uploaded:CID: %s\n", hash)
	nc.network.ShareMetaData(metadata)
	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully", "cid": hash})
}

func (nc *NodeController) GetFileHandler(c *gin.Context) {
	cid := c.Param("cid")

	tempDir := fmt.Sprintf("./temp/%s", nc.network.GetHost().ID())
	tempFilePath := fmt.Sprintf("%s/%s", tempDir, cid)

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
		return
	}

	err := nc.network.RetrieveFile(cid, tempFilePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	log.Printf("tempFilePath: %s /n", tempFilePath)
	c.File(tempFilePath)
}

func (n *NodeController) GetFilesHandler(c *gin.Context) {
	localFiles := n.store.ListFiles()

	response := gin.H{
		"local_files":   localFiles,
		"network_files": []gin.H{},
	}

	// Fetch files available on the network
	networkPeers := n.network.GetHost().Peerstore().Peers()
	for _, peerID := range networkPeers {
		if peerID == n.network.GetHost().ID() {
			continue
		}

		stream, err := n.network.GetHost().NewStream(n.ctx, peerID, utils.ProtocolID)
		if err != nil {
			log.Printf("Failed to create stream to peer %s: %v\n", peerID, err)
			continue
		}
		defer stream.Close()

		_, err = stream.Write([]byte("list_files")) // Protocol to list files
		if err != nil {
			log.Printf("Failed to request files from peer %s: %v\n", peerID, err)
			continue
		}

		// Read response
		peerFileData, err := io.ReadAll(stream)
		if err != nil {
			log.Printf("Failed to read files from peer %s: %v\n", peerID, err)
			continue
		}

		var peerFiles map[string]string
		if err := json.Unmarshal(peerFileData, &peerFiles); err != nil {
			log.Printf("Failed to decode files from peer %s: %v\n", peerID, err)
			continue
		}

		response["network_files"] = append(response["network_files"].([]gin.H), gin.H{
			"peer_id": peerID.String(),
			"files":   peerFiles,
		})
	}

	c.JSON(http.StatusOK, response)
}
