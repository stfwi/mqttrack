.PHONY: clean mrproper dist test default arm64 run
GO=$(shell which go)

default: dist

dist:
	@$(GO) build -C ./src -o ../dist/amd64/
	@cp -R conf dist/amd64/

clean:
	@$(GO) clean

mrproper: clean
	@rm -rf dist

arm64:
	@env GOOS=linux GOARCH=arm64 $(GO) build -o dist/arm64/
	@cp -R conf dist/arm64/

test:
	@echo "No unit tests for this evaluation project, say make run."

run: dist
	@mkdir -p data
	@./dist/amd64/mqttrack
