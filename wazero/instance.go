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
	"sync"
	"sync/atomic"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"mosn.io/mosn/pkg/log"

	importsv1 "mosn.io/proxy-wasm-go-host/internal/imports/v1"
	importsv2 "mosn.io/proxy-wasm-go-host/internal/imports/v2"
	"mosn.io/proxy-wasm-go-host/proxywasm/common"
	v1 "mosn.io/proxy-wasm-go-host/proxywasm/v1"
	v2 "mosn.io/proxy-wasm-go-host/proxywasm/v2"
)

var (
	ErrInstanceNotStart     = errors.New("instance has not started")
	ErrInstanceAlreadyStart = errors.New("instance has already started")
)

type Instance struct {
	vm     *VM
	module *Module

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
	r := vm.runtime

	ins := &Instance{
		vm:     vm,
		module: module,
		lock:   sync.Mutex{},
	}
	ins.stopCond = sync.NewCond(&ins.lock)

	for _, option := range options {
		option(ins)
	}

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		_ = r.Close(ctx)
		log.DefaultLogger.Warnf("[wazero][instance] NewInstance fail to create wasi_snapshot_preview1 env, err: %v", err)
		panic(err)
	}

	// Instantiate WASI also under the unstable name for old compilers,
	// such as TinyGo 0.19 used for v1 ABI.
	wasiBuilder := r.NewHostModuleBuilder("wasi_unstable")
	wasi_snapshot_preview1.NewFunctionExporter().ExportFunctions(wasiBuilder)
	if _, err := wasiBuilder.Instantiate(ctx, r); err != nil {
		log.DefaultLogger.Warnf("[wazero][instance] NewInstance fail to create wasi_unstable env, err: %v", err)
		_ = r.Close(ctx)
		panic(err)
	}

	return ins
}

func (i *Instance) GetData() interface{} {
	return i.data
}

func (i *Instance) SetData(data interface{}) {
	i.data = data
}

func (i *Instance) Acquire() bool {
	i.lock.Lock()
	defer i.lock.Unlock()

	if !i.checkStart() {
		return false
	}

	i.refCount++

	return true
}

func (i *Instance) Release() {
	i.lock.Lock()
	i.refCount--

	if i.refCount <= 0 {
		i.stopCond.Broadcast()
	}
	i.lock.Unlock()
}

func (i *Instance) Lock(data interface{}) {
	i.lock.Lock()
	i.data = data
}

func (i *Instance) Unlock() {
	i.data = nil
	i.lock.Unlock()
}

func (i *Instance) GetModule() common.WasmModule {
	return i.module
}

func (i *Instance) Start() error {
	ctx := context.Background()

	ins, err := i.vm.runtime.InstantiateModule(ctx, i.module.module, wazero.NewModuleConfig())
	if err != nil {
		log.DefaultLogger.Errorf("[wazero][instance] Start failed to instantiate module, err: %v", err)
		i.vm.runtime.Close(ctx)
		return err
	}

	i.instance = ins

	atomic.StoreUint32(&i.started, 1)

	return nil
}

func (i *Instance) Stop() {
	go func() {
		i.lock.Lock()
		for i.refCount > 0 {
			i.stopCond.Wait()
		}
		i.vm.runtime.Close(context.Background())
		_ = atomic.CompareAndSwapUint32(&i.started, 1, 0)
		i.lock.Unlock()
	}()
}

// return true is Instance is started, false if not started.
func (i *Instance) checkStart() bool {
	return atomic.LoadUint32(&i.started) == 1
}

