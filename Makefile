.PHONY: build

build:
	go build -o ./bin/exchange ./cmd/exchange

run: build
	./bin/exchange

test:
	go test -v ./...