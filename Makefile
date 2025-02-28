run-bootstrap:
	go run main.go serve --port 5001 --api-port 8001 --pkey keys/boostrap-1-privatekey.pem

run-client:
	go run main.go serve --port 5003 --api-port 8003  --pkey keys/boostrap-2-privatekey.pem

tests:
	go test ./tests -run TestCodec

build:
	CGO_ENABLED=0 go build .
