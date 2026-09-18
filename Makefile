.PHONY: build test run check fmt cross dev-cross
build:
	go build -o sigwatch ./cmd/sigwatch
run:
	go run ./cmd/sigwatch -config ./config.yaml
check:
	go run ./cmd/sigwatch -config ./config.yaml -check
test:
	go test ./...
fmt:
	gofmt -w $$(find . -name '*.go')
cross:
	mkdir -p dist
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/sigwatch-linux-arm64 ./cmd/sigwatch
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/sigwatch-linux-amd64 ./cmd/sigwatch

dev-cross:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o dist/sigwatch-darwin-arm64 ./cmd/sigwatch
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o dist/sigwatch-windows-amd64.exe ./cmd/sigwatch
