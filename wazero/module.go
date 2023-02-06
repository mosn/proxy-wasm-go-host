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
	"strings"

	"github.com/tetratelabs/wazero"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
)

type Module struct {
	vm          *VM
	rawBytes    []byte
	abiNameList []string
}

func NewModule(vm *VM, wasmBytes []byte) *Module {
	m := &Module{vm: vm, rawBytes: wasmBytes}

	m.Init()

	return m
}

// Init reads the exported functions to support later calls to GetABINameList.
func (w *Module) Init() {
	r := wazero.NewRuntimeWithConfig(ctx, w.vm.config)
	defer r.Close(ctx)

	m, err := r.CompileModule(ctx, w.rawBytes)
	if err != nil {
		panic(err)
	}

	for export := range m.ExportedFunctions() {
		if strings.HasPrefix(export, "proxy_abi") {
			w.abiNameList = append(w.abiNameList, export)
		}
	}
}

func (w *Module) NewInstance() common.WasmInstance {
	return NewInstance(w.vm, w)
}

func (w *Module) GetABINameList() []string {
	return w.abiNameList
}
