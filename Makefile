.PHONY: build build-installer test check run install install-autostart install-systemd install-xdg uninstall clean

PROFILE ?=
AUTOSTART ?= auto
export KEY_SHORT_PROFILE = $(PROFILE)
export KEY_SHORT_AUTOSTART = $(AUTOSTART)

build:
	mkdir -p bin
	go build -trimpath -o bin/key-short ./cmd/key-short

build-installer:
	mkdir -p bin
	go build -trimpath -o bin/key-short-install ./cmd/key-short-install

test:
	go test ./...

check:
	go vet ./...

run:
	go run ./cmd/key-short

install: build build-installer
	./bin/key-short-install install

install-autostart: build build-installer
	./bin/key-short-install install-autostart

install-systemd: export KEY_SHORT_AUTOSTART = systemd
install-systemd: install-autostart

install-xdg: export KEY_SHORT_AUTOSTART = xdg
install-xdg: install-autostart

uninstall: build-installer
	./bin/key-short-install uninstall

clean:
	rm -rf bin
