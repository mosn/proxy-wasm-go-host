//go:build wasmer

package proxy_wasm_go_host

import (
	_ "embed"
	"testing"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	v1 "mosn.io/proxy-wasm-go-host/proxywasm/v1"
	"mosn.io/proxy-wasm-go-host/wasmer"
)

//go:embed example/data/http.wasm
var exampleWasm []byte

func BenchmarkStartInstanceV1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if instance, err := startInstanceV1(); err != nil {
			b.Fatal(err)
		} else {
			instance.Stop()
		}
	}
}

func startInstanceV1() (instance common.WasmInstance, err error) {
	instance = wasmer.NewInstanceFromBinary(exampleWasm)

	// register ABI imports into the wasm vm instance
	v1.RegisterImports(instance)

	// start the wasm vm instance
	if err = instance.Start(); err != nil {
		instance.Stop()
	}
	return
}

func BenchmarkCallGuestV1(b *testing.B) {
	instance, err := startInstanceV1()
	if err != nil {
		b.Fatal(err)
	}
	defer instance.Stop()

	// create abi context
	ctx := &v1.ABIContext{Imports: &v1.DefaultImportsHandler{}, Instance: instance}

	// make the root context
	if err = ctx.GetExports().ProxyOnContextCreate(0, 0); err != nil {
		b.Fatal(err)
	}

	// Time the guest call for context creation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err = ctx.GetExports().ProxyOnContextCreate(int32(i+1), 0); err != nil {
			b.Fatal(err)
		}
	}
}
