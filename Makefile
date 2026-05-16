BINARY_NAME=mikro-midi
GOFLAGS=-trimpath

# Local Go toolchain (for native builds)
GOROOT=$(CURDIR)/.tools/go
GO=$(GOROOT)/bin/go

# Docker
DOCKER_IMAGE=mikro-midi-builder
DOCKER=docker run --rm -v $(CURDIR):/src -w /src $(DOCKER_IMAGE)

# Windows cross-compile (native, requires mingw installed)
CC_WINDOWS=x86_64-w64-mingw32-gcc

.PHONY: build build-windows release-windows docker-image docker-build docker-release clean tidy

# Build for current platform
build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) .

# Cross-compile for Windows (requires mingw on host)
build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=$(CC_WINDOWS) \
		$(GO) build $(GOFLAGS) -o $(BINARY_NAME).exe .

release-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=$(CC_WINDOWS) \
		$(GO) build $(GOFLAGS) -ldflags="-s -w" -o $(BINARY_NAME).exe .

# --- Docker targets (no mingw/go needed on host) ---

docker-image:
	docker build -t $(DOCKER_IMAGE) -f Dockerfile.build .

docker-build: docker-image
	$(DOCKER) sh -c 'GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -buildvcs=false -trimpath -o $(BINARY_NAME).exe .'

docker-release: docker-image
	$(DOCKER) sh -c 'GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -buildvcs=false -trimpath -ldflags="-s -w" -o $(BINARY_NAME).exe .'

tidy:
	$(GO) mod tidy

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe
