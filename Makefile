gofumpt := mvdan.cc/gofumpt@v0.4.0
gosimports := github.com/rinchsan/gosimports/cmd/gosimports@v0.3.4

.PHONY: format
format:
	@go run $(gofumpt) -l -w .
	@go run $(gosimports) -local mosn.io/proxy-wasm-go-host/ -w $(shell find . -name '*.go' -type f)
