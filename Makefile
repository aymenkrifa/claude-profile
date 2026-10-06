BIN       := claude-profile
PREFIX    ?= $(HOME)/.local
SHAREDIR  := $(PREFIX)/share/claude-profile
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
SHELLS    := zsh bash fish

.PHONY: all build test vet fmt install uninstall dist e2e clean

all: fmt vet test build

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BIN) ./cmd/$(BIN)

test:
	go test ./...

vet:
	go vet ./...
	@set -e; for s in $(SHELLS); do \
	  if command -v $$s >/dev/null 2>&1; then \
	    $$s -n shell/claude-profile.$$s; \
	    echo "shell/claude-profile.$$s ok"; \
	  else \
	    echo "shell/claude-profile.$$s not checked ($$s not installed)"; \
	  fi; \
	done

# install.sh against freshly built archives, then a profile's life in real
# shells, all under a throwaway $HOME. See test/e2e.sh.
e2e:
	$(MAKE) dist VERSION=v0.0.0-e2e
	sh test/e2e.sh dist v0.0.0-e2e

fmt:
	gofmt -l -w .

# Installs over any earlier version. ~/.local/bin precedes ~/go/bin in PATH,
# so this is the copy the shell will find. The shell integrations go beside it
# where 'claude-profile shell-init' looks for them: one directory up from the
# binary, then share/claude-profile. All of them are installed whatever shell
# is in use -- they are a few kilobytes and it means shell-init can answer for
# any of them.
install: build
	install -d $(PREFIX)/bin $(SHAREDIR)
	install -m 0755 bin/$(BIN) $(PREFIX)/bin/$(BIN)
	@for s in $(SHELLS); do \
	  install -m 0644 shell/claude-profile.$$s $(SHAREDIR)/claude-profile.$$s; \
	  echo "install -m 0644 shell/claude-profile.$$s $(SHAREDIR)/"; \
	done
	@echo "installed $(PREFIX)/bin/$(BIN)  ($(VERSION))"
	@echo "next: $(PREFIX)/bin/$(BIN) shell-init   # the line to add to your shell startup file"

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)
	rm -rf $(SHAREDIR)

# dist builds what a release serves and what install.sh downloads: one archive
# per platform, each self-contained (binary plus the shell integrations), and a
# checksum file to verify them against.
dist:
	rm -rf dist && mkdir -p dist
	@set -eu; for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; \
	  name=$(BIN)_$(VERSION)_$${os}_$${arch}; \
	  mkdir -p dist/$$name; \
	  GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build \
	    -ldflags "-s -w -X main.version=$(VERSION)" -o dist/$$name/$(BIN) ./cmd/$(BIN); \
	  cp shell/claude-profile.* README.md LICENSE dist/$$name/; \
	  tar -czf dist/$$name.tar.gz -C dist $$name; \
	  rm -rf dist/$$name; \
	  echo "  dist/$$name.tar.gz"; \
	done
	@cd dist && { sha256sum *.tar.gz 2>/dev/null || shasum -a 256 *.tar.gz; } > checksums.txt
	@echo "  dist/checksums.txt"

clean:
	rm -rf bin dist
