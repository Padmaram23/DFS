package networking

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

func (n *Network) RequestFile(peerID peer.ID, protocolID protocol.ID, cid string) error {
	stream, err := n.host.NewStream(n.ctx, peerID, protocolID)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	_, err = stream.Write([]byte(fmt.Sprintf("REQ:%s\n", cid)))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	reader := bufio.NewReader(stream)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if strings.HasPrefix(response, "ERR:") {
		return fmt.Errorf("error from peer: %s", strings.TrimPrefix(response, "ERR:"))
	}

	log.Printf("File transfer started for CID: %s\n", cid)
	file, err := os.Create(fmt.Sprintf("downloaded_%s", cid))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	log.Printf("File successfully downloaded for CID: %s\n", cid)
	return nil
}
