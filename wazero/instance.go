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
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/wasm/abi"
	"mosn.io/pkg/utils"

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

	namespace wazero.Namespace
	instance  api.Module
	abiList   []types.ABI

	lock     sync.Mutex
	started  uint32
	refCount int
	stopCond *sync.Cond

	// user-defined data
	data interface{}
}

type InstanceOptions func(instance *Instance)

func NewInstance(vm *VM, module *Module, options ...InstanceOptions) *Instance {
	// Here, we initialize an empty namespace as imports are defined prior to start.
	ins := &Instance{
		vm:        vm,
		module:    module,
		namespace: vm.runtime.NewNamespace(ctx),
		lock:      sync.Mutex{},
	}

	ins.stopCond = sync.NewCond(&ins.lock)

	for _, option := range options {
		option(ins)
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

// Start makes a new namespace which has the module dependencies of the guest.
func (i *Instance) Start() error {
	ctx := context.Background()
	r := i.vm.runtime
	ns := i.namespace

	if _, err := wasi_snapshot_preview1.NewBuilder(r).Instantiate(ctx, ns); err != nil {
		ns.Close(ctx)
		log.DefaultLogger.Warnf("[wazero][instance] Start fail to create wasi_snapshot_preview1 env, err: %v", err)
		panic(err)
	}

	i.abiList = abi.GetABIList(i)

	// Instantiate any ABI needed by the guest.
	for _, abi := range i.abiList {
		abi.OnInstanceCreate(i)
	}

	ins, err := ns.InstantiateModule(ctx, i.module.module, wazero.NewModuleConfig())
	if err != nil {
		ns.Close(ctx)
		log.DefaultLogger.Errorf("[wazero][instance] Start failed to instantiate module, err: %v", err)
		return err
	}

	// Handle any ABI requirements after the guest is instantiated.
	for _, abi := range i.abiList {
		abi.OnInstanceStart(i)
	}

	i.instance = ins

	atomic.StoreUint32(&i.started, 1)

	return nil
}

func (i *Instance) Stop() {
	utils.GoWithRecover(func() {
		i.lock.Lock()
		for i.refCount > 0 {
			i.stopCond.Wait()
		}
		swapped := atomic.CompareAndSwapUint32(&i.started, 1, 0)
		i.lock.Unlock()

		if swapped {
			for _, abi := range i.abiList {
				abi.OnInstanceDestroy(i)
			}
		}

		if ns := i.namespace; ns != nil {
			ns.Close(ctx)
		}
	}, nil)
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

	r := i.vm.runtime
	ns := i.namespace

	// proxy-wasm cannot run multiple ABI in the same instance because the ABI
	// collides. They all use the same module name: "env"
	module := "env"

	var hostFunctions func(common.WasmInstance) map[string]interface{}
	switch abiName {
	case v1.ProxyWasmABI_0_1_0:
		hostFunctions = importsv1.HostFunctions

		// Instantiate WASI also under the unstable name for old compilers,
		// such as TinyGo 0.19 used for v1 ABI.
		wasiBuilder := r.NewHostModuleBuilder("wasi_unstable")
		wasi_snapshot_preview1.NewFunctionExporter().ExportFunctions(wasiBuilder)
		if _, err := wasiBuilder.Instantiate(ctx, ns); err != nil {
			ns.Close(ctx)
			log.DefaultLogger.Warnf("[wazero][instance] RegisterImports fail to create wasi_unstable env, err: %v", err)
			panic(err)
		}
	case v2.ProxyWasmABI_0_2_0:
		hostFunctions = importsv2.HostFunctions
	default:
		return fmt.Errorf("unknown ABI: %s", abiName)
	}

	b := r.NewHostModuleBuilder(module)
	for n, f := range hostFunctions(i) {
		b.NewFunctionBuilder().WithFunc(f).Export(n)
	}

	if _, err := b.Instantiate(ctx, ns); err != nil {
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
	if wf == nil {
		return nil, fmt.Errorf("[wazero][instance] GetExportsFunc unknown func %s", funcName)
	}
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
		if _, v, err := convertFromGoValue(a); err != nil {
			return nil, err
		} else {
			realArgs = append(realArgs, v)
		}
	}
	if ret, err := f.fn.Call(ctx, realArgs...); err != nil {
		return nil, err
	} else if len(ret) == 0 {
		return nil, nil
	} else {
		return convertToGoValue(f.rt, ret[0])
	}
}

func (i *Instance) GetExportsMem(memName string) ([]byte, error) {
	if !i.checkStart() {
		return nil, ErrInstanceNotStart
	}

	ctx := context.Background()
	mem := i.instance.ExportedMemory(memName)
	return i.GetMemory(0, uint64(mem.Size(ctx)))
}

func (i *Instance) GetMemory(addr uint64, size uint64) ([]byte, error) {
	ctx := context.Background()
	mem := i.instance.Memory()
	ret, ok := mem.Read(ctx, uint32(addr), uint32(size))
	if !ok { // unexpected
		return nil, fmt.Errorf("[wazero][instance] GetMemory unable to read %d bytes", size)
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
