package api

import (
	"context"
	"net/http"

	"obscure-fs-rebuild/internal/networking"
	"obscure-fs-rebuild/internal/storage"

	"github.com/gin-gonic/gin"
)

type NodeController struct {
	ctx      context.Context
	store    *storage.FileStore
	registry *networking.NodeRegistry
	network  *networking.Network
}

func NewNodeController(ctx context.Context, store *storage.FileStore, registry *networking.NodeRegistry, network *networking.Network) *NodeController {
	return &NodeController{
		ctx:      ctx,
		store:    store,
		registry: registry,
		network:  network,
	}
}

func (nc *NodeController) RegisterNodeHandler(c *gin.Context) {
	var node networking.Node
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	nc.registry.RegisterNode(node)
	c.JSON(http.StatusOK, gin.H{"message": "Node registered successfully", "node_id": node.ID})
}

func (nc *NodeController) GetAllNodesHandler(c *gin.Context) {
	nodes := nc.registry.GetAllNodes()
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}
