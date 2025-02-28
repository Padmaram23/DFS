package cmd

import (
	"context"
	"log"
	"os"

	"obscure-fs-rebuild/internal/networking"
	"obscure-fs-rebuild/internal/storage"

	"github.com/spf13/cobra"
)

var (
	rootCmd    = &cobra.Command{Use: "obscure-fs"}
	network    *networking.Network // Shared network instance
	store      *storage.FileStore  // Shared storage instance
	registry   *networking.NodeRegistry
	ctx        = context.Background()
	listenPort int
	apiPort    int
	pkey       string

	bootstrapNodes = []string{
		"/ip4/127.0.0.1/tcp/9090/p2p/QmR3nBwr1XLjpNqxTPhngV9auQGrsEjoWEdfC8UwKTX8bS",
		"/ip4/127.0.0.1/tcp/9091/p2p/QmezUhAv3bfTJRtkZcsuiJd5D8DZBUNpiWM9eBwVw9YjVB",
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().IntVar(&listenPort, "port", 0, "Port to listen on")
	rootCmd.PersistentFlags().IntVar(&apiPort, "api-port", 8080, "Port for the REST API")
	rootCmd.PersistentFlags().StringVar(&pkey, "pkey", "", "Private key path")

	rootCmd.MarkPersistentFlagRequired("port")
	rootCmd.MarkPersistentFlagRequired("api-port")
	rootCmd.MarkPersistentFlagRequired("pkey")
}
