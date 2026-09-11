BINARY := bin/torrent
GO := go

.PHONY: all build fmt vet test test-race swarm-up swarm-down clean

all: fmt vet test

build:
	$(GO) build -o $(BINARY) ./cmd

fmt:
	@test -z "$$(gofmt -l .)" || (echo "files need gofmt:"; gofmt -l .; exit 1)

vet:
	$(GO) vet ./...

test:
	$(GO) test ./internal/parser ./internal/store ./internal/downloader -race -count=1

test-race:
	$(GO) test ./... -race -count=1

swarm-up:
	docker compose up -d tracker peer1 peer2 peer3

swarm-down:
	docker compose down

clean:
	rm -rf bin
