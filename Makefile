.PHONY: build run fmt fmt-check vet

build:
	go build ./...

run:
	go run ./cmd/server

fmt:
	gofmt -w .

fmt-check:
	@gofmt -l . | grep . && exit 1 || exit 0

vet:
	go vet ./...
