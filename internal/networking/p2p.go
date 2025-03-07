package networking

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"obscure-fs-rebuild/internal/codec"
	"obscure-fs-rebuild/internal/storage"
	internalUtils "obscure-fs-rebuild/internal/utils"
	"obscure-fs-rebuild/utils"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

type Network struct {
	ctx            context.Context
	port           int
	host           host.Host
	dht            *dual.DHT
	bootstrapNodes []string
	fileStore      *storage.FileStore
}

func NewNetwork(ctx context.Context, port int, pkey string, bootstrapNodes []string, fs *storage.FileStore) *Network {
	var host host.Host
	addresses := libp2p.ListenAddrStrings(
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
		fmt.Sprintf("/ip6/::/tcp/%d", port),
	)

	privKey, err := LoadPrivateKey(pkey)
	if err == nil {
		host, err = libp2p.New(libp2p.Identity(privKey), addresses)
	} else {
		log.Printf("unable to load PKEY %v, error: %v\n", pkey, err)
		log.Println("generate new keypairs...")
		host, err = libp2p.New(addresses)
	}

	if err != nil {
		log.Fatalln(err)
	}

	// creating new Distributed Hash Table
	dhtInstance, err := dual.New(ctx, host, dual.DHTOption())
	if err != nil {
		log.Fatalln(err)
	}

	err = dhtInstance.Bootstrap(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Host created. Listening on: %s\n", host.Addrs())
	return &Network{
		ctx:            ctx,
		port:           port,
		host:           host,
		dht:            dhtInstance,
		bootstrapNodes: bootstrapNodes,
		fileStore:      fs,
	}
}

func (n *Network) GetHost() host.Host {
	return n.host
}

func (n *Network) FindPeer(peerID string) (peer.AddrInfo, error) {
	id, err := peer.Decode(peerID)
	if err != nil {
		return peer.AddrInfo{}, err
	}

	peerInfo, err := n.dht.FindPeer(n.ctx, id)
	if err != nil {
		return peer.AddrInfo{}, err
	}

	return peerInfo, nil
}

func (n *Network) AnnounceFile(id string) error {
	return n.dht.Provide(n.ctx, cid.MustParse(id), true)
}

func (n *Network) FindFile(id string) ([]peer.AddrInfo, error) {
	peerChan := n.dht.FindProvidersAsync(n.ctx, cid.MustParse(id), 10)
	peers := make([]peer.AddrInfo, 0)
	for p := range peerChan {
		peers = append(peers, p)
	}

	return peers, nil
}

func (n *Network) ShareFile(cid string, path string) (err error) {
	if err != nil {
		return
	}

	err = n.fileStore.StoreFile(cid, path)
	if err != nil {
		return
	}

	err = n.AnnounceFile(cid)
	if err != nil {
		return
	}

	log.Printf("File shared with CID: %s\n", cid)
	return nil
}

func (n *Network) ShareMetaData(meta *storage.Metadata) (err error) {

	err = n.fileStore.StoreMetadata(meta)
	if err != nil {
		return
	}

	// err = n.AnnounceFile(meta.Checksum)
	// if err != nil {
	// 	return
	// }

	// log.Printf("File shared with CID: %s\n", meta.Checksum)
	// return err
	return
}

func (n *Network) RetrieveFile(cid, outputPath string) error {
	meta, err := n.fileStore.GetMetaData(cid)
	if err != nil {
		log.Printf("meta data not found for ci: %s", cid)
		return fmt.Errorf("meta data not found for ci: %s", cid)
	}
	var Shards [][]byte
	for _, part := range meta.Parts {
		providers, err := n.FindFile(part)
		if err != nil || len(providers) == 0 {
			return fmt.Errorf("no providers found for CID: %s", cid)
		}

		fmt.Printf("providers: %v\n", providers)

		for _, provider := range providers {
			stream, err := n.host.NewStream(n.ctx, provider.ID, utils.ProtocolID)
			if err != nil {
				log.Printf("failed to open stream with provider: %s, error: %v\n", provider.ID.String(), err)
				continue
			}
			defer stream.Close()
			_, err = stream.Write([]byte(part))
			if err != nil {
				log.Printf("failed to send CID to provider: %s, error: %v\n", provider.ID.String(), err)
				continue
			}

			fileData, err := io.ReadAll(stream)
			if err != nil {
				log.Printf("Failed to read file data from provider: %s, error: %v\n", provider.ID.String(), err)
				continue
			}
			Shards = append(Shards, fileData)
			// err = os.WriteFile(outputPath, fileData, 0644)
			// if err != nil {
			// 	return fmt.Errorf("failed to save file to path: %s, error: %w", outputPath, err)
			// }
		}
	}
	if len(Shards) == 0 {
		return fmt.Errorf("failed to retrieve any valid shards for CID: %s", cid)
	}
	if err == nil {
		ec := codec.ErasureCodec{}
		outfile, err := ec.Decode(meta, Shards)
		if err != nil {
			panic(err)
		}
		return utils.CopyFile(outfile, outputPath)
	}

	// log.Printf("file not found locally! searching on the n/w for file: %s", cid)
	// providers, err := n.FindFile(cid)
	// if err != nil || len(providers) == 0 {
	// 	return fmt.Errorf("no providers found for CID: %s", cid)
	// }

	// fmt.Printf("providers: %v\n", providers)

	// for _, provider := range providers {
	// 	stream, err := n.host.NewStream(n.ctx, provider.ID, utils.ProtocolID)
	// 	if err != nil {
	// 		log.Printf("failed to open stream with provider: %s, error: %v\n", provider.ID.String(), err)
	// 		continue
	// 	}
	// 	defer stream.Close()

	// 	_, err = stream.Write([]byte(cid))
	// 	if err != nil {
	// 		log.Printf("failed to send CID to provider: %s, error: %v\n", provider.ID.String(), err)
	// 		continue
	// 	}

	// 	fileData, err := io.ReadAll(stream)
	// 	if err != nil {
	// 		log.Printf("Failed to read file data from provider: %s, error: %v\n", provider.ID.String(), err)
	// 		continue
	// 	}

	// 	err = os.WriteFile(outputPath, fileData, 0644)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to save file to path: %s, error: %w", outputPath, err)
	// 	}

	// 	log.Printf("file retrieved successfully and saved at: %s\n", outputPath)
	// }

	return nil
}

func (n *Network) ConnectToBootstrapNodes() {
	for _, addr := range n.bootstrapNodes {
		// skip self announcement
		if strings.Contains(addr, n.GetHost().ID().String()) {
			continue
		}

		multiAddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			log.Printf("Invalid bootstrap address: %s\n", addr)
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
		if err != nil {
			log.Printf("Failed to parse peer address: %s\n", addr)
			continue
		}

		if err := n.host.Connect(n.ctx, *peerInfo); err != nil {
			log.Printf("Failed to connect to bootstrap node: %s\n", addr)
		} else {
			log.Printf("Connected to bootstrap node: %s\n", addr)
		}
	}
}

func (n *Network) ConnectToPeer(addr string) (err error) {
	mulAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(mulAddr)
	if err != nil {
		return
	}

	n.host.Peerstore().AddAddr(peerInfo.ID, mulAddr, peerstore.PermanentAddrTTL)
	if err = n.host.Connect(n.ctx, *peerInfo); err != nil {
		return
	}

	log.Printf("connected to peer: %s\n", addr)
	return nil
}

func (n *Network) AnnounceToPeers(nodeID, address string) {
	var p = 1
	peers := n.host.Peerstore().Peers()
	for _, peerID := range peers {

		// FIXME : skip self announcement & port issue
		if peerID == n.GetHost().ID() {
			continue
		}

		peerAddr := fmt.Sprintf("http://localhost:400%d/nodes/register", p)
		p++
		body := Node{
			ID:       nodeID,
			Address:  address,
			IsOnline: true,
		}

		jsonData, _ := json.Marshal(body)
		resp, err := http.Post(peerAddr, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Failed to announce to peer %s: %v\n", peerID, err)
		} else {
			log.Printf("Node announced to peer %s: %s\n", peerID, resp.Status)
		}
	}
}

func (n *Network) StartSimpleProtocol(protocolID protocol.ID) {
	n.host.SetStreamHandler(protocolID, streamHandler(n, n.fileStore))
}

func (n *Network) SendMessage(peerID peer.ID, protocolID protocol.ID, msg string) (err error) {
	stream, err := n.host.NewStream(n.ctx, peerID, protocolID)
	if err != nil {
		return
	}
	defer stream.Close()

	_, err = stream.Write([]byte(msg))
	if err != nil {
		return err
	}

	log.Printf("Message sent: %s\n", msg)
	return nil
}

func streamHandler(net *Network, fileStore *storage.FileStore) network.StreamHandler {
	return func(stream network.Stream) {
		log.Println("new stream opened")
		defer stream.Close()

		buf := make([]byte, 256)
		n, err := stream.Read(buf)
		if err != nil {
			log.Printf("error reading from stream: %s\n", err)
			return
		}

		command := strings.TrimSpace(string(buf[:n]))
		log.Printf("received command: %s\n", command)

		switch command {
		case "list_files":
			files := fileStore.ListFiles()
			response, err := json.Marshal(files)
			if err != nil {
				log.Printf("failed to encode file list: %s\n", err)
				return
			}
			_, err = stream.Write(response)
			if err != nil {
				log.Printf("error writing file list to stream: %s\n", err)
			} else {
				log.Println("file list sent successfully")
			}

		case "send_file":
			// Receiving file logic
			reader := bufio.NewReader(stream)

			// Read JSON metadata from stream
			jsonData, err := reader.ReadBytes('\n') // Assumes sender adds a newline at the end
			if err != nil {
				log.Printf("error reading file metadata from stream: %s\n", err)
				return
			}

			var data map[string]string
			if err := json.Unmarshal(jsonData, &data); err != nil {
				log.Printf("error unmarshaling file metadata: %s\n", err)
				return
			}

			// Decode Base64 file data
			fileData, err := base64.StdEncoding.DecodeString(data["value"])
			if err != nil {
				log.Printf("error decoding base64 file data: %s\n", err)
				return
			}

			// Save the file
			uploadDir := fmt.Sprintf("./uploads/%s", net.GetHost().ID().String())

			// Ensure the directory exists
			if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
				log.Printf("error creating directory: %s\n", err)
				return
			}

			filePath := fmt.Sprintf("%s/%s", uploadDir, data["key"])

			// Write the file after ensuring the directory exists
			if err := os.WriteFile(filePath, fileData, 0644); err != nil {
				log.Printf("error writing received file: %s\n", err)
				return
			}

			err = net.ShareFile(data["key"], filePath)
			log.Printf("File received and saved as %s\n", data["key"])

		default:
			cid := command
			filePath, err := fileStore.GetFile(cid)
			if err != nil {
				log.Printf("file not found for CID: %s\n", cid)
				return
			}

			// ec := codec.ErasureCodec{}
			// outfile, err := ec.Decode(meta)
			// if err != nil {
			// 	panic(err)
			// }
			fileData, err := os.ReadFile(filePath)
			if err != nil {
				log.Printf("failed to read file: %s\n", err)
				return
			}

			_, err = stream.Write(fileData)
			if err != nil {
				log.Printf("error writing file to stream: %s\n", err)
			} else {
				log.Printf("file sent successfully for CID: %s\n", cid)
			}
		}
	}
}

