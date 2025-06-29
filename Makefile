.PHONY: clean mrproper dist test default native arm64 run all
MAKEFLAGS+=--no-print-directory
GO=$(shell which go)
GIT_VERSION=$(shell git log -1 --format='%h' 2>/dev/null || echo '0000000')

default: dist

dist: native

all: mrproper
	@echo "Building ..."
	@$(MAKE) arm64 native
	@echo "Running tests ..."
	@$(MAKE) test

clean:
	@$(GO) -C ./src clean

mrproper:
	@rm -rf dist
	@$(GO) -C ./src clean -r
	@$(GO) -C ./src clean -r -cache
	@$(GO) -C ./src clean -r -testcache
	@$(GO) -C ./src clean -r -fuzzcache

arm64:
	@env GOOS=linux GOARCH=arm64 $(GO) build -C ./src -o ../dist/arm64/ -ldflags="-s -w -X main.GIT_VERSION=$(GIT_VERSION)"
	@[ ! -d conf ] || cp -R conf dist/arm64/

native:
	@$(GO) build -C ./src -o ../dist/native/ -ldflags="-s -w -X main.GIT_VERSION=$(GIT_VERSION)"
	@[ ! -d conf ] || cp -R conf dist/native/

test:
	@$(GO) test -C ./src -coverpkg=./recorder ./recorder -ldflags="-X main.GIT_VERSION=$(GIT_VERSION)"

run: dist
	@mkdir -p data
	@./dist/amd64/mqttrack
