BINARY     := forgelet
CMD        := ./cmd/forgelet
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -X github.com/Lamassau/forgelet/internal/cli.Version=$(VERSION) \
              -X github.com/Lamassau/forgelet/internal/cli.Commit=$(COMMIT) \
              -X github.com/Lamassau/forgelet/internal/cli.BuildDate=$(BUILD_DATE)

.PHONY: build test lint install clean release

build:
go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

test:
go test -v ./...

lint:
golangci-lint run ./...

install:
go install -ldflags "$(LDFLAGS)" $(CMD)

clean:
rm -rf bin/ dist/

release:
goreleaser release --clean
