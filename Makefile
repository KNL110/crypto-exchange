.PHONY: build build-cli run run-cli test test-race
CONFIG ?= ./config/config.yaml
build:
	@go build -o ./bin/exchange ./cmd

build-cli:
	@go build -o ./bin/exchangectl ./cmd/exchangectl

run: build
	@CONFIG_PATH=$(CONFIG) ./bin/exchange
 
run-cli: build-cli
	@./bin/exchangectl

test:
	@go test -v ./...

test-race:
	@go test -race ./...