BINARY      := tensorwatch
PKG         := github.com/mesutoezdil/tensorwatch
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GOFLAGS     ?=
TAGS        ?=

.PHONY: build build-nvidia run test vet fmt tidy clean install release

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -tags '$(TAGS)' -o bin/$(BINARY) ./cmd/tensorwatch

build-nvidia:
	CGO_ENABLED=1 go build -trimpath -ldflags '$(LDFLAGS)' -tags 'nvidia $(TAGS)' -o bin/$(BINARY) ./cmd/tensorwatch

run: build
	./bin/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

install: build
	install -m 0755 bin/$(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -rf bin

release:
	@mkdir -p dist
	@for os in linux darwin; do \
	  for arch in amd64 arm64; do \
	    echo "building $$os/$$arch"; \
	    CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	      go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-$$os-$$arch ./cmd/tensorwatch; \
	  done; \
	done
