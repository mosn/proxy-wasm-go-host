# WebAssembly for Proxies (GoLang host implementation)

The GoLang implementation for [proxy-wasm](https://github.com/proxy-wasm/spec), enabling developer to run proxy-wasm extensions in Go.

## Run Example

- build and run host
```bash
cd example
go run .
```

- send http request
```bash
curl http://127.0.0.1:2045/
```

- host log
```text
receive request /
print header from server host, User-Agent -> [curl/7.64.1]
print header from server host, Accept -> [*/*]
[http_wasm_example.cc:33]::onRequestHeaders() print from wasm, onRequestHeaders, context id: 2
[http_wasm_example.cc:38]::onRequestHeaders() print from wasm, Accept -> */*
[http_wasm_example.cc:38]::onRequestHeaders() print from wasm, User-Agent -> curl/7.64.1
```

## references

- https://github.com/proxy-wasm/spec
- https://github.com/proxy-wasm/proxy-wasm-cpp-sdk
- https://github.com/proxy-wasm/proxy-wasm-rust-sdk
- https://github.com/tetratelabs/envoy-wasm-rust-sdk
- https://github.com/tetratelabs/proxy-wasm-go-sdk