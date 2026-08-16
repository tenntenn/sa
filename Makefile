GO ?= go
PNPM ?= pnpm

.PHONY: build web bin test lint dev clean install

build: web bin

## web builds the review UI that gets embedded into the binary.
web:
	cd web && $(PNPM) install --frozen-lockfile && $(PNPM) run build

bin:
	$(GO) build ./...

install: web
	$(GO) install .

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...
	gofmt -l . | grep -v '^web/node_modules' && exit 1 || true

## dev runs the sa server in the foreground and the Vite dev server, which
## proxies /_ to it.
dev:
	$(GO) run . --foreground & \
	cd web && $(PNPM) run dev

## clean leaves web/dist alone: its contents are committed and go:embed
## needs them to build the binary. Run `make web` to regenerate them.
clean:
	rm -rf web/node_modules
