package networking

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"obscure-fs-rebuild/internal/hashing"
	"obscure-fs-rebuild/internal/storage"
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

func (n *Network) ShareFile(path string) (cid string, err error) {
	cid, err = hashing.HashFile(path)
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
	return cid, nil
}

func (n *Network) RetrieveFile(cid, outputPath string) error {
	path, err := n.fileStore.GetFile(cid)
	if err == nil {
		return utils.CopyFile(path, outputPath)
	}

	log.Printf("file not found locally! searching on the n/w for file: %s", cid)
	providers, err := n.FindFile(cid)
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

		_, err = stream.Write([]byte(cid))
		if err != nil {
			log.Printf("failed to send CID to provider: %s, error: %v\n", provider.ID.String(), err)
			continue
		}

		fileData, err := io.ReadAll(stream)
		if err != nil {
			log.Printf("Failed to read file data from provider: %s, error: %v\n", provider.ID.String(), err)
			continue
		}

		err = os.WriteFile(outputPath, fileData, 0644)
		if err != nil {
			return fmt.Errorf("failed to save file to path: %s, error: %w", outputPath, err)
		}

		log.Printf("file retrieved successfully and saved at: %s\n", outputPath)
	}

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
	n.host.SetStreamHandler(protocolID, streamHandler(n.fileStore))
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

func streamHandler(fileStore *storage.FileStore) network.StreamHandler {
	return func(stream network.Stream) {
		log.Println("new stream opened")
		defer stream.Close()

		buf := make([]byte, 256)
		n, err := stream.Read(buf)
		if err != nil {
			log.Printf("error reading from stream: %s\n", err)
			return
		}

		command := string(buf[:n])
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

		default:
			cid := command
			path, err := fileStore.GetFile(cid)
			if err != nil {
				log.Printf("file not found for CID: %s\n", cid)
				return
			}

			fileData, err := os.ReadFile(path)
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
