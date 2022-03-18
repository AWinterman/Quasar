
all: build test
build:
	cd protos && buf generate --config buf.yaml --template buf.gen.yaml
	@mkdir -p bin
	@go mod tidy
	@go mod vendor
	GO111MODULE=on go build -o bin/quasar cmd/main.go
	@echo "Go Build complete"


test:
	go test ./...

