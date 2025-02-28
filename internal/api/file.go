package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"obscure-fs-rebuild/utils"

	"github.com/gin-gonic/gin"
)

func (nc *NodeController) FileUploadsHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upload file"})
		return
	}

	uploadDir := "./uploads"
	filePath := fmt.Sprintf("%s/%s", uploadDir, file.Filename)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	cid, err := nc.network.ShareFile(filePath)
	if err != nil {
		return
	}

	log.Printf("file uploaded: %s (CID: %s)\n", filePath, cid)
	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully", "cid": cid})
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
