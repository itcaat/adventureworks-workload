.PHONY: build test tidy release

build:
	go build ./cmd/awload

test:
	go test ./... -v

tidy:
	go mod tidy

release:
	@./scripts/release.sh