func (i *Instance) RegisterImports(abiName string) error {
	if i.checkStart() {
		log.DefaultLogger.Errorf("[wazero][instance] RegisterFunc not allow to register func after instance started, abiName: %s",
			abiName)
		return ErrInstanceAlreadyStart
	}

	// proxy-wasm cannot run multiple ABI in the same instance because the ABI
	// collides. They all use the same module name: "env"
	module := "env"

	var hostFunctions func(common.WasmInstance) map[string]interface{}
	switch abiName {
	case v1.ProxyWasmABI_0_1_0:
		hostFunctions = importsv1.HostFunctions
	case v2.ProxyWasmABI_0_2_0:
		hostFunctions = importsv2.HostFunctions
	default:
		return fmt.Errorf("unknown ABI: %s", abiName)
	}

	b := i.vm.runtime.NewHostModuleBuilder(module)
	for n, f := range hostFunctions(i) {
		b.NewFunctionBuilder().WithFunc(f).Export(n)
	}

	if _, err := b.Instantiate(ctx, i.vm.runtime); err != nil {
		log.DefaultLogger.Errorf("[wazero][instance] RegisterImports failed to instantiate ABI %s, err: %v", abiName, err)
		return err
	}
	return nil
}

func (i *Instance) Malloc(size int32) (uint64, error) {
	if !i.checkStart() {
		return 0, ErrInstanceNotStart
	}

	malloc, err := i.GetExportsFunc("malloc")
	if err != nil {
		return 0, err
	}

	addr, err := malloc.Call(size)
	if err != nil {
		i.HandleError(err)
		return 0, err
	}

	return uint64(addr.(int32)), nil
}

func (i *Instance) GetExportsFunc(funcName string) (common.WasmFunction, error) {
	if !i.checkStart() {
		return nil, ErrInstanceNotStart
	}

	wf := i.instance.ExportedFunction(funcName)
	f := &wasmFunction{fn: wf}
	if rts := wf.Definition().ResultTypes(); len(rts) > 0 {
		f.rt = rts[0]
	}
	return f, nil
}

type wasmFunction struct {
	fn api.Function
	rt api.ValueType
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
	if ret, err := f.fn.Call(ctx, realArgs...); err != nil {
		return nil, err
	} else if len(ret) == 0 {
		return nil, nil
	} else {
		v := ret[0]
		switch f.rt {
		case api.ValueTypeI32:
			return int32(v), nil
		case api.ValueTypeI64:
			return int64(v), nil
		default:
			panic(fmt.Errorf("unexpected result type %v", f.rt))
		}
	}
}

func (i *Instance) GetExportsMem(memName string) ([]byte, error) {
	if !i.checkStart() {
		return nil, ErrInstanceNotStart
	}

	ctx := context.Background()
	size := i.instance.ExportedMemory(memName).Size(ctx) * 65536
	return i.GetMemory(0, uint64(size))
}

func (i *Instance) GetMemory(addr uint64, size uint64) ([]byte, error) {
	ctx := context.Background()
	mem := i.instance.Memory()
	ret, ok := mem.Read(ctx, uint32(addr), uint32(size))
	if !ok { // unexpected
		return nil, fmt.Errorf("unable to read %d bytes", size)
	}
	return ret, nil
}

func (i *Instance) PutMemory(addr uint64, size uint64, content []byte) error {
	ctx := context.Background()
	mem := i.instance.Memory()
	ok := mem.Write(ctx, uint32(addr), content[0:size])
	if !ok {
		return errors.New("out of memory")
	}
	return nil
}

func (i *Instance) GetByte(addr uint64) (byte, error) {
	ctx := context.Background()
	mem := i.instance.Memory()
	b, ok := mem.ReadByte(ctx, uint32(addr))
	if !ok {
		return b, errors.New("out of memory")
	}
	return b, nil
}

func (i *Instance) PutByte(addr uint64, b byte) error {
	ctx := context.Background()
	mem := i.instance.Memory()
	ok := mem.WriteByte(ctx, uint32(addr), b)
	if !ok {
		return errors.New("out of memory")
	}
	return nil
}

func (i *Instance) GetUint32(addr uint64) (uint32, error) {
	ctx := context.Background()
	mem := i.instance.Memory()
	n, ok := mem.ReadUint32Le(ctx, uint32(addr))
	if !ok {
		return n, errors.New("out of memory")
	}
	return n, nil
}

func (i *Instance) PutUint32(addr uint64, value uint32) error {
	ctx := context.Background()
	mem := i.instance.Memory()
	ok := mem.WriteUint32Le(ctx, uint32(addr), value)
	if !ok {
		return errors.New("out of memory")
	}
	return nil
}

func (i *Instance) HandleError(error) {}
