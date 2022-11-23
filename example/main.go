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
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	v1 "mosn.io/proxy-wasm-go-host/proxywasm/v1"
	"mosn.io/proxy-wasm-go-host/wazero"
)

var (
	contextIDGenerator int32
	rootContextID      int32
)

var (
	lock    sync.Mutex
	once    sync.Once
	wasmCtx *v1.ABIContext
)

var _ v1.ImportsHandler = &importHandler{}

// implement v1.ImportsHandler.
type importHandler struct {
	reqHeader common.HeaderMap
	v1.DefaultImportsHandler
}

// override.
func (im *importHandler) GetHttpRequestHeader() common.HeaderMap {
	return im.reqHeader
}

// override.
func (im *importHandler) Log(level v1.LogLevel, msg string) v1.WasmResult {
	fmt.Println(msg)
	return v1.WasmResultOk
}

// serve HTTP req
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("receive request %s\n", r.URL)
	for k, v := range r.Header {
		fmt.Printf("print header from server host, %v -> %v\n", k, v)
	}

	// get wasm vm instance
	ctx := getWasmContext()

	// create context id for the http req
	contextID := atomic.AddInt32(&contextIDGenerator, 1)

	// do wasm

	// according to ABI, we should create a root context id before any operations
	once.Do(func() {
		if err := ctx.GetExports().ProxyOnContextCreate(rootContextID, 0); err != nil {
			log.Panicln(err)
		}
	})

	// lock wasm vm instance for exclusive ownership
	ctx.Instance.Lock(ctx)
	defer ctx.Instance.Unlock()

	// Set the import handler to the current request.
	ctx.SetImports(&importHandler{reqHeader: &myHeaderMap{r.Header}})

	// create wasm-side context id for current http req
	if err := ctx.GetExports().ProxyOnContextCreate(contextID, rootContextID); err != nil {
		log.Panicln(err)
	}

	// call wasm-side on_request_header
	if _, err := ctx.GetExports().ProxyOnRequestHeaders(contextID, int32(len(r.Header)), 1); err != nil {
		log.Panicln(err)
	}

	// delete wasm-side context id to prevent memory leak
	if err := ctx.GetExports().ProxyOnDelete(contextID); err != nil {
		log.Panicln(err)
	}

	// reply with ok
	w.WriteHeader(http.StatusOK)
}

var vm common.WasmVM

func main() {
	defer func() {
		if vm != nil {
			vm.Close()
		}
	}()

	// create root context id
	rootContextID = atomic.AddInt32(&contextIDGenerator, 1)

	// serve http
	http.HandleFunc("/", ServeHTTP)
	if err := http.ListenAndServe("127.0.0.1:2045", nil); err != nil {
		log.Panicln(err)
	}
}

func getWasmContext() *v1.ABIContext {
	lock.Lock()
	defer lock.Unlock()

	if wasmCtx == nil {
		guest, err := os.ReadFile("data/http.wasm")
		if err != nil {
			log.Panicln(err)
		}

		vm = wazero.NewVM()

		module := vm.NewModule(guest)

		instance := module.NewInstance()

		// create ABI context
		wasmCtx = &v1.ABIContext{
			Imports:  &v1.DefaultImportsHandler{},
			Instance: instance,
		}

		// register ABI imports into the wasm vm instance
		if err = instance.RegisterImports(wasmCtx.Name()); err != nil {
			log.Panicln(err)
		}

		// start the wasm vm instance
		if err = instance.Start(); err != nil {
			log.Panicln(err)
		}
	}

	return wasmCtx
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
	for k := range m.realMap {
		v := m.realMap.Get(k)
		f(k, v)
	}
}

func (m *myHeaderMap) Clone() common.HeaderMap { panic("implemented") }

func (m *myHeaderMap) ByteSize() uint64 { panic("implemented") }
