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
	wazero "github.com/tetratelabs/wazero"
	"mosn.io/proxy-wasm-go-host/proxywasm/common"
)

type VM struct {
	engine wazero.Runtime
}

func NewVM() common.WasmVM {
	vm := &VM{}
	vm.Init()

	return vm
}

func (w *VM) Name() string {
	return "wazero"
}

func (w *VM) Init() {
	w.engine = wazero.NewRuntime(context.Background())
}

func (w *VM) NewModule(wasmBytes []byte) common.WasmModule {
	if len(wasmBytes) == 0 {
		return nil
	}

	m, err := w.engine.CompileModule(context.Background(), wasmBytes)
	if err != nil {
		return nil
	}

	return NewModule(w, m, wasmBytes)
}
