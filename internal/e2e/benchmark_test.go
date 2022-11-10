/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package wasm

import (
	_ "embed"
	"testing"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	v1 "mosn.io/proxy-wasm-go-host/proxywasm/v1"
	v2 "mosn.io/proxy-wasm-go-host/proxywasm/v2"
	"mosn.io/proxy-wasm-go-host/wasmer"
)

func BenchmarkStartABIContextV1_wasmer(b *testing.B) {
	benchmarkStartABIContextV1(b, wasmer.NewInstanceFromBinary)
}

func benchmarkStartABIContextV1(b *testing.B, newInstance func([]byte) common.WasmInstance) {
	for i := 0; i < b.N; i++ {
		if wasmCtx, err := startABIContextV1(newInstance(binAddRequestHeaderV1)); err != nil {
			b.Fatal(err)
		} else {
			wasmCtx.Instance.Stop()
		}
	}
}

func BenchmarkAddRequestHeaderV1_wasmer(b *testing.B) {
	benchmarkAddRequestHeaderV1(b, wasmer.NewInstanceFromBinary)
}

func benchmarkAddRequestHeaderV1(b *testing.B, newInstance func([]byte) common.WasmInstance) {
	instance := newInstance(binAddRequestHeaderV1)
	defer instance.Stop()
	benchmarkV1(b, instance, testAddRequestHeaderV1)
}

func benchmarkV1(b *testing.B, instance common.WasmInstance, testV1 func(wasmCtx *v1.ABIContext, contextID int32) error) {
	wasmCtx, err := startABIContextV1(instance)
	if err != nil {
		b.Fatal(err)
	}
	defer wasmCtx.Instance.Stop()

	exports := wasmCtx.GetExports()

	// make the root context
	rootContextID := int32(1)
	if err = exports.ProxyOnContextCreate(rootContextID, int32(0)); err != nil {
		b.Fatal(err)
	}

	// lock wasm vm instance for exclusive ownership
	wasmCtx.Instance.Lock(wasmCtx)
	defer wasmCtx.Instance.Unlock()

	// Time the guest call for context create and delete, which happens per-request.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contextID := int32(2)
		if err = exports.ProxyOnContextCreate(contextID, rootContextID); err != nil {
			b.Fatal(err)
		}

		if err = testV1(wasmCtx, contextID); err != nil {
			b.Fatal(err)
		}

		if _, err = exports.ProxyOnDone(contextID); err != nil {
			b.Fatal(err)
		}

		if err = exports.ProxyOnDelete(contextID); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStartABIContextV2_wasmer(b *testing.B) {
	benchmarkStartABIContextV2(b, wasmer.NewInstanceFromBinary)
}

func benchmarkStartABIContextV2(b *testing.B, newInstance func([]byte) common.WasmInstance) {
	for i := 0; i < b.N; i++ {
		if wasmCtx, err := startABIContextV2(newInstance(binAddRequestHeaderV2)); err != nil {
			b.Fatal(err)
		} else {
			wasmCtx.Instance.Stop()
		}
	}
}

func BenchmarkAddRequestHeaderV2_wasmer(b *testing.B) {
	benchmarkAddRequestHeaderV2(b, wasmer.NewInstanceFromBinary)
}

func benchmarkAddRequestHeaderV2(b *testing.B, newInstance func([]byte) common.WasmInstance) {
	instance := newInstance(binAddRequestHeaderV2)
	defer instance.Stop()
	benchmarkV2(b, instance, testAddRequestHeaderV2)
}

func benchmarkV2(b *testing.B, instance common.WasmInstance, testV2 func(wasmCtx *v2.ABIContext, contextID int32) error) {
	wasmCtx, err := startABIContextV2(instance)
	if err != nil {
		b.Fatal(err)
	}
	defer wasmCtx.Instance.Stop()

	exports := wasmCtx.GetExports()

	// make the root context
	rootContextID := int32(1)
	if err = exports.ProxyOnContextCreate(rootContextID, int32(0), v2.ContextTypeHttpContext); err != nil {
		b.Fatal(err)
	}

	// lock wasm vm instance for exclusive ownership
	wasmCtx.Instance.Lock(wasmCtx)
	defer wasmCtx.Instance.Unlock()

	// Time the guest call for context create and delete, which happens per-request.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contextID := int32(2)
		if err = exports.ProxyOnContextCreate(contextID, rootContextID, v2.ContextTypeHttpContext); err != nil {
			b.Fatal(err)
		}

		if err = testV2(wasmCtx, contextID); err != nil {
			b.Fatal(err)
		}

		if _, err = exports.ProxyOnDone(contextID); err != nil {
			b.Fatal(err)
		}

		if err = exports.ProxyOnDelete(contextID); err != nil {
			b.Fatal(err)
		}
	}
}
