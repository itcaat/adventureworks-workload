.PHONY: build test tidy release fire

build:
	go build -o awload ./cmd/awload

test:
	go test ./... -v

tidy:
	go mod tidy

release:
	@./scripts/release.sh

fire: build
	@test -n "$$AWLOAD_DSN" || (echo "AWLOAD_DSN is required" >&2 && exit 1)
	./awload \
		-dsn "$$AWLOAD_DSN" \
		-users 100 \
		-duration 60s \
		-ramp 10s \
		-profile write-light \
		-write-mode cart \
		-report-name smoke-write
