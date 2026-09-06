BIN     := claude-profile
PREFIX  ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all build test vet fmt install uninstall clean

all: fmt vet test build

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BIN) ./cmd/$(BIN)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Installs over any earlier version. ~/.local/bin precedes ~/go/bin in PATH,
# so this is the copy the shell will find.
install: build
	install -d $(PREFIX)/bin
	install -m 0755 bin/$(BIN) $(PREFIX)/bin/$(BIN)
	@echo "installed $(PREFIX)/bin/$(BIN)  ($(VERSION))"

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

clean:
	rm -rf bin