func (n *Network) Shutdown() error {
	log.Println("Shutting down host...")
	log.Println("Saving file store before shutting down...")

	filepath := fmt.Sprintf(internalUtils.BackupPath, n.GetHost().ID().String())
	if err := n.fileStore.SaveToFile(filepath); err != nil {
		log.Println("Error saving file store: %v", err)
		return err
	}
	return n.GetHost().Close()
}

func LoadPrivateKey(path string) (crypto.PrivKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(keyBytes)
	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	pkey, _, err := crypto.KeyPairFromStdKey(privKey)
	if err != nil {
		return nil, err
	}

	return pkey, nil
}

func (n *Network) ReAnnounceFiles() {
	cid := n.fileStore.GetFileCid()
	for _, id := range cid {
		fmt.Printf("Re-announcing file: %s\n", id)
		if err := n.AnnounceFile(id); err != nil {
			fmt.Printf("Error announcing file %s: %v\n", id, err)
		}
	}
}

func (n *Network) LodeBackupFileStore() {
	err := n.fileStore.LoadFromFile(fmt.Sprintf(internalUtils.BackupPath, n.GetHost().ID().String()))
	if err != nil {
		log.Println("No existing file store found, starting fresh.")
	} else {
		log.Println("Loading existing file store...")
	}
}

