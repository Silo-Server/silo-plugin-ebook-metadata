.PHONY: build test clean

BINARY=plugin
VERSION ?= $(shell git describe --tags --always 2>/dev/null | sed 's/^v//')
LDFLAGS=-s -w -X main.version=$(VERSION)

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

clean:
	rm -f $(BINARY)
