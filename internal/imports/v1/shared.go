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

package v1

import (
	"context"

	v1 "mosn.io/proxy-wasm-go-host/proxywasm/v1"
)

func (h *host) ProxyGetProperty(ctx context.Context, keyPtr int32, keySize int32, returnValueData int32, returnValueSize int32) int32 {
	instance := h.Instance
	key, err := instance.GetMemory(uint64(keyPtr), uint64(keySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	ih := getImportHandler(instance)

	value, res := ih.GetProperty(string(key))
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	return copyIntoInstance(instance, value, returnValueData, returnValueSize).Int32()
}

func (h *host) ProxySetProperty(ctx context.Context, keyPtr int32, keySize int32, valuePtr int32, valueSize int32) int32 {
	instance := h.Instance
	key, err := instance.GetMemory(uint64(keyPtr), uint64(keySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	value, err := instance.GetMemory(uint64(valuePtr), uint64(valueSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	ih := getImportHandler(instance)

	return ih.SetProperty(string(key), string(value)).Int32()
}

func (h *host) ProxyRegisterSharedQueue(ctx context.Context, queueNamePtr int32, queueNameSize int32, tokenPtr int32) int32 {
	instance := h.Instance
	queueName, err := instance.GetMemory(uint64(queueNamePtr), uint64(queueNameSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(queueName) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	ih := getImportHandler(instance)

	queueID, res := ih.RegisterSharedQueue(string(queueName))
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	err = instance.PutUint32(uint64(tokenPtr), queueID)
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxyRemoveSharedQueue(ctx context.Context, queueID int32) int32 {
	ih := getImportHandler(h.Instance)
	res := ih.RemoveSharedQueue(uint32(queueID))

	return res.Int32()
}

func (h *host) ProxyResolveSharedQueue(ctx context.Context, queueNamePtr int32, queueNameSize int32, tokenPtr int32) int32 {
	instance := h.Instance
	queueName, err := instance.GetMemory(uint64(queueNamePtr), uint64(queueNameSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(queueName) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	ih := getImportHandler(instance)

	queueID, res := ih.ResolveSharedQueue(string(queueName))
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	err = instance.PutUint32(uint64(tokenPtr), queueID)
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxyDequeueSharedQueue(ctx context.Context, token int32, dataPtr int32, dataSize int32) int32 {
	instance := h.Instance
	ih := getImportHandler(instance)

	value, res := ih.DequeueSharedQueue(uint32(token))
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	return copyIntoInstance(instance, value, dataPtr, dataSize).Int32()
}

func (h *host) ProxyEnqueueSharedQueue(ctx context.Context, token int32, dataPtr int32, dataSize int32) int32 {
	instance := h.Instance
	value, err := instance.GetMemory(uint64(dataPtr), uint64(dataSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	ih := getImportHandler(instance)

	return ih.EnqueueSharedQueue(uint32(token), string(value)).Int32()
}

func (h *host) ProxyGetSharedData(ctx context.Context, keyPtr int32, keySize int32, valuePtr int32, valueSizePtr int32, casPtr int32) int32 {
	instance := h.Instance
	key, err := instance.GetMemory(uint64(keyPtr), uint64(keySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	ih := getImportHandler(instance)

	v, cas, res := ih.GetSharedData(string(key))
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	res = copyIntoInstance(instance, v, valuePtr, valueSizePtr)
	if res != v1.WasmResultOk {
		return res.Int32()
	}

	err = instance.PutUint32(uint64(casPtr), cas)
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	return v1.WasmResultOk.Int32()
}

func (h *host) ProxySetSharedData(ctx context.Context, keyPtr int32, keySize int32, valuePtr int32, valueSize int32, cas int32) int32 {
	instance := h.Instance
	key, err := instance.GetMemory(uint64(keyPtr), uint64(keySize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}
	if len(key) == 0 {
		return v1.WasmResultBadArgument.Int32()
	}

	value, err := instance.GetMemory(uint64(valuePtr), uint64(valueSize))
	if err != nil {
		return v1.WasmResultInvalidMemoryAccess.Int32()
	}

	ih := getImportHandler(instance)

	return ih.SetSharedData(string(key), string(value), uint32(cas)).Int32()
}
