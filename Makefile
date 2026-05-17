# smart-speaker-mcp — local build helpers
#
# Targets:
#   make build           Build native binary for the current OS/arch
#   make rebuild         build + restart Claude Desktop (cross-platform)
#   make restart-claude  Just restart Claude Desktop (no rebuild)
#   make build-all       Cross-compile binaries for all release targets
#   make clean           Remove built binaries
#   make tidy            Tidy go.mod / go.sum
#   make vet             Run go vet ./...
#   make tag VERSION=v3.2.0   Create + push a release tag (triggers GitHub Action)

VERSION ?= 4.0.0
LDFLAGS := -s -w -X github.com/WeCodeBase/smart-speaker-mcp/internal/mcpserver.Version=$(VERSION)
GO_BUILD := go build -trimpath -ldflags="$(LDFLAGS)"
PKG := ./cmd/smart-speaker-mcp

# Detect host OS (Darwin, Linux, MINGW*/MSYS*/CYGWIN* on Windows-with-bash)
UNAME_S := $(shell uname -s 2>/dev/null || echo Windows)

.PHONY: build rebuild restart-claude build-all clean tidy vet tag help

build: vet
	CGO_ENABLED=0 $(GO_BUILD) -o smart-speaker-mcp $(PKG)
	@echo "  ✅ built smart-speaker-mcp ($(VERSION))"

vet:
	@go vet ./...

rebuild: build restart-claude

restart-claude:
ifeq ($(UNAME_S),Darwin)
	@echo "▶ Restarting Claude Desktop (macOS)..."
	@osascript -e 'quit app "Claude"' 2>/dev/null || true
	@sleep 1
	@open -a "Claude"
	@echo "  ✅ restarted"
else ifeq ($(UNAME_S),Linux)
	@echo "▶ Restarting Claude Desktop (Linux)..."
	@pkill -x claude-desktop 2>/dev/null || pkill -f -i claude 2>/dev/null || true
	@sleep 1
	@(setsid claude-desktop >/dev/null 2>&1 &) || (setsid claude >/dev/null 2>&1 &) || \
	  echo "  ⚠  Could not auto-launch Claude — start it manually."
	@echo "  ✅ restart triggered"
else
	@echo "▶ Restarting Claude Desktop (Windows)..."
	@powershell -NoProfile -Command "Get-Process Claude -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1; Start-Process Claude"
	@echo "  ✅ restarted"
endif

build-all: clean
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 $(GO_BUILD) -o dist/smart-speaker-mcp-darwin-arm64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 $(GO_BUILD) -o dist/smart-speaker-mcp-darwin-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO_BUILD) -o dist/smart-speaker-mcp-windows-amd64.exe $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 $(GO_BUILD) -o dist/smart-speaker-mcp-linux-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 $(GO_BUILD) -o dist/smart-speaker-mcp-linux-arm64 $(PKG)
	cd dist && shasum -a 256 smart-speaker-mcp-* > SHA256SUMS.txt
	@echo
	@ls -lh dist/

clean:
	rm -rf dist/
	rm -f smart-speaker-mcp smart-speaker-mcp.exe

tidy:
	go mod tidy
	go mod download

tag:
ifndef VERSION
	$(error VERSION is required, e.g. make tag VERSION=v3.2.0)
endif
	@echo "Tagging $(VERSION) and pushing to origin..."
	git tag $(VERSION)
	git push origin $(VERSION)
	@echo "Done. Watch the release build at:"
	@echo "  https://github.com/WeCodeBase/smart-speaker-mcp/actions"

help:
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN {FS=":.*?##"}; {printf "%-15s %s\n", $$1, $$2}'
