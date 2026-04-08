APP := weaver
VERSION ?= dev
LDFLAGS := -X github.com/lutefd/weaver/cmd.version=$(VERSION)

.PHONY: build test test-integration install fmt

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) .

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

install:
	go install -ldflags "$(LDFLAGS)" .

fmt:
	go fmt ./...
