.PHONY: clean mrproper dist test default native arm64 run all
GO=$(shell which go)
GIT_VERSION=$(shell git log -1 --format='%h' 2>/dev/null || echo '0000000')

default: dist

dist: native

all: mrproper | arm64 native

clean:
	@$(GO) clean

mrproper: clean
	@rm -rf dist

arm64:
	@env GOOS=linux GOARCH=arm64 $(GO) build -C ./src -o ../dist/arm64/ -ldflags="-s -w -X main.GIT_VERSION=$(GIT_VERSION)"
	@cp -R conf dist/arm64/

native:
	@$(GO) build -C ./src -o ../dist/native/ -ldflags="-s -w -X main.GIT_VERSION=$(GIT_VERSION)"
	@cp -R conf dist/native/

test:
	@echo "No unit tests for this evaluation project, say make run."

run: dist
	@mkdir -p data
	@./dist/amd64/mqttrack
