BINARY      := tensorwatch
PKG         := github.com/mesutoezdil/tensorwatch
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
TAGS        ?=

.PHONY: all build build-nvidia twload run test vet fmt tidy clean install release lint

all: build twload

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -tags '$(TAGS)' -o bin/$(BINARY) ./cmd/tensorwatch

build-nvidia:
	CGO_ENABLED=1 go build -trimpath -ldflags '$(LDFLAGS)' -tags 'nvidia $(TAGS)' -o bin/$(BINARY) ./cmd/tensorwatch

twload:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/twload ./cmd/twload

run: build
	./bin/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

lint: vet
	gofmt -l . | tee /tmp/tw-fmt && [ ! -s /tmp/tw-fmt ]

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

install: build twload
	install -m 0755 bin/$(BINARY) /usr/local/bin/$(BINARY)
	install -m 0755 bin/twload /usr/local/bin/twload

clean:
	rm -rf bin dist

release: clean
	@mkdir -p dist
	@for combo in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
	  GOOS=$${combo%/*}; GOARCH=$${combo#*/}; \
	  echo "build $$GOOS/$$GOARCH"; \
	  CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build -trimpath \
	    -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-$$GOOS-$$GOARCH ./cmd/tensorwatch; \
	  CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build -trimpath \
	    -ldflags '$(LDFLAGS)' -o dist/twload-$$GOOS-$$GOARCH ./cmd/twload; \
	done
	@( cd dist && shasum -a 256 * > SHA256SUMS )
	@ls -la dist
