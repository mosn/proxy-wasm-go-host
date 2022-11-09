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

package wasmer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"

	wasmerGo "github.com/wasmerio/wasmer-go/wasmer"
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
	ErrAddrOverflow         = errors.New("addr overflow")
	ErrInstanceNotStart     = errors.New("instance has not started")
	ErrInstanceAlreadyStart = errors.New("instance has already started")
	ErrInvalidParam         = errors.New("invalid param")
	ErrRegisterNotFunc      = errors.New("register a non-func object")
	ErrRegisterArgNum       = errors.New("register func with invalid arg num")
	ErrRegisterArgType      = errors.New("register func with invalid arg type")
)

type Instance struct {
	vm           *VM
	module       *Module
	importObject *wasmerGo.ImportObject
	instance     *wasmerGo.Instance
	debug        *dwarfInfo
	abiList      []types.ABI

	lock     sync.Mutex
	started  uint32
	refCount int
	stopCond *sync.Cond

	// for cache
	memory    *wasmerGo.Memory
	funcCache sync.Map // string -> *wasmerGo.Function

	// user-defined data
	data interface{}
}

type InstanceOptions func(instance *Instance)

func InstanceWithDebug(debug *dwarfInfo) InstanceOptions {
	return func(instance *Instance) {
		if debug != nil {
			instance.debug = debug
		}
	}
}

func NewWasmerInstance(vm *VM, module *Module, options ...InstanceOptions) *Instance {
	ins := &Instance{
		vm:     vm,
		module: module,
		lock:   sync.Mutex{},
	}
	ins.stopCond = sync.NewCond(&ins.lock)

	for _, option := range options {
		option(ins)
	}

	wasiEnv, err := wasmerGo.NewWasiStateBuilder("").Finalize()
	if err != nil || wasiEnv == nil {
		log.DefaultLogger.Warnf("[wasmer][instance] NewWasmerInstance fail to create wasi env, err: %v", err)

		ins.importObject = wasmerGo.NewImportObject()
		return ins
	}

	imo, err := wasiEnv.GenerateImportObject(ins.vm.store, ins.module.module)
	if err != nil {
		log.DefaultLogger.Warnf("[wasmer][instance] NewWasmerInstance fail to create import object, err: %v", err)

		ins.importObject = wasmerGo.NewImportObject()
	} else {
		ins.importObject = imo
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

func (i *Instance) GetModule() types.WasmModule {
	return i.module
}

func (i *Instance) Start() error {
	i.abiList = abi.GetABIList(i)

	for _, abi := range i.abiList {
		abi.OnInstanceCreate(i)
	}

	ins, err := wasmerGo.NewInstance(i.module.module, i.importObject)
	if err != nil {
		log.DefaultLogger.Errorf("[wasmer][instance] Start fail to new wasmer-go instance, err: %v", err)
		return err
	}

	i.instance = ins

	f, err := i.instance.Exports.GetFunction("_start")
	if err != nil {
		log.DefaultLogger.Errorf("[wasmer][instance] Start fail to get export func: _start, err: %v", err)
		return err
	}

	_, err = f()
	if err != nil {
		log.DefaultLogger.Errorf("[wasmer][instance] Start fail to call _start func, err: %v", err)
		i.HandleError(err)
		return err
	}

	for _, abi := range i.abiList {
		abi.OnInstanceStart(i)
	}

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
	}, nil)
}

// return true is Instance is started, false if not started.
func (i *Instance) checkStart() bool {
	return atomic.LoadUint32(&i.started) == 1
}

