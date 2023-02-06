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

package wazero

import (
	"context"

	"github.com/tetratelabs/wazero"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
)

type VM struct {
	cache  wazero.CompilationCache
	config wazero.RuntimeConfig
}

func NewVM() common.WasmVM {
	vm := &VM{}
	vm.Init()

	return vm
}

func (w *VM) Name() string {
	return "wazero"
}

var ctx = context.Background()

func (w *VM) Init() {
	w.cache = wazero.NewCompilationCache()
	w.config = wazero.NewRuntimeConfig().WithCompilationCache(w.cache)
}

func (w *VM) NewModule(wasmBytes []byte) common.WasmModule {
	if len(wasmBytes) == 0 {
		panic("wasm was empty")
	}

	return NewModule(w, wasmBytes)
}

// Close implements io.Closer
func (w *VM) Close() (err error) {
	if c := w.cache; c != nil {
		err = c.Close(ctx)
	}
	return
}
