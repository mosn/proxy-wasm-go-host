//go:build wasmer
// +build wasmer

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

package e2e

import (
	_ "embed"
	"testing"

	"mosn.io/proxy-wasm-go-host/wasmer"
)

func TestStartABIContextV1_wasmer(t *testing.T) {
	vm := wasmer.NewWasmerVM()
	defer vm.Close()

	testStartABIContextV1(t, vm)
}

func TestStartABIContextV2_wasmer(t *testing.T) {
	vm := wasmer.NewWasmerVM()
	defer vm.Close()

	testStartABIContextV2(t, vm)
}

func TestAddRequestHeaderV1_wasmer(t *testing.T) {
	vm := wasmer.NewWasmerVM()
	defer vm.Close()

	testV1(t, vm, testAddRequestHeaderV1)
}

func TestAddRequestHeaderV2_wasmer(t *testing.T) {
	vm := wasmer.NewWasmerVM()
	defer vm.Close()

	testV2(t, vm, testAddRequestHeaderV2)
}
