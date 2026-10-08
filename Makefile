VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build run web vet test test-web test-e2e check clean

## build: build the dungeon binary into bin/
build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/dungeon ./cmd/dungeon

## run: build and serve on the default loopback address
run: build
	./bin/dungeon serve

## web: build the frontend into web/dist (embedded by the Go build)
web:
	cd web && npm ci && npm run build
	touch web/dist/.gitkeep

vet:
	go vet ./...

test:
	go test -race ./...

## test-web: frontend type check and unit/component tests
test-web:
	cd web && { [ -d node_modules ] || npm ci; } && npm run check && npm test

## test-e2e: browser tests against the real binary (run `npx playwright install chromium` once)
test-e2e: build
	cd web && npm run test:e2e

## check: what CI runs
check: vet test test-web

clean:
	rm -rf bin build
	find web/dist -mindepth 1 ! -name .gitkeep -delete
