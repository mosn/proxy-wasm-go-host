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
	"errors"
	"fmt"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"sync"
	"sync/atomic"

	wazero "github.com/tetratelabs/wazero"
	"mosn.io/proxy-wasm-go-host/proxywasm/common"
)

var (
	ErrInstanceNotStart     = errors.New("instance has not started")
	ErrInstanceAlreadyStart = errors.New("instance has already started")
)

type Instance struct {
	vm      *VM
	module  *Module
	imports map[string]wazero.HostModuleBuilder

	instance api.Module

	lock     sync.Mutex
	started  uint32
	refCount int
	stopCond *sync.Cond

	// user-defined data
	data interface{}
}

type InstanceOptions func(instance *Instance)

func NewInstance(vm *VM, module *Module, options ...InstanceOptions) *Instance {
	r := vm.engine
	ctx := context.Background()

	ins := &Instance{
		vm:      vm,
		imports: map[string]wazero.HostModuleBuilder{},
		module:  module,
		lock:    sync.Mutex{},
	}
	ins.stopCond = sync.NewCond(&ins.lock)

	for _, option := range options {
		option(ins)
	}

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		_ = r.Close(ctx)
		panic(err)
	}

	return ins
}

func (w *Instance) GetData() interface{} {
	return w.data
}

func (w *Instance) SetData(data interface{}) {
	w.data = data
}

func (w *Instance) Acquire() bool {
	w.lock.Lock()
	defer w.lock.Unlock()

	if !w.checkStart() {
		return false
	}

	w.refCount++

	return true
}

func (w *Instance) Release() {
	w.lock.Lock()
	w.refCount--

	if w.refCount <= 0 {
		w.stopCond.Broadcast()
	}
	w.lock.Unlock()
}

func (w *Instance) Lock(data interface{}) {
	w.lock.Lock()
	w.data = data
}

func (w *Instance) Unlock() {
	w.data = nil
	w.lock.Unlock()
}

func (w *Instance) GetModule() common.WasmModule {
	return w.module
}

func (w *Instance) Start() error {
	ctx := context.Background()

	for n, b := range w.imports {
		if _, err := b.Instantiate(ctx, w.vm.engine); err != nil {
			w.vm.engine.Close(ctx)
			return err
		}
		delete(w.imports, n)
	}

	ins, err := w.vm.engine.InstantiateModule(ctx, w.module.module, wazero.NewModuleConfig())
	if err != nil {
		w.vm.engine.Close(ctx)
		return err
	}

	w.instance = ins

	atomic.StoreUint32(&w.started, 1)

	return nil
}

func (w *Instance) Stop() {
	go func() {
		w.lock.Lock()
		for w.refCount > 0 {
			w.stopCond.Wait()
		}
		w.vm.engine.Close(context.Background())
		_ = atomic.CompareAndSwapUint32(&w.started, 1, 0)
		w.lock.Unlock()
	}()
}

// return true is Instance is started, false if not started.
func (w *Instance) checkStart() bool {
	return atomic.LoadUint32(&w.started) == 1
}

func (w *Instance) RegisterFunc(namespace string, funcName string, f interface{}) error {
	if w.checkStart() {
		return ErrInstanceAlreadyStart
	}

	b, ok := w.imports[namespace]
	if !ok {
		b = w.vm.engine.NewHostModuleBuilder(namespace)
		w.imports[namespace] = b
	}

	b.NewFunctionBuilder().WithFunc(f).Export(funcName)

	return nil
}

func (w *Instance) Malloc(size int32) (uint64, error) {
	if !w.checkStart() {
		return 0, ErrInstanceNotStart
	}

	malloc, err := w.GetExportsFunc("malloc")
	if err != nil {
		return 0, err
	}

	addr, err := malloc.Call(size)
	if err != nil {
		w.HandleError(err)
		return 0, err
	}

	return uint64(addr.(int32)), nil
}

func (w *Instance) GetExportsFunc(funcName string) (common.WasmFunction, error) {
	if !w.checkStart() {
		return nil, ErrInstanceNotStart
	}

	f := w.instance.ExportedFunction(funcName)
	return &wasmFunction{f}, nil
}

type wasmFunction struct {
	fn api.Function
}

// Call implements common.WasmFunction
func (f *wasmFunction) Call(args ...interface{}) (interface{}, error) {
	realArgs := make([]uint64, 0, len(args))
	for _, a := range args {
		switch a := a.(type) {
		case int32:
			realArgs = append(realArgs, api.EncodeI32(a))
		case int64:
			realArgs = append(realArgs, api.EncodeI64(a))
		default:
			panic(fmt.Errorf("unexpected arg type %v", a))
		}
	}
	if ret, err := f.fn.Call(context.Background(), realArgs...); err != nil {
		return nil, err
	} else if len(ret) == 0 {
		return nil, nil
	} else {
		v := ret[0]
		rt := f.fn.Definition().ResultTypes()[0]
		switch rt {
		case api.ValueTypeI32:
			return int32(v), nil
		case api.ValueTypeI64:
			return int64(v), nil
		default:
			panic(fmt.Errorf("unexpected result type %v", rt))
		}
	}
}

func (w *Instance) GetExportsMem(memName string) ([]byte, error) {
	if !w.checkStart() {
		return nil, ErrInstanceNotStart
	}

	ctx := context.Background()
	size := w.instance.ExportedMemory(memName).Size(ctx) * 65536
	return w.GetMemory(0, uint64(size))
}

func (w *Instance) GetMemory(addr uint64, size uint64) ([]byte, error) {
	ctx := context.Background()
	mem := w.instance.Memory()
	ret, ok := mem.Read(ctx, uint32(addr), uint32(size))
	if !ok { // unexpected
		return nil, fmt.Errorf("unable to read %d bytes", size)
	}
	return ret, nil
}

func (w *Instance) PutMemory(addr uint64, size uint64, content []byte) error {
	ctx := context.Background()
	mem := w.instance.Memory()
	ok := mem.Write(ctx, uint32(addr), content[0:size])
	if !ok {
		return errors.New("out of memory")
	}
	return nil
}

func (w *Instance) GetByte(addr uint64) (byte, error) {
	ctx := context.Background()
	mem := w.instance.Memory()
	b, ok := mem.ReadByte(ctx, uint32(addr))
	if !ok {
		return b, errors.New("out of memory")
	}
	return b, nil
}

func (w *Instance) PutByte(addr uint64, b byte) error {
	ctx := context.Background()
	mem := w.instance.Memory()
	ok := mem.WriteByte(ctx, uint32(addr), b)
	if !ok {
		return errors.New("out of memory")
	}
	return nil
}

func (w *Instance) GetUint32(addr uint64) (uint32, error) {
	ctx := context.Background()
	mem := w.instance.Memory()
	n, ok := mem.ReadUint32Le(ctx, uint32(addr))
	if !ok {
		return n, errors.New("out of memory")
	}
	return n, nil
}

func (w *Instance) PutUint32(addr uint64, value uint32) error {
	ctx := context.Background()
	mem := w.instance.Memory()
	ok := mem.WriteUint32Le(ctx, uint32(addr), value)
	if !ok {
		return errors.New("out of memory")
	}
	return nil
}

func (w *Instance) HandleError(error) {}
