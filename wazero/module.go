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
	wazero "github.com/tetratelabs/wazero"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
)

type Module struct {
	vm          *VM
	module      wazero.CompiledModule
	abiNameList []string
	rawBytes    []byte
}

func NewModule(vm *VM, module wazero.CompiledModule, wasmBytes []byte) *Module {
	m := &Module{
		vm:       vm,
		module:   module,
		rawBytes: wasmBytes,
	}

	m.Init()

	return m
}

func (w *Module) Init() {
}

func (w *Module) NewInstance() common.WasmInstance {
	return NewInstance(w.vm, w)
}

func (w *Module) GetABINameList() []string {
	return nil
}
