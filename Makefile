VERSION := $(shell tr -d '\n' < VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test run check verify fmt cross dev-cross
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o sigwatch ./cmd/sigwatch
run:
	go run -ldflags "$(LDFLAGS)" ./cmd/sigwatch -config ./config.yaml
check:
	go run ./cmd/sigwatch -config ./config.yaml -check
test:
	go test ./...
verify:
	go test ./...
	go vet ./...
	go run ./cmd/sigwatch -config ./config.yaml -check
	bash -n scripts/install-pi.sh scripts/update-pi.sh scripts/update-runner.sh
fmt:
	gofmt -w $$(find . -name '*.go')
cross:
	mkdir -p dist
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/sigwatch-linux-arm64 ./cmd/sigwatch
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/sigwatch-linux-amd64 ./cmd/sigwatch

dev-cross:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/sigwatch-darwin-arm64 ./cmd/sigwatch
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/sigwatch-windows-amd64.exe ./cmd/sigwatch
