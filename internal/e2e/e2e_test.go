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
	"fmt"
	"strconv"
	"testing"

	"mosn.io/pkg/log"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	v1 "mosn.io/proxy-wasm-go-host/proxywasm/v1"
	v2 "mosn.io/proxy-wasm-go-host/proxywasm/v2"
	"mosn.io/proxy-wasm-go-host/wazero"
)

func init() {
	log.DefaultLogger.SetLogLevel(log.ERROR)
}

func TestStartABIContextV1_wazero(t *testing.T) {
	vm := wazero.NewVM()
	defer vm.Close()

	testStartABIContextV1(t, vm)
}

func testStartABIContextV1(t *testing.T, vm common.WasmVM) {
	module := vm.NewModule(binAddRequestHeaderV1)
	instance := module.NewInstance()
	defer instance.Stop()

	if _, err := startABIContextV1(instance); err != nil {
		t.Fatal(err)
	}
}

func startABIContextV1(instance common.WasmInstance) (wasmCtx *v1.ABIContext, err error) {
	// create ABI context
	wasmCtx = &v1.ABIContext{Imports: &v1.DefaultImportsHandler{}, Instance: instance}

	// register ABI imports into the wasm vm instance
	if err = instance.RegisterImports(wasmCtx.Name()); err != nil {
		return
	}

	// start the wasm vm instance
	err = instance.Start()
	return
}

func TestAddRequestHeaderV1_wazero(t *testing.T) {
	vm := wazero.NewVM()
	defer vm.Close()

	testV1(t, vm, testAddRequestHeaderV1)
}

func testAddRequestHeaderV1(wasmCtx *v1.ABIContext, contextID int32) error {
	handler := &headersHandlerV1{reqHeader: &common.CommonHeader{}}
	wasmCtx.SetImports(handler)

	if action, err := wasmCtx.GetExports().ProxyOnRequestHeaders(contextID, 0, 1); err != nil {
		return err
	} else if want, have := v1.ActionContinue, action; want != have {
		return fmt.Errorf("unexpected action, want: %v, have: %v", want, have)
	}

	expectedHeader := "Wasm-Context"
	want := strconv.Itoa(int(contextID))
	if have, _ := handler.GetHttpRequestHeader().Get(expectedHeader); want != have {
		return fmt.Errorf("unexpected %s, want: %v, have: %v", expectedHeader, want, have)
	}
	return nil
}

func testV1(t *testing.T, vm common.WasmVM, test func(wasmCtx *v1.ABIContext, contextID int32) error) {
	module := vm.NewModule(binAddRequestHeaderV1)
	instance := module.NewInstance()
	defer instance.Stop()

	wasmCtx, err := startABIContextV1(instance)
	if err != nil {
		t.Fatal(err)
	}
	defer wasmCtx.Instance.Stop()

	exports := wasmCtx.GetExports()

	// make the root context
	rootContextID := int32(1)
	if err := exports.ProxyOnContextCreate(rootContextID, int32(0)); err != nil {
		t.Fatal(err)
	}

	// lock wasm vm instance for exclusive ownership
	wasmCtx.Instance.Lock(wasmCtx)
	defer wasmCtx.Instance.Unlock()

	contextID := int32(2)
	if err = exports.ProxyOnContextCreate(contextID, rootContextID); err != nil {
		t.Fatal(err)
	}

	if err = test(wasmCtx, contextID); err != nil {
		t.Fatal(err)
	}

	if _, err = exports.ProxyOnDone(contextID); err != nil {
		t.Fatal(err)
	}

	if err = exports.ProxyOnDelete(contextID); err != nil {
		t.Fatal(err)
	}
}

var _ v1.ImportsHandler = &headersHandlerV1{}

// headersHandlerV1 implements v1.ImportsHandler.
type headersHandlerV1 struct {
	reqHeader common.HeaderMap
	v1.DefaultImportsHandler
}

// override.
func (im *headersHandlerV1) GetHttpRequestHeader() common.HeaderMap {
	return im.reqHeader
}

func TestStartABIContextV2_wazero(t *testing.T) {
	vm := wazero.NewVM()
	defer vm.Close()

	testStartABIContextV2(t, vm)
}

func testStartABIContextV2(t *testing.T, vm common.WasmVM) {
	module := vm.NewModule(binAddRequestHeaderV2)
	instance := module.NewInstance()
	defer instance.Stop()

	if _, err := startABIContextV2(instance); err != nil {
		t.Fatal(err)
	}
}

func startABIContextV2(instance common.WasmInstance) (wasmCtx *v2.ABIContext, err error) {
	// create ABI context
	wasmCtx = &v2.ABIContext{Imports: &v2.DefaultImportsHandler{}, Instance: instance}

	// register ABI imports into the wasm vm instance
	if err = instance.RegisterImports(wasmCtx.Name()); err != nil {
		return
	}

	// start the wasm vm instance
	err = instance.Start()
	return
}

func TestAddRequestHeaderV2_wazero(t *testing.T) {
	vm := wazero.NewVM()
	defer vm.Close()

	testV2(t, vm, testAddRequestHeaderV2)
}

func testAddRequestHeaderV2(wasmCtx *v2.ABIContext, contextID int32) error {
	handler := &headersHandlerV2{reqHeader: &common.CommonHeader{}}
	wasmCtx.SetImports(handler)

	if action, err := wasmCtx.GetExports().ProxyOnRequestHeaders(contextID, 0, 1); err != nil {
		return err
	} else if want, have := v2.ActionContinue, action; want != have {
		return fmt.Errorf("unexpected action, want: %v, have: %v", want, have)
	}

	expectedHeader := "Wasm-Context"
	want := strconv.Itoa(int(contextID))
	if have, _ := handler.GetHttpRequestHeader().Get(expectedHeader); want != have {
		return fmt.Errorf("unexpected %s, want: %v, have: %v", expectedHeader, want, have)
	}
	return nil
}

func testV2(t *testing.T, vm common.WasmVM, test func(wasmCtx *v2.ABIContext, contextID int32) error) {
	module := vm.NewModule(binAddRequestHeaderV2)
	instance := module.NewInstance()
	defer instance.Stop()

	wasmCtx, err := startABIContextV2(instance)
	if err != nil {
		t.Fatal(err)
	}
	defer wasmCtx.Instance.Stop()

	exports := wasmCtx.GetExports()

	// make the root context
	rootContextID := int32(1)
	if err := exports.ProxyOnContextCreate(rootContextID, int32(0), v2.ContextTypeHttpContext); err != nil {
		t.Fatal(err)
	}

	// lock wasm vm instance for exclusive ownership
	wasmCtx.Instance.Lock(wasmCtx)
	defer wasmCtx.Instance.Unlock()

	contextID := int32(2)
	if err = exports.ProxyOnContextCreate(contextID, rootContextID, v2.ContextTypeHttpContext); err != nil {
		t.Fatal(err)
	}

	if err = test(wasmCtx, contextID); err != nil {
		t.Fatal(err)
	}

	if _, err = exports.ProxyOnDone(contextID); err != nil {
		t.Fatal(err)
	}

	if err = exports.ProxyOnDelete(contextID); err != nil {
		t.Fatal(err)
	}
}

var _ v2.ImportsHandler = &headersHandlerV2{}

// headersHandlerV2 implements v2.ImportsHandler.
type headersHandlerV2 struct {
	reqHeader common.HeaderMap
	v2.DefaultImportsHandler
}

// override.
func (im *headersHandlerV2) GetHttpRequestHeader() common.HeaderMap {
	return im.reqHeader
}
