APP := weaver

.PHONY: build test test-integration install fmt

build:
	mkdir -p bin
	go build -o bin/$(APP) .

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

install:
	go install .

fmt:
	go fmt ./...
