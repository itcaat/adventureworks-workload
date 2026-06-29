.PHONY: build test tidy

build:
	go build ./cmd/awload

test:
	go test ./...

tidy:
	go mod tidy

