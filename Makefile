.PHONY: build test tidy

build:
	go build ./cmd/awload

test:
	go test ./... -v

tidy:
	go mod tidy

