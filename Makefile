VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build run web vet test check clean

## build: build the dungeon binary into bin/
build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/dungeon ./cmd/dungeon

## run: build and serve on the default loopback address
run: build
	./bin/dungeon serve

## web: stage the frontend into web/dist (placeholder until the Svelte app lands)
web:
	mkdir -p web/dist
	cp web/placeholder/index.html web/dist/index.html

vet:
	go vet ./...

test:
	go test ./...

## check: what CI runs
check: vet test

clean:
	rm -rf bin build
	find web/dist -mindepth 1 ! -name .gitkeep -delete