func (i *Instance) RegisterImports(abiName string) error {
	if i.checkStart() {
		log.DefaultLogger.Errorf("[wasmer][instance] RegisterFunc not allow to register func after instance started, abiName: %s",
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

	for n, f := range hostFunctions(i) {
		if err := i.registerFunc(module, n, f); err != nil {
			return err
		}
	}
	return nil
}

func (i *Instance) registerFunc(namespace string, funcName string, f interface{}) error {
	if namespace == "" || funcName == "" {
		log.DefaultLogger.Errorf("[wasmer][instance] RegisterFunc invalid param, namespace: %v, funcName: %v", namespace, funcName)
		return ErrInvalidParam
	}

	if f == nil || reflect.ValueOf(f).IsNil() {
		log.DefaultLogger.Errorf("[wasmer][instance] RegisterFunc f is nil")
		return ErrInvalidParam
	}

	if reflect.TypeOf(f).Kind() != reflect.Func {
		log.DefaultLogger.Errorf("[wasmer][instance] RegisterFunc f is not func, actual type: %v", reflect.TypeOf(f))
		return ErrRegisterNotFunc
	}

	funcType := reflect.TypeOf(f)

	argsNum := funcType.NumIn()
	if argsNum < 1 {
		log.DefaultLogger.Errorf("[wasmer][instance] RegisterFunc invalid args num: %v, must >= 1", argsNum)
		return ErrRegisterArgNum
	}

	argsKind := make([]*wasmerGo.ValueType, argsNum-1)
	for i := 1; i < argsNum; i++ {
		argsKind[i-1] = convertFromGoType(funcType.In(i))
	}

	retsNum := funcType.NumOut()
	retsKind := make([]*wasmerGo.ValueType, retsNum)
	for i := 0; i < retsNum; i++ {
		retsKind[i] = convertFromGoType(funcType.Out(i))
	}

	fwasmer := wasmerGo.NewFunction(
		i.vm.store,
		wasmerGo.NewFunctionType(argsKind, retsKind),
		func(args []wasmerGo.Value) (callRes []wasmerGo.Value, err error) {
			defer func() {
				if r := recover(); r != nil {
					log.DefaultLogger.Errorf("[wasmer][instance] RegisterFunc recover func call: %v, r: %v, stack: %v",
						funcName, r, string(debug.Stack()))
					callRes = nil
					err = fmt.Errorf("panic [%v] when calling func [%v]", r, funcName)
				}
			}()

			callArgs := make([]reflect.Value, 1+len(args))

			// wasmer cannot propagate context at the moment, so substitute with context.Background
			callArgs[0] = reflect.ValueOf(context.Background())

			for i, arg := range args {
				callArgs[i+1] = convertToGoTypes(arg)
			}

			callResult := reflect.ValueOf(f).Call(callArgs)

			ret := convertFromGoValue(callResult[0])

			return []wasmerGo.Value{ret}, nil
		},
	)

	i.importObject.Register(namespace, map[string]wasmerGo.IntoExtern{
		funcName: fwasmer,
	})

	return nil
}

func (i *Instance) Malloc(size int32) (uint64, error) {
	if !i.checkStart() {
		log.DefaultLogger.Errorf("[wasmer][instance] call malloc before starting instance")
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

func (i *Instance) GetExportsFunc(funcName string) (types.WasmFunction, error) {
	if !i.checkStart() {
		log.DefaultLogger.Errorf("[wasmer][instance] call GetExportsFunc before starting instance")
		return nil, ErrInstanceNotStart
	}

	if v, ok := i.funcCache.Load(funcName); ok {
		return v.(*wasmerGo.Function), nil
	}

	f, err := i.instance.Exports.GetRawFunction(funcName)
	if err != nil {
		return nil, err
	}

	i.funcCache.Store(funcName, f)

	return f, nil
}

func (i *Instance) GetExportsMem(memName string) ([]byte, error) {
	if !i.checkStart() {
		log.DefaultLogger.Errorf("[wasmer][instance] call GetExportsMem before starting instance")
		return nil, ErrInstanceNotStart
	}

	if i.memory == nil {
		m, err := i.instance.Exports.GetMemory(memName)
		if err != nil {
			return nil, err
		}

		i.memory = m
	}

	return i.memory.Data(), nil
}

func (i *Instance) GetMemory(addr uint64, size uint64) ([]byte, error) {
	mem, err := i.GetExportsMem("memory")
	if err != nil {
		return nil, err
	}

	if int(addr) > len(mem) || int(addr+size) > len(mem) {
		return nil, ErrAddrOverflow
	}

	return mem[addr : addr+size], nil
}

func (i *Instance) PutMemory(addr uint64, size uint64, content []byte) error {
	mem, err := i.GetExportsMem("memory")
	if err != nil {
		return err
	}

	if int(addr) > len(mem) || int(addr+size) > len(mem) {
		return ErrAddrOverflow
	}

	copySize := uint64(len(content))
	if size < copySize {
		copySize = size
	}

	copy(mem[addr:], content[:copySize])

	return nil
}

func (i *Instance) GetByte(addr uint64) (byte, error) {
	mem, err := i.GetExportsMem("memory")
	if err != nil {
		return 0, err
	}

	if int(addr) > len(mem) {
		return 0, ErrAddrOverflow
	}

	return mem[addr], nil
}

func (i *Instance) PutByte(addr uint64, b byte) error {
	mem, err := i.GetExportsMem("memory")
	if err != nil {
		return err
	}

	if int(addr) > len(mem) {
		return ErrAddrOverflow
	}

	mem[addr] = b

	return nil
}

func (i *Instance) GetUint32(addr uint64) (uint32, error) {
	mem, err := i.GetExportsMem("memory")
	if err != nil {
		return 0, err
	}

	if int(addr) > len(mem) || int(addr+4) > len(mem) {
		return 0, ErrAddrOverflow
	}

	return binary.LittleEndian.Uint32(mem[addr:]), nil
}

func (i *Instance) PutUint32(addr uint64, value uint32) error {
	mem, err := i.GetExportsMem("memory")
	if err != nil {
		return err
	}

	if int(addr) > len(mem) || int(addr+4) > len(mem) {
		return ErrAddrOverflow
	}

	binary.LittleEndian.PutUint32(mem[addr:], value)

	return nil
}

func (i *Instance) HandleError(err error) {
	var trapError *wasmerGo.TrapError
	if !errors.As(err, &trapError) {
		return
	}

	trace := trapError.Trace()
	if trace == nil {
		return
	}

	log.DefaultLogger.Errorf("[wasmer][instance] HandleError err: %v, trace:", err)

	if i.debug == nil {
		// do not have dwarf debug info
		for _, t := range trace {
			log.DefaultLogger.Errorf("[wasmer][instance]\t funcIndex: %v, funcOffset: 0x%08x, moduleOffset: 0x%08x",
				t.FunctionIndex(), t.FunctionOffset(), t.ModuleOffset())
		}
	} else {
		for _, t := range trace {
			pc := uint64(t.ModuleOffset())
			line := i.debug.SeekPC(pc)
			if line != nil {
				log.DefaultLogger.Errorf("[wasmer][instance]\t funcIndex: %v, funcOffset: 0x%08x, pc: 0x%08x %v:%v",
					t.FunctionIndex(), t.FunctionOffset(), pc, line.File.Name, line.Line)
			} else {
				log.DefaultLogger.Errorf("[wasmer][instance]\t funcIndex: %v, funcOffset: 0x%08x, pc: 0x%08x fail to seek pc",
					t.FunctionIndex(), t.FunctionOffset(), t.ModuleOffset())
			}
		}
	}
}
