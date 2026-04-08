APP := weaver

.PHONY: build test install fmt

build:
	mkdir -p bin
	go build -o bin/$(APP) .

test:
	go test ./...

install:
	go install .

fmt:
	go fmt ./...
