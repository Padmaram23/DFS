# ObscureFS

This project is a decentralized file-sharing network built using LibP2P. It allows nodes to share and retrieve files, discover other peers, and list available files for download.

## Features
- **File Sharing**: Share files using unique CIDs (Content Identifiers).
- **File Retrieval**: Retrieve files by their CIDs from the network.
- **File Listing**: List all files available on a node.
- **Peer-to-Peer Networking**: Connect to other peers using the LibP2P stack.
- **Custom Protocols**: Support for `list_files` and CID-based file retrieval commands.

## Getting Started

### Prerequisites
- Go 1.18+
- Git
- OpenSSL


### Clone the Repository
```bash
git clone https://github.com/gokul656/obscure-fs.git
cd obscure-fs
```

### Generate Private Key

Use the following command to generate a private key before starting the node:

```bash
openssl genpkey -algorithm RSA -out keys/private-key.pem -pkeyopt rsa_keygen_bits:2048
```

### Build the Project
```bash
make build
```
This generates the `obscure-fs` binary in the project directory.

### Run the Node
```bash
./obscure-fs serve --port <node-port> --api-port <api-port> --pkey <private-key>
```
- `--port`: Port for the LibP2P network.
- `--api-port`: Port for the HTTP API.
- `--pkey`: Private key for peer

Example:
```bash
./obscure-fs serve --port 3000 --api-port 8080 --pkey keys/private-key.pem
```

## Custom Protocols

### 1. **list_files**
- Command: `list_files`
- Description: Returns a JSON-encoded list of files available on the node.

### 2. **Retrieve by CID**
- Command: `<CID>`
- Description: Retrieves a file corresponding to the CID.

## License
This project is licensed under the GNU Affero General Public License v3.0. See the [LICENSE](LICENSE) file for details.

## Acknowledgments
- [LibP2P](https://libp2p.io/)
- [Go](https://golang.org/)
