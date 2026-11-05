.PHONY: build test race vet fmt lint smoke demo install clean

BINARY ?= nameary
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/yufeiminds/nameary/internal/version.Version=$(VERSION) \
	-X github.com/yufeiminds/nameary/internal/version.Commit=$(COMMIT) \
	-X github.com/yufeiminds/nameary/internal/version.Date=$(DATE)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/nameary

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

lint:
	golangci-lint run

smoke:
	./scripts/smoke.sh

demo:
	vhs demo/demo.tape

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/nameary

clean:
	rm -f $(BINARY) nameary-*.json nameary-*.md
