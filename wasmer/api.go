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

package wasmer

import (
	"os"
	"path/filepath"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
)

func NewWasmerInstanceFromFile(path string) common.WasmInstance {
	wasmBytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil
	}
	return NewInstanceFromBinary(wasmBytes)
}

func NewInstanceFromBinary(wasmBytes []byte) common.WasmInstance {
	vm := NewVM()

	module := vm.NewModule(wasmBytes)

	instance := module.NewInstance()

	return instance
}
