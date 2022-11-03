/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the //License//); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an //AS IS// BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package proxy_wasm_go_host

import (
	"log"
	"os"
	"testing"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	v1 "mosn.io/proxy-wasm-go-host/proxywasm/v1"
	"mosn.io/proxy-wasm-go-host/wasmer"
)

var exampleWasm = func() []byte {
	wasmBytes, err := os.ReadFile("example/data/http.wasm")
	if err != nil {
		log.Panicln(err)
	}
	return wasmBytes
}()

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

func BenchmarkContextLifecycleV1(b *testing.B) {
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

	// Time the guest call for context create and delete, which happens per-request.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err = ctx.GetExports().ProxyOnContextCreate(int32(1), int32(0)); err != nil {
			b.Fatal(err)
		}
		if _, err = ctx.GetExports().ProxyOnDone(int32(1)); err != nil {
			b.Fatal(err)
		}
	}
}
