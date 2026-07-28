BINARY   := cortex-cc
MCP      := cortex-mcp
CMD      := ./cmd/server
MCP_CMD  := ./cmd/mcp-server
GOFLAGS  := -ldflags="-s -w"

.PHONY: run build build-mcp build-all test lint docker-up docker-down clean

## run: start the server (requires Ollama running locally)
run:
	go run $(CMD)

## build: compile the main server binary
build:
	go build $(GOFLAGS) -o bin/$(BINARY) $(CMD)

## build-mcp: compile the standalone MCP stdio server
##   Add to Cursor: {"mcpServers":{"cortex-cc":{"command":"$(PWD)/bin/cortex-mcp"}}}
build-mcp:
	go build $(GOFLAGS) -o bin/$(MCP) $(MCP_CMD)

## build-all: compile both binaries
build-all: build build-mcp

## test: run all unit tests with race detector
test:
	go test -race -count=1 ./...

## test-short: run only unit tests (skip slow integration tests)
test-short:
	go test -short -count=1 ./...

## lint: vet + staticcheck
lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed — run: go install honnef.co/go/tools/cmd/staticcheck@latest"

## tidy: tidy and vendor modules
tidy:
	go mod tidy

## docker-up: start all services (ollama + whisper + sentiment + cortex-cc)
docker-up:
	docker compose up --build -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@docker compose ps

## docker-down: stop and remove all containers
docker-down:
	docker compose down

## docker-logs: tail all container logs
docker-logs:
	docker compose logs -f

## docker-pull-model: pull the Llama 3.1:8b model into the Ollama container
docker-pull-model:
	docker compose exec ollama ollama pull llama3.1:8b

## clean: remove build artefacts
clean:
	rm -rf bin/

## help: list all targets
help:
	@grep -E '^##' Makefile | sed 's/## //'
