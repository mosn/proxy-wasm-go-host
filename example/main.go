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

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	proxywasm "mosn.io/proxy-wasm-go-host/proxywasm/v1"
	"mosn.io/proxy-wasm-go-host/wazero"
)

var contextIDGenerator int32
var rootContextID int32

var lock sync.Mutex
var once sync.Once
var instance common.WasmInstance

// implement proxywasm.ImportsHandler.
type importHandler struct {
	reqHeader common.HeaderMap
	proxywasm.DefaultImportsHandler
}

// override.
func (im *importHandler) GetHttpRequestHeader() common.HeaderMap {
	return im.reqHeader
}

// override.
func (im *importHandler) Log(level proxywasm.LogLevel, msg string) proxywasm.WasmResult {
	fmt.Println(msg)
	return proxywasm.WasmResultOk
}

// serve HTTP req
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("receive request %s\n", r.URL)
	for k, v := range r.Header {
		fmt.Printf("print header from server host, %v -> %v\n", k, v)
	}

	// get wasm vm instance
	instance := getWasmInstance()

	// create abi context
	ctx := &proxywasm.ABIContext{
		Imports:  &importHandler{reqHeader: &myHeaderMap{r.Header}},
		Instance: instance,
	}

	// create context id for the http req
	contextID := atomic.AddInt32(&contextIDGenerator, 1)

	// do wasm

	// according to ABI, we should create a root context id before any operations
	once.Do(func() {
		_ = ctx.GetExports().ProxyOnContextCreate(rootContextID, 0)
	})

	// lock wasm vm instance for exclusive ownership
	instance.Lock(ctx)
	defer instance.Unlock()

	// create wasm-side context id for current http req
	_ = ctx.GetExports().ProxyOnContextCreate(contextID, rootContextID)

	// call wasm-side on_request_header
	_, _ = ctx.GetExports().ProxyOnRequestHeaders(contextID, int32(len(r.Header)), 1)

	// delete wasm-side context id to prevent memory leak
	_ = ctx.GetExports().ProxyOnDelete(contextID)

	// reply with ok
	w.WriteHeader(http.StatusOK)
}

func main() {
	// create root context id
	rootContextID = atomic.AddInt32(&contextIDGenerator, 1)

	// serve http
	http.HandleFunc("/", ServeHTTP)
	_ = http.ListenAndServe("127.0.0.1:2045", nil)
}

func getWasmInstance() common.WasmInstance {
	lock.Lock()
	defer lock.Unlock()

	if instance == nil {
		pwd, _ := os.Getwd()
		instance = wazero.NewInstanceFromFile(filepath.Join(pwd, "data/http.wasm"))

		// register ABI imports into the wasm vm instance
		proxywasm.RegisterImports(instance)

		// start the wasm vm instance
		_ = instance.Start()
	}

	return instance
}

// wrapper for http.Header, convert Header to api.HeaderMap.
type myHeaderMap struct {
	realMap http.Header
}

func (m *myHeaderMap) Get(key string) (string, bool) {
	return m.realMap.Get(key), true
}

func (m *myHeaderMap) Set(key, value string) { panic("implemented") }

func (m *myHeaderMap) Add(key, value string) { panic("implemented") }

func (m *myHeaderMap) Del(key string) { panic("implemented") }

func (m *myHeaderMap) Range(f func(key string, value string) bool) {
	for k, _ := range m.realMap {
		v := m.realMap.Get(k)
		f(k, v)
	}
}

func (m *myHeaderMap) Clone() common.HeaderMap { panic("implemented") }

func (m *myHeaderMap) ByteSize() uint64 { panic("implemented") }
