VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DEV_ADDR ?= 127.0.0.1:7878
DEV_PORT ?= 5173
DEV_WATCH = 0
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build run run-fresh dev dev-watch web vet test test-linux test-web test-e2e test-dev check clean

## build: build the dungeon binary into bin/
build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/dungeon ./cmd/dungeon

## run: build and serve on the default loopback address
run: build
	./bin/dungeon serve

## dev: Vite hot reload with a stable backend (Ctrl+C stops both)
dev:
	@test -d web/node_modules || { cd web && npm ci; }
	@echo "Development UI: http://127.0.0.1:$(DEV_PORT) (open the backend's token URL once to sign in)"
	DEV_ADDR=$(DEV_ADDR) DEV_PORT=$(DEV_PORT) DEV_WATCH=$(DEV_WATCH) node dev/run.mjs

## dev-watch: also restart Go on changes; each restart terminates active sessions
dev-watch: DEV_WATCH = 1
dev-watch: dev

## test-dev: isolated development startup, proxy, restart and shutdown checks
test-dev:
	node --test dev/dev.test.mjs

## run-fresh: build and serve with an empty, temporary config directory
run-fresh: build
	@fresh_dir=$$(mktemp -d "$${TMPDIR:-/tmp}/dungeon-fresh.XXXXXX") && \
	  trap 'rm -rf "$$fresh_dir"' EXIT INT TERM; \
	  DUNGEON_HOME="$$fresh_dir" ./bin/dungeon serve

## web: build the frontend into web/dist (embedded by the Go build)
web:
	cd web && npm ci && npm run build
	touch web/dist/.gitkeep

vet:
	go vet ./...

test:
	go test -race ./...

## test-linux: run the Go tests in a Linux container (PTY behaviour differs from macOS)
test-linux:
	docker run --rm -v "$(CURDIR)":/src -v dungeon-gocache:/root/.cache -v dungeon-gomod:/go/pkg/mod -w /src \
		golang:1.26 go test -count=1 ./...

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
