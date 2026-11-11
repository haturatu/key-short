.PHONY: build test check run install clean

build:
	mkdir -p bin
	go build -trimpath -o bin/key-short ./cmd/key-short

test:
	go test ./...

check:
	go vet ./...

run:
	go run ./cmd/key-short

install: build
	./scripts/install-desktop.sh

clean:
	rm -rf bin