func (n *Network) SendFileInStream(s network.Stream, fileId string, fileData []byte) error {
	writer := bufio.NewWriter(s)

	// Step 1: Send "send_file" command first
	_, err := writer.WriteString("send_file\n") // Ensure newline for proper reading
	if err != nil {
		return err
	}
	err = writer.Flush() // Flush to ensure the command is sent separately
	if err != nil {
		return err
	}

	// Step 2: Create a key-value map for file metadata
	data := map[string]string{
		"key":   fileId,
		"value": base64.StdEncoding.EncodeToString(fileData),
	}

	// Step 3: Serialize to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Step 4: Send JSON file metadata
	_, err = writer.Write(jsonData)
	if err != nil {
		return err
	}

	// Step 5: Append a newline to mark the end of JSON data
	_, err = writer.WriteString("\n")
	if err != nil {
		return err
	}

	// Step 6: Flush the writer to ensure all data is sent
	return writer.Flush()
}

func getRandomNumber(length int) int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(length)
}

func (n *Network) getPeersWithoutSelf() []peer.ID {
	var filteredPeers []peer.ID
	selfID := n.GetHost().ID()

	for _, p := range n.host.Peerstore().Peers() {
		if p != selfID {
			filteredPeers = append(filteredPeers, p)
		}
	}

	return filteredPeers
}

func (n *Network) ShareFileToPeers(data map[string][]byte) {
	peers := n.getPeersWithoutSelf()

	for fileId, file := range data {
		// FIXME: change a better strategy to choose a peer to save
		ranNum := getRandomNumber(len(peers))
		peerID := peers[ranNum%len(peers)]
		stream, err := n.host.NewStream(n.ctx, peerID, utils.ProtocolID)
		if err != nil {
			log.Printf("Failed to open stream to peer %s: %v\n", peerID, err)
			continue
		}

		err = n.SendFileInStream(stream, fileId, file)
		if err != nil {
			log.Printf("Failed to send %s: %v\n", file, err)
		}
		stream.Close()

	}
	return
}
