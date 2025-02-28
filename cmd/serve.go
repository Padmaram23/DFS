package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"obscure-fs-rebuild/internal/api"
	"obscure-fs-rebuild/internal/networking"
	"obscure-fs-rebuild/internal/storage"
	"obscure-fs-rebuild/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the node and start listening for connections",
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting the node...")

		if store == nil {
			log.Println("Initializing file store...")
			store = storage.NewFileStore()
			log.Println("Sucessfully initialzied file store...")
		}

		if network == nil {
			log.Println("Initializing network...")
			if listenPort <= 0 {
				log.Fatalf("Invalid port: %d\n", listenPort)
			}
			network = networking.NewNetwork(ctx, listenPort, pkey, bootstrapNodes, store)
			network.StartSimpleProtocol(utils.ProtocolID)
			log.Printf("Node ID: %s\n", network.GetHost().ID().String())
			network.ConnectToBootstrapNodes()
			network.AnnounceToPeers(network.GetHost().ID().String(), fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort))
		}

		if registry == nil {
			registry = networking.NewNodeRegistry()
		}

		log.Printf("Node is listening on port %d. Press Ctrl+C to stop.\n", listenPort)

		router := gin.Default()
		router.Use(func(c *gin.Context) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			c.Next()
		})

		nodeController := api.NewNodeController(ctx, store, registry, network)

		nodes := router.Group("/nodes")
		nodes.POST("/register", nodeController.RegisterNodeHandler)
		nodes.GET("/", nodeController.GetAllNodesHandler)

		files := router.Group("/files")
		files.GET("/", nodeController.GetFilesHandler)
		files.POST("/upload", nodeController.FileUploadsHandler)
		files.GET("/:cid", nodeController.GetFileHandler)

		go func() {
			if err := router.Run(fmt.Sprintf(":%d", apiPort)); err != nil {
				log.Fatalf("Failed to start HTTP server: %v", err)
			}
		}()

		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, os.Interrupt)
		<-shutdown

		log.Println("Shutting down...")
		if err := network.Shutdown(); err != nil {
			log.Printf("Failed to shut down network: %v", err)
		}
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
