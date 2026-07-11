BINARY := bin/server

.PHONY: build run clean

build:
	go build -o $(BINARY) ./cmd/server

run:
	go run ./cmd/server

clean:
	rm -rf bin
