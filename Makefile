.PHONY: build run fmt fmt-check vet test-e2e

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

test-e2e:
	TESTCONTAINERS_RYUK_DISABLED=true go test -tags e2e ./... -timeout 600s
